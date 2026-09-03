package main

import (
    "context"
    "encoding/json"
    "flag"
    "fmt"
    "log"
    "math/rand"
    "net/http"
    "os"
    "time"
    "bytes"

    neo4j "github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

func mustEnv(key, def string) string {
    v := os.Getenv(key)
    if v == "" {
        return def
    }
    return v
}

func main() {
    interval := flag.Duration("interval", 0, "if >0, send events repeatedly at this interval")
    flag.Parse()

    uri := mustEnv("NEO4J_URI", "neo4j://127.0.0.1:7687")
    user := mustEnv("NEO4J_USER", "neo4j")
    pass := mustEnv("NEO4J_PASS", "testpass123")
    api := mustEnv("API_GATEWAY", "http://127.0.0.1:8080/api/v1/signals")

    ctx := context.Background()
    drv, err := neo4j.NewDriverWithContext(uri, neo4j.BasicAuth(user, pass, ""))
    if err != nil {
        log.Fatalf("neo4j driver: %v", err)
    }
    defer drv.Close(ctx)

    sess := drv.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
    defer sess.Close(ctx)

    var services []string
    _, err = sess.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
        res, err := tx.Run(ctx, `MATCH (s:Service) WHERE s.demo = true RETURN s.id as id LIMIT 100`, nil)
        if err != nil {
            return nil, err
        }
        for res.Next(ctx) {
            rec := res.Record()
            if v, ok := rec.Get("id"); ok {
                services = append(services, fmt.Sprintf("%v", v))
            }
        }
        return nil, res.Err()
    })
    if err != nil {
        log.Fatalf("query services: %v", err)
    }
    if len(services) == 0 {
        log.Fatalf("no demo services found in neo4j")
    }

    rand.Seed(time.Now().UnixNano())

    sendOnce := func() error {
        svc := services[rand.Intn(len(services))]
        sig := map[string]any{
            "tenantId":  "t_acme",
            "projectId": "p_payments",
            "service":   svc,
            "statusClass": "5xx",
            "errorRate": 1.0,
            "timestamp": time.Now().Unix(),
            "source":    "generator",
        }
        b, _ := json.Marshal(sig)
        resp, err := http.Post(api, "application/json", bytes.NewReader(b))
        if err != nil {
            return err
        }
        defer resp.Body.Close()
        log.Printf("sent fault for %s -> status %d", svc, resp.StatusCode)
        return nil
    }

    // support repeated mode
    if *interval > 0 {
        ticker := time.NewTicker(*interval)
        defer ticker.Stop()
        for {
            if err := sendOnce(); err != nil {
                log.Printf("send error: %v", err)
            }
            <-ticker.C
        }
    }

    if err := sendOnce(); err != nil {
        log.Fatalf("send failed: %v", err)
    }
}
