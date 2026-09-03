# Changelog

All notable changes to FaultIQ are documented here.

## [0.0.1] — 2026-09-02

### Initial open-source release

#### Core SOP Engine
- 7-phase state machine: DETECTING → NARROWING → CONFIRMED → TRIAGING → FIXING → VERIFYING → RESOLVED
- BFS graph traversal with PageRank-style evidence propagation
- Exponential time decay on evidence signals (λ=0.1/min, half-life ≈7 min)
- Signal-condition auto-verification (`2xx`, `latency<Nms`, `errorRate<N`)
- Structured SOP playbooks per fault type (7 types × 3-5 steps)
- Custom playbooks per service in service-map.yaml
- Playbook learning — step reordering from historical success rates

#### Infrastructure
- 12 Go microservices, all non-root Docker images
- 3 clean SQL migration files covering full schema
- Single docker-compose.yml with TLS and Redis-HA profiles
- Health poller: HTTP/HTTPS/TCP, Spring Boot, K8s, OAuth2 dynamic tokens
- Redis-backed incident deduplication (survives restarts)
- Signal history (7-day retention, SLO computation)

#### Deployment & Code Awareness
- CI/CD deployment tracking with 5-minute health gates
- Deployment correlation with incidents (fault within 30 min = flagged)
- code-indexer: git clone → AST function extraction → Neo4j → cleanup
- GitHub push webhook for on-demand re-indexing
- K8s service discovery via annotations

#### Observability
- Structured JSON reasoning logs (`docker logs ... | grep REASONING | jq .`)
- Full decision chain API (`GET /incidents/{id}/reasoning`)
- SOP audit trail (`sop_audit` table with actor + signal snapshot)
- Prometheus metrics on detection-engine
- P1/P2 severity routing (PagerDuty vs Slack)

#### Multi-tenancy & Security
- Full Keycloak OIDC multi-tenant auth
- M2M API key auth for CI/CD (`X-API-Key` header)
- Input sanitization and rate limiting on all endpoints
- Per-IP rate limiting (50 req/s, burst 100)

#### Frontend
- Service dependency graph (Cytoscape.js, real-time SSE colors)
- Incident detail with phase stepper, SOP steps panel, audit trail
- Project onboarding with YAML import and 3 built-in templates
- Interactive demo walkthrough at `/dashboard/demo`
- Deployments × incidents correlation timeline
- SLO dashboard with real uptime% from signal history
- MTTR analytics and phase dwell time charts

---

## Format

This project follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/)
and [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
