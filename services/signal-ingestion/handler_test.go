package main

import (
    "bytes"
    "context"
    "net/http"
    "net/http/httptest"
    "testing"
)

type mockPublisher struct{
    last []byte
}

func (m *mockPublisher) Publish(ctx context.Context, channel string, payload []byte) error {
    m.last = append([]byte{}, payload...)
    return nil
}

func TestHandleSignals_AcceptsValid(t *testing.T) {
    m := &mockPublisher{}
    srv := NewServer(m)

    body := []byte(`{"tenant":"t","project":"p","environment":"e","service":"s","metric":"m","value":1.2}`)
    req := httptest.NewRequest(http.MethodPost, "/api/v1/signals", bytes.NewReader(body))
    w := httptest.NewRecorder()
    srv.HandleSignals(w, req)
    if w.Result().StatusCode != http.StatusAccepted {
        t.Fatalf("expected 202 accepted, got %d", w.Result().StatusCode)
    }
    if len(m.last) == 0 {
        t.Fatalf("expected publisher to receive payload")
    }
}

func TestHandleSignals_RateLimit(t *testing.T) {
    m := &mockPublisher{}
    srv := NewServer(m)
    // exhaust limiter — include required tenant/project fields
    validBody := []byte(`{"tenant":"t","project":"p","service":"s"}`)
    for i := 0; i < 20; i++ {
        req := httptest.NewRequest(http.MethodPost, "/api/v1/signals", bytes.NewReader(validBody))
        w := httptest.NewRecorder()
        srv.HandleSignals(w, req)
        if i == 0 && w.Result().StatusCode != http.StatusAccepted {
            t.Fatalf("first request should be accepted, got %d", w.Result().StatusCode)
        }
    }
    // one more should hit rate limit
    req := httptest.NewRequest(http.MethodPost, "/api/v1/signals", bytes.NewReader(validBody))
    w := httptest.NewRecorder()
    srv.HandleSignals(w, req)
    if w.Result().StatusCode != http.StatusTooManyRequests {
        t.Fatalf("expected rate limit exceeded, got %d", w.Result().StatusCode)
    }
}
