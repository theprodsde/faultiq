package main

import (
    "context"
    "net/http"
    "net/http/httptest"
    "testing"
)

func TestRequireRole_AllowsSuperAdmin(t *testing.T) {
    handler := RequireRole("tenant_admin", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
    }))
    req := httptest.NewRequest("GET", "/", nil)
    // inject claims with super_admin
    ctx := req.Context()
    ctx = contextWithClaims(ctx, map[string]interface{}{"realm_access": map[string]interface{}{"roles": []interface{}{"super_admin"}}})
    req = req.WithContext(ctx)
    w := httptest.NewRecorder()
    handler.ServeHTTP(w, req)
    if w.Result().StatusCode != http.StatusOK {
        t.Fatalf("expected 200 OK, got %d", w.Result().StatusCode)
    }
}

func TestRequireRole_DeniesMissingRole(t *testing.T) {
    handler := RequireRole("tenant_admin", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
    }))
    req := httptest.NewRequest("GET", "/", nil)
    ctx := req.Context()
    ctx = contextWithClaims(ctx, map[string]interface{}{"realm_access": map[string]interface{}{"roles": []interface{}{"viewer"}}})
    req = req.WithContext(ctx)
    w := httptest.NewRecorder()
    handler.ServeHTTP(w, req)
    if w.Result().StatusCode != http.StatusForbidden {
        t.Fatalf("expected 403 Forbidden, got %d", w.Result().StatusCode)
    }
}

func TestRequireTenant_AllowsMatchingTenant(t *testing.T) {
    handler := RequireTenant("tenant-a", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
    }))
    req := httptest.NewRequest("GET", "/", nil)
    ctx := req.Context()
    ctx = contextWithClaims(ctx, map[string]interface{}{"tenant": "tenant-a"})
    req = req.WithContext(ctx)
    w := httptest.NewRecorder()
    handler.ServeHTTP(w, req)
    if w.Result().StatusCode != http.StatusOK {
        t.Fatalf("expected 200 OK, got %d", w.Result().StatusCode)
    }
}

func TestRequireTenant_DeniesMismatch(t *testing.T) {
    handler := RequireTenant("tenant-a", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
    }))
    req := httptest.NewRequest("GET", "/", nil)
    ctx := req.Context()
    ctx = contextWithClaims(ctx, map[string]interface{}{"tenant": "tenant-b"})
    req = req.WithContext(ctx)
    w := httptest.NewRecorder()
    handler.ServeHTTP(w, req)
    if w.Result().StatusCode != http.StatusForbidden {
        t.Fatalf("expected 403 Forbidden, got %d", w.Result().StatusCode)
    }
}

// helper to attach claims to context
func contextWithClaims(ctx context.Context, claims map[string]interface{}) context.Context {
    return context.WithValue(ctx, ctxUserKey, claims)
}
