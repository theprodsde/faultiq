// seed-demo.go
// =============================================================================
// FaultIQ Demo Seeder (API-driven)
// =============================================================================
// Seeds a complete demo tenant with a realistic payments-platform graph,
// live signals, and a full incident analysis run.
//
// Usage:
//   go run scripts/seed-demo.go
//   go run scripts/seed-demo.go --reset       — wipe and re-seed
//   go run scripts/seed-demo.go --env prod     — seed prod environment
//
// Prerequisites:
//   - docker compose up -d (all services healthy)
//   - API Gateway running on localhost:8080
//   - Keycloak running on localhost:8180
// =============================================================================

package main

import (
    "bytes"
    "context"
    "encoding/json"
    "flag"
    "fmt"
    "io"
    "log"
    "net/http"
    "net/url"
    "os"
    "strings"
    "time"

    "github.com/jackc/pgx/v5"
)

// =============================================================================
// Config
// =============================================================================

const (
    apiBase      = "http://localhost:8080"
    keycloakBase = "http://localhost:8081"
    realm        = "faultiq"
    clientID     = "faultiq-ui"
    adminUser    = "super"
    adminPass    = "superpass"
)

var (
    flagReset = flag.Bool("reset", false, "wipe existing demo data before seeding")
    flagEnv   = flag.String("env", "prod", "target environment to seed")
)

// Demo nodes/edges/signals (payments platform)
var demoNodes = []map[string]interface{}{
    {"id":"svc_api_gateway","name":"api-gateway","type":"GATEWAY","tags":[]string{"entry-point","public"},"slaThresholdMs":200},
    {"id":"svc_payment_api","name":"payment-api","type":"SERVICE","tags":[]string{"core","financial"},"slaThresholdMs":500},
    {"id":"svc_auth_service","name":"auth-service","type":"SERVICE","tags":[]string{"auth","shared"},"slaThresholdMs":100},
    {"id":"svc_ledger_service","name":"ledger-service","type":"SERVICE","tags":[]string{"financial","core"},"slaThresholdMs":500},
    {"id":"svc_settlement_worker","name":"settlement-worker","type":"SERVICE","tags":[]string{"async","financial"},"slaThresholdMs":2000},
    {"id":"svc_notification_service","name":"notification-service","type":"SERVICE","tags":[]string{"async","non-critical"},"slaThresholdMs":1000},
    {"id":"db_postgres","name":"postgres-db","type":"DATABASE","tags":[]string{"primary-db","shared"},"slaThresholdMs":50},
    {"id":"db_redis_cache","name":"redis-cache","type":"DATABASE","tags":[]string{"cache"},"slaThresholdMs":10},
    {"id":"queue_ledger_events","name":"ledger-events-topic","type":"QUEUE","tags":[]string{"events"},"slaThresholdMs":100},
    {"id":"ext_stripe","name":"stripe-api","type":"EXTERNAL","tags":[]string{"payment-gateway","external"},"slaThresholdMs":2000},
}

var demoEdges = []map[string]interface{}{
    {"from":"svc_api_gateway","to":"svc_payment_api","type":"CALLS","confidence":0.99},
    {"from":"svc_api_gateway","to":"svc_auth_service","type":"CALLS","confidence":0.99},
    {"from":"svc_payment_api","to":"svc_auth_service","type":"CALLS","confidence":0.95},
    {"from":"svc_payment_api","to":"svc_ledger_service","type":"CALLS","confidence":0.95},
    {"from":"svc_payment_api","to":"ext_stripe","type":"CALLS","confidence":0.9},
    {"from":"svc_payment_api","to":"db_redis_cache","type":"DEPENDS_ON","confidence":0.95},
    {"from":"svc_ledger_service","to":"db_postgres","type":"DEPENDS_ON","confidence":0.99},
    {"from":"svc_ledger_service","to":"queue_ledger_events","type":"PUBLISHES_TO","confidence":0.9},
    {"from":"svc_settlement_worker","to":"queue_ledger_events","type":"CONSUMES_FROM","confidence":0.9},
    {"from":"svc_settlement_worker","to":"svc_ledger_service","type":"CALLS","confidence":0.85},
    {"from":"svc_auth_service","to":"db_redis_cache","type":"DEPENDS_ON","confidence":0.95},
}

var demoSignals = []map[string]interface{}{
    {"service":"ledger-service","statusClass":"timeout","errorRate":0.42,"latencyP95":4800,"timeoutCount":18},
    {"service":"payment-api","statusClass":"5xx","errorRate":0.18,"latencyP95":1200},
    {"service":"settlement-worker","statusClass":"5xx","errorRate":0.31,"latencyP95":3200},
    {"service":"auth-service","statusClass":"2xx","errorRate":0.001,"latencyP95":38},
    {"service":"api-gateway","statusClass":"2xx","errorRate":0.002,"latencyP95":55},
}

// =============================================================================
// Main
// =============================================================================

func main() {
    flag.Parse()
    env := *flagEnv

    log.Println("FaultIQ API-driven Demo Seeder")

    c := newClient()

    log.Println("→ Authenticating with Keycloak...")
    token := c.mustGetToken()
    log.Println("✓ Token acquired")

    log.Println("→ Creating tenant: acme-corp...")
    tenantID := c.mustCreateTenant(token)
    log.Printf("✓ Tenant created: %s\n", tenantID)

    log.Println("→ Creating project: payments-platform...")
    projectID := c.mustCreateProject(token, tenantID)
    log.Printf("✓ Project created: %s\n", projectID)

    log.Printf("→ Adding %d nodes to %s graph...\n", len(demoNodes), env)
    for _, node := range demoNodes {
        c.mustAddNode(token, projectID, env, node)
        log.Printf("  ✓ Node: %s (%v)\n", node["name"], node["type"])
    }

    log.Printf("→ Adding %d edges...\n", len(demoEdges))
    for _, edge := range demoEdges {
        c.mustAddEdge(token, projectID, env, edge)
        log.Printf("  ✓ Edge: %s -> %s\n", edge["from"], edge["to"])
    }

    log.Println("→ Publishing graph version 1...")
    c.mustPublishGraph(token, projectID, env)
    log.Println("✓ Graph published")

    log.Println("→ Submitting fault signals...")
    c.mustSubmitSignals(token, tenantID, projectID, env)
    log.Println("✓ Signals submitted")

    log.Println("→ Waiting 3s for detection engine...")
    time.Sleep(3 * time.Second)

    log.Println("→ Triggering incident analysis...")
    incidentID := c.mustAnalyze(token, projectID, env)
    log.Printf("✓ Incident created: %s\n", incidentID)

    log.Println("→ Fetching incident result...")
    result := c.mustGetIncident(token, incidentID)
    printResult(result)

    log.Println("Seed complete")
}

// =============================================================================
// Client
// =============================================================================

type client struct{ http *http.Client }

func newClient() *client { return &client{http: &http.Client{Timeout: 15 * time.Second}} }

func (c *client) mustGetToken() string {
    form := url.Values{}
    form.Set("grant_type", "password")
    form.Set("client_id", clientID)
    form.Set("username", adminUser)
    form.Set("password", adminPass)

    resp := c.mustPost(fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token", keycloakBase, realm), "application/x-www-form-urlencoded", strings.NewReader(form.Encode()), "")
    var result map[string]interface{}
    mustDecode(resp, &result)
    token, ok := result["access_token"].(string)
    if !ok { log.Fatal("failed to parse access_token") }
    return token
}

func (c *client) mustCreateTenant(token string) string {
    // Try API first (some deployments implement tenant creation)
    body := map[string]interface{}{"name":"Acme Corp","slug":"acme-corp","plan":"pro"}
    resp := c.mustPostJSON(apiBase+"/api/v1/tenants", body, token)
    if resp.StatusCode == http.StatusNotFound {
        // Fallback: insert directly into Postgres
        dsn := os.Getenv("PG_DSN")
        if dsn == "" {
            dsn = "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"
        }
        ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
        defer cancel()
        conn, err := pgx.Connect(ctx, dsn)
        if err != nil {
            log.Fatalf("db connect failed: %v", err)
        }
        defer conn.Close(ctx)
        id := fmt.Sprintf("t_%d", time.Now().UnixNano())
        if _, err := conn.Exec(ctx, `INSERT INTO tenants (id, name, slug) VALUES ($1,$2,$3)`, id, "Acme Corp", "acme-corp"); err != nil {
            log.Fatalf("failed to insert tenant: %v", err)
        }
        return id
    }
    var result map[string]interface{}
    mustDecode(resp, &result)
    return result["id"].(string)
}

func (c *client) mustCreateProject(token, tenantID string) string {
    // Try API create; if 404, insert into Postgres directly
    body := map[string]interface{}{"name":"Payments Platform","slug":"payments-platform","domain":"payments","environments":[]string{"dev","stage","prod"}}
    resp := c.mustPostJSON(fmt.Sprintf("%s/api/v1/tenants/%s/projects", apiBase, tenantID), body, token)
    if resp.StatusCode == http.StatusNotFound {
        dsn := os.Getenv("PG_DSN")
        if dsn == "" {
            dsn = "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"
        }
        ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
        defer cancel()
        conn, err := pgx.Connect(ctx, dsn)
        if err != nil { log.Fatalf("db connect failed: %v", err) }
        defer conn.Close(ctx)
        id := fmt.Sprintf("p_%d", time.Now().UnixNano())
        if _, err := conn.Exec(ctx, `INSERT INTO projects (id, tenant_id, name, slug, environment) VALUES ($1,$2,$3,$4,$5)`, id, tenantID, "Payments Platform", "payments-platform", ""); err != nil {
            log.Fatalf("failed to insert project: %v", err)
        }
        return id
    }
    var result map[string]interface{}
    mustDecode(resp, &result)
    return result["id"].(string)
}

func (c *client) mustAddNode(token, projectID, env string, node map[string]interface{}) {
    body := map[string]interface{}{"environment":env,"node":node}
    resp := c.mustPutJSON(fmt.Sprintf("%s/api/v1/projects/%s/graphs/nodes", apiBase, projectID), body, token)
    resp.Body.Close()
}

func (c *client) mustAddEdge(token, projectID, env string, edge map[string]interface{}) {
    body := map[string]interface{}{"environment":env,"edge":edge}
    resp := c.mustPutJSON(fmt.Sprintf("%s/api/v1/projects/%s/graphs/edges", apiBase, projectID), body, token)
    resp.Body.Close()
}

func (c *client) mustPublishGraph(token, projectID, env string) {
    body := map[string]interface{}{"environment":env,"notes":"Demo seed"}
    resp := c.mustPostJSON(fmt.Sprintf("%s/api/v1/projects/%s/graphs/publish", apiBase, projectID), body, token)
    resp.Body.Close()
}

func (c *client) mustSubmitSignals(token, tenantID, projectID, env string) {
    signals := make([]map[string]interface{}, len(demoSignals))
    for i, s := range demoSignals {
        sig := map[string]interface{}{"service":s["service"],"statusClass":s["statusClass"],"errorRate":s["errorRate"],"latencyP95":s["latencyP95"],"timestamp":time.Now().UTC().Format(time.RFC3339)}
        if tc, ok := s["timeoutCount"]; ok { sig["timeoutCount"] = tc }
        signals[i] = sig
    }
    body := map[string]interface{}{"tenantId":tenantID,"projectId":projectID,"environment":env,"signals":signals,"source":"seed-demo"}
    resp := c.mustPostJSON(apiBase+"/api/v1/signals", body, token)
    resp.Body.Close()
}

func (c *client) mustAnalyze(token, projectID, env string) string {
    body := map[string]interface{}{"environment":env,"entryService":"api-gateway","timeWindowMin":5,"signals":demoSignals}
    resp := c.mustPostJSON(fmt.Sprintf("%s/api/v1/projects/%s/incidents/analyze", apiBase, projectID), body, token)
    var result map[string]interface{}
    mustDecode(resp, &result)
    return result["incidentId"].(string)
}

func (c *client) mustGetIncident(token, incidentID string) map[string]interface{} {
    req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/api/v1/incidents/%s", apiBase, incidentID), nil)
    req.Header.Set("Authorization", "Bearer "+token)
    resp, err := c.http.Do(req)
    if err != nil { log.Fatalf("GET incident failed: %v", err) }
    var result map[string]interface{}
    mustDecode(resp, &result)
    return result
}

// HTTP helpers
func (c *client) mustPostJSON(url string, body interface{}, token string) *http.Response {
    b, _ := json.Marshal(body)
    return c.mustPost(url, "application/json", bytes.NewReader(b), token)
}

func (c *client) mustPutJSON(rawURL string, body interface{}, token string) *http.Response {
    b, _ := json.Marshal(body)
    req, _ := http.NewRequest(http.MethodPut, rawURL, bytes.NewReader(b))
    req.Header.Set("Content-Type", "application/json")
    if token != "" { req.Header.Set("Authorization", "Bearer "+token) }
    resp, err := c.http.Do(req)
    if err != nil { log.Fatalf("PUT %s failed: %v", rawURL, err) }
    if resp.StatusCode >= 400 { b, _ := io.ReadAll(resp.Body); log.Fatalf("PUT %s returned %d: %s", rawURL, resp.StatusCode, string(b)) }
    return resp
}

func (c *client) mustPost(rawURL, contentType string, body io.Reader, token string) *http.Response {
    req, _ := http.NewRequest(http.MethodPost, rawURL, body)
    req.Header.Set("Content-Type", contentType)
    if token != "" { req.Header.Set("Authorization", "Bearer "+token) }
    resp, err := c.http.Do(req)
    if err != nil { log.Fatalf("POST %s failed: %v", rawURL, err) }
    if resp.StatusCode >= 400 { b, _ := io.ReadAll(resp.Body); log.Fatalf("POST %s returned %d: %s", rawURL, resp.StatusCode, string(b)) }
    return resp
}

func mustDecode(resp *http.Response, v interface{}) {
    defer resp.Body.Close()
    if err := json.NewDecoder(resp.Body).Decode(v); err != nil { log.Fatalf("Failed to decode response: %v", err) }
}

// Result printer
func printResult(result map[string]interface{}) {
    fmt.Println("INCIDENT ANALYSIS RESULT")
    if stats, ok := result["traversalStats"].(map[string]interface{}); ok {
        fmt.Printf("Nodes visited : %.0f\n", stats["nodesVisited"])
        fmt.Printf("Paths pruned  : %.0f\n", stats["pathsPruned"])
    }
    if candidates, ok := result["rootCauseCandidates"].([]interface{}); ok {
        for _, raw := range candidates {
            c := raw.(map[string]interface{})
            fmt.Printf("- %s (confidence: %.2f)\n", c["name"], c["confidence"])
        }
    }
}
