# FaultIQ — Microservice Fault Detection & Analysis Platform

> Real-time fault detection, root cause analysis, and dependency visualization for distributed systems.
> Graph-based RCA with interactive Neo4j visualization and signal-driven incident detection.

**Current Status**: ✅ MVP Complete and Demo-Ready (May 25, 2026)

---

## What It Does

FaultIQ models your microservice dependencies as an interactive graph. When a fault signal arrives (error spike, timeout burst, high latency), it:

1. **Detects** the incident via signal ingestion (5xx errors, timeouts, high error rates)
2. **Visualizes** the impact on the service dependency graph (interactive Cytoscape.js)
3. **Analyzes** root cause by examining service dependencies and health states
4. **Recommends** remediation steps based on incident patterns
5. **Tracks** incidents with confidence scores and evidence

**No auto-remediation in MVP** — operators review recommendations and act manually. Intentional design for safety.

---

## Live Demo Flow

1. **Send Error Signals** → `/dashboard/signals`
   - Submit realistic fault scenarios (timeout, 5xx errors, high error rates)
   - Three demo signals: Root Cause, Symptom, Blast Radius

2. **Auto-Redirect to Graph** → `/dashboard/graph`
   - View interactive service dependency visualization
   - Click nodes to see error rates, latency, and status
   - Color-coded edges show service health (green=healthy, yellow=degraded, red=failed)

3. **View Incidents** → `/dashboard/incidents`
   - Real-time incident list with auto-refresh (5-second polling)
   - Shows root cause candidate, confidence score, and evidence count
   - Filter by status (OPEN, ACKNOWLEDGED, RESOLVED)

4. **Analyze Results** → `/dashboard/reports`
   - KPI cards: total incidents, open, resolved, avg confidence
   - Charts: daily incident trends, status distribution, confidence histogram

---

## Repository Structure (Current)

```
FaultIQ/
├── docker-compose.yml                ← Full local stack (single command startup) ✅
├── frontend/                         ← React 19 + Next.js 16.2.6 + TypeScript ✅
│   ├── src/
│   │   ├── app/(protected)/dashboard/
│   │   │   ├── page.tsx             ← Home dashboard with stats cards
│   │   │   ├── signals/page.tsx     ← Error signal injection (3 demo scenarios)
│   │   │   ├── incidents/page.tsx   ← Incident list with real-time auto-refresh
│   │   │   ├── graph/page.tsx       ← Interactive Cytoscape.js graph visualization
│   │   │   ├── reports/page.tsx     ← Analytics and KPI cards
│   │   │   └── settings/page.tsx    ← User preferences and theme
│   │   ├── components/
│   │   │   ├── interactive-graph-cytoscape.tsx  ← Cytoscape graph component (NEW)
│   │   │   └── ui/                  ← Reusable UI components
│   │   ├── store/
│   │   │   ├── api.ts               ← RTK Query base configuration
│   │   │   └── services.ts          ← API endpoint definitions
│   │   ├── config/index.ts          ← Environment configuration
│   │   ├── contexts/                ← React contexts (tenant, auth)
│   │   ├── providers/               ← Auth provider with Keycloak
│   │   └── types/api.ts             ← TypeScript API types
│   ├── package.json                 ← 646 dependencies (Cytoscape added)
│   ├── next.config.ts               ← Next.js configuration
│   └── tsconfig.json                ← TypeScript strict mode
│
├── services/
│   ├── api-gateway/                 ← Go API routing, auth middleware ✅
│   │   ├── main.go                  ← Incident CRUD, signal handling
│   │   ├── auth.go                  ← JWT validation, RBAC
│   │   └── internal/types/          ← Shared domain types
│   ├── rca-ranker/                  ← Root cause analysis scoring ⚠️
│   │   ├── main.go                  ← HTTP endpoint for ranking
│   │   ├── ranker.go                ← Ranking algorithm
│   │   ├── Dockerfile               ← Go 1.24 Alpine image
│   │   └── go.mod                   ← Go 1.24 required
│   ├── graph-manager/               ← Neo4j graph operations ✅
│   ├── detection-engine/            ← Signal processing ✅
│   ├── signal-ingestion/            ← Redis pub/sub ✅
│   └── onboarding-worker/           ← Project setup ✅
│
├── pkg/
│   └── graphclient/                 ← Neo4j driver wrapper
│
├── scripts/
│   ├── migrate/                     ← PostgreSQL migrations
│   ├── seed-demo.go                 ← 22 seeded services + demo data
│   └── integration/                 ← E2E test scripts
│
└── docs/
    ├── README.md                    ← This file
    ├── architecture.md              ← System design
    ├── api.md                       ← API reference
    └── DEPLOYMENT.md                ← Docker/K8s setup
```

---

## Tech Stack (May 2026)

| Layer | Technology | Version | License | Status |
|---|---|---|---|---|
| **Backend** | Go | 1.24 | BSD-3 | ✅ Running |
| **Frontend** | Next.js + React + TypeScript | 16.2.6 + 19 + 5 | MIT | ✅ Running (port 3001/4001) |
| **Graph DB** | Neo4j Community | 5.x | GPL-3 | ✅ Running (7687) |
| **Relational DB** | PostgreSQL | 16 | PostgreSQL License | ✅ Running (5432) |
| **Cache/Pub-Sub** | Redis | 7 | BSD-3 | ✅ Running (6379) |
| **Authentication** | Keycloak | 24 | Apache 2.0 | ✅ Running (8081) |
| **Graph Viz** | Cytoscape.js + Cose-Bilkent | 3.30 | MIT | ✅ Interactive |
| **UI Framework** | Tailwind CSS | 4.x | MIT | ✅ Dark mode |
| **State Management** | Redux Toolkit Query | 2.12.0 | MIT | ✅ Auto-caching |

---

## Core Concepts

### Tenant
Multi-tenant boundary. Each tenant owns multiple projects. Keycloak realm user assignment.

### Project / Product
One product or business system (e.g., "Payments Platform"). 
Each project has multiple environments and services.

### Environment
Deployment scope: `dev`, `staging`, `production`.
Each environment has its own isolated graph namespace: `{project-slug}:{environment}`.

### Service Graph (Neo4j)
Runtime dependency model with real-time health metrics.
- **Nodes**: microservices, databases, queues, gateways, external services
- **Node State**: status (2xx/5xx/timeout), error rate, latency p95, health score
- **Edges**: API calls, event flows, data dependencies
- **Edge State**: success ratio, confidence, last observed timestamp

### Incidents (PostgreSQL + In-Memory)
Detected faults with root cause analysis.
- **Status**: OPEN, ACKNOWLEDGED, RESOLVED
- **Fields**: service name, root cause candidate, confidence score, evidence list
- **Confidence**: calculated from error rate, impact, and evidence

---

## Local Development Setup

### Prerequisites
- Docker + Docker Compose (all infrastructure in containers)
- Node 20+ (frontend development)
- Git

### Start Complete Stack

```bash
# Clone repo
cd /Users/karangehlod/Downloads/FaultIQ

# Start all services (one command)
docker-compose up -d

# Frontend dev server (hot reload)
cd frontend && npm install && npm run dev
# Opens on http://localhost:3001

# OR run frontend in Docker
docker-compose up -d frontend
# Opens on http://localhost:4001
```

### Service Ports

| Service | Port | Status |
|---|---|---|
| Frontend (dev) | 3001 | ✅ Running |
| Frontend (Docker) | 4001 | ✅ Available |
| API Gateway | 8080 | ✅ Running |
| Neo4j Browser | 7474 | ✅ Available |
| Keycloak Admin | 8081 | ✅ Running |
| PostgreSQL | 5432 | ✅ Running |
| Redis | 6379 | ✅ Running |
| RCA Ranker | 8087 | ⚠️ Known Issue |

---

## Demo Test Credentials

### Keycloak Users
- **Super Admin**: `super` / `superpass` (full access)
- **Analyst**: `analyst` / `analystpass` (read-only)
- **Realm**: `faultiq`

---

## Demo Scenario

**Two Products Seeded**:
1. **Payments Platform** (prod)
   - 15 services: payment-api → ledger-service → postgres
   - payment-api → auth-service
   - settlement-worker → ledger-service

2. **Orders Platform** (prod)
   - 7 services: order-api → inventory-service
   - notification-service

**Demo Flow**:
1. Go to `/dashboard/signals`
2. Send error signals:
   - "Ledger Service Timeout" (root cause)
   - "Payment API Failures" (symptom)
   - "Settlement Worker Degraded" (blast radius)
3. Auto-redirect to `/dashboard/graph` after each signal
4. View interactive graph with color-coded service health
5. Check `/dashboard/incidents` for detected incidents (5-second auto-refresh)
6. Click nodes in graph to see detailed metrics

---

## Key Features Implemented

✅ **Signal Injection**
- Realistic error scenarios (timeout, 5xx, high error rate)
- One-click submission with success feedback
- Auto-redirect to graph for visual impact

✅ **Interactive Graph Visualization**
- Cytoscape.js with Cose-Bilkent layout algorithm
- Node selection with detailed info panels
- Color-coded edges by health status
- Zoom/pan controls
- Dark mode support

✅ **Real-Time Incident Tracking**
- 5-second auto-refresh polling
- Manual refresh button
- Status filtering (OPEN/ACKNOWLEDGED/RESOLVED)
- Confidence scores and evidence counts

✅ **Dashboard Analytics**
- KPI cards: total incidents, open, resolved, avg confidence
- Charts: daily trends, status distribution, confidence histogram
- Real-time updates

✅ **Authentication & Multi-Tenancy**
- Keycloak 24 integration
- JWT token validation
- Tenant scoping on all endpoints
- Role-based access control (super_admin vs analyst)

✅ **Type Safety**
- Full TypeScript with strict mode
- RTK Query with automatic cache management
- Zero TypeScript compilation errors

---

## What's Working

| Component | Status | Notes |
|---|---|---|
| Frontend Dev Server | ✅ | Port 3001, hot reload enabled |
| Docker Frontend | ✅ | Port 4001 |
| API Gateway | ✅ | All endpoints functional |
| Neo4j Graph | ✅ | 22 services seeded, queries working |
| PostgreSQL | ✅ | Incident storage working |
| Keycloak Auth | ✅ | JWT validation working |
| Redis Cache | ✅ | Signal pub/sub working |
| Signal Submission | ✅ | Creates incidents, auto-redirect |
| Graph Visualization | ✅ | Cytoscape clean interactive graph |
| Incident Auto-Refresh | ✅ | 5-second polling |
| Dark/Light Theme | ✅ | Full Tailwind dark mode |

---

## Known Issues & Workarounds

See [KNOWN_ISSUES.md](./KNOWN_ISSUES.md) for detailed troubleshooting guide.

### Summary:
| Issue | Severity | Impact | Workaround |
|---|---|---|---|
| RCA Ranker SIGSEGV | ⚠️ Medium | Container crashes, restarts | Non-blocking: Demo works without it |
| 401 Token Expiry | 🔴 Low | Occasional auth errors | Manual page reload or re-login |
| Graph Incident Delay | 🟡 Low | 1-2s delay before first incident appears | Use 5s auto-refresh or manual refresh |
| Mobile Graph Rendering | 🟡 Low | Graph UI poor on mobile | Use desktop for graph views |

---

## Getting Started (Quick Demo)

### 1. Start All Services
```bash
cd /Users/karangehlod/Downloads/FaultIQ
docker-compose up -d
```

### 2. Start Frontend (Dev)
```bash
cd frontend
npm install --legacy-peer-deps
npm run dev
# Opens http://localhost:3001
```

### 3. Login
- URL: `http://localhost:3001/login`
- Username: `super`
- Password: `superpass`

### 4. Send Error Signals
- Go to `/dashboard/signals`
- Select "Payments Platform"
- Click "Send Signal" (3 pre-filled scenarios)
- Auto-redirects to graph after each signal

### 5. View Results
- **Graph**: `/dashboard/graph` - Interactive dependency visualization
- **Incidents**: `/dashboard/incidents` - List of detected faults
- **Reports**: `/dashboard/reports` - Analytics and trends

---

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────┐
│                    Browser (React)                       │
│  Dashboard │ Signals │ Incidents │ Graph │ Reports       │
└─────────────────┬───────────────────────────────────────┘
                  │ HTTPS/REST
┌─────────────────▼───────────────────────────────────────┐
│              API Gateway (Go) :8080                      │
│  Routes, Auth, Rate Limiting, Incident CRUD             │
└─────────────────┬───────────────────────────────────────┘
        ┌─────────┼─────────┬──────────┐
        │         │         │          │
    ┌───▼──┐  ┌──▼──┐  ┌───▼──┐  ┌───▼──┐
    │Neo4j │  │  PG │  │Redis │  │KC-24 │
    │:7687│  │:5432│  │:6379 │  │:8081 │
    └──────┘  └─────┘  └──────┘  └──────┘
     Graphs  Metadata  Signals   JWT Auth
```

---

## All Features Status

| Feature | Status | Test Location |
|---|---|---|
| Multi-tenant Auth | ✅ | Login with super/analyst users |
| Signal Injection | ✅ | `/dashboard/signals` |
| Graph Visualization | ✅ | `/dashboard/graph` (Cytoscape.js) |
| Incident Detection | ✅ | `/dashboard/incidents` (5s refresh) |
| Real-time Analytics | ✅ | `/dashboard/reports` (charts) |
| Dashboard Home | ✅ | `/dashboard` (stats cards) |
| Settings/Theme | ✅ | `/dashboard/settings` (dark mode) |
| Auto-redirect | ✅ | Signal → Graph (1.5s) |
| Dark/Light Mode | ✅ | Top-right theme toggle |
| Type-safe API | ✅ | `npm run type-check` (0 errors) |

---

## Next Steps (Post-MVP)

1. **Persistent Incident Storage** → Move incidents from memory to PostgreSQL
2. **RCA Ranker Stability** → Use Go 1.23 or newer stable version
3. **Real Observability Connectors** → Prometheus, OpenTelemetry, Jaeger
4. **ML-based Anomaly Detection** → Replace rules with trained models
5. **Mobile Optimization** → Responsive graph visualization
6. **User Feedback Loop** → Incident annotations and ML retraining
7. **Kubernetes Integration** → Helm charts for production deployment
8. **Multi-region Support** → Cross-datacenter incident correlation

---

## License

This project is proprietary. All rights reserved.
Patent pending on graph-based fault detection methodology.
