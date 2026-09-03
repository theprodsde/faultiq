package main

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "log"
    "math"
    "net/http"
    "net/url"
    "os"
    "strconv"
    "strings"
    "sync"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promhttp"
    "github.com/redis/go-redis/v9"
    "github.com/faultiq/graphclient"
)

type Signal struct {
    Tenant             string  `json:"tenant"`
    TenantID           string  `json:"tenantId"`
    Project            string  `json:"project"`
    ProjectID          string  `json:"projectId"`
    Environment        string  `json:"environment"`
    Service            string  `json:"service"`
    StatusClass        string  `json:"statusClass"`
    ErrorRate          float64 `json:"errorRate"`
    LatencyP95         float64 `json:"latencyP95"`
    Metric             string  `json:"metric"`
    Value              float64 `json:"value"`
    Timestamp          int64   `json:"timestamp"`
    ErrorRateThreshold float64 `json:"errorRateThreshold,omitempty"` // per-service threshold from health-poller
    MinSignalCount     int     `json:"minSignalCount,omitempty"`     // consecutive unhealthy polls required (default 1)
}

// Prometheus metrics — registered once at startup.
var (
    signalsReceivedTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
        Name: "signals_received_total",
        Help: "Total number of signals received by the detection engine.",
    }, []string{"service", "statusClass"})

    incidentsCreatedTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
        Name: "incidents_created_total",
        Help: "Total number of incidents created.",
    }, []string{"faultType"})

    sopPhaseTransitionsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
        Name: "sop_phase_transitions_total",
        Help: "Total number of SOP phase transitions.",
    }, []string{"from", "to"})

    bfsDurationMs = prometheus.NewHistogram(prometheus.HistogramOpts{
        Name:    "bfs_duration_ms",
        Help:    "Duration of BFS detection in milliseconds.",
        Buckets: []float64{1, 5, 10, 25, 50, 100, 250},
    })
)

func init() {
    prometheus.MustRegister(signalsReceivedTotal)
    prometheus.MustRegister(incidentsCreatedTotal)
    prometheus.MustRegister(sopPhaseTransitionsTotal)
    prometheus.MustRegister(bfsDurationMs)
}

// deployGate tracks an active deployment health gate.
type deployGate struct {
    serviceID string
    projectID string
    expiresAt time.Time
}

// Detector encapsulates dependencies for detection logic.
type Detector struct {
    gp             GraphProvider
    store          Store
    graphCache     *GraphCache
    namespace      string
    pgDSN          string
    redisAddr      string
    conn           *pgxpool.Pool  // pool — safe for concurrent goroutine use
    nsCache        sync.Map       // projectID → resolved namespace (cache)
    rdb            *redis.Client
}

func NewDetector(ctx context.Context) (*Detector, error) {
    gmURL := os.Getenv("GRAPH_MANAGER_URL")
    if gmURL == "" {
        gmURL = "http://graph-manager:8086/api/v1/graphs"
    }
    ns := os.Getenv("NAMESPACE")
    if ns == "" {
        ns = "payments-platform:prod"
    }
    pg := os.Getenv("PG_DSN")
    if pg == "" {
        return nil, fmt.Errorf("PG_DSN is required in environment")
    }
    raddr := os.Getenv("REDIS_ADDR")
    if raddr == "" {
        raddr = "redis:6379"
    }

    pool, err := pgxpool.New(ctx, pg)
    if err != nil {
        return nil, fmt.Errorf("pg pool: %w", err)
    }
    if err := ensureSchema(ctx, pool); err != nil {
        pool.Close()
        return nil, err
    }

    rdb := redis.NewClient(&redis.Options{Addr: raddr})
    if err := rdb.Ping(ctx).Err(); err != nil {
        log.Printf("warning: redis ping failed: %v", err)
    }

    gp := graphclient.NewHTTPProvider(gmURL)
    store := NewPGStore(pool)

    cache := NewGraphCache(gp)
    cache.Preload(ctx, []string{ns, "payments-platform:prod", "orders-platform:prod"})

    return &Detector{gp: gp, store: store, graphCache: cache, namespace: ns, pgDSN: pg, redisAddr: raddr, conn: pool, rdb: rdb}, nil
}

func (d *Detector) Close(ctx context.Context) {
    if d.conn != nil {
        d.conn.Close()
    }
    if d.rdb != nil {
        d.rdb.Close()
    }
}

// deduplication window: signals from the same service within this window are grouped into one incident
const deduplicationWindow = 5 * time.Minute

// defaultPollInterval matches health-poller's default interval; used for consecutive-check TTL.
const defaultPollInterval = 30 * time.Second

// recentIncidents tracks recently created incidents for deduplication
type recentIncidentEntry struct {
    incidentID string
    service    string
    namespace  string
    createdAt  time.Time
}

var (
    recentMu        sync.Mutex
    recentIncidents []recentIncidentEntry
)

// findRecentIncident checks if an incident already exists for this service within the dedup window
func findRecentIncident(service, namespace string) (string, bool) {
    recentMu.Lock()
    defer recentMu.Unlock()
    cutoff := time.Now().Add(-deduplicationWindow)
    // also prune old entries
    pruned := recentIncidents[:0]
    var found string
    for _, entry := range recentIncidents {
        if entry.createdAt.After(cutoff) {
            pruned = append(pruned, entry)
            if entry.service == service && entry.namespace == namespace {
                found = entry.incidentID
            }
        }
    }
    recentIncidents = pruned
    return found, found != ""
}

// recordRecentIncident registers a new incident for deduplication
func recordRecentIncident(incidentID, service, namespace string) {
    recentMu.Lock()
    defer recentMu.Unlock()
    recentIncidents = append(recentIncidents, recentIncidentEntry{
        incidentID: incidentID, service: service, namespace: namespace, createdAt: time.Now(),
    })
}

// findRecentIncidentRedis checks Redis for a recent incident for this service within the dedup window.
// Falls back to in-memory if Redis is unavailable.
func (d *Detector) findRecentIncidentRedis(ctx context.Context, service, namespace string) (string, bool) {
    key := fmt.Sprintf("incident_dedup:%s:%s", service, namespace)
    val, err := d.rdb.Get(ctx, key).Result()
    if err != nil {
        // Redis unavailable — fall back to in-memory
        return findRecentIncident(service, namespace)
    }
    return val, true
}

// recordRecentIncidentRedis stores a new incident ID in Redis with TTL = deduplicationWindow.
// Falls back to in-memory if Redis is unavailable.
func (d *Detector) recordRecentIncidentRedis(ctx context.Context, incidentID, service, namespace string) {
    key := fmt.Sprintf("incident_dedup:%s:%s", service, namespace)
    if err := d.rdb.SetEx(ctx, key, incidentID, deduplicationWindow).Err(); err != nil {
        // Redis unavailable — fall back to in-memory
        recordRecentIncident(incidentID, service, namespace)
    }
}

// recordSignalHistory inserts a signal into the signal_history table for SLO tracking.
func (d *Detector) recordSignalHistory(ctx context.Context, s Signal, namespace string) {
    if d.conn == nil {
        return
    }
    _, _ = d.conn.Exec(ctx,
        `INSERT INTO signal_history (tenant_id, project_id, service_id, namespace, status_class, error_rate, latency_p95, source)
         VALUES ($1,$2,$3,$4,$5,$6,$7,'health-poller')`,
        s.TenantID, s.ProjectID, s.Service, namespace, s.StatusClass, s.ErrorRate, s.LatencyP95)
}

// Run subscribes to Redis channel "signals" and processes incoming signals.
func (d *Detector) Run(ctx context.Context) error {
    sub := d.rdb.Subscribe(ctx, "signals")
    // consume subscription confirmation
    if _, err := sub.Receive(ctx); err != nil {
        return fmt.Errorf("subscribe failed: %w", err)
    }
    ch := sub.Channel()
    log.Println("detection-engine: subscribed to signals channel")

    // Start auto-recovery checker in background
    go d.autoRecoveryLoop(ctx)

    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case msg, ok := <-ch:
            if !ok {
                return fmt.Errorf("signals channel closed")
            }
            var s Signal
            if err := json.Unmarshal([]byte(msg.Payload), &s); err != nil {
                log.Printf("invalid signal payload: %v", err)
                continue
            }
            go d.handleSignal(context.Background(), s)
        }
    }
}

// autoRecoveryLoop checks for healthy signals on services with open incidents and auto-resolves them
func (d *Detector) autoRecoveryLoop(ctx context.Context) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            d.checkAutoRecovery(ctx)
            d.checkSLAEscalations(ctx)
        }
    }
}

// isInMaintenance checks if a maintenance window is active for this service or project.
func (d *Detector) isInMaintenance(ctx context.Context, projectID, serviceID string) bool {
    if d.conn == nil {
        return false
    }
    ctx2, cancel := context.WithTimeout(ctx, 1*time.Second)
    defer cancel()
    var count int
    _ = d.conn.QueryRow(ctx2,
        `SELECT COUNT(*) FROM maintenance_windows
         WHERE project_id=$1 AND ends_at > NOW()
         AND (service_id IS NULL OR service_id=$2)`,
        projectID, serviceID).Scan(&count)
    return count > 0
}

// checkSLAEscalations finds incidents in TRIAGING/FIXING with no sop_audit activity in the SLA window
// and re-fires webhook notifications and writes an escalation audit record.
func (d *Detector) checkSLAEscalations(ctx context.Context) {
    if d.conn == nil {
        return
    }

    slaMinutes := 30 // default: escalate after 30 min in TRIAGING/FIXING with no action
    if v := os.Getenv("SLA_ESCALATION_MINUTES"); v != "" {
        if n, _ := strconv.Atoi(v); n > 0 {
            slaMinutes = n
        }
    }

    ctx2, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()

    rows, err := d.conn.Query(ctx2,
        `SELECT i.id, COALESCE(i.service,''), COALESCE(i.project_id,''), COALESCE(i.tenant_id,''),
                i.phase, i.detected_at
         FROM incidents i
         WHERE i.status = 'OPEN'
         AND i.phase IN ('TRIAGING', 'FIXING')
         AND NOT EXISTS (
             SELECT 1 FROM sop_audit a
             WHERE a.incident_id = i.id
             AND a.created_at > NOW() - ($1 || ' minutes')::interval
         )
         ORDER BY i.detected_at ASC LIMIT 10`,
        slaMinutes)
    if err != nil {
        return
    }
    defer rows.Close()

    for rows.Next() {
        var id, service, projectID, tenantID, phase string
        var detectedAt time.Time
        if rows.Scan(&id, &service, &projectID, &tenantID, &phase, &detectedAt) != nil {
            continue
        }

        ageMin := time.Since(detectedAt).Minutes()
        reasoningLog(map[string]interface{}{
            "event":       "sla_escalation",
            "incident_id": id,
            "service":     service,
            "phase":       phase,
            "age_minutes": ageMin,
            "reasoning":   fmt.Sprintf("ESCALATION: incident %s in phase %s for %.0f minutes with no action (SLA: %d min)", id, phase, ageMin, slaMinutes),
        })

        // Re-fire webhook notification for escalation
        go d.sendWebhookNotification(ctx, id, projectID, tenantID)

        // Write escalation audit record
        _, _ = d.conn.Exec(ctx2,
            `INSERT INTO sop_audit (incident_id, action, actor, phase_from, phase_to)
             VALUES ($1, 'sla_escalation', 'system', $2, $2)`,
            id, phase)
    }
}

// checkTopologyDrift runs every 15 minutes, comparing each project's graph diff against its snapshot.
// If new edges appear, a topology_drift event is published to Redis.
func (d *Detector) checkTopologyDrift(ctx context.Context) {
    ticker := time.NewTicker(15 * time.Minute)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            d.runTopologyDriftCheck(ctx)
        }
    }
}

func (d *Detector) runTopologyDriftCheck(ctx context.Context) {
    if d.conn == nil {
        return
    }
    apiURL := os.Getenv("API_GATEWAY_URL")
    if apiURL == "" {
        apiURL = "http://api-gateway:8080"
    }

    ctx2, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()

    rows, err := d.conn.Query(ctx2, `SELECT id FROM projects LIMIT 50`)
    if err != nil {
        return
    }
    defer rows.Close()

    for rows.Next() {
        var projectID string
        if rows.Scan(&projectID) != nil {
            continue
        }
        go d.checkProjectTopologyDrift(ctx, projectID, apiURL)
    }
}

func (d *Detector) checkProjectTopologyDrift(ctx context.Context, projectID, apiURL string) {
    ctx2, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()

    req, err := http.NewRequestWithContext(ctx2, "GET",
        fmt.Sprintf("%s/api/v1/projects/%s/graphs/diff", apiURL, projectID), nil)
    if err != nil {
        return
    }
    resp, err := http.DefaultClient.Do(req)
    if err != nil || resp.StatusCode != 200 {
        if resp != nil {
            resp.Body.Close()
        }
        return
    }
    defer resp.Body.Close()

    var diff struct {
        AddedEdges []struct {
            From string `json:"from"`
            To   string `json:"to"`
        } `json:"addedEdges"`
    }
    if json.NewDecoder(resp.Body).Decode(&diff) != nil {
        return
    }

    if len(diff.AddedEdges) == 0 {
        return
    }

    for _, edge := range diff.AddedEdges {
        reasoningLog(map[string]interface{}{
            "event":      "topology_drift",
            "project_id": projectID,
            "from":       edge.From,
            "to":         edge.To,
            "reasoning":  fmt.Sprintf("TOPOLOGY_DRIFT: %s now calls %s (not in baseline)", edge.From, edge.To),
        })
    }
    event := map[string]interface{}{
        "type":        "topology_drift",
        "project_id":  projectID,
        "added_edges": diff.AddedEdges,
    }
    eventJSON, _ := json.Marshal(event)
    _ = d.rdb.Publish(ctx, "deployment_events", eventJSON).Err()
}

func (d *Detector) checkAutoRecovery(ctx context.Context) {
    if d.conn == nil {
        return
    }
    ctx2, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    // Find OPEN incidents older than 2 minutes that have had no new error signals
    rows, err := d.conn.Query(ctx2,
        `SELECT id, service, project_id FROM incidents
         WHERE status = 'OPEN' AND detected_at < NOW() - INTERVAL '2 minutes'
         ORDER BY detected_at ASC LIMIT 20`)
    if err != nil {
        return
    }
    defer rows.Close()
    for rows.Next() {
        var id, service, projectID string
        if err := rows.Scan(&id, &service, &projectID); err != nil {
            continue
        }
        // Don't auto-resolve incidents actively being worked (TRIAGING/FIXING/VERIFYING)
        var phase string
        _ = d.conn.QueryRow(ctx2, `SELECT COALESCE(phase,'NARROWING') FROM incidents WHERE id=$1`, id).Scan(&phase)
        if phase == "TRIAGING" || phase == "FIXING" || phase == "VERIFYING" {
            continue
        }
        // Check if this service still has recent signals in the dedup window.
        // Use the resolved namespace (not projectID) for the check.
        ns := d.resolveNamespace(ctx2, projectID, "prod")
        _, stillActive := d.findRecentIncidentRedis(ctx2, service, ns)
        if !stillActive {
            // Auto-resolve
            _, _ = d.conn.Exec(ctx2,
                `UPDATE incidents SET status = 'RESOLVED', phase = 'RESOLVED', resolved_at = NOW(), updated_at = NOW() WHERE id = $1 AND status = 'OPEN'`, id)
            log.Printf("auto-resolved incident %s for service %s (no recent signals)", id, service)
            // Clear Redis dedup key so new incident can be created after recovery
            d.rdb.Del(ctx2, fmt.Sprintf("incident_dedup:%s:%s", service, ns))
            event := map[string]string{"type": "incident_resolved", "incidentId": id, "service": service, "projectId": projectID}
            eventJSON, _ := json.Marshal(event)
            _ = d.rdb.Publish(ctx2, "incident_events", eventJSON).Err()
        }
    }
}

// resolveNamespace takes a projectID and environment, and returns the correct namespace
// by looking up the project slug in Postgres, falling back to sanitizing the projectID
func (d *Detector) resolveNamespace(ctx context.Context, projectID, environment string) string {
    if environment == "" {
        environment = "prod"
    }
    // Fast path: namespace already cached for this projectID
    cacheKey := projectID + ":" + environment
    if cached, ok := d.nsCache.Load(cacheKey); ok {
        return cached.(string)
    }

    // Query project slug from pool (goroutine-safe)
    if d.conn != nil {
        ctx2, cancel := context.WithTimeout(ctx, 2*time.Second)
        defer cancel()
        var slug string
        err := d.conn.QueryRow(ctx2, "SELECT slug FROM projects WHERE id=$1", projectID).Scan(&slug)
        if err == nil && slug != "" {
            ns := fmt.Sprintf("%s:%s", slug, environment)
            d.nsCache.Store(cacheKey, ns)
            return ns
        }
    }

    // Fallback: sanitize common prefixes from projectID
    pid := strings.TrimPrefix(projectID, "p_")
    pid = strings.TrimPrefix(pid, "proj-")
    return fmt.Sprintf("%s:%s", pid, environment)
}

func (d *Detector) handleSignal(ctx context.Context, s Signal) {
    startTime := time.Now()

    // Normalise tenant/project from either field name convention
    tenantID := s.TenantID
    if tenantID == "" {
        tenantID = s.Tenant
    }
    projectID := s.ProjectID
    if projectID == "" {
        projectID = s.Project
    }

    // Skip signal if in maintenance window
    if d.isInMaintenance(ctx, projectID, s.Service) {
        log.Printf("signal suppressed: %s/%s in maintenance window", projectID, s.Service)
        return
    }

    // Resolve namespace from projectID and environment
    namespace := d.resolveNamespace(ctx, projectID, s.Environment)

    // ── FAST PATH: Get graph from in-memory cache (no HTTP, <1ms) ──
    g := d.graphCache.Get(ctx, namespace)
    
    // Apply signal status to the impacted node
    impactedNode, nodeExists := g.Nodes[s.Service]
    if !nodeExists {
        log.Printf("warning: signal service %s not found in namespace=%s graph", s.Service, namespace)
        return
    }
    
    // Snapshot the previous state to detect transient vs persistent
    previousStatus := impactedNode.StatusClass
    impactedNode.StatusClass = s.StatusClass
    impactedNode.LatencyP95 = int(s.LatencyP95)

    // ── FAST PATH: Classify the fault type immediately (<1ms) ──
    faultType := ClassifyFault(s.StatusClass, s.ErrorRate, int(s.LatencyP95))
    
    // Increment signals_received_total Prometheus counter
    signalsReceivedTotal.WithLabelValues(s.Service, s.StatusClass).Inc()

    // If it was already failing and now recovering → mark as transient, auto-resolve
    if isHealthyStatus(s.StatusClass) && !isHealthyStatus(previousStatus) {
        log.Printf("recovery detected: %s went from %s → %s (transient)", s.Service, previousStatus, s.StatusClass)
        // Clear consecutive counter on recovery
        d.rdb.Del(ctx, fmt.Sprintf("consec:%s:%s", s.Service, namespace))
        existingID, exists := d.findRecentIncidentRedis(ctx, s.Service, namespace)
        if exists {
            // Fire-and-forget: resolve the incident
            go func() {
                _, _ = d.conn.Exec(context.Background(),
                    `UPDATE incidents SET status = 'RESOLVED', phase = 'RESOLVED', resolved_at = NOW(), updated_at = NOW() WHERE id = $1 AND status = 'OPEN'`, existingID)
                event := map[string]interface{}{
                    "type": "incident_resolved", "incidentId": existingID,
                    "service": s.Service, "projectId": projectID, "tenantId": tenantID,
                    "transient": true,
                }
                eventJSON, _ := json.Marshal(event)
                _ = d.rdb.Publish(context.Background(), "incident_events", eventJSON).Err()
            }()
        }
        return
    }

    // If signal is healthy, nothing to detect — but check if a VERIFYING incident can be resolved
    // Use per-service threshold if provided by health-poller; fall back to global 0.1 default
    healthThreshold := 0.1
    if s.ErrorRateThreshold > 0 {
        healthThreshold = s.ErrorRateThreshold
    }
    if isHealthyStatus(s.StatusClass) && s.ErrorRate < healthThreshold {
        // Clear consecutive unhealthy counter when service recovers
        d.rdb.Del(ctx, fmt.Sprintf("consec:%s:%s", s.Service, namespace))
        go d.handleHealthySignal(context.Background(), s.Service, s)
        return
    }

    // ── FAST PATH: BFS detection on in-memory graph (<5ms for typical graphs) ──
    bfsStart := time.Now()
    impacted := []string{s.Service}
    suspects := DetectSuspects(g, impacted, 5)
    bfsDurationMs.Observe(float64(time.Since(bfsStart).Milliseconds()))

    // ── Deduplication: check if an incident already exists ──
    existingID, exists := d.findRecentIncidentRedis(ctx, s.Service, namespace)
    if exists {
        // Accumulate evidence and re-rank — this is the SOP NARROWING step
        go d.accumulateEvidence(context.Background(), existingID, s, namespace, g)
        return
    }

    // ── ITEM 4: Consecutive signal threshold check ──
    minRequired := s.MinSignalCount
    if minRequired <= 0 {
        minRequired = 1
    }
    if minRequired > 1 {
        consecKey := fmt.Sprintf("consec:%s:%s", s.Service, namespace)
        count, _ := d.rdb.Incr(ctx, consecKey).Result()
        d.rdb.Expire(ctx, consecKey, 2*defaultPollInterval)
        if int(count) < minRequired {
            log.Printf("consecutive check: %s count=%d/%d — not enough to alert", s.Service, count, minRequired)
            return
        }
    }

    // ── FAST PATH: Generate recommendations immediately (<1ms) ──
    nodeType := "SERVICE"
    if impactedNode != nil {
        // node type isn't stored in detection graph — use from graphclient cache
    }
    // Use inline recommendations as the fast-path default.
    // The async path (persistIncidentAsync) will call the recommendation service
    // and REPLACE these with the external playbooks — avoiding duplication.
    recommendations := GenerateRecommendations(faultType, s.Service, nodeType)

    // Generate incident ID
    incidentID := fmt.Sprintf("inc-%d", time.Now().UnixMilli())

    // ── FAST PATH: Publish to Redis FIRST for instant frontend notification (<2ms) ──
    // This is the key latency optimization: notify the UI before doing DB writes
    incidentEvent := map[string]interface{}{
        "type":            "incident_created",
        "incidentId":      incidentID,
        "service":         s.Service,
        "projectId":       projectID,
        "tenantId":        tenantID,
        "faultType":       string(faultType),
        "statusClass":     s.StatusClass,
        "errorRate":       s.ErrorRate,
        "latencyP95":      s.LatencyP95,
        "suspects":        suspects,
        "rootCause":       s.Service, // preliminary — RCA ranker refines async
        "confidence":      0.8,
        "status":          "OPEN",
        "recommendations": recommendations,
        "detectedInMs":    time.Since(startTime).Milliseconds(),
    }
    eventJSON, _ := json.Marshal(incidentEvent)
    _ = d.rdb.Publish(ctx, "incident_events", eventJSON).Err()

    // Record signal in history for SLO tracking (fire-and-forget)
    go d.recordSignalHistory(context.Background(), s, namespace)

    detectionLatency := time.Since(startTime)
    // Structured reasoning log — machine-parseable and human-readable
    reasoningLog(map[string]interface{}{
        "event":           "fault_detected",
        "incident_id":     incidentID,
        "service":         s.Service,
        "namespace":       namespace,
        "fault_type":      string(faultType),
        "status_class":    s.StatusClass,
        "error_rate":      s.ErrorRate,
        "latency_p95_ms":  s.LatencyP95,
        "suspects":        suspects,
        "suspect_count":   len(suspects),
        "bfs_depth":       5,
        "detection_ms":    detectionLatency.Milliseconds(),
        "reasoning":       fmt.Sprintf("BFS from %s found %d suspects; fault classified as %s (statusClass=%s, errorRate=%.3f)", s.Service, len(suspects), faultType, s.StatusClass, s.ErrorRate),
    })

    // Record for deduplication (must happen before async work) — Redis-backed with in-memory fallback
    d.recordRecentIncidentRedis(ctx, incidentID, s.Service, namespace)

    // ── ASYNC PATH: Heavy operations in background (DB writes, RCA ranker) ──
    go d.persistIncidentAsync(ctx, incidentID, tenantID, projectID, s, suspects, faultType, recommendations, g)
}

// isHealthyStatus checks if a status class indicates a healthy node
func isHealthyStatus(status string) bool {
    return status == "" || status == "2xx" || (len(status) > 0 && status[0] == '2')
}

// persistIncidentAsync handles all slow operations after the detection event is already published
func (d *Detector) persistIncidentAsync(ctx context.Context, incidentID, tenantID, projectID string, s Signal, suspects []string, faultType FaultType, recommendations []Recommendation, g *Graph) {
    // Call RCA Ranker for refined confidence scoring
    rcaRankerURL := os.Getenv("RCA_RANKER_URL")
    if rcaRankerURL == "" {
        rcaRankerURL = "http://rca-ranker:8087"
    }
    
    type rcaService struct {
        EvidenceCount int     `json:"evidence_count"`
        Impact        float64 `json:"impact"`
        ErrorRate     float64 `json:"error_rate"`
    }
    // Build reverse edge map to compute caller-based blast radius impact
    reverseEdges := make(map[string][]string)
    for from, tos := range g.Edges {
        for _, to := range tos {
            reverseEdges[to] = append(reverseEdges[to], from)
        }
    }
    totalNodes := len(g.Nodes)
    if totalNodes == 0 {
        totalNodes = 1
    }
    rcaServices := make(map[string]rcaService)
    for _, sid := range suspects {
        node := g.Nodes[sid]
        callerCount := len(reverseEdges[sid])
        impact := float64(callerCount) / float64(totalNodes)
        if impact > 1.0 {
            impact = 1.0
        }
        errorRate := 0.0
        if node != nil {
            errorRate = node.ErrorRate
        }
        rcaServices[sid] = rcaService{EvidenceCount: 1, Impact: impact, ErrorRate: errorRate}
    }
    if _, ok := rcaServices[s.Service]; !ok {
        node := g.Nodes[s.Service]
        callerCount := len(reverseEdges[s.Service])
        impact := float64(callerCount) / float64(totalNodes)
        errorRate := s.ErrorRate
        if node != nil {
            errorRate = node.ErrorRate
        }
        rcaServices[s.Service] = rcaService{EvidenceCount: 1, Impact: impact, ErrorRate: errorRate}
    }

    rootCause := s.Service
    confidence := 0.8
    type rankedCandidate struct {
        Service    string  `json:"service"`
        Confidence float64 `json:"confidence"`
        Rank       int     `json:"rank"`
    }
    var rankedCandidates []rankedCandidate

    rcaPayload := map[string]interface{}{
        "incident_id": incidentID,
        "services":    rcaServices,
    }
    rcaBody, _ := json.Marshal(rcaPayload)
    rcaCtx, rcaCancel := context.WithTimeout(context.Background(), 3*time.Second)
    defer rcaCancel()
    
    req, _ := http.NewRequestWithContext(rcaCtx, "POST", rcaRankerURL+"/rank", bytes.NewReader(rcaBody))
    req.Header.Set("Content-Type", "application/json")
    resp, rcaErr := http.DefaultClient.Do(req)
    if rcaErr == nil && resp.StatusCode == 200 {
        var rcaResp struct {
            Candidates []rankedCandidate `json:"candidates"`
            RootCause  string            `json:"root_cause"`
        }
        if json.NewDecoder(resp.Body).Decode(&rcaResp) == nil && len(rcaResp.Candidates) > 0 {
            rankedCandidates = rcaResp.Candidates
            rootCause = rcaResp.RootCause
            confidence = rcaResp.Candidates[0].Confidence
        }
        resp.Body.Close()
    } else {
        // Fallback scoring
        for i, sid := range suspects {
            c := 0.85 - float64(i)*0.08
            if c < 0.1 {
                c = 0.1
            }
            rankedCandidates = append(rankedCandidates, rankedCandidate{
                Service: sid, Confidence: c, Rank: i + 1,
            })
        }
        if len(suspects) > 0 {
            rootCause = suspects[0]
            confidence = 0.85
        }
    }

    // ── Call recommendation service — REPLACE inline recommendations to avoid duplication ──
    recSvcURL := os.Getenv("RECOMMENDATION_SERVICE_URL")
    if recSvcURL == "" {
        recSvcURL = "http://recommendation:8090"
    }
    symptom := string(faultType)
    recResp, recErr := http.Get(recSvcURL + "/recommend?symptom=" + url.QueryEscape(symptom))
    if recErr == nil && recResp.StatusCode == 200 {
        var extRecs []struct {
            ID    string `json:"id"`
            Title string `json:"title"`
        }
        if json.NewDecoder(recResp.Body).Decode(&extRecs) == nil && len(extRecs) > 0 {
            // REPLACE (not append) inline recommendations with external playbooks
            // This prevents the same recommendations appearing twice
            externalRecs := make([]Recommendation, 0, len(extRecs))
            for i, er := range extRecs {
                _ = i
                externalRecs = append(externalRecs, Recommendation{
                    Title:    er.Title,
                    Category: "Playbook",
                    Priority: "high",
                    Steps:    []string{er.Title},
                    Reason:   fmt.Sprintf("From recommendation library (playbook: %s)", er.ID),
                })
            }
            recommendations = externalRecs
        }
        recResp.Body.Close()
    }

    // Serialize evidence
    type suspectItem struct {
        NodeID     string  `json:"nodeId"`
        Confidence float64 `json:"confidence"`
        FaultType  string  `json:"faultType"`
    }
    evidenceItems := make([]suspectItem, 0, len(rankedCandidates))
    for _, rc := range rankedCandidates {
        evidenceItems = append(evidenceItems, suspectItem{
            NodeID: rc.Service, Confidence: rc.Confidence, FaultType: string(faultType),
        })
    }
    evidenceJSON, _ := json.Marshal(evidenceItems)

    // Serialize recommendations
    recsJSON, _ := json.Marshal(recommendations)

    // Increment incidents_created_total Prometheus counter
    incidentsCreatedTotal.WithLabelValues(string(faultType)).Inc()

    // Persist incident to Postgres with initial phase DETECTING
    _, _ = d.conn.Exec(context.Background(),
        `INSERT INTO incidents (id, tenant_id, project_id, service, status, phase, detected_at, evidence, root_cause_candidate, confidence)
         VALUES ($1,$2,$3,$4,'OPEN','DETECTING',NOW(),$5,$6,$7) ON CONFLICT (id) DO NOTHING`,
        incidentID, tenantID, projectID, s.Service, string(evidenceJSON), rootCause, confidence,
    )
    // Immediately advance to NARROWING — first signal always starts the narrowing phase
    d.advancePhase(context.Background(), incidentID, projectID, tenantID, PhaseDetecting, PhaseNarrowing)

    // Persist RCA candidates
    for i, rc := range rankedCandidates {
        candidateID := fmt.Sprintf("%s-rca-%d", incidentID, i)
        _, _ = d.conn.Exec(context.Background(),
            `INSERT INTO rca_candidates (id, incident_id, candidate_node_id, confidence_score, evidence)
             VALUES ($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`,
            candidateID, incidentID, rc.Service, rc.Confidence, string(evidenceJSON),
        )
    }

    // Persist recommendations
    for i, rec := range recommendations {
        recID := fmt.Sprintf("%s-rec-%d", incidentID, i)
        stepsJSON, _ := json.Marshal(rec.Steps)
        _, _ = d.conn.Exec(context.Background(),
            `INSERT INTO recommendations (id, incident_id, rank, playbook_title, steps)
             VALUES ($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`,
            recID, incidentID, i+1, rec.Title, string(stepsJSON),
        )
    }

    // Check for recent deployments (within 30 min) — correlate with the fault
    recentDeploy := d.findRecentDeployment(context.Background(), projectID, s.Service, 30*time.Minute)
    if recentDeploy != nil {
        reasoningLog(map[string]interface{}{
            "event":       "deployment_correlation",
            "incident_id": incidentID,
            "service":     s.Service,
            "deploy_id":   recentDeploy["id"],
            "version":     recentDeploy["version"],
            "deployed_by": recentDeploy["deployed_by"],
            "age_minutes": recentDeploy["age_minutes"],
            "reasoning":   fmt.Sprintf("⚠ DEPLOYMENT CORRELATION: %s was deployed %.0f minutes ago by %s (version %s) — likely cause of fault", s.Service, recentDeploy["age_minutes"], recentDeploy["deployed_by"], recentDeploy["version"]),
        })
        // Add deployment info to the incident evidence
        _, _ = d.conn.Exec(context.Background(),
            `UPDATE incidents SET evidence = evidence || $1::jsonb WHERE id = $2`,
            fmt.Sprintf(`{"recentDeploy":{"version":"%s","deployedBy":"%s","ageMinutes":%v,"deployId":"%s"}}`,
                recentDeploy["version"], recentDeploy["deployed_by"], recentDeploy["age_minutes"], recentDeploy["id"]),
            incidentID)
    }

    // If RCA ranker refined the root cause, publish an update event
    if rootCause != s.Service {
        updateEvent := map[string]interface{}{
            "type": "incident_updated", "incidentId": incidentID,
            "rootCause": rootCause, "confidence": confidence,
            "projectId": projectID, "tenantId": tenantID,
            "recommendations": recsJSON,
            "recentDeploy": recentDeploy,
        }
        updateJSON, _ := json.Marshal(updateEvent)
        _ = d.rdb.Publish(context.Background(), "incident_events", updateJSON).Err()
    }
}

// findRecentDeployment checks if a service was deployed recently (within the given window).
// This correlates faults with deployments — when a deploy happened just before a fault, it's likely the cause.
func (d *Detector) findRecentDeployment(ctx context.Context, projectID, serviceID string, window time.Duration) map[string]interface{} {
    if d.conn == nil {
        return nil
    }
    ctx2, cancel := context.WithTimeout(ctx, 2*time.Second)
    defer cancel()

    var deployID, version, deployedBy string
    var deployedAt time.Time
    err := d.conn.QueryRow(ctx2,
        `SELECT id, COALESCE(version,'unknown'), COALESCE(deployed_by,'unknown'), deployed_at
         FROM deployments
         WHERE project_id=$1 AND service_id=$2 AND deployed_at > NOW() - $3::interval
         ORDER BY deployed_at DESC LIMIT 1`,
        projectID, serviceID, fmt.Sprintf("%d seconds", int(window.Seconds()))).
        Scan(&deployID, &version, &deployedBy, &deployedAt)
    if err != nil {
        return nil
    }
    ageMinutes := time.Since(deployedAt).Minutes()
    return map[string]interface{}{
        "id":          deployID,
        "version":     version,
        "deployed_by": deployedBy,
        "deployed_at": deployedAt.UTC().Format(time.RFC3339),
        "age_minutes": ageMinutes,
    }
}

// ─────────────────────────────────────────────────────────────────────────────
// SOP Phase State Machine
// ─────────────────────────────────────────────────────────────────────────────

// Phase represents the current stage of an incident in the SOP pipeline.
type Phase string

const (
    PhaseDetecting  Phase = "DETECTING"
    PhaseNarrowing  Phase = "NARROWING"
    PhaseConfirmed  Phase = "CONFIRMED"
    PhaseTriaging   Phase = "TRIAGING"
    PhaseFixing     Phase = "FIXING"
    PhaseVerifying  Phase = "VERIFYING"
    PhaseResolved   Phase = "RESOLVED"
)

// confidenceThreshold is the minimum confidence score to advance NARROWING → CONFIRMED.
// With a typical 10-node graph: max score ≈ 0.65 (evidence saturated + 2/10 callers + 0.4 errorRate).
// Set to 0.35 so CONFIRMED is reachable after ~5 evidence signals.
const confidenceThreshold = 0.35

// decayEvidence applies exponential decay to signal timestamps.
// Recent signals count more; signals older than 30 min contribute ~5%.
// λ = 0.1 per minute → half-life ≈ 7 minutes
func decayEvidence(signals []map[string]interface{}) float64 {
    if len(signals) == 0 {
        return 0
    }
    const lambda = 0.1
    now := float64(time.Now().Unix())
    total := 0.0
    for _, sig := range signals {
        if ts, ok := sig["timestamp"].(float64); ok {
            ageMinutes := (now - ts/1000) / 60.0 // timestamp is milliseconds
            total += math.Exp(-lambda * ageMinutes)
        } else {
            total += 0.5 // unknown age gets half credit
        }
    }
    return math.Min(total/5.0, 1.0) // normalize: 5 fresh signals = max score
}

// scoreCandidate mirrors the rca-ranker Score formula so the detection-engine
// can locally re-rank without a round-trip to the ranker service.
// decayedEvidence is computed from signal timestamps and blended with raw count.
func scoreCandidate(evidenceCount int, impact, errorRate, decayedEvidence float64) float64 {
    ev := math.Max(float64(evidenceCount)/10.0, decayedEvidence)
    if ev > 1.0 {
        ev = 1.0
    }
    return ev*0.5 + impact*0.3 + errorRate*0.2
}

// advancePhase updates the incident phase, writes an audit record, and publishes a Redis event.
func (d *Detector) advancePhase(ctx context.Context, incidentID, projectID, tenantID string, from, to Phase) {
    if d.conn == nil {
        return
    }
    res, err := d.conn.Exec(ctx,
        `UPDATE incidents SET phase = $1, updated_at = NOW() WHERE id = $2 AND phase = $3`,
        string(to), incidentID, string(from))
    if err != nil || res.RowsAffected() == 0 {
        if err != nil {
            log.Printf("advancePhase %s→%s for %s: %v", from, to, incidentID, err)
        }
        return
    }
    // Increment sop_phase_transitions_total Prometheus counter
    sopPhaseTransitionsTotal.WithLabelValues(string(from), string(to)).Inc()

    // Audit record
    _, _ = d.conn.Exec(ctx,
        `INSERT INTO sop_audit (incident_id, action, actor, phase_from, phase_to)
         VALUES ($1, 'phase_change', 'system', $2, $3)`,
        incidentID, string(from), string(to))

    event := map[string]interface{}{
        "type":       "incident_phase_changed",
        "incidentId": incidentID,
        "from":       string(from),
        "to":         string(to),
        "projectId":  projectID,
        "tenantId":   tenantID,
    }
    b, _ := json.Marshal(event)
    _ = d.rdb.Publish(ctx, "incident_events", b).Err()
    reasoningLog(map[string]interface{}{
        "event":       "phase_advanced",
        "incident_id": incidentID,
        "from_phase":  string(from),
        "to_phase":    string(to),
        "project_id":  projectID,
        "reasoning":   fmt.Sprintf("Phase advanced %s → %s for incident %s", from, to, incidentID),
    })

    // Send webhook notification when root cause is confirmed
    if to == PhaseTriaging {
        go d.sendWebhookNotification(ctx, incidentID, projectID, tenantID)
    }
}

// accumulateEvidence increments evidence for a service on an existing incident,
// re-scores all RCA candidates, and advances the SOP phase when confidence is sufficient.
func (d *Detector) accumulateEvidence(ctx context.Context, incidentID string, s Signal, namespace string, g *Graph) {
    if d.conn == nil {
        return
    }

    // 1. Append signal to evidence array and increment evidence_count for this candidate
    evidenceItem := map[string]interface{}{
        "service":     s.Service,
        "statusClass": s.StatusClass,
        "errorRate":   s.ErrorRate,
        "latencyP95":  s.LatencyP95,
        "timestamp":   time.Now().Unix(),
    }
    evidenceJSON, _ := json.Marshal(evidenceItem)
    _, _ = d.conn.Exec(ctx,
        `UPDATE incidents SET evidence = evidence || $1::jsonb, updated_at = NOW() WHERE id = $2`,
        string(evidenceJSON), incidentID)

    // Increment evidence_count for the signaling service's candidate
    _, _ = d.conn.Exec(ctx,
        `UPDATE rca_candidates
         SET evidence_count = evidence_count + 1
         WHERE incident_id = $1 AND candidate_node_id = $2`,
        incidentID, s.Service)

    // 2. Fetch all candidates with their current evidence counts
    type candidateRow struct {
        id            string
        nodeID        string
        evidenceCount int
        currentScore  float64
    }
    rows, err := d.conn.Query(ctx,
        `SELECT id, candidate_node_id, evidence_count, confidence_score
         FROM rca_candidates WHERE incident_id = $1`, incidentID)
    if err != nil {
        return
    }
    var candidates []candidateRow
    for rows.Next() {
        var c candidateRow
        _ = rows.Scan(&c.id, &c.nodeID, &c.evidenceCount, &c.currentScore)
        candidates = append(candidates, c)
    }
    rows.Close()

    // Query incident evidence JSONB to compute time-decayed evidence score
    var evidenceRaw []byte
    _ = d.conn.QueryRow(ctx,
        `SELECT COALESCE(evidence::text, '[]') FROM incidents WHERE id = $1`, incidentID).Scan(&evidenceRaw)
    var signals []map[string]interface{}
    _ = json.Unmarshal(evidenceRaw, &signals)
    decayedEv := decayEvidence(signals)

    // 3. Re-score each candidate with real inputs from the graph
    reverseEdges := make(map[string][]string)
    for from, tos := range g.Edges {
        for _, to := range tos {
            reverseEdges[to] = append(reverseEdges[to], from)
        }
    }
    totalNodes := len(g.Nodes)
    if totalNodes == 0 {
        totalNodes = 1
    }

    bestScore := 0.0
    bestNodeID := ""
    for _, c := range candidates {
        node := g.Nodes[c.nodeID]
        errorRate := 0.0
        if node != nil {
            errorRate = node.ErrorRate
        }
        callerCount := len(reverseEdges[c.nodeID])
        impact := float64(callerCount) / float64(totalNodes)
        newScore := scoreCandidate(c.evidenceCount, impact, errorRate, decayedEv)
        _, _ = d.conn.Exec(ctx,
            `UPDATE rca_candidates SET confidence_score = $1 WHERE id = $2`,
            newScore, c.id)
        if newScore > bestScore {
            bestScore = newScore
            bestNodeID = c.nodeID
        }
    }

    // 4. Get current phase
    var currentPhase Phase
    var projectID, tenantID string
    err = d.conn.QueryRow(ctx,
        `SELECT phase, COALESCE(project_id,''), COALESCE(tenant_id,'') FROM incidents WHERE id = $1`,
        incidentID).Scan(&currentPhase, &projectID, &tenantID)
    if err != nil {
        return
    }

    // 5. Publish re-ranked update event
    updateEvent := map[string]interface{}{
        "type":       "incident_updated",
        "incidentId": incidentID,
        "rootCause":  bestNodeID,
        "confidence": bestScore,
        "projectId":  projectID,
        "tenantId":   tenantID,
        "phase":      string(currentPhase),
    }
    b, _ := json.Marshal(updateEvent)
    _ = d.rdb.Publish(ctx, "incident_events", b).Err()

    // 6. Advance phase if warranted
    switch currentPhase {
    case PhaseDetecting:
        // Immediately advance to NARROWING when evidence starts accumulating
        d.advancePhase(ctx, incidentID, projectID, tenantID, PhaseDetecting, PhaseNarrowing)
    case PhaseNarrowing:
        // Advance to CONFIRMED once top candidate confidence exceeds threshold
        if bestScore >= confidenceThreshold && bestNodeID != "" {
            // Log full score breakdown — the reasoning behind CONFIRMED
            reasoningLog(map[string]interface{}{
                "event":          "root_cause_confirmed",
                "incident_id":    incidentID,
                "root_cause":     bestNodeID,
                "score":          bestScore,
                "threshold":      confidenceThreshold,
                "score_passed":   bestScore >= confidenceThreshold,
                "reasoning":      fmt.Sprintf("%.3f ≥ %.2f threshold — CONFIRMED after %d evidence signals", bestScore, confidenceThreshold, func() int {
                    // count total signals from all candidates
                    total := 0
                    for _, c := range candidates {
                        total += c.evidenceCount
                    }
                    return total
                }()),
            })
            _, _ = d.conn.Exec(ctx,
                `UPDATE incidents SET root_cause_candidate = $1, confidence = $2 WHERE id = $3`,
                bestNodeID, bestScore, incidentID)
            d.advancePhase(ctx, incidentID, projectID, tenantID, PhaseNarrowing, PhaseConfirmed)
            // Trigger code-indexer for the confirmed root service (fire-and-forget)
            // This ensures code context is fresh by the time the operator opens the incident
            go triggerCodeIndex(bestNodeID)
            // Immediately generate SOP playbook and advance to TRIAGING
            d.generateAndStoreSOP(ctx, incidentID, s.Service, g.Nodes[bestNodeID])
            d.advancePhase(ctx, incidentID, projectID, tenantID, PhaseConfirmed, PhaseTriaging)
        }
    }
}

// generateAndStoreSOP generates structured SOP steps for a confirmed root cause
// and persists them alongside the existing recommendations.
func (d *Detector) generateAndStoreSOP(ctx context.Context, incidentID, rootService string, node *Node) {
    if d.conn == nil {
        return
    }
    faultType := FaultServiceDown
    if node != nil {
        faultType = ClassifyFault(node.StatusClass, node.ErrorRate, node.LatencyP95)
    }

    // Look up project's custom playbook for this service from projects.metadata.customPlaybooks
    var sopSteps []SOPStep
    var projectID string
    _ = d.conn.QueryRow(ctx, `SELECT COALESCE(project_id,'') FROM incidents WHERE id=$1`, incidentID).Scan(&projectID)
    if projectID != "" {
        var metaRaw []byte
        _ = d.conn.QueryRow(ctx, `SELECT metadata->'customPlaybooks' FROM projects WHERE id=$1`, projectID).Scan(&metaRaw)
        if len(metaRaw) > 2 {
            var customPlaybooks map[string][]SOPStep
            if json.Unmarshal(metaRaw, &customPlaybooks) == nil {
                if steps, ok := customPlaybooks[rootService]; ok && len(steps) > 0 {
                    sopSteps = steps
                    log.Printf("generateAndStoreSOP: using custom playbook for service=%s incident=%s (%d steps)", rootService, incidentID, len(sopSteps))
                }
            }
        }
    }
    if sopSteps == nil {
        // Query learning data for this service and fault type before generating playbook
        var learning []StepLearning
        lrows, _ := d.conn.Query(ctx,
            `SELECT step_order,
                    CASE WHEN total_count > 0 THEN success_count::float/total_count ELSE 0 END,
                    total_count
             FROM playbook_learning
             WHERE service_id=$1 AND fault_type=$2
             ORDER BY step_order`, rootService, string(faultType))
        if lrows != nil {
            defer lrows.Close()
            for lrows.Next() {
                var sl StepLearning
                if lrows.Scan(&sl.StepOrder, &sl.SuccessRate, &sl.TotalCount) == nil {
                    learning = append(learning, sl)
                }
            }
        }
        sopSteps = GenerateSOPPlaybookWithLearning(faultType, rootService, learning)
    }

    sopJSON, _ := json.Marshal(sopSteps)

    // Store on the first recommendation row for this incident
    _, _ = d.conn.Exec(ctx,
        `UPDATE recommendations SET sop_steps = $1 WHERE incident_id = $2 AND rank = 1`,
        string(sopJSON), incidentID)

    // If no recommendation row exists yet, insert a placeholder
    var count int
    _ = d.conn.QueryRow(ctx, `SELECT COUNT(*) FROM recommendations WHERE incident_id = $1`, incidentID).Scan(&count)
    if count == 0 {
        recID := fmt.Sprintf("%s-sop", incidentID)
        _, _ = d.conn.Exec(ctx,
            `INSERT INTO recommendations (id, incident_id, rank, playbook_title, steps, sop_steps)
             VALUES ($1,$2,1,'SOP Playbook','[]',$3) ON CONFLICT DO NOTHING`,
            recID, incidentID, string(sopJSON))
    }
    log.Printf("SOP playbook generated for incident=%s rootService=%s faultType=%s steps=%d",
        incidentID, rootService, faultType, len(sopSteps))
}

// handleHealthySignal checks if a healthy signal satisfies the expectedSignal condition
// on a VERIFYING incident. This is the targeted auto-verify path.
func (d *Detector) handleHealthySignal(ctx context.Context, service string, s Signal) {
    if d.conn == nil {
        return
    }
    // Find any VERIFYING incident for this service
    var incidentID, projectID, tenantID string
    var expectedSignal string
    err := d.conn.QueryRow(ctx,
        `SELECT id, COALESCE(project_id,''), COALESCE(tenant_id,''), COALESCE(expected_signal,'2xx')
         FROM incidents
         WHERE service = $1 AND status IN ('OPEN','ACKNOWLEDGED') AND phase = 'VERIFYING'
         ORDER BY detected_at DESC LIMIT 1`,
        service).Scan(&incidentID, &projectID, &tenantID, &expectedSignal)
    if err != nil {
        return // no VERIFYING incident for this service
    }

    // Evaluate condition against actual signal values
    if !meetsExpectedSignal(expectedSignal, s) {
        log.Printf("SOP auto-verify: condition %q not met for incident=%s service=%s (statusClass=%s lat=%.0f err=%.3f)",
            expectedSignal, incidentID, service, s.StatusClass, s.LatencyP95, s.ErrorRate)
        return
    }

    // Condition met — auto-advance VERIFYING → RESOLVED
    res, _ := d.conn.Exec(ctx,
        `UPDATE incidents SET status = 'RESOLVED', phase = 'RESOLVED', resolved_at = NOW(), updated_at = NOW()
         WHERE id = $1 AND phase = 'VERIFYING' AND status IN ('OPEN','ACKNOWLEDGED')`,
        incidentID)
    if res.RowsAffected() == 0 {
        return
    }
    // Audit record
    signalJSON, _ := json.Marshal(map[string]interface{}{
        "service": service, "statusClass": s.StatusClass,
        "errorRate": s.ErrorRate, "latencyP95": s.LatencyP95,
    })
    _, _ = d.conn.Exec(ctx,
        `INSERT INTO sop_audit (incident_id, action, actor, phase_from, phase_to, signal_snapshot)
         VALUES ($1, 'auto_resolved', 'system', 'VERIFYING', 'RESOLVED', $2)`,
        incidentID, string(signalJSON))

    event := map[string]interface{}{
        "type":         "incident_resolved",
        "incidentId":   incidentID,
        "service":      service,
        "projectId":    projectID,
        "tenantId":     tenantID,
        "autoVerified": true,
        "condition":    expectedSignal,
        "phase":        "RESOLVED",
    }
    b, _ := json.Marshal(event)
    _ = d.rdb.Publish(ctx, "incident_events", b).Err()
    log.Printf("SOP auto-verified RESOLVED: incident=%s service=%s condition=%q", incidentID, service, expectedSignal)

    // Record playbook outcomes for learning — which steps succeeded
    faultType := string(ClassifyFault(s.StatusClass, s.ErrorRate, int(s.LatencyP95)))
    go d.recordPlaybookOutcome(context.Background(), incidentID, service, faultType)
}

// meetsExpectedSignal evaluates a signal against an expectedSignal condition string.
// Supported forms: "2xx", "latency<Nms", "errorRate<N.NN"
func meetsExpectedSignal(condition string, s Signal) bool {
    if condition == "" || condition == "2xx" {
        return isHealthyStatus(s.StatusClass) && s.ErrorRate < 0.05
    }
    if strings.HasPrefix(condition, "latency<") {
        threshold := strings.TrimSuffix(strings.TrimPrefix(condition, "latency<"), "ms")
        if n, err := strconv.ParseFloat(threshold, 64); err == nil {
            return isHealthyStatus(s.StatusClass) && s.LatencyP95 < n
        }
    }
    if strings.HasPrefix(condition, "errorRate<") {
        threshold := strings.TrimPrefix(condition, "errorRate<")
        if n, err := strconv.ParseFloat(threshold, 64); err == nil {
            return s.ErrorRate < n
        }
    }
    // Unknown condition — fall back to healthy status check
    return isHealthyStatus(s.StatusClass)
}

// sendWebhookWithRetry posts to a webhook URL with exponential backoff retry.
// Tries up to maxRetries times: delays 1s, 2s, 4s between attempts.
func sendWebhookWithRetry(ctx context.Context, url string, body []byte, maxRetries int) error {
	delay := 1 * time.Second
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
				delay *= 2 // exponential backoff: 1s, 2s, 4s
			}
		}
		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "TechGraph-SOP/1.0")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			lastErr = err
			log.Printf("webhook attempt %d/%d failed: %v", attempt+1, maxRetries+1, err)
			continue
		}
		resp.Body.Close()

		// 2xx = success
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			if attempt > 0 {
				log.Printf("webhook succeeded on attempt %d", attempt+1)
			}
			return nil
		}
		// 4xx (except 429) = permanent failure, don't retry
		if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != 429 {
			return fmt.Errorf("webhook permanent failure: HTTP %d", resp.StatusCode)
		}
		// 5xx or 429 = retry
		lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
		log.Printf("webhook attempt %d/%d returned %d, retrying", attempt+1, maxRetries+1, resp.StatusCode)
	}
	return fmt.Errorf("webhook failed after %d attempts: %w", maxRetries+1, lastErr)
}

// getWebhookURLForFault determines the webhook URL and priority based on fault severity.
// P1 faults (service-down, cascading): use P1_WEBHOOK_URL (PagerDuty, OpsGenie)
// P2 faults (latency, transient): use P2_WEBHOOK_URL (Slack, Teams)
// Falls back to global NOTIFICATION_WEBHOOK_URL if the specific URL is not set.
func (d *Detector) getWebhookURLForFault(ctx context.Context, incidentID string) (string, string) {
    var faultType string
    if d.conn != nil {
        _ = d.conn.QueryRow(ctx,
            `SELECT COALESCE(evidence->0->>'faultType','')
             FROM rca_candidates WHERE incident_id=$1
             ORDER BY confidence_score DESC LIMIT 1`,
            incidentID).Scan(&faultType)
    }

    p1Faults := map[string]bool{"service-down": true, "cascading-failure": true, "connection-refused": true}

    var webhookURL, priority string
    if p1Faults[faultType] {
        priority = "P1"
        webhookURL = os.Getenv("P1_WEBHOOK_URL")
    } else {
        priority = "P2"
        webhookURL = os.Getenv("P2_WEBHOOK_URL")
    }
    if webhookURL == "" {
        webhookURL = os.Getenv("NOTIFICATION_WEBHOOK_URL")
    }
    return webhookURL, priority
}

// sendWebhookNotification fires a POST to the appropriate webhook URL when an incident
// reaches TRIAGING phase (root cause confirmed). Routes to P1_WEBHOOK_URL for critical
// faults (service-down, cascading) and P2_WEBHOOK_URL for informational faults.
// Works with Slack incoming webhooks, PagerDuty, OpsGenie, or any HTTP endpoint.
func (d *Detector) sendWebhookNotification(ctx context.Context, incidentID, projectID, tenantID string) {
    webhookURL, priority := d.getWebhookURLForFault(ctx, incidentID)

    // Fetch incident details for the notification
    var service, rootCause string
    var confidence float64
    _ = d.conn.QueryRow(ctx,
        `SELECT COALESCE(service,''), COALESCE(root_cause_candidate,''), COALESCE(confidence,0)
         FROM incidents WHERE id = $1`, incidentID).
        Scan(&service, &rootCause, &confidence)

    // Look up per-service alert channel from projects.metadata.serviceAlertChannels
    // If found, route to that channel instead of the global webhook
    if service != "" && d.conn != nil && projectID != "" {
        ctx2, cancel2 := context.WithTimeout(ctx, 2*time.Second)
        defer cancel2()
        var channelsRaw []byte
        err := d.conn.QueryRow(ctx2,
            `SELECT metadata->'serviceAlertChannels' FROM projects WHERE id=$1`, projectID).
            Scan(&channelsRaw)
        if err == nil && len(channelsRaw) > 2 { // > 2 means it's not just "null" or "{}"
            var channels map[string]string
            if json.Unmarshal(channelsRaw, &channels) == nil {
                if ch, ok := channels[service]; ok && ch != "" {
                    webhookURL = ch
                    log.Printf("webhook: routing %s notification to per-service channel %s", service, ch)
                }
            }
        }
    }

    if webhookURL == "" {
        return
    }

    payload := map[string]interface{}{
        "text": fmt.Sprintf("🚨 *Incident Confirmed* — Root cause: `%s` (confidence: %.0f%%)\nService: `%s` | Project: `%s`\nSOP playbook is ready. <http://localhost:4001/dashboard/incidents/%s|View incident>",
            rootCause, confidence*100, service, projectID, incidentID),
        "priority": priority,
        "incident": map[string]interface{}{
            "id":         incidentID,
            "service":    service,
            "rootCause":  rootCause,
            "confidence": confidence,
            "projectId":  projectID,
            "tenantId":   tenantID,
            "phase":      "TRIAGING",
            "priority":   priority,
        },
    }
    body, _ := json.Marshal(payload)
    if err := sendWebhookWithRetry(ctx, webhookURL, body, 3); err != nil {
        log.Printf("webhook notification failed after retries: %v", err)
        return
    }
    log.Printf("webhook notification sent for incident=%s rootCause=%s", incidentID, rootCause)
}

// storeSopExpectedSignal saves the expectedSignal condition for a VERIFYING step
// so handleHealthySignal can check it precisely.
func (d *Detector) storeSopExpectedSignal(ctx context.Context, incidentID, expectedSignal string, stepOrder int) {
    if d.conn == nil {
        return
    }
    _, _ = d.conn.Exec(ctx,
        `UPDATE incidents SET expected_signal = $1, verifying_step_order = $2 WHERE id = $3`,
        expectedSignal, stepOrder, incidentID)
}

// triggerCodeIndex calls the code-indexer's on-demand API to re-index a specific service.
// Called when CONFIRMED fires so code context is fresh before operator opens the incident.
// Fire-and-forget: if the code-indexer is not running, this silently no-ops.
func triggerCodeIndex(serviceID string) {
    indexerURL := os.Getenv("CODE_INDEXER_URL")
    if indexerURL == "" {
        indexerURL = "http://code-indexer:8092"
    }
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    url := fmt.Sprintf("%s/index/service/%s", indexerURL, serviceID)
    req, _ := http.NewRequestWithContext(ctx, "POST", url, nil)
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        log.Printf("code-index trigger %s: %v (code-indexer may not be running)", serviceID, err)
        return
    }
    resp.Body.Close()
    log.Printf("code-index trigger: %s → %d", serviceID, resp.StatusCode)
}

// startSignalHistoryCleanup deletes old rows from signal_history and sop_audit.
// Runs every hour to prevent unbounded table growth.
//   signal_history: 7-day retention  (~60k rows/day for 10 services)
//   sop_audit:      90-day retention (~600 rows/day for active systems)
func (d *Detector) startSignalHistoryCleanup(ctx context.Context) {
    ticker := time.NewTicker(1 * time.Hour)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            if d.conn == nil {
                continue
            }
            ctx2, cancel := context.WithTimeout(ctx, 30*time.Second)

            res, err := d.conn.Exec(ctx2,
                `DELETE FROM signal_history WHERE recorded_at < NOW() - INTERVAL '7 days'`)
            if err != nil {
                log.Printf("signal_history cleanup: %v", err)
            } else {
                log.Printf("signal_history cleanup: deleted %d rows", res.RowsAffected())
            }

            res2, err2 := d.conn.Exec(ctx2,
                `DELETE FROM sop_audit WHERE created_at < NOW() - INTERVAL '90 days'`)
            if err2 != nil {
                log.Printf("sop_audit cleanup: %v", err2)
            } else if res2.RowsAffected() > 0 {
                log.Printf("sop_audit cleanup: deleted %d rows older than 90 days", res2.RowsAffected())
            }

            cancel()
        }
    }
}

func main() {
    log.Println("detection-engine: starting")
    ctx := context.Background()
    det, err := NewDetector(ctx)
    if err != nil {
        log.Fatalf("new detector: %v", err)
    }
    defer det.Close(ctx)

    // run detection loop in background and expose a health endpoint
    go func() {
        if err := det.Run(ctx); err != nil {
            log.Fatalf("detector run: %v", err)
        }
    }()

    // watch deployment health gates in background
    go det.watchDeploymentHealthGates(ctx)

    // cleanup old signal_history rows hourly
    go det.startSignalHistoryCleanup(ctx)

    // check topology drift every 15 minutes
    go det.checkTopologyDrift(ctx)

    mux := http.NewServeMux()
    mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        _, _ = w.Write([]byte("ok"))
    })
    mux.Handle("/metrics", promhttp.Handler())
    srv := &http.Server{
        Addr:    ":8084",
        Handler: mux,
    }
    log.Printf("detection-engine: health endpoint listening on %s", srv.Addr)
    if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        log.Fatalf("health server: %v", err)
    }
}

func fetchGraph(ctx context.Context, baseURL, namespace string) (*Graph, error) {
    req, _ := http.NewRequestWithContext(ctx, "GET", baseURL+"?namespace="+namespace, nil)
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    var g Graph
    if err := json.NewDecoder(resp.Body).Decode(&g); err != nil {
        return nil, err
    }
    return &g, nil
}

// convert Graph from graph-manager to local Graph type
func convertGraph(g *Graph) *Graph {
    // they are compatible structurally; return as-is
    return g
}

// convertGraphFromRemote converts a graphclient.Graph into local Graph type
func convertGraphFromRemote(g *graphclient.Graph) *Graph {
    lg := &Graph{Nodes: make(map[string]*Node), Edges: make(map[string][]string)}
    if g == nil { return lg }
    for id, n := range g.Nodes {
        if n == nil { continue }
        lg.Nodes[id] = &Node{ID: n.ID, StatusClass: n.StatusClass, LatencyP95: n.LatencyP95, ErrorRate: n.ErrorRate}
    }
    for k, arr := range g.Edges {
        lg.Edges[k] = append([]string{}, arr...)
    }
    return lg
}

func ensureSchema(ctx context.Context, conn *pgxpool.Pool) error {
    _, err := conn.Exec(ctx, `
        CREATE TABLE IF NOT EXISTS incidents (
            id                   TEXT PRIMARY KEY,
            tenant_id            TEXT,
            project_id           TEXT,
            service              TEXT,
            status               TEXT DEFAULT 'OPEN',
            phase                TEXT DEFAULT 'DETECTING',
            detected_at          TIMESTAMPTZ DEFAULT NOW(),
            updated_at           TIMESTAMPTZ DEFAULT NOW(),
            evidence             JSONB DEFAULT '[]',
            root_cause_candidate TEXT,
            confidence           FLOAT DEFAULT 0.0
        )`)
    if err != nil {
        return err
    }
    _, err = conn.Exec(ctx, `
        CREATE TABLE IF NOT EXISTS rca_candidates (
            id                 TEXT PRIMARY KEY,
            incident_id        TEXT REFERENCES incidents(id) ON DELETE CASCADE,
            candidate_node_id  TEXT,
            confidence_score   FLOAT,
            evidence_count     INT DEFAULT 1,
            evidence           JSONB DEFAULT '{}',
            created_at         TIMESTAMPTZ DEFAULT NOW()
        )`)
    if err != nil {
        return err
    }
    _, err = conn.Exec(ctx, `
        CREATE TABLE IF NOT EXISTS recommendations (
            id             TEXT PRIMARY KEY,
            incident_id    TEXT REFERENCES incidents(id) ON DELETE CASCADE,
            rank           INT,
            playbook_title TEXT,
            steps          JSONB,
            sop_steps      JSONB DEFAULT '[]',
            created_at     TIMESTAMPTZ DEFAULT NOW()
        )`)
    if err != nil {
        return err
    }
    _, err = conn.Exec(ctx, `
        CREATE TABLE IF NOT EXISTS sop_audit (
            id              TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
            incident_id     TEXT NOT NULL,
            step_order      INT,
            action          TEXT NOT NULL,
            actor           TEXT DEFAULT 'system',
            phase_from      TEXT,
            phase_to        TEXT,
            signal_snapshot JSONB DEFAULT '{}',
            created_at      TIMESTAMPTZ DEFAULT NOW()
        )`)
    if err != nil {
        return err
    }
    // Idempotent index creation
    for _, idx := range []string{
        `CREATE INDEX IF NOT EXISTS idx_incidents_tenant ON incidents(tenant_id)`,
        `CREATE INDEX IF NOT EXISTS idx_incidents_project ON incidents(project_id)`,
        `CREATE INDEX IF NOT EXISTS idx_incidents_service ON incidents(service)`,
        `CREATE INDEX IF NOT EXISTS idx_incidents_phase ON incidents(phase) WHERE status = 'OPEN'`,
        `CREATE INDEX IF NOT EXISTS idx_incidents_open ON incidents(detected_at DESC) WHERE status = 'OPEN'`,
    } {
        if _, err = conn.Exec(ctx, idx); err != nil {
            return err
        }
    }
    // Ensure deployments table exists (supports deployment health gate tracking)
    _, err = conn.Exec(ctx, `
        CREATE TABLE IF NOT EXISTS deployments (
            id               TEXT PRIMARY KEY,
            tenant_id        TEXT,
            project_id       TEXT,
            service_id       TEXT,
            service_name     TEXT,
            version          TEXT,
            commit_hash      TEXT,
            deployed_by      TEXT,
            environment      TEXT DEFAULT 'prod',
            status           TEXT DEFAULT 'deploying',
            health_gate_until TIMESTAMPTZ,
            health_status    TEXT,
            health_checked_at TIMESTAMPTZ,
            notes            TEXT,
            deployed_at      TIMESTAMPTZ DEFAULT NOW()
        )`)
    if err != nil {
        return err
    }

    // Ensure signal_history table exists (for SLO computation and sparklines)
    _, err = conn.Exec(ctx, `
        CREATE TABLE IF NOT EXISTS signal_history (
            id BIGSERIAL PRIMARY KEY, tenant_id TEXT, project_id TEXT NOT NULL,
            service_id TEXT NOT NULL, namespace TEXT, status_class TEXT NOT NULL,
            error_rate FLOAT DEFAULT 0, latency_p95 FLOAT DEFAULT 0, source TEXT,
            recorded_at TIMESTAMPTZ DEFAULT NOW()
        )`)
    if err != nil {
        return err
    }
    for _, idx := range []string{
        `CREATE INDEX IF NOT EXISTS idx_signal_history_service ON signal_history(service_id, recorded_at DESC)`,
        `CREATE INDEX IF NOT EXISTS idx_signal_history_project ON signal_history(project_id, recorded_at DESC)`,
    } {
        if _, err = conn.Exec(ctx, idx); err != nil {
            return err
        }
    }

    // Ensure playbook_learning table exists (supports playbook outcome recording)
    _, err = conn.Exec(ctx, `
        CREATE TABLE IF NOT EXISTS playbook_learning (
            service_id    TEXT,
            fault_type    TEXT,
            step_order    INT,
            step_title    TEXT,
            success_count INT DEFAULT 0,
            total_count   INT DEFAULT 0,
            last_updated  TIMESTAMPTZ DEFAULT NOW(),
            PRIMARY KEY (service_id, fault_type, step_order)
        )`)
    if err != nil {
        return err
    }

    // Ensure maintenance_windows table exists (ITEM 5)
    _, err = conn.Exec(ctx, `
        CREATE TABLE IF NOT EXISTS maintenance_windows (
            id          TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
            project_id  TEXT NOT NULL,
            service_id  TEXT,
            reason      TEXT,
            created_by  TEXT DEFAULT 'operator',
            starts_at   TIMESTAMPTZ DEFAULT NOW(),
            ends_at     TIMESTAMPTZ NOT NULL,
            created_at  TIMESTAMPTZ DEFAULT NOW()
        )`)
    if err != nil {
        return err
    }
    _, err = conn.Exec(ctx, `CREATE INDEX IF NOT EXISTS idx_maintenance_active ON maintenance_windows(project_id, ends_at)`)
    if err != nil {
        return err
    }

    // Backfill columns on pre-existing tables (migration 008 equivalent, safe to re-run)
    backfills := []string{
        `ALTER TABLE incidents ADD COLUMN IF NOT EXISTS phase TEXT DEFAULT 'DETECTING'`,
        `ALTER TABLE incidents ADD COLUMN IF NOT EXISTS tenant_id TEXT`,
        `ALTER TABLE incidents ADD COLUMN IF NOT EXISTS service TEXT`,
        `ALTER TABLE incidents ADD COLUMN IF NOT EXISTS status TEXT DEFAULT 'OPEN'`,
        `ALTER TABLE incidents ADD COLUMN IF NOT EXISTS evidence JSONB DEFAULT '[]'`,
        `ALTER TABLE incidents ADD COLUMN IF NOT EXISTS root_cause_candidate TEXT`,
        `ALTER TABLE incidents ADD COLUMN IF NOT EXISTS confidence FLOAT DEFAULT 0.0`,
        `ALTER TABLE incidents ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ DEFAULT NOW()`,
        `ALTER TABLE incidents ADD COLUMN IF NOT EXISTS resolved_at TIMESTAMPTZ`,
        `ALTER TABLE incidents ADD COLUMN IF NOT EXISTS expected_signal TEXT`,
        `ALTER TABLE incidents ADD COLUMN IF NOT EXISTS verifying_step_order INT`,
        `ALTER TABLE rca_candidates ADD COLUMN IF NOT EXISTS evidence_count INT DEFAULT 1`,
        `ALTER TABLE rca_candidates ADD COLUMN IF NOT EXISTS candidate_node_id TEXT`,
        `ALTER TABLE rca_candidates ADD COLUMN IF NOT EXISTS confidence_score FLOAT`,
        `ALTER TABLE recommendations ADD COLUMN IF NOT EXISTS sop_steps JSONB DEFAULT '[]'`,
        `ALTER TABLE deployments ADD COLUMN IF NOT EXISTS health_status TEXT`,
        `ALTER TABLE deployments ADD COLUMN IF NOT EXISTS health_checked_at TIMESTAMPTZ`,
        `ALTER TABLE deployments ADD COLUMN IF NOT EXISTS health_gate_until TIMESTAMPTZ`,
        `ALTER TABLE incidents ADD COLUMN IF NOT EXISTS assigned_to TEXT`,
        `ALTER TABLE incidents ADD COLUMN IF NOT EXISTS assigned_at TIMESTAMPTZ`,
    }
    for _, b := range backfills {
        if _, err = conn.Exec(ctx, b); err != nil {
            return err
        }
    }
    return nil
}

// watchDeploymentHealthGates subscribes to deployment_events Redis channel and tracks
// active deployment health gates. Every 30s it evaluates expired gates and marks deployments
// as healthy or degraded based on whether any OPEN incidents fired during the gate window.
func (d *Detector) watchDeploymentHealthGates(ctx context.Context) {
    gates := new(sync.Map) // deployID → deployGate

    // Subscribe to deployment_events channel
    sub := d.rdb.Subscribe(ctx, "deployment_events")
    if _, err := sub.Receive(ctx); err != nil {
        log.Printf("deployment health gate: subscribe failed: %v", err)
        return
    }
    ch := sub.Channel()
    log.Println("detection-engine: subscribed to deployment_events channel")

    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case msg, ok := <-ch:
            if !ok {
                return
            }
            var event map[string]interface{}
            if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
                continue
            }
            evType, _ := event["type"].(string)
            if evType != "deployment" {
                continue
            }
            deployID, _ := event["deploymentId"].(string)
            serviceID, _ := event["serviceId"].(string)
            projectID, _ := event["projectId"].(string)
            healthGateStr, _ := event["healthGateUntil"].(string)
            if deployID == "" || serviceID == "" {
                continue
            }
            expiresAt := time.Now().Add(5 * time.Minute)
            if healthGateStr != "" {
                if t, err := time.Parse(time.RFC3339, healthGateStr); err == nil {
                    expiresAt = t
                }
            }
            gates.Store(deployID, deployGate{serviceID: serviceID, projectID: projectID, expiresAt: expiresAt})
            log.Printf("deployment gate registered: deploy=%s service=%s expires=%s", deployID, serviceID, expiresAt.Format(time.RFC3339))
        case <-ticker.C:
            now := time.Now()
            gates.Range(func(key, val interface{}) bool {
                deployID := key.(string)
                gate := val.(deployGate)
                if !gate.expiresAt.Before(now) {
                    return true // gate not yet expired
                }
                // Remove from tracking and evaluate
                gates.Delete(deployID)
                go d.evaluateDeploymentGate(ctx, deployID, gate)
                return true
            })
        }
    }
}

// evaluateDeploymentGate checks whether any OPEN incident fired during the deployment gate window
// and marks the deployment as healthy or degraded accordingly.
func (d *Detector) evaluateDeploymentGate(ctx context.Context, deployID string, gate deployGate) {
    if d.conn == nil {
        return
    }
    ctx2, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()

    // Fetch deployment timestamp so we can bound the gate window from the start
    var deployedAt time.Time
    if err := d.conn.QueryRow(ctx2, `SELECT deployed_at FROM deployments WHERE id=$1`, deployID).Scan(&deployedAt); err != nil {
        deployedAt = gate.expiresAt.Add(-5 * time.Minute)
    }

    // If a maintenance window was active during the gate window, skip evaluation.
    // Signals were suppressed during maintenance — absence of incidents doesn't mean healthy.
    var maintenanceCount int
    _ = d.conn.QueryRow(ctx2,
        `SELECT COUNT(*) FROM maintenance_windows
         WHERE project_id=$1 AND ends_at > $2
         AND (service_id IS NULL OR service_id=$3)
         AND starts_at < $4`,
        gate.projectID, deployedAt, gate.serviceID, gate.expiresAt).Scan(&maintenanceCount)
    if maintenanceCount > 0 {
        reasoningLog(map[string]interface{}{
            "event":      "deployment_gate_skipped",
            "deploy_id":  deployID,
            "service":    gate.serviceID,
            "reasoning":  fmt.Sprintf("maintenance window was active during gate window for %s — skipping gate evaluation", gate.serviceID),
        })
        _, _ = d.conn.Exec(ctx2, `UPDATE deployments SET status='healthy', health_status='skipped (maintenance)', health_checked_at=NOW() WHERE id=$1`, deployID)
        return
    }

    // Check whether any OPEN incident was created for this service during the gate window
    var count int
    _ = d.conn.QueryRow(ctx2,
        `SELECT COUNT(*) FROM incidents
         WHERE project_id=$1 AND service=$2 AND status='OPEN'
         AND detected_at >= $3 AND detected_at <= $4`,
        gate.projectID, gate.serviceID, deployedAt, gate.expiresAt).Scan(&count)

    // Also check rca_candidates for any fault signals in that window
    var rcaCount int
    _ = d.conn.QueryRow(ctx2,
        `SELECT COUNT(*) FROM rca_candidates rc
         JOIN incidents i ON rc.incident_id = i.id
         WHERE i.project_id=$1 AND rc.candidate_node_id=$2
         AND i.detected_at >= $3 AND i.detected_at <= $4`,
        gate.projectID, gate.serviceID, deployedAt, gate.expiresAt).Scan(&rcaCount)

    status := "healthy"
    reasoning := fmt.Sprintf("no incidents detected for %s during %s health gate (%.0f min window)",
        gate.serviceID, deployID, gate.expiresAt.Sub(deployedAt).Minutes())
    if count > 0 || rcaCount > 0 {
        status = "degraded"
        reasoning = fmt.Sprintf("%d incident(s) detected for %s during %s health gate — marking deployment degraded",
            count+rcaCount, gate.serviceID, deployID)
    }

    if _, err := d.conn.Exec(ctx2,
        `UPDATE deployments SET status=$1, health_status=$1, health_checked_at=NOW() WHERE id=$2`,
        status, deployID); err != nil {
        log.Printf("deployment gate update failed for %s: %v", deployID, err)
        return
    }
    reasoningLog(map[string]interface{}{
        "event":      "deployment_health_gate_evaluated",
        "deploy_id":  deployID,
        "service":    gate.serviceID,
        "project_id": gate.projectID,
        "status":     status,
        "incidents":  count + rcaCount,
        "reasoning":  reasoning,
    })
    // Notify via webhook that the deployment gate has closed
    go d.notifyDeploymentGateClose(ctx, deployID, gate.serviceID, gate.projectID, status)
}

// notifyDeploymentGateClose fires a webhook notification when the deployment health gate closes.
func (d *Detector) notifyDeploymentGateClose(ctx context.Context, deployID, serviceID, projectID, status string) {
    webhookURL := os.Getenv("NOTIFICATION_WEBHOOK_URL")

    // Check per-service alert channel
    if serviceID != "" && d.conn != nil && projectID != "" {
        ctx2, cancel2 := context.WithTimeout(ctx, 2*time.Second)
        defer cancel2()
        var channelsRaw []byte
        err := d.conn.QueryRow(ctx2,
            `SELECT metadata->'serviceAlertChannels' FROM projects WHERE id=$1`, projectID).
            Scan(&channelsRaw)
        if err == nil && len(channelsRaw) > 2 {
            var channels map[string]string
            if json.Unmarshal(channelsRaw, &channels) == nil {
                if ch, ok := channels[serviceID]; ok && ch != "" {
                    webhookURL = ch
                }
            }
        }
    }

    if webhookURL == "" {
        return
    }

    emoji := "✅"
    if status == "degraded" {
        emoji = "🔴"
    }

    payload := map[string]interface{}{
        "text": fmt.Sprintf("%s *Deployment Health Gate*: `%s` marked *%s* after 5-minute observation window", emoji, serviceID, status),
        "deployment": map[string]interface{}{
            "id": deployID, "serviceId": serviceID, "status": status, "projectId": projectID,
        },
    }
    body, _ := json.Marshal(payload)
    if err := sendWebhookWithRetry(ctx, webhookURL, body, 3); err != nil {
        log.Printf("deployment gate webhook failed after retries: %v", err)
    }
}

// recordPlaybookOutcome records which SOP steps were completed (DONE/FAILED) for a resolved
// incident, so the system can learn which steps have the highest success rates over time.
func (d *Detector) recordPlaybookOutcome(ctx context.Context, incidentID, serviceID, faultType string) {
    if d.conn == nil {
        return
    }

    // Query sop_audit for this incident: which steps were done vs failed
    rows, err := d.conn.Query(ctx,
        `SELECT COALESCE(step_order,0), action FROM sop_audit
         WHERE incident_id=$1 AND action IN ('step_done','step_failed')
         ORDER BY step_order`, incidentID)
    if err != nil {
        return
    }
    defer rows.Close()

    type stepResult struct {
        order  int
        action string
    }
    var steps []stepResult
    for rows.Next() {
        var s stepResult
        if rows.Scan(&s.order, &s.action) == nil {
            steps = append(steps, s)
        }
    }
    rows.Close()

    if len(steps) == 0 {
        return
    }

    for _, s := range steps {
        successIncr := 0
        if s.action == "step_done" {
            successIncr = 1
        }
        _, _ = d.conn.Exec(ctx,
            `INSERT INTO playbook_learning (service_id, fault_type, step_order, success_count, total_count, last_updated)
             VALUES ($1, $2, $3, $4, 1, NOW())
             ON CONFLICT (service_id, fault_type, step_order) DO UPDATE
             SET success_count = playbook_learning.success_count + $4,
                 total_count   = playbook_learning.total_count + 1,
                 last_updated  = NOW()`,
            serviceID, faultType, s.order, successIncr)
    }
    log.Printf("playbook_learning: recorded %d step outcomes for incident=%s service=%s fault=%s",
        len(steps), incidentID, serviceID, faultType)
}
