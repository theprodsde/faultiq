package servicemap

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// OAuth2TokenConfig configures dynamic Bearer token fetching via client_credentials flow.
// The health-poller fetches a fresh token before each health check and caches it until expiry.
type OAuth2TokenConfig struct {
	// TokenURL is the OAuth2 token endpoint.
	// Example: https://keycloak.company.com/realms/myrealm/protocol/openid-connect/token
	TokenURL string `yaml:"tokenUrl"`

	// ClientID for the service account / machine client.
	ClientID string `yaml:"clientId"`

	// ClientSecret — can reference env var with ${VAR_NAME} syntax.
	// Example: clientSecret: ${HEALTH_CHECKER_SECRET}
	ClientSecret string `yaml:"clientSecret"`

	// Scope — optional OAuth2 scope(s) to request.
	// Example: "health:read openid"
	Scope string `yaml:"scope,omitempty"`
}

// HealthCheckConfig defines how to check a service's health.
// If not specified, defaults to GET healthUrl with 5s timeout.
type HealthCheckConfig struct {
	// Type of check: "http" | "tcp" | "https"
	// Default: "http" (or "https" if healthUrl starts with https://)
	Type string `yaml:"type"`

	// HTTP method — default GET
	Method string `yaml:"method"`

	// Expected HTTP status codes — default [200, 204]
	// Example: [200, 201, 204]
	ExpectedStatus []int `yaml:"expectedStatus"`

	// ExpectedBodyContains — substring that must appear in the response body
	// If set, a 200 response that doesn't contain this string is treated as degraded.
	// Example: '"status":"UP"' for Spring Boot, '"status":"ok"' for standard JSON
	ExpectedBodyContains string `yaml:"expectedBodyContains"`

	// BodyFormat controls how the response body is parsed for metrics:
	//   "auto"        — try JSON first, fall back to plain text (default)
	//   "spring"      — Spring Boot Actuator: {"status":"UP/DOWN/OUT_OF_SERVICE"}
	//   "kubernetes"  — K8s style: {"status":"ok"} or plain "ok"
	//   "standard"    — TechGraph standard: {"status","latency_p95_ms","error_rate"}
	//   "none"        — ignore body, use HTTP status code only
	BodyFormat string `yaml:"bodyFormat"`

	// TLSSkipVerify — skip TLS certificate verification (for internal self-signed certs)
	// WARNING: only use for internal services, never for public endpoints
	TLSSkipVerify bool `yaml:"tlsSkipVerify"`

	// Headers to include in the health check request (static values)
	// Use this for static API keys that don't expire.
	// Example: {"X-Health-Token": "abc", "X-Internal-Key": "secret"}
	// WARNING: Do NOT put JWT tokens here — they expire. Use OAuth2Config instead.
	Headers map[string]string `yaml:"headers"`

	// OAuth2Config — fetch a fresh Bearer token before each health check.
	// Use this when the health endpoint requires a real OAuth2 access token.
	// The token is cached and refreshed automatically when it expires.
	// Example (Keycloak client credentials):
	//   tokenUrl: http://keycloak:8080/realms/myrealm/protocol/openid-connect/token
	//   clientId: health-checker
	//   clientSecret: ${HEALTH_CHECKER_SECRET}   # reference env var with ${VAR}
	//   scope: health:read
	OAuth2Config *OAuth2TokenConfig `yaml:"oauth2,omitempty"`

	// ProbeType controls the interpretation of the check result:
	//   "readiness"  — service is ready to serve traffic (default)
	//   "liveness"   — process is alive (less strict)
	// Liveness failures trigger immediate alerts; readiness failures cause degraded state.
	ProbeType string `yaml:"probeType"`

	// SyntheticPath — in addition to healthUrl, also check this path.
	// If this path returns an error while healthUrl is healthy, the service is "degraded".
	// Example: "/api/v1/products?limit=1" to verify the API actually works
	SyntheticPath string `yaml:"syntheticPath"`

	// SyntheticMethod — HTTP method for synthetic check, default GET
	SyntheticMethod string `yaml:"syntheticMethod"`

	// SyntheticBody — request body for synthetic check (e.g. for POST endpoints)
	SyntheticBody string `yaml:"syntheticBody"`
}

// ThresholdConfig defines per-service signal thresholds for fault detection.
// These override the global defaults in the detection engine.
type ThresholdConfig struct {
	// ErrorRateThreshold — skip fault signal if error rate is below this value.
	// Default: 0.10 (10%). For payment-service SLO of 0.1%, set this to 0.001.
	ErrorRateThreshold float64 `yaml:"errorRateThreshold"`
	// LatencyThresholdMs — alert if P95 latency exceeds this value (ms). 0 = disabled.
	LatencyThresholdMs int `yaml:"latencyThresholdMs"`
	// MinSignalCount — minimum consecutive unhealthy signals before alerting. Default 1.
	MinSignalCount int `yaml:"minSignalCount"`
}

// CustomSOPStep defines a single step in a custom SOP playbook.
// Custom playbooks override the auto-generated steps for a specific service.
type CustomSOPStep struct {
	Order          int    `yaml:"order"`
	Title          string `yaml:"title"`
	Action         string `yaml:"action"`
	ExpectedSignal string `yaml:"expectedSignal"`
	AutoVerify     bool   `yaml:"autoVerify"`
}

// ServiceEntry defines a single service to monitor.
type ServiceEntry struct {
	ID   string `yaml:"id"`
	Name string `yaml:"name"`
	Type string `yaml:"type"`

	// HealthURL is the primary health endpoint URL.
	// Supports: http://, https://, tcp:// (for TCP port checks)
	// Examples:
	//   http://payment-service:8080/health          standard HTTP
	//   https://payment-service:8443/health         HTTPS (TLSSkipVerify if needed)
	//   tcp://postgres:5432                         TCP port check (for databases)
	//   http://localhost:8080/actuator/health        Spring Boot
	//   http://pod-name/readyz                      Kubernetes readiness
	HealthURL string `yaml:"healthUrl"`

	// HealthCheck — advanced configuration for how to interpret the health endpoint.
	// Optional: if not set, uses sensible defaults based on the URL and response.
	HealthCheck *HealthCheckConfig `yaml:"healthCheck,omitempty"`

	Calls               []string `yaml:"calls"`
	Tags                []string `yaml:"tags"`
	Repo                string   `yaml:"repo"`
	Branch              string   `yaml:"branch"`
	CodePath            string   `yaml:"codePath"`
	PollIntervalSeconds int      `yaml:"pollIntervalSeconds"`
	TimeoutSeconds      int      `yaml:"timeoutSeconds"`

	// Thresholds — per-service signal thresholds. Overrides global defaults in detection-engine.
	// If nil, detection-engine uses its global defaults (errorRate < 0.10).
	Thresholds *ThresholdConfig `yaml:"thresholds,omitempty"`

	// CustomPlaybook — custom SOP steps for this service. If set, these steps replace
	// the auto-generated fault-type defaults when an incident is created.
	// Example: payment-service step 3 is "kubectl rollout undo payment-service"
	CustomPlaybook []CustomSOPStep `yaml:"sopPlaybook,omitempty"`

	// AlertChannel — Slack channel or webhook URL specific to this service.
	// If set, notifications for this service go to this channel, not the global NOTIFICATION_WEBHOOK_URL.
	// Example: "#payments-oncall" or "https://hooks.slack.com/services/..."
	AlertChannel string `yaml:"alertChannel"`

	// AlertPriority controls which notification channel receives alerts for this service.
	// "P1" → urgent/paging alert (e.g. PagerDuty), "P2" → informational alert (e.g. Slack)
	// If empty, uses the global NOTIFICATION_WEBHOOK_URL.
	AlertPriority string `yaml:"alertPriority"` // P1 | P2 | "" (default)
}

type ProjectEntry struct {
	ID          string         `yaml:"id"`
	Name        string         `yaml:"name"`
	Namespace   string         `yaml:"namespace"`
	Tenant      string         `yaml:"tenant"`
	Environment string         `yaml:"environment"`
	Services    []ServiceEntry `yaml:"services"`
}

type ServiceMap struct {
	Version  string         `yaml:"version"`
	Projects []ProjectEntry `yaml:"projects"`
}

func Load(path string) (*ServiceMap, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read service map %s: %w", path, err)
	}
	// Expand ${ENV_VAR} references before parsing YAML.
	// This lets secrets be stored in environment variables instead of plain YAML.
	expanded := os.Expand(string(data), func(key string) string {
		if val := os.Getenv(key); val != "" {
			return val
		}
		return "${" + key + "}" // leave unexpanded if var not set
	})
	_ = strings.Contains // use the import
	var sm ServiceMap
	if err := yaml.Unmarshal([]byte(expanded), &sm); err != nil {
		return nil, fmt.Errorf("parse service map: %w", err)
	}
	return &sm, nil
}
