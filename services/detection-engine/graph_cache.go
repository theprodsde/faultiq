package main

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/faultiq/graphclient"
	"golang.org/x/sync/singleflight"
)

// GraphCache maintains an in-memory copy of graphs per namespace.
// This eliminates the HTTP fetch on every signal, bringing detection latency to <10ms.
type GraphCache struct {
	mu       sync.RWMutex
	graphs   map[string]*cachedGraph
	gp       GraphProvider
	sfGroup  singleflight.Group
}

type cachedGraph struct {
	graph        *Graph
	reverseEdges map[string][]string
	fetchedAt    time.Time
}

const graphCacheTTL = 30 * time.Second // refresh graph every 30s

func NewGraphCache(gp GraphProvider) *GraphCache {
	return &GraphCache{
		graphs: make(map[string]*cachedGraph),
		gp:     gp,
	}
}

// Get returns the cached graph for a namespace. If not cached or stale, fetches async
// but returns whatever is in cache immediately (stale-while-revalidate pattern).
func (c *GraphCache) Get(ctx context.Context, namespace string) *Graph {
	c.mu.RLock()
	entry, ok := c.graphs[namespace]
	c.mu.RUnlock()

	if ok && entry.graph != nil {
		// If stale, trigger async refresh — singleflight coalesces concurrent calls
		if time.Since(entry.fetchedAt) > graphCacheTTL {
			go func() {
				c.sfGroup.Do(namespace, func() (interface{}, error) {
					c.refresh(namespace)
					return nil, nil
				})
			}()
		}
		return entry.graph
	}

	// Cache miss — must fetch synchronously (first time only)
	return c.fetchAndCache(ctx, namespace)
}

// GetWithReverse returns the cached graph and its precomputed reverse edge map.
// If not cached or stale, behaves like Get (stale-while-revalidate).
func (c *GraphCache) GetWithReverse(ctx context.Context, namespace string) (*Graph, map[string][]string) {
	c.mu.RLock()
	entry, ok := c.graphs[namespace]
	c.mu.RUnlock()

	if ok && entry.graph != nil {
		if time.Since(entry.fetchedAt) > graphCacheTTL {
			go func() {
				c.sfGroup.Do(namespace, func() (interface{}, error) {
					c.refresh(namespace)
					return nil, nil
				})
			}()
		}
		return entry.graph, entry.reverseEdges
	}

	g := c.fetchAndCache(ctx, namespace)
	// Re-read entry to get reverseEdges after fetch
	c.mu.RLock()
	entry, ok = c.graphs[namespace]
	c.mu.RUnlock()
	if ok && entry.reverseEdges != nil {
		return g, entry.reverseEdges
	}
	return g, buildReverseEdges(g.Edges)
}

func (c *GraphCache) refresh(namespace string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c.fetchAndCache(ctx, namespace)
}

func (c *GraphCache) fetchAndCache(ctx context.Context, namespace string) *Graph {
	remote, err := c.gp.GetSubgraph(ctx, namespace)
	if err != nil {
		log.Printf("graph-cache: fetch failed for %s: %v", namespace, err)
		// Return stale if available
		c.mu.RLock()
		if entry, ok := c.graphs[namespace]; ok && entry.graph != nil {
			c.mu.RUnlock()
			return entry.graph
		}
		c.mu.RUnlock()
		return &Graph{Nodes: make(map[string]*Node), Edges: make(map[string][]string)}
	}

	g := convertFromRemote(remote)

	c.mu.Lock()
	c.graphs[namespace] = &cachedGraph{graph: g, reverseEdges: buildReverseEdges(g.Edges), fetchedAt: time.Now()}
	c.mu.Unlock()

	return g
}

// Invalidate forces a refresh on next access
func (c *GraphCache) Invalidate(namespace string) {
	c.mu.Lock()
	delete(c.graphs, namespace)
	c.mu.Unlock()
}

// Preload fetches graphs for known namespaces at startup
func (c *GraphCache) Preload(ctx context.Context, namespaces []string) {
	for _, ns := range namespaces {
		go c.fetchAndCache(ctx, ns)
	}
}

func convertFromRemote(g *graphclient.Graph) *Graph {
	lg := &Graph{Nodes: make(map[string]*Node), Edges: make(map[string][]string)}
	if g == nil {
		return lg
	}
	for id, n := range g.Nodes {
		if n == nil {
			continue
		}
		lg.Nodes[id] = &Node{ID: n.ID, StatusClass: n.StatusClass, LatencyP95: n.LatencyP95, ErrorRate: n.ErrorRate}
	}
	for k, arr := range g.Edges {
		lg.Edges[k] = append([]string{}, arr...)
	}
	return lg
}
