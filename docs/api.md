# FaultIQ — API Reference

> Base URL: `http://localhost:8080`
> All endpoints require `Authorization: Bearer <token>` except `/health` and `/auth/*`
> All request/response bodies are `application/json`
> All endpoints are tenant-scoped — token claims carry `tenantId`

---

## Error Response Format

All errors follow this shape:

```json
{
  "error": {
    "code": "PROJECT_NOT_FOUND",
    "message": "Project with id p_xyz does not exist",
    "traceId": "01HZ9MXKQ2B3T4V5W6Y7Z8A9BC"
  }
}
```

| HTTP Code | Meaning |
|---|---|
| 400 | Validation error — bad request body |
| 401 | Missing or invalid JWT |
| 403 | Insufficient role for this resource |
| 404 | Resource not found |
| 409 | Conflict — duplicate slug or resource already exists |
| 422 | Semantic error — valid JSON but business rule violated |
| 500 | Internal server error |

---

## Auth Endpoints

### Get Token (Keycloak)

```http
POST /auth/realms/faultiq/protocol/openid-connect/token
Content-Type: application/x-www-form-urlencoded

grant_type=password
&client_id=faultiq-api
&username=admin@acme.com
&password=secret
```

**Response 200**
```json
{
  "access_token": "eyJhbGci...",
  "expires_in": 3600,
  "token_type": "Bearer",
  "refresh_token": "eyJhbGci..."
}
```

---

## Tenant Endpoints

### Create Tenant

```http
POST /api/v1/tenants
Authorization: Bearer <superadmin-token>
```

**Request**
```json
{
  "name": "Acme Corp",
  "slug": "acme-corp",
  "plan": "pro"
}
```

**Response 201**
```json
{
  "id": "t_01HZ9MXK",
  "name": "Acme Corp",
  "slug": "acme-corp",
  "plan": "pro",
  "createdAt": "2026-05-14T00:00:00Z"
}
```

---

### Get Tenant

```http
GET /api/v1/tenants/{tenantId}
Authorization: Bearer <token>
```

**Response 200**
```json
{
  "id": "t_01HZ9MXK",
  "name": "Acme Corp",
  "slug": "acme-corp",
  "plan": "pro",
  "projectCount": 3,
  "createdAt": "2026-05-14T00:00:00Z"
}
```

---

## Project Endpoints

### Create Project

```http
POST /api/v1/tenants/{tenantId}/projects
Authorization: Bearer <token>
```

**Request**
```json
{
  "name": "Payments Platform",
  "slug": "payments-platform",
  "domain": "payments",
  "environments": ["dev", "stage", "prod"]
}
```

**Response 201**
```json
{
  "id": "p_02KA1NYL",
  "tenantId": "t_01HZ9MXK",
  "name": "Payments Platform",
  "slug": "payments-platform",
  "domain": "payments",
  "environments": [
    { "id": "e_dev",   "name": "dev",   "namespace": "payments-platform:dev" },
    { "id": "e_stg",   "name": "stage", "namespace": "payments-platform:stage" },
    { "id": "e_prod",  "name": "prod",  "namespace": "payments-platform:prod" }
  ],
  "createdAt": "2026-05-14T00:00:00Z"
}
```

---

### List Projects

```http
GET /api/v1/tenants/{tenantId}/projects
Authorization: Bearer <token>
```

**Response 200**
```json
{
  "projects": [
    {
      "id": "p_02KA1NYL",
      "name": "Payments Platform",
      "slug": "payments-platform",
      "graphStatus": "PUBLISHED",
      "incidentCount": 4
    },
    {
      "id": "p_03KB2OZM",
      "name": "Orders Platform",
      "slug": "orders-platform",
      "graphStatus": "DRAFT",
      "incidentCount": 0
    }
  ],
  "total": 2
}
```

---

### Get Project

```http
GET /api/v1/projects/{projectId}
Authorization: Bearer <token>
```

**Response 200**
```json
{
  "id": "p_02KA1NYL",
  "tenantId": "t_01HZ9MXK",
  "name": "Payments Platform",
  "slug": "payments-platform",
  "domain": "payments",
  "environments": [
    { "id": "e_prod", "name": "prod", "namespace": "payments-platform:prod" }
  ],
  "latestGraph": {
    "version": 3,
    "status": "PUBLISHED",
    "nodeCount": 12,
    "edgeCount": 18,
    "publishedAt": "2026-05-13T22:00:00Z"
  }
}
```

---

## Onboarding Endpoints

### Start Onboarding Import

Triggers graph bootstrap for a project environment.

```http
POST /api/v1/projects/{projectId}/onboarding/import
Authorization: Bearer <token>
```

**Request**
```json
{
  "environment": "prod",
  "mode": "hybrid",
  "sources": {
    "openapi": ["s3://specs/payment-api.yaml", "s3://specs/ledger-api.yaml"],
    "kubernetes": true,
    "traces": true,
    "gatewayLogs": false
  }
}
```

**Response 202 Accepted**
```json
{
  "jobId": "job_04LC3PAM",
  "projectId": "p_02KA1NYL",
  "environment": "prod",
  "status": "RUNNING",
  "startedAt": "2026-05-14T00:01:00Z"
}
```

---

### Get Onboarding Job Status

```http
GET /api/v1/projects/{projectId}/onboarding/jobs/{jobId}
Authorization: Bearer <token>
```

**Response 200**
```json
{
  "jobId": "job_04LC3PAM",
  "status": "VALIDATION_PENDING",
  "progress": {
    "connectors": "done",
    "catalogBuild": "done",
    "graphBuild": "done",
    "validation": "pending"
  },
  "validation": {
    "orphanNodes": 1,
    "lowConfidenceEdges": 3,
    "missingEdges": 0,
    "cycles": 0
  }
}
```

---

## Graph Endpoints

### Get Current Graph

Returns the published graph for a project environment.

```http
GET /api/v1/projects/{projectId}/graphs/current?environment=prod
Authorization: Bearer <token>
```

**Response 200**
```json
{
  "version": 3,
  "status": "PUBLISHED",
  "namespace": "payments-platform:prod",
  "publishedAt": "2026-05-13T22:00:00Z",
  "nodes": [
    {
      "id": "svc_api_gateway",
      "name": "api-gateway",
      "type": "SERVICE",
      "tags": ["entry-point", "public"],
      "runtimeState": {
        "statusClass": "2xx",
        "errorRate": 0.001,
        "latencyP95": 42
      }
    },
    {
      "id": "svc_ledger",
      "name": "ledger-service",
      "type": "SERVICE",
      "tags": ["financial", "core"],
      "runtimeState": {
        "statusClass": "timeout",
        "errorRate": 0.42,
        "latencyP95": 4800
      }
    }
  ],
  "edges": [
    {
      "id": "edge_01",
      "from": "svc_api_gateway",
      "to": "svc_payment_api",
      "type": "CALLS",
      "successRatio": 0.99,
      "confidence": 0.95
    },
    {
      "id": "edge_02",
      "from": "svc_payment_api",
      "to": "svc_ledger",
      "type": "CALLS",
      "successRatio": 0.58,
      "confidence": 0.95
    }
  ]
}
```

---

### Add Node (Manual Onboarding)

```http
PUT /api/v1/projects/{projectId}/graphs/nodes
Authorization: Bearer <token>
```

**Request**
```json
{
  "environment": "prod",
  "node": {
    "id": "svc_notification",
    "name": "notification-service",
    "type": "SERVICE",
    "tags": ["async", "non-critical"]
  }
}
```

**Response 200**
```json
{
  "id": "svc_notification",
  "name": "notification-service",
  "type": "SERVICE",
  "createdAt": "2026-05-14T00:05:00Z"
}
```

---

### Add Edge (Manual Onboarding)

```http
PUT /api/v1/projects/{projectId}/graphs/edges
Authorization: Bearer <token>
```

**Request**
```json
{
  "environment": "prod",
  "edge": {
    "from": "svc_payment_api",
    "to": "svc_notification",
    "type": "CALLS",
    "confidence": 0.80
  }
}
```

**Response 200**
```json
{
  "id": "edge_09",
  "from": "svc_payment_api",
  "to": "svc_notification",
  "type": "CALLS",
  "confidence": 0.80,
  "createdAt": "2026-05-14T00:05:10Z"
}
```

---

### Publish Graph Version

Locks the current draft and makes it active for incident analysis.

```http
POST /api/v1/projects/{projectId}/graphs/publish
Authorization: Bearer <token>
```

**Request**
```json
{
  "environment": "prod",
  "notes": "Added notification-service dependency"
}
```

**Response 200**
```json
{
  "version": 4,
  "status": "PUBLISHED",
  "namespace": "payments-platform:prod",
  "nodeCount": 13,
  "edgeCount": 19,
  "publishedAt": "2026-05-14T00:06:00Z"
}
```

---

### List Graph Versions

```http
GET /api/v1/projects/{projectId}/graphs/versions?environment=prod
Authorization: Bearer <token>
```

**Response 200**
```json
{
  "versions": [
    { "version": 4, "status": "PUBLISHED", "nodeCount": 13, "publishedAt": "2026-05-14T00:06:00Z" },
    { "version": 3, "status": "ARCHIVED",  "nodeCount": 12, "publishedAt": "2026-05-13T22:00:00Z" },
    { "version": 2, "status": "ARCHIVED",  "nodeCount": 10, "publishedAt": "2026-05-12T10:00:00Z" }
  ]
}
```

---

## Signal Ingestion Endpoints

### Submit Signal (Webhook / Manual)

```http
POST /api/v1/signals
Authorization: Bearer <signal-publisher-token>
```

**Request — single signal**
```json
{
  "tenantId": "t_01HZ9MXK",
  "projectId": "p_02KA1NYL",
  "environment": "prod",
  "service": "ledger-service",
  "endpoint": "/entries",
  "statusClass": "timeout",
  "errorRate": 0.42,
  "latencyP95": 4800,
  "timeoutCount": 18,
  "timestamp": "2026-05-14T00:10:00Z",
  "source": "prometheus-webhook"
}
```

**Request — batch signals**
```json
{
  "tenantId": "t_01HZ9MXK",
  "projectId": "p_02KA1NYL",
  "environment": "prod",
  "signals": [
    {
      "service": "ledger-service",
      "statusClass": "timeout",
      "errorRate": 0.42,
      "latencyP95": 4800,
      "timestamp": "2026-05-14T00:10:00Z"
    },
    {
      "service": "payment-api",
      "statusClass": "5xx",
      "errorRate": 0.18,
      "latencyP95": 1200,
      "timestamp": "2026-05-14T00:10:00Z"
    },
    {
      "service": "auth-service",
      "statusClass": "2xx",
      "errorRate": 0.001,
      "latencyP95": 38,
      "timestamp": "2026-05-14T00:10:00Z"
    }
  ],
  "source": "prometheus-webhook"
}
```

**Response 202 Accepted**
```json
{
  "signalBatchId": "sig_05MD4QBN",
  "received": 3,
  "queued": true,
  "analysisTriggered": true
}
```

---

## Incident Analysis Endpoints

### Trigger Manual Analysis

Use when you want to analyse a specific set of signals without waiting for a webhook.

```http
POST /api/v1/projects/{projectId}/incidents/analyze
Authorization: Bearer <token>
```

**Request**
```json
{
  "environment": "prod",
  "entryService": "api-gateway",
  "timeWindowMin": 5,
  "signals": [
    { "service": "ledger-service", "statusClass": "timeout",  "errorRate": 0.42, "latencyP95": 4800 },
    { "service": "payment-api",    "statusClass": "5xx",      "errorRate": 0.18, "latencyP95": 1200 },
    { "service": "auth-service",   "statusClass": "2xx",      "errorRate": 0.001,"latencyP95": 38 }
  ]
}
```

**Response 200**
```json
{
  "incidentId": "inc_06NE5RCO",
  "projectId": "p_02KA1NYL",
  "environment": "prod",
  "detectedAt": "2026-05-14T00:10:02Z",
  "graphVersion": 4,
  "traversalStats": {
    "nodesVisited": 12,
    "pathsPruned": 6,
    "suspectNodes": 3,
    "traversalDepth": 4
  },
  "rootCauseCandidates": [
    {
      "rank": 1,
      "nodeId": "svc_ledger",
      "name": "ledger-service",
      "confidence": 0.91,
      "faultType": "timeout-burst",
      "evidence": {
        "directSignal": 0.40,
        "sharedDownstreamImpact": 0.25,
        "branchIsolation": 0.20,
        "latencyAnomaly": 0.06,
        "recentDeployment": 0.00
      },
      "impactedCallers": ["payment-api", "settlement-worker"]
    },
    {
      "rank": 2,
      "nodeId": "svc_payment_api",
      "name": "payment-api",
      "confidence": 0.42,
      "faultType": "5xx-propagation",
      "evidence": {
        "directSignal": 0.20,
        "sharedDownstreamImpact": 0.10,
        "branchIsolation": 0.08,
        "latencyAnomaly": 0.04,
        "recentDeployment": 0.00
      },
      "impactedCallers": ["api-gateway"]
    }
  ],
  "prunedServices": ["auth-service"],
  "blastRadius": ["payment-api", "settlement-worker"],
  "recommendations": [
    {
      "rank": 1,
      "title": "Check recent deployment for ledger-service",
      "category": "deployment",
      "steps": [
        "Run: kubectl rollout history deployment/ledger-service",
        "Compare current image tag with last known stable",
        "Check deployment timestamp against first timeout spike"
      ],
      "reason": "Timeout burst on shared dependency with no caller-side change"
    },
    {
      "rank": 2,
      "title": "Inspect PostgreSQL connection pool saturation",
      "category": "database",
      "steps": [
        "Check pg_stat_activity for blocked/waiting connections",
        "Review max_connections vs current active connections",
        "Check ledger-service DB pool config (maxOpen, maxIdle)"
      ],
      "reason": "High latencyP95 (4800ms) on a DB-backed service suggests pool exhaustion"
    },
    {
      "rank": 3,
      "title": "Review ledger-service memory and GC metrics",
      "category": "saturation",
      "steps": [
        "Check RSS and heap usage in Grafana",
        "Look for GC pause spikes aligned with timeout window",
        "Review goroutine count for leak signals"
      ],
      "reason": "Sustained timeout without 5xx suggests processing slowdown not crash"
    },
    {
      "rank": 4,
      "title": "Check settlement-worker queue consumer lag",
      "category": "queue",
      "steps": [
        "Check Redpanda consumer group lag for ledger-events topic",
        "Verify settlement-worker is not stuck in retry loop",
        "Check DLQ for overflow"
      ],
      "reason": "settlement-worker depends on ledger-service — may amplify backpressure"
    }
  ]
}
```

---

### Get Incident

```http
GET /api/v1/incidents/{incidentId}
Authorization: Bearer <token>
```

**Response 200** — same shape as `/analyze` response above, plus:
```json
{
  "status": "OPEN",
  "feedback": null,
  "resolvedAt": null
}
```

---

### List Incidents

```http
GET /api/v1/projects/{projectId}/incidents?environment=prod&status=OPEN&limit=20&offset=0
Authorization: Bearer <token>
```

**Response 200**
```json
{
  "incidents": [
    {
      "id": "inc_06NE5RCO",
      "status": "OPEN",
      "detectedAt": "2026-05-14T00:10:02Z",
      "topCandidate": "ledger-service",
      "confidence": 0.91,
      "recommendationCount": 4
    }
  ],
  "total": 1,
  "limit": 20,
  "offset": 0
}
```

---

### Submit Operator Feedback

Closes the incident loop and stores feedback for future ranking improvement.

```http
POST /api/v1/incidents/{incidentId}/feedback
Authorization: Bearer <token>
```

**Request**
```json
{
  "confirmedRootCause": "svc_ledger",
  "operatorNote": "DB connection pool was maxed out after deploy at 23:45",
  "usefulRecommendations": [1, 2],
  "resolution": "Rolled back ledger-service to v1.4.2",
  "resolvedAt": "2026-05-14T01:30:00Z"
}
```

**Response 200**
```json
{
  "incidentId": "inc_06NE5RCO",
  "status": "RESOLVED",
  "feedbackRecorded": true,
  "resolvedAt": "2026-05-14T01:30:00Z"
}
```

---

## Health Endpoints

### Platform Health

```http
GET /health
```

**Response 200**
```json
{
  "status": "ok",
  "services": {
    "api-gateway":            "ok",
    "project-service":        "ok",
    "detection-engine":       "ok",
    "recommendation-service": "ok",
    "graph-manager":          "ok"
  },
  "dependencies": {
    "neo4j":     "ok",
    "postgres":  "ok",
    "redis":     "ok",
    "redpanda":  "ok",
    "keycloak":  "ok"
  }
}
```

### Readiness

```http
GET /ready
```

**Response 200**
```json
{ "ready": true }
```

---

## Webhook Integration Guide

FaultIQ can receive signals from any tool that supports outbound webhooks.

### Prometheus Alertmanager

```yaml
# alertmanager.yml
receivers:
  - name: faultiq
    webhook_configs:
      - url: http://faultiq:8080/api/v1/signals
        http_config:
          authorization:
            credentials: <signal-publisher-token>
        send_resolved: false
```

The signal ingestion service auto-maps Prometheus alert labels to the canonical signal schema.
Required labels: `service`, `statusClass` (mapped from `severity`).

### PagerDuty Event Rules

Forward incidents to:
```
POST https://your-faultiq-domain/api/v1/signals
Authorization: Bearer <token>
```

Map PagerDuty event fields:
- `service.name` → `service`
- `severity: critical` → `statusClass: 5xx`
- `severity: warning` → `statusClass: timeout`

---

## Rate Limits

| Endpoint group | Limit |
|---|---|
| Signal ingestion | 1000 req/min per project |
| Incident analysis | 60 req/min per project |
| Graph read/write | 120 req/min per project |
| All other endpoints | 300 req/min per tenant |
