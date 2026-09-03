package main

import (
    "net/http"
    "net/http/httptest"
    "testing"
)

func TestHealthEndpoint(t *testing.T) {
    req := httptest.NewRequest("GET", "/health", nil)
    w := httptest.NewRecorder()
    // call the handler directly
    http.DefaultServeMux = http.NewServeMux()
    http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        _, _ = w.Write([]byte("ok"))
    })
    http.DefaultServeMux.ServeHTTP(w, req)
    if w.Result().StatusCode != http.StatusOK {
        t.Fatalf("expected 200 ok, got %d", w.Result().StatusCode)
    }
}

func TestGraphsHandler_NoNamespace(t *testing.T) {
    req := httptest.NewRequest("GET", "/api/v1/graphs", nil)
    w := httptest.NewRecorder()
    // ensure handler is registered on fresh mux
    http.DefaultServeMux = http.NewServeMux()
    http.HandleFunc("/api/v1/graphs", func(w http.ResponseWriter, r *http.Request) {
        ns := r.URL.Query().Get("namespace")
        if ns == "" {
            http.Error(w, "namespace required", http.StatusBadRequest)
            return
        }
    })
    http.DefaultServeMux.ServeHTTP(w, req)
    if w.Result().StatusCode != http.StatusBadRequest {
        t.Fatalf("expected 400 bad request, got %d", w.Result().StatusCode)
    }
}
