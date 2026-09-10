// otel-receiver accepts OpenTelemetry trace JSON payloads on POST /v1/traces,
// aggregates spans per service in 30-second windows, and posts computed error_rate
// and latency_p95 signals to signal-ingestion.
//
// This enables any service instrumented with OTel to feed TechGraph automatically:
//   Service → OTLP exporter → otel-receiver → signal-ingestion → detection-engine
//
// Input format: OTel JSON export (same as Jaeger/Tempo JSON output).
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
	"sort"
	"strings"
	"sync"
	"time"
)

// ─────────────────────────────────────────────────────────────────────────────
// OTel JSON types (minimal subset we need)
// ─────────────────────────────────────────────────────────────────────────────

type otelRequest struct {
	ResourceSpans []resourceSpans `json:"resourceSpans"`
}

type resourceSpans struct {
	Resource   otelResource   `json:"resource"`
	ScopeSpans []scopeSpans   `json:"scopeSpans"`
}

type otelResource struct {
	Attributes []otelAttribute `json:"attributes"`
}

type scopeSpans struct {
	Spans []otelSpan `json:"spans"`
}

type otelSpan struct {
	TraceID           string          `json:"traceId"`
	SpanID            string          `json:"spanId"`
	ParentSpanID      string          `json:"parentSpanId"`
	Name              string          `json:"name"`
	Kind              int             `json:"kind"`
	StartTimeUnixNano string          `json:"startTimeUnixNano"`
	EndTimeUnixNano   string          `json:"endTimeUnixNano"`
	Attributes        []otelAttribute `json:"attributes"`
	Status            otelStatus      `json:"status"`
	ServiceName       string          `json:"-"` // populated during processing, not from JSON
}

type otelStatus struct {
	Code    int    `json:"code"`    // 0=unset, 1=ok, 2=error
	Message string `json:"message"`
}

type otelAttribute struct {
	Key   string          `json:"key"`
	Value json.RawMessage `json:"value"`
}

// ─────────────────────────────────────────────────────────────────────────────
// OTel Metrics JSON types
// ─────────────────────────────────────────────────────────────────────────────

type otelMetricsRequest struct {
	ResourceMetrics []resourceMetrics `json:"resourceMetrics"`
}

type resourceMetrics struct {
	Resource     otelResource   `json:"resource"`
	ScopeMetrics []scopeMetrics `json:"scopeMetrics"`
}

type scopeMetrics struct {
	Metrics []otelMetric `json:"metrics"`
}

type otelMetric struct {
	Name      string         `json:"name"`
	Histogram *histogramData `json:"histogram,omitempty"`
	Sum       *sumData       `json:"sum,omitempty"`
	Gauge     *gaugeData     `json:"gauge,omitempty"`
}

type histogramData struct {
	DataPoints []histogramDataPoint `json:"dataPoints"`
}

type histogramDataPoint struct {
	Sum        float64         `json:"sum"`
	Count      float64         `json:"count"`
	Attributes []otelAttribute `json:"attributes"`
}

type sumData struct {
	DataPoints []numberDataPoint `json:"dataPoints"`
}

type gaugeData struct {
	DataPoints []numberDataPoint `json:"dataPoints"`
}

type numberDataPoint struct {
	AsDouble   float64         `json:"asDouble"`
	AsInt      float64         `json:"asInt"`
	Attributes []otelAttribute `json:"attributes"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Aggregation window
// ─────────────────────────────────────────────────────────────────────────────

type spanRecord struct {
	serviceID   string
	durationMs  float64
	isError     bool
}

type windowAggregator struct {
	mu      sync.Mutex
	spans   []spanRecord
}

func (a *windowAggregator) add(s spanRecord) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.spans = append(a.spans, s)
}

// flush drains the window and returns per-service aggregates.
func (a *windowAggregator) flush() map[string]serviceAgg {
	a.mu.Lock()
	all := a.spans
	a.spans = nil
	a.mu.Unlock()

	byService := make(map[string][]spanRecord)
	for _, s := range all {
		byService[s.serviceID] = append(byService[s.serviceID], s)
	}
	result := make(map[string]serviceAgg)
	for svcID, records := range byService {
		durations := make([]float64, 0, len(records))
		errorCount := 0
		for _, r := range records {
			durations = append(durations, r.durationMs)
			if r.isError {
				errorCount++
			}
		}
		sort.Float64s(durations)
		p95 := 0.0
		if len(durations) > 0 {
			idx := int(math.Ceil(float64(len(durations))*0.95)) - 1
			if idx < 0 {
				idx = 0
			}
			p95 = durations[idx]
		}
		errorRate := float64(errorCount) / float64(len(records))
		result[svcID] = serviceAgg{
			serviceID: svcID,
			p95Ms:     p95,
			errorRate: errorRate,
			spanCount: len(records),
		}
	}
	return result
}

type serviceAgg struct {
	serviceID string
	p95Ms     float64
	errorRate float64
	spanCount int
}

// ─────────────────────────────────────────────────────────────────────────────
// Signal output
// ─────────────────────────────────────────────────────────────────────────────

type signal struct {
	Service     string  `json:"service"`
	StatusClass string  `json:"statusClass"`
	ErrorRate   float64 `json:"errorRate"`
	LatencyP95  float64 `json:"latencyP95"`
	Timestamp   int64   `json:"timestamp"`
	Source      string  `json:"source"`
}

func postSignal(ctx context.Context, ingestionURL string, sig signal) {
	body, err := json.Marshal(sig)
	if err != nil {
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ingestionURL+"/api/v1/signals", bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("otel-receiver: post signal failed for %s: %v", sig.Service, err)
		return
	}
	resp.Body.Close()
}

// ─────────────────────────────────────────────────────────────────────────────
// Span parsing helpers
// ─────────────────────────────────────────────────────────────────────────────

// getAttrString extracts a string attribute value by key.
func getAttrString(attrs []otelAttribute, key string) string {
	for _, a := range attrs {
		if a.Key == key {
			var v struct {
				StringValue string `json:"stringValue"`
			}
			if json.Unmarshal(a.Value, &v) == nil && v.StringValue != "" {
				return v.StringValue
			}
			// Also try plain string
			var s string
			if json.Unmarshal(a.Value, &s) == nil {
				return s
			}
		}
	}
	return ""
}

// getAttrInt extracts an integer attribute value by key.
func getAttrInt(attrs []otelAttribute, key string) int {
	for _, a := range attrs {
		if a.Key == key {
			var v struct {
				IntValue    int    `json:"intValue"`
				StringValue string `json:"stringValue"`
			}
			if json.Unmarshal(a.Value, &v) == nil {
				if v.IntValue != 0 {
					return v.IntValue
				}
			}
		}
	}
	return 0
}

// parseNanos converts an OTel nanosecond timestamp string to int64.
func parseNanos(s string) int64 {
	if s == "" {
		return 0
	}
	var n int64
	fmt.Sscanf(s, "%d", &n)
	return n
}

// ─────────────────────────────────────────────────────────────────────────────
// Call-graph edge extraction
// ─────────────────────────────────────────────────────────────────────────────

type graphEdge struct {
	From      string
	To        string
	Namespace string
}

// extractCallEdges reads trace spans and infers service-to-service CALLS relationships.
// Returns unique edges deduped by (from, to).
// For each span that is a child of another span: parent service → CALLS → child service.
func extractCallEdges(spans []otelSpan, namespace string) []graphEdge {
	// Build map: spanId → serviceName for quick lookup
	spanServices := make(map[string]string)
	for _, sp := range spans {
		if sp.ServiceName != "" {
			spanServices[sp.SpanID] = sp.ServiceName
		}
	}

	seen := map[string]bool{}
	edges := make([]graphEdge, 0, len(spans))
	for _, sp := range spans {
		if sp.ParentSpanID == "" {
			continue
		}
		parentSvc := spanServices[sp.ParentSpanID]
		if parentSvc == "" || parentSvc == sp.ServiceName {
			continue
		}
		key := parentSvc + "→" + sp.ServiceName
		if !seen[key] {
			seen[key] = true
			edges = append(edges, graphEdge{From: parentSvc, To: sp.ServiceName, Namespace: namespace})
		}
	}
	return edges
}

// sanitizeServiceID lowercases name and replaces non-alphanumeric characters with '_'.
func sanitizeServiceID(name string) string {
	name = strings.ToLower(name)
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			return r
		}
		return '_'
	}, name)
}

// postCallGraphEdges posts service nodes and CALLS edges to graph-manager.
// Called in a goroutine so it does not block the HTTP response.
func postCallGraphEdges(graphManagerURL, namespace string, spans []otelSpan) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	edges := extractCallEdges(spans, namespace)

	// Collect unique service names
	seenSvcs := make(map[string]bool)
	for _, sp := range spans {
		if sp.ServiceName != "" {
			seenSvcs[sp.ServiceName] = true
		}
	}

	// Ensure a node exists for every service seen in these traces.
	// graph-manager uses ON CONFLICT DO NOTHING so this is idempotent.
	for svc := range seenSvcs {
		node := map[string]interface{}{
			"id":          sanitizeServiceID(svc),
			"name":        svc,
			"type":        "SERVICE",
			"statusClass": "2xx",
			"latencyP95":  0,
		}
		body, _ := json.Marshal(node)
		u := fmt.Sprintf("%s/api/v1/graphs/nodes?namespace=%s", graphManagerURL, url.QueryEscape(namespace))
		req, err := http.NewRequestWithContext(ctx, "POST", u, bytes.NewReader(body))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			resp.Body.Close()
		}
	}

	// Post every unique CALLS edge.
	for _, edge := range edges {
		payload := map[string]interface{}{
			"id":           "otel-" + edge.From + "-" + edge.To,
			"from":         sanitizeServiceID(edge.From),
			"to":           sanitizeServiceID(edge.To),
			"type":         "CALLS",
			"confidence":   0.95,
			"successRatio": 0.99,
		}
		body, _ := json.Marshal(payload)
		u := fmt.Sprintf("%s/api/v1/graphs/edges?namespace=%s", graphManagerURL, url.QueryEscape(edge.Namespace))
		req, err := http.NewRequestWithContext(ctx, "POST", u, bytes.NewReader(body))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Printf("otel-receiver: post edge %s→%s failed: %v", edge.From, edge.To, err)
			continue
		}
		resp.Body.Close()
	}
	if len(edges) > 0 {
		log.Printf("otel-receiver: posted %d call edges to graph-manager (namespace=%s)", len(edges), namespace)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// HTTP handlers
// ─────────────────────────────────────────────────────────────────────────────

// metricsHandler accepts OTel JSON metrics on POST /v1/metrics.
// It extracts per-service error rates and latency from histogram metrics,
// then immediately posts a signal to signal-ingestion.
func metricsHandler(ingestionURL string, apiKey string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if apiKey != "" && r.Header.Get("X-API-Key") != apiKey {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var payload otelMetricsRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
			return
		}

		type metricAgg struct {
			totalDurationMs float64
			totalCount      float64
			errorCount      float64
		}
		byService := make(map[string]*metricAgg)

		for _, rm := range payload.ResourceMetrics {
			serviceID := getAttrString(rm.Resource.Attributes, "service.name")
			if serviceID == "" {
				serviceID = "unknown-service"
			}
			if byService[serviceID] == nil {
				byService[serviceID] = &metricAgg{}
			}
			agg := byService[serviceID]

			for _, sm := range rm.ScopeMetrics {
				for _, metric := range sm.Metrics {
					switch metric.Name {
					case "http.server.request.duration":
						if metric.Histogram != nil {
							for _, dp := range metric.Histogram.DataPoints {
								statusCode := getAttrInt(dp.Attributes, "http.status_code")
								// OTel duration histogram is in seconds per spec; convert to ms
								durationMs := dp.Sum * 1000.0
								agg.totalDurationMs += durationMs
								agg.totalCount += dp.Count
								if statusCode >= 500 {
									agg.errorCount += dp.Count
								}
							}
						}
					case "http.server.errors", "rpc.server.errors":
						if metric.Sum != nil {
							for _, dp := range metric.Sum.DataPoints {
								agg.errorCount += dp.AsDouble + dp.AsInt
							}
						}
						if metric.Gauge != nil {
							for _, dp := range metric.Gauge.DataPoints {
								agg.errorCount += dp.AsDouble + dp.AsInt
							}
						}
					}
				}
			}
		}

		ctx := r.Context()
		count := 0
		for svcID, agg := range byService {
			if agg.totalCount == 0 {
				continue
			}
			errorRate := agg.errorCount / agg.totalCount
			meanMs := agg.totalDurationMs / agg.totalCount
			// Approximate p95 as 1.5× mean (conservative estimate without explicit bucket data)
			p95Ms := meanMs * 1.5

			statusClass := "2xx"
			if errorRate > 0.1 {
				statusClass = "5xx"
			}

			sig := signal{
				Service:     svcID,
				StatusClass: statusClass,
				ErrorRate:   errorRate,
				LatencyP95:  p95Ms,
				Timestamp:   time.Now().UnixMilli(),
				Source:      "otel-metrics",
			}
			sigCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			postSignal(sigCtx, ingestionURL, sig)
			cancel()
			log.Printf("otel-receiver: metrics signal service=%s errorRate=%.3f p95=%.0fms",
				svcID, errorRate, p95Ms)
			count++
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]int{"servicesProcessed": count})
	}
}

func tracesHandler(agg *windowAggregator, apiKey string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if apiKey != "" && r.Header.Get("X-API-Key") != apiKey {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var payload otelRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
			return
		}

		count := 0
		var allSpans []otelSpan // collected for call-graph edge extraction

		for _, rs := range payload.ResourceSpans {
			// Extract service name from resource attributes
			serviceID := getAttrString(rs.Resource.Attributes, "service.name")
			if serviceID == "" {
				serviceID = "unknown-service"
			}

			for _, ss := range rs.ScopeSpans {
				for _, span := range ss.Spans {
					startNs := parseNanos(span.StartTimeUnixNano)
					endNs := parseNanos(span.EndTimeUnixNano)
					durationMs := 0.0
					if startNs > 0 && endNs > startNs {
						durationMs = float64(endNs-startNs) / 1e6
					}

					// Determine if span is an error:
					// Status code 2 = error, or http.status_code >= 500
					isError := span.Status.Code == 2
					if !isError {
						httpStatus := getAttrInt(span.Attributes, "http.status_code")
						if httpStatus >= 500 {
							isError = true
						}
					}

					// Override serviceID from span attribute if present
					svcOverride := getAttrString(span.Attributes, "service.name")
					if svcOverride != "" {
						serviceID = svcOverride
					}

					agg.add(spanRecord{
						serviceID:  serviceID,
						durationMs: durationMs,
						isError:    isError,
					})

					// Tag span with resolved service name for call-graph extraction
					span.ServiceName = serviceID
					allSpans = append(allSpans, span)
					count++
				}
			}
		}

		// Extract and post service-to-service CALLS edges in the background
		// so we don't block the HTTP response.
		if len(allSpans) > 0 {
			namespace := os.Getenv("OTEL_DEFAULT_NAMESPACE")
			if namespace == "" {
				namespace = "otel-auto:prod"
			}
			gmURL := os.Getenv("GRAPH_MANAGER_URL")
			if gmURL == "" {
				gmURL = "http://graph-manager:8086"
			}
			go postCallGraphEdges(gmURL, namespace, allSpans)
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]int{"spansAccepted": count})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Main
// ─────────────────────────────────────────────────────────────────────────────

func main() {
	ingestionURL := os.Getenv("SIGNAL_INGESTION_URL")
	if ingestionURL == "" {
		ingestionURL = "http://signal-ingestion:8085"
	}
	port := os.Getenv("OTEL_RECEIVER_PORT")
	if port == "" {
		port = "4317"
	}
	apiKey := os.Getenv("OTEL_API_KEY")
	windowSec := 30 * time.Second

	agg := &windowAggregator{}

	// Background goroutine: flush window every 30s and post signals
	go func() {
		ticker := time.NewTicker(windowSec)
		defer ticker.Stop()
		for range ticker.C {
			aggregates := agg.flush()
			for _, a := range aggregates {
				statusClass := "2xx"
				if a.errorRate > 0.5 {
					statusClass = "5xx"
				} else if a.errorRate > 0.1 {
					statusClass = "5xx" // treat as degraded
				}
				sig := signal{
					Service:     a.serviceID,
					StatusClass: statusClass,
					ErrorRate:   a.errorRate,
					LatencyP95:  a.p95Ms,
					Timestamp:   time.Now().UnixMilli(),
					Source:      "otel-receiver",
				}
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				postSignal(ctx, ingestionURL, sig)
				cancel()
				log.Printf("otel-receiver: posted signal service=%s errorRate=%.3f p95=%.0fms spans=%d",
					a.serviceID, a.errorRate, a.p95Ms, a.spanCount)
			}
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("/v1/traces", tracesHandler(agg, apiKey))
	mux.HandleFunc("/v1/metrics", metricsHandler(ingestionURL, apiKey))

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	log.Printf("otel-receiver: listening on :%s, forwarding to %s", port, ingestionURL)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("otel-receiver: %v", err)
	}
}
