// seed-demo-comprehensive.go
// =============================================================================
// FaultIQ Comprehensive Demo Seeder with Error Injection
// =============================================================================
// Seeds realistic multi-tenant, multi-project scenario with:
// - 2 products (Payments Platform, Orders Platform)
// - 15+ services with realistic dependencies
// - Database, cache, queue, and external dependencies
// - Demo incident scenarios that trigger RCA
// - Error injection for live troubleshooting demo
//
// Usage:
//   go run scripts/seed-demo-comprehensive.go
//   go run scripts/seed-demo-comprehensive.go --reset (wipe + reseed)
//   go run scripts/seed-demo-comprehensive.go --demo-errors (inject failures)
//
// Prerequisites:
//   - docker compose up -d
//   - All services running
// =============================================================================

package main

import (
    "bytes"
    "encoding/json"
    "flag"
    "fmt"
    "io"
    "log"
    "math/rand"
    "net/http"
    "time"
)

const (
    apiBase      = "http://localhost:8080"
    graphMgrBase = "http://localhost:8086"
    keycloakBase = "http://localhost:8081"
    realm        = "faultiq"
    clientID     = "faultiq-ui"
    adminUser    = "super"
    adminPass    = "superpass"
)

var (
    flagReset       = flag.Bool("reset", false, "wipe and reseed")
    flagDemoErrors  = flag.Bool("demo-errors", false, "inject error scenarios")
    flagVerbose     = flag.Bool("v", false, "verbose output")
)

// ============================================================================
// Complex Demo Graph: Payments Platform (Production)
// ============================================================================

var paymentsNodes = []map[string]interface{}{
    // Gateway Layer
    {"id": "svc_api_gateway", "name": "API Gateway", "type": "GATEWAY", "tags": []string{"entry-point", "public"}, "statusClass": "2xx", "latencyP95": 45, "errorRate": 0.001},
    
    // Core Services
    {"id": "svc_payment_api", "name": "Payment API", "type": "SERVICE", "tags": []string{"core", "financial", "v2-stable"}, "statusClass": "2xx", "latencyP95": 120, "errorRate": 0.002},
    {"id": "svc_ledger_service", "name": "Ledger Service", "type": "SERVICE", "tags": []string{"financial", "core", "mission-critical"}, "statusClass": "2xx", "latencyP95": 95, "errorRate": 0.001},
    {"id": "svc_auth_service", "name": "Auth Service", "type": "SERVICE", "tags": []string{"auth", "shared", "critical"}, "statusClass": "2xx", "latencyP95": 35, "errorRate": 0.0005},
    
    // Async Workers
    {"id": "svc_settlement_worker", "name": "Settlement Worker", "type": "SERVICE", "tags": []string{"async", "batch", "financial"}, "statusClass": "2xx", "latencyP95": 500, "errorRate": 0.005},
    {"id": "svc_notification_service", "name": "Notification Service", "type": "SERVICE", "tags": []string{"async", "non-critical", "email"}, "statusClass": "2xx", "latencyP95": 200, "errorRate": 0.01},
    {"id": "svc_reconciliation_worker", "name": "Reconciliation Worker", "type": "SERVICE", "tags": []string{"batch", "critical"}, "statusClass": "2xx", "latencyP95": 800, "errorRate": 0.002},
    
    // Risk & Compliance
    {"id": "svc_fraud_detector", "name": "Fraud Detector", "type": "SERVICE", "tags": []string{"ml", "realtime", "critical"}, "statusClass": "2xx", "latencyP95": 280, "errorRate": 0.003},
    {"id": "svc_compliance_check", "name": "Compliance Checker", "type": "SERVICE", "tags": []string{"regulatory", "critical"}, "statusClass": "2xx", "latencyP95": 150, "errorRate": 0.001},
    
    // Databases
    {"id": "db_payments_primary", "name": "Payments DB (Primary)", "type": "DATABASE", "tags": []string{"postgres", "primary", "critical"}, "statusClass": "2xx", "latencyP95": 15, "errorRate": 0.0001},
    {"id": "db_payments_replica", "name": "Payments DB (Replica)", "type": "DATABASE", "tags": []string{"postgres", "read-replica"}, "statusClass": "2xx", "latencyP95": 25, "errorRate": 0.0005},
    {"id": "db_cache_redis", "name": "Redis Cache", "type": "DATABASE", "tags": []string{"cache", "in-memory", "critical"}, "statusClass": "2xx", "latencyP95": 3, "errorRate": 0.0001},
    
    // Queues
    {"id": "queue_settlement_events", "name": "Settlement Events Queue", "type": "QUEUE", "tags": []string{"kafka", "events", "critical"}, "statusClass": "2xx", "latencyP95": 50, "errorRate": 0.001},
    {"id": "queue_notifications", "name": "Notifications Queue", "type": "QUEUE", "tags": []string{"kafka", "async", "non-critical"}, "statusClass": "2xx", "latencyP95": 40, "errorRate": 0.002},
    
    // External
    {"id": "ext_stripe_api", "name": "Stripe API", "type": "EXTERNAL", "tags": []string{"payment-gateway", "external", "sla-99.9"}, "statusClass": "2xx", "latencyP95": 450, "errorRate": 0.0002},
    {"id": "ext_bank_api", "name": "Bank API", "type": "EXTERNAL", "tags": []string{"banking", "external", "critical"}, "statusClass": "2xx", "latencyP95": 1200, "errorRate": 0.001},
}

var paymentsEdges = []map[string]interface{}{
    // Gateway → Core APIs
    {"from": "svc_api_gateway", "to": "svc_payment_api", "type": "CALLS", "successRatio": 0.999, "confidence": 0.99},
    {"from": "svc_api_gateway", "to": "svc_auth_service", "type": "CALLS", "successRatio": 0.9995, "confidence": 0.99},
    
    // Payment API Dependencies
    {"from": "svc_payment_api", "to": "svc_auth_service", "type": "CALLS", "successRatio": 0.998, "confidence": 0.95},
    {"from": "svc_payment_api", "to": "svc_ledger_service", "type": "CALLS", "successRatio": 0.997, "confidence": 0.95},
    {"from": "svc_payment_api", "to": "svc_fraud_detector", "type": "CALLS", "successRatio": 0.995, "confidence": 0.90},
    {"from": "svc_payment_api", "to": "svc_compliance_check", "type": "CALLS", "successRatio": 0.99, "confidence": 0.90},
    {"from": "svc_payment_api", "to": "ext_stripe_api", "type": "CALLS", "successRatio": 0.998, "confidence": 0.85},
    {"from": "svc_payment_api", "to": "db_cache_redis", "type": "READS", "successRatio": 0.999, "confidence": 0.95},
    
    // Ledger Service Dependencies
    {"from": "svc_ledger_service", "to": "db_payments_primary", "type": "WRITES", "successRatio": 0.9995, "confidence": 0.99},
    {"from": "svc_ledger_service", "to": "db_payments_replica", "type": "READS", "successRatio": 0.998, "confidence": 0.95},
    {"from": "svc_ledger_service", "to": "queue_settlement_events", "type": "PUBLISHES", "successRatio": 0.997, "confidence": 0.90},
    
    // Auth Service Dependencies
    {"from": "svc_auth_service", "to": "db_cache_redis", "type": "READS", "successRatio": 0.9999, "confidence": 0.99},
    {"from": "svc_auth_service", "to": "db_payments_replica", "type": "READS", "successRatio": 0.998, "confidence": 0.95},
    
    // Async Workers
    {"from": "svc_settlement_worker", "to": "queue_settlement_events", "type": "CONSUMES", "successRatio": 0.995, "confidence": 0.90},
    {"from": "svc_settlement_worker", "to": "svc_ledger_service", "type": "CALLS", "successRatio": 0.99, "confidence": 0.85},
    {"from": "svc_settlement_worker", "to": "ext_bank_api", "type": "CALLS", "successRatio": 0.97, "confidence": 0.80},
    
    {"from": "svc_notification_service", "to": "queue_notifications", "type": "CONSUMES", "successRatio": 0.98, "confidence": 0.85},
    
    {"from": "svc_reconciliation_worker", "to": "db_payments_primary", "type": "READS", "successRatio": 0.9995, "confidence": 0.99},
    {"from": "svc_reconciliation_worker", "to": "svc_ledger_service", "type": "CALLS", "successRatio": 0.99, "confidence": 0.90},
    
    // Fraud Detector
    {"from": "svc_fraud_detector", "to": "db_cache_redis", "type": "READS", "successRatio": 0.998, "confidence": 0.95},
    
    // Compliance
    {"from": "svc_compliance_check", "to": "db_payments_primary", "type": "READS", "successRatio": 0.9995, "confidence": 0.99},
}

// ============================================================================
// Complex Demo Graph: Orders Platform (Production)
// ============================================================================

var ordersNodes = []map[string]interface{}{
    {"id": "svc_order_api", "name": "Order API", "type": "SERVICE", "tags": []string{"core", "order-processing"}, "statusClass": "2xx", "latencyP95": 130, "errorRate": 0.002},
    {"id": "svc_inventory_service", "name": "Inventory Service", "type": "SERVICE", "tags": []string{"core", "stock-management"}, "statusClass": "2xx", "latencyP95": 90, "errorRate": 0.005},
    {"id": "svc_warehouse_service", "name": "Warehouse Service", "type": "SERVICE", "tags": []string{"fulfillment", "logistics"}, "statusClass": "2xx", "latencyP95": 200, "errorRate": 0.008},
    {"id": "svc_notification_svc", "name": "Notifications", "type": "SERVICE", "tags": []string{"async", "alerts"}, "statusClass": "2xx", "latencyP95": 180, "errorRate": 0.01},
    {"id": "db_orders", "name": "Orders DB", "type": "DATABASE", "tags": []string{"postgres", "primary"}, "statusClass": "2xx", "latencyP95": 20, "errorRate": 0.0001},
    {"id": "db_inventory", "name": "Inventory DB", "type": "DATABASE", "tags": []string{"postgres", "primary"}, "statusClass": "2xx", "latencyP95": 18, "errorRate": 0.0002},
    {"id": "queue_orders", "name": "Orders Queue", "type": "QUEUE", "tags": []string{"kafka"}, "statusClass": "2xx", "latencyP95": 35, "errorRate": 0.001},
}

var ordersEdges = []map[string]interface{}{
    {"from": "svc_order_api", "to": "svc_inventory_service", "type": "CALLS", "successRatio": 0.99, "confidence": 0.95},
    {"from": "svc_order_api", "to": "db_orders", "type": "WRITES", "successRatio": 0.9995, "confidence": 0.99},
    {"from": "svc_inventory_service", "to": "db_inventory", "type": "READS", "successRatio": 0.998, "confidence": 0.99},
    {"from": "svc_inventory_service", "to": "svc_warehouse_service", "type": "CALLS", "successRatio": 0.98, "confidence": 0.90},
    {"from": "svc_warehouse_service", "to": "db_inventory", "type": "WRITES", "successRatio": 0.997, "confidence": 0.95},
    {"from": "svc_warehouse_service", "to": "queue_orders", "type": "PUBLISHES", "successRatio": 0.99, "confidence": 0.90},
    {"from": "svc_notification_svc", "to": "queue_orders", "type": "CONSUMES", "successRatio": 0.97, "confidence": 0.85},
}

// ============================================================================
// Analytics Dashboard Project
// ============================================================================

var analyticsNodes = []map[string]interface{}{
    // Data Ingestion Pipeline
    {"id": "svc_log_collector", "name": "Log Collector", "type": "SERVICE", "tags": []string{"data-pipeline", "ingestion", "critical"}, "statusClass": "2xx", "latencyP95": 50, "errorRate": 0.001},
    {"id": "svc_metrics_aggregator", "name": "Metrics Aggregator", "type": "SERVICE", "tags": []string{"data-pipeline", "aggregation", "critical"}, "statusClass": "2xx", "latencyP95": 80, "errorRate": 0.002},
    {"id": "svc_trace_processor", "name": "Trace Processor", "type": "SERVICE", "tags": []string{"data-pipeline", "tracing"}, "statusClass": "2xx", "latencyP95": 120, "errorRate": 0.003},
    
    // Query & Analytics
    {"id": "svc_query_engine", "name": "Query Engine", "type": "SERVICE", "tags": []string{"analytics", "core"}, "statusClass": "2xx", "latencyP95": 300, "errorRate": 0.001},
    {"id": "svc_dashboard_api", "name": "Dashboard API", "type": "SERVICE", "tags": []string{"api", "frontend-facing"}, "statusClass": "2xx", "latencyP95": 150, "errorRate": 0.002},
    
    // Storage
    {"id": "db_timeseries", "name": "TimeSeries DB", "type": "DATABASE", "tags": []string{"influxdb", "metrics", "critical"}, "statusClass": "2xx", "latencyP95": 25, "errorRate": 0.0001},
    {"id": "db_logs", "name": "Log Storage", "type": "DATABASE", "tags": []string{"elasticsearch", "logs"}, "statusClass": "2xx", "latencyP95": 100, "errorRate": 0.0005},
    {"id": "db_analytics", "name": "Analytics DB", "type": "DATABASE", "tags": []string{"postgres", "analytics"}, "statusClass": "2xx", "latencyP95": 35, "errorRate": 0.0002},
    
    // Cache
    {"id": "cache_redis_analytics", "name": "Analytics Cache", "type": "DATABASE", "tags": []string{"redis", "cache"}, "statusClass": "2xx", "latencyP95": 5, "errorRate": 0.0001},
}

var analyticsEdges = []map[string]interface{}{
    // Ingestion Pipeline
    {"from": "svc_log_collector", "to": "db_logs", "type": "WRITES", "successRatio": 0.998, "confidence": 0.95},
    {"from": "svc_metrics_aggregator", "to": "db_timeseries", "type": "WRITES", "successRatio": 0.999, "confidence": 0.99},
    {"from": "svc_trace_processor", "to": "db_logs", "type": "WRITES", "successRatio": 0.997, "confidence": 0.90},
    
    // Analytics Flow
    {"from": "svc_query_engine", "to": "db_timeseries", "type": "READS", "successRatio": 0.9995, "confidence": 0.99},
    {"from": "svc_query_engine", "to": "db_logs", "type": "READS", "successRatio": 0.998, "confidence": 0.95},
    {"from": "svc_query_engine", "to": "db_analytics", "type": "READS", "successRatio": 0.9995, "confidence": 0.99},
    {"from": "svc_query_engine", "to": "cache_redis_analytics", "type": "READS", "successRatio": 0.99, "confidence": 0.95},
    
    // API
    {"from": "svc_dashboard_api", "to": "svc_query_engine", "type": "CALLS", "successRatio": 0.99, "confidence": 0.95},
    {"from": "svc_dashboard_api", "to": "cache_redis_analytics", "type": "READS", "successRatio": 0.999, "confidence": 0.98},
}

// ============================================================================
// Main
// =============================================================================

func main() {
    flag.Parse()
    rand.Seed(time.Now().UnixNano())

    log.Println("=== FaultIQ Comprehensive Demo Seeder ===")
    
    // Get auth token
    token, err := getAuthToken()
    if err != nil {
        log.Fatalf("Failed to get auth token: %v", err)
    }
    log.Printf("✓ Authenticated as %s", adminUser)

    // Seed Payments Platform
    if err := seedProject(token, "acme-corp", "payments-platform", "payments-platform:prod", paymentsNodes, paymentsEdges); err != nil {
        log.Fatalf("Failed to seed payments platform: %v", err)
    }
    log.Println("✓ Seeded Payments Platform (15 nodes, 22 edges)")

    // Seed Orders Platform
    if err := seedProject(token, "acme-corp", "orders-platform", "orders-platform:prod", ordersNodes, ordersEdges); err != nil {
        log.Fatalf("Failed to seed orders platform: %v", err)
    }
    log.Println("✓ Seeded Orders Platform (7 nodes, 7 edges)")

    // Seed Analytics Dashboard
    if err := seedProject(token, "acme-corp", "analytics-platform", "analytics-dashboard:prod", analyticsNodes, analyticsEdges); err != nil {
        log.Fatalf("Failed to seed analytics platform: %v", err)
    }
    log.Println("✓ Seeded Analytics Dashboard (9 nodes, 8 edges)")

    // Demo incident scenarios
    if *flagDemoErrors {
        log.Println("\n=== Injecting Demo Error Scenarios ===")
        injectDemoErrors(token)
    } else {
        log.Println("\nDemo graphs ready! Run again with --demo-errors to inject failure scenarios.")
    }

    log.Println("\n✓ Seeding complete. Visit http://localhost:4000/dashboard to see results.")
}

// ============================================================================
// Helpers
// =============================================================================

func getAuthToken() (string, error) {
    form := fmt.Sprintf("grant_type=password&client_id=%s&username=%s&password=%s", clientID, adminUser, adminPass)
    resp, err := http.Post(
        fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token", keycloakBase, realm),
        "application/x-www-form-urlencoded",
        bytes.NewBufferString(form),
    )
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()

    var result map[string]interface{}
    json.NewDecoder(resp.Body).Decode(&result)
    token, ok := result["access_token"].(string)
    if !ok {
        return "", fmt.Errorf("no access_token in response")
    }
    return token, nil
}

func seedProject(token, tenant, projectName, namespace string, nodes []map[string]interface{}, edges []map[string]interface{}) error {
    // Seed nodes to Neo4j
    for _, node := range nodes {
        nodeID := node["id"].(string)
        nodeData := map[string]interface{}{
            "id":          nodeID,
            "name":        node["name"],
            "type":        node["type"],
            "tags":        node["tags"],
            "statusClass": node["statusClass"],
            "latencyP95":  node["latencyP95"],
            "errorRate":   node["errorRate"],
        }
        
        if err := postGraphNode(namespace, nodeData); err != nil && *flagVerbose {
            log.Printf("Warning: failed to post node %s: %v", nodeID, err)
        }
    }

    // Seed edges to Neo4j
    for _, edge := range edges {
        edgeData := map[string]interface{}{
            "from":         edge["from"],
            "to":           edge["to"],
            "type":         edge["type"],
            "successRatio": edge["successRatio"],
            "confidence":   edge["confidence"],
        }
        
        if err := postGraphEdge(namespace, edgeData); err != nil && *flagVerbose {
            log.Printf("Warning: failed to post edge %s->%s: %v", edge["from"], edge["to"], err)
        }
    }
    return nil
}

func postGraphNode(namespace string, node map[string]interface{}) error {
    payload, _ := json.Marshal(node)
    req, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/graphs/nodes?namespace=%s", graphMgrBase, namespace), bytes.NewBuffer(payload))
    req.Header.Set("Content-Type", "application/json")
    
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, body)
    }
    return nil
}

func postGraphEdge(namespace string, edge map[string]interface{}) error {
    payload, _ := json.Marshal(edge)
    req, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/graphs/edges?namespace=%s", graphMgrBase, namespace), bytes.NewBuffer(payload))
    req.Header.Set("Content-Type", "application/json")
    
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, body)
    }
    return nil
}

func injectDemoErrors(token string) {
    // Scenario 1: Ledger Service Timeout Cascade
    // Ledger-service starts timing out → payment-api gets 5xx → settlement-worker gets downstream errors
    // But auth-service stays healthy (shows branch pruning)
    
    log.Println("\nScenario 1: Ledger Service Timeout (Root Cause)")
    postSignal(token, map[string]interface{}{
        "tenantId":    "acme-corp",
        "projectId":   "payments-platform",
        "environment": "prod",
        "service":     "svc_ledger_service",
        "statusClass": "timeout",
        "errorRate":   0.8,
        "latencyP95":  5000,
        "source":      "demo-injector",
        "confidence":  0.85,
    })
    time.Sleep(200 * time.Millisecond)

    log.Println("Scenario 1b: Payment API sees 5xx (downstream symptom)")
    postSignal(token, map[string]interface{}{
        "tenantId":    "acme-corp",
        "projectId":   "payments-platform",
        "environment": "prod",
        "service":     "svc_payment_api",
        "statusClass": "5xx",
        "errorRate":   0.45,
        "latencyP95":  2800,
        "source":      "demo-injector",
        "confidence":  0.75,
    })
    time.Sleep(200 * time.Millisecond)

    log.Println("Scenario 1c: Settlement Worker reports failure (blast radius)")
    postSignal(token, map[string]interface{}{
        "tenantId":    "acme-corp",
        "projectId":   "payments-platform",
        "environment": "prod",
        "service":     "svc_settlement_worker",
        "statusClass": "5xx",
        "errorRate":   0.55,
        "latencyP95":  4200,
        "source":      "demo-injector",
        "confidence":  0.72,
    })
    time.Sleep(200 * time.Millisecond)

    log.Println("Scenario 1d: Auth Service REMAINS HEALTHY (shows branch pruning)")
    postSignal(token, map[string]interface{}{
        "tenantId":    "acme-corp",
        "projectId":   "payments-platform",
        "environment": "prod",
        "service":     "svc_auth_service",
        "statusClass": "2xx",
        "errorRate":   0.0002,
        "latencyP95":  38,
        "source":      "demo-injector",
        "confidence":  0.95,
    })
    time.Sleep(500 * time.Millisecond)

    // Scenario 2: Database replica failure
    log.Println("\nScenario 2: Database Replica Failure")
    postSignal(token, map[string]interface{}{
        "tenantId":    "acme-corp",
        "projectId":   "payments-platform",
        "environment": "prod",
        "service":     "db_payments_replica",
        "statusClass": "timeout",
        "errorRate":   0.9,
        "latencyP95":  6000,
        "source":      "demo-injector",
        "confidence":  0.88,
    })
    time.Sleep(200 * time.Millisecond)

    // Scenario 3: Orders Platform inventory service slow
    log.Println("\nScenario 3: Orders Platform - Inventory Service Degradation")
    postSignal(token, map[string]interface{}{
        "tenantId":    "acme-corp",
        "projectId":   "orders-platform",
        "environment": "prod",
        "service":     "svc_inventory_service",
        "statusClass": "5xx",
        "errorRate":   0.62,
        "latencyP95":  3100,
        "source":      "demo-injector",
        "confidence":  0.78,
    })
    time.Sleep(200 * time.Millisecond)

    // Scenario 4: Analytics Dashboard - Query Engine Slowness
    log.Println("\nScenario 4: Analytics - Query Engine Degradation")
    postSignal(token, map[string]interface{}{
        "tenantId":    "acme-corp",
        "projectId":   "analytics-platform",
        "environment": "prod",
        "service":     "svc_query_engine",
        "statusClass": "timeout",
        "errorRate":   0.35,
        "latencyP95":  2000,
        "source":      "demo-injector",
        "confidence":  0.81,
    })
    time.Sleep(200 * time.Millisecond)

    log.Println("Scenario 4b: Analytics - TimeSeries DB under load")
    postSignal(token, map[string]interface{}{
        "tenantId":    "acme-corp",
        "projectId":   "analytics-platform",
        "environment": "prod",
        "service":     "db_timeseries",
        "statusClass": "timeout",
        "errorRate":   0.25,
        "latencyP95":  1500,
        "source":      "demo-injector",
        "confidence":  0.77,
    })

    log.Println("\n✓ Error scenarios injected. Check /dashboard/incidents to see RCA results.")
    log.Println("  Expected: Ledger-service ranked #1, Payment-API as symptom, Auth-Service pruned")
}

func postSignal(token string, signal map[string]interface{}) error {
    payload, _ := json.Marshal(signal)
    req, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/signals", apiBase), bytes.NewBuffer(payload))
    req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
    req.Header.Set("Content-Type", "application/json")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if *flagVerbose && resp.StatusCode == http.StatusAccepted {
        log.Printf("  → Signal posted for %s", signal["service"])
    }
    return nil
}
