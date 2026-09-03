package types

import "time"

// =============================================================================
// Tenant + Project hierarchy
// =============================================================================

type Tenant struct {
	ID        string    `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	Slug      string    `json:"slug" db:"slug"`
	Plan      string    `json:"plan" db:"plan"`
	CreatedAt time.Time `json:"createdAt" db:"created_at"`
}

type Project struct {
	ID           string        `json:"id" db:"id"`
	TenantID     string        `json:"tenantId" db:"tenant_id"`
	Name         string        `json:"name" db:"name"`
	Slug         string        `json:"slug" db:"slug"`
	Domain       string        `json:"domain" db:"domain"`
	Environments []Environment `json:"environments,omitempty" db:"-"`
	CreatedAt    time.Time     `json:"createdAt" db:"created_at"`
}

type Environment struct {
	ID             string  `json:"id" db:"id"`
	ProjectID      string  `json:"projectId" db:"project_id"`
	Name           string  `json:"name" db:"name"`
	GraphNamespace string  `json:"namespace" db:"graph_namespace"`
}

// =============================================================================
// Graph model
// =============================================================================

// NodeType classifies what kind of infrastructure entity a node represents.
type NodeType string

const (
	NodeTypeService  NodeType = "SERVICE"
	NodeTypeDatabase NodeType = "DATABASE"
	NodeTypeQueue    NodeType = "QUEUE"
	NodeTypeExternal NodeType = "EXTERNAL"
	NodeTypeGateway  NodeType = "GATEWAY"
	NodeTypeSidecar  NodeType = "SIDECAR"
)

// EdgeType describes the nature of the dependency between two nodes.
type EdgeType string

const (
	EdgeTypeCalls        EdgeType = "CALLS"
	EdgeTypeDependsOn    EdgeType = "DEPENDS_ON"
	EdgeTypePublishesTo  EdgeType = "PUBLISHES_TO"
	EdgeTypeConsumesFrom EdgeType = "CONSUMES_FROM"
	EdgeTypeProxiedBy    EdgeType = "PROXIED_BY"
)

// GraphStatus tracks the lifecycle of a graph version.
type GraphStatus string

const (
	GraphStatusDraft     GraphStatus = "DRAFT"
	GraphStatusPublished GraphStatus = "PUBLISHED"
	GraphStatusArchived  GraphStatus = "ARCHIVED"
)

// ServiceNode is a vertex in the Detection Graph.
// Stored in Neo4j with namespace isolation.
type ServiceNode struct {
	ID          string            `json:"id"`
	Namespace   string            `json:"namespace"`   // tenantId:projectSlug:environment
	Name        string            `json:"name"`
	Type        NodeType          `json:"type"`
	Tags        []string          `json:"tags"`
	SLAMs       int               `json:"slaThresholdMs"` // latency SLA in ms
	RuntimeState *NodeRuntimeState `json:"runtimeState,omitempty"`
}

// NodeRuntimeState holds the latest observed health signal for a node.
// Populated during detection traversal from ingested signals.
type NodeRuntimeState struct {
	StatusClass  StatusClass `json:"statusClass"`
	ErrorRate    float64     `json:"errorRate"`
	LatencyP95   int         `json:"latencyP95"`
	TimeoutCount int         `json:"timeoutCount"`
	ObservedAt   time.Time   `json:"observedAt"`
}

// DependencyEdge is a directed edge between two ServiceNodes.
type DependencyEdge struct {
	ID           string    `json:"id"`
	Namespace    string    `json:"namespace"`
	From         string    `json:"from"`   // ServiceNode.ID
	To           string    `json:"to"`     // ServiceNode.ID
	Type         EdgeType  `json:"type"`
	Confidence   float64   `json:"confidence"`   // 0.0 – 1.0, inferred accuracy
	SuccessRatio float64   `json:"successRatio"` // live observed success rate
	LastObserved time.Time `json:"lastObserved"`
	Critical     bool      `json:"critical"`
	Metadata     EdgeMeta  `json:"metadata,omitempty"`
}

type EdgeMeta struct {
	Protocol string `json:"protocol,omitempty"` // HTTP, gRPC, AMQP
	Method   string `json:"method,omitempty"`   // GET, POST etc.
	Path     string `json:"path,omitempty"`
}

// GraphVersion represents a published or draft snapshot of a project graph.
type GraphVersion struct {
	ID            string      `json:"id" db:"id"`
	EnvironmentID string      `json:"environmentId" db:"environment_id"`
	Version       int         `json:"version" db:"version_number"`
	Status        GraphStatus `json:"status" db:"status"`
	NodeCount     int         `json:"nodeCount" db:"node_count"`
	EdgeCount     int         `json:"edgeCount" db:"edge_count"`
	Notes         string      `json:"notes" db:"notes"`
	PublishedAt   *time.Time  `json:"publishedAt,omitempty" db:"published_at"`
	CreatedAt     time.Time   `json:"createdAt" db:"created_at"`
}

// Subgraph is the in-memory structure the Detection Engine works with.
// Loaded from Neo4j or Redis cache.
type Subgraph struct {
	Namespace string
	Nodes     map[string]*ServiceNode  // keyed by node ID
	Edges     map[string][]*DependencyEdge // keyed by from-node ID (adjacency list)
	Reverse   map[string][]*DependencyEdge // keyed by to-node ID (reverse adjacency)
}

// =============================================================================
// Signal ingestion
// =============================================================================

// StatusClass represents the observed HTTP response class or outcome.
type StatusClass string

const (
	StatusClass2xx            StatusClass = "2xx"
	StatusClass3xx            StatusClass = "3xx"
	StatusClass4xx            StatusClass = "4xx"
	StatusClass5xx            StatusClass = "5xx"
	StatusClassTimeout        StatusClass = "timeout"
	StatusClassConnectionErr  StatusClass = "connection_error"
	StatusClassDown           StatusClass = "down"
	StatusClass503            StatusClass = "503"
	StatusClass504            StatusClass = "504"
	StatusClassUnknown        StatusClass = "unknown"
)

// FaultSignal is the canonical normalised signal unit.
// All connectors and webhooks must produce this shape.
type FaultSignal struct {
	TenantID     string      `json:"tenantId"`
	ProjectID    string      `json:"projectId"`
	Environment  string      `json:"environment"`
	Service      string      `json:"service"`
	Endpoint     string      `json:"endpoint,omitempty"`
	StatusClass  StatusClass `json:"statusClass"`
	ErrorRate    float64     `json:"errorRate"`
	LatencyP95   int         `json:"latencyP95"`
	TimeoutCount int         `json:"timeoutCount"`
	Timestamp    time.Time   `json:"timestamp"`
	Source       string      `json:"source"` // prometheus-webhook, manual, grafana
}

// SignalBatch is used when multiple signals arrive together (e.g. alertmanager).
type SignalBatch struct {
	TenantID    string        `json:"tenantId"`
	ProjectID   string        `json:"projectId"`
	Environment string        `json:"environment"`
	Signals     []FaultSignal `json:"signals"`
	Source      string        `json:"source"`
}

// SignalBatchResult is returned after ingestion.
type SignalBatchResult struct {
	SignalBatchID      string `json:"signalBatchId"`
	Received           int    `json:"received"`
	Queued             bool   `json:"queued"`
	AnalysisTriggered  bool   `json:"analysisTriggered"`
}

// =============================================================================
// Detection engine
// =============================================================================

// SuspectLevel indicates how suspicious a node is during traversal.
type SuspectLevel string

const (
	SuspectLevelHealthy  SuspectLevel = "HEALTHY"
	SuspectLevelLow      SuspectLevel = "LOW"
	SuspectLevelMedium   SuspectLevel = "MEDIUM"
	SuspectLevelHigh     SuspectLevel = "HIGH"
	SuspectLevelBlastRad SuspectLevel = "BLAST_RADIUS"
)

// NodeEvaluation is the result of evaluating one node during BFS traversal.
type NodeEvaluation struct {
	NodeID       string       `json:"nodeId"`
	Name         string       `json:"name"`
	SuspectLevel SuspectLevel `json:"suspectLevel"`
	Signal       *FaultSignal `json:"signal,omitempty"`
	Pruned       bool         `json:"pruned"`
}

// TraversalResult is produced by the Detection Engine after BFS completes.
type TraversalResult struct {
	Namespace      string            `json:"namespace"`
	GraphVersion   int               `json:"graphVersion"`
	NodesVisited   int               `json:"nodesVisited"`
	PathsPruned    int               `json:"pathsPruned"`
	TraversalDepth int               `json:"traversalDepth"`
	Evaluations    []NodeEvaluation  `json:"evaluations"`
	SuspectChain   []string          `json:"suspectChain"` // ordered node IDs, root → leaf
	PrunedServices []string          `json:"prunedServices"`
	BlastRadius    []string          `json:"blastRadius"`
}

// =============================================================================
// Root cause analysis
// =============================================================================

// FaultType classifies the detected failure pattern.
type FaultType string

const (
	FaultTypeTimeoutBurst      FaultType = "timeout-burst"
	FaultTypeFiveXXPropagation FaultType = "5xx-propagation"
	FaultTypeAuthFailure       FaultType = "auth-failure-cluster"
	FaultTypeDepUnavailable    FaultType = "dependency-unavailable"
	FaultTypeLatencyAnomaly    FaultType = "latency-anomaly"
)

// EvidenceBreakdown holds the weighted score components for a root cause candidate.
type EvidenceBreakdown struct {
	DirectSignal           float64 `json:"directSignal"`           // weight 0.40
	SharedDownstreamImpact float64 `json:"sharedDownstreamImpact"` // weight 0.25
	BranchIsolation        float64 `json:"branchIsolation"`        // weight 0.20
	LatencyAnomaly         float64 `json:"latencyAnomaly"`         // weight 0.10
	RecentDeployment       float64 `json:"recentDeployment"`       // weight 0.05
}

// CompositeScore computes the final confidence score from evidence components.
func (e EvidenceBreakdown) CompositeScore() float64 {
	return (e.DirectSignal * 0.40) +
		(e.SharedDownstreamImpact * 0.25) +
		(e.BranchIsolation * 0.20) +
		(e.LatencyAnomaly * 0.10) +
		(e.RecentDeployment * 0.05)
}

// ConfidenceLevel returns a human-readable confidence tier.
func (e EvidenceBreakdown) ConfidenceLevel() string {
	score := e.CompositeScore()
	switch {
	case score >= 0.80:
		return "HIGH"
	case score >= 0.50:
		return "MEDIUM"
	default:
		return "LOW"
	}
}

// RCACandidate is a ranked root cause candidate node.
type RCACandidate struct {
	Rank            int               `json:"rank" db:"rank"`
	NodeID          string            `json:"nodeId" db:"node_id"`
	Name            string            `json:"name" db:"name"`
	Confidence      float64           `json:"confidence" db:"confidence_score"`
	FaultType       FaultType         `json:"faultType" db:"fault_type"`
	Evidence        EvidenceBreakdown `json:"evidence" db:"evidence"`
	ImpactedCallers []string          `json:"impactedCallers" db:"-"`
	OperatorConfirmed bool            `json:"operatorConfirmed" db:"operator_confirmed"`
}

// =============================================================================
// Incident
// =============================================================================

// IncidentStatus tracks the resolution state of an incident.
type IncidentStatus string

const (
	IncidentStatusOpen     IncidentStatus = "OPEN"
	IncidentStatusResolved IncidentStatus = "RESOLVED"
	IncidentStatusIgnored  IncidentStatus = "IGNORED"
)

// Incident is the top-level record produced after a full analysis run.
type Incident struct {
	ID              string          `json:"incidentId" db:"id"`
	ProjectID       string          `json:"projectId" db:"project_id"`
	EnvironmentID   string          `json:"environmentId" db:"environment_id"`
	Status          IncidentStatus  `json:"status" db:"status"`
	GraphVersion    int             `json:"graphVersion" db:"graph_version"`
	TraversalStats  TraversalStats  `json:"traversalStats" db:"-"`
	Candidates      []RCACandidate  `json:"rootCauseCandidates" db:"-"`
	PrunedServices  []string        `json:"prunedServices" db:"-"`
	BlastRadius     []string        `json:"blastRadius" db:"-"`
	Recommendations []Recommendation `json:"recommendations" db:"-"`
	Feedback        *OperatorFeedback `json:"feedback,omitempty" db:"-"`
	DetectedAt      time.Time       `json:"detectedAt" db:"detected_at"`
	ResolvedAt      *time.Time      `json:"resolvedAt,omitempty" db:"resolved_at"`
}

// TraversalStats are summary metrics from the detection traversal.
type TraversalStats struct {
	NodesVisited   int `json:"nodesVisited"`
	PathsPruned    int `json:"pathsPruned"`
	SuspectNodes   int `json:"suspectNodes"`
	TraversalDepth int `json:"traversalDepth"`
}

// =============================================================================
// Recommendation
// =============================================================================

// RecommendationCategory classifies the type of action recommended.
type RecommendationCategory string

const (
	CategoryDeployment RecommendationCategory = "deployment"
	CategoryDatabase   RecommendationCategory = "database"
	CategorySaturation RecommendationCategory = "saturation"
	CategoryQueue      RecommendationCategory = "queue"
	CategoryAuth       RecommendationCategory = "auth"
	CategoryNetwork    RecommendationCategory = "network"
)

// Recommendation is a ranked human action to investigate or resolve the incident.
type Recommendation struct {
	Rank     int                    `json:"rank" db:"rank"`
	Title    string                 `json:"title" db:"playbook_title"`
	Category RecommendationCategory `json:"category" db:"category"`
	Steps    []string               `json:"steps" db:"steps"`
	Reason   string                 `json:"reason" db:"reason"`
}

// =============================================================================
// Operator feedback
// =============================================================================

// OperatorFeedback closes the incident loop.
// Stored to improve future RCA ranking.
type OperatorFeedback struct {
	IncidentID           string    `json:"incidentId" db:"incident_id"`
	ConfirmedRootCause   string    `json:"confirmedRootCause" db:"confirmed_root_cause"`
	UsefulRecommendations []int    `json:"usefulRecommendations" db:"useful_recommendations"`
	OperatorNote         string    `json:"operatorNote" db:"operator_note"`
	Resolution           string    `json:"resolution" db:"resolution"`
	ResolvedAt           time.Time `json:"resolvedAt" db:"resolved_at"`
}

// =============================================================================
// Onboarding
// =============================================================================

// OnboardingMode defines how the graph is bootstrapped.
type OnboardingMode string

const (
	OnboardingModeManual    OnboardingMode = "manual"
	OnboardingModeSpec      OnboardingMode = "spec"
	OnboardingModeTelemetry OnboardingMode = "telemetry"
	OnboardingModeHybrid    OnboardingMode = "hybrid"
)

// OnboardingJobStatus tracks async onboarding pipeline state.
type OnboardingJobStatus string

const (
	JobStatusRunning            OnboardingJobStatus = "RUNNING"
	JobStatusValidationPending  OnboardingJobStatus = "VALIDATION_PENDING"
	JobStatusDone               OnboardingJobStatus = "DONE"
	JobStatusFailed             OnboardingJobStatus = "FAILED"
)

// OnboardingJob is the async task created when import is triggered.
type OnboardingJob struct {
	ID          string              `json:"jobId" db:"id"`
	ProjectID   string              `json:"projectId" db:"project_id"`
	Environment string              `json:"environment" db:"environment"`
	Mode        OnboardingMode      `json:"mode" db:"mode"`
	Status      OnboardingJobStatus `json:"status" db:"status"`
	Validation  *ValidationReport   `json:"validation,omitempty" db:"-"`
	StartedAt   time.Time           `json:"startedAt" db:"started_at"`
	CompletedAt *time.Time          `json:"completedAt,omitempty" db:"completed_at"`
}

// ValidationReport holds issues found during graph validation.
type ValidationReport struct {
	OrphanNodes        int               `json:"orphanNodes"`
	LowConfidenceEdges int               `json:"lowConfidenceEdges"`
	MissingEdges       int               `json:"missingEdges"`
	Cycles             int               `json:"cycles"`
	Issues             []ValidationIssue `json:"issues"`
}

// ValidationIssue describes a single graph validation problem.
type ValidationIssue struct {
	Type     string `json:"type"`     // ORPHAN_NODE, LOW_CONFIDENCE_EDGE, CYCLE_DETECTED
	Severity string `json:"severity"` // ERROR, WARNING, INFO
	NodeID   string `json:"nodeId,omitempty"`
	EdgeID   string `json:"edgeId,omitempty"`
	From     string `json:"from,omitempty"`
	To       string `json:"to,omitempty"`
	Message  string `json:"message"`
}

// =============================================================================
// Audit
// =============================================================================

// AuditEvent is an immutable record of a user or system action.
type AuditEvent struct {
	ID        string                 `json:"id" db:"id"`
	TenantID  string                 `json:"tenantId" db:"tenant_id"`
	Actor     string                 `json:"actor" db:"actor"` // user ID or service account
	Action    string                 `json:"action" db:"action"`
	Payload   map[string]interface{} `json:"payload" db:"payload"`
	CreatedAt time.Time              `json:"createdAt" db:"created_at"`
}

// =============================================================================
// HTTP request / response wrappers
// =============================================================================

// AnalyzeRequest is the body for POST /incidents/analyze
type AnalyzeRequest struct {
	Environment  string        `json:"environment"`
	EntryService string        `json:"entryService"`
	TimeWindowMin int          `json:"timeWindowMin"`
	Signals      []FaultSignal `json:"signals"`
}

// CreateProjectRequest is the body for POST /tenants/{id}/projects
type CreateProjectRequest struct {
	Name         string   `json:"name"`
	Slug         string   `json:"slug"`
	Domain       string   `json:"domain"`
	Environments []string `json:"environments"`
}

// PublishGraphRequest is the body for POST /graphs/publish
type PublishGraphRequest struct {
	Environment string `json:"environment"`
	Notes       string `json:"notes"`
}

// ImportRequest is the body for POST /onboarding/import
type ImportRequest struct {
	Environment string        `json:"environment"`
	Mode        OnboardingMode `json:"mode"`
	Sources     ImportSources `json:"sources"`
}

// ImportSources specifies which connectors to activate.
type ImportSources struct {
	OpenAPI     []string `json:"openapi,omitempty"`
	Kubernetes  bool     `json:"kubernetes"`
	Traces      bool     `json:"traces"`
	GatewayLogs bool     `json:"gatewayLogs"`
}

// FeedbackRequest is the body for POST /incidents/{id}/feedback
type FeedbackRequest struct {
	ConfirmedRootCause    string   `json:"confirmedRootCause"`
	OperatorNote          string   `json:"operatorNote"`
	UsefulRecommendations []int    `json:"usefulRecommendations"`
	Resolution            string   `json:"resolution"`
	ResolvedAt            time.Time `json:"resolvedAt"`
}

// APIError is the standard error envelope for all error responses.
type APIError struct {
	Error APIErrorBody `json:"error"`
}

type APIErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	TraceID string `json:"traceId"`
}
