package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "time"

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
    uri := mustEnv("NEO4J_URI", "neo4j://localhost:7687")
    user := mustEnv("NEO4J_USER", "neo4j")
    pass := mustEnv("NEO4J_PASS", "test")

    ctx := context.Background()
    driver, err := neo4j.NewDriverWithContext(uri, neo4j.BasicAuth(user, pass, ""))
    if err != nil {
        log.Fatalf("neo4j driver: %v", err)
    }
    defer driver.Close(ctx)

    session := driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
    defer session.Close(ctx)

    // Create two projects each with dev/stage/prod and nodes/edges
    _, err = session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
        // clean previous demo nodes
        _, _ = tx.Run(ctx, `MATCH (n) WHERE n.demo = true DETACH DELETE n`, nil)

        // helper to create the same topology for multiple namespaces
        namespaces := []string{"dev", "staging", "production"}
        for _, ns := range namespaces {
            // Payments platform nodes
            stmts := []string{
                fmt.Sprintf(`CREATE (n:Service {id:'svc_api_gateway_%s', name:'api-gateway', type:'GATEWAY', namespace:'payments-platform:%s', demo:true, statusClass:'2xx', latencyP95:55})`, ns, ns),
                fmt.Sprintf(`CREATE (n:Service {id:'svc_payment_api_%s', name:'payment-api', type:'SERVICE', namespace:'payments-platform:%s', demo:true, statusClass:'5xx', latencyP95:1200})`, ns, ns),
                fmt.Sprintf(`CREATE (n:Service {id:'svc_auth_service_%s', name:'auth-service', type:'SERVICE', namespace:'payments-platform:%s', demo:true, statusClass:'2xx', latencyP95:38})`, ns, ns),
                fmt.Sprintf(`CREATE (n:Service {id:'svc_ledger_service_%s', name:'ledger-service', type:'SERVICE', namespace:'payments-platform:%s', demo:true, statusClass:'timeout', latencyP95:4800})`, ns, ns),
                fmt.Sprintf(`CREATE (n:Service {id:'svc_settlement_worker_%s', name:'settlement-worker', type:'SERVICE', namespace:'payments-platform:%s', demo:true, statusClass:'5xx', latencyP95:3200})`, ns, ns),
                fmt.Sprintf(`CREATE (n:Database {id:'db_postgres_%s', name:'postgres-db', type:'DATABASE', namespace:'payments-platform:%s', demo:true, statusClass:'2xx', latencyP95:42})`, ns, ns),
            }
            for _, s := range stmts {
                if _, err := tx.Run(ctx, s, nil); err != nil {
                    return nil, err
                }
            }

            // edges for payments
            edges := []string{
                fmt.Sprintf(`MATCH (a {id:'svc_api_gateway_%s'}),(b {id:'svc_payment_api_%s'}) CREATE (a)-[:CALLS]->(b)`, ns, ns),
                fmt.Sprintf(`MATCH (a {id:'svc_payment_api_%s'}),(b {id:'svc_auth_service_%s'}) CREATE (a)-[:CALLS]->(b)`, ns, ns),
                fmt.Sprintf(`MATCH (a {id:'svc_payment_api_%s'}),(b {id:'svc_ledger_service_%s'}) CREATE (a)-[:CALLS]->(b)`, ns, ns),
                fmt.Sprintf(`MATCH (a {id:'svc_settlement_worker_%s'}),(b {id:'svc_ledger_service_%s'}) CREATE (a)-[:CALLS]->(b)`, ns, ns),
                fmt.Sprintf(`MATCH (a {id:'svc_ledger_service_%s'}),(b {id:'db_postgres_%s'}) CREATE (a)-[:DEPENDS_ON]->(b)`, ns, ns),
            }
            for _, q := range edges {
                if _, err := tx.Run(ctx, q, nil); err != nil {
                    return nil, err
                }
            }

            // Catalog platform nodes
            catalog := []string{
                fmt.Sprintf(`CREATE (n:Service {id:'catalog-api_%s', name:'catalog-api', type:'SERVICE', namespace:'catalog-platform:%s', demo:true, statusClass:'2xx', latencyP95:120})`, ns, ns),
                fmt.Sprintf(`CREATE (n:Service {id:'search-service_%s', name:'search-service', type:'SERVICE', namespace:'catalog-platform:%s', demo:true, statusClass:'2xx', latencyP95:80})`, ns, ns),
                fmt.Sprintf(`CREATE (n:Database {id:'catalog-db_%s', name:'catalog-db', type:'DATABASE', namespace:'catalog-platform:%s', demo:true, statusClass:'2xx', latencyP95:60})`, ns, ns),
                fmt.Sprintf(`CREATE (n:Service {id:'cache-service_%s', name:'cache-service', type:'DATABASE', namespace:'catalog-platform:%s', demo:true, statusClass:'2xx', latencyP95:10})`, ns, ns),
            }
            for _, s := range catalog {
                if _, err := tx.Run(ctx, s, nil); err != nil {
                    return nil, err
                }
            }
            // catalog edges
            catEdges := []string{
                fmt.Sprintf(`MATCH (a {id:'catalog-api_%s'}),(b {id:'search-service_%s'}) CREATE (a)-[:CALLS]->(b)`, ns, ns),
                fmt.Sprintf(`MATCH (a {id:'search-service_%s'}),(b {id:'catalog-db_%s'}) CREATE (a)-[:DEPENDS_ON]->(b)`, ns, ns),
                fmt.Sprintf(`MATCH (a {id:'catalog-api_%s'}),(b {id:'cache-service_%s'}) CREATE (a)-[:CALLS]->(b)`, ns, ns),
            }
            for _, q := range catEdges {
                if _, err := tx.Run(ctx, q, nil); err != nil {
                    return nil, err
                }
            }
        }

        return nil, nil
    })
    if err != nil {
        log.Fatalf("seeding neo4j failed: %v", err)
    }

    // give Neo4j a moment
    time.Sleep(300 * time.Millisecond)

    fmt.Println("Neo4j seeding complete")
}
