# FaultIQ — Architecture Guide

> All diagrams use Mermaid syntax. Render natively in GitHub, GitLab, Notion, VS Code, or [mermaid.live](https://mermaid.live)

---

## 1. System Context

```mermaid
flowchart TD
    OPS(["SRE / On-Call Engineer"])
    ADMIN(["Tenant Admin"])
    FIQ["FaultIQ Platform"]
    MON["Monitoring Tools
Prometheus · Grafana · Datadog"]
    TRACE["Tracing Systems
Jaeger · Tempo · OpenTelemetry"]
    K8S["Kubernetes / Service Mesh"]
    APIGW["API Gateways
Kong · NGINX"]
    ALERT["Alerting
PagerDuty · OpsGenie"]

    ADMIN -->|"Onboards products
Manages graphs"| FIQ
    OPS -->|"Analyzes incidents
Reviews recommendations"| FIQ
    MON -->|"Fault signals via webhook"| FIQ
    TRACE -->|"Dependency trace data"| FIQ
    K8S -->|"Service discovery"| FIQ
    APIGW -->|"Access log signals"| FIQ
    ALERT -->|"Incident webhook trigger"| FIQ
```

---

## 2. Container Architecture

```mermaid
flowchart TB
    subgraph Client["Client Layer"]
        UI["Console UI — React + TypeScript"]
        EXT["External Webhooks
Prometheus / PagerDuty"]
    end

    subgraph CP["Control Plane"]
        GW["API Gateway — Go :8080"]
        IAM["Keycloak — Auth + RBAC :8180"]
        PROJ["Project Service — Go :8081"]
        ONB["Onboarding Service — Go :8082"]
        GM["Graph Manager — Go :8086"]
    end

    subgraph RP["Runtime Plane"]
        SIG["Signal Ingestion — Go :8083"]
        DET["Detection Engine — Go :8084"]
        RCA["Root Cause Ranker — Go internal"]
        REC["Recommendation Service — Go :8085"]
    end

    subgraph DP["Data Plane"]
        NEO[("Neo4j — Graph Store :7687")]
        PG[("PostgreSQL — Metadata :5432")]
        REDIS[("Redis — Cache :6379")]
        RPDQ[("Redpanda — Events :9092")]
    end

    UI --> GW
    EXT --> SIG
    GW --> IAM
    GW --> PROJ
    GW --> ONB
    GW --> SIG
    GW --> REC
    PROJ --> PG
    ONB --> GM
    ONB --> RPDQ
    GM --> NEO
    GM --> PG
    SIG --> RPDQ
    RPDQ --> DET
    DET --> NEO
    DET --> REDIS
    DET --> RCA
    RCA --> REC
    RCA --> PG
    REC --> PG
```

---

## 3. Tenant and Project Hierarchy

```mermaid
flowchart LR
    T["Tenant: acme-corp"]
    T --> P1["Project: payments-platform"]
    T --> P2["Project: orders-platform"]

    P1 --> E1["dev — NS: payments:dev"]
    P1 --> E2["stage — NS: payments:stage"]
    P1 --> E3["prod — NS: payments:prod"]
    P2 --> E4["prod — NS: orders:prod"]

    E3 --> G1["Graph v3 — PUBLISHED"]
    E3 --> I1["Incidents + Evidence"]
    E3 --> R1["Playbooks + Recommendations"]
```

---

## 4. Dual-Graph Model

```mermaid
flowchart TB
    subgraph DG["Detection Graph — Neo4j"]
        direction LR
        GW2["api-gateway
2xx healthy"]
        PA["payment-api
5xx failing"]
        AS["auth-service
2xx healthy"]
        LS["ledger-service
timeout"]
        SW["settlement-worker
degraded"]
        DB[("postgres-db
slow")]

        GW2 -->|"CALLS successRatio:0.99"| PA
        PA -->|"CALLS successRatio:0.61"| AS
        PA -->|"CALLS successRatio:0.58"| LS
        LS -->|"DEPENDS_ON latency:4800ms"| DB
        SW -->|"CALLS successRatio:0.31"| LS
    end

    subgraph RG["Recommendation Graph — Rule Templates"]
        direction LR
        I1["Issue: timeout burst
on shared dependency"]
        I2["Issue: downstream
5xx propagation"]
        R1["Check recent deployment"]
        R2["Inspect DB connection pool"]
        R3["Compare canary vs stable"]
        R4["Review queue consumer lag"]

        I1 -->|"rank 1"| R1
        I1 -->|"rank 2"| R2
        I1 -->|"rank 3"| R4
        I2 -->|"rank 1"| R1
        I2 -->|"rank 2"| R3
    end

    LS -- "root cause event
ledger-service / timeout-burst" --> I1
    PA -- "symptom event
payment-api / 5xx-propagation" --> I2
```

---

## 5. Product Onboarding — Sequence

```mermaid
sequenceDiagram
    actor Admin as Tenant Admin
    participant UI as Console UI
    participant GW as API Gateway
    participant ONB as Onboarding Service
    participant CONN as Connector Workers
    participant CAT as Service Catalog
    participant GM as Graph Manager
    participant NEO as Neo4j
    participant VAL as Graph Validator
    participant RP as Redpanda

    Admin->>UI: Create project (name, slug, environments)
    UI->>GW: POST /api/v1/tenants/{id}/projects
    GW-->>UI: 201 Created {projectId}

    Admin->>UI: Start onboarding (mode: hybrid, sources)
    UI->>GW: POST /api/v1/projects/{id}/onboarding/import
    GW->>ONB: Dispatch job (tenantId, projectId)
    ONB->>RP: Publish onboarding.started

    ONB->>CONN: Trigger connectors (OpenAPI + k8s + traces)
    CONN->>CONN: Parse specs and manifests
    CONN->>CAT: POST normalized services + dependency edges

    CAT->>CAT: Deduplicate, classify, confidence-score
    CAT->>GM: Build graph draft (nodes + edges)

    GM->>NEO: Write draft graph status=DRAFT
    GM->>VAL: Run validation checks

    VAL->>VAL: Check orphan nodes, missing edges,
low confidence, cycles
    VAL-->>UI: Return validation report

    Admin->>UI: Review graph, fix issues, approve
    UI->>GM: POST /api/v1/projects/{id}/graphs/publish

    GM->>NEO: Set graph status=PUBLISHED version++
    GM->>RP: Publish graph.published
    RP-->>UI: SSE — graph ready
```

---

## 6. Incident Analysis — Request-Response Cycle

```mermaid
sequenceDiagram
    actor MON as Prometheus Webhook
    participant SIG as Signal Ingestion :8083
    participant RP as Redpanda
    participant DET as Detection Engine :8084
    participant NEO as Neo4j :7687
    participant REDIS as Redis :6379
    participant RCA as Root Cause Ranker
    participant REC as Recommendation Service :8085
    participant PG as PostgreSQL
    actor OPS as SRE Operator

    MON->>SIG: POST /signals
{service:ledger-service, statusClass:timeout, errorRate:0.42}
    SIG->>SIG: Validate + enrich with tenantId/projectId
    SIG->>RP: Publish signals.fault.detected

    RP->>DET: Consume event

    DET->>REDIS: GET graph:payments-platform:prod
    alt Cache HIT
        REDIS-->>DET: Return adjacency list
    else Cache MISS
        DET->>NEO: MATCH subgraph WHERE namespace=payments:prod
        NEO-->>DET: Return nodes + edges
        DET->>REDIS: SET graph cache TTL 300s
    end

    DET->>DET: auth-service → 2xx + latency OK → PRUNE
    DET->>DET: payment-api → 5xx → SUSPECT HIGH
    DET->>DET: ledger-service → timeout → SUSPECT HIGH
    DET->>DET: settlement-worker → degraded → BLAST RADIUS
    DET->>DET: BFS queue empty — score suspects

    DET->>RCA: suspect path + evidence bundle

    RCA->>RCA: ledger-service score:
directSignal 0.40 + sharedImpact 0.25
+ branchIsolation 0.20 + latencyDelta 0.10 = 0.91
    RCA->>RCA: payment-api score: 0.42 symptom node
    RCA->>PG: INSERT incident + rca_candidates + evidence

    RCA->>REC: rootCause:ledger-service faultType:timeout-burst
    REC->>PG: SELECT playbooks WHERE fault_type=timeout-burst
    PG-->>REC: 4 ranked playbooks
    REC->>PG: INSERT recommendation_run

    OPS->>GW: GET /api/v1/incidents/{incidentId}
    GW-->>OPS: 200 OK root cause + path + recommendations
```

---

## 7. Detection Engine — BFS State Machine

```mermaid
stateDiagram-v2
    [*] --> LoadSubgraph: Signal event consumed

    LoadSubgraph --> InitTraversal: Subgraph loaded
    InitTraversal --> VisitNode: BFS queue seeded

    VisitNode --> EvaluateSignal: Dequeue next node

    EvaluateSignal --> MarkHealthy: 2xx AND latency OK
    EvaluateSignal --> ClassifyFourXX: 4xx response
    EvaluateSignal --> MarkSuspectHigh: 5xx OR timeout
    EvaluateSignal --> MarkSuspectLow: unknown

    MarkHealthy --> PruneBranch: Skip downstream neighbors
    ClassifyFourXX --> ApplyProjectRules: Load per-project config
    ApplyProjectRules --> MarkHealthy: Rule = client_error
    ApplyProjectRules --> MarkSuspectHigh: Rule = auth_cascade
    ApplyProjectRules --> MarkSuspectLow: Rule = unconfigured

    MarkSuspectHigh --> ExpandNeighbors: Add to BFS queue
    MarkSuspectLow --> ExpandNeighbors

    PruneBranch --> CheckQueue
    ExpandNeighbors --> CheckQueue

    CheckQueue --> VisitNode: Queue not empty
    CheckQueue --> ScoreNodes: Queue empty

    ScoreNodes --> RankCandidates: Weighted confidence per node
    RankCandidates --> EmitCandidates: Sort by score DESC
    EmitCandidates --> [*]: Send to RCA Ranker
```

---

## 8. Signal Classification Decision Tree

```mermaid
flowchart TD
    SIG(["Incoming Signal"])
    SIG --> Q1{"statusClass?"}

    Q1 -->|"2xx"| Q2{"latencyP95 < SLA?"}
    Q2 -->|"Yes"| HEALTHY(["HEALTHY — Prune branch
confidence 0.0"])
    Q2 -->|"No"| LAT(["LATENCY ANOMALY
Suspect LOW — confidence 0.2"])

    Q1 -->|"4xx"| Q3{"Project 4xx
rule configured?"}
    Q3 -->|"client_error"| HEALTHY
    Q3 -->|"auth_cascade"| SM(["SUSPECT MED — confidence 0.5"])
    Q3 -->|"rate_limit"| SM
    Q3 -->|"not configured"| SL(["SUSPECT LOW — confidence 0.3"])

    Q1 -->|"5xx"| SH(["SUSPECT HIGH — confidence 0.8"])
    Q1 -->|"timeout"| SH
    Q1 -->|"unknown"| SL

    SH --> EXP(["Expand BFS to all neighbors"])
    SM --> EXP
    SL --> EXP
    LAT --> EXP
    HEALTHY --> PRUNE(["Prune subtree — skip neighbors"])
```

---

## 9. Root Cause Scoring Model

```
Score = 0.40 x DirectSignal
      + 0.25 x SharedDownstreamImpact
      + 0.20 x BranchIsolation
      + 0.10 x LatencyAnomaly
      + 0.05 x RecentDeploymentFlag
```

```mermaid
flowchart LR
    N(["Suspect Node"])

    N --> S1["Direct 5xx/timeout
weight 0.40"]
    N --> S2["Multiple callers fail
through this node
weight 0.25"]
    N --> S3["Neighbors healthy
branch isolated
weight 0.20"]
    N --> S4["Latency > 2x baseline
weight 0.10"]
    N --> S5["Deploy in last 30min
weight 0.05"]

    S1 --> SCORE(["Composite Score
0.0 to 1.0"])
    S2 --> SCORE
    S3 --> SCORE
    S4 --> SCORE
    S5 --> SCORE

    SCORE -->|">= 0.80"| HIGH(["HIGH CONFIDENCE
Top root cause candidate"])
    SCORE -->|"0.50 to 0.79"| MED(["MEDIUM
Secondary candidate"])
    SCORE -->|"< 0.50"| LOW(["LOW
Blast radius only"])
```

---

## 10. Operator Workflow — End-to-End

```mermaid
flowchart LR
    A(["Alert fires
PagerDuty / Prometheus"])
    B["Signal ingested
normalised"]
    C["Graph loaded
for product"]
    D["Healthy paths
pruned"]
    E["Suspect chain
identified"]
    F["Root cause
candidates ranked"]
    G["Recommendations
generated"]
    H["SRE reviews
in console"]
    I{"Agrees with
root cause?"}
    J["Fix in
source system"]
    K["Override candidate
mark correct node"]
    L["Verify health
restored"]
    M["Close incident
submit feedback"]
    N["Feedback stored
for future ranking"]

    A --> B --> C --> D --> E --> F --> G --> H --> I
    I -->|"Yes"| J
    I -->|"No"| K
    J --> L
    K --> L
    L --> M --> N
```

---

## 11. Data Model — Entity Relationships

```mermaid
erDiagram
    TENANT {
        uuid id PK
        string name
        string slug
        timestamp created_at
    }
    PROJECT {
        uuid id PK
        uuid tenant_id FK
        string name
        string slug
        string domain
    }
    ENVIRONMENT {
        uuid id PK
        uuid project_id FK
        string name
        string graph_namespace
    }
    GRAPH_VERSION {
        uuid id PK
        uuid environment_id FK
        int version_number
        string status
        timestamp published_at
    }
    SERVICE_NODE {
        uuid id PK
        uuid environment_id FK
        string name
        string type
    }
    DEPENDENCY_EDGE {
        uuid id PK
        uuid from_node_id FK
        uuid to_node_id FK
        string edge_type
        float confidence
        timestamp last_observed
    }
    INCIDENT {
        uuid id PK
        uuid project_id FK
        uuid environment_id FK
        string status
        timestamp detected_at
    }
    RCA_CANDIDATE {
        uuid id PK
        uuid incident_id FK
        uuid node_id FK
        float confidence_score
        jsonb evidence
        bool operator_confirmed
    }
    RECOMMENDATION {
        uuid id PK
        uuid incident_id FK
        int rank
        string playbook_title
        jsonb steps
    }
    AUDIT_EVENT {
        uuid id PK
        uuid tenant_id FK
        string actor
        string action
        timestamp created_at
    }

    TENANT ||--o{ PROJECT : "owns"
    PROJECT ||--o{ ENVIRONMENT : "has"
    ENVIRONMENT ||--o{ GRAPH_VERSION : "versioned by"
    ENVIRONMENT ||--o{ SERVICE_NODE : "contains"
    SERVICE_NODE ||--o{ DEPENDENCY_EDGE : "from"
    SERVICE_NODE ||--o{ DEPENDENCY_EDGE : "to"
    ENVIRONMENT ||--o{ INCIDENT : "tracks"
    INCIDENT ||--o{ RCA_CANDIDATE : "produces"
    INCIDENT ||--o{ RECOMMENDATION : "generates"
    TENANT ||--o{ AUDIT_EVENT : "logs"
```

---

## 12. Service Communication Map

```mermaid
flowchart TB
    subgraph SYNC["Synchronous REST"]
        GW3["API Gateway"]
        GW3 -->|"auth check"| KC["Keycloak"]
        GW3 -->|"project ops"| PS["Project Service"]
        GW3 -->|"onboarding"| OS["Onboarding Service"]
        GW3 -->|"graph ops"| GMS["Graph Manager"]
        GW3 -->|"signal submit"| SI["Signal Ingestion"]
        GW3 -->|"get incident"| RS["Recommendation Service"]
    end

    subgraph ASYNC["Asynchronous Redpanda Topics"]
        SI2["Signal Ingestion"] -->|"signals.fault.detected"| DE["Detection Engine"]
        OS2["Onboarding Service"] -->|"onboarding.started"| CW["Connector Workers"]
        CW -->|"catalog.updated"| GMS2["Graph Manager"]
        GMS2 -->|"graph.published"| SUB["All Subscribers"]
        DE -->|"rca.candidates.ready"| RS2["Recommendation Service"]
        RS2 -->|"recommendations.ready"| UISSE["Console UI SSE"]
    end
```

---

## 13. Kubernetes Deployment

```mermaid
flowchart TB
    subgraph ING["Ingress"]
        NGINX["Nginx Ingress"]
    end

    subgraph APP["Namespace: faultiq-app"]
        GW4["api-gateway"]
        UIPOD["console-web"]
        PROJPOD["project-service"]
        ONBPOD["onboarding-service"]
        SIPOD["signal-ingestion"]
        DETPOD["detection-engine x2"]
        RECPOD["recommendation-service"]
        GMPOD["graph-manager"]
    end

    subgraph DATA["Namespace: faultiq-data"]
        NEOPOD[("Neo4j StatefulSet")]
        PGPOD[("PostgreSQL StatefulSet")]
        RPOD[("Redpanda StatefulSet")]
        REDISPOD[("Redis StatefulSet")]
    end

    subgraph AUTH["Namespace: faultiq-auth"]
        KCPOD["Keycloak"]
    end

    NGINX -->|"/api"| GW4
    NGINX -->|"/console"| UIPOD
    NGINX -->|"/auth"| KCPOD

    GW4 --> PROJPOD
    GW4 --> ONBPOD
    GW4 --> SIPOD
    GW4 --> RECPOD

    SIPOD --> RPOD
    RPOD --> DETPOD
    DETPOD --> NEOPOD
    DETPOD --> REDISPOD
    DETPOD --> RECPOD
    RECPOD --> PGPOD
    ONBPOD --> GMPOD
    GMPOD --> NEOPOD
    PROJPOD --> PGPOD
    KCPOD --> PGPOD
```

---

## 14. Security Architecture

```mermaid
flowchart LR
    USER(["User or Service Account"])
    GW5["API Gateway"]
    KC2["Keycloak OIDC"]
    SVC["Downstream Services"]
    DB2[("PostgreSQL + Neo4j")]
    AUD2["Audit Store"]

    USER -->|"Bearer JWT"| GW5
    GW5 -->|"Introspect token"| KC2
    KC2 -->|"Claims: tenantId
projectId, roles"| GW5
    GW5 -->|"X-Tenant-ID
X-Project-ID
X-User-Roles"| SVC
    SVC -->|"All queries scoped
by tenant + project"| DB2
    SVC -->|"All actions logged"| AUD2
```

### RBAC Roles

| Role | Scope | Permissions |
|---|---|---|
| `tenant:admin` | Tenant-wide | Create projects, manage members, view all |
| `project:admin` | Project | Onboard product, manage graph, view incidents |
| `project:analyst` | Project | Analyze incidents, view graph, submit feedback |
| `project:viewer` | Project | Read-only incidents and recommendations |
| `signal:publisher` | Project | Ingest signals only — monitoring accounts |

---

## 15. Architecture Decision Records

### ADR-001 — Single Language: Go

**Status**: Accepted
**Decision**: Go for all backend services.
**Why**: 15-day deadline. All-Go saves 9.5 dev-days vs polyglot (Java + Python + Go). Single toolchain. Shared types package. Goroutines handle all concurrency natively.
**Tradeoff**: Python connector ecosystem skipped in MVP. Real connectors added post-demo.

---

### ADR-002 — Graph DB: Neo4j Community (GPL-3)

**Status**: Accepted
**Decision**: Neo4j Community over Memgraph or FalkorDB.
**Why**: Disk-native — handles larger-than-RAM graphs. Deep hop traversal (up to 6 hops) is stable. Memgraph has documented working-memory explosion at deep hops. FalkorDB benchmarks are vendor-published.
**Tradeoff**: No clustering in Community. Single node until paid tier or ArangoDB migration.

---

### ADR-003 — Messaging: Redpanda

**Status**: Accepted
**Decision**: Redpanda over Apache Kafka.
**Why**: Sub-1ms p99 latency. No ZooKeeper. Single binary. Full Kafka API compatibility — zero client code changes.
**Tradeoff**: Smaller community than Kafka at extreme throughput.

---

### ADR-004 — Auth: Keycloak

**Status**: Accepted
**Decision**: Keycloak (Apache 2.0) self-hosted via Helm.
**Why**: Zero per-user cost. Full control over realms and RBAC. Official K8s Helm chart. PostgreSQL backend — same data plane.
**Tradeoff**: Operator manages Keycloak infra vs zero-burden Okta SaaS.

---

### ADR-005 — MVP: Detection + Recommendation Only

**Status**: Accepted
**Decision**: No auto-remediation in MVP.
**Why**: Safer rollout. Operators retain control. Cleaner patent story around recommendation as a distinct decoupled mechanism.
**Tradeoff**: Full loop automation deferred to post-MVP.
