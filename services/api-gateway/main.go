package main

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "log"
    "math"
    "net"
    "net/http"
    "net/url"
    "os"
    "os/signal"
    "sort"
    "strconv"
    "strings"
    "sync"
    "syscall"
    "time"

    "github.com/faultiq/api-gateway/internal/types"
    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/prometheus/client_golang/prometheus/promhttp"
    "github.com/redis/go-redis/v9"
    "golang.org/x/time/rate"
    "gopkg.in/yaml.v3"
)

    

var (
    // logLevel controls simple logging: debug|info|error
    logLevel = "info"
    // global JWKS manager (initialized at startup if configured)
    globalJWKS *JWKSManager
    // globalDB is a shared pgxpool.Pool initialized once at startup.
    // All handlers should use getDB() rather than opening per-request connections.
    globalDB *pgxpool.Pool
)

// initDB creates the global connection pool. Must be called once in main() before registering routes.
func initDB(ctx context.Context) error {
    dsn := os.Getenv("PG_DSN")
    if dsn == "" {
        dsn = "postgres://postgres:postgres@postgres:5432/faultiq?sslmode=disable"
    }
    config, err := pgxpool.ParseConfig(dsn)
    if err != nil {
        return fmt.Errorf("db pool config: %w", err)
    }
    config.MaxConns = 20                         // max concurrent connections to Postgres
    config.MinConns = 2                          // keep 2 warm connections
    config.MaxConnIdleTime = 5 * time.Minute     // release idle connections
    config.MaxConnLifetime = 30 * time.Minute    // recycle connections periodically
    globalDB, err = pgxpool.NewWithConfig(ctx, config)
    if err != nil {
        return fmt.Errorf("db pool init: %w", err)
    }
    return nil
}

// getDB returns the global connection pool. If the pool has not been initialized
// (e.g. Postgres was unavailable at startup), returns nil and callers fall back
// to per-request connections via pgx.Connect.
func getDB() *pgxpool.Pool {
    return globalDB
}

// ─────────────────────────────────────────────────────────────────────────────
// Per-IP rate limiting
// ─────────────────────────────────────────────────────────────────────────────

var (
    ipLimiterMu sync.Mutex
    ipLimiters  = make(map[string]*rate.Limiter)
)

func getIPLimiter(ip string) *rate.Limiter {
    ipLimiterMu.Lock()
    defer ipLimiterMu.Unlock()
    if lim, ok := ipLimiters[ip]; ok {
        return lim
    }
    lim := rate.NewLimiter(50, 100)
    ipLimiters[ip] = lim
    return lim
}

func rateLimitMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Extract real IP (handles X-Forwarded-For from nginx TLS proxy)
        ip := r.RemoteAddr
        if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
            ip = strings.Split(forwarded, ",")[0]
        }
        ip = strings.TrimSpace(ip)
        if !getIPLimiter(ip).Allow() {
            http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
            return
        }
        next.ServeHTTP(w, r)
    })
}

// ─────────────────────────────────────────────────────────────────────────────
// Input sanitization
// ─────────────────────────────────────────────────────────────────────────────

// sanitizeString trims whitespace and truncates s to maxLen bytes.
func sanitizeString(s string, maxLen int) string {
    s = strings.TrimSpace(s)
    if len(s) > maxLen {
        s = s[:maxLen]
    }
    return s
}

func Debugf(format string, v ...interface{}) {
    if logLevel == "debug" {
        log.Printf("DEBUG: "+format, v...)
    }
}

func Infof(format string, v ...interface{}) {
    if logLevel == "debug" || logLevel == "info" {
        log.Printf("INFO: "+format, v...)
    }
}

func Errorf(format string, v ...interface{}) {
    log.Printf("ERROR: "+format, v...)
}

// corsWrap returns a handler that adds CORS headers and handles preflight.
func corsWrap(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        origin := r.Header.Get("Origin")
        allowed := os.Getenv("ALLOWED_ORIGIN")
        if allowed == "" {
            allowed = "*"
        }

        // Prefer echoing request Origin when ALLOWED_ORIGIN is not '*'
        if allowed == "*" {
            w.Header().Set("Access-Control-Allow-Origin", "*")
        } else if origin != "" {
            w.Header().Set("Access-Control-Allow-Origin", origin)
            w.Header().Set("Vary", "Origin")
        } else {
            w.Header().Set("Access-Control-Allow-Origin", allowed)
        }

        w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
        w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Requested-With")
        // If callers want to use credentials (cookies/authorization), they can set ALLOW_CREDENTIALS=1
        if os.Getenv("ALLOW_CREDENTIALS") == "1" {
            w.Header().Set("Access-Control-Allow-Credentials", "true")
        }

        // Preflight
        if r.Method == http.MethodOptions {
            w.WriteHeader(http.StatusNoContent)
            return
        }

        next.ServeHTTP(w, r)
    })
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
    // check JWKS health if enabled
    if globalJWKS != nil {
        if !globalJWKS.Healthy() {
            http.Error(w, "jwks not healthy", http.StatusServiceUnavailable)
            return
        }
    }
    w.WriteHeader(http.StatusOK)
    _, _ = w.Write([]byte("ok"))
}

func projectsHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method == http.MethodPost {
        // create project - enforce tenant scoping from token
        var body struct{
            Name string `json:"name"`
            Slug string `json:"slug"`
            TenantID string `json:"tenant_id"`
        }
        if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
            http.Error(w, "bad request", http.StatusBadRequest)
            return
        }
        // enforce tenant claim
        claims := GetClaims(r)
        if claims == nil {
            http.Error(w, "unauthorized", http.StatusUnauthorized)
            return
        }
        if tc, ok := claims["tenant"].(string); ok {
            if tc != body.TenantID && !hasRole(claims, "super_admin") {
                http.Error(w, "forbidden - tenant mismatch", http.StatusForbidden)
                return
            }
        }
        // For demo, return created project id
        w.WriteHeader(http.StatusCreated)
        _ = json.NewEncoder(w).Encode(map[string]string{"projectId":"proj-123"})
        return
    }
    // Query all projects from DB (tenant-scoped via JWT if available)
    claims := GetClaims(r)
    tenantID := ""
    if claims != nil {
        if tc, ok := claims["tenant"].(string); ok {
            tenantID = tc
        }
    }
    db := getDB()
    if db == nil {
        http.Error(w, "db unavailable", http.StatusServiceUnavailable)
        return
    }
    ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
    defer cancel()
    var rows pgx.Rows
    var err error
    if tenantID != "" {
        rows, err = db.Query(ctx, `SELECT id, name, COALESCE(environment,'prod') FROM projects WHERE tenant_id=$1 ORDER BY name`, tenantID)
    } else {
        rows, err = db.Query(ctx, `SELECT id, name, COALESCE(environment,'prod') FROM projects ORDER BY name LIMIT 50`)
    }
    if err != nil {
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode([]types.Project{})
        return
    }
    defer rows.Close()
    var projects []types.Project
    for rows.Next() {
        var p types.Project
        if rows.Scan(&p.ID, &p.Name, &p.Environment) == nil {
            projects = append(projects, p)
        }
    }
    if projects == nil {
        projects = []types.Project{}
    }
    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(projects)
}

// publicProjectsHandler returns a public list of published projects (no auth required)
func publicProjectsHandler(w http.ResponseWriter, r *http.Request) {
    db := getDB()
    if db == nil {
        http.Error(w, "db unavailable", http.StatusServiceUnavailable)
        return
    }
    ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
    defer cancel()
    rows, err := db.Query(ctx,
        `SELECT id, name, tenant_id, COALESCE(environment,'prod') FROM projects
         WHERE metadata->>'graphStatus' = 'PUBLISHED' ORDER BY name LIMIT 20`)
    if err != nil {
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode([]types.Project{})
        return
    }
    defer rows.Close()
    var projects []types.Project
    for rows.Next() {
        var p types.Project
        if rows.Scan(&p.ID, &p.Name, &p.TenantID, &p.Environment) == nil {
            projects = append(projects, p)
        }
    }
    if projects == nil {
        projects = []types.Project{}
    }
    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(projects)
}
// servicesHandler probes local services and returns their reachability and latency.
func servicesHandler(w http.ResponseWriter, r *http.Request) {
    type svc struct {
        ID          string
        Name        string
        Description string
        Type        string // "http" or "tcp"
        Target      string // host:port or full URL
    }

    // Default list of services to probe (container hostnames + ports)
    servicesToCheck := []svc{
        {ID: "api-gateway", Name: "API Gateway", Description: "Main API gateway routing requests", Type: "http", Target: "http://localhost:8080/health"},
        {ID: "detection-engine", Name: "Detection Engine", Description: "Real-time anomaly detection service", Type: "http", Target: "http://detection-engine:8084/health"},
        {ID: "graph-manager", Name: "Graph Manager", Description: "Service graph and dependency tracking", Type: "http", Target: "http://graph-manager:8086/health"},
        {ID: "neo4j", Name: "Neo4j Database", Description: "Graph database for relationship management", Type: "http", Target: "http://neo4j:7474/"},
        {ID: "postgres", Name: "PostgreSQL Database", Description: "Relational database for application data", Type: "tcp", Target: "postgres:5432"},
        {ID: "redis", Name: "Redis Cache", Description: "In-memory cache and session store", Type: "tcp", Target: "redis:6379"},
        {ID: "keycloak", Name: "Keycloak", Description: "Authentication provider", Type: "http", Target: "http://keycloak:8080/realms/faultiq/.well-known/openid-configuration"},
    }

    // Allow overriding/adding targets via env var (comma-separated id=target)
    // Example: STATUS_OVERRIDE=custom=http://some:1234
    if extras := os.Getenv("STATUS_OVERRIDE"); extras != "" {
        parts := strings.Split(extras, ",")
        for _, p := range parts {
            if kv := strings.SplitN(p, "=", 2); len(kv) == 2 {
                servicesToCheck = append(servicesToCheck, svc{ID: kv[0], Name: kv[0], Description: "custom", Type: "http", Target: kv[1]})
            }
        }
    }

    type outSvc struct {
        ID          string  `json:"id"`
        Name        string  `json:"name"`
        Status      string  `json:"status"`
        Description string  `json:"description"`
        Uptime      float32 `json:"uptime"`
        ResponseTime int64  `json:"responseTime"`
    }

    var wg sync.WaitGroup
    mu := sync.Mutex{}
    results := make([]outSvc, 0, len(servicesToCheck))

    httpClient := &http.Client{Timeout: 1500 * time.Millisecond}

    for _, s := range servicesToCheck {
        wg.Add(1)
        go func(s svc) {
            defer wg.Done()
            start := time.Now()
            status := "unknown"
            respTime := int64(0)
            uptime := float32(0)

            switch s.Type {
            case "http":
                // try GET
                req, _ := http.NewRequest("GET", s.Target, nil)
                resp, err := httpClient.Do(req)
                if err == nil && resp != nil {
                    resp.Body.Close()
                    respTime = time.Since(start).Milliseconds()
                    if resp.StatusCode >= 200 && resp.StatusCode < 300 {
                        status = "healthy"
                        uptime = 100
                    } else {
                        status = "degraded"
                    }
                } else {
                    status = "unhealthy"
                }
            case "tcp":
                // target is host:port
                connStart := time.Now()
                timeout := 1500 * time.Millisecond
                conn, err := net.DialTimeout("tcp", s.Target, timeout)
                if err == nil {
                    conn.Close()
                    respTime = time.Since(connStart).Milliseconds()
                    status = "healthy"
                    uptime = 100
                } else {
                    status = "unhealthy"
                }
            default:
                status = "unknown"
            }

            mu.Lock()
            results = append(results, outSvc{ID: s.ID, Name: s.Name, Status: status, Description: s.Description, Uptime: uptime, ResponseTime: respTime})
            mu.Unlock()
        }(s)
    }
    wg.Wait()

    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(results)
}

// projectStatusHandler returns combined onboarding & incident status for a project
func projectStatusHandler(w http.ResponseWriter, r *http.Request) {
    // path: /api/v1/projects/{id}/status
    parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
    if len(parts) < 4 {
        http.Error(w, "bad path", http.StatusBadRequest)
        return
    }
    projectID := parts[3]

    // If this is a graphs request, proxy to graph-manager
    // expected path: /api/v1/projects/{projectId}/graphs/{action}
    // project-scoped incidents: GET /api/v1/projects/{projectId}/incidents
    if len(parts) >= 5 && parts[4] == "incidents" {
        if r.Method != http.MethodGet {
            http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
            return
        }
        // return incidents filtered by project
        project := projectID
        incidentMu.Lock()
        defer incidentMu.Unlock()
        var out []types.Incident
        claims := GetClaims(r)
        for _, inc := range incidents {
            if inc.ProjectID != project {
                continue
            }
            // enforce tenant scoping unless super_admin
            if claims != nil && !hasRole(claims, "super_admin") {
                if t, ok := claims["tenant"].(string); ok {
                    if inc.TenantID != t {
                        continue
                    }
                }
            }
            out = append(out, inc)
        }
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(map[string]interface{}{"incidents": out, "total": len(out)})
        return
    }

    if len(parts) >= 5 && parts[4] == "graphs" {
        // Only implement proxy for 'current' for now
        action := ""
        if len(parts) >= 6 {
            action = parts[5]
        }
        graphManagerURL := os.Getenv("GRAPH_MANAGER_URL")
        if graphManagerURL == "" {
            graphManagerURL = "http://graph-manager:8086"
        }

        switch action {
        case "current":
            // Allow callers to pass explicit namespace, otherwise construct from projectID and environment
            namespace := r.URL.Query().Get("namespace")
            if namespace == "" {
                env := r.URL.Query().Get("environment")
                if env == "" {
                    env = "prod"
                }
                // Prefer to build namespace from project slug stored in Postgres
                if db2 := getDB(); db2 != nil {
                    ctx2, cancel2 := context.WithTimeout(r.Context(), 2*time.Second)
                    defer cancel2()
                    var slug string
                    if err := db2.QueryRow(ctx2, "select slug from projects where id=$1", projectID).Scan(&slug); err == nil && slug != "" {
                        namespace = fmt.Sprintf("%s:%s", slug, env)
                    }
                }
                // fallback: sanitize common prefixes from project ID
                if namespace == "" {
                    pid := strings.TrimPrefix(projectID, "p_")
                    pid = strings.TrimPrefix(pid, "proj-")
                    namespace = fmt.Sprintf("%s:%s", pid, env)
                }
            }
            target := fmt.Sprintf("%s/api/v1/graphs?namespace=%s", graphManagerURL, url.QueryEscape(namespace))
            resp, err := http.Get(target)
            if err != nil {
                http.Error(w, "graph-manager unreachable", http.StatusServiceUnavailable)
                return
            }
            defer resp.Body.Close()
            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(resp.StatusCode)
            _, _ = io.Copy(w, resp.Body)
            return
        default:
            http.Error(w, "graph action not implemented", http.StatusNotImplemented)
            return
        }
    }

    // get latest onboarding job for project
    db := getDB()
    if db == nil {
        http.Error(w, "db unavailable", http.StatusServiceUnavailable)
        return
    }
    ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
    defer cancel()
    var jobID, status, created string
    err := db.QueryRow(ctx, "select id,status,created_at from onboarding_jobs where project_id=$1 order by created_at desc limit 1", projectID).Scan(&jobID, &status, &created)
    if err != nil {
        // not found is OK
        jobID = ""
        status = "not_started"
    }

    // count incidents in-memory
    incidentMu.Lock()
    cnt := 0
    var lastIncident types.Incident
    for _, inc := range incidents {
        if inc.ProjectID == projectID {
            cnt++
            lastIncident = inc
        }
    }
    incidentMu.Unlock()

    _ = json.NewEncoder(w).Encode(map[string]interface{}{"project_id": projectID, "onboarding": map[string]string{"job_id": jobID, "status": status, "created_at": created}, "incidents_count": cnt, "last_incident": lastIncident})
}

// In-memory incident store
var (
    incidentMu  sync.Mutex
    incidents   = make(map[string]types.Incident)
    incidentSeq = 0
)

func listIncidentsHandler(w http.ResponseWriter, r *http.Request) {
    claims := GetClaims(r)
    var out []types.IncidentListItem

    // Get optional projectId filter from query params
    projectIDFilter := r.URL.Query().Get("projectId")

    ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
    defer cancel()

    // Use shared pool if available; fall back to per-request connection
    pool := getDB()
    connOk := pool != nil
    var fallbackConn *pgx.Conn
    if !connOk {
        dsn := os.Getenv("PG_DSN")
        if dsn == "" {
            dsn = "postgres://postgres:postgres@postgres:5432/faultiq?sslmode=disable"
        }
        if c, err := pgx.Connect(ctx, dsn); err == nil {
            fallbackConn = c
            connOk = true
            defer fallbackConn.Close(ctx)
        }
    }
    // queryIncidents runs the query against whichever connection is available
    queryIncidents := func(query string, args ...interface{}) (pgx.Rows, error) {
        if pool != nil {
            return pool.Query(ctx, query, args...)
        }
        if fallbackConn != nil {
            return fallbackConn.Query(ctx, query, args...)
        }
        return nil, fmt.Errorf("no db connection")
    }
    if connOk {
        // Join with projects table to get tenant_id through project relationship
        query := "SELECT i.id, COALESCE(p.tenant_id,''), i.project_id, i.service, i.status, COALESCE(EXTRACT(EPOCH FROM i.detected_at)::BIGINT, 0), COALESCE(i.root_cause_candidate,''), COALESCE(i.confidence,0.8) FROM incidents i LEFT JOIN projects p ON i.project_id = p.id"
        args := []interface{}{}
        argIdx := 1
        if claims != nil && !hasRole(claims, "super_admin") {
            if t, ok := claims["tenant"].(string); ok {
                query += fmt.Sprintf(" WHERE p.tenant_id=$%d", argIdx)
                args = append(args, t)
                argIdx++
            }
        }
        if projectIDFilter != "" {
            if strings.Contains(query, "WHERE") {
                query += fmt.Sprintf(" AND i.project_id=$%d", argIdx)
            } else {
                query += fmt.Sprintf(" WHERE i.project_id=$%d", argIdx)
            }
            args = append(args, projectIDFilter)
            argIdx++
        }
        query += " ORDER BY i.detected_at DESC LIMIT 200"
        rows, qErr := queryIncidents(query, args...)
        if qErr == nil {
            defer rows.Close()
            for rows.Next() {
                var id, tenantID, projectID, service, status, rootCause string
                var detectedAt int64
                var confidence float64
                if sErr := rows.Scan(&id, &tenantID, &projectID, &service, &status, &detectedAt, &rootCause, &confidence); sErr == nil {
                    out = append(out, types.IncidentListItem{
                        ID: id, ProjectID: projectID, Service: service,
                        Status: status, DetectedAt: detectedAt,
                        RootCauseCandidate: rootCause, Confidence: confidence, EvidenceCount: 1,
                    })
                }
            }
        }
    } else {
        // Fallback to in-memory store
        incidentMu.Lock()
        defer incidentMu.Unlock()
        for _, v := range incidents {
            if claims != nil {
                if !hasRole(claims, "super_admin") {
                    if t, ok := claims["tenant"].(string); ok {
                        if v.TenantID != t {
                            continue
                        }
                    }
                }
            }
            if projectIDFilter != "" && v.ProjectID != projectIDFilter {
                continue
            }
            out = append(out, types.IncidentListItem{
                ID: v.ID, ProjectID: v.ProjectID, Service: v.Service,
                Status: v.Status, DetectedAt: v.DetectedAt,
                RootCauseCandidate: v.RootCauseCandidate, Confidence: v.Confidence,
                EvidenceCount: len(v.Evidence),
            })
        }
    }

    if out == nil {
        out = []types.IncidentListItem{}
    }
    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(map[string]interface{}{"incidents": out, "total": len(out)})
}

// getIncidentHandler handles GET /api/v1/incidents/{incidentId}
func getIncidentHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }

    parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/incidents/"), "/")
    if len(parts) < 1 || parts[0] == "" {
        http.Error(w, "invalid path", http.StatusBadRequest)
        return
    }
    incidentId := parts[0]

    claims := GetClaims(r)

    conn := getDB()
    if conn != nil {
        dbCtx, dbCancel := context.WithTimeout(r.Context(), 5*time.Second)
        defer dbCancel()
        var id, tenantId, projectId, service, status, rootCauseCandidate, evidenceJSON string
        var confidence float64
        var detectedAtTime time.Time

        err := conn.QueryRow(dbCtx,
            `SELECT id, COALESCE(tenant_id,''), COALESCE(project_id,''), COALESCE(service,''),
                    COALESCE(status,'OPEN'), detected_at,
                    COALESCE(root_cause_candidate,''), COALESCE(confidence,0.8),
                    COALESCE(evidence::text,'[]')
             FROM incidents WHERE id = $1`,
            incidentId,
        ).Scan(&id, &tenantId, &projectId, &service, &status, &detectedAtTime,
            &rootCauseCandidate, &confidence, &evidenceJSON)

        if err == nil {
            if claims != nil && !hasRole(claims, "super_admin") {
                if t, ok := claims["tenant"].(string); ok {
                    if t != tenantId {
                        http.Error(w, "forbidden", http.StatusForbidden)
                        return
                    }
                }
            }

            // Parse raw evidence (node IDs + confidence stored by detection-engine)
            type EvidenceItem struct {
                NodeID     string  `json:"nodeId"`
                Confidence float64 `json:"confidence"`
            }
            var evidenceItems []EvidenceItem
            _ = json.Unmarshal([]byte(evidenceJSON), &evidenceItems)

            // Query rca_candidates table
            type rcaRow struct {
                NodeID     string
                Confidence float64
            }
            var rcaRows []rcaRow
            rows, rcaErr := conn.Query(dbCtx,
                `SELECT COALESCE(candidate_node_id,''), COALESCE(confidence_score,0.0)
                 FROM rca_candidates WHERE incident_id = $1
                 ORDER BY confidence_score DESC LIMIT 10`,
                id,
            )
            if rcaErr == nil {
                defer rows.Close()
                for rows.Next() {
                    var rr rcaRow
                    if sErr := rows.Scan(&rr.NodeID, &rr.Confidence); sErr == nil {
                        rcaRows = append(rcaRows, rr)
                    }
                }
            }
            if len(rcaRows) == 0 && rootCauseCandidate != "" {
                rcaRows = []rcaRow{{NodeID: rootCauseCandidate, Confidence: confidence}}
            }

            type FrontendRCACandidate struct {
                Rank            int                `json:"rank"`
                NodeID          string             `json:"nodeId"`
                Name            string             `json:"name"`
                Confidence      float64            `json:"confidence"`
                FaultType       string             `json:"faultType"`
                Evidence        map[string]float64 `json:"evidence"`
                ImpactedCallers []string           `json:"impactedCallers"`
            }
            candidates := make([]FrontendRCACandidate, len(rcaRows))
            for i, row := range rcaRows {
                faultType := "unknown"
                if row.Confidence > 0.7 {
                    faultType = "high-error-rate"
                } else if row.Confidence > 0.4 {
                    faultType = "latency-anomaly"
                }
                candidates[i] = FrontendRCACandidate{
                    Rank:            i + 1,
                    NodeID:          row.NodeID,
                    Name:            row.NodeID,
                    Confidence:      row.Confidence,
                    FaultType:       faultType,
                    Evidence:        map[string]float64{"directSignal": row.Confidence},
                    ImpactedCallers: []string{},
                }
            }

            blastRadius := []string{}
            for i, c := range candidates {
                if i > 0 {
                    blastRadius = append(blastRadius, c.Name)
                }
            }

            // Generate recommendations from fault types
            type FrontendRecommendation struct {
                Rank     int      `json:"rank"`
                Title    string   `json:"title"`
                Category string   `json:"category"`
                Steps    []string `json:"steps"`
                Reason   string   `json:"reason"`
            }
            type recTemplate struct {
                Title    string
                Category string
                Steps    []string
                Reason   string
            }
            recsByFault := map[string]recTemplate{
                "high-error-rate": {
                    Title:    "Investigate error logs and recent deployments",
                    Category: "Debugging",
                    Steps: []string{
                        "Review service logs for 5xx / exception patterns",
                        "Check deployments to this service in the last 2 hours",
                        "Verify downstream dependency health (databases, queues)",
                        "Enable or tighten circuit-breaker thresholds",
                        "Consider rolling back the most recent release",
                    },
                    Reason: "High error rate indicates a code defect, bad deployment, or cascading dependency failure.",
                },
                "latency-anomaly": {
                    Title:    "Diagnose resource saturation or slow downstream calls",
                    Category: "Performance",
                    Steps: []string{
                        "Check database connection pool utilization",
                        "Review CPU and memory usage — look for saturation",
                        "Inspect request queue depth and backpressure metrics",
                        "Check for N+1 query patterns or missing indexes",
                        "Trace slow spans to identify the bottleneck call",
                    },
                    Reason: "Latency spike is commonly caused by DB contention, resource exhaustion, or slow downstream calls.",
                },
                "timeout-burst": {
                    Title:    "Mitigate timeout cascade",
                    Category: "Reliability",
                    Steps: []string{
                        "Check for resource exhaustion (threads, file descriptors)",
                        "Reduce upstream timeout budgets to fail fast",
                        "Scale the affected service horizontally",
                        "Check downstream services for independent outages",
                    },
                    Reason: "Timeout bursts indicate an overloaded service or an unresponsive downstream dependency.",
                },
                "unknown": {
                    Title:    "Triage and isolate the fault",
                    Category: "Operations",
                    Steps: []string{
                        "Review the service health dashboard for correlated metrics",
                        "Check correlated alerts from monitoring systems",
                        "Examine distributed traces for the affected service",
                        "Escalate if SLA breach is imminent",
                    },
                    Reason: "Anomalous behavior detected — begin triage to identify and isolate the root cause.",
                },
            }

            // Deduplicate by fault type — one recommendation per distinct fault type
            seenFault := map[string]bool{}
            recommendations := []FrontendRecommendation{}
            rank := 1
            for _, c := range candidates {
                ft := c.FaultType
                if seenFault[ft] {
                    continue
                }
                seenFault[ft] = true
                tmpl, ok := recsByFault[ft]
                if !ok {
                    tmpl = recsByFault["unknown"]
                }
                recommendations = append(recommendations, FrontendRecommendation{
                    Rank:     rank,
                    Title:    tmpl.Title,
                    Category: tmpl.Category,
                    Steps:    tmpl.Steps,
                    Reason:   "For " + c.Name + ": " + tmpl.Reason,
                })
                rank++
            }
            // Always add a generic triage step if we have no recommendations
            if len(recommendations) == 0 {
                tmpl := recsByFault["unknown"]
                recommendations = append(recommendations, FrontendRecommendation{
                    Rank: 1, Title: tmpl.Title, Category: tmpl.Category,
                    Steps: tmpl.Steps, Reason: tmpl.Reason,
                })
            }

            // Build trigger signals for display (what anomalies were observed)
            type TriggerSignal struct {
                Service    string  `json:"service"`
                NodeID     string  `json:"nodeId"`
                Confidence float64 `json:"confidence"`
                FaultType  string  `json:"faultType"`
            }
            triggerSignals := make([]TriggerSignal, 0, len(evidenceItems))
            for _, ev := range evidenceItems {
                ft := "unknown"
                if ev.Confidence > 0.7 {
                    ft = "high-error-rate"
                } else if ev.Confidence > 0.4 {
                    ft = "latency-anomaly"
                }
                triggerSignals = append(triggerSignals, TriggerSignal{
                    Service: ev.NodeID, NodeID: ev.NodeID,
                    Confidence: ev.Confidence, FaultType: ft,
                })
            }
            // If no evidence items, use the triggering service itself
            if len(triggerSignals) == 0 && service != "" {
                ft := "high-error-rate"
                if confidence <= 0.7 {
                    ft = "latency-anomaly"
                }
                triggerSignals = []TriggerSignal{{
                    Service: service, NodeID: service,
                    Confidence: confidence, FaultType: ft,
                }}
            }

            type FrontendIncident struct {
                IncidentID          string                   `json:"incidentId"`
                ProjectID           string                   `json:"projectId"`
                Service             string                   `json:"service"`
                Environment         string                   `json:"environment"`
                DetectedAt          string                   `json:"detectedAt"`
                GraphVersion        int                      `json:"graphVersion"`
                Status              string                   `json:"status"`
                RootCauseCandidates []FrontendRCACandidate   `json:"rootCauseCandidates"`
                PrunedServices      []string                 `json:"prunedServices"`
                BlastRadius         []string                 `json:"blastRadius"`
                Recommendations     []FrontendRecommendation `json:"recommendations"`
                TriggerSignals      []TriggerSignal          `json:"triggerSignals"`
                ResolvedAt          *string                  `json:"resolvedAt"`
                Feedback            interface{}              `json:"feedback"`
            }

            resp := FrontendIncident{
                IncidentID:          id,
                ProjectID:           projectId,
                Service:             service,
                Environment:         service,
                DetectedAt:          detectedAtTime.UTC().Format(time.RFC3339),
                GraphVersion:        1,
                Status:              status,
                RootCauseCandidates: candidates,
                PrunedServices:      []string{},
                BlastRadius:         blastRadius,
                Recommendations:     recommendations,
                TriggerSignals:      triggerSignals,
                ResolvedAt:          nil,
                Feedback:            nil,
            }

            w.Header().Set("Content-Type", "application/json")
            _ = json.NewEncoder(w).Encode(resp)
            return
        }
    }

    // Fallback to in-memory if DB unavailable
    incidentMu.Lock()
    defer incidentMu.Unlock()

    for _, inc := range incidents {
        if inc.ID == incidentId {
            if claims != nil && !hasRole(claims, "super_admin") {
                if t, ok := claims["tenant"].(string); ok {
                    if inc.TenantID != t {
                        http.Error(w, "forbidden", http.StatusForbidden)
                        return
                    }
                }
            }
            w.Header().Set("Content-Type", "application/json")
            _ = json.NewEncoder(w).Encode(inc)
            return
        }
    }

    http.Error(w, "incident not found", http.StatusNotFound)
}

// projectIncidentsHandler handles GET /api/v1/projects/{projectId}/incidents
// Supports: ?q=<search>, ?status=<status>, ?cursor=<ISO timestamp>, ?limit=N (max 100)
// Returns: { incidents, total, hasMore, nextCursor }
func projectIncidentsHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet && r.Method != http.MethodPost {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }

    // Handle POST /api/v1/projects/{projectId}/incidents/analyze
    if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/analyze") {
        analyzeIncidentHandler(w, r)
        return
    }

    // Extract projectId from URL path: /api/v1/projects/{projectId}/incidents
    parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/projects/"), "/")
    if len(parts) < 2 || parts[0] == "" {
        http.Error(w, "invalid path", http.StatusBadRequest)
        return
    }
    projectId := parts[0]
    claims := GetClaims(r)

    // Parse query params
    searchQ := r.URL.Query().Get("q")
    statusFilter := r.URL.Query().Get("status")
    cursorStr := r.URL.Query().Get("cursor")
    limitStr := r.URL.Query().Get("limit")
    limit := 50
    if limitStr != "" {
        if n, err := strconv.Atoi(limitStr); err == nil && n > 0 {
            limit = n
        }
    }
    if limit > 100 {
        limit = 100
    }

    db := getDB()
    ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
    defer cancel()

    type fetchedRow struct {
        item       types.IncidentListItem
        tenantID   string
        detectedAt time.Time
    }

    var fetched []fetchedRow
    hasMore := false
    nextCursor := ""
    var out []types.IncidentListItem

    if db != nil {
        // Parse cursor (ISO timestamp) — nil pointer maps to SQL NULL
        var cursorTime *time.Time
        if cursorStr != "" {
            if t, parseErr := time.Parse(time.RFC3339, cursorStr); parseErr == nil {
                cursorTime = &t
            }
        }

        query := `SELECT id, COALESCE(tenant_id,''), COALESCE(project_id,''), COALESCE(service,''),
                         COALESCE(status,'OPEN'), COALESCE(EXTRACT(EPOCH FROM detected_at)::BIGINT,0),
                         COALESCE(root_cause_candidate,''), COALESCE(confidence,0.8), detected_at,
                         COALESCE(phase,'DETECTING'), COALESCE(environment,'prod')
                  FROM incidents
                  WHERE project_id=$1
                    AND ($2 = '' OR service ILIKE '%' || $2 || '%' OR root_cause_candidate ILIKE '%' || $2 || '%' OR id ILIKE '%' || $2 || '%')
                    AND ($3 = '' OR status = $3)
                    AND ($4::timestamptz IS NULL OR detected_at < $4)
                  ORDER BY detected_at DESC
                  LIMIT $5`

        dbRows, qErr := db.Query(ctx, query, projectId, searchQ, statusFilter, cursorTime, limit+1)
        if qErr == nil {
            defer dbRows.Close()
            for dbRows.Next() {
                var id, tenantID, projID, service, status, rootCause, phase, environment string
                var detectedAtEpoch int64
                var confidence float64
                var detectedAtTime time.Time
                if sErr := dbRows.Scan(&id, &tenantID, &projID, &service, &status, &detectedAtEpoch, &rootCause, &confidence, &detectedAtTime, &phase, &environment); sErr == nil {
                    fetched = append(fetched, fetchedRow{
                        item: types.IncidentListItem{
                            ID: id, ProjectID: projID, Service: service,
                            Status: status, DetectedAt: detectedAtEpoch,
                            RootCauseCandidate: rootCause, Confidence: confidence, EvidenceCount: 1,
                            Phase: phase, Environment: environment,
                        },
                        tenantID:   tenantID,
                        detectedAt: detectedAtTime,
                    })
                }
            }
        }

        // Determine hasMore by checking if we fetched one extra row
        if len(fetched) > limit {
            hasMore = true
            fetched = fetched[:limit]
        }
        // nextCursor = detected_at of the last returned row (client sends this for next page)
        if hasMore && len(fetched) > 0 {
            nextCursor = fetched[len(fetched)-1].detectedAt.UTC().Format(time.RFC3339)
        }

        // Apply tenant filter and build output
        for _, row := range fetched {
            if claims != nil && !hasRole(claims, "super_admin") {
                if t, ok := claims["tenant"].(string); ok {
                    if row.tenantID != t {
                        continue
                    }
                }
            }
            out = append(out, row.item)
        }
    } else {
        // Fallback to in-memory (no cursor/search support)
        incidentMu.Lock()
        defer incidentMu.Unlock()
        for _, v := range incidents {
            if v.ProjectID != projectId {
                continue
            }
            if claims != nil && !hasRole(claims, "super_admin") {
                if t, ok := claims["tenant"].(string); ok {
                    if v.TenantID != t {
                        continue
                    }
                }
            }
            out = append(out, types.IncidentListItem{
                ID: v.ID, ProjectID: v.ProjectID, Service: v.Service,
                Status: v.Status, DetectedAt: v.DetectedAt,
                RootCauseCandidate: v.RootCauseCandidate, Confidence: v.Confidence,
                EvidenceCount: len(v.Evidence),
            })
        }
    }
    if out == nil {
        out = []types.IncidentListItem{}
    }
    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(map[string]interface{}{
        "incidents":  out,
        "total":      len(out),
        "hasMore":    hasMore,
        "nextCursor": nextCursor,
    })
}

// analyzeIncidentHandler handles POST /api/v1/projects/{projectId}/incidents/analyze
func analyzeIncidentHandler(w http.ResponseWriter, r *http.Request) {
    parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/projects/"), "/")
    projectId := parts[0]
    claims := GetClaims(r)
    var tenantID string
    if claims != nil {
        if t, ok := claims["tenant"].(string); ok {
            tenantID = t
        }
    }
    incidentID := fmt.Sprintf("inc-%d", time.Now().UnixNano())
    ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
    defer cancel()
    if db := getDB(); db != nil {
        _, _ = db.Exec(ctx,
            `INSERT INTO incidents (id, tenant_id, project_id, service, status, detected_at, root_cause_candidate, confidence)
             VALUES ($1,$2,$3,$4,'OPEN',NOW(),$5,$6) ON CONFLICT DO NOTHING`,
            incidentID, tenantID, projectId, "analysis-triggered", "unknown", 0.5,
        )
    }
    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(map[string]interface{}{"incidentId": incidentID, "status": "OPEN", "projectId": projectId})
}

func signalsHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }
    var s types.Signal
    if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
        http.Error(w, "bad request", http.StatusBadRequest)
        return
    }

    // Detection rule: 5xx errors, timeouts, high error rate, or service down -> incident
    isIncident := false
    if len(s.StatusClass) > 0 && s.StatusClass[0] == '5' {
        isIncident = true
    }
    if s.StatusClass == "timeout" || s.StatusClass == "connection_error" || s.StatusClass == "down" {
        isIncident = true
    }
    if s.ErrorRate > 0.5 {
        isIncident = true
    }

    if isIncident {
        // enforce tenant scoping: token tenant must match signal tenant unless super_admin
        claims := GetClaims(r)
        if claims == nil {
            http.Error(w, "unauthorized", http.StatusUnauthorized)
            return
        }
        if !hasRole(claims, "super_admin") {
            if t, ok := claims["tenant"].(string); ok {
                if t != s.TenantID {
                    http.Error(w, "forbidden - tenant mismatch", http.StatusForbidden)
                    return
                }
            }
        }
        // Publish to Redis for detection engine to consume
        raddr := os.Getenv("REDIS_ADDR")
        if raddr == "" {
            raddr = "redis:6379"
        }
        rdb := redis.NewClient(&redis.Options{Addr: raddr})
        b, _ := json.Marshal(s)
        _ = rdb.Publish(r.Context(), "signals", b).Err()
        _ = rdb.Close()
        
        key := s.TenantID + ":" + s.ProjectID + ":" + s.Service
        incidentMu.Lock()
        inc, ok := incidents[key]
        now := time.Now()
        
        if !ok {
            incidentSeq++
            inc = types.Incident{
                ID:         fmt.Sprintf("inc-%d", incidentSeq),
                TenantID:   s.TenantID,
                ProjectID:  s.ProjectID,
                Service:    s.Service,
                Status:     "OPEN",
                DetectedAt: now.UnixMilli(),
                FirstSeen:  now.Unix(),
                Confidence: 0.8,
                Evidence:   []types.Signal{s},
            }
            incidents[key] = inc
            
            // Save new incident to PostgreSQL
            go func(incident types.Incident) {
                gdb := getDB()
                if gdb == nil {
                    return
                }
                evidenceJSON, _ := json.Marshal(incident.Evidence)
                _, _ = gdb.Exec(context.Background(),
                    "INSERT INTO incidents (id, tenant_id, project_id, service, status, detected_at, evidence) VALUES ($1, $2, $3, $4, $5, $6, $7)",
                    incident.ID, incident.TenantID, incident.ProjectID, incident.Service, incident.Status, incident.DetectedAt, string(evidenceJSON),
                )
            }(inc)
        } else {
            inc.Evidence = append(inc.Evidence, s)
            incidents[key] = inc
            
            // Update incident in PostgreSQL with new evidence
            go func(incident types.Incident) {
                gdb := getDB()
                if gdb == nil {
                    return
                }
                evidenceJSON, _ := json.Marshal(incident.Evidence)
                _, _ = gdb.Exec(context.Background(),
                    "UPDATE incidents SET evidence = $1, detected_at = $2 WHERE id = $3",
                    string(evidenceJSON), incident.DetectedAt, incident.ID,
                )
            }(inc)
        }
        incidentMu.Unlock()
        w.WriteHeader(http.StatusAccepted)
        return
    }

    w.WriteHeader(http.StatusNoContent)
}

// signalHistoryHandler handles GET /api/v1/projects/{id}/services/{svcId}/signal-history?hours=24
func signalHistoryHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }
    // path: /api/v1/projects/{id}/services/{svcId}/signal-history
    parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/projects/"), "/")
    if len(parts) < 4 {
        http.Error(w, "invalid path", http.StatusBadRequest)
        return
    }
    projectID := parts[0]
    svcID := parts[2]

    hoursStr := r.URL.Query().Get("hours")
    hours := 24
    if hoursStr != "" {
        if n, err := strconv.Atoi(hoursStr); err == nil && n > 0 {
            hours = n
        }
    }
    if hours < 1 || hours > 720 {
        hours = 24
    }

    pool := getDB()
    if pool == nil {
        http.Error(w, "database unavailable", http.StatusServiceUnavailable)
        return
    }
    ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
    defer cancel()

    rows, err := pool.Query(ctx,
        `SELECT recorded_at, error_rate, latency_p95, status_class
         FROM signal_history
         WHERE project_id=$1 AND service_id=$2 AND recorded_at > NOW() - ($3::int * INTERVAL '1 hour')
         ORDER BY recorded_at ASC`,
        projectID, svcID, hours)
    if err != nil {
        http.Error(w, "query failed", http.StatusInternalServerError)
        return
    }
    defer rows.Close()

    type timePoint struct {
        Time      string  `json:"time"`
        ErrorRate float64 `json:"errorRate"`
    }
    var timeline []timePoint
    totalCount := 0
    okCount := 0
    latencySum := 0.0

    for rows.Next() {
        var ts time.Time
        var errRate, latencyP95 float64
        var statusClass string
        if err := rows.Scan(&ts, &errRate, &latencyP95, &statusClass); err != nil {
            continue
        }
        timeline = append(timeline, timePoint{Time: ts.UTC().Format(time.RFC3339), ErrorRate: errRate})
        latencySum += latencyP95
        totalCount++
        if statusClass == "2xx" {
            okCount++
        }
    }

    uptime := 0.0
    if totalCount > 0 {
        uptime = float64(okCount) / float64(totalCount) * 100.0
    }
    avgLatency := 0.0
    if totalCount > 0 {
        avgLatency = latencySum / float64(totalCount)
    }
    faultCount := totalCount - okCount
    if timeline == nil {
        timeline = []timePoint{}
    }

    // Compute sloTarget and sloStatus from projects.metadata.sloConfig
    sloTarget := 0.0
    sloStatus := "ok"
    var sloMetaRaw []byte
    if err2 := pool.QueryRow(ctx, `SELECT metadata->'sloConfig'->$1 FROM projects WHERE id=$2`, svcID, projectID).Scan(&sloMetaRaw); err2 == nil && len(sloMetaRaw) > 2 {
        var sloConf struct {
            ErrorRateThreshold float64 `json:"errorRateThreshold"`
        }
        if json.Unmarshal(sloMetaRaw, &sloConf) == nil && sloConf.ErrorRateThreshold > 0 {
            sloTarget = (1.0 - sloConf.ErrorRateThreshold) * 100.0
            if uptime < sloTarget-1.0 {
                sloStatus = "critical"
            } else if uptime < sloTarget {
                sloStatus = "warning"
            }
        }
    }

    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(map[string]interface{}{
        "errorRateTimeline": timeline,
        "avgLatency":        avgLatency,
        "faultCount":        faultCount,
        "uptime":            uptime,
        "p95LatencyMs":      avgLatency,
        "sloTarget":         sloTarget,
        "sloStatus":         sloStatus,
        "projectId":         projectID,
        "serviceId":         svcID,
        "hoursQueried":      hours,
    })
}

// signalHistoryBatchHandler handles:
//   GET /api/v1/projects/{id}/signal-history/batch?services=svc1,svc2,svc3&hours=1
// Returns a map keyed by service_id with errorRateTimeline, avgLatency, faultCount, uptime.
// Uses a single query for all requested services (avoids N+1 problem).
func signalHistoryBatchHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }
    // path: /api/v1/projects/{id}/signal-history/batch
    parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/projects/"), "/")
    if len(parts) < 1 || parts[0] == "" {
        http.Error(w, "invalid path", http.StatusBadRequest)
        return
    }
    projectID := parts[0]

    servicesParam := r.URL.Query().Get("services")
    if servicesParam == "" {
        http.Error(w, "services query param required", http.StatusBadRequest)
        return
    }
    serviceIDs := strings.Split(servicesParam, ",")
    if len(serviceIDs) > 50 {
        serviceIDs = serviceIDs[:50]
    }
    // Trim whitespace from service IDs
    for i, s := range serviceIDs {
        serviceIDs[i] = strings.TrimSpace(s)
    }

    hoursStr := r.URL.Query().Get("hours")
    hours := 1
    if hoursStr != "" {
        if n, err := strconv.Atoi(hoursStr); err == nil && n > 0 {
            hours = n
        }
    }
    if hours < 1 || hours > 720 {
        hours = 1
    }

    pool := getDB()
    if pool == nil {
        http.Error(w, "database unavailable", http.StatusServiceUnavailable)
        return
    }
    ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
    defer cancel()

    rows, err := pool.Query(ctx,
        `SELECT service_id, status_class, error_rate, latency_p95, recorded_at
         FROM signal_history
         WHERE project_id=$1 AND service_id = ANY($2) AND recorded_at > NOW() - ($3::int * INTERVAL '1 hour')
         ORDER BY recorded_at ASC`,
        projectID, serviceIDs, hours)
    if err != nil {
        http.Error(w, "query failed", http.StatusInternalServerError)
        return
    }
    defer rows.Close()

    // Per-service accumulators
    type bucket struct {
        Time      string  `json:"time"`
        ErrorRate float64 `json:"errorRate"`
    }
    type svcAcc struct {
        timeline   []bucket
        totalCount int
        okCount    int
        latencySum float64
        faultCount int
    }
    accs := make(map[string]*svcAcc)
    for _, id := range serviceIDs {
        accs[id] = &svcAcc{}
    }

    // 15-minute bucket grouping
    const bucketSize = 15 * time.Minute
    type bucketKey struct {
        serviceID string
        bucket    int64 // unix seconds of bucket start
    }
    bucketRates := make(map[bucketKey][]float64)

    for rows.Next() {
        var svcID, statusClass string
        var errRate, latencyP95 float64
        var ts time.Time
        if err := rows.Scan(&svcID, &statusClass, &errRate, &latencyP95, &ts); err != nil {
            continue
        }
        acc, ok := accs[svcID]
        if !ok {
            continue
        }
        acc.totalCount++
        acc.latencySum += latencyP95
        if statusClass == "2xx" {
            acc.okCount++
        } else {
            acc.faultCount++
        }
        // Accumulate into 15-min bucket
        bStart := ts.Unix() / int64(bucketSize.Seconds()) * int64(bucketSize.Seconds())
        bk := bucketKey{serviceID: svcID, bucket: bStart}
        bucketRates[bk] = append(bucketRates[bk], errRate)
    }
    rows.Close()

    // Build timelines from buckets
    // Collect unique buckets per service and sort
    type bkEntry struct {
        svcID string
        start int64
        rates []float64
    }
    entries := make([]bkEntry, 0, len(bucketRates))
    for bk, rates := range bucketRates {
        entries = append(entries, bkEntry{svcID: bk.serviceID, start: bk.bucket, rates: rates})
    }
    sort.Slice(entries, func(i, j int) bool {
        if entries[i].svcID != entries[j].svcID {
            return entries[i].svcID < entries[j].svcID
        }
        return entries[i].start < entries[j].start
    })
    for _, e := range entries {
        acc := accs[e.svcID]
        avgRate := 0.0
        for _, r := range e.rates {
            avgRate += r
        }
        if len(e.rates) > 0 {
            avgRate /= float64(len(e.rates))
        }
        acc.timeline = append(acc.timeline, bucket{
            Time:      time.Unix(e.start, 0).UTC().Format(time.RFC3339),
            ErrorRate: avgRate,
        })
    }

    // Query SLO config for all services in this project at once
    type sloConfEntry struct {
        ErrorRateThreshold float64 `json:"errorRateThreshold"`
    }
    sloConfigs := make(map[string]sloConfEntry)
    var sloMetaRaw []byte
    if err2 := pool.QueryRow(ctx, `SELECT COALESCE(metadata->'sloConfig', '{}') FROM projects WHERE id=$1`, projectID).Scan(&sloMetaRaw); err2 == nil {
        var configs map[string]sloConfEntry
        if json.Unmarshal(sloMetaRaw, &configs) == nil {
            sloConfigs = configs
        }
    }

    // Build response map
    type svcResult struct {
        ErrorRateTimeline []bucket `json:"errorRateTimeline"`
        AvgLatency        float64  `json:"avgLatency"`
        FaultCount        int      `json:"faultCount"`
        Uptime            float64  `json:"uptime"`
        SloTarget         float64  `json:"sloTarget,omitempty"`
        SloStatus         string   `json:"sloStatus,omitempty"`
        P95LatencyMs      float64  `json:"p95LatencyMs,omitempty"`
    }
    result := make(map[string]svcResult, len(serviceIDs))
    for _, id := range serviceIDs {
        acc := accs[id]
        avgLatency := 0.0
        if acc.totalCount > 0 {
            avgLatency = acc.latencySum / float64(acc.totalCount)
        }
        uptime := 0.0
        if acc.totalCount > 0 {
            uptime = float64(acc.okCount) / float64(acc.totalCount) * 100.0
        }
        tl := acc.timeline
        if tl == nil {
            tl = []bucket{}
        }
        // Compute sloTarget and sloStatus for this service
        sloTarget := 0.0
        sloStatus := "ok"
        if conf, ok := sloConfigs[id]; ok && conf.ErrorRateThreshold > 0 {
            sloTarget = (1.0 - conf.ErrorRateThreshold) * 100.0
            if uptime < sloTarget-1.0 {
                sloStatus = "critical"
            } else if uptime < sloTarget {
                sloStatus = "warning"
            }
        }
        result[id] = svcResult{
            ErrorRateTimeline: tl,
            AvgLatency:        math.Round(avgLatency*100) / 100,
            FaultCount:        acc.faultCount,
            Uptime:            math.Round(uptime*100) / 100,
            SloTarget:         math.Round(sloTarget*100) / 100,
            SloStatus:         sloStatus,
            P95LatencyMs:      math.Round(avgLatency*100) / 100,
        }
    }

    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(result)
}

// sloConfigHandler handles:
//   PUT /api/v1/projects/{id}/services/{svcId}/slo  — save SLO thresholds for a service
//   GET /api/v1/projects/{id}/slo                   — return all SLO configs for a project
func sloConfigHandler(w http.ResponseWriter, r *http.Request) {
    // Determine projectID and optional svcID from path
    trimmed := strings.TrimPrefix(r.URL.Path, "/api/v1/projects/")
    parts := strings.Split(trimmed, "/")
    projectID := parts[0]

    pool := getDB()
    if pool == nil {
        http.Error(w, "database unavailable", http.StatusServiceUnavailable)
        return
    }
    ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
    defer cancel()

    // GET /api/v1/projects/{id}/slo — return all SLO configs
    if r.Method == http.MethodGet && (len(parts) < 2 || parts[1] == "slo") {
        var metaRaw []byte
        err := pool.QueryRow(ctx, `SELECT COALESCE(metadata->'sloConfig', '{}') FROM projects WHERE id=$1`, projectID).Scan(&metaRaw)
        if err != nil {
            w.Header().Set("Content-Type", "application/json")
            _ = json.NewEncoder(w).Encode(map[string]interface{}{})
            return
        }
        w.Header().Set("Content-Type", "application/json")
        _, _ = w.Write(metaRaw)
        return
    }

    // PUT /api/v1/projects/{id}/services/{svcId}/slo
    if r.Method != http.MethodPut {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }
    if len(parts) < 4 || parts[1] != "services" {
        http.Error(w, "invalid path", http.StatusBadRequest)
        return
    }
    svcID := parts[2]

    var body struct {
        ErrorRateThreshold float64 `json:"errorRateThreshold"`
        LatencyThresholdMs int     `json:"latencyThresholdMs"`
        MinSignalCount     int     `json:"minSignalCount"`
    }
    if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
        http.Error(w, "bad request", http.StatusBadRequest)
        return
    }

    sloJSON, _ := json.Marshal(body)
    _, err := pool.Exec(ctx,
        `UPDATE projects
         SET metadata = jsonb_set(COALESCE(metadata,'{}')::jsonb, ARRAY['sloConfig', $1], $2::jsonb)
         WHERE id=$3`,
        svcID, string(sloJSON), projectID)
    if err != nil {
        http.Error(w, "db update failed: "+err.Error(), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(map[string]interface{}{
        "projectId": projectID,
        "serviceId": svcID,
        "sloConfig": body,
    })
}

func main() {
    // initialize log level
    if l := os.Getenv("LOG_LEVEL"); l != "" {
        logLevel = l
    }

    // initialize global JWKS manager if requested
    if os.Getenv("AUTH_MODE") == "jwks" {
        jwksURL := os.Getenv("JWKS_URL")
        if jwksURL != "" {
            interval := time.Hour
            if s := os.Getenv("JWKS_REFRESH_INTERVAL"); s != "" {
                if d, err := time.ParseDuration(s); err == nil {
                    interval = d
                }
            }
            globalJWKS = NewJWKSManager(jwksURL, interval)
        }
    }

    // Initialize global DB pool before registering routes — all handlers share this pool
    initCtx, initCancel := context.WithTimeout(context.Background(), 10*time.Second)
    if err := initDB(initCtx); err != nil {
        log.Printf("api-gateway: db pool init failed (will use per-request connections): %v", err)
    } else {
        log.Println("api-gateway: global db pool initialized")
    }
    initCancel()

    mux := http.NewServeMux()
    mux.HandleFunc("/health", healthHandler)
    // metrics endpoint
    mux.Handle("/metrics", promhttp.Handler())
    // onboarding status endpoint
    mux.HandleFunc("/api/v1/projects/onboarding/status", onboardingStatusHandler)
    // public projects for home
    mux.HandleFunc("/api/v1/public/projects", publicProjectsHandler)
    // services status for UI
    mux.HandleFunc("/api/services", servicesHandler)
    // Public docs endpoint
    mux.HandleFunc("/api/v1/docs", func(w http.ResponseWriter, r *http.Request) {
        // serve architecture by default for home page
        // try relative paths depending on working dir inside container
            // try container path first (copied in Dockerfile)
            if _, err := os.Stat("/docs/architecture.md"); err == nil {
                http.ServeFile(w, r, "/docs/architecture.md")
                return
            }
            if _, err := os.Stat("/docs/PROJECT_SETUP.md"); err == nil {
                http.ServeFile(w, r, "/docs/PROJECT_SETUP.md")
                return
            }
            // fallback to repo-relative paths (local dev)
            candidates := []string{"./docs/architecture.md", "../../docs/architecture.md", "./docs/PROJECT_SETUP.md", "../../docs/PROJECT_SETUP.md"}
            for _, p := range candidates {
                if _, err := os.Stat(p); err == nil {
                    http.ServeFile(w, r, p)
                    return
                }
            }
            http.Error(w, "docs not found", http.StatusNotFound)
    })
    // Protected endpoints
    mux.Handle("/api/v1/projects", RequireAuth(http.HandlerFunc(projectsHandler)))
    // Tenant project routes — GET open to any authenticated user; mutating ops require tenant_admin or super_admin
    mux.Handle("/api/v1/tenants/", RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        path := strings.Trim(r.URL.Path, "/")
        // LIST projects: any authenticated user can list their own tenant's projects
        if r.Method == http.MethodGet && strings.HasSuffix(path, "projects") {
            listTenantProjectsHandler(w, r)
            return
        }
        // LIST tenant-scoped incidents: GET /api/v1/tenants/{tenant}/incidents
        if r.Method == http.MethodGet && strings.HasSuffix(path, "incidents") {
            listTenantIncidentsHandler(w, r)
            return
        }
        // CREATE project: requires tenant_admin or super_admin
        if r.Method == http.MethodPost && strings.HasSuffix(path, "projects") {
            claims := GetClaims(r)
            if !hasRole(claims, "tenant_admin") && !hasRole(claims, "super_admin") && !hasRole(claims, "project_admin") {
                http.Error(w, "forbidden", http.StatusForbidden)
                return
            }
            createProjectHandler(w, r)
            return
        }
        // Per-project GET: any authenticated user
        parts := strings.Split(path, "/")
        if len(parts) >= 6 && parts[4] == "projects" {
            if r.Method == http.MethodGet {
                getProjectHandler(w, r)
                return
            }
            // Mutating ops require elevation
            claims := GetClaims(r)
            if !hasRole(claims, "tenant_admin") && !hasRole(claims, "super_admin") && !hasRole(claims, "project_admin") {
                http.Error(w, "forbidden", http.StatusForbidden)
                return
            }
            if r.Method == http.MethodPut || r.Method == http.MethodPatch {
                updateProjectHandler(w, r)
                return
            }
            if r.Method == http.MethodDelete {
                deleteProjectHandler(w, r)
                return
            }
        }
        http.NotFound(w, r)
    })))
    // Admin listing (tenant_admin required)
    mux.Handle("/api/v1/admin/projects", RequireAuth(RequireRole("tenant_admin", http.HandlerFunc(listProjectsHandler))))
    // Namespace listing — proxy to graph-manager
    mux.HandleFunc("/api/v1/graphs/namespaces", func(w http.ResponseWriter, r *http.Request) {
        gmURL := os.Getenv("GRAPH_MANAGER_URL")
        if gmURL == "" {
            gmURL = "http://graph-manager:8086"
        }
        resp, err := http.Get(gmURL + "/api/v1/graphs/namespaces")
        if err != nil {
            http.Error(w, err.Error(), http.StatusServiceUnavailable)
            return
        }
        defer resp.Body.Close()
        w.Header().Set("Content-Type", "application/json")
        io.Copy(w, resp.Body)
    })
    mux.Handle("/api/v1/signals", RequireAuth(http.HandlerFunc(signalsHandler)))
    mux.Handle("/api/v1/incidents", RequireAuth(http.HandlerFunc(listIncidentsHandler)))
    
    // Single incident endpoint: /api/v1/incidents/{incidentId}, feedback, and status
    mux.HandleFunc("/api/v1/incidents/", func(w http.ResponseWriter, r *http.Request) {
        parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/incidents/"), "/")
        if len(parts) > 0 && parts[0] != "" {
            if len(parts) >= 2 && parts[1] == "feedback" && r.Method == http.MethodPost {
                RequireAuth(http.HandlerFunc(submitFeedbackHandler)).ServeHTTP(w, r)
                return
            }
            if len(parts) >= 2 && parts[1] == "status" && r.Method == http.MethodPut {
                RequireAuth(http.HandlerFunc(updateIncidentStatusHandler)).ServeHTTP(w, r)
                return
            }
            // SOP endpoints: GET /api/v1/incidents/{id}/sop
            //                PUT /api/v1/incidents/{id}/sop/steps/{order}
            //                GET /api/v1/incidents/{id}/phase
            if len(parts) >= 2 && parts[1] == "sop" {
                RequireAuth(http.HandlerFunc(sopHandler)).ServeHTTP(w, r)
                return
            }
            if len(parts) >= 2 && parts[1] == "phase" && r.Method == http.MethodGet {
                RequireAuth(http.HandlerFunc(incidentPhaseHandler)).ServeHTTP(w, r)
                return
            }
            // GET /api/v1/incidents/{id}/audit — full audit trail with reasoning
            if len(parts) >= 2 && parts[1] == "audit" && r.Method == http.MethodGet {
                RequireAuth(http.HandlerFunc(incidentAuditHandler)).ServeHTTP(w, r)
                return
            }
            // GET /api/v1/incidents/{id}/reasoning — human-readable decision chain
            if len(parts) >= 2 && parts[1] == "reasoning" && r.Method == http.MethodGet {
                RequireAuth(http.HandlerFunc(incidentReasoningHandler)).ServeHTTP(w, r)
                return
            }
            // PATCH /api/v1/incidents/{id}/assign — assign or unassign an incident
            if len(parts) >= 2 && parts[1] == "assign" {
                RequireAuth(http.HandlerFunc(assignIncidentHandler)).ServeHTTP(w, r)
                return
            }
            // GET /api/v1/incidents/{id}/related — find related incidents by root cause
            if len(parts) >= 2 && parts[1] == "related" {
                RequireAuth(http.HandlerFunc(relatedIncidentsHandler)).ServeHTTP(w, r)
                return
            }
            // POST /api/v1/incidents/{id}/rca-feedback — operator confirms/denies root cause
            if len(parts) >= 2 && parts[1] == "rca-feedback" && r.Method == http.MethodPost {
                RequireAuth(http.HandlerFunc(rcaFeedbackHandler)).ServeHTTP(w, r)
                return
            }
            RequireAuth(http.HandlerFunc(getIncidentHandler)).ServeHTTP(w, r)
            return
        }
        http.NotFound(w, r)
    })
    
    // Project-scoped sub-routes: incidents, graphs, status
    mux.HandleFunc("/api/v1/projects/", func(w http.ResponseWriter, r *http.Request) {
        path := r.URL.Path
        if strings.Contains(path, "/incidents") {
            RequireAuth(http.HandlerFunc(projectIncidentsHandler)).ServeHTTP(w, r)
            return
        }
        // Project settings: GET/PUT /api/v1/projects/{id}/settings
        if strings.Contains(path, "/settings") {
            RequireAuth(http.HandlerFunc(projectSettingsHandler)).ServeHTTP(w, r)
            return
        }
        // Code context: GET /api/v1/projects/{id}/services/{svcId}/code-context
        if strings.Contains(path, "/code-context") {
            RequireAuth(http.HandlerFunc(codeContextHandler)).ServeHTTP(w, r)
            return
        }
        // Request path: GET /api/v1/projects/{id}/services/{svcId}/request-path
        // Returns the full traversal from user-facing entry point to this service
        if strings.Contains(path, "/request-path") {
            RequireAuth(http.HandlerFunc(requestPathHandler)).ServeHTTP(w, r)
            return
        }
        // Recent deployments for a service: GET /api/v1/projects/{id}/services/{svcId}/deployments
        if strings.Contains(path, "/services/") && strings.HasSuffix(strings.TrimSuffix(path, "/"), "deployments") {
            RequireAuth(http.HandlerFunc(serviceDeploymentsHandler)).ServeHTTP(w, r)
            return
        }
        // Service map YAML: GET /api/v1/projects/{id}/service-map.yaml
        if strings.HasSuffix(path, "/service-map.yaml") {
            RequireAuth(http.HandlerFunc(generateServiceMapHandler)).ServeHTTP(w, r)
            return
        }
        // Code index status: GET /api/v1/projects/{id}/code-index/status
        if strings.Contains(path, "/code-index/status") {
            RequireAuth(http.HandlerFunc(codeIndexStatusHandler)).ServeHTTP(w, r)
            return
        }
        // Runbook version history (must precede /sop-playbook to avoid prefix match shadowing)
        if strings.Contains(path, "/sop-playbook/history") {
            RequireAuth(http.HandlerFunc(sopPlaybookHistoryHandler)).ServeHTTP(w, r)
            return
        }
        if strings.Contains(path, "/sop-playbook/revert") {
            RequireAuth(http.HandlerFunc(sopPlaybookRevertHandler)).ServeHTTP(w, r)
            return
        }
        // Custom SOP playbook: PUT /api/v1/projects/{id}/services/{svcId}/sop-playbook
        if strings.Contains(path, "/sop-playbook") {
            RequireAuth(http.HandlerFunc(sopPlaybookHandler)).ServeHTTP(w, r)
            return
        }
        // Playbook learning: GET /api/v1/projects/{id}/services/{svcId}/playbook-learning
        if strings.Contains(path, "/playbook-learning") {
            RequireAuth(http.HandlerFunc(playbookLearningHandler)).ServeHTTP(w, r)
            return
        }
        // Batch signal history: GET /api/v1/projects/{id}/signal-history/batch?services=svc1,svc2&hours=1
        if strings.Contains(path, "/signal-history/batch") {
            RequireAuth(http.HandlerFunc(signalHistoryBatchHandler)).ServeHTTP(w, r)
            return
        }
        // Signal history: GET /api/v1/projects/{id}/services/{svcId}/signal-history?hours=24
        if strings.Contains(path, "/signal-history") {
            RequireAuth(http.HandlerFunc(signalHistoryHandler)).ServeHTTP(w, r)
            return
        }
        // SLO config: PUT /api/v1/projects/{id}/services/{svcId}/slo
        //             GET /api/v1/projects/{id}/slo
        if strings.Contains(path, "/slo") {
            RequireAuth(http.HandlerFunc(sloConfigHandler)).ServeHTTP(w, r)
            return
        }
        // Graph snapshot: POST /api/v1/projects/{id}/graphs/snapshot
        if strings.HasSuffix(path, "/graphs/snapshot") && r.Method == http.MethodPost {
            RequireAuth(http.HandlerFunc(graphSnapshotHandler)).ServeHTTP(w, r)
            return
        }
        // Graph diff: GET /api/v1/projects/{id}/graphs/diff
        if strings.HasSuffix(path, "/graphs/diff") {
            RequireAuth(http.HandlerFunc(graphDiffHandler)).ServeHTTP(w, r)
            return
        }
        // Maintenance windows: POST/DELETE /api/v1/projects/{id}/maintenance
        if strings.Contains(path, "/maintenance") {
            RequireAuth(http.HandlerFunc(maintenanceHandler)).ServeHTTP(w, r)
            return
        }
        // graphs, status and other project sub-routes
        projectStatusHandler(w, r)
    })
    
    // Onboarding API - only tenant_admin or super_admin can trigger
    mux.Handle("/api/v1/projects/onboarding/import", RequireAuth(RequireRole("tenant_admin", http.HandlerFunc(onboardingHandler))))

    // Deployment tracking — called from CI/CD pipelines after each deploy
    // /batch must be registered before the catch-all /api/v1/deployments/ handler
    mux.Handle("/api/v1/deployments/batch", RequireAuth(http.HandlerFunc(deploymentsBatchHandler)))
    mux.Handle("/api/v1/deployments", RequireAuth(http.HandlerFunc(deploymentsHandler)))
    mux.HandleFunc("/api/v1/deployments/", func(w http.ResponseWriter, r *http.Request) {
        RequireAuth(http.HandlerFunc(deploymentDetailHandler)).ServeHTTP(w, r)
    })

    // Serve static UI (single page app) from /usr/share/ui if present
    mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        // prefer static files under /usr/share/ui
        fsPath := "/usr/share/ui" + r.URL.Path
        if r.URL.Path == "/" {
            fsPath = "/usr/share/ui/index.html"
        }
        if _, err := os.Stat(fsPath); err == nil {
            http.ServeFile(w, r, fsPath)
            return
        }
        // fallback to index.html for SPA routes
        if _, err := os.Stat("/usr/share/ui/index.html"); err == nil {
            http.ServeFile(w, r, "/usr/share/ui/index.html")
            return
        }
        http.NotFound(w, r)
    })

    // Background goroutine to prevent IP rate limiter map from growing unbounded.
    go func() {
        for range time.Tick(10 * time.Minute) {
            ipLimiterMu.Lock()
            ipLimiters = make(map[string]*rate.Limiter)
            ipLimiterMu.Unlock()
        }
    }()

    addr := ":8080"
    // Wrap the mux with rate limiting — applied to ALL routes
    srv := &http.Server{
        Addr:         addr,
        Handler:      corsWrap(rateLimitMiddleware(mux)),
        ReadTimeout:  5 * time.Second,
        WriteTimeout: 10 * time.Second,
        IdleTimeout:  30 * time.Second,
    }

    // Start server
    go func() {
        log.Printf("api-gateway: starting on %s", srv.Addr)
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Fatalf("listen: %s", err)
        }
    }()

    // Graceful shutdown
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit
    log.Println("api-gateway: shutting down")

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    if err := srv.Shutdown(ctx); err != nil {
        log.Fatalf("api-gateway: forced to shutdown: %v", err)
    }
    log.Println("api-gateway: stopped")
}

// onboardingStatusHandler returns the status of an onboarding job by job_id query param
func onboardingStatusHandler(w http.ResponseWriter, r *http.Request) {
    q := r.URL.Query().Get("job_id")
    if q == "" {
        http.Error(w, "missing job_id", http.StatusBadRequest)
        return
    }
    db := getDB()
    if db == nil {
        http.Error(w, "db unavailable", http.StatusServiceUnavailable)
        return
    }
    ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
    defer cancel()
    var status string
    var created string
    err := db.QueryRow(ctx, "select status, created_at from onboarding_jobs where id=$1", q).Scan(&status, &created)
    if err != nil {
        http.Error(w, "not found", http.StatusNotFound)
        return
    }
    _ = json.NewEncoder(w).Encode(map[string]string{"job_id": q, "status": status, "created_at": created})
}

// onboardingHandler handles onboarding import requests
func onboardingHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }
    var body struct {
        ProjectID string `json:"project_id"`
        Mode      string `json:"mode"`
        Sources   map[string]interface{} `json:"sources"`
    }
    if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
        http.Error(w, "bad request", http.StatusBadRequest)
        return
    }
    // enqueue job into Redis (if available) and return job id
    jobID := fmt.Sprintf("onb-%d", time.Now().UnixNano())
    job := map[string]interface{}{"job_id": jobID, "project_id": body.ProjectID, "mode": body.Mode, "sources": body.Sources}
    b, _ := json.Marshal(job)
    raddr := os.Getenv("REDIS_ADDR")
    if raddr == "" {
        raddr = "redis:6379"
    }
    rdb := redis.NewClient(&redis.Options{Addr: raddr})
    if err := rdb.RPush(r.Context(), "onboarding_jobs", b).Err(); err != nil {
        // fallback to in-memory response but signal accepted
        Errorf("failed to enqueue onboarding job: %v", err)
    }
    w.WriteHeader(http.StatusAccepted)
    _ = json.NewEncoder(w).Encode(map[string]string{"job_id": jobID})
}

// createProjectHandler stores a new project in Postgres
func createProjectHandler(w http.ResponseWriter, r *http.Request) {
    // path: /api/v1/tenants/{tenant}/projects
    pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
    var tenantFromPath string
    if len(pathParts) >= 4 {
        tenantFromPath = pathParts[3]
    }

    var body struct {
        Name         string                   `json:"name"`
        Slug         string                   `json:"slug"`
        TenantID     string                   `json:"tenantId"`
        Domain       string                   `json:"domain"`
        Environments []string                 `json:"environments"`
        Environment  string                   `json:"environment"`
        Services     []map[string]interface{} `json:"services"` // Array of {name, dependencies:[]}
    }
    if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
        http.Error(w, "bad request", http.StatusBadRequest)
        return
    }
    // Use tenant from URL path if body doesn't supply it
    if body.TenantID == "" {
        body.TenantID = tenantFromPath
    }
    if body.Name == "" || body.TenantID == "" {
        http.Error(w, "name and tenantId are required", http.StatusBadRequest)
        return
    }
    // Generate slug if not provided
    if body.Slug == "" {
        body.Slug = strings.ToLower(strings.ReplaceAll(body.Name, " ", "-"))
    }
    // Use first environment or default to "prod"
    env := body.Environment
    if env == "" && len(body.Environments) > 0 {
        env = body.Environments[0]
    }
    if env == "" {
        env = "prod"
    }

    id := fmt.Sprintf("proj-%d", time.Now().UnixNano())
    db := getDB()
    if db == nil {
        http.Error(w, "db unavailable", http.StatusServiceUnavailable)
        return
    }
    ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
    defer cancel()

    // Build metadata with graph state if services are provided
    metadata := map[string]interface{}{
        "graphStatus": "DRAFT",
    }
    if len(body.Services) > 0 {
        metadata["services"] = body.Services
        metadata["graphStatus"] = "READY" // Mark as ready if services provided
    }
    metadataJSON, _ := json.Marshal(metadata)
    
    if _, err := db.Exec(ctx,
        `INSERT INTO projects (id, tenant_id, name, slug, environment, metadata)
         VALUES ($1,$2,$3,$4,$5,$6)`,
        id, body.TenantID, body.Name, body.Slug, env, string(metadataJSON)); err != nil {
        http.Error(w, "db insert failed: "+err.Error(), http.StatusInternalServerError)
        return
    }

    // Create environment records for each specified environment
    envs := body.Environments
    if len(envs) == 0 {
        envs = []string{env}
    }
    type envResp struct {
        ID        string `json:"id"`
        Name      string `json:"name"`
        Namespace string `json:"namespace"`
    }
    var createdEnvs []envResp
    for _, e := range envs {
        eid := fmt.Sprintf("%s-%s", id, e)
        ns := fmt.Sprintf("%s:%s", body.Slug, e)
        _, _ = db.Exec(ctx,
            `INSERT INTO environments (id, project_id, name, graph_namespace)
             VALUES ($1,$2,$3,$4) ON CONFLICT DO NOTHING`,
            eid, id, e, ns)
        createdEnvs = append(createdEnvs, envResp{ID: eid, Name: e, Namespace: ns})
    }

    // If services are provided, write them to graph-manager
    if len(body.Services) > 0 {
        go func() {
            gmURL := os.Getenv("GRAPH_MANAGER_URL")
            if gmURL == "" {
                gmURL = "http://graph-manager:8086"
            }
            for _, svc := range body.Services {
                svcName, _ := svc["name"].(string)
                if svcName == "" {
                    continue
                }
                for _, e := range envs {
                    ns := fmt.Sprintf("%s:%s", body.Slug, e)
                    nodePayload := map[string]interface{}{
                        "id": fmt.Sprintf("svc_%s", strings.ReplaceAll(svcName, "-", "_")),
                        "name": svcName, "type": "SERVICE",
                        "statusClass": "2xx", "latencyP95": 100, "errorRate": 0.0,
                    }
                    nodeBody, _ := json.Marshal(nodePayload)
                    req, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/graphs/nodes?namespace=%s", gmURL, ns), strings.NewReader(string(nodeBody)))
                    req.Header.Set("Content-Type", "application/json")
                    resp, err := http.DefaultClient.Do(req)
                    if err == nil {
                        resp.Body.Close()
                    }
                }
                // Create edges for dependencies
                deps, _ := svc["dependencies"].([]interface{})
                for _, dep := range deps {
                    depName, _ := dep.(string)
                    if depName == "" {
                        continue
                    }
                    for _, e := range envs {
                        ns := fmt.Sprintf("%s:%s", body.Slug, e)
                        edgePayload := map[string]interface{}{
                            "id":   fmt.Sprintf("edge_%s_%s", strings.ReplaceAll(svcName, "-", "_"), strings.ReplaceAll(depName, "-", "_")),
                            "from": fmt.Sprintf("svc_%s", strings.ReplaceAll(svcName, "-", "_")),
                            "to":   fmt.Sprintf("svc_%s", strings.ReplaceAll(depName, "-", "_")),
                            "type": "CALLS", "confidence": 0.9, "successRatio": 0.99,
                        }
                        edgeBody, _ := json.Marshal(edgePayload)
                        req, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/graphs/edges?namespace=%s", gmURL, ns), strings.NewReader(string(edgeBody)))
                        req.Header.Set("Content-Type", "application/json")
                        resp, err := http.DefaultClient.Do(req)
                        if err == nil {
                            resp.Body.Close()
                        }
                    }
                }
            }
            // Update project metadata to PUBLISHED
            if gdb := getDB(); gdb != nil {
                ctx2, cancel2 := context.WithTimeout(context.Background(), 3*time.Second)
                defer cancel2()
                _, _ = gdb.Exec(ctx2, `UPDATE projects SET metadata = jsonb_set(COALESCE(metadata,'{}')::jsonb, '{graphStatus}', '"PUBLISHED"') WHERE id=$1`, id)
            }
        }()
    }

    w.WriteHeader(http.StatusCreated)
    graphStatus := "DRAFT"
    if len(body.Services) > 0 {
        graphStatus = "READY"
    }
    _ = json.NewEncoder(w).Encode(map[string]interface{}{
        "id": id, "tenantId": body.TenantID, "name": body.Name,
        "slug": body.Slug, "graphStatus": graphStatus, "incidentCount": 0,
        "environments": createdEnvs,
        "services": body.Services,
    })
}

// getProjectHandler returns a single project by id
func getProjectHandler(w http.ResponseWriter, r *http.Request) {
    // path: /api/v1/tenants/{tenant}/projects/{id}
    parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
    if len(parts) < 6 {
        http.Error(w, "bad path", http.StatusBadRequest)
        return
    }
    tenant := parts[3]
    projectID := parts[5]

    db := getDB()
    if db == nil {
        http.Error(w, "db unavailable", http.StatusServiceUnavailable)
        return
    }
    ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
    defer cancel()

    var id, tenantID, name, slug, env string
    var created time.Time
    err := db.QueryRow(ctx, "select id, tenant_id, name, slug, environment, created_at from projects where id=$1 and tenant_id=$2", projectID, tenant).Scan(&id, &tenantID, &name, &slug, &env, &created)
    if err != nil {
        http.Error(w, "not found", http.StatusNotFound)
        return
    }
    _ = json.NewEncoder(w).Encode(map[string]interface{}{"id": id, "tenant_id": tenantID, "name": name, "slug": slug, "environment": env, "created_at": created})
}

// updateProjectHandler updates project metadata
func updateProjectHandler(w http.ResponseWriter, r *http.Request) {
    parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
    if len(parts) < 6 {
        http.Error(w, "bad path", http.StatusBadRequest)
        return
    }
    tenant := parts[3]
    projectID := parts[5]

    var body struct {
        Name        string                 `json:"name"`
        Slug        string                 `json:"slug"`
        Environment string                 `json:"environment"`
        Metadata    map[string]interface{} `json:"metadata"`
    }
    if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
        http.Error(w, "bad request", http.StatusBadRequest)
        return
    }
    db := getDB()
    if db == nil {
        http.Error(w, "db unavailable", http.StatusServiceUnavailable)
        return
    }
    ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
    defer cancel()

    // update fields
    _, err := db.Exec(ctx, `UPDATE projects SET name=$1, slug=$2, environment=$3, metadata=$4 WHERE id=$5 AND tenant_id=$6`, body.Name, body.Slug, body.Environment, body.Metadata, projectID, tenant)
    if err != nil {
        http.Error(w, "db update failed", http.StatusInternalServerError)
        return
    }
    _ = json.NewEncoder(w).Encode(map[string]string{"projectId": projectID})
}

// deleteProjectHandler deletes a project
func deleteProjectHandler(w http.ResponseWriter, r *http.Request) {
    parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
    if len(parts) < 6 {
        http.Error(w, "bad path", http.StatusBadRequest)
        return
    }
    tenant := parts[3]
    projectID := parts[5]

    db := getDB()
    if db == nil {
        http.Error(w, "db unavailable", http.StatusServiceUnavailable)
        return
    }
    ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
    defer cancel()

    if _, err := db.Exec(ctx, "DELETE FROM projects WHERE id=$1 AND tenant_id=$2", projectID, tenant); err != nil {
        http.Error(w, "db delete failed", http.StatusInternalServerError)
        return
    }
    w.WriteHeader(http.StatusNoContent)
}

// listProjectsHandler returns projects from Postgres (admin)
func listProjectsHandler(w http.ResponseWriter, r *http.Request) {
    db := getDB()
    if db == nil {
        http.Error(w, "db unavailable", http.StatusServiceUnavailable)
        return
    }
    ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
    defer cancel()
    rows, err := db.Query(ctx, "select id, tenant_id, name, slug, environment, created_at from projects order by created_at desc")
    if err != nil {
        http.Error(w, "db query failed", http.StatusInternalServerError)
        return
    }
    defer rows.Close()
    var out []map[string]interface{}
    for rows.Next() {
        var id, tenant, name, slug, env string
        var created time.Time
        _ = rows.Scan(&id, &tenant, &name, &slug, &env, &created)
        out = append(out, map[string]interface{}{"id": id, "tenant_id": tenant, "name": name, "slug": slug, "environment": env, "created_at": created})
    }
    _ = json.NewEncoder(w).Encode(out)
}

// listTenantProjectsHandler returns projects for a specific tenant enriched with graphStatus + incidentCount
func listTenantProjectsHandler(w http.ResponseWriter, r *http.Request) {
    parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
    if len(parts) < 5 {
        http.Error(w, "bad path", http.StatusBadRequest)
        return
    }
    tenant := parts[3]

    ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
    defer cancel()

    // Use shared pool if available; fall back to per-request connection
    type queryable interface {
        Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
        QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
    }
    var conn queryable
    pool := getDB()
    if pool != nil {
        conn = pool
    } else {
        dsn := os.Getenv("PG_DSN")
        if dsn == "" {
            dsn = "postgres://postgres:postgres@postgres:5432/faultiq?sslmode=disable"
        }
        c, err := pgx.Connect(ctx, dsn)
        if err != nil {
            w.Header().Set("Content-Type", "application/json")
            _ = json.NewEncoder(w).Encode(map[string]interface{}{"projects": []interface{}{}, "total": 0})
            return
        }
        defer c.Close(ctx)
        conn = c
    }

    // Enriched query: count incidents and pull graphStatus from metadata
    query := `
        SELECT
            p.id,
            p.name,
            p.slug,
            COALESCE(p.environment, 'prod') AS env,
            COALESCE(p.metadata->>'graphStatus', 'DRAFT') AS graph_status,
            COALESCE((SELECT COUNT(*) FROM incidents i WHERE i.project_id = p.id AND i.status = 'OPEN'), 0) AS incident_count,
            p.created_at
        FROM projects p
        WHERE p.tenant_id = $1
        ORDER BY p.name ASC, p.environment ASC`
    rows, err := conn.Query(ctx, query, tenant)
    if err != nil {
        http.Error(w, "db query failed", http.StatusInternalServerError)
        return
    }
    defer rows.Close()

    // Group projects by slug to consolidate environments
    type envItem struct {
        ID        string `json:"id"`
        Name      string `json:"name"`
        Namespace string `json:"namespace"`
    }
    type projectItem struct {
        ID            string    `json:"id"`
        Name          string    `json:"name"`
        Slug          string    `json:"slug"`
        GraphStatus   string    `json:"graphStatus"`
        IncidentCount int64     `json:"incidentCount"`
        CreatedAt     string    `json:"createdAt"`
        Environments  []envItem `json:"environments"`
    }

    // slug → consolidated project
    projectBySlug := make(map[string]*projectItem)
    var projectOrder []string

    for rows.Next() {
        var id, name, slug, env, graphStatus string
        var incidentCount int64
        var created time.Time
        if err := rows.Scan(&id, &name, &slug, &env, &graphStatus, &incidentCount, &created); err != nil {
            continue
        }

        if existing, ok := projectBySlug[slug]; ok {
            // Add this environment to existing project
            existing.Environments = append(existing.Environments, envItem{
                ID:        id,
                Name:      env,
                Namespace: fmt.Sprintf("%s:%s", slug, env),
            })
            existing.IncidentCount += incidentCount
            // Use PUBLISHED status if any environment has it
            if graphStatus == "PUBLISHED" {
                existing.GraphStatus = "PUBLISHED"
            }
        } else {
            p := &projectItem{
                ID:            id,
                Name:          name,
                Slug:          slug,
                GraphStatus:   graphStatus,
                IncidentCount: incidentCount,
                CreatedAt:     created.Format(time.RFC3339),
                Environments: []envItem{
                    {ID: id, Name: env, Namespace: fmt.Sprintf("%s:%s", slug, env)},
                },
            }
            projectBySlug[slug] = p
            projectOrder = append(projectOrder, slug)
        }
    }
    rows.Close()

    // Also load environments from the environments table
    envRows, envErr := conn.Query(ctx, `
        SELECT e.id, e.name, e.project_id, COALESCE(e.graph_namespace, '') 
        FROM environments e
        JOIN projects p ON e.project_id = p.id
        WHERE p.tenant_id = $1`, tenant)
    if envErr == nil {
        defer envRows.Close()
        for envRows.Next() {
            var eid, ename, epid, ens string
            if envRows.Scan(&eid, &ename, &epid, &ens) == nil {
                // Find the project this belongs to
                for _, p := range projectBySlug {
                    matchesProject := p.ID == epid
                    if !matchesProject {
                        for _, env := range p.Environments {
                            if env.ID == epid {
                                matchesProject = true
                                break
                            }
                        }
                    }
                    if matchesProject {
                        // Check if this env already exists
                        found := false
                        for _, existing := range p.Environments {
                            if existing.Name == ename {
                                found = true
                                break
                            }
                        }
                        if !found {
                            ns := ens
                            if ns == "" {
                                ns = fmt.Sprintf("%s:%s", p.Slug, ename)
                            }
                            p.Environments = append(p.Environments, envItem{ID: eid, Name: ename, Namespace: ns})
                        }
                        break
                    }
                }
            }
        }
    }

    // Build ordered result
    var projects []projectItem
    for _, slug := range projectOrder {
        projects = append(projects, *projectBySlug[slug])
    }

    if projects == nil {
        projects = []projectItem{}
    }
    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(map[string]interface{}{"projects": projects, "total": len(projects)})
}

// submitFeedbackHandler handles POST /api/v1/incidents/{id}/feedback
func submitFeedbackHandler(w http.ResponseWriter, r *http.Request) {
    parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/incidents/"), "/")
    if len(parts) < 1 {
        http.Error(w, "bad path", http.StatusBadRequest)
        return
    }
    incidentID := parts[0]

    var body struct {
        ConfirmedRootCause string `json:"confirmedRootCause"`
        OperatorNote       string `json:"operatorNote"`
        Resolution         string `json:"resolution"`
        ResolvedAt         string `json:"resolvedAt"`
    }
    if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
        http.Error(w, "bad request", http.StatusBadRequest)
        return
    }
    db := getDB()
    if db == nil {
        http.Error(w, "db unavailable", http.StatusServiceUnavailable)
        return
    }
    ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
    defer cancel()

    rootCause := body.ConfirmedRootCause
    if rootCause == "" {
        rootCause = "unknown"
    }
    _, _ = db.Exec(ctx,
        `UPDATE incidents SET status='RESOLVED', root_cause_candidate=$1, resolved_at=NOW(), updated_at=NOW() WHERE id=$2`,
        rootCause, incidentID,
    )
    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(map[string]interface{}{
        "incidentId": incidentID, "status": "RESOLVED", "feedbackRecorded": true,
    })
}

// rcaFeedbackHandler handles POST /api/v1/incidents/{id}/rca-feedback.
// Body: {"wasCorrect": true} or {"wasCorrect": false, "actualRoot": "svc_db", "note": "was the DB"}
// Inserts a row into rca_feedback and, when the prediction was wrong, updates
// incidents.root_cause_candidate to the operator-supplied actual root.
func rcaFeedbackHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }
    parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/incidents/"), "/")
    if len(parts) < 1 || parts[0] == "" {
        http.Error(w, "invalid path", http.StatusBadRequest)
        return
    }
    incidentID := parts[0]

    var body struct {
        WasCorrect bool   `json:"wasCorrect"`
        ActualRoot string `json:"actualRoot"`
        Note       string `json:"note"`
    }
    if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
        http.Error(w, "bad request", http.StatusBadRequest)
        return
    }

    claims := GetClaims(r)
    createdBy := "operator"
    if claims != nil {
        if email, ok := claims["email"].(string); ok && email != "" {
            createdBy = email
        }
    }

    db := getDB()
    if db == nil {
        http.Error(w, "db unavailable", http.StatusServiceUnavailable)
        return
    }
    ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
    defer cancel()

    // Retrieve the current predicted root cause from the incident row.
    var predictedRoot string
    _ = db.QueryRow(ctx,
        "SELECT COALESCE(root_cause_candidate,'') FROM incidents WHERE id=$1",
        incidentID,
    ).Scan(&predictedRoot)

    var actualRootArg interface{}
    if body.ActualRoot != "" {
        actualRootArg = body.ActualRoot
    }

    feedbackID := fmt.Sprintf("rcafb-%d", time.Now().UnixNano())
    _, err := db.Exec(ctx,
        `INSERT INTO rca_feedback (id, incident_id, predicted_root, actual_root, was_correct, operator_note, created_by)
         VALUES ($1, $2, $3, $4, $5, $6, $7)`,
        feedbackID, incidentID, predictedRoot, actualRootArg, body.WasCorrect, body.Note, createdBy,
    )
    if err != nil {
        Errorf("rcaFeedbackHandler: insert failed: %v", err)
        http.Error(w, "db error", http.StatusInternalServerError)
        return
    }

    // When the prediction was wrong, update the incident's root_cause_candidate.
    if !body.WasCorrect && body.ActualRoot != "" {
        _, _ = db.Exec(ctx,
            "UPDATE incidents SET root_cause_candidate=$1, updated_at=NOW() WHERE id=$2",
            body.ActualRoot, incidentID,
        )
    }

    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(map[string]interface{}{
        "feedbackId": feedbackID,
        "incidentId": incidentID,
        "wasCorrect": body.WasCorrect,
    })
}

// updateIncidentStatusHandler handles PUT /api/v1/incidents/{id}/status
func updateIncidentStatusHandler(w http.ResponseWriter, r *http.Request) {
    parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/incidents/"), "/")
    if len(parts) < 1 {
        http.Error(w, "bad path", http.StatusBadRequest)
        return
    }
    incidentID := parts[0]

    var body struct {
        Status string `json:"status"`
    }
    if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
        http.Error(w, "bad request", http.StatusBadRequest)
        return
    }

    // Validate status value
    validStatuses := map[string]bool{"OPEN": true, "ACKNOWLEDGED": true, "RESOLVED": true}
    if !validStatuses[body.Status] {
        http.Error(w, "invalid status: must be OPEN, ACKNOWLEDGED, or RESOLVED", http.StatusBadRequest)
        return
    }

    db := getDB()
    if db == nil {
        http.Error(w, "db unavailable", http.StatusServiceUnavailable)
        return
    }
    ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
    defer cancel()

    _, err := db.Exec(ctx,
        `UPDATE incidents SET status=$1, updated_at=NOW(),
         resolved_at = CASE WHEN $1 = 'RESOLVED' THEN NOW() ELSE resolved_at END
         WHERE id=$2`,
        body.Status, incidentID,
    )
    if err != nil {
        http.Error(w, "update failed", http.StatusInternalServerError)
        return
    }

    // Also update in-memory store
    incidentMu.Lock()
    if inc, ok := incidents[incidentID]; ok {
        inc.Status = body.Status
        incidents[incidentID] = inc
    }
    incidentMu.Unlock()

    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(map[string]interface{}{
        "incidentId": incidentID,
        "status":     body.Status,
        "updatedAt":  time.Now().UTC().Format(time.RFC3339),
    })
}

// getActorFromRequest extracts the authenticated user's subject claim, falling back to "operator".
func getActorFromRequest(r *http.Request) string {
    claims := GetClaims(r)
    if claims == nil {
        return "operator"
    }
    if sub, ok := claims["sub"].(string); ok && sub != "" {
        return sub
    }
    if email, ok := claims["email"].(string); ok && email != "" {
        return email
    }
    return "operator"
}

// ─────────────────────────────────────────────────────────────────────────────
// SOP Handlers
// ─────────────────────────────────────────────────────────────────────────────

// sopHandler handles:
//   GET  /api/v1/incidents/{id}/sop           → list SOP steps
//   PUT  /api/v1/incidents/{id}/sop/steps/{n} → mark step done/failed
func sopHandler(w http.ResponseWriter, r *http.Request) {
    trimmed := strings.TrimPrefix(r.URL.Path, "/api/v1/incidents/")
    parts := strings.Split(trimmed, "/")
    if len(parts) < 1 {
        http.Error(w, "bad path", http.StatusBadRequest)
        return
    }
    incidentID := parts[0]

    db := getDB()
    if db == nil {
        http.Error(w, "db unavailable", http.StatusServiceUnavailable)
        return
    }
    ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
    defer cancel()

    // GET /sop — return current SOP steps from recommendations table
    if r.Method == http.MethodGet && (len(parts) == 2 && parts[1] == "sop") {
        var sopSteps []byte
        var phase, service string
        err := db.QueryRow(ctx,
            `SELECT COALESCE(r.sop_steps, '[]'), COALESCE(i.phase, 'DETECTING'), COALESCE(i.service, '')
             FROM incidents i
             LEFT JOIN recommendations r ON r.incident_id = i.id AND r.rank = 1
             WHERE i.id = $1`, incidentID).Scan(&sopSteps, &phase, &service)
        if err != nil {
            w.Header().Set("Content-Type", "application/json")
            _ = json.NewEncoder(w).Encode(map[string]interface{}{
                "incidentId": incidentID, "phase": "DETECTING", "steps": []interface{}{},
            })
            return
        }

        // If sop_steps is empty, fall back to custom playbook from project metadata
        var stepsToReturn json.RawMessage
        if string(sopSteps) == "[]" || len(sopSteps) == 0 || string(sopSteps) == "null" {
            // Look up project_id from the incident, then check metadata.customPlaybooks
            var projectID string
            _ = db.QueryRow(ctx, `SELECT COALESCE(project_id,'') FROM incidents WHERE id=$1`, incidentID).Scan(&projectID)
            if projectID != "" {
                var metaRaw []byte
                _ = db.QueryRow(ctx, `SELECT metadata->'customPlaybooks' FROM projects WHERE id=$1`, projectID).Scan(&metaRaw)
                if len(metaRaw) > 2 {
                    var customPlaybooks map[string]json.RawMessage
                    if json.Unmarshal(metaRaw, &customPlaybooks) == nil {
                        if steps, ok := customPlaybooks[service]; ok {
                            stepsToReturn = steps
                        }
                    }
                }
            }
        }
        if stepsToReturn == nil {
            stepsToReturn = json.RawMessage(sopSteps)
        }

        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(map[string]interface{}{
            "incidentId": incidentID,
            "phase":      phase,
            "service":    service,
            "steps":      stepsToReturn,
        })
        return
    }

    // PUT /sop/steps/{order} — mark step done or failed
    if r.Method == http.MethodPut && len(parts) >= 4 && parts[1] == "sop" && parts[2] == "steps" {
        stepOrder := parts[3]
        var body struct {
            Status string `json:"status"` // DONE or FAILED
        }
        if err := json.NewDecoder(r.Body).Decode(&body); err != nil || (body.Status != "DONE" && body.Status != "FAILED") {
            http.Error(w, "status must be DONE or FAILED", http.StatusBadRequest)
            return
        }

        // Load current SOP steps
        var sopSteps json.RawMessage
        var currentPhase, projectID, tenantID string
        err := db.QueryRow(ctx,
            `SELECT COALESCE(r.sop_steps, '[]'), COALESCE(i.phase,'DETECTING'),
                    COALESCE(i.project_id,''), COALESCE(i.tenant_id,'')
             FROM incidents i
             LEFT JOIN recommendations r ON r.incident_id = i.id AND r.rank = 1
             WHERE i.id = $1`, incidentID).Scan(&sopSteps, &currentPhase, &projectID, &tenantID)
        if err != nil {
            http.Error(w, "incident not found", http.StatusNotFound)
            return
        }

        // Parse and update the matching step
        var steps []map[string]interface{}
        if err := json.Unmarshal(sopSteps, &steps); err != nil {
            steps = []map[string]interface{}{}
        }
        updated := false
        nextStepActivated := false
        for i, step := range steps {
            if fmt.Sprintf("%v", step["order"]) == stepOrder {
                steps[i]["status"] = body.Status
                updated = true
                // Activate the next step if this one is DONE
                if body.Status == "DONE" && i+1 < len(steps) {
                    steps[i+1]["status"] = "ACTIVE"
                    nextStepActivated = true
                }
                break
            }
        }
        if !updated {
            http.Error(w, "step not found", http.StatusNotFound)
            return
        }

        newSteps, _ := json.Marshal(steps)
        _, _ = db.Exec(ctx,
            `UPDATE recommendations SET sop_steps = $1 WHERE incident_id = $2 AND rank = 1`,
            string(newSteps), incidentID)

        // Phase transitions triggered by step completion
        if body.Status == "DONE" {
            redisAddr := os.Getenv("REDIS_ADDR")
            if redisAddr == "" {
                redisAddr = "redis:6379"
            }
            rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
            defer rdb.Close()

            switch currentPhase {
            case "TRIAGING":
                // Operator acknowledged first step → advance to FIXING
                _, _ = db.Exec(ctx,
                    `UPDATE incidents SET phase = 'FIXING', status = 'ACKNOWLEDGED', updated_at = NOW() WHERE id = $1`,
                    incidentID)
                // Audit
                stepOrderInt := 0
                fmt.Sscanf(stepOrder, "%d", &stepOrderInt)
                _, _ = db.Exec(ctx,
                    `INSERT INTO sop_audit (incident_id, step_order, action, actor, phase_from, phase_to)
                     VALUES ($1, $2, 'step_done', $3, 'TRIAGING', 'FIXING') ON CONFLICT DO NOTHING`,
                    incidentID, stepOrderInt, getActorFromRequest(r))
                event := map[string]interface{}{
                    "type": "incident_phase_changed", "incidentId": incidentID,
                    "from": "TRIAGING", "to": "FIXING", "projectId": projectID, "tenantId": tenantID,
                }
                b, _ := json.Marshal(event)
                _ = rdb.Publish(ctx, "incident_events", b).Err()
            case "FIXING", "NARROWING", "CONFIRMED":
                // Step done, check if this step has autoVerify
                var autoVerify bool
                for _, step := range steps {
                    if fmt.Sprintf("%v", step["order"]) == stepOrder {
                        if av, ok := step["autoVerify"].(bool); ok {
                            autoVerify = av
                        }
                        break
                    }
                }
                if autoVerify {
                    // Store expectedSignal condition so detection-engine can check it precisely
                    var expectedSig string
                    for _, step := range steps {
                        if fmt.Sprintf("%v", step["order"]) == stepOrder {
                            if es, ok := step["expectedSignal"].(string); ok {
                                expectedSig = es
                            }
                            break
                        }
                    }
                    stepOrderInt := 0
                    fmt.Sscanf(stepOrder, "%d", &stepOrderInt)
                    _, _ = db.Exec(ctx,
                        `UPDATE incidents SET phase = 'VERIFYING', updated_at = NOW(),
                         expected_signal = $2, verifying_step_order = $3 WHERE id = $1`,
                        incidentID, expectedSig, stepOrderInt)
                    // Audit
                    _, _ = db.Exec(ctx,
                        `INSERT INTO sop_audit (incident_id, step_order, action, actor, phase_from, phase_to)
                         VALUES ($1, $2, 'step_done', $3, 'FIXING', 'VERIFYING')
                         ON CONFLICT DO NOTHING`,
                        incidentID, stepOrderInt, getActorFromRequest(r))
                    event := map[string]interface{}{
                        "type": "incident_phase_changed", "incidentId": incidentID,
                        "from": "FIXING", "to": "VERIFYING",
                        "expectedSignal": expectedSig,
                        "projectId": projectID, "tenantId": tenantID,
                    }
                    b, _ := json.Marshal(event)
                    _ = rdb.Publish(ctx, "incident_events", b).Err()
                } else if !nextStepActivated {
                    // All steps done, no more auto-verify — resolve
                    _, _ = db.Exec(ctx,
                        `UPDATE incidents SET phase = 'RESOLVED', status = 'RESOLVED', resolved_at = NOW(), updated_at = NOW() WHERE id = $1`,
                        incidentID)
                    event := map[string]interface{}{
                        "type": "incident_resolved", "incidentId": incidentID,
                        "projectId": projectID, "tenantId": tenantID, "phase": "RESOLVED",
                    }
                    b, _ := json.Marshal(event)
                    _ = rdb.Publish(ctx, "incident_events", b).Err()
                }
            }
        }

        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(map[string]interface{}{
            "incidentId": incidentID,
            "stepOrder":  stepOrder,
            "status":     body.Status,
            "steps":      json.RawMessage(newSteps),
        })
        return
    }

    http.NotFound(w, r)
}

// incidentPhaseHandler handles GET /api/v1/incidents/{id}/phase
func incidentPhaseHandler(w http.ResponseWriter, r *http.Request) {
    parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/incidents/"), "/")
    if len(parts) < 1 {
        http.Error(w, "bad path", http.StatusBadRequest)
        return
    }
    incidentID := parts[0]

    db := getDB()
    if db == nil {
        http.Error(w, "db unavailable", http.StatusServiceUnavailable)
        return
    }
    ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
    defer cancel()

    var phase, status, service string
    var detectedAt, updatedAt time.Time
    err := db.QueryRow(ctx,
        `SELECT COALESCE(phase,'DETECTING'), COALESCE(status,'OPEN'), COALESCE(service,''),
                detected_at, COALESCE(updated_at, detected_at)
         FROM incidents WHERE id = $1`, incidentID).
        Scan(&phase, &status, &service, &detectedAt, &updatedAt)
    if err != nil {
        http.Error(w, "incident not found", http.StatusNotFound)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(map[string]interface{}{
        "incidentId": incidentID,
        "phase":      phase,
        "status":     status,
        "service":    service,
        "detectedAt": detectedAt.UTC().Format(time.RFC3339),
        "updatedAt":  updatedAt.UTC().Format(time.RFC3339),
    })
}

// listTenantIncidentsHandler handles GET /api/v1/tenants/{tenant}/incidents
func listTenantIncidentsHandler(w http.ResponseWriter, r *http.Request) {
    parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
    if len(parts) < 4 {
        http.Error(w, "bad path", http.StatusBadRequest)
        return
    }
    tenant := parts[3]

    limit := 20
    statusFilter := r.URL.Query().Get("status")

    db := getDB()
    if db == nil {
        http.Error(w, "db unavailable", http.StatusServiceUnavailable)
        return
    }
    ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
    defer cancel()

    var rows pgx.Rows
    var err error
    if statusFilter != "" {
        rows, err = db.Query(ctx,
            `SELECT id, COALESCE(service,''), COALESCE(status,'OPEN'), COALESCE(phase,'DETECTING'),
                    COALESCE(project_id,''), COALESCE(root_cause_candidate,''), COALESCE(confidence,0),
                    detected_at
             FROM incidents WHERE tenant_id = $1 AND status = $2
             ORDER BY detected_at DESC LIMIT $3`,
            tenant, statusFilter, limit)
    } else {
        rows, err = db.Query(ctx,
            `SELECT id, COALESCE(service,''), COALESCE(status,'OPEN'), COALESCE(phase,'DETECTING'),
                    COALESCE(project_id,''), COALESCE(root_cause_candidate,''), COALESCE(confidence,0),
                    detected_at, COALESCE(environment,'prod')
             FROM incidents WHERE tenant_id = $1
             ORDER BY detected_at DESC LIMIT $2`,
            tenant, limit)
    }
    if err != nil {
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(map[string]interface{}{"incidents": []interface{}{}, "total": 0})
        return
    }
    defer rows.Close()

    type incRow struct {
        ID                  string  `json:"id"`
        Service             string  `json:"service"`
        Status              string  `json:"status"`
        Phase               string  `json:"phase"`
        ProjectID           string  `json:"projectId"`
        RootCauseCandidate  string  `json:"rootCauseCandidate"`
        Confidence          float64 `json:"confidence"`
        DetectedAt          string  `json:"detectedAt"`
        Environment         string  `json:"environment"`
    }
    var result []incRow
    for rows.Next() {
        var inc incRow
        var detectedAt time.Time
        if err := rows.Scan(&inc.ID, &inc.Service, &inc.Status, &inc.Phase,
            &inc.ProjectID, &inc.RootCauseCandidate, &inc.Confidence, &detectedAt, &inc.Environment); err == nil {
            inc.DetectedAt = detectedAt.UTC().Format(time.RFC3339)
            result = append(result, inc)
        }
    }
    if result == nil {
        result = []incRow{}
    }
    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(map[string]interface{}{"incidents": result, "total": len(result)})
}

// ─────────────────────────────────────────────────────────────────────────────
// Project Settings — repo config for code-aware RCA
// ─────────────────────────────────────────────────────────────────────────────

type ServiceRepoConfig struct {
    ServiceID   string `json:"serviceId"`
    RepoURL     string `json:"repoUrl"`
    Branch      string `json:"branch,omitempty"`
    CodePath    string `json:"codePath,omitempty"`
    SignalTag   string `json:"signalTag,omitempty"`
    LastIndexed string `json:"lastIndexed,omitempty"`
}

// projectSettingsHandler handles GET/PUT /api/v1/projects/{id}/settings
func projectSettingsHandler(w http.ResponseWriter, r *http.Request) {
    parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/projects/"), "/")
    if len(parts) < 1 {
        http.Error(w, "bad path", http.StatusBadRequest)
        return
    }
    projectID := parts[0]

    db := getDB()
    if db == nil {
        http.Error(w, "db unavailable", http.StatusServiceUnavailable)
        return
    }
    ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
    defer cancel()

    if r.Method == http.MethodGet {
        // Read settings from projects.metadata
        var metaRaw []byte
        err := db.QueryRow(ctx, `SELECT COALESCE(metadata, '{}') FROM projects WHERE id=$1`, projectID).Scan(&metaRaw)
        if err != nil {
            w.Header().Set("Content-Type", "application/json")
            _ = json.NewEncoder(w).Encode(map[string]interface{}{
                "projectId": projectID, "services": []interface{}{},
            })
            return
        }
        var meta map[string]interface{}
        if json.Unmarshal(metaRaw, &meta) != nil {
            meta = map[string]interface{}{}
        }
        services := meta["serviceRepos"]
        if services == nil {
            services = []interface{}{}
        }
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(map[string]interface{}{
            "projectId": projectID,
            "services":  services,
        })
        return
    }

    if r.Method == http.MethodPut {
        var body struct {
            Services []ServiceRepoConfig `json:"services"`
        }
        if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
            http.Error(w, "bad json", http.StatusBadRequest)
            return
        }
        servicesJSON, _ := json.Marshal(body.Services)
        _, err := db.Exec(ctx,
            `UPDATE projects SET metadata = jsonb_set(
                COALESCE(metadata, '{}'),
                '{serviceRepos}', $1::jsonb
            ) WHERE id = $2`,
            string(servicesJSON), projectID)
        if err != nil {
            http.Error(w, "update failed", http.StatusInternalServerError)
            return
        }
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(map[string]interface{}{
            "projectId": projectID,
            "services":  body.Services,
        })
        return
    }
    http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

// faultKeywords maps fault types to keywords that indicate a commit may be related.
var faultKeywords = map[string][]string{
    "latency-anomaly":   {"timeout", "retry", "slow", "performance", "cache", "query", "latency", "pool", "connection", "optimize", "index", "n+1"},
    "timeout-burst":     {"timeout", "deadline", "retry", "connection", "pool", "circuit", "slow"},
    "high-error-rate":   {"fix", "bug", "error", "exception", "panic", "crash", "failure", "revert"},
    "service-down":      {"crash", "oom", "memory", "restart", "kill", "exit", "sigkill", "core"},
    "cascading-failure": {"dependency", "downstream", "circuit", "bulkhead", "fallback"},
}

// commitRelevanceScore scores a commit by keyword overlap with the fault type and recency.
func commitRelevanceScore(message, faultType string, ageHours float64) float64 {
    lower := strings.ToLower(message)
    keywords := faultKeywords[faultType]
    if len(keywords) == 0 {
        keywords = faultKeywords["high-error-rate"] // default
    }
    keywordScore := 0.0
    for _, kw := range keywords {
        if strings.Contains(lower, kw) {
            keywordScore += 1.0 / float64(len(keywords))
        }
    }
    // Recency bonus: commits < 2h ago get full recency, decays to 0 at 48h
    recencyScore := math.Max(0, 1.0-ageHours/48.0)
    return keywordScore*0.6 + recencyScore*0.4
}

// codeContextHandler handles GET /api/v1/projects/{id}/services/{svcId}/code-context
// Returns recent git commits for a service if a repo is configured.
// In MVP this is a placeholder — the code-indexer service will populate Neo4j
// with real CodeFunction + GitCommit nodes.
func codeContextHandler(w http.ResponseWriter, r *http.Request) {
    parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/projects/"), "/")
    // parts: [projectId, "services", serviceId, "code-context"]
    if len(parts) < 3 {
        http.Error(w, "bad path", http.StatusBadRequest)
        return
    }
    projectID := parts[0]
    serviceID := parts[2]

    db := getDB()
    if db == nil {
        http.Error(w, "db unavailable", http.StatusServiceUnavailable)
        return
    }
    ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
    defer cancel()

    // Read repo config for this service from project metadata
    var metaRaw []byte
    _ = db.QueryRow(ctx, `SELECT COALESCE(metadata, '{}') FROM projects WHERE id=$1`, projectID).Scan(&metaRaw)
    var meta map[string]json.RawMessage
    _ = json.Unmarshal(metaRaw, &meta)

    var repos []ServiceRepoConfig
    if reposRaw, ok := meta["serviceRepos"]; ok {
        _ = json.Unmarshal(reposRaw, &repos)
    }

    repoURL := ""
    for _, r := range repos {
        if r.ServiceID == serviceID {
            repoURL = r.RepoURL
            break
        }
    }

    // Query graph-manager for GITCOMMIT nodes linked to this service via IMPLEMENTS
    // These are populated by the code-indexer service after each git index run.
    type commitOut struct {
        Hash         string   `json:"hash"`
        Message      string   `json:"message"`
        Author       string   `json:"author"`
        Timestamp    string   `json:"timestamp"`
        ChangedFiles []string `json:"changedFiles"`
    }
    var recentCommits []commitOut
    var suspectFunctions []string

    gmURL := os.Getenv("GRAPH_MANAGER_URL")
    if gmURL == "" {
        gmURL = "http://graph-manager:8086"
    }
    // Resolve project namespace from slug
    var slug string
    _ = db.QueryRow(ctx, `SELECT COALESCE(slug,'') FROM projects WHERE id=$1`, projectID).Scan(&slug)
    namespace := slug + ":prod"

    // Fetch graph nodes for this namespace and filter GITCOMMIT nodes for this service
    graphURL := fmt.Sprintf("%s/api/v1/graphs?namespace=%s", gmURL, url.QueryEscape(namespace))
    if graphResp, err := http.Get(graphURL); err == nil { //nolint:noctx
        defer graphResp.Body.Close()
        var graph struct {
            Nodes map[string]struct {
                ID         string   `json:"id"`
                Name       string   `json:"name"`
                Type       string   `json:"type"`
                ServiceID  string   `json:"serviceId"`
                CommitHash string   `json:"commitHash"`
                Author     string   `json:"author"`
                Message    string   `json:"message"`
                Timestamp  string   `json:"timestamp"`
                File       string   `json:"file"`
            } `json:"nodes"`
        }
        if json.NewDecoder(graphResp.Body).Decode(&graph) == nil {
            // Collect GITCOMMIT nodes for this service (deduplicated by hash)
            seenHash := map[string]bool{}
            seenFn := map[string]bool{}
            for _, n := range graph.Nodes {
                if n.ServiceID != serviceID {
                    continue
                }
                if n.Type == "GITCOMMIT" && n.CommitHash != "" && !seenHash[n.CommitHash] {
                    seenHash[n.CommitHash] = true
                    recentCommits = append(recentCommits, commitOut{
                        Hash:      n.CommitHash,
                        Message:   n.Message,
                        Author:    n.Author,
                        Timestamp: n.Timestamp,
                    })
                }
                if n.Type == "CODEFUNCTION" && n.File != "" && !seenFn[n.Name] {
                    seenFn[n.Name] = true
                    suspectFunctions = append(suspectFunctions, fmt.Sprintf("%s (%s)", n.Name, n.File))
                }
            }
        }
    }

    if recentCommits == nil {
        recentCommits = []commitOut{}
    }
    if suspectFunctions == nil {
        suspectFunctions = []string{}
    }

    // Sort commits by relevance to the fault type being investigated
    faultType := r.URL.Query().Get("faultType")
    if faultType != "" && len(recentCommits) > 0 {
        now := time.Now()
        type scoredCommit struct {
            commit commitOut
            score  float64
        }
        scored := make([]scoredCommit, len(recentCommits))
        for i, c := range recentCommits {
            ageHours := 0.0
            if t, err := time.Parse(time.RFC3339, c.Timestamp); err == nil {
                ageHours = now.Sub(t).Hours()
            }
            scored[i] = scoredCommit{commit: c, score: commitRelevanceScore(c.Message, faultType, ageHours)}
        }
        sort.Slice(scored, func(i, j int) bool {
            return scored[i].score > scored[j].score
        })
        for i, sc := range scored {
            recentCommits[i] = sc.commit
        }
    }

    summary := ""
    switch {
    case repoURL == "":
        summary = "No repository configured — add a repo URL in Project Settings to enable code-level RCA"
    case len(recentCommits) == 0:
        summary = fmt.Sprintf("Repository %s configured — code-indexer will populate commits after next run (interval: 60 min, or POST /index to trigger now)", repoURL)
    default:
        summary = fmt.Sprintf("%d recent commits indexed from %s — suspect functions highlighted", len(recentCommits), repoURL)
    }

    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(map[string]interface{}{
        "serviceId":        serviceID,
        "serviceName":      serviceID,
        "repoUrl":          repoURL,
        "recentCommits":    recentCommits,
        "suspectFunctions": suspectFunctions,
        "summary":          summary,
    })
}

// ─────────────────────────────────────────────────────────────────────────────
// Service-map YAML generation
// ─────────────────────────────────────────────────────────────────────────────

// yamlServiceEntry mirrors the structure of scripts/service-map.yaml service entries.
type yamlServiceEntry struct {
    ID       string   `yaml:"id"`
    Name     string   `yaml:"name"`
    Type     string   `yaml:"type,omitempty"`
    Repo     string   `yaml:"repo,omitempty"`
    Branch   string   `yaml:"branch,omitempty"`
    CodePath string   `yaml:"codePath,omitempty"`
    Calls    []string `yaml:"calls,omitempty"`
}

type yamlProjectEntry struct {
    ID          string             `yaml:"id"`
    Name        string             `yaml:"name"`
    Namespace   string             `yaml:"namespace"`
    Tenant      string             `yaml:"tenant"`
    Environment string             `yaml:"environment"`
    Services    []yamlServiceEntry `yaml:"services"`
}

type yamlServiceMap struct {
    Version  string             `yaml:"version"`
    Projects []yamlProjectEntry `yaml:"projects"`
}

// generateServiceMapHandler handles GET /api/v1/projects/{id}/service-map.yaml
// Reads project + graph nodes + repo configs and returns a complete service-map.yaml.
func generateServiceMapHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }
    parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/projects/"), "/")
    if len(parts) < 1 || parts[0] == "" {
        http.Error(w, "bad path", http.StatusBadRequest)
        return
    }
    projectID := parts[0]

    db := getDB()
    if db == nil {
        http.Error(w, "db unavailable", http.StatusServiceUnavailable)
        return
    }
    ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
    defer cancel()

    // 1. Read project from Postgres
    var name, slug, tenant, environment string
    err := db.QueryRow(ctx,
        `SELECT COALESCE(name,''), COALESCE(slug,''), COALESCE(tenant_id,''), COALESCE(environment,'prod')
         FROM projects WHERE id=$1`, projectID).
        Scan(&name, &slug, &tenant, &environment)
    if err != nil {
        http.Error(w, "project not found", http.StatusNotFound)
        return
    }
    namespace := fmt.Sprintf("%s:%s", slug, environment)

    // 2. Read service repo configs from metadata.serviceRepos
    var metaRaw []byte
    _ = db.QueryRow(ctx, `SELECT COALESCE(metadata, '{}') FROM projects WHERE id=$1`, projectID).Scan(&metaRaw)
    var meta map[string]json.RawMessage
    _ = json.Unmarshal(metaRaw, &meta)
    var serviceRepos []ServiceRepoConfig
    if reposRaw, ok := meta["serviceRepos"]; ok {
        _ = json.Unmarshal(reposRaw, &serviceRepos)
    }
    repoByService := make(map[string]ServiceRepoConfig)
    for _, rc := range serviceRepos {
        repoByService[rc.ServiceID] = rc
    }

    // 3. Fetch graph nodes and edges from graph-manager
    gmURL := os.Getenv("GRAPH_MANAGER_URL")
    if gmURL == "" {
        gmURL = "http://graph-manager:8086"
    }
    graphURL := fmt.Sprintf("%s/api/v1/graphs?namespace=%s", gmURL, url.QueryEscape(namespace))
    type graphNode struct {
        ID   string `json:"id"`
        Name string `json:"name"`
        Type string `json:"type"`
    }
    type graphResp struct {
        Nodes map[string]graphNode `json:"nodes"`
        Edges map[string][]string  `json:"edges"`
    }
    var gr graphResp
    if resp, err := http.Get(graphURL); err == nil { //nolint:noctx
        _ = json.NewDecoder(resp.Body).Decode(&gr)
        resp.Body.Close()
    }

    // 4. Build service entries
    var services []yamlServiceEntry
    for _, node := range gr.Nodes {
        if node.Type == "GITCOMMIT" || node.Type == "CODEFUNCTION" || node.Type == "CODEFILE" {
            continue // only include runtime service/db/queue nodes
        }
        entry := yamlServiceEntry{
            ID:   node.ID,
            Name: node.Name,
            Type: node.Type,
        }
        if rc, ok := repoByService[node.ID]; ok {
            entry.Repo = rc.RepoURL
            entry.Branch = rc.Branch
            entry.CodePath = rc.CodePath
        }
        // Add calls (outbound edges from this node)
        entry.Calls = gr.Edges[node.ID]
        services = append(services, entry)
    }

    // 5. Marshal to YAML
    sm := yamlServiceMap{
        Version: "1.0",
        Projects: []yamlProjectEntry{
            {
                ID:          projectID,
                Name:        name,
                Namespace:   namespace,
                Tenant:      tenant,
                Environment: environment,
                Services:    services,
            },
        },
    }
    out, err := yaml.Marshal(sm)
    if err != nil {
        http.Error(w, "yaml marshal failed", http.StatusInternalServerError)
        return
    }
    w.Header().Set("Content-Type", "text/plain; charset=utf-8")
    w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="service-map-%s.yaml"`, projectID))
    _, _ = w.Write(out)
}

// ─────────────────────────────────────────────────────────────────────────────
// Code-indexer status API
// ─────────────────────────────────────────────────────────────────────────────

// codeIndexStatusHandler handles GET /api/v1/projects/{id}/code-index/status
// Returns the indexing state for each service in the project.
func codeIndexStatusHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }
    parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/projects/"), "/")
    if len(parts) < 1 || parts[0] == "" {
        http.Error(w, "bad path", http.StatusBadRequest)
        return
    }
    projectID := parts[0]

    db := getDB()
    if db == nil {
        http.Error(w, "db unavailable", http.StatusServiceUnavailable)
        return
    }
    ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
    defer cancel()

    // Read service repo configs for repoUrl per service
    var metaRaw []byte
    _ = db.QueryRow(ctx, `SELECT COALESCE(metadata, '{}') FROM projects WHERE id=$1`, projectID).Scan(&metaRaw)
    var meta map[string]json.RawMessage
    _ = json.Unmarshal(metaRaw, &meta)
    var serviceRepos []ServiceRepoConfig
    if reposRaw, ok := meta["serviceRepos"]; ok {
        _ = json.Unmarshal(reposRaw, &serviceRepos)
    }
    repoURLByService := make(map[string]string)
    for _, rc := range serviceRepos {
        repoURLByService[rc.ServiceID] = rc.RepoURL
    }

    // Query code_index_state table
    rows, err := db.Query(ctx,
        `SELECT service_id, COALESCE(last_commit_hash,''), last_indexed_at
         FROM code_index_state WHERE project_id=$1`, projectID)
    if err != nil {
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode([]interface{}{})
        return
    }
    defer rows.Close()

    type indexStatus struct {
        ServiceID            string `json:"serviceId"`
        RepoURL              string `json:"repoUrl"`
        LastCommitHash       string `json:"lastCommitHash"`
        LastIndexedAt        string `json:"lastIndexedAt"`
        TotalCommitsIndexed  int    `json:"totalCommitsIndexed"`
    }
    var result []indexStatus
    for rows.Next() {
        var svcID, hash string
        var indexedAt time.Time
        if err := rows.Scan(&svcID, &hash, &indexedAt); err == nil {
            result = append(result, indexStatus{
                ServiceID:      svcID,
                RepoURL:        repoURLByService[svcID],
                LastCommitHash: hash,
                LastIndexedAt:  indexedAt.UTC().Format(time.RFC3339),
            })
        }
    }
    if result == nil {
        result = []indexStatus{}
    }
    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(result)
}

// ─────────────────────────────────────────────────────────────────────────────
// Audit Trail + Reasoning Chain
// ─────────────────────────────────────────────────────────────────────────────

// incidentAuditHandler handles GET /api/v1/incidents/{id}/audit
// Returns the full sop_audit trail for an incident — every phase change, step action, and auto-resolution.
func incidentAuditHandler(w http.ResponseWriter, r *http.Request) {
    parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/incidents/"), "/")
    if len(parts) < 1 {
        http.Error(w, "bad path", http.StatusBadRequest)
        return
    }
    incidentID := parts[0]

    db := getDB()
    if db == nil {
        http.Error(w, "db unavailable", http.StatusServiceUnavailable)
        return
    }
    ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
    defer cancel()

    type auditRow struct {
        ID             string          `json:"id"`
        Action         string          `json:"action"`
        Actor          string          `json:"actor"`
        PhaseFrom      string          `json:"phaseFrom,omitempty"`
        PhaseTo        string          `json:"phaseTo,omitempty"`
        StepOrder      *int            `json:"stepOrder,omitempty"`
        CreatedAt      string          `json:"createdAt"`
        SignalSnapshot json.RawMessage `json:"signalSnapshot,omitempty"`
    }

    rows, err := db.Query(ctx,
        `SELECT id, action, COALESCE(actor,'system'),
                COALESCE(phase_from,''), COALESCE(phase_to,''),
                step_order,
                to_char(created_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
                COALESCE(signal_snapshot,'{}')
         FROM sop_audit WHERE incident_id = $1
         ORDER BY created_at ASC`, incidentID)
    if err != nil {
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(map[string]interface{}{"events": []interface{}{}, "incidentId": incidentID})
        return
    }
    defer rows.Close()

    var events []auditRow
    for rows.Next() {
        var a auditRow
        var stepOrder *int
        var snapshot []byte
        if err := rows.Scan(&a.ID, &a.Action, &a.Actor, &a.PhaseFrom, &a.PhaseTo,
            &stepOrder, &a.CreatedAt, &snapshot); err == nil {
            a.StepOrder = stepOrder
            if len(snapshot) > 2 {
                a.SignalSnapshot = json.RawMessage(snapshot)
            }
            events = append(events, a)
        }
    }
    if events == nil {
        events = []auditRow{}
    }
    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(map[string]interface{}{
        "incidentId": incidentID,
        "events":     events,
        "total":      len(events),
    })
}

// incidentReasoningHandler handles GET /api/v1/incidents/{id}/reasoning
// Reconstructs the full decision chain in plain English + structured data.
// This answers "WHY did the system make each decision?"
func incidentReasoningHandler(w http.ResponseWriter, r *http.Request) {
    parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/incidents/"), "/")
    if len(parts) < 1 {
        http.Error(w, "bad path", http.StatusBadRequest)
        return
    }
    incidentID := parts[0]

    db := getDB()
    if db == nil {
        http.Error(w, "db unavailable", http.StatusServiceUnavailable)
        return
    }
    ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
    defer cancel()

    // 1. Fetch incident core data
    var service, faultType, rootCause, currentPhase string
    var confidence float64
    var detectedAt time.Time
    var evidenceRaw []byte
    _ = db.QueryRow(ctx,
        `SELECT COALESCE(service,''), COALESCE(root_cause_candidate,''),
                COALESCE(confidence,0), COALESCE(phase,'DETECTING'),
                detected_at, COALESCE(evidence,'[]')
         FROM incidents WHERE id=$1`, incidentID).
        Scan(&service, &rootCause, &confidence, &currentPhase, &detectedAt, &evidenceRaw)

    // 2. Fetch RCA candidates with their scores
    type candidateInfo struct {
        Service       string  `json:"service"`
        Score         float64 `json:"score"`
        EvidenceCount int     `json:"evidenceCount"`
        Rank          int     `json:"rank"`
    }
    var candidates []candidateInfo
    candRows, _ := db.Query(ctx,
        `SELECT candidate_node_id, COALESCE(confidence_score,0), COALESCE(evidence_count,1)
         FROM rca_candidates WHERE incident_id=$1
         ORDER BY confidence_score DESC`, incidentID)
    if candRows != nil {
        defer candRows.Close()
        rank := 1
        for candRows.Next() {
            var c candidateInfo
            if candRows.Scan(&c.Service, &c.Score, &c.EvidenceCount) == nil {
                c.Rank = rank
                candidates = append(candidates, c)
                rank++
            }
        }
    }

    // 3. Fetch audit trail for phase change reasoning
    type phaseStep struct {
        Action    string `json:"action"`
        PhaseFrom string `json:"phaseFrom"`
        PhaseTo   string `json:"phaseTo"`
        Actor     string `json:"actor"`
        Time      string `json:"time"`
    }
    var phaseSteps []phaseStep
    auditRows, _ := db.Query(ctx,
        `SELECT action, COALESCE(phase_from,''), COALESCE(phase_to,''),
                COALESCE(actor,'system'),
                to_char(created_at,'HH24:MI:SS')
         FROM sop_audit WHERE incident_id=$1 ORDER BY created_at ASC`, incidentID)
    if auditRows != nil {
        defer auditRows.Close()
        for auditRows.Next() {
            var ps phaseStep
            if auditRows.Scan(&ps.Action, &ps.PhaseFrom, &ps.PhaseTo, &ps.Actor, &ps.Time) == nil {
                phaseSteps = append(phaseSteps, ps)
            }
        }
    }

    // 4. Count evidence signals
    var evidenceItems []map[string]interface{}
    _ = json.Unmarshal(evidenceRaw, &evidenceItems)

    // 5. Build human-readable reasoning chain
    type reasonStep struct {
        Step      string                 `json:"step"`
        Time      string                 `json:"time,omitempty"`
        Message   string                 `json:"message"`
        Data      map[string]interface{} `json:"data,omitempty"`
    }
    var chain []reasonStep

    chain = append(chain, reasonStep{
        Step:    "signal_received",
        Time:    detectedAt.UTC().Format("15:04:05"),
        Message: fmt.Sprintf("First fault signal received from service '%s'", service),
        Data:    map[string]interface{}{"service": service, "phase": "DETECTING"},
    })

    if len(candidates) > 0 {
        faultType = "unknown"
        // Infer fault type from the top candidate's fault data
        chain = append(chain, reasonStep{
            Step:    "bfs_traversal",
            Message: fmt.Sprintf("BFS graph traversal found %d suspect services. Traversed dependency edges to identify which service's failure propagated upstream.", len(candidates)),
            Data:    map[string]interface{}{"suspects": func() []string {
                ss := make([]string, len(candidates))
                for i, c := range candidates { ss[i] = c.Service }
                return ss
            }()},
        })
    }

    chain = append(chain, reasonStep{
        Step:    "evidence_accumulation",
        Message: fmt.Sprintf("Collected %d evidence signals. Each new signal re-scored all candidates using: evidence(×0.40) + propagation(×0.35) + impact(×0.15) + errorRate(×0.10). Time-decay applied so recent signals count more.", len(evidenceItems)),
        Data: map[string]interface{}{
            "signals_collected": len(evidenceItems),
            "confidence_threshold": 0.35,
            "formula": "score = decayedEvidence×0.40 + propagationScore×0.35 + callerImpact×0.15 + errorRate×0.10",
        },
    })

    if rootCause != "" {
        chain = append(chain, reasonStep{
            Step:    "root_cause_confirmed",
            Message: fmt.Sprintf("Root cause confirmed: '%s' with confidence %.1f%% (threshold: 35%%)", rootCause, confidence*100),
            Data: map[string]interface{}{
                "root_cause":  rootCause,
                "confidence":  fmt.Sprintf("%.3f", confidence),
                "threshold":   "0.350",
                "why_winner":  fmt.Sprintf("'%s' had the highest combined score across evidence count, upstream caller failures, and observed error rate", rootCause),
            },
        })
    }

    for _, ps := range phaseSteps {
        if ps.Action == "phase_change" {
            msg := phaseChangeReason(ps.PhaseFrom, ps.PhaseTo, rootCause, confidence)
            chain = append(chain, reasonStep{
                Step:    "phase_change",
                Time:    ps.Time,
                Message: msg,
                Data:    map[string]interface{}{"from": ps.PhaseFrom, "to": ps.PhaseTo, "actor": ps.Actor},
            })
        }
    }

    _ = faultType
    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(map[string]interface{}{
        "incidentId":     incidentID,
        "service":        service,
        "rootCause":      rootCause,
        "confidence":     confidence,
        "currentPhase":   currentPhase,
        "candidates":     candidates,
        "reasoningChain": chain,
        "summary": fmt.Sprintf(
            "Fault in '%s' detected at %s. After %d signals, '%s' identified as root cause (%.0f%% confidence). Current phase: %s.",
            service, detectedAt.Format("15:04"), len(evidenceItems), rootCause, confidence*100, currentPhase,
        ),
    })
}

func phaseChangeReason(from, to, rootCause string, confidence float64) string {
    switch to {
    case "NARROWING":
        return "Started accumulating evidence — multiple signals expected to narrow down root cause"
    case "CONFIRMED":
        return fmt.Sprintf("Root cause confirmed: confidence %.1f%% exceeded 35%% threshold. '%s' identified as the origin of the fault.", confidence*100, rootCause)
    case "TRIAGING":
        return "SOP playbook generated for the confirmed root cause — operator can now begin structured remediation"
    case "FIXING":
        return "Operator acknowledged the first SOP step — active remediation has begun"
    case "VERIFYING":
        return "Operator completed a step with auto-verify — system is now watching for the expected healthy signal"
    case "RESOLVED":
        return "Expected healthy signal received — the fault condition has cleared and the incident is automatically resolved"
    default:
        return fmt.Sprintf("Phase advanced from %s to %s", from, to)
    }
}

// ─────────────────────────────────────────────────────────────────────────────
// Deployment Tracking
// ─────────────────────────────────────────────────────────────────────────────

// deploymentsHandler handles:
//   POST /api/v1/deployments  — record a new deployment (called from CI/CD)
//   GET  /api/v1/deployments  — list recent deployments (with ?projectId= filter)
func deploymentsHandler(w http.ResponseWriter, r *http.Request) {
    ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
    defer cancel()

    // Use shared pool if available; fall back to per-request connection
    type execQueryable interface {
        Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
        QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
        Exec(ctx context.Context, sql string, args ...interface{}) (interface{ RowsAffected() int64 }, error)
    }
    pool := getDB()
    var conn interface {
        Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
        QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
        Exec(ctx context.Context, sql string, arguments ...interface{}) (interface{ RowsAffected() int64 }, error)
    }
    // pgx.Conn and pgxpool.Pool both have Query/QueryRow/Exec — use pgx.Conn type for fallback
    var singleConn *pgx.Conn
    if pool != nil {
        singleConn = nil // use pool below directly
    } else {
        dsn := os.Getenv("PG_DSN")
        if dsn == "" {
            dsn = "postgres://postgres:postgres@postgres:5432/faultiq?sslmode=disable"
        }
        var err error
        singleConn, err = pgx.Connect(ctx, dsn)
        if err != nil {
            http.Error(w, "db unavailable", http.StatusServiceUnavailable)
            return
        }
        defer singleConn.Close(ctx)
    }
    _ = conn

    // execDB runs Exec against pool or singleConn
    execDB := func(sql string, args ...interface{}) error {
        if pool != nil {
            _, err := pool.Exec(ctx, sql, args...)
            return err
        }
        _, err := singleConn.Exec(ctx, sql, args...)
        return err
    }
    // queryDB runs Query against pool or singleConn
    queryDB := func(sql string, args ...interface{}) (pgx.Rows, error) {
        if pool != nil {
            return pool.Query(ctx, sql, args...)
        }
        return singleConn.Query(ctx, sql, args...)
    }
    // queryRowDB runs QueryRow against pool or singleConn
    queryRowDB := func(sql string, args ...interface{}) pgx.Row {
        if pool != nil {
            return pool.QueryRow(ctx, sql, args...)
        }
        return singleConn.QueryRow(ctx, sql, args...)
    }
    _ = queryRowDB

    if r.Method == http.MethodPost {
        var body struct {
            ServiceID   string `json:"serviceId"`
            ServiceName string `json:"serviceName"`
            Version     string `json:"version"`
            CommitHash  string `json:"commitHash"`
            DeployedBy  string `json:"deployedBy"`
            ProjectID   string `json:"projectId"`
            TenantID    string `json:"tenantId"`
            Environment string `json:"environment"`
            Notes       string `json:"notes"`
        }
        if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
            http.Error(w, "bad json", http.StatusBadRequest)
            return
        }
        if body.ServiceID == "" || body.ProjectID == "" {
            http.Error(w, "serviceId and projectId required", http.StatusBadRequest)
            return
        }
        if body.Environment == "" {
            body.Environment = "prod"
        }
        // Extract from JWT if not provided
        claims := GetClaims(r)
        if body.TenantID == "" && claims != nil {
            if tc, ok := claims["tenant"].(string); ok {
                body.TenantID = tc
            }
        }
        if body.DeployedBy == "" && claims != nil {
            if email, ok := claims["email"].(string); ok {
                body.DeployedBy = email
            }
        }

        // Sanitize inputs (ITEM 7)
        body.Version = sanitizeString(body.Version, 128)
        body.CommitHash = sanitizeString(body.CommitHash, 64)
        body.DeployedBy = sanitizeString(body.DeployedBy, 128)
        body.Notes = sanitizeString(body.Notes, 512)
        body.ServiceID = sanitizeString(body.ServiceID, 128)
        body.ServiceName = sanitizeString(body.ServiceName, 128)

        deployID := fmt.Sprintf("dep-%d", time.Now().UnixMilli())
        healthGateUntil := time.Now().Add(5 * time.Minute)

        if err := execDB(
            `INSERT INTO deployments (id, tenant_id, project_id, service_id, service_name, version, commit_hash,
             deployed_by, environment, status, health_gate_until, notes)
             VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'deploying',$10,$11)`,
            deployID, body.TenantID, body.ProjectID, body.ServiceID, body.ServiceName,
            body.Version, body.CommitHash, body.DeployedBy, body.Environment,
            healthGateUntil, body.Notes); err != nil {
            http.Error(w, "insert failed: "+err.Error(), http.StatusInternalServerError)
            return
        }

        // Publish deployment event to Redis for detection-engine to correlate
        raddr := os.Getenv("REDIS_ADDR")
        if raddr == "" {
            raddr = "redis:6379"
        }
        rdb := redis.NewClient(&redis.Options{Addr: raddr})
        defer rdb.Close()
        event := map[string]interface{}{
            "type":            "deployment",
            "deploymentId":    deployID,
            "serviceId":       body.ServiceID,
            "serviceName":     body.ServiceName,
            "version":         body.Version,
            "commitHash":      body.CommitHash,
            "deployedBy":      body.DeployedBy,
            "projectId":       body.ProjectID,
            "tenantId":        body.TenantID,
            "environment":     body.Environment,
            "healthGateUntil": healthGateUntil.UTC().Format(time.RFC3339),
            "deployedAt":      time.Now().UTC().Format(time.RFC3339),
        }
        b, _ := json.Marshal(event)
        _ = rdb.Publish(ctx, "deployment_events", b).Err()

        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusCreated)
        _ = json.NewEncoder(w).Encode(map[string]interface{}{
            "deploymentId":    deployID,
            "serviceId":       body.ServiceID,
            "status":          "deploying",
            "healthGateUntil": healthGateUntil.UTC().Format(time.RFC3339),
            "message":         fmt.Sprintf("Deployment recorded. TechGraph will watch %s for the next 5 minutes.", body.ServiceID),
        })
        return
    }

    // GET — list recent deployments
    projectID := r.URL.Query().Get("projectId")
    serviceID := r.URL.Query().Get("serviceId")
    limit := 20

    type depRow struct {
        ID           string  `json:"id"`
        ServiceID    string  `json:"serviceId"`
        ServiceName  string  `json:"serviceName"`
        Version      string  `json:"version"`
        CommitHash   string  `json:"commitHash"`
        DeployedBy   string  `json:"deployedBy"`
        Status       string  `json:"status"`
        Environment  string  `json:"environment"`
        DeployedAt   string  `json:"deployedAt"`
        HealthStatus string  `json:"healthStatus"`
        Notes        string  `json:"notes"`
    }

    var rows pgx.Rows
    var err error
    if serviceID != "" {
        rows, err = queryDB(
            `SELECT id, service_id, COALESCE(service_name,''), COALESCE(version,''), COALESCE(commit_hash,''),
                    COALESCE(deployed_by,''), COALESCE(status,''), COALESCE(environment,'prod'),
                    to_char(deployed_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
                    COALESCE(health_status,''), COALESCE(notes,'')
             FROM deployments WHERE project_id=$1 AND service_id=$2
             ORDER BY deployed_at DESC LIMIT $3`, projectID, serviceID, limit)
    } else {
        rows, err = queryDB(
            `SELECT id, service_id, COALESCE(service_name,''), COALESCE(version,''), COALESCE(commit_hash,''),
                    COALESCE(deployed_by,''), COALESCE(status,''), COALESCE(environment,'prod'),
                    to_char(deployed_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
                    COALESCE(health_status,''), COALESCE(notes,'')
             FROM deployments WHERE project_id=$1
             ORDER BY deployed_at DESC LIMIT $2`, projectID, limit)
    }
    if err != nil {
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(map[string]interface{}{"deployments": []interface{}{}})
        return
    }
    defer rows.Close()

    var deps []depRow
    for rows.Next() {
        var d depRow
        if rows.Scan(&d.ID, &d.ServiceID, &d.ServiceName, &d.Version, &d.CommitHash,
            &d.DeployedBy, &d.Status, &d.Environment, &d.DeployedAt,
            &d.HealthStatus, &d.Notes) == nil {
            deps = append(deps, d)
        }
    }
    if deps == nil {
        deps = []depRow{}
    }
    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(map[string]interface{}{"deployments": deps, "total": len(deps)})
}

// ─────────────────────────────────────────────────────────────────────────────
// Batch Deployment Endpoint (ITEM 6)
// ─────────────────────────────────────────────────────────────────────────────

// CreateDeploymentInput is the body shape for a single deployment in the batch.
type CreateDeploymentInput struct {
    ServiceID   string `json:"serviceId"`
    ServiceName string `json:"serviceName"`
    Version     string `json:"version"`
    CommitHash  string `json:"commitHash"`
    DeployedBy  string `json:"deployedBy"`
    ProjectID   string `json:"projectId"`
    TenantID    string `json:"tenantId"`
    Environment string `json:"environment"`
    Notes       string `json:"notes"`
}

// deploymentsBatchHandler handles POST /api/v1/deployments/batch
// Accepts an array of deployment inputs (max 50) and inserts them all at once.
func deploymentsBatchHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }

    var inputs []CreateDeploymentInput
    if err := json.NewDecoder(r.Body).Decode(&inputs); err != nil {
        http.Error(w, "bad json", http.StatusBadRequest)
        return
    }
    if len(inputs) == 0 {
        http.Error(w, "no deployments provided", http.StatusBadRequest)
        return
    }
    if len(inputs) > 50 {
        http.Error(w, "batch size exceeds maximum of 50", http.StatusBadRequest)
        return
    }

    db := getDB()
    if db == nil {
        http.Error(w, "db unavailable", http.StatusServiceUnavailable)
        return
    }
    ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
    defer cancel()

    claims := GetClaims(r)

    raddr := os.Getenv("REDIS_ADDR")
    if raddr == "" {
        raddr = "redis:6379"
    }
    rdb := redis.NewClient(&redis.Options{Addr: raddr})
    defer rdb.Close()

    type batchResult struct {
        DeploymentID    string `json:"deploymentId"`
        ServiceID       string `json:"serviceId"`
        Status          string `json:"status"`
        HealthGateUntil string `json:"healthGateUntil"`
    }
    results := make([]batchResult, 0, len(inputs))

    for _, body := range inputs {
        // Fill in from JWT if not provided
        if body.TenantID == "" && claims != nil {
            if tc, ok := claims["tenant"].(string); ok {
                body.TenantID = tc
            }
        }
        if body.DeployedBy == "" && claims != nil {
            if email, ok := claims["email"].(string); ok {
                body.DeployedBy = email
            }
        }
        if body.Environment == "" {
            body.Environment = "prod"
        }
        if body.ServiceID == "" || body.ProjectID == "" {
            continue // skip invalid entries
        }

        // Sanitize
        body.Version = sanitizeString(body.Version, 128)
        body.CommitHash = sanitizeString(body.CommitHash, 64)
        body.DeployedBy = sanitizeString(body.DeployedBy, 128)
        body.Notes = sanitizeString(body.Notes, 512)
        body.ServiceID = sanitizeString(body.ServiceID, 128)
        body.ServiceName = sanitizeString(body.ServiceName, 128)

        deployID := fmt.Sprintf("dep-%d", time.Now().UnixNano())
        healthGateUntil := time.Now().Add(5 * time.Minute)

        if err := func() error {
            _, err := db.Exec(ctx,
                `INSERT INTO deployments (id, tenant_id, project_id, service_id, service_name, version, commit_hash,
                 deployed_by, environment, status, health_gate_until, notes)
                 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'deploying',$10,$11)`,
                deployID, body.TenantID, body.ProjectID, body.ServiceID, body.ServiceName,
                body.Version, body.CommitHash, body.DeployedBy, body.Environment,
                healthGateUntil, body.Notes)
            return err
        }(); err != nil {
            continue
        }

        // Publish to Redis
        event := map[string]interface{}{
            "type":            "deployment",
            "deploymentId":    deployID,
            "serviceId":       body.ServiceID,
            "serviceName":     body.ServiceName,
            "version":         body.Version,
            "commitHash":      body.CommitHash,
            "deployedBy":      body.DeployedBy,
            "projectId":       body.ProjectID,
            "tenantId":        body.TenantID,
            "environment":     body.Environment,
            "healthGateUntil": healthGateUntil.UTC().Format(time.RFC3339),
            "deployedAt":      time.Now().UTC().Format(time.RFC3339),
        }
        b, _ := json.Marshal(event)
        _ = rdb.Publish(ctx, "deployment_events", b).Err()

        results = append(results, batchResult{
            DeploymentID:    deployID,
            ServiceID:       body.ServiceID,
            Status:          "deploying",
            HealthGateUntil: healthGateUntil.UTC().Format(time.RFC3339),
        })
    }

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    _ = json.NewEncoder(w).Encode(map[string]interface{}{
        "deployments": results,
        "total":       len(results),
    })
}

// deploymentDetailHandler handles GET/PUT /api/v1/deployments/{id}
func deploymentDetailHandler(w http.ResponseWriter, r *http.Request) {
    deployID := strings.TrimPrefix(r.URL.Path, "/api/v1/deployments/")
    if deployID == "" {
        http.Error(w, "deployment id required", http.StatusBadRequest)
        return
    }

    db := getDB()
    if db == nil {
        http.Error(w, "db unavailable", http.StatusServiceUnavailable)
        return
    }
    ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
    defer cancel()

    if r.Method == http.MethodPut {
        // Update deployment status (e.g. mark as rolled_back)
        var body struct {
            Status       string `json:"status"`
            HealthStatus string `json:"healthStatus"`
        }
        if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
            http.Error(w, "bad json", http.StatusBadRequest)
            return
        }
        _, _ = db.Exec(ctx,
            `UPDATE deployments SET status=$1, health_status=$2, health_checked_at=NOW() WHERE id=$3`,
            body.Status, body.HealthStatus, deployID)
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(map[string]string{"id": deployID, "status": body.Status})
        return
    }

    // GET — single deployment
    var d struct {
        ID           string `json:"id"`
        ServiceID    string `json:"serviceId"`
        ServiceName  string `json:"serviceName"`
        Version      string `json:"version"`
        CommitHash   string `json:"commitHash"`
        DeployedBy   string `json:"deployedBy"`
        Status       string `json:"status"`
        DeployedAt   string `json:"deployedAt"`
        HealthStatus string `json:"healthStatus"`
    }
    err := db.QueryRow(ctx,
        `SELECT id, service_id, COALESCE(service_name,''), COALESCE(version,''), COALESCE(commit_hash,''),
                COALESCE(deployed_by,''), COALESCE(status,''),
                to_char(deployed_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"'), COALESCE(health_status,'')
         FROM deployments WHERE id=$1`, deployID).
        Scan(&d.ID, &d.ServiceID, &d.ServiceName, &d.Version, &d.CommitHash,
            &d.DeployedBy, &d.Status, &d.DeployedAt, &d.HealthStatus)
    if err != nil {
        http.Error(w, "not found", http.StatusNotFound)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(d)
}

// serviceDeploymentsHandler handles GET /api/v1/projects/{id}/services/{svcId}/deployments
func serviceDeploymentsHandler(w http.ResponseWriter, r *http.Request) {
    parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/projects/"), "/")
    // parts: [projectId, "services", serviceId, "deployments"]
    if len(parts) < 4 {
        http.Error(w, "bad path", http.StatusBadRequest)
        return
    }
    projectID, serviceID := parts[0], parts[2]

    // Reuse deploymentsHandler logic
    r2 := r.Clone(r.Context())
    q := r2.URL.Query()
    q.Set("projectId", projectID)
    q.Set("serviceId", serviceID)
    r2.URL.RawQuery = q.Encode()
    r2.URL.Path = "/api/v1/deployments"
    deploymentsHandler(w, r2)
}


// ─────────────────────────────────────────────────────────────────────────────
// Request Path — shows how a request travels from entry point to a service
// ─────────────────────────────────────────────────────────────────────────────

// requestPathHandler handles GET /api/v1/projects/{id}/services/{svcId}/request-path
// Traverses the CALLS edges to find the complete path from user-facing entry (GATEWAY)
// down to the specified service. Shows how a user request reaches this service.
func requestPathHandler(w http.ResponseWriter, r *http.Request) {
    parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/projects/"), "/")
    if len(parts) < 3 {
        http.Error(w, "bad path", http.StatusBadRequest)
        return
    }
    projectID, serviceID := parts[0], parts[2]

    // Resolve namespace from project slug
    db := getDB()
    if db == nil {
        http.Error(w, "db unavailable", http.StatusServiceUnavailable)
        return
    }
    ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
    defer cancel()

    var slug string
    _ = db.QueryRow(ctx, `SELECT COALESCE(slug,'') FROM projects WHERE id=$1`, projectID).Scan(&slug)

    namespace := slug + ":prod"

    // Fetch graph
    gmURL := os.Getenv("GRAPH_MANAGER_URL")
    if gmURL == "" {
        gmURL = "http://graph-manager:8086"
    }
    graphURL := fmt.Sprintf("%s/api/v1/graphs?namespace=%s", gmURL, url.QueryEscape(namespace))
    graphResp, err := http.Get(graphURL) //nolint:noctx
    if err != nil {
        http.Error(w, "graph unavailable", http.StatusServiceUnavailable)
        return
    }
    defer graphResp.Body.Close()

    var graph struct {
        Nodes map[string]struct {
            ID   string `json:"id"`
            Name string `json:"name"`
            Type string `json:"type"`
        } `json:"nodes"`
        Edges map[string][]string `json:"edges"` // from → []to
    }
    if err := json.NewDecoder(graphResp.Body).Decode(&graph); err != nil {
        http.Error(w, "graph parse error", http.StatusInternalServerError)
        return
    }

    // Build reverse edge map: to → []from (who calls this node)
    reverseEdges := make(map[string][]string)
    for from, tos := range graph.Edges {
        for _, to := range tos {
            reverseEdges[to] = append(reverseEdges[to], from)
        }
    }

    // BFS backwards from serviceID to find all paths to GATEWAY nodes
    type pathNode struct {
        ServiceID   string `json:"serviceId"`
        ServiceName string `json:"serviceName"`
        ServiceType string `json:"serviceType"`
        Depth       int    `json:"depth"`
    }

    visited := map[string]bool{}
    var paths [][]pathNode

    // DFS to find all paths from any GATEWAY to this service
    var findPaths func(current string, path []pathNode)
    findPaths = func(current string, path []pathNode) {
        node := graph.Nodes[current]
        if node.ID == "" {
            return
        }
        path = append(path, pathNode{
            ServiceID:   current,
            ServiceName: node.Name,
            ServiceType: node.Type,
            Depth:       len(path),
        })
        if node.Type == "GATEWAY" || len(reverseEdges[current]) == 0 {
            // Reached an entry point — record this path (reversed so it flows user→service)
            reversed := make([]pathNode, len(path))
            for i, p := range path {
                reversed[len(path)-1-i] = p
                reversed[len(path)-1-i].Depth = i
            }
            paths = append(paths, reversed)
            return
        }
        if visited[current] {
            return
        }
        visited[current] = true
        for _, caller := range reverseEdges[current] {
            findPaths(caller, path)
        }
        visited[current] = false
    }

    findPaths(serviceID, nil)

    // If no path found (service is already the entry point or disconnected)
    if len(paths) == 0 {
        node := graph.Nodes[serviceID]
        paths = [][]pathNode{{{
            ServiceID:   serviceID,
            ServiceName: node.Name,
            ServiceType: node.Type,
            Depth:       0,
        }}}
    }

    // Find any recent deployments for this service (for the "check deployments" context)
    ctx2, cancel2 := context.WithTimeout(r.Context(), 3*time.Second)
    defer cancel2()
    var recentDeploys []map[string]interface{}
    if db != nil {
        depRows, _ := db.Query(ctx2,
            `SELECT version, commit_hash, deployed_by, status, to_char(deployed_at, 'YYYY-MM-DD HH24:MI')
             FROM deployments WHERE service_id=$1 AND project_id=$2
             ORDER BY deployed_at DESC LIMIT 3`, serviceID, projectID)
        if depRows != nil {
            defer depRows.Close()
            for depRows.Next() {
                var version, hash, deployedBy, status, deployedAt string
                if depRows.Scan(&version, &hash, &deployedBy, &status, &deployedAt) == nil {
                    recentDeploys = append(recentDeploys, map[string]interface{}{
                        "version": version, "commitHash": hash, "deployedBy": deployedBy,
                        "status": status, "deployedAt": deployedAt,
                    })
                }
            }
        }
    }
    if recentDeploys == nil {
        recentDeploys = []map[string]interface{}{}
    }

    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(map[string]interface{}{
        "serviceId":      serviceID,
        "paths":          paths,
        "pathCount":      len(paths),
        "recentDeploys":  recentDeploys,
        "summary":        fmt.Sprintf("Found %d path(s) from entry point to %s", len(paths), serviceID),
    })
}

// ─────────────────────────────────────────────────────────────────────────────
// Custom SOP Playbook per service
// ─────────────────────────────────────────────────────────────────────────────

// sopPlaybookHandler handles PUT /api/v1/projects/{id}/services/{svcId}/sop-playbook
// Stores a custom SOP playbook for a service in projects.metadata.customPlaybooks.
func sopPlaybookHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPut {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }
    // Path: /api/v1/projects/{id}/services/{svcId}/sop-playbook
    parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/projects/"), "/")
    if len(parts) < 4 || parts[1] != "services" {
        http.Error(w, "bad path", http.StatusBadRequest)
        return
    }
    projectID := parts[0]
    serviceID := parts[2]

    type PlaybookStep struct {
        Order          int    `json:"order"`
        Title          string `json:"title"`
        Action         string `json:"action"`
        ExpectedSignal string `json:"expectedSignal"`
        AutoVerify     bool   `json:"autoVerify"`
    }
    var body struct {
        Steps []PlaybookStep `json:"steps"`
    }
    if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
        http.Error(w, "bad json", http.StatusBadRequest)
        return
    }

    // Validate step fields (ITEM 7)
    for _, step := range body.Steps {
        if len(step.Title) > 200 {
            http.Error(w, "step title exceeds 200 characters", http.StatusBadRequest)
            return
        }
        if len(step.Action) > 1000 {
            http.Error(w, "step action exceeds 1000 characters", http.StatusBadRequest)
            return
        }
    }

    stepsJSON, _ := json.Marshal(body.Steps)

    ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
    defer cancel()

    db := getDB()
    if db == nil {
        http.Error(w, "db unavailable", http.StatusServiceUnavailable)
        return
    }

    // ── Version history: save current playbook before overwriting ─────────────
    actor := getActorFromRequest(r)
    var currentRaw []byte
    _ = db.QueryRow(ctx,
        `SELECT metadata->'customPlaybooks'->$1 FROM projects WHERE id=$2`,
        serviceID, projectID).Scan(&currentRaw)
    if len(currentRaw) > 2 { // non-empty JSON (not null or {})
        var nextVer int
        _ = db.QueryRow(ctx,
            `SELECT COALESCE(MAX(version),0)+1 FROM runbook_versions WHERE project_id=$1 AND service_id=$2`,
            projectID, serviceID).Scan(&nextVer)
        if nextVer < 1 {
            nextVer = 1
        }
        _, _ = db.Exec(ctx,
            `INSERT INTO runbook_versions (project_id, service_id, version, steps, created_by)
             VALUES ($1,$2,$3,$4,$5) ON CONFLICT (project_id, service_id, version) DO NOTHING`,
            projectID, serviceID, nextVer, string(currentRaw), actor)
    }

    // Save the new playbook
    _, execErr := db.Exec(ctx,
        `UPDATE projects SET metadata = jsonb_set(
            COALESCE(metadata, '{}'),
            ARRAY['customPlaybooks', $1], $2::jsonb
        ) WHERE id=$3`,
        serviceID, string(stepsJSON), projectID)
    if execErr != nil {
        http.Error(w, "update failed: "+execErr.Error(), http.StatusInternalServerError)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(map[string]interface{}{
        "projectId": projectID,
        "serviceId": serviceID,
        "steps":     body.Steps,
        "message":   fmt.Sprintf("Custom SOP playbook saved for service %s", serviceID),
    })
}

// ─────────────────────────────────────────────────────────────────────────────
// Playbook Learning
// ─────────────────────────────────────────────────────────────────────────────

// ─────────────────────────────────────────────────────────────────────────────
// Graph Snapshot + Diff
// ─────────────────────────────────────────────────────────────────────────────

// graphSnapshotHandler handles POST /api/v1/projects/{id}/graphs/snapshot
// Fetches the current graph from graph-manager and stores it in projects.metadata.graphSnapshot.
func graphSnapshotHandler(w http.ResponseWriter, r *http.Request) {
    parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/projects/"), "/")
    if len(parts) < 1 || parts[0] == "" {
        http.Error(w, "bad path", http.StatusBadRequest)
        return
    }
    projectID := parts[0]

    ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
    defer cancel()

    db := getDB()
    if db == nil {
        http.Error(w, "db unavailable", http.StatusServiceUnavailable)
        return
    }

    // Resolve namespace from project slug
    var slug, env string
    _ = db.QueryRow(ctx, `SELECT COALESCE(slug,''), COALESCE(environment,'prod') FROM projects WHERE id=$1`, projectID).Scan(&slug, &env)
    namespace := fmt.Sprintf("%s:%s", slug, env)
    if slug == "" {
        namespace = projectID + ":prod"
    }

    // Fetch current graph from graph-manager
    gmURL := os.Getenv("GRAPH_MANAGER_URL")
    if gmURL == "" {
        gmURL = "http://graph-manager:8086"
    }
    graphURL := fmt.Sprintf("%s/api/v1/graphs?namespace=%s", gmURL, url.QueryEscape(namespace))
    resp, err := http.Get(graphURL) //nolint:noctx
    if err != nil {
        http.Error(w, "graph-manager unreachable", http.StatusServiceUnavailable)
        return
    }
    defer resp.Body.Close()

    var graphData map[string]interface{}
    if decErr := json.NewDecoder(resp.Body).Decode(&graphData); decErr != nil {
        http.Error(w, "graph parse error", http.StatusInternalServerError)
        return
    }

    // Store snapshot with timestamp in projects.metadata.graphSnapshot
    snapshot := map[string]interface{}{
        "graph":       graphData,
        "snapshotAt":  time.Now().UTC().Format(time.RFC3339),
        "namespace":   namespace,
    }
    snapshotJSON, _ := json.Marshal(snapshot)

    _, execErr := db.Exec(ctx,
        `UPDATE projects SET metadata = jsonb_set(COALESCE(metadata,'{}'), '{graphSnapshot}', $1::jsonb) WHERE id=$2`,
        string(snapshotJSON), projectID)
    if execErr != nil {
        http.Error(w, "snapshot save failed: "+execErr.Error(), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    _ = json.NewEncoder(w).Encode(map[string]interface{}{
        "projectId":   projectID,
        "namespace":   namespace,
        "snapshotAt":  snapshot["snapshotAt"],
        "message":     "Graph snapshot saved",
    })
}

// graphDiffHandler handles GET /api/v1/projects/{id}/graphs/diff
// Compares the current graph with the stored snapshot and returns a structured diff.
func graphDiffHandler(w http.ResponseWriter, r *http.Request) {
    parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/projects/"), "/")
    if len(parts) < 1 || parts[0] == "" {
        http.Error(w, "bad path", http.StatusBadRequest)
        return
    }
    projectID := parts[0]

    ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
    defer cancel()

    db := getDB()
    if db == nil {
        http.Error(w, "db unavailable", http.StatusServiceUnavailable)
        return
    }

    // Load stored snapshot from projects.metadata.graphSnapshot
    var metaRaw []byte
    _ = db.QueryRow(ctx, `SELECT COALESCE(metadata,'{}') FROM projects WHERE id=$1`, projectID).Scan(&metaRaw)
    var meta map[string]json.RawMessage
    _ = json.Unmarshal(metaRaw, &meta)

    type graphNode struct {
        ID          string `json:"id"`
        Name        string `json:"name"`
        Type        string `json:"type"`
        StatusClass string `json:"statusClass"`
    }
    type graphShape struct {
        Nodes map[string]graphNode `json:"nodes"`
        Edges map[string][]string  `json:"edges"`
    }

    var snapshotGraph graphShape
    var snapshotAt string
    if snapRaw, ok := meta["graphSnapshot"]; ok {
        var snap struct {
            Graph      graphShape `json:"graph"`
            SnapshotAt string     `json:"snapshotAt"`
        }
        if json.Unmarshal(snapRaw, &snap) == nil {
            snapshotGraph = snap.Graph
            snapshotAt = snap.SnapshotAt
        }
    }

    // Fetch current graph from graph-manager
    var slug, env string
    _ = db.QueryRow(ctx, `SELECT COALESCE(slug,''), COALESCE(environment,'prod') FROM projects WHERE id=$1`, projectID).Scan(&slug, &env)
    namespace := fmt.Sprintf("%s:%s", slug, env)
    if slug == "" {
        namespace = projectID + ":prod"
    }

    gmURL := os.Getenv("GRAPH_MANAGER_URL")
    if gmURL == "" {
        gmURL = "http://graph-manager:8086"
    }
    graphURL := fmt.Sprintf("%s/api/v1/graphs?namespace=%s", gmURL, url.QueryEscape(namespace))
    gmResp, gmErr := http.Get(graphURL) //nolint:noctx
    if gmErr != nil {
        http.Error(w, "graph-manager unreachable", http.StatusServiceUnavailable)
        return
    }
    defer gmResp.Body.Close()

    var currentGraph graphShape
    _ = json.NewDecoder(gmResp.Body).Decode(&currentGraph)

    if currentGraph.Nodes == nil {
        currentGraph.Nodes = make(map[string]graphNode)
    }
    if currentGraph.Edges == nil {
        currentGraph.Edges = make(map[string][]string)
    }
    if snapshotGraph.Nodes == nil {
        snapshotGraph.Nodes = make(map[string]graphNode)
    }
    if snapshotGraph.Edges == nil {
        snapshotGraph.Edges = make(map[string][]string)
    }

    // Compute diff
    type diffNode struct {
        ID   string `json:"id"`
        Name string `json:"name"`
        Type string `json:"type"`
    }
    type changedNode struct {
        ID      string   `json:"id"`
        Changes []string `json:"changes"`
    }
    type diffEdge struct {
        From string `json:"from"`
        To   string `json:"to"`
    }

    var added, removed []diffNode
    var changed []changedNode
    var addedEdges, removedEdges []diffEdge

    // Added nodes: in current but not in snapshot
    for id, n := range currentGraph.Nodes {
        if _, ok := snapshotGraph.Nodes[id]; !ok {
            added = append(added, diffNode{ID: id, Name: n.Name, Type: n.Type})
        }
    }
    // Removed nodes: in snapshot but not in current
    for id, n := range snapshotGraph.Nodes {
        if _, ok := currentGraph.Nodes[id]; !ok {
            removed = append(removed, diffNode{ID: id, Name: n.Name, Type: n.Type})
        }
    }
    // Changed nodes: in both but status changed
    for id, cur := range currentGraph.Nodes {
        if snap, ok := snapshotGraph.Nodes[id]; ok {
            if cur.StatusClass != snap.StatusClass {
                changed = append(changed, changedNode{
                    ID:      id,
                    Changes: []string{fmt.Sprintf("statusClass: %s→%s", snap.StatusClass, cur.StatusClass)},
                })
            }
        }
    }

    // Added edges: in current but not in snapshot
    for from, tos := range currentGraph.Edges {
        snapTos := snapshotGraph.Edges[from]
        snapSet := make(map[string]bool, len(snapTos))
        for _, t := range snapTos {
            snapSet[t] = true
        }
        for _, to := range tos {
            if !snapSet[to] {
                addedEdges = append(addedEdges, diffEdge{From: from, To: to})
            }
        }
    }
    // Removed edges: in snapshot but not in current
    for from, tos := range snapshotGraph.Edges {
        curTos := currentGraph.Edges[from]
        curSet := make(map[string]bool, len(curTos))
        for _, t := range curTos {
            curSet[t] = true
        }
        for _, to := range tos {
            if !curSet[to] {
                removedEdges = append(removedEdges, diffEdge{From: from, To: to})
            }
        }
    }

    // Compute snapshot age
    snapshotAge := "no snapshot"
    if snapshotAt != "" {
        if t, err := time.Parse(time.RFC3339, snapshotAt); err == nil {
            d := time.Since(t)
            h := int(d.Hours())
            m := int(d.Minutes()) % 60
            snapshotAge = fmt.Sprintf("%dh %dm", h, m)
        }
    }

    // Ensure non-nil slices for clean JSON
    if added == nil {
        added = []diffNode{}
    }
    if removed == nil {
        removed = []diffNode{}
    }
    if changed == nil {
        changed = []changedNode{}
    }
    if addedEdges == nil {
        addedEdges = []diffEdge{}
    }
    if removedEdges == nil {
        removedEdges = []diffEdge{}
    }

    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(map[string]interface{}{
        "added":        added,
        "removed":      removed,
        "changed":      changed,
        "addedEdges":   addedEdges,
        "removedEdges": removedEdges,
        "snapshotAge":  snapshotAge,
        "snapshotAt":   snapshotAt,
    })
}

// ─────────────────────────────────────────────────────────────────────────────
// Runbook Version History
// ─────────────────────────────────────────────────────────────────────────────

// sopPlaybookHistoryHandler handles GET /api/v1/projects/{id}/services/{svcId}/sop-playbook/history
// Returns all saved runbook versions for a service, sorted by version DESC.
func sopPlaybookHistoryHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }
    // Path: /api/v1/projects/{id}/services/{svcId}/sop-playbook/history
    parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/projects/"), "/")
    if len(parts) < 4 || parts[1] != "services" {
        http.Error(w, "bad path", http.StatusBadRequest)
        return
    }
    projectID := parts[0]
    serviceID := parts[2]

    ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
    defer cancel()

    type versionRow struct {
        ID        string          `json:"id"`
        Version   int             `json:"version"`
        Steps     json.RawMessage `json:"steps"`
        CreatedBy string          `json:"createdBy"`
        CreatedAt string          `json:"createdAt"`
    }

    var rows pgx.Rows
    var err error
    pool := getDB()
    if pool != nil {
        rows, err = pool.Query(ctx,
            `SELECT id, version, steps, COALESCE(created_by,'system'),
                    to_char(created_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"')
             FROM runbook_versions
             WHERE project_id=$1 AND service_id=$2
             ORDER BY version DESC`, projectID, serviceID)
    } else {
        dsn := os.Getenv("PG_DSN")
        if dsn == "" {
            dsn = "postgres://postgres:postgres@postgres:5432/faultiq?sslmode=disable"
        }
        conn, cerr := pgx.Connect(ctx, dsn)
        if cerr != nil {
            w.Header().Set("Content-Type", "application/json")
            _ = json.NewEncoder(w).Encode(map[string]interface{}{"versions": []interface{}{}, "total": 0})
            return
        }
        defer conn.Close(ctx)
        rows, err = conn.Query(ctx,
            `SELECT id, version, steps, COALESCE(created_by,'system'),
                    to_char(created_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"')
             FROM runbook_versions
             WHERE project_id=$1 AND service_id=$2
             ORDER BY version DESC`, projectID, serviceID)
    }
    if err != nil {
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(map[string]interface{}{"versions": []interface{}{}, "total": 0})
        return
    }
    defer rows.Close()

    var versions []versionRow
    for rows.Next() {
        var v versionRow
        var stepsRaw []byte
        if rows.Scan(&v.ID, &v.Version, &stepsRaw, &v.CreatedBy, &v.CreatedAt) == nil {
            v.Steps = json.RawMessage(stepsRaw)
            versions = append(versions, v)
        }
    }
    if versions == nil {
        versions = []versionRow{}
    }
    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(map[string]interface{}{"versions": versions, "total": len(versions)})
}

// sopPlaybookRevertHandler handles POST /api/v1/projects/{id}/services/{svcId}/sop-playbook/revert?version=N
// Restores a previous runbook version as the active playbook.
func sopPlaybookRevertHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }
    // Path: /api/v1/projects/{id}/services/{svcId}/sop-playbook/revert
    parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/projects/"), "/")
    if len(parts) < 4 || parts[1] != "services" {
        http.Error(w, "bad path", http.StatusBadRequest)
        return
    }
    projectID := parts[0]
    serviceID := parts[2]
    versionParam := r.URL.Query().Get("version")

    ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
    defer cancel()

    db := getDB()
    if db == nil {
        http.Error(w, "db unavailable", http.StatusServiceUnavailable)
        return
    }

    // Fetch the requested version's steps
    var stepsRaw []byte
    var version int
    var query string
    var args []interface{}
    if versionParam != "" {
        query = `SELECT version, steps FROM runbook_versions WHERE project_id=$1 AND service_id=$2 AND version=$3 LIMIT 1`
        args = []interface{}{projectID, serviceID, versionParam}
    } else {
        // Default to latest stored version
        query = `SELECT version, steps FROM runbook_versions WHERE project_id=$1 AND service_id=$2 ORDER BY version DESC LIMIT 1`
        args = []interface{}{projectID, serviceID}
    }
    if scanErr := db.QueryRow(ctx, query, args...).Scan(&version, &stepsRaw); scanErr != nil {
        http.Error(w, "version not found", http.StatusNotFound)
        return
    }

    // Save current steps as a new version before reverting
    var currentRaw []byte
    _ = db.QueryRow(ctx,
        `SELECT metadata->'customPlaybooks'->$1 FROM projects WHERE id=$2`,
        serviceID, projectID).Scan(&currentRaw)
    if len(currentRaw) > 2 {
        var nextVer int
        _ = db.QueryRow(ctx,
            `SELECT COALESCE(MAX(version),0)+1 FROM runbook_versions WHERE project_id=$1 AND service_id=$2`,
            projectID, serviceID).Scan(&nextVer)
        actor := getActorFromRequest(r)
        _, _ = db.Exec(ctx,
            `INSERT INTO runbook_versions (project_id, service_id, version, steps, created_by)
             VALUES ($1,$2,$3,$4,$5) ON CONFLICT (project_id, service_id, version) DO NOTHING`,
            projectID, serviceID, nextVer, string(currentRaw), actor)
    }

    // Restore the requested version as the active playbook
    _, execErr := db.Exec(ctx,
        `UPDATE projects SET metadata = jsonb_set(COALESCE(metadata,'{}'), ARRAY['customPlaybooks',$1], $2::jsonb) WHERE id=$3`,
        serviceID, string(stepsRaw), projectID)
    if execErr != nil {
        http.Error(w, "revert failed: "+execErr.Error(), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(map[string]interface{}{
        "projectId": projectID,
        "serviceId": serviceID,
        "version":   version,
        "message":   fmt.Sprintf("Reverted to version %d", version),
    })
}

// playbookLearningHandler handles GET /api/v1/projects/{id}/services/{svcId}/playbook-learning
// Returns learned step success rates so the frontend can highlight high-confidence steps.
func playbookLearningHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }
    // Path: /api/v1/projects/{id}/services/{svcId}/playbook-learning
    parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/projects/"), "/")
    if len(parts) < 4 || parts[1] != "services" {
        http.Error(w, "bad path", http.StatusBadRequest)
        return
    }
    serviceID := parts[2]

    ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
    defer cancel()

    type stepLearning struct {
        FaultType    string  `json:"faultType"`
        StepOrder    int     `json:"stepOrder"`
        StepTitle    string  `json:"stepTitle"`
        SuccessCount int     `json:"successCount"`
        TotalCount   int     `json:"totalCount"`
        SuccessRate  float64 `json:"successRate"`
    }

    var rows pgx.Rows
    var err error
    pool := getDB()
    if pool != nil {
        rows, err = pool.Query(ctx,
            `SELECT fault_type, step_order, COALESCE(step_title,''), success_count, total_count
             FROM playbook_learning WHERE service_id=$1
             ORDER BY fault_type, CASE WHEN total_count > 0 THEN success_count::float/total_count ELSE 0 END DESC`,
            serviceID)
    } else {
        dsn := os.Getenv("PG_DSN")
        if dsn == "" {
            dsn = "postgres://postgres:postgres@postgres:5432/faultiq?sslmode=disable"
        }
        conn, cerr := pgx.Connect(ctx, dsn)
        if cerr != nil {
            w.Header().Set("Content-Type", "application/json")
            _ = json.NewEncoder(w).Encode([]interface{}{})
            return
        }
        defer conn.Close(ctx)
        rows, err = conn.Query(ctx,
            `SELECT fault_type, step_order, COALESCE(step_title,''), success_count, total_count
             FROM playbook_learning WHERE service_id=$1
             ORDER BY fault_type, CASE WHEN total_count > 0 THEN success_count::float/total_count ELSE 0 END DESC`,
            serviceID)
    }
    if err != nil {
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode([]interface{}{})
        return
    }
    defer rows.Close()

    var result []stepLearning
    for rows.Next() {
        var sl stepLearning
        if rows.Scan(&sl.FaultType, &sl.StepOrder, &sl.StepTitle, &sl.SuccessCount, &sl.TotalCount) == nil {
            if sl.TotalCount > 0 {
                sl.SuccessRate = float64(sl.SuccessCount) / float64(sl.TotalCount)
            }
            result = append(result, sl)
        }
    }
    if result == nil {
        result = []stepLearning{}
    }
    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(result)
}

// ─────────────────────────────────────────────────────────────────────────────
// Maintenance Windows (ITEM 5)
// ─────────────────────────────────────────────────────────────────────────────

// maintenanceHandler handles POST/DELETE /api/v1/projects/{id}/maintenance
func maintenanceHandler(w http.ResponseWriter, r *http.Request) {
    parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/projects/"), "/")
    if len(parts) < 1 || parts[0] == "" {
        http.Error(w, "invalid path", http.StatusBadRequest)
        return
    }
    projectID := parts[0]

    pool := getDB()
    ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
    defer cancel()

    if r.Method == http.MethodPost {
        var body struct {
            ServiceID       string `json:"serviceId"`
            DurationMinutes int    `json:"durationMinutes"`
            Reason          string `json:"reason"`
        }
        if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
            http.Error(w, "bad request", http.StatusBadRequest)
            return
        }
        // Sanitize and validate inputs (ITEM 7)
        body.Reason = sanitizeString(body.Reason, 256)
        if body.DurationMinutes <= 0 {
            body.DurationMinutes = 15
        }
        if body.DurationMinutes < 1 || body.DurationMinutes > 1440 {
            http.Error(w, "durationMinutes must be between 1 and 1440", http.StatusBadRequest)
            return
        }

        createdBy := "operator"
        if claims := GetClaims(r); claims != nil {
            if email, ok := claims["email"].(string); ok && email != "" {
                createdBy = email
            }
        }

        var id string
        var query string
        var args []interface{}
        if body.ServiceID != "" {
            query = `INSERT INTO maintenance_windows (project_id, service_id, reason, created_by, ends_at)
                     VALUES ($1,$2,$3,$4, NOW() + ($5 || ' minutes')::interval)
                     RETURNING id`
            args = []interface{}{projectID, body.ServiceID, body.Reason, createdBy, body.DurationMinutes}
        } else {
            query = `INSERT INTO maintenance_windows (project_id, reason, created_by, ends_at)
                     VALUES ($1,$2,$3, NOW() + ($4 || ' minutes')::interval)
                     RETURNING id`
            args = []interface{}{projectID, body.Reason, createdBy, body.DurationMinutes}
        }

        var scanErr error
        if pool != nil {
            scanErr = pool.QueryRow(ctx, query, args...).Scan(&id)
        } else {
            dsn := os.Getenv("PG_DSN")
            if dsn == "" {
                dsn = "postgres://postgres:postgres@postgres:5432/faultiq?sslmode=disable"
            }
            conn, err := pgx.Connect(ctx, dsn)
            if err != nil {
                http.Error(w, "db connect failed", http.StatusInternalServerError)
                return
            }
            defer conn.Close(ctx)
            scanErr = conn.QueryRow(ctx, query, args...).Scan(&id)
        }
        if scanErr != nil {
            http.Error(w, "db insert failed: "+scanErr.Error(), http.StatusInternalServerError)
            return
        }

        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusCreated)
        _ = json.NewEncoder(w).Encode(map[string]interface{}{
            "id":              id,
            "projectId":       projectID,
            "serviceId":       body.ServiceID,
            "reason":          body.Reason,
            "createdBy":       createdBy,
            "durationMinutes": body.DurationMinutes,
        })
        return
    }

    if r.Method == http.MethodDelete {
        var execErr error
        if pool != nil {
            _, execErr = pool.Exec(ctx,
                `UPDATE maintenance_windows SET ends_at = NOW() WHERE project_id=$1 AND ends_at > NOW()`, projectID)
        } else {
            dsn := os.Getenv("PG_DSN")
            if dsn == "" {
                dsn = "postgres://postgres:postgres@postgres:5432/faultiq?sslmode=disable"
            }
            conn, err := pgx.Connect(ctx, dsn)
            if err != nil {
                http.Error(w, "db connect failed", http.StatusInternalServerError)
                return
            }
            defer conn.Close(ctx)
            _, execErr = conn.Exec(ctx,
                `UPDATE maintenance_windows SET ends_at = NOW() WHERE project_id=$1 AND ends_at > NOW()`, projectID)
        }
        if execErr != nil {
            http.Error(w, "db update failed: "+execErr.Error(), http.StatusInternalServerError)
            return
        }
        w.WriteHeader(http.StatusNoContent)
        return
    }

    http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

// ─────────────────────────────────────────────────────────────────────────────
// Incident Assignment (ITEM 8)
// ─────────────────────────────────────────────────────────────────────────────

// assignIncidentHandler handles PATCH /api/v1/incidents/{id}/assign
func assignIncidentHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPatch {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }

    parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/incidents/"), "/")
    if len(parts) < 1 || parts[0] == "" {
        http.Error(w, "invalid path", http.StatusBadRequest)
        return
    }
    incidentID := parts[0]

    var body struct {
        AssignTo *string `json:"assignTo"`
    }
    if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
        http.Error(w, "bad request", http.StatusBadRequest)
        return
    }

    // Sanitize and validate (ITEM 7)
    if body.AssignTo != nil {
        sanitized := sanitizeString(*body.AssignTo, 256)
        body.AssignTo = &sanitized
        if *body.AssignTo != "" && !strings.Contains(*body.AssignTo, "@") {
            http.Error(w, "assignTo must be empty or a valid email address", http.StatusBadRequest)
            return
        }
    }

    ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
    defer cancel()

    var execErr error
    pool := getDB()
    if body.AssignTo == nil || *body.AssignTo == "" {
        // Unassign
        query := `UPDATE incidents SET assigned_to=NULL, assigned_at=NULL, updated_at=NOW() WHERE id=$1`
        if pool != nil {
            _, execErr = pool.Exec(ctx, query, incidentID)
        } else {
            dsn := os.Getenv("PG_DSN")
            if dsn == "" {
                dsn = "postgres://postgres:postgres@postgres:5432/faultiq?sslmode=disable"
            }
            conn, err := pgx.Connect(ctx, dsn)
            if err != nil {
                http.Error(w, "db connect failed", http.StatusInternalServerError)
                return
            }
            defer conn.Close(ctx)
            _, execErr = conn.Exec(ctx, query, incidentID)
        }
    } else {
        // Assign
        query := `UPDATE incidents SET assigned_to=$1, assigned_at=NOW(), updated_at=NOW() WHERE id=$2`
        if pool != nil {
            _, execErr = pool.Exec(ctx, query, *body.AssignTo, incidentID)
        } else {
            dsn := os.Getenv("PG_DSN")
            if dsn == "" {
                dsn = "postgres://postgres:postgres@postgres:5432/faultiq?sslmode=disable"
            }
            conn, err := pgx.Connect(ctx, dsn)
            if err != nil {
                http.Error(w, "db connect failed", http.StatusInternalServerError)
                return
            }
            defer conn.Close(ctx)
            _, execErr = conn.Exec(ctx, query, *body.AssignTo, incidentID)
        }
    }

    if execErr != nil {
        http.Error(w, "db update failed: "+execErr.Error(), http.StatusInternalServerError)
        return
    }

    // Publish incident_updated event to Redis
    raddr := os.Getenv("REDIS_ADDR")
    if raddr == "" {
        raddr = "redis:6379"
    }
    rdb := redis.NewClient(&redis.Options{Addr: raddr})
    assignTo := ""
    if body.AssignTo != nil {
        assignTo = *body.AssignTo
    }
    event := map[string]interface{}{
        "type":       "incident_updated",
        "incidentId": incidentID,
        "assignedTo": assignTo,
    }
    b, _ := json.Marshal(event)
    _ = rdb.Publish(r.Context(), "incident_events", b).Err()
    _ = rdb.Close()

    w.WriteHeader(http.StatusNoContent)
}

// ─────────────────────────────────────────────────────────────────────────────
// Related Incidents (ITEM 9)
// ─────────────────────────────────────────────────────────────────────────────

// relatedIncidentsHandler handles GET /api/v1/incidents/{id}/related
func relatedIncidentsHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }

    parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/incidents/"), "/")
    if len(parts) < 1 || parts[0] == "" {
        http.Error(w, "invalid path", http.StatusBadRequest)
        return
    }
    incidentID := parts[0]

    ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
    defer cancel()

    type relatedItem struct {
        ID         string  `json:"id"`
        Service    string  `json:"service"`
        Status     string  `json:"status"`
        Phase      string  `json:"phase"`
        Confidence float64 `json:"confidence"`
    }

    var related []relatedItem

    query := `SELECT id, COALESCE(service,''), COALESCE(status,'OPEN'), COALESCE(phase,'DETECTING'), COALESCE(confidence,0)
              FROM incidents
              WHERE root_cause_candidate = (SELECT root_cause_candidate FROM incidents WHERE id=$1)
              AND id != $1
              AND status IN ('OPEN','ACKNOWLEDGED')
              AND detected_at > NOW() - INTERVAL '2 hours'
              ORDER BY detected_at DESC LIMIT 10`

    pool := getDB()
    var execRows pgx.Rows
    var qErr error
    if pool != nil {
        execRows, qErr = pool.Query(ctx, query, incidentID)
    } else {
        dsn := os.Getenv("PG_DSN")
        if dsn == "" {
            dsn = "postgres://postgres:postgres@postgres:5432/faultiq?sslmode=disable"
        }
        conn, err := pgx.Connect(ctx, dsn)
        if err != nil {
            w.Header().Set("Content-Type", "application/json")
            _ = json.NewEncoder(w).Encode(map[string]interface{}{"incidents": []interface{}{}})
            return
        }
        defer conn.Close(ctx)
        execRows, qErr = conn.Query(ctx, query, incidentID)
    }

    if qErr == nil {
        defer execRows.Close()
        for execRows.Next() {
            var item relatedItem
            if execRows.Scan(&item.ID, &item.Service, &item.Status, &item.Phase, &item.Confidence) == nil {
                related = append(related, item)
            }
        }
    }

    if related == nil {
        related = []relatedItem{}
    }
    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(map[string]interface{}{"incidents": related})
}
