# FaultIQ — Graph-Based Fault Analysis & SOP Automation

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.24-00ADD8?logo=go)](https://go.dev)
[![TypeScript](https://img.shields.io/badge/TypeScript-5-3178C6?logo=typescript)](https://typescriptlang.org)

FaultIQ is an open-source platform that detects faults in distributed systems, identifies the root cause through graph-based traversal, and guides operators through structured remediation with automated verification.

> **The problem**: When payment-api fails, it cascades to 4 other services. Every tool fires a separate alert. Your operator spends 45 minutes manually tracing which service actually caused the failure.
>
> **FaultIQ's answer**: BFS traversal across your service dependency graph identifies the single root cause service in milliseconds. A structured SOP playbook sequences the fix steps. The system auto-verifies recovery when healthy signals arrive.

---

## How It Works

```
Health poller          Detection engine         SOP engine
(polls /health)   →    (BFS + evidence)    →    (phases + playbook)
     ↓                       ↓                        ↓
Signal: ledger-service   Root cause: ledger-service   TRIAGING → FIXING
timeout, err=0.42        confidence: 0.47             Step 1: Check deploy
                         7 signals accumulated        Step 3: AUTO-VERIFY ←── healthy signal
```

**Phases:** `DETECTING → NARROWING → CONFIRMED → TRIAGING → FIXING → VERIFYING → RESOLVED`

Every phase transition is driven by real signal conditions, not timers or manual clicks.

---

## Quick Start

```bash
git clone https://github.com/[your-org]/FaultIQ
cd FaultIQ
cp .env.example .env   # edit with your values
docker compose up --build
```

Open http://localhost:4001 · Login: `super` / `FaultIQ2026!`

**Inject a demo fault in 3 commands:**
```bash
# 1. Set ledger-service to timeout
curl -X PUT http://localhost:8091/admin/services/ledger-service/status \
  -H "Content-Type: application/json" \
  -d '{"statusClass":"timeout","errorRate":0.42,"latencyP95":4800}'

# 2. Watch detection (automatic via health poller, or inject manually)
curl -X POST http://localhost:8085/api/v1/signals \
  -H "Content-Type: application/json" \
  -d '{"tenant":"acme-corp","project":"proj-payments-prod","service":"svc_ledger_service","statusClass":"timeout","errorRate":0.42,"latencyP95":4800}'

# 3. Restore — system auto-resolves
curl -X PUT http://localhost:8091/admin/services/ledger-service/status \
  -H "Content-Type: application/json" \
  -d '{"statusClass":"2xx","errorRate":0.001,"latencyP95":45}'
```

Or use the **interactive demo** at http://localhost:4001/dashboard/demo — no terminal needed.

---

## Architecture

| Layer | Technology | Role |
|-------|-----------|------|
| Service graph | Neo4j + Go | Stores topology + CALLS edges |
| Signal ingestion | Go + Redis pub/sub | Receives health signals, rate-limited |
| Detection engine | Go (BFS + pgxpool) | Traverses graph, accumulates evidence, runs SOP phases |
| RCA ranking | Go (PageRank-style scoring) | Ranks root cause candidates by evidence + propagation |
| SOP playbooks | Go + Postgres | Ordered steps with auto-verify conditions |
| Code awareness | Go (git clone + AST) | Links incidents to recent commits |
| Frontend | Next.js 14 + Cytoscape | Graph visualization, incident detail, SOP panel |
| Auth | Keycloak (OIDC) | Multi-tenant JWT auth |

**12 Go services. 3 SQL migration files. 1 docker-compose.yml.**

---

## Onboarding a Project

Define your service topology in a YAML file (supports 15+ services in under 30 seconds):

```bash
# 1. Start FaultIQ
docker compose up --build

# 2. Import your project via UI:
#    → http://localhost:4001/dashboard/onboarding
#    → Choose "Import YAML" → paste/upload your service-map.yaml
#    → Graph builds automatically

# 3. Or use a built-in template (E-Commerce, SaaS Analytics, Fintech Payments)
```

For Kubernetes: annotate your services and let k8s-connector auto-discover:
```yaml
metadata:
  annotations:
    faultiq.io/health-path: "/health"
    faultiq.io/calls: "svc_ledger,svc_auth"
    faultiq.io/repo: "https://github.com/org/payment-service"
```

See [docs/SERVICE_MAP.md](docs/SERVICE_MAP.md) for full format reference including auth patterns (OAuth2, static keys, TCP checks).

---

## Features

- **Graph-based root cause analysis** — BFS traversal with PageRank-style propagation scoring
- **SOP state machine** — 7-phase pipeline with signal-condition auto-verification
- **Playbook learning** — step reordering based on historical success rates
- **Code-level RCA** — links faults to specific git commits via code-indexer
- **Deployment health gates** — 5-min observation window after CI/CD deploy
- **Multi-tenant** — full isolation per tenant via Keycloak JWT
- **Signal history + SLO tracking** — 7-day retention, real uptime% computation
- **OTel receiver** — auto-builds service graph from distributed traces
- **K8s service discovery** — via annotations or in-cluster service account

---

## Running Tests

```bash
# End-to-end test suite (requires running stack)
bash scripts/test-e2e.sh

# Go unit tests
for svc in detection-engine api-gateway signal-ingestion rca-ranker; do
  cd services/$svc && go test ./... && cd ../..
done

# Frontend TypeScript
cd frontend && npx tsc --noEmit
```

---

## Project Templates

Three ready-to-import templates for common architectures:

| Template | Services | Use case |
|----------|---------|----------|
| [E-Commerce](scripts/templates/ecommerce.yaml) | 12 | Cart, orders, payments, inventory |
| [SaaS Analytics](scripts/templates/saas-analytics.yaml) | 8 | Data pipeline, aggregation, reporting |
| [Fintech Payments](scripts/templates/fintech-payments.yaml) | 11 | Payment processor, fraud detection, ledger |

---

## Documentation

| Doc | What it covers |
|-----|---------------|
| [docs/LOCAL_DEMO.md](docs/LOCAL_DEMO.md) | Running locally and demo walkthrough |
| [docs/SERVICE_MAP.md](docs/SERVICE_MAP.md) | service-map.yaml format + auth patterns |
| [docs/CODE_INDEXER.md](docs/CODE_INDEXER.md) | Git repo integration for code-level RCA |
| [docs/SECRETS.md](docs/SECRETS.md) | Production secrets management |
| [docs/REDIS_HA.md](docs/REDIS_HA.md) | Redis Sentinel high availability |

---

## Comparison

| | FaultIQ | Datadog APM | Dynatrace | Causely |
|--|---------|------------|-----------|---------|
| Graph-based root cause | ✅ | ❌ (per-service alerts) | ✅ (black box) | ✅ (K8s only) |
| SOP state machine | ✅ | ❌ | ❌ | ❌ |
| Reasoning transparency | ✅ Full chain | ❌ | ❌ | ❌ |
| Code commit correlation | ✅ | ✅ ($$$) | ❌ | ❌ |
| Open source / self-hosted | ✅ | ❌ | ❌ | ❌ |
| Pricing | Free | $100k+/yr | $80k+/yr | Enterprise |

---

## Support the Project

If FaultIQ has saved you debugging time, consider buying me a coffee — it helps keep the project alive.

[![Buy Me A Coffee](https://img.shields.io/badge/Buy%20Me%20a%20Coffee-ffdd00?style=for-the-badge&logo=buy-me-a-coffee&logoColor=black)](https://buymeacoffee.com/theprodsde)

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). We welcome bug reports, feature requests, SOP playbook improvements, and pull requests.

## Security

See [SECURITY.md](SECURITY.md) for responsible disclosure guidelines.

## License

Apache 2.0 — see [LICENSE](LICENSE). This means you can use FaultIQ commercially, modify it, and distribute it, as long as you include the license notice.
