# FaultIQ — Project Structure & Assembly Guide

## Complete Directory Structure

```
faultiq/
├── README.md
├── architecture.md
├── api.md
├── onboarding.md
├── go.work                          ← Go workspace (links all modules)
│
├── packages/
│   └── types/
│       ├── go.mod                   ← module: github.com/your-org/faultiq/packages/types
│       └── types.go                 ← ✅ generated
│
├── services/
│   ├── api-gateway/
│   │   ├── go.mod
│   │   ├── Dockerfile
│   │   └── main.go
│   ├── project-service/
│   │   ├── go.mod
│   │   ├── Dockerfile
│   │   └── main.go
│   ├── onboarding-service/
│   │   ├── go.mod
│   │   ├── Dockerfile
│   │   └── main.go
│   ├── signal-ingestion/
│   │   ├── go.mod
│   │   ├── Dockerfile
│   │   └── main.go
│   ├── detection-engine/
│   │   ├── go.mod
│   │   ├── Dockerfile
│   │   ├── main.go
│   │   └── engine.go                ← ✅ generated
│   ├── recommendation-service/
│   │   ├── go.mod
│   │   ├── Dockerfile
│   │   └── main.go
│   └── graph-manager/
│       ├── go.mod
│       ├── Dockerfile
│       └── main.go
│
├── ui/
│   ├── Dockerfile
│   ├── package.json
│   └── src/
│
├── infra/
│   ├── neo4j/
│   │   └── init/                    ← Cypher init scripts
│   ├── postgres/
│   │   └── migrations/              ← SQL migration files
│   │       ├── 001_tenants.sql
│   │       ├── 002_projects.sql
│   │       ├── 003_incidents.sql
│   │       └── 004_keycloak_schema.sql
│   ├── keycloak/
│   │   └── realm-export.json        ← Keycloak realm config
│   ├── redpanda/
│   │   └── console-config.yaml
│   ├── prometheus/
│   │   └── prometheus.yml
│   └── grafana/
│       ├── dashboards/
│       └── provisioning/
│
├── scripts/
│   └── seed-demo.go                 ← ✅ generated
│
└── docker-compose.yml               ← ✅ generated
```

---

## File Placement

| Generated File | Place at |
|---|---|
| `README.md` | `faultiq/README.md` |
| `architecture.md` | `faultiq/architecture.md` |
| `api.md` | `faultiq/api.md` |
| `onboarding.md` | `faultiq/onboarding.md` |
| `types.go` | `faultiq/packages/types/types.go` |
| `engine.go` | `faultiq/services/detection-engine/engine.go` |
| `docker-compose.yml` | `faultiq/docker-compose.yml` |
| `seed-demo.go` | `faultiq/scripts/seed-demo.go` |

---

## go.work (workspace root)

```
go 1.24

use (
    ./packages/types
    ./services/api-gateway
    ./services/project-service
    ./services/onboarding-service
    ./services/signal-ingestion
    ./services/detection-engine
    ./services/recommendation-service
    ./services/graph-manager
)
```

---

## go.mod per service (example — detection-engine)

```
module github.com/your-org/faultiq/services/detection-engine

go 1.24

require (
    github.com/your-org/faultiq/packages/types v0.0.0
    github.com/neo4j/neo4j-go-driver/v5 v5.20.0
    github.com/redis/go-redis/v9 v9.5.1
    github.com/twmb/franz-go v1.17.0
    github.com/go-chi/chi/v5 v5.0.12
)

replace github.com/your-org/faultiq/packages/types => ../../packages/types
```

All other services follow same pattern — replace module path + adjust replace directive depth.

---

## packages/types/go.mod

```
module github.com/your-org/faultiq/packages/types

go 1.24
```

No external dependencies — pure Go stdlib.

---

## Dockerfile (same template for all services)

```dockerfile
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.work go.work.sum ./
COPY packages/ packages/
COPY services/<service-name>/ services/<service-name>/
RUN go build -o /bin/service ./services/<service-name>/...

FROM alpine:3.19
RUN apk --no-cache add ca-certificates tzdata
COPY --from=builder /bin/service /service
EXPOSE <port>
ENTRYPOINT ["/service"]
```

---

## infra/postgres/migrations/001_tenants.sql

```sql
CREATE TABLE tenants (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    slug       TEXT NOT NULL UNIQUE,
    plan       TEXT NOT NULL DEFAULT 'pro',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE projects (
    id         TEXT PRIMARY KEY,
    tenant_id  TEXT NOT NULL REFERENCES tenants(id),
    name       TEXT NOT NULL,
    slug       TEXT NOT NULL,
    domain     TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(tenant_id, slug)
);

CREATE TABLE environments (
    id              TEXT PRIMARY KEY,
    project_id      TEXT NOT NULL REFERENCES projects(id),
    name            TEXT NOT NULL,
    graph_namespace TEXT NOT NULL UNIQUE
);
```

---

## infra/postgres/migrations/002_incidents.sql

```sql
CREATE TABLE incidents (
    id             TEXT PRIMARY KEY,
    project_id     TEXT NOT NULL REFERENCES projects(id),
    environment_id TEXT NOT NULL REFERENCES environments(id),
    status         TEXT NOT NULL DEFAULT 'OPEN',
    graph_version  INT,
    detected_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at    TIMESTAMPTZ
);

CREATE TABLE rca_candidates (
    id                 TEXT PRIMARY KEY,
    incident_id        TEXT NOT NULL REFERENCES incidents(id),
    node_id            TEXT NOT NULL,
    rank               INT NOT NULL,
    confidence_score   FLOAT NOT NULL,
    fault_type         TEXT,
    evidence           JSONB,
    operator_confirmed BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE TABLE recommendations (
    id              TEXT PRIMARY KEY,
    incident_id     TEXT NOT NULL REFERENCES incidents(id),
    rank            INT NOT NULL,
    playbook_title  TEXT NOT NULL,
    category        TEXT,
    steps           JSONB,
    reason          TEXT
);

CREATE TABLE operator_feedback (
    incident_id            TEXT PRIMARY KEY REFERENCES incidents(id),
    confirmed_root_cause   TEXT,
    operator_note          TEXT,
    useful_recommendations JSONB,
    resolution             TEXT,
    resolved_at            TIMESTAMPTZ
);
```

---

## infra/redpanda/console-config.yaml

```yaml
kafka:
  brokers: ["redpanda:9092"]
  schemaRegistry:
    enabled: false
```

---

## infra/prometheus/prometheus.yml

```yaml
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: faultiq-services
    static_configs:
      - targets:
          - api-gateway:8080
          - project-service:8081
          - signal-ingestion:8083
          - detection-engine:8084
          - recommendation-service:8085
          - graph-manager:8086
    metrics_path: /metrics
```

---

## Quick Start (after placing all files)

```bash
# 1. Init workspace
cd faultiq
go work sync

# 2. Start full stack
docker compose up -d

# 3. Wait for all services healthy (~60s for Keycloak)
docker compose ps

# 4. Seed demo data + run first incident
go run scripts/seed-demo.go

# 5. Open console
open http://localhost:3000
```

Here's the summary so you can act on it directly:

File Placement
Generated File	Drop it at
README.md	faultiq/README.md
architecture.md	faultiq/architecture.md
api.md	faultiq/api.md
onboarding.md	faultiq/onboarding.md
types.go	faultiq/packages/types/types.go
engine.go	faultiq/services/detection-engine/engine.go
docker-compose.yml	faultiq/docker-compose.yml
seed-demo.go	faultiq/scripts/seed-demo.go
What You Need to Create Manually
go.work — workspace file linking all modules (template in PROJECT_SETUP.md)

go.mod per service — one per services/* folder (template included)

packages/types/go.mod — no deps, just module declaration

Dockerfile per service — same template, swap service name and port

2 SQL migration files — 001_tenants.sql and 002_incidents.sql (full SQL in PROJECT_SETUP.md)

infra/redpanda/console-config.yaml — 4 lines

infra/prometheus/prometheus.yml — scrape config (included)

main.go per service — HTTP handler stubs wiring to engine.go and types.go

Quick Start
bash
cd faultiq
go work sync
docker compose up -d          # ~60s for Keycloak to boot
go run scripts/seed-demo.go   # seeds graph + runs first incident
open http://localhost:3000
The only pieces not generated yet are the main.go stubs per service and the Keycloak realm-export.json
