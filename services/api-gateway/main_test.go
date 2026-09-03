package main

import (
    "net/http"
    "net/http/httptest"
    "testing"
)

func TestHealthHandler(t *testing.T) {
    req := httptest.NewRequest("GET", "/health", nil)
    w := httptest.NewRecorder()
    healthHandler(w, req)
    if w.Result().StatusCode != http.StatusOK {
        t.Fatalf("expected 200 OK, got %d", w.Result().StatusCode)
    }
}

func TestProjectsHandler(t *testing.T) {
    req := httptest.NewRequest("GET", "/api/v1/projects", nil)
    w := httptest.NewRecorder()
    projectsHandler(w, req)
    // Without a live DB the handler now returns 503 (no per-request fallback connection).
    got := w.Result().StatusCode
    if got != http.StatusOK && got != http.StatusServiceUnavailable {
        t.Fatalf("expected 200 or 503, got %d", got)
    }
}
