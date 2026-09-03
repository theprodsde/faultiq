// demo-services is a configurable mock HTTP server that simulates multiple
// services for local TechGraph demos. Each "virtual" service has an independently
// controllable health state so you can simulate realistic fault cascades without
// needing real microservices running.
//
// Health endpoint:  GET  /services/{name}/health
// Admin control:    PUT  /admin/services/{name}/status
// Reset all:        POST /admin/reset
// List all:         GET  /admin/services

package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type ServiceState struct {
	StatusClass string  `json:"statusClass"` // 2xx | 5xx | timeout | degraded | connection_error
	ErrorRate   float64 `json:"errorRate"`
	LatencyP95  int     `json:"latencyP95"` // ms — poller uses this for latency detection
	UpdatedAt   string  `json:"updatedAt"`
}

type HealthResponse struct {
	Status       string  `json:"status"`        // ok | degraded | down
	LatencyP95Ms int     `json:"latency_p95_ms"`
	ErrorRate    float64 `json:"error_rate"`
	ServiceName  string  `json:"service"`
}

var (
	mu       sync.RWMutex
	states   = map[string]*ServiceState{}
	defaults = map[string]*ServiceState{
		// Payments Platform — all healthy by default
		"api-gateway":           {StatusClass: "2xx", ErrorRate: 0.001, LatencyP95: 55},
		"payment-api":           {StatusClass: "2xx", ErrorRate: 0.002, LatencyP95: 95},
		"auth-service":          {StatusClass: "2xx", ErrorRate: 0.001, LatencyP95: 38},
		"ledger-service":        {StatusClass: "2xx", ErrorRate: 0.002, LatencyP95: 45},
		"settlement-worker":     {StatusClass: "2xx", ErrorRate: 0.001, LatencyP95: 120},
		"notification-service":  {StatusClass: "2xx", ErrorRate: 0.001, LatencyP95: 80},
		"postgres-db":           {StatusClass: "2xx", ErrorRate: 0.0, LatencyP95: 12},
		"redis-cache":           {StatusClass: "2xx", ErrorRate: 0.0, LatencyP95: 2},
		"ledger-events-topic":   {StatusClass: "2xx", ErrorRate: 0.0, LatencyP95: 8},
		"stripe-api":            {StatusClass: "2xx", ErrorRate: 0.003, LatencyP95: 380},
		// Orders Platform — all healthy by default
		"order-api":             {StatusClass: "2xx", ErrorRate: 0.001, LatencyP95: 80},
		"order-service":         {StatusClass: "2xx", ErrorRate: 0.002, LatencyP95: 95},
		"inventory-service":     {StatusClass: "2xx", ErrorRate: 0.001, LatencyP95: 60},
		"warehouse-service":     {StatusClass: "2xx", ErrorRate: 0.003, LatencyP95: 120},
		"orders-postgres":       {StatusClass: "2xx", ErrorRate: 0.0, LatencyP95: 10},
		"shipping-api":          {StatusClass: "2xx", ErrorRate: 0.005, LatencyP95: 450},
		"order-events-topic":    {StatusClass: "2xx", ErrorRate: 0.0, LatencyP95: 5},
	}
)

func init() {
	for name, d := range defaults {
		s := *d
		s.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		states[name] = &s
	}
}

func getState(name string) *ServiceState {
	mu.RLock()
	defer mu.RUnlock()
	s, ok := states[name]
	if !ok {
		return &ServiceState{StatusClass: "2xx", ErrorRate: 0.001, LatencyP95: 100, UpdatedAt: time.Now().UTC().Format(time.RFC3339)}
	}
	return s
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	// Path: /services/{name}/health
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 2 {
		http.Error(w, "bad path", http.StatusBadRequest)
		return
	}
	name := parts[1] // services/{name}/health

	s := getState(name)

	// Simulate latency if timeout
	if s.StatusClass == "timeout" {
		time.Sleep(time.Duration(s.LatencyP95) * time.Millisecond)
		http.Error(w, "timeout", http.StatusGatewayTimeout)
		return
	}
	if s.StatusClass == "connection_error" || s.StatusClass == "down" {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}

	// Determine HTTP status code
	httpStatus := http.StatusOK
	if len(s.StatusClass) > 0 && s.StatusClass[0] == '5' {
		httpStatus = http.StatusInternalServerError
	}

	// Map statusClass to health response status
	healthStatus := "ok"
	if s.StatusClass == "degraded" {
		healthStatus = "degraded"
	} else if httpStatus >= 500 {
		healthStatus = "down"
	}

	resp := HealthResponse{
		Status:       healthStatus,
		LatencyP95Ms: s.LatencyP95,
		ErrorRate:    s.ErrorRate,
		ServiceName:  name,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	_ = json.NewEncoder(w).Encode(resp)
}

func adminSetStatusHandler(w http.ResponseWriter, r *http.Request) {
	// PUT /admin/services/{name}/status
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 3 {
		http.Error(w, "bad path", http.StatusBadRequest)
		return
	}
	name := parts[2] // admin/services/{name}/status

	var body struct {
		StatusClass string  `json:"statusClass"`
		ErrorRate   float64 `json:"errorRate"`
		LatencyP95  int     `json:"latencyP95"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if body.StatusClass == "" {
		http.Error(w, "statusClass required", http.StatusBadRequest)
		return
	}

	mu.Lock()
	states[name] = &ServiceState{
		StatusClass: body.StatusClass,
		ErrorRate:   body.ErrorRate,
		LatencyP95:  body.LatencyP95,
		UpdatedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	mu.Unlock()

	log.Printf("demo-services: %s → %s (errorRate=%.3f latency=%dms)", name, body.StatusClass, body.ErrorRate, body.LatencyP95)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"service": name, "status": body.StatusClass})
}

func adminResetHandler(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	for name, d := range defaults {
		s := *d
		s.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		states[name] = &s
	}
	mu.Unlock()
	log.Printf("demo-services: all services reset to healthy defaults")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "reset"})
}

func adminListHandler(w http.ResponseWriter, r *http.Request) {
	mu.RLock()
	defer mu.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(states)
}

func main() {
	mux := http.NewServeMux()

	// Health endpoints — one per service name
	mux.HandleFunc("/services/", healthHandler)

	// Admin control endpoints
	mux.HandleFunc("/admin/services/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			adminSetStatusHandler(w, r)
		} else if r.Method == http.MethodGet {
			adminListHandler(w, r)
		} else {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/admin/reset", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			adminResetHandler(w, r)
		}
	})
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	port := os.Getenv("DEMO_SERVICES_PORT")
	if port == "" {
		port = "8091"
	}
	log.Printf("demo-services: listening on :%s", port)
	log.Printf("demo-services: %d services pre-configured", len(states))
	log.Printf("demo-services: health at /services/{name}/health")
	log.Printf("demo-services: control at PUT /admin/services/{name}/status")
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
