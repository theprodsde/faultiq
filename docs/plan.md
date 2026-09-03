# FaultIQ Development Plan

This document is the prioritized implementation plan for FaultIQ. It breaks the work into phases, milestones, and actionable tasks with estimates and owners.

## Goal
Deliver a minimal end-to-end RCA platform (Detection → RCA → Recommendations) that can ingest signals, traverse a service graph, identify root-cause candidates, and present recommendations.

---

## Phases & Priorities

- Phase 1 — Core Pipeline (Weeks 0–2)
  - Detection Engine (priority): BFS traversal, pruning, suspect scoring. Unit tests and deterministic examples. (Owner: core)
  - Graph Manager: Neo4j client wrapper, subgraph queries, caching hooks (Redis). (Owner: core)
  - Signal Ingestion: normalize incoming signals and publish to messaging (or in-memory queue for MVP). (Owner: core)
  - Persistence: Postgres schema for incidents, RCA candidates, recommendations; migrations. (Owner: infra)

- Phase 2 — RCA & Recommendations (Weeks 2–4)
  - RCA Ranker: implement scoring model and evidence bundling; write integration tests.
  - Recommendation Service: playbook mapping and ranked recommendations.

- Phase 3 — Platform & UX (Weeks 4–6)
  - Project service & Onboarding connectors (OpenAPI + k8s traces).
  - Console UI: React + graph visualization, incident panel + recommendation panel.
  - Auth & RBAC: Keycloak integration and token middleware.

- Phase 4 — Hardening & CI (Weeks 6+)
  - Messaging: Redpanda topics and consumer groups; compose entries.
  - CI: unit tests, integration compose target, image builds.
  - Observability: metrics, logs, distributed tracing.
  - Deployment: Helm charts and k8s manifests.

---

## Milestones (deliverable-oriented)

1. Seed + signal path: `scripts/seed-demo.go` + `/api/v1/signals` + simple detection consumer that produces incident entries (done for demo).
2. Deterministic Detection Engine with unit tests and a public API or message consumer.
3. RCA Ranker + Postgres persistence + recommendations mapping.
4. Console UI with incident list and RCA detail (graph view).
5. CI pipeline that verifies integration tests (compose) on each push.

---

## Immediate tasks (next 7 days)

- Implement Detection Engine scaffold and unit tests (in repo under `services/detection-engine`).
- Add Postgres schema and simple migration for `incidents` and `rca_candidates`.
- Wire `signal-ingestion` to publish to an internal in-memory channel (for local dev) and to Redpanda (for prod).
- Expand integration test to assert RCA candidate persisted in Postgres.

---

## Testing strategy

- Unit tests: deterministic graph traversal and scoring.
- Integration tests: docker-compose orchestration (Neo4j, Postgres, api-gateway, detection-engine) using in-network containers; seeds + generator produce an incident within 3 minutes.
- E2E tests: Cypress or Playwright against the console UI.

---

## Risks & Mitigations

- Neo4j startup timing issues: run seeder from container on compose network (already implemented).
- Networking quirks on macOS (IPv6 vs IPv4): use container hostnames on the compose network for tests.
- Large graph traversal performance: cache subgraphs in Redis and limit traversal depth.

---

## How I'll proceed now

1. Scaffold `services/detection-engine` with a deterministic BFS implementation and unit tests. (Completed)
2. Implement Postgres schema migration for incidents and wire detection output to persist candidates. (Completed)
3. Integrate Keycloak for authentication and RBAC: added Keycloak docker service with realm import, `RequireAuth` middleware, JWKS support, `RequireRole` and `RequireTenant` middleware, and protected onboarding endpoints. (Completed)
4. Scaffold React frontend with Keycloak JS integration and onboarding form that posts to protected API endpoints. (Completed)

Next planned items:
- Switch API gateway to JWKS mode in production by setting `AUTH_MODE=jwks` and `JWKS_URL` to the Keycloak realm JWKS endpoint.
- Implement per-endpoint tenant scoping enforcement across services and add policy tests.

If you'd like different priorities or ownership, tell me which items to reorder.
