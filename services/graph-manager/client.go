package main

import (
    "context"
    "encoding/json"
    "fmt"
    "os"
    "time"

    neo4j "github.com/neo4j/neo4j-go-driver/v5/neo4j"
    "github.com/redis/go-redis/v9"
    "github.com/faultiq/graphclient"
)


type Manager struct {
    driver neo4j.DriverWithContext
    redis  *redis.Client
}

func NewManager(ctx context.Context, uri, user, pass string) (*Manager, error) {
    drv, err := neo4j.NewDriverWithContext(uri, neo4j.BasicAuth(user, pass, ""))
    if err != nil {
        return nil, err
    }
    // verify connectivity
    ctx2, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    if err := drv.VerifyConnectivity(ctx2); err != nil {
        drv.Close(ctx)
        return nil, err
    }
    // optional redis
    var rclient *redis.Client
    if addr := os.Getenv("REDIS_ADDR"); addr != "" {
        rclient = redis.NewClient(&redis.Options{Addr: addr})
        // quick ping
        _ = rclient.Ping(ctx).Err()
    }

    return &Manager{driver: drv, redis: rclient}, nil
}

func (m *Manager) Close(ctx context.Context) error {
    return m.driver.Close(ctx)
}

// GetSubgraph loads nodes and edges for a given namespace (namespace = project:env)
func (m *Manager) GetSubgraph(ctx context.Context, namespace string) (*graphclient.Graph, error) {
    // check cache first
    if m.redis != nil {
        key := "graph:" + namespace
        b, err := m.redis.Get(ctx, key).Bytes()
        if err == nil && len(b) > 0 {
            var g graphclient.Graph
            if err := json.Unmarshal(b, &g); err == nil {
                return &g, nil
            }
        }
    }

    session := m.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
    defer session.Close(ctx)

    g := &graphclient.Graph{
        Nodes: make(map[string]*graphclient.Node),
        Edges: make(map[string][]string),
        EdgeDetails: make(map[string]*graphclient.Edge),
    }

    // Load nodes with full details
    _, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
        // Exclude code-layer nodes (CODEFUNCTION, GITCOMMIT, CODEFILE) written by code-indexer.
        // Those belong to the code graph, not the service dependency graph, and would pollute the visualization.
        q := `MATCH (n) WHERE n.namespace = $ns AND NOT n.type IN ['CODEFUNCTION','GITCOMMIT','CODEFILE'] RETURN n.id as id, n.name as name, n.type as type, n.tags as tags, n.statusClass as statusClass, coalesce(n.latencyP95,0) as latencyP95, coalesce(n.errorRate,0) as errorRate`
        params := map[string]any{"ns": namespace}
        res, err := tx.Run(ctx, q, params)
        if err != nil {
            return nil, err
        }
        for res.Next(ctx) {
            rec := res.Record()
            id, _ := rec.Get("id")
            name, _ := rec.Get("name")
            ntype, _ := rec.Get("type")
            tags, _ := rec.Get("tags")
            sc, _ := rec.Get("statusClass")
            lat, _ := rec.Get("latencyP95")
            errRate, _ := rec.Get("errorRate")
            
            idStr := fmt.Sprintf("%v", id)
            latInt := 0
            if lat != nil {
                switch v := lat.(type) {
                case int64:
                    latInt = int(v)
                case int:
                    latInt = v
                case float64:
                    latInt = int(v)
                }
            }
            errRateFloat := 0.0
            if errRate != nil {
                switch v := errRate.(type) {
                case float64:
                    errRateFloat = v
                case int64:
                    errRateFloat = float64(v)
                }
            }
            tagsList := []string{}
            if tags != nil {
                if tl, ok := tags.([]interface{}); ok {
                    for _, t := range tl {
                        tagsList = append(tagsList, fmt.Sprintf("%v", t))
                    }
                }
            }
            
            g.Nodes[idStr] = &graphclient.Node{
                ID: idStr,
                Name: fmt.Sprintf("%v", name),
                Type: fmt.Sprintf("%v", ntype),
                Tags: tagsList,
                StatusClass: fmt.Sprintf("%v", sc),
                LatencyP95: latInt,
                ErrorRate: errRateFloat,
            }
        }
        return nil, res.Err()
    })
    if err != nil {
        return nil, err
    }

    // Load edges with full details
    _, err = session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
        // Exclude code-layer edges (IMPLEMENTS, CHANGED_IN) — same reason as node filter above
        q := `MATCH (n)-[r]->(m) WHERE n.namespace = $ns AND m.namespace = $ns AND NOT type(r) IN ['IMPLEMENTS','CHANGED_IN'] AND NOT n.type IN ['CODEFUNCTION','GITCOMMIT','CODEFILE'] AND NOT m.type IN ['CODEFUNCTION','GITCOMMIT','CODEFILE'] RETURN n.id as from, m.id as to, type(r) as relType, coalesce(r.successRatio, 1.0) as successRatio, coalesce(r.confidence, 0.8) as confidence, coalesce(r.lastObserved, 0) as lastObserved` 
        params := map[string]any{"ns": namespace}
        res, err := tx.Run(ctx, q, params)
        if err != nil {
            return nil, err
        }
        for res.Next(ctx) {
            rec := res.Record()
            from, _ := rec.Get("from")
            to, _ := rec.Get("to")
            relType, _ := rec.Get("relType")
            successRatio, _ := rec.Get("successRatio")
            confidence, _ := rec.Get("confidence")
            lastObserved, _ := rec.Get("lastObserved")
            
            fromStr := fmt.Sprintf("%v", from)
            toStr := fmt.Sprintf("%v", to)
            g.Edges[fromStr] = append(g.Edges[fromStr], toStr)
            
            successRatioFloat := 1.0
            if sr, ok := successRatio.(float64); ok {
                successRatioFloat = sr
            }
            confFloat := 0.8
            if c, ok := confidence.(float64); ok {
                confFloat = c
            }
            lastObsInt := int64(0)
            if lo, ok := lastObserved.(int64); ok {
                lastObsInt = lo
            }
            
            edgeID := fromStr + "->" + toStr
            g.EdgeDetails[edgeID] = &graphclient.Edge{
                ID: edgeID,
                From: fromStr,
                To: toStr,
                Type: fmt.Sprintf("%v", relType),
                SuccessRatio: successRatioFloat,
                Confidence: confFloat,
                LastObserved: lastObsInt,
            }
        }
        return nil, res.Err()
    })
    if err != nil {
        return nil, err
    }

    // set cache
    if m.redis != nil {
        key := "graph:" + namespace
        if jb, err := json.Marshal(g); err == nil {
            _ = m.redis.Set(ctx, key, jb, 60*time.Second).Err() // shorter TTL for fresh data
        }
    }

    return g, nil
}

// ListNamespaces returns all unique namespace values stored across all nodes in Neo4j.
func (m *Manager) ListNamespaces(ctx context.Context) ([]string, error) {
    session := m.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
    defer session.Close(ctx)

    var namespaces []string
    _, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
        res, err := tx.Run(ctx,
            `MATCH (n) WHERE n.namespace IS NOT NULL RETURN DISTINCT n.namespace ORDER BY n.namespace`,
            nil)
        if err != nil {
            return nil, err
        }
        for res.Next(ctx) {
            rec := res.Record()
            ns, _ := rec.Get("n.namespace")
            if ns != nil {
                namespaces = append(namespaces, fmt.Sprintf("%v", ns))
            }
        }
        return nil, res.Err()
    })
    if err != nil {
        return nil, err
    }
    if namespaces == nil {
        namespaces = []string{}
    }
    return namespaces, nil
}

// WriteNode creates or updates a node in Neo4j
func (m *Manager) WriteNode(ctx context.Context, namespace string, node *graphclient.Node) error {
    session := m.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
    defer session.Close(ctx)

    _, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
        q := `MERGE (n {id: $id})
              SET n.namespace = $namespace, n.name = $name, n.type = $type, n.tags = $tags,
                  n.statusClass = $statusClass, n.latencyP95 = $latencyP95, n.errorRate = $errorRate,
                  n.updatedAt = timestamp()`
        params := map[string]any{
            "id": node.ID,
            "namespace": namespace,
            "name": node.Name,
            "type": node.Type,
            "tags": node.Tags,
            "statusClass": node.StatusClass,
            "latencyP95": node.LatencyP95,
            "errorRate": node.ErrorRate,
        }
        return tx.Run(ctx, q, params)
    })
    
    // invalidate cache
    if m.redis != nil && err == nil {
        key := "graph:" + namespace
        _ = m.redis.Del(ctx, key).Err()
    }
    
    return err
}

// WriteEdge creates or updates an edge in Neo4j
func (m *Manager) WriteEdge(ctx context.Context, namespace string, edge *graphclient.Edge) error {
    session := m.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
    defer session.Close(ctx)

    _, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
        // Neo4j requires relationship type to be static in MERGE, so use a generic type
        // and store the actual type as a property
        q := `MATCH (from {id: $fromId, namespace: $namespace}), (to {id: $toId, namespace: $namespace})
              MERGE (from)-[r:DEPENDS_ON]->(to)
              SET r.type = $edgeType, r.successRatio = $successRatio, r.confidence = $confidence, r.lastObserved = $lastObserved`
        params := map[string]any{
            "fromId": edge.From,
            "toId": edge.To,
            "namespace": namespace,
            "edgeType": edge.Type,
            "successRatio": edge.SuccessRatio,
            "confidence": edge.Confidence,
            "lastObserved": time.Now().UnixMilli(),
        }
        return tx.Run(ctx, q, params)
    })
    
    // invalidate cache
    if m.redis != nil && err == nil {
        key := "graph:" + namespace
        _ = m.redis.Del(ctx, key).Err()
    }
    
    return err
}
