package main

import (
    "net/http"
    "net/http/httptest"
    "testing"
)

func TestRecommendHandler(t *testing.T) {
    s, err := NewStoreFromFile("recommendations.json")
    if err != nil {
        t.Fatalf("failed to load recommendations: %v", err)
    }
    srv := httptest.NewServer(s.Handler())
    defer srv.Close()

    resp, err := http.Get(srv.URL + "/recommend?symptom=5xx")
    if err != nil {
        t.Fatalf("http get failed: %v", err)
    }
    if resp.StatusCode != 200 {
        t.Fatalf("expected 200, got %d", resp.StatusCode)
    }
}
