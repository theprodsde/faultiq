// health-poller autonomously polls registered service health endpoints and
// posts signals to signal-ingestion so the detection engine can process them.
//
// Supported health check types:
//   HTTP  — GET/POST to http:// or https:// endpoint (TLS-configurable)
//   HTTPS — same as HTTP; set tlsSkipVerify: true for self-signed certs
//   TCP   — tcp://host:port — just checks if the port is open (for databases)
//
// Response body formats auto-detected:
//   TechGraph standard: {"status","latency_p95_ms","error_rate"}
//   Spring Boot:        {"status":"UP"/"DOWN"/"OUT_OF_SERVICE"}
//   Kubernetes:         {"status":"ok"/"error"} or plain "ok"/"error"
//   Plain text:         "ok", "healthy", "UP" → healthy; anything else → degraded
//   None:               HTTP status code only

package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/faultiq/health-poller/servicemap"
)

const defaultPollInterval = 30 * time.Second
const defaultTimeout = 5 * time.Second

type Signal struct {
	TenantID           string  `json:"tenantId"`
	ProjectID          string  `json:"projectId"`
	Environment        string  `json:"environment"`
	Service            string  `json:"service"`
	StatusClass        string  `json:"statusClass"`
	ErrorRate          float64 `json:"errorRate"`
	LatencyP95         float64 `json:"latencyP95"`
	Timestamp          int64   `json:"timestamp"`
	Source             string  `json:"source"`
	ErrorRateThreshold float64 `json:"errorRateThreshold,omitempty"` // per-service threshold from service-map
	MinSignalCount     int     `json:"minSignalCount,omitempty"`     // consecutive unhealthy polls required (default 1)
}

type HealthResponse struct {
	Status       string  `json:"status"`
	LatencyP95Ms int     `json:"latency_p95_ms"`
	ErrorRate    float64 `json:"error_rate"`
}

// tokenCacheEntry holds a cached OAuth2 access token and its expiry.
type tokenCacheEntry struct {
	token     string
	expiresAt time.Time
}

type Poller struct {
	signalURL    string
	httpClient   *http.Client
	smMu         sync.RWMutex
	cachedSM     *servicemap.ServiceMap
	smPath       string
	smModTime    time.Time
	tokenMu      sync.Mutex
	tokenCache   map[string]*tokenCacheEntry // key = tokenURL+clientID
	clientCache  sync.Map                   // key = "tlsSkipVerify:timeout" → *http.Client
}

func NewPoller(signalURL, smPath string) *Poller {
	return &Poller{
		signalURL:  signalURL,
		smPath:     smPath,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		tokenCache: make(map[string]*tokenCacheEntry),
	}
}

// getOAuth2Token fetches or returns a cached Bearer token via client_credentials flow.
// The token is cached until 30 seconds before its expiry to avoid edge-case failures.
func (p *Poller) getOAuth2Token(ctx context.Context, cfg *servicemap.OAuth2TokenConfig) (string, error) {
	cacheKey := cfg.TokenURL + "|" + cfg.ClientID
	p.tokenMu.Lock()
	defer p.tokenMu.Unlock()

	// Return cached token if still valid
	if entry, ok := p.tokenCache[cacheKey]; ok && time.Now().Before(entry.expiresAt) {
		return entry.token, nil
	}

	// Fetch fresh token via client_credentials
	form := "grant_type=client_credentials&client_id=" + cfg.ClientID + "&client_secret=" + cfg.ClientSecret
	if cfg.Scope != "" {
		form += "&scope=" + cfg.Scope
	}
	req, err := http.NewRequestWithContext(ctx, "POST", cfg.TokenURL,
		strings.NewReader(form))
	if err != nil {
		return "", fmt.Errorf("oauth2 request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("oauth2 token fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("oauth2 token endpoint returned %d", resp.StatusCode)
	}

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"` // seconds
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("oauth2 decode: %w", err)
	}
	if result.AccessToken == "" {
		return "", fmt.Errorf("oauth2: empty access_token")
	}

	// Cache with 30s safety margin
	ttl := time.Duration(result.ExpiresIn)*time.Second - 30*time.Second
	if ttl < 10*time.Second {
		ttl = 10 * time.Second
	}
	p.tokenCache[cacheKey] = &tokenCacheEntry{
		token:     result.AccessToken,
		expiresAt: time.Now().Add(ttl),
	}
	log.Printf("health-poller: fetched OAuth2 token for %s (expires in %ds)", cfg.ClientID, result.ExpiresIn)
	return result.AccessToken, nil
}

// getServiceMap returns the cached service map, reloading from disk only when the file has changed.
// Thread-safe: uses a read lock for cache hits and a write lock for reloads.
func (p *Poller) getServiceMap() (*servicemap.ServiceMap, error) {
	info, err := os.Stat(p.smPath)
	if err != nil {
		return nil, err
	}

	p.smMu.RLock()
	if p.cachedSM != nil && !info.ModTime().After(p.smModTime) {
		defer p.smMu.RUnlock()
		return p.cachedSM, nil
	}
	p.smMu.RUnlock()

	// Reload
	sm, err := servicemap.Load(p.smPath)
	if err != nil {
		return nil, err
	}

	p.smMu.Lock()
	p.cachedSM = sm
	p.smModTime = info.ModTime()
	p.smMu.Unlock()

	log.Printf("health-poller: service map reloaded (%d projects)", len(sm.Projects))
	return sm, nil
}

// poll checks a single service health endpoint and returns a Signal.
// Dispatches to the appropriate check type based on healthUrl scheme and HealthCheck config.
func (p *Poller) poll(ctx context.Context, svc servicemap.ServiceEntry) Signal {
	timeout := defaultTimeout
	if svc.TimeoutSeconds > 0 {
		timeout = time.Duration(svc.TimeoutSeconds) * time.Second
	}

	// TCP check: tcp://host:port — for databases and message brokers
	if strings.HasPrefix(svc.HealthURL, "tcp://") {
		return p.pollTCP(ctx, svc, timeout)
	}

	// HTTPS or HTTP check
	return p.pollHTTP(ctx, svc, timeout)
}

// pollTCP checks if a TCP port is open.
// Used for databases (postgres:5432, redis:6379, kafka:9092) that don't have HTTP health endpoints.
func (p *Poller) pollTCP(ctx context.Context, svc servicemap.ServiceEntry, timeout time.Duration) Signal {
	addr := strings.TrimPrefix(svc.HealthURL, "tcp://")
	start := time.Now()

	conn, err := net.DialTimeout("tcp", addr, timeout)
	elapsed := time.Since(start)

	if err != nil {
		if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "deadline") {
			return Signal{Service: svc.ID, StatusClass: "timeout", LatencyP95: float64(timeout.Milliseconds()), Source: "health-poller"}
		}
		return Signal{Service: svc.ID, StatusClass: "connection_error", Source: "health-poller"}
	}
	conn.Close()

	log.Printf("health-poller: TCP %s → open (%.0fms)", addr, float64(elapsed.Milliseconds()))
	return Signal{
		Service:     svc.ID,
		StatusClass: "2xx",
		LatencyP95:  float64(elapsed.Milliseconds()),
		Source:      "health-poller",
	}
}

// pollHTTP performs an HTTP or HTTPS health check with full config support.
func (p *Poller) pollHTTP(ctx context.Context, svc servicemap.ServiceEntry, timeout time.Duration) Signal {
	cfg := svc.HealthCheck // may be nil → use defaults

	// Build HTTP client — cache by (tlsSkipVerify, timeout) so transports are reused across polls.
	tlsSkipVerify := cfg != nil && cfg.TLSSkipVerify
	cacheKey := fmt.Sprintf("%v:%v", tlsSkipVerify, timeout)
	var client *http.Client
	if v, ok := p.clientCache.Load(cacheKey); ok {
		client = v.(*http.Client)
	} else {
		tlsConfig := &tls.Config{}
		if tlsSkipVerify {
			tlsConfig.InsecureSkipVerify = true //nolint:gosec — intentional for internal services
		}
		client = &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				TLSClientConfig: tlsConfig,
				// Reasonable defaults for health polling
				MaxIdleConns:    10,
				IdleConnTimeout: 30 * time.Second,
			},
		}
		p.clientCache.Store(cacheKey, client)
	}

	// Determine HTTP method
	method := "GET"
	if cfg != nil && cfg.Method != "" {
		method = strings.ToUpper(cfg.Method)
	}

	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, method, svc.HealthURL, nil)
	if err != nil {
		return Signal{Service: svc.ID, StatusClass: "connection_error", Source: "health-poller"}
	}
	req.Header.Set("User-Agent", "TechGraph-HealthPoller/1.0")

	// Add custom headers (for auth-protected health endpoints with static keys)
	if cfg != nil {
		for k, v := range cfg.Headers {
			req.Header.Set(k, v)
		}
		// Fetch and inject OAuth2 Bearer token if configured
		if cfg.OAuth2Config != nil {
			token, tokenErr := p.getOAuth2Token(ctx, cfg.OAuth2Config)
			if tokenErr != nil {
				log.Printf("health-poller: OAuth2 token error for %s: %v — falling back to no auth", svc.ID, tokenErr)
			} else {
				req.Header.Set("Authorization", "Bearer "+token)
			}
		}
	}

	resp, err := client.Do(req)
	elapsed := time.Since(start)

	if err != nil {
		statusClass := "connection_error"
		if ctx.Err() != nil || strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "deadline") {
			statusClass = "timeout"
		} else if strings.Contains(err.Error(), "certificate") || strings.Contains(err.Error(), "tls") {
			// TLS errors are surfaced explicitly so operators know to set tlsSkipVerify
			log.Printf("health-poller: TLS error for %s: %v — consider setting tlsSkipVerify: true", svc.ID, err)
			statusClass = "connection_error"
		}
		return Signal{Service: svc.ID, StatusClass: statusClass, LatencyP95: float64(elapsed.Milliseconds()), Source: "health-poller"}
	}
	defer resp.Body.Close()

	latency := float64(elapsed.Milliseconds())
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))

	// Determine expected status codes
	expectedCodes := []int{200, 204}
	if cfg != nil && len(cfg.ExpectedStatus) > 0 {
		expectedCodes = cfg.ExpectedStatus
	}
	codeOK := false
	for _, c := range expectedCodes {
		if resp.StatusCode == c {
			codeOK = true
			break
		}
	}

	// Parse body based on format
	bodyFormat := "auto"
	if cfg != nil && cfg.BodyFormat != "" {
		bodyFormat = cfg.BodyFormat
	}

	errorRate := 0.0
	statusClass := parseHealthBody(body, bodyFormat, resp.StatusCode, codeOK)

	// Apply expected body substring check
	if cfg != nil && cfg.ExpectedBodyContains != "" && codeOK {
		if !strings.Contains(string(body), cfg.ExpectedBodyContains) {
			statusClass = "degraded" // body doesn't match expected pattern
			log.Printf("health-poller: %s returned 200 but body missing %q — marking degraded", svc.ID, cfg.ExpectedBodyContains)
		}
	}

	// Parse numeric metrics if available
	var hr HealthResponse
	if json.Unmarshal(body, &hr) == nil {
		if hr.LatencyP95Ms > 0 {
			latency = float64(hr.LatencyP95Ms)
		}
		if hr.ErrorRate > 0 {
			errorRate = hr.ErrorRate
		}
	}

	// Optional synthetic check — verify a real API endpoint works
	if cfg != nil && cfg.SyntheticPath != "" && statusClass == "2xx" {
		syntheticStatus := p.checkSynthetic(ctx, svc, cfg, client, timeout)
		if syntheticStatus != "2xx" {
			log.Printf("health-poller: %s health=OK but synthetic check %s failed → %s", svc.ID, cfg.SyntheticPath, syntheticStatus)
			statusClass = "degraded" // health says OK but real endpoint fails
		}
	}

	sig := Signal{
		Service:     svc.ID,
		StatusClass: statusClass,
		ErrorRate:   errorRate,
		LatencyP95:  latency,
		Source:      "health-poller",
	}

	// Log faults explicitly
	if statusClass != "2xx" {
		log.Printf("health-poller: FAULT %s → %s (HTTP %d, lat=%.0fms)", svc.ID, statusClass, resp.StatusCode, latency)
	}
	return sig
}

// parseHealthBody interprets the response body according to the specified format.
func parseHealthBody(body []byte, format string, httpCode int, codeOK bool) string {
	if !codeOK {
		return classifyHTTPStatusCode(httpCode)
	}

	bodyStr := strings.TrimSpace(string(body))

	switch format {
	case "none":
		// HTTP code only — ignore body
		return "2xx"

	case "spring":
		// Spring Boot Actuator: {"status":"UP"/"DOWN"/"OUT_OF_SERVICE"/"UNKNOWN"}
		var r struct{ Status string `json:"status"` }
		if json.Unmarshal(body, &r) == nil {
			switch strings.ToUpper(r.Status) {
			case "UP":
				return "2xx"
			case "DOWN":
				return "5xx"
			case "OUT_OF_SERVICE":
				return "503"
			default:
				return "degraded"
			}
		}
		return "2xx"

	case "kubernetes":
		// Kubernetes-style: {"status":"ok"} or plain "ok"
		var r struct{ Status string `json:"status"` }
		if json.Unmarshal(body, &r) == nil {
			if strings.ToLower(r.Status) == "ok" || strings.ToLower(r.Status) == "healthy" {
				return "2xx"
			}
			return "degraded"
		}
		// Plain text fallback
		lower := strings.ToLower(bodyStr)
		if lower == "ok" || lower == "healthy" || lower == "alive" {
			return "2xx"
		}
		return "degraded"

	default: // "auto" or "standard"
		// Single unmarshal covers both TechGraph-standard and Spring Boot formats.
		// Eliminates the previous double/triple json.Unmarshal: std and spring were
		// identical structs, and the spring branch recursively called this function again.
		var r struct {
			Status string `json:"status"`
		}
		if json.Unmarshal(body, &r) == nil {
			switch strings.ToLower(r.Status) {
			case "ok", "healthy", "up", "alive":
				return "2xx"
			case "down", "error", "unhealthy":
				return "5xx"
			case "degraded", "warn", "warning":
				return "degraded"
			case "out_of_service":
				return "503"
			default:
				if r.Status != "" {
					// Non-empty but unrecognized status (e.g. "UNKNOWN") → degraded
					return "degraded"
				}
			}
		}
		// Plain text
		lower := strings.ToLower(strings.TrimSpace(bodyStr))
		if lower == "ok" || lower == "healthy" || lower == "up" || lower == "alive" || lower == "1" {
			return "2xx"
		}
		// No body or unrecognised — trust HTTP status code
		return "2xx"
	}
}

// classifyHTTPStatusCode maps an HTTP status code to a statusClass.
func classifyHTTPStatusCode(code int) string {
	switch {
	case code >= 200 && code < 300:
		return "2xx"
	case code == 503:
		return "503"
	case code == 504:
		return "timeout"
	case code >= 500:
		return "5xx"
	case code == 401 || code == 403:
		return "connection_error" // auth failure — treat as misconfigured
	case code == 404:
		return "degraded" // health endpoint doesn't exist
	case code == 0:
		return "connection_error"
	default:
		return fmt.Sprintf("%dxx", code/100)
	}
}

// checkSynthetic performs an additional check against a real API endpoint.
// If the main /health passes but this synthetic endpoint fails, the service is "degraded".
func (p *Poller) checkSynthetic(ctx context.Context, svc servicemap.ServiceEntry, cfg *servicemap.HealthCheckConfig, client *http.Client, timeout time.Duration) string {
	// Build synthetic URL: replace /health with the synthetic path
	baseURL := svc.HealthURL
	if idx := strings.Index(baseURL, "/health"); idx > 0 {
		baseURL = baseURL[:idx]
	} else if idx := strings.LastIndex(baseURL, "/"); idx > 0 {
		baseURL = baseURL[:idx]
	}
	syntheticURL := baseURL + cfg.SyntheticPath

	method := "GET"
	if cfg.SyntheticMethod != "" {
		method = cfg.SyntheticMethod
	}

	var bodyReader io.Reader
	if cfg.SyntheticBody != "" {
		bodyReader = strings.NewReader(cfg.SyntheticBody)
	}

	ctx2, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx2, method, syntheticURL, bodyReader)
	if err != nil {
		return "degraded"
	}
	for k, v := range cfg.Headers {
		req.Header.Set(k, v)
	}
	if cfg.SyntheticBody != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.Do(req)
	if err != nil {
		return "degraded"
	}
	resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return "2xx"
	}
	return classifyHTTPStatusCode(resp.StatusCode)
}

// postSignal sends a signal to the signal-ingestion service.
func (p *Poller) postSignal(ctx context.Context, project servicemap.ProjectEntry, sig Signal) {
	sig.TenantID = project.Tenant
	sig.ProjectID = project.ID
	sig.Environment = project.Environment
	sig.Timestamp = time.Now().UnixMilli()

	body, _ := json.Marshal(sig)
	req, err := http.NewRequestWithContext(ctx, "POST", p.signalURL+"/api/v1/signals", bytes.NewReader(body))
	if err != nil {
		log.Printf("health-poller: build request error: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		log.Printf("health-poller: post signal failed for %s: %v", sig.Service, err)
		return
	}
	resp.Body.Close()

	if resp.StatusCode == http.StatusAccepted {
		if sig.StatusClass != "2xx" {
			log.Printf("health-poller: FAULT detected %s/%s → %s (err=%.3f lat=%.0fms)",
				project.Namespace, sig.Service, sig.StatusClass, sig.ErrorRate, sig.LatencyP95)
		}
	} else {
		log.Printf("health-poller: signal rejected %s status=%d", sig.Service, resp.StatusCode)
	}
}

// pollProject polls all services in one project concurrently.
func (p *Poller) pollProject(ctx context.Context, project servicemap.ProjectEntry) {
	var wg sync.WaitGroup
	for _, svc := range project.Services {
		if svc.HealthURL == "" {
			continue
		}
		wg.Add(1)
		go func(s servicemap.ServiceEntry) {
			defer wg.Done()
			sig := p.poll(ctx, s)
			// Propagate per-service thresholds to signal so detection-engine can use them
			if s.Thresholds != nil {
				if s.Thresholds.ErrorRateThreshold > 0 {
					sig.ErrorRateThreshold = s.Thresholds.ErrorRateThreshold
				}
				if s.Thresholds.MinSignalCount > 0 {
					sig.MinSignalCount = s.Thresholds.MinSignalCount
				}
			}
			p.postSignal(ctx, project, sig)
		}(svc)
	}
	wg.Wait()
}

// runProject starts a polling loop for one project.
// On each tick it calls getServiceMap() to pick up hot-reloaded service config.
func (p *Poller) runProject(ctx context.Context, project servicemap.ProjectEntry, interval time.Duration) {
	log.Printf("health-poller: project=%s namespace=%s services=%d interval=%s",
		project.Name, project.Namespace, len(project.Services), interval)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Poll immediately on start
	p.pollProject(ctx, project)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Hot-reload: refresh project from service map (no-op if file unchanged)
			if sm, err := p.getServiceMap(); err == nil {
				smByID := make(map[string]servicemap.ProjectEntry, len(sm.Projects))
				for _, proj := range sm.Projects {
					smByID[proj.ID] = proj
				}
				if updated, ok := smByID[project.ID]; ok {
					project = updated
				}
			}
			p.pollProject(ctx, project)
		}
	}
}

func main() {
	serviceMapFile := os.Getenv("SERVICE_MAP_FILE")
	if serviceMapFile == "" {
		serviceMapFile = "/etc/health-poller/service-map.yaml"
	}

	signalURL := os.Getenv("SIGNAL_INGESTION_URL")
	if signalURL == "" {
		signalURL = "http://signal-ingestion:8085"
	}

	pollIntervalStr := os.Getenv("POLL_INTERVAL_SECONDS")
	pollInterval := defaultPollInterval
	if pollIntervalStr != "" {
		if n, err := strconv.Atoi(pollIntervalStr); err == nil && n > 0 {
			pollInterval = time.Duration(n) * time.Second
		}
	}

	log.Printf("health-poller: starting (map=%s signal=%s interval=%s)", serviceMapFile, signalURL, pollInterval)

	poller := NewPoller(signalURL, serviceMapFile)
	sm, err := poller.getServiceMap()
	if err != nil {
		log.Fatalf("health-poller: cannot load service map: %v", err)
	}
	log.Printf("health-poller: loaded %d projects from service map", len(sm.Projects))
	ctx := context.Background()

	var wg sync.WaitGroup
	for _, project := range sm.Projects {
		wg.Add(1)
		go func(proj servicemap.ProjectEntry) {
			defer wg.Done()
			poller.runProject(ctx, proj, pollInterval)
		}(project)
	}
	wg.Wait()
}
