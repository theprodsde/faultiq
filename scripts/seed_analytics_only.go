package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net/http"
)

const (
    graphMgrBase = "http://localhost:8086"
)

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

func main() {
    log.Println("=== Seeding Analytics Dashboard Graph ===")
    
    namespace := "analytics-platform:prod"
    
    // Seed nodes
    for _, node := range analyticsNodes {
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
        
        if err := postGraphNode(namespace, nodeData); err != nil {
            log.Printf("Warning: failed to post node %s: %v", nodeID, err)
        } else {
            log.Printf("✓ Node %s", nodeID)
        }
    }

    // Seed edges
    for _, edge := range analyticsEdges {
        edgeData := map[string]interface{}{
            "from":         edge["from"],
            "to":           edge["to"],
            "type":         edge["type"],
            "successRatio": edge["successRatio"],
            "confidence":   edge["confidence"],
        }
        
        if err := postGraphEdge(namespace, edgeData); err != nil {
            log.Printf("Warning: failed to post edge %s->%s: %v", edge["from"], edge["to"], err)
        } else {
            log.Printf("✓ Edge %s -> %s", edge["from"], edge["to"])
        }
    }
    
    log.Println("\n✓ Analytics Dashboard graph seeded!")
}
