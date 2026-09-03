# Local Demo Guide
## Running FaultIQ SOP Locally and Onboarding Projects

---

## Prerequisites

```bash
# Required
docker --version          # Docker 24+
docker compose version    # Compose v2 (not v1 docker-compose)
go version                # Go 1.21+ (only needed to rebuild Go services)
node --version            # Node 18+ (only needed to rebuild frontend)
```

---

## Quick Start (Full Stack)

```bash
cd /path/to/FaultIQ

# 1. Copy and configure environment
cp .env.example .env
# Defaults work for local dev — no changes needed

# 2. Start everything
docker compose up --build

# 3. Wait for healthy (takes ~60s on first run for Keycloak + Neo4j)
docker compose ps

# 4. Open in browser
open http://localhost:4001
```

**Service URLs once running:**

| Service | URL | Purpose |
|---------|-----|---------|
| Frontend | http://localhost:4001 | Main UI |
| API Gateway | http://localhost:8080 | REST API |
| Keycloak | http://localhost:8081 | Auth (admin/admin) |
| Neo4j Browser | http://localhost:7474 | Graph database UI |
| Signal Ingestion | http://localhost:8085 | SSE events endpoint |
| Graph Manager | http://localhost:8086 | Graph read/write |
| RCA Ranker | http://localhost:8087 | Scoring service |
| Demo Services | http://localhost:8091 | Mock services (simulated faults) |

---

## Login Credentials

These accounts are pre-configured in `scripts/keycloak/realm.json`:

| Email | Password | Role | Tenant |
|-------|----------|------|--------|
| `admin@acme.com` | `admin123` | super_admin | acme-corp |
| `operator@acme.com` | `admin123` | tenant_admin | acme-corp |
| `dev@zen.com` | `admin123` | tenant_admin | zen-inc |

---

## Onboarding a Project (the right way)

A project represents one team's set of services. You define the topology once in a `service-map.yaml` file. FaultIQ reads it to:
1. Build the graph in Neo4j (topology, dependencies)
2. Know which health endpoints to poll (autonomous fault detection)

### Step 1: Define your service map

Copy and edit the demo template:

```bash
cp scripts/service-map.yaml scripts/my-project.yaml
```

Format (see `docs/SERVICE_MAP.md` for full spec):

```yaml
version: "1"
projects:
  - id: proj-payments-prod
    name: Payments Platform
    namespace: payments-platform:prod
    tenant: acme-corp
    environment: prod
    services:
      - id: svc_api_gateway
        name: api-gateway
        type: GATEWAY
        healthUrl: http://demo-services:8091/services/api-gateway/health
        calls: [svc_payment_api, svc_auth_service]

      - id: svc_payment_api
        name: payment-api
        type: SERVICE
        healthUrl: http://demo-services:8091/services/payment-api/health
        calls: [svc_ledger_service, svc_auth_service]

      - id: svc_ledger_service
        name: ledger-service
        type: SERVICE
        healthUrl: http://demo-services:8091/services/ledger-service/health
        calls: [db_postgres]

      - id: svc_auth_service
        name: auth-service
        type: SERVICE
        healthUrl: http://demo-services:8091/services/auth-service/health
        calls: [db_redis_cache]

      - id: db_postgres
        name: postgres-db
        type: DATABASE
        healthUrl: http://demo-services:8091/services/postgres-db/health

      - id: db_redis_cache
        name: redis-cache
        type: DATABASE
        healthUrl: http://demo-services:8091/services/redis-cache/health
```

### Step 2: Trigger onboarding via UI or API

**Via UI:**
1. Login → Dashboard → New Project (onboarding wizard)
2. Fill in project name and slug
3. In the "Service Sources" step, paste the contents of your `service-map.yaml`
4. Click "Import" — the onboarding worker reads the YAML and builds the graph

**Via API (fastest for demo):**
```bash
# Get a token first
TOKEN=$(curl -s -X POST http://localhost:8081/realms/faultiq/protocol/openid-connect/token \
  -d "grant_type=password&client_id=faultiq-ui&username=admin@acme.com&password=admin123" \
  | jq -r '.access_token')

# Create project
curl -s -X POST http://localhost:8080/api/v1/tenants/acme-corp/projects \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Payments Platform",
    "slug": "payments-platform",
    "environment": "prod"
  }'

# Trigger onboarding with service map (replace PROJECT_ID with response above)
curl -s -X POST http://localhost:8080/api/v1/projects/PROJECT_ID/onboarding/import \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "mode": "service-map",
    "sources": {
      "serviceMapFile": "scripts/service-map.yaml"
    }
  }'
```

### Step 3: Verify graph is built

```bash
# Check graph-manager directly (no auth needed)
curl "http://localhost:8086/api/v1/graphs?namespace=payments-platform:prod" | jq '.nodes | keys'
```

Expected: array of service IDs like `["db_postgres", "db_redis_cache", "svc_api_gateway", ...]`

Also visible in:
- **Neo4j Browser**: http://localhost:7474 → run `MATCH (n) RETURN n LIMIT 50`
- **FaultIQ UI**: Dashboard → Service Graph → select project

---

## Simulating Faults (Demo Flow)

Once the graph is built and the health poller is running, faults are detected automatically. But for a controlled demo, use the demo-services admin API:

### Simulate a fault cascade (the golden demo path)

This simulates the most realistic scenario: a database slowdown causes upstream timeouts.

```bash
# 1. Make ledger-service timeout (simulates DB contention)
curl -X PUT http://localhost:8091/admin/services/ledger-service/status \
  -H "Content-Type: application/json" \
  -d '{"statusClass":"timeout","errorRate":0.42,"latencyP95":4800}'

# Wait 30-60 seconds — health poller detects it automatically
# OR manually inject the signal immediately:
curl -X POST http://localhost:8080/api/v1/signals \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "tenantId": "acme-corp",
    "projectId": "PROJECT_ID",
    "service": "svc_ledger_service",
    "statusClass": "timeout",
    "errorRate": 0.42,
    "latencyP95": 4800
  }'

# 2. Send 2 more signals to advance from NARROWING → CONFIRMED
# (fires after ~30s naturally via health poller, or manually)
curl -X POST http://localhost:8080/api/v1/signals \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"tenantId":"acme-corp","projectId":"PROJECT_ID","service":"svc_ledger_service","statusClass":"timeout","errorRate":0.45,"latencyP95":5100}'

curl -X POST http://localhost:8080/api/v1/signals \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"tenantId":"acme-corp","projectId":"PROJECT_ID","service":"svc_payment_api","statusClass":"5xx","errorRate":0.18,"latencyP95":1200}'

# 3. Watch in UI → Incidents page — phase advances to CONFIRMED → TRIAGING
# SOP playbook appears in incident detail

# 4. Resolve — restore the service
curl -X PUT http://localhost:8091/admin/services/ledger-service/status \
  -H "Content-Type: application/json" \
  -d '{"statusClass":"2xx","errorRate":0.001,"latencyP95":12}'

# Health poller detects recovery → VERIFYING → RESOLVED automatically
```

### What you should see in the UI during the demo

```
t=0s    Fault injected  →  Incident card appears (OPEN, phase: DETECTING)
t=5s    Evidence grows  →  Phase badge: NARROWING, confidence bar fills
t=15s   Confidence 0.75 →  Phase: CONFIRMED → TRIAGING, SOP playbook unlocked
t=30s   Operator opens  →  Incident detail: phase stepper, step 1 ACTIVE
        incident detail
t=40s   Mark step 1 done → Phase: FIXING, step 2 activates
t=55s   Mark step 3 done → Phase: VERIFYING, "Watching for 2xx from ledger-service"
t=90s   Service restored → Phase: RESOLVED (automatic), audit record written
```

---

## Running Without Docker (local Go services)

If you only want to run backend services locally:

```bash
# Start dependencies only
docker compose up neo4j postgres redis keycloak -d

# Run detection engine
cd services/detection-engine
PG_DSN="postgres://postgres:postgres@localhost:5434/faultiq?sslmode=disable" \
GRAPH_MANAGER_URL="http://localhost:8086/api/v1/graphs" \
REDIS_ADDR="localhost:6379" \
go run .

# Run graph manager
cd services/graph-manager
NEO4J_URI="bolt://localhost:7687" NEO4J_USER="neo4j" NEO4J_PASS="password" \
go run .

# Run health poller (points at demo-services)
cd services/health-poller
SERVICE_MAP_FILE="../../scripts/service-map.yaml" \
SIGNAL_INGESTION_URL="http://localhost:8085/api/v1/signals" \
POLL_INTERVAL_SECONDS=30 \
go run .
```

---

## What Each Service Does (quick reference)

| Service | Port | Role in SOP |
|---------|------|-------------|
| `frontend` | 4001 | Dashboard — SOP phase visualizer, incident detail, service health grid |
| `api-gateway` | 8080 | Auth, routing, SOP step management endpoints |
| `signal-ingestion` | 8085 | Receives signals (pushed or from health-poller), SSE stream to browser |
| `detection-engine` | 8084* | Core SOP brain — BFS, phase transitions, evidence accumulation |
| `graph-manager` | 8086 | Graph topology store (Neo4j backed) |
| `rca-ranker` | 8087 | Scores RCA candidates, returns ranked root causes |
| `health-poller` | — | Polls health endpoints → auto-POSTs signals (autonomous detection) |
| `onboarding-worker` | — | Reads service-map.yaml → writes graph topology to graph-manager |
| `demo-services` | 8091 | Configurable mock services for demo faults (not in production) |

*detection-engine has no external port — health endpoint is at :8084 internal only

---

## Troubleshooting

**Neo4j takes too long to start (>60s)**
```bash
docker compose logs neo4j | tail -20
# Wait for "Started." in logs before running detection-engine
```

**Keycloak auth fails (invalid_client)**
```bash
# Verify realm imported correctly
curl http://localhost:8081/realms/faultiq/.well-known/openid-configuration | jq .issuer
# Should return: "http://localhost:8081/realms/faultiq"
```

**Incident created but phase stays DETECTING**
```bash
# Check detection-engine is running
docker compose logs detection-engine | tail -30
# Check PG_DSN is correct — schema might not be applied
curl http://localhost:8084/health
```

**Graph shows no nodes**
```bash
# Check graph was written to Neo4j
curl "http://localhost:8086/api/v1/graphs?namespace=payments-platform:prod"
# If empty, run onboarding again
```
