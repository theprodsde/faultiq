package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "os"
    "github.com/faultiq/graphclient"
)

func main() {
    ctx := context.Background()
    uri := os.Getenv("NEO4J_URI")
    if uri == "" {
        log.Fatal("NEO4J_URI is required in the environment")
    }
    user := os.Getenv("NEO4J_USER")
    if user == "" {
        log.Fatal("NEO4J_USER is required in the environment")
    }
    pass := os.Getenv("NEO4J_PASS")
    if pass == "" {
        log.Fatal("NEO4J_PASS is required in the environment")
    }
    ns := os.Getenv("NAMESPACE")
    if ns == "" {
        ns = "payments-platform:prod"
    }

    mgr, err := NewManager(ctx, uri, user, pass)
    if err != nil {
        log.Fatalf("graph-manager: cannot connect to neo4j: %v", err)
    }
    defer mgr.Close(ctx)

    // initial probe
    if g, err := mgr.GetSubgraph(ctx, ns); err == nil {
        fmt.Printf("subgraph %s: nodes=%d edges=%d\n", ns, len(g.Nodes), countEdges(g))
    } else {
        log.Printf("warning: initial subgraph probe failed: %v", err)
    }

    // start HTTP API (blocks)
    serveAPI(mgr)
}

// HTTP API
func serveAPI(mgr *Manager) {
    http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        _, _ = w.Write([]byte("ok"))
    })
    
    http.HandleFunc("/api/v1/graphs", func(w http.ResponseWriter, r *http.Request) {
        ns := r.URL.Query().Get("namespace")
        if ns == "" {
            http.Error(w, "namespace required", http.StatusBadRequest)
            return
        }
        ctx := r.Context()
        g, err := mgr.GetSubgraph(ctx, ns)
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(g)
    })
    
    // List all unique namespaces
    http.HandleFunc("/api/v1/graphs/namespaces", func(w http.ResponseWriter, r *http.Request) {
        ctx := r.Context()
        namespaces, err := mgr.ListNamespaces(ctx)
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(map[string]interface{}{
            "namespaces": namespaces,
            "total":      len(namespaces),
        })
    })

    // Write node endpoint
    http.HandleFunc("/api/v1/graphs/nodes", func(w http.ResponseWriter, r *http.Request) {
        if r.Method != "POST" {
            http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
            return
        }
        ns := r.URL.Query().Get("namespace")
        if ns == "" {
            http.Error(w, "namespace required", http.StatusBadRequest)
            return
        }
        var node graphclient.Node
        if err := json.NewDecoder(r.Body).Decode(&node); err != nil {
            http.Error(w, "bad request", http.StatusBadRequest)
            return
        }
        ctx := r.Context()
        if err := mgr.WriteNode(ctx, ns, &node); err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        w.WriteHeader(http.StatusCreated)
        _ = json.NewEncoder(w).Encode(map[string]string{"id": node.ID, "status": "created"})
    })
    
    // Write edge endpoint
    http.HandleFunc("/api/v1/graphs/edges", func(w http.ResponseWriter, r *http.Request) {
        if r.Method != "POST" {
            http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
            return
        }
        ns := r.URL.Query().Get("namespace")
        if ns == "" {
            http.Error(w, "namespace required", http.StatusBadRequest)
            return
        }
        var edge graphclient.Edge
        if err := json.NewDecoder(r.Body).Decode(&edge); err != nil {
            http.Error(w, "bad request", http.StatusBadRequest)
            return
        }
        ctx := r.Context()
        if err := mgr.WriteEdge(ctx, ns, &edge); err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        w.WriteHeader(http.StatusCreated)
        _ = json.NewEncoder(w).Encode(map[string]string{"from": edge.From, "to": edge.To, "status": "created"})
    })
    
    addr := os.Getenv("GRAPH_MANAGER_ADDR")
    if addr == "" {
        addr = ":8086"
    }
    log.Printf("graph-manager: serving on %s", addr)
    log.Fatal(http.ListenAndServe(addr, nil))
}

func countEdges(g *graphclient.Graph) int {
    c := 0
    for _, arr := range g.Edges {
        c += len(arr)
    }
    return c
}
