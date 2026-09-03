/**
 * All FaultIQ domain types — sourced from docs/api.md + docs/types.go
 */

// ─── Auth / User ──────────────────────────────────────────────────────────
export interface User {
  id: string
  email: string
  username: string
  firstName: string
  lastName: string
  roles: string[]
  permissions: string[]
}

// ─── Tenant ───────────────────────────────────────────────────────────────
export interface Tenant {
  id: string
  name: string
  slug: string
  plan: "free" | "pro" | "enterprise"
  projectCount?: number
  createdAt: string
}

export interface CreateTenantInput {
  name: string
  slug: string
  plan: Tenant["plan"]
}

// ─── Project / Environment ────────────────────────────────────────────────
export interface Environment {
  id: string
  name: string
  namespace: string
}

export type GraphStatus = "DRAFT" | "PUBLISHED" | "ARCHIVED"

export interface Project {
  id: string
  tenantId: string
  name: string
  slug: string
  domain: string
  environments: Environment[]
  graphStatus?: GraphStatus
  incidentCount?: number
  createdAt: string
  latestGraph?: {
    version: number
    status: GraphStatus
    nodeCount: number
    edgeCount: number
    publishedAt: string
  }
}

export interface ProjectSettings {
  projectId: string
  services: ServiceRepo[]
  serviceMapYaml?: string   // raw YAML for display/edit
  notificationWebhook?: string
}

export interface CreateProjectInput {
  name: string
  slug: string
  domain?: string
  environments: string[]
  services?: Array<{ name: string; dependencies: string[] }>
}

export interface ProjectListItem {
  id: string
  name: string
  slug: string
  graphStatus: GraphStatus
  incidentCount: number
  environments?: Environment[]
  createdAt?: string
}

// ─── Graph ────────────────────────────────────────────────────────────────
export type NodeType = "SERVICE" | "DATABASE" | "QUEUE" | "GATEWAY" | "EXTERNAL"
export type EdgeType = "CALLS" | "READS" | "WRITES" | "SUBSCRIBES" | "PUBLISHES"

export interface ServiceNode {
  id: string
  name: string
  type: NodeType
  tags: string[]
  runtimeState?: {
    statusClass: string
    errorRate: number
    latencyP95: number
  }
}

export interface ServiceEdge {
  id: string
  from: string
  to: string
  type: EdgeType
  successRatio?: number
  confidence: number
}

export interface Graph {
  version: number
  status: GraphStatus
  namespace: string
  publishedAt: string
  nodes: ServiceNode[]
  edges: ServiceEdge[]
}

export interface GraphVersion {
  version: number
  status: GraphStatus
  nodeCount: number
  publishedAt: string
}

// ─── Onboarding ───────────────────────────────────────────────────────────
export type JobStatus =
  | "RUNNING"
  | "VALIDATION_PENDING"
  | "COMPLETE"
  | "FAILED"

export interface OnboardingJob {
  jobId: string
  projectId: string
  environment: string
  status: JobStatus
  startedAt: string
  progress?: {
    connectors: string
    catalogBuild: string
    graphBuild: string
    validation: string
  }
  validation?: {
    orphanNodes: number
    lowConfidenceEdges: number
    missingEdges: number
    cycles: number
  }
}

export interface StartOnboardingInput {
  environment: string
  mode: "hybrid" | "manual" | "openapi" | "kubernetes"
  sources: {
    openapi?: string[]
    kubernetes?: boolean
    traces?: boolean
    gatewayLogs?: boolean
  }
}

// ─── Signals ─────────────────────────────────────────────────────────────
export interface Signal {
  service: string
  endpoint?: string
  statusClass: string
  errorRate: number
  latencyP95: number
  timeoutCount?: number
  timestamp: string
  source?: string
}

export interface SignalBatch {
  tenantId: string
  projectId: string
  environment: string
  signals: Signal[]
  source: string
}

export interface SignalBatchResponse {
  signalBatchId: string
  received: number
  queued: boolean
  analysisTriggered: boolean
}

// ─── Code Awareness ───────────────────────────────────────────────────────

export interface ServiceRepo {
  serviceId: string
  repoUrl: string       // https://github.com/org/repo
  branch?: string       // default: main
  codePath?: string     // for monolith: subdirectory e.g. "src/payments/"
  signalTag?: string    // for monolith: how signals identify this module
  lastIndexed?: string  // ISO timestamp of last code-indexer run
}

export interface CodeCommit {
  hash: string
  message: string
  author: string
  timestamp: string
  changedFiles: string[]
  serviceId?: string
}

export interface CodeContext {
  serviceId: string
  serviceName: string
  repoUrl?: string
  recentCommits: CodeCommit[]
  suspectFunctions: string[]
  summary: string  // AI-friendly one-liner
}

// ─── SOP (Sequenced Faults for Automation) ────────────────────────────────

export type IncidentPhase =
  | "DETECTING"
  | "NARROWING"
  | "CONFIRMED"
  | "TRIAGING"
  | "FIXING"
  | "VERIFYING"
  | "RESOLVED"

export type SOPStepStatus = "PENDING" | "ACTIVE" | "DONE" | "FAILED"

export interface SOPStep {
  order: number
  title: string
  action: string
  expectedSignal: string
  autoVerify: boolean
  status: SOPStepStatus
}

export interface SOPPlaybook {
  incidentId: string
  phase: IncidentPhase
  service: string
  steps: SOPStep[]
}

// ─── Incidents ────────────────────────────────────────────────────────────
export type IncidentStatus = "OPEN" | "ACKNOWLEDGED" | "RESOLVED"

export interface RootCauseCandidate {
  rank: number
  nodeId: string
  name: string
  confidence: number
  faultType: string
  evidence: Record<string, number>
  impactedCallers: string[]
}

export interface Recommendation {
  rank: number
  title: string
  category: string
  steps: string[]
  reason: string
}

export interface TriggerSignal {
  service: string
  nodeId: string
  confidence: number
  faultType: string
}

export interface Incident {
  incidentId: string
  projectId: string
  service?: string
  environment: string
  detectedAt: string
  graphVersion: number
  status: IncidentStatus
  phase?: IncidentPhase
  rootCauseCandidates: RootCauseCandidate[]
  prunedServices: string[]
  blastRadius: string[]
  recommendations: Recommendation[]
  triggerSignals?: TriggerSignal[]
  feedback?: IncidentFeedback | null
  resolvedAt?: string | null
  sopSteps?: SOPStep[]
  assignedTo?: string
  assignedAt?: string
}

export interface IncidentListItem {
  id: string
  projectId: string
  service: string
  status: IncidentStatus
  phase?: IncidentPhase
  environment?: string
  detectedAt: string | number
  resolvedAt?: string | null
  rootCauseCandidate?: string
  confidence: number
  evidenceCount: number
  assignedTo?: string
  assignedAt?: string
}

export interface IncidentFeedback {
  confirmedRootCause: string
  operatorNote?: string
  usefulRecommendations?: number[]
  resolution?: string
  resolvedAt: string
}

export interface AnalyzeIncidentInput {
  environment: string
  entryService: string
  timeWindowMin: number
  signals: Pick<Signal, "service" | "statusClass" | "errorRate" | "latencyP95">[]
}

// ─── Deployments ─────────────────────────────────────────────────────────

export type DeploymentStatus = "deploying" | "healthy" | "degraded" | "rolled_back"

export interface Deployment {
  id: string
  serviceId: string
  serviceName: string
  version: string
  commitHash: string
  deployedBy: string
  status: DeploymentStatus
  environment: string
  deployedAt: string
  healthStatus?: string
  notes?: string
}

export interface CreateDeploymentInput {
  serviceId: string
  serviceName?: string
  version: string
  commitHash: string
  deployedBy?: string
  projectId: string
  tenantId?: string
  environment?: string
  notes?: string
}

export interface RequestPathNode {
  serviceId: string
  serviceName: string
  serviceType: string
  depth: number
}

export interface RequestPath {
  serviceId: string
  paths: RequestPathNode[][]
  pathCount: number
  recentDeploys: Array<{
    version: string
    commitHash: string
    deployedBy: string
    status: string
    deployedAt: string
  }>
  summary: string
}

// ─── Health ───────────────────────────────────────────────────────────────
export interface PlatformHealth {
  status: "ok" | "degraded" | "down"
  services: Record<string, "ok" | "degraded" | "down">
}

// ─── Pagination ───────────────────────────────────────────────────────────
export interface PaginatedResponse<T> {
  items: T[]
  total: number
  limit: number
  offset: number
}

export interface APIError {
  error: {
    code: string
    message: string
    traceId?: string
  }
}

// ─── Audit Trail ──────────────────────────────────────────────────────────
export interface AuditEvent {
  id: string
  action: string        // phase_change | step_done | step_failed | auto_resolved
  actor: string         // "system" or user email/UUID
  phaseFrom?: string
  phaseTo?: string
  stepOrder?: number
  createdAt: string
  signalSnapshot?: Record<string, unknown>
}

// ─── Code Index Status ────────────────────────────────────────────────────
export interface CodeIndexStatus {
  serviceId: string
  repoUrl?: string
  lastCommitHash?: string
  lastIndexedAt?: string
}

// ─── Signal History ───────────────────────────────────────────────────────
export interface SignalHistoryData {
  errorRateTimeline: { time: string; errorRate: number }[]
  avgLatency: number
  faultCount: number
  uptime: number
  sloTarget?: number
  sloStatus?: string
  p95LatencyMs?: number
  projectId?: string
  serviceId?: string
  hoursQueried?: number
}

// ─── Maintenance Windows ──────────────────────────────────────────────────
export interface MaintenanceWindow {
  id: string
  projectId: string
  serviceId?: string
  reason?: string
  createdBy: string
  startsAt: string
  endsAt: string
}
