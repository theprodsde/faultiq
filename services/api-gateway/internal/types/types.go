package types

// Project represents a minimal project entity used by the API gateway.
type Project struct {
    ID          string `json:"id"`
    Name        string `json:"name"`
    Slug        string `json:"slug,omitempty"`
    Environment string `json:"environment,omitempty"`
    TenantID    string `json:"tenant_id,omitempty"`
}

// ProjectListItem for list responses
type ProjectListItem struct {
    ID            string `json:"id"`
    Name          string `json:"name"`
    Slug          string `json:"slug"`
    GraphStatus   string `json:"graphStatus"` // DRAFT, PUBLISHED, ARCHIVED
    IncidentCount int    `json:"incidentCount"`
}

// Signal represents an incoming telemetry or event from services.
type Signal struct {
    TenantID    string  `json:"tenantId"`
    ProjectID   string  `json:"projectId"`
    Service     string  `json:"service"`
    StatusClass string  `json:"statusClass"` // e.g. "2xx", "5xx", "timeout"
    ErrorRate   float64 `json:"errorRate,omitempty"`
    LatencyP95  float64 `json:"latencyP95,omitempty"`
    Timestamp   int64   `json:"timestamp,omitempty"`
    Source      string  `json:"source,omitempty"`
}

// Incident is a synthesized incident created from incoming signals.
type Incident struct {
    ID                   string   `json:"id"`
    TenantID             string   `json:"tenantId"`
    ProjectID            string   `json:"projectId"`
    Service              string   `json:"service"`
    Status               string   `json:"status"` // OPEN, ACKNOWLEDGED, RESOLVED
    DetectedAt           int64    `json:"detectedAt"` // Unix timestamp in milliseconds
    FirstSeen            int64    `json:"firstSeen"`
    RootCauseCandidate   string   `json:"rootCauseCandidate,omitempty"` // top suspect node ID
    Confidence           float64  `json:"confidence,omitempty"` // 0.0-1.0
    AffectedServices     []string `json:"affectedServices,omitempty"` // list of impacted service IDs
    Evidence             []Signal `json:"evidence,omitempty"`
}

// IncidentListItem for list responses
type IncidentListItem struct {
    ID                 string  `json:"id"`
    ProjectID          string  `json:"projectId"`
    Service            string  `json:"service"`
    Status             string  `json:"status"` // OPEN, ACKNOWLEDGED, RESOLVED
    Phase              string  `json:"phase,omitempty"`
    Environment        string  `json:"environment,omitempty"`
    DetectedAt         int64   `json:"detectedAt"`
    RootCauseCandidate string  `json:"rootCauseCandidate,omitempty"`
    Confidence         float64 `json:"confidence,omitempty"`
    EvidenceCount      int     `json:"evidenceCount"`
}
