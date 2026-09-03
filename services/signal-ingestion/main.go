package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "os"
    "sync"
    "time"

    "github.com/redis/go-redis/v9"
    "golang.org/x/time/rate"
)

type Signal struct {
    // Support both legacy snake_case and new camelCase field names
    Tenant      string  `json:"tenant"`
    TenantID    string  `json:"tenantId"`
    Project     string  `json:"project"`
    ProjectID   string  `json:"projectId"`
    Environment string  `json:"environment"`
    Service     string  `json:"service"`
    StatusClass string  `json:"statusClass"`
    ErrorRate   float64 `json:"errorRate"`
    LatencyP95  float64 `json:"latencyP95"`
    // Legacy fields kept for backward compat
    Metric    string  `json:"metric"`
    Value     float64 `json:"value"`
    Timestamp int64   `json:"timestamp"`
}

func main() {
    redisAddr := os.Getenv("REDIS_ADDR")
    if redisAddr == "" {
        redisAddr = "redis:6379"
    }

    rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
    ctx := context.Background()
    if err := rdb.Ping(ctx).Err(); err != nil {
        log.Printf("warning: redis ping failed: %v", err)
    }

    pub := NewRedisPublisher(rdb)
    srv := NewServer(pub)

    // SSE hub for real-time incident events
    sseHub := NewSSEHub(rdb)
    go sseHub.Run(ctx)

    http.HandleFunc("/api/v1/signals", srv.HandleSignals)
    http.HandleFunc("/api/v1/events", sseHub.HandleSSE)
    http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        _, _ = w.Write([]byte("ok"))
    })

    port := os.Getenv("INGESTION_PORT")
    if port == "" {
        port = "8085"
    }
    log.Printf("signal-ingestion: listening %s (redis=%s) with SSE endpoint", port, redisAddr)
    log.Fatal(http.ListenAndServe(":"+port, nil))
}

// Server holds dependencies for HTTP handlers
type Server struct{
    pub SignalPublisher
    mu sync.Mutex
    limiters map[string]*rate.Limiter
}

func NewServer(pub SignalPublisher) *Server {
    return &Server{pub: pub, limiters: map[string]*rate.Limiter{}}
}

// HandleSignals is the HTTP handler for /api/v1/signals
func (s *Server) HandleSignals(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }
    var sig Signal
    if err := json.NewDecoder(r.Body).Decode(&sig); err != nil {
        http.Error(w, "bad request", http.StatusBadRequest)
        return
    }
    // Normalise camelCase → legacy fields so detection engine sees both
    if sig.Tenant == "" {
        sig.Tenant = sig.TenantID
    }
    if sig.Project == "" {
        sig.Project = sig.ProjectID
    }
    // Validate required fields
    if sig.Service == "" {
        http.Error(w, "service is required", http.StatusBadRequest)
        return
    }
    if sig.Tenant == "" || sig.Project == "" {
        http.Error(w, "tenant/tenantId and project/projectId are required", http.StatusBadRequest)
        return
    }
    // Validate statusClass against allowed values
    validStatusClasses := map[string]bool{
        "2xx": true, "5xx": true, "timeout": true, "connection_error": true,
        "down": true, "degraded": true, "503": true, "504": true,
        "4xx": true, "3xx": true, "1xx": true,
        // Allow empty (will be treated as 2xx)
        "": true,
    }
    if sig.StatusClass != "" && !validStatusClasses[sig.StatusClass] {
        http.Error(w, fmt.Sprintf("invalid statusClass %q: must be one of 2xx, 5xx, timeout, connection_error, down, degraded, 503, 504", sig.StatusClass), http.StatusBadRequest)
        return
    }
    // Validate error rate range
    if sig.ErrorRate < 0 || sig.ErrorRate > 1 {
        http.Error(w, "errorRate must be between 0.0 and 1.0", http.StatusBadRequest)
        return
    }
    // rate-limit by service name
    key := sig.Service
    if key == "" {
        key = "_global"
    }
    limiter := s.getLimiter(key)
    if !limiter.Allow() {
        http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
        return
    }
    if sig.Timestamp == 0 {
        sig.Timestamp = time.Now().UnixMilli()
    }
    b, _ := json.Marshal(sig)
    if err := s.pub.Publish(r.Context(), "signals", b); err != nil {
        log.Printf("publish failed: %v", err)
        http.Error(w, "publish failed", http.StatusInternalServerError)
        return
    }
    w.WriteHeader(http.StatusAccepted)
}

func (s *Server) getLimiter(key string) *rate.Limiter {
    s.mu.Lock()
    defer s.mu.Unlock()
    l, ok := s.limiters[key]
    if !ok {
        l = rate.NewLimiter(5, 10)
        s.limiters[key] = l
    }
    return l
}
