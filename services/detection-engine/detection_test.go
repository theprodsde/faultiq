package main

import (
    "reflect"
    "testing"
)

func TestDetectSuspects_Simple(t *testing.T) {
    g := &Graph{
        Nodes: map[string]*Node{
            "svc-a": {ID: "svc-a", StatusClass: "2xx"},
            "svc-b": {ID: "svc-b", StatusClass: "5xx"},
            "svc-c": {ID: "svc-c", StatusClass: "2xx"},
        },
        Edges: map[string][]string{
            "svc-a": {"svc-b", "svc-c"},
            "svc-b": {"svc-c"},
        },
    }

    suspects := DetectSuspects(g, []string{"svc-a"}, 3)
    found := map[string]bool{}
    for _, s := range suspects { found[s] = true }

    if !found["svc-b"] {
        t.Fatalf("expected svc-b to be a suspect, got %v", suspects)
    }
}
func TestDetectSuspects_PrunesHealthy(t *testing.T) {
    g := &Graph{
        Nodes: map[string]*Node{
            "gw": {ID: "gw", StatusClass: "2xx"},
            "a":  {ID: "a", StatusClass: "5xx"},
            "b":  {ID: "b", StatusClass: "2xx"},
            "c":  {ID: "c", StatusClass: "timeout"},
        },
        Edges: map[string][]string{
            "gw": {"a", "b"},
            "a":  {"c"},
            "b":  {"c"},
        },
    }

    suspects := DetectSuspects(g, []string{"gw"}, 5)
    // Should detect 'a' and 'c' as suspects, but 'b' is healthy and pruned
    want := []string{"a", "c"}
    // convert to map for comparison
    gotMap := make(map[string]bool)
    for _, s := range suspects {
        gotMap[s] = true
    }
    for _, w := range want {
        if !gotMap[w] {
            t.Fatalf("expected suspect %s in result %v", w, suspects)
        }
    }
    // ensure b not in suspects
    if gotMap["b"] {
        t.Fatalf("did not expect 'b' to be suspect")
    }
    // basic deterministic check length >= 2
    if len(suspects) < 2 {
        t.Fatalf("expected at least 2 suspects, got %v", suspects)
    }
    // sanity: ensure reflect doesn't panic
    _ = reflect.DeepEqual(suspects, want)
}
