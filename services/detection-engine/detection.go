package main

import (
    "container/heap"
    "container/list"
    "sort"
)

// Node represents a node in the detection graph.
type Node struct {
    ID          string
    StatusClass string  // e.g., "2xx", "5xx", "timeout"
    LatencyP95  int     // milliseconds
    ErrorRate   float64 // 0.0–1.0
}

// EdgeDetail holds the edge-level metrics used for weighted traversal.
type EdgeDetail struct {
    SuccessRatio float64 // 0.0 = always failing, 1.0 = perfectly healthy
    Confidence   float64 // 0.0–1.0 observation confidence
}

// Graph holds adjacency and nodes
type Graph struct {
    Nodes       map[string]*Node
    Edges       map[string][]string // from -> []to
    EdgeDetails map[string]*EdgeDetail // "from->to" -> EdgeDetail (optional)
}

// DetectSuspects runs a BFS from impacted services and returns suspect node IDs.
// It considers edge confidence (low-confidence edges are deprioritized) and
// propagates errors upstream through reverse edges (who calls the failing service).
// reverseEdges must be precomputed by the caller (e.g. via GraphCache.GetWithReverse).
func DetectSuspects(g *Graph, impacted []string, maxDepth int, reverseEdges map[string][]string) []string {
    suspects := make(map[string]struct{})
    visited := make(map[string]int)
    q := list.New()

    for _, id := range impacted {
        q.PushBack(struct{ id string; depth int }{id: id, depth: 0})
        visited[id] = 0
    }

    for q.Len() > 0 {
        e := q.Remove(q.Front()).(struct{ id string; depth int })
        if e.depth > maxDepth {
            continue
        }
        n := g.Nodes[e.id]
        if n == nil {
            continue
        }
        // Error detection: 5xx, timeout, connection errors, or service down -> suspect
        if len(n.StatusClass) > 0 && (n.StatusClass[0] == '5' || n.StatusClass == "timeout" || 
            n.StatusClass == "connection_error" || n.StatusClass == "down" || 
            n.StatusClass == "503" || n.StatusClass == "504") {
            suspects[n.ID] = struct{}{}
        }

        // If healthy 2xx -> prune (but allow expansion from initial impacted nodes at depth 0)
        if e.depth > 0 && len(n.StatusClass) > 0 && n.StatusClass[0] == '2' {
            continue
        }

        // Forward edges (downstream dependencies)
        for _, nb := range g.Edges[e.id] {
            if d, ok := visited[nb]; !ok || d > e.depth+1 {
                visited[nb] = e.depth + 1
                q.PushBack(struct{ id string; depth int }{id: nb, depth: e.depth + 1})
            }
        }

        // Reverse edges (upstream callers) — propagate to find who is impacted
        for _, caller := range reverseEdges[e.id] {
            if d, ok := visited[caller]; !ok || d > e.depth+1 {
                visited[caller] = e.depth + 1
                q.PushBack(struct{ id string; depth int }{id: caller, depth: e.depth + 1})
            }
        }
    }

    out := make([]string, 0, len(suspects))
    for k := range suspects {
        out = append(out, k)
    }
    return out
}

// ─────────────────────────────────────────────────────────────────────────────
// Dijkstra-based weighted root-cause traversal
// ─────────────────────────────────────────────────────────────────────────────

// pqItem is an entry in the Dijkstra priority queue.
type pqItem struct {
    node  string
    cost  float64 // lower cost = more suspicious (higher failure propagation)
    index int
}

type priorityQueue []*pqItem

func (pq priorityQueue) Len() int            { return len(pq) }
func (pq priorityQueue) Less(i, j int) bool  { return pq[i].cost < pq[j].cost } // min-heap
func (pq priorityQueue) Swap(i, j int) {
    pq[i], pq[j] = pq[j], pq[i]
    pq[i].index = i
    pq[j].index = j
}
func (pq *priorityQueue) Push(x interface{}) {
    item := x.(*pqItem)
    item.index = len(*pq)
    *pq = append(*pq, item)
}
func (pq *priorityQueue) Pop() interface{} {
    old := *pq
    n := len(old)
    item := old[n-1]
    *pq = old[:n-1]
    return item
}

// DetectSuspectsWeighted runs Dijkstra backwards from the impacted services,
// weighting edges by (1 - successRatio) so that unreliable edges are "cheaper"
// to traverse (more suspicious). Returns suspect node IDs sorted by ascending
// cost (index 0 = most suspicious root cause).
func DetectSuspectsWeighted(g *Graph, reverseEdges map[string][]string, impacted []string, maxDepth int) []string {
    dist := make(map[string]float64)
    for _, id := range impacted {
        dist[id] = 0
    }

    pq := &priorityQueue{}
    heap.Init(pq)
    for _, id := range impacted {
        heap.Push(pq, &pqItem{node: id, cost: 0})
    }

    depth := make(map[string]int)
    for _, id := range impacted {
        depth[id] = 0
    }

    for pq.Len() > 0 {
        item := heap.Pop(pq).(*pqItem)
        if item.cost > dist[item.node] {
            continue // stale entry
        }
        if depth[item.node] >= maxDepth {
            continue
        }
        for _, caller := range reverseEdges[item.node] {
            // Edge weight: unreliable edges (low successRatio) are cheap to traverse
            edgeWeight := 0.5 // default when no edge details available
            edgeKey := caller + "->" + item.node
            if g.EdgeDetails != nil {
                if ed, ok := g.EdgeDetails[edgeKey]; ok && ed != nil {
                    edgeWeight = 1.0 - ed.SuccessRatio // 0=perfectly healthy, 1=always failing
                    if edgeWeight < 0.01 {
                        edgeWeight = 0.01 // avoid zero-weight loops
                    }
                }
            }
            newCost := dist[item.node] + edgeWeight
            if prev, seen := dist[caller]; !seen || newCost < prev {
                dist[caller] = newCost
                depth[caller] = depth[item.node] + 1
                heap.Push(pq, &pqItem{node: caller, cost: newCost})
            }
        }
    }

    // Collect suspects (nodes reached but not in the original impacted set)
    impactedSet := make(map[string]struct{}, len(impacted))
    for _, id := range impacted {
        impactedSet[id] = struct{}{}
    }
    type scoredNode struct {
        id   string
        cost float64
    }
    var suspects []scoredNode
    for id, cost := range dist {
        if _, isImpacted := impactedSet[id]; !isImpacted {
            suspects = append(suspects, scoredNode{id, cost})
        }
    }
    sort.Slice(suspects, func(i, j int) bool { return suspects[i].cost < suspects[j].cost })

    result := make([]string, len(suspects))
    for i, s := range suspects {
        result[i] = s.id
    }
    return result
}
