package main

import (
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
    "time"

    "github.com/faultiq/graphclient"
)

func TestHTTPGraphProvider_GetSubgraph(t *testing.T) {
    // prepare a test graph (use graphclient.Graph shape)
    g := &graphclient.Graph{
        Nodes: map[string]*graphclient.Node{"n1": {ID: "n1", StatusClass: "5xx"}},
        Edges: map[string][]string{"n1": {}},
    }
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // return JSON graph regardless of namespace
        _ = json.NewEncoder(w).Encode(g)
    }))
    defer srv.Close()

    gp := graphclient.NewHTTPProvider(srv.URL)
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancel()
    got, err := gp.GetSubgraph(ctx, "any:ns")
    if err != nil {
        t.Fatalf("GetSubgraph failed: %v", err)
    }
    if got == nil || got.Nodes == nil || got.Nodes["n1"].StatusClass != "5xx" {
        t.Fatalf("unexpected graph returned: %+v", got)
    }
}
