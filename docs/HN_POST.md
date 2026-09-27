# Hacker News Post Draft

## Title
Show OSS: FaultIQ — Graph-based root cause analysis for distributed systems

## Body

When a microservice fails, it cascades. payment-api fails → 4 upstream services alert → your operator spends 45 minutes tracing which service actually caused it.

FaultIQ solves this with graph-based traversal:

1. **Ingest** — Health poller, OpenTelemetry traces, or manual webhook
2. **Traverse** — BFS across your service dependency graph (Neo4j)
3. **Rank** — PageRank-style scoring: evidence + propagation impact + error rate
4. **Remediate** — Structured SOP playbook with auto-verification

**Stack:** 12 Go microservices, Next.js 16, Neo4j, PostgreSQL, Redis, Keycloak

**Key differentiator:** Most RCA tools are black boxes. FaultIQ shows the full reasoning chain — every suspect node, every score, every phase transition is explainable.

**Try it:**
```bash
git clone https://github.com/theprodsde/faultiq && cd faultiq
docker compose up --build
# Open http://localhost:4001/dashboard/demo
```

The demo page has a one-click fault cascade simulation — no terminal needed.

**Links:**
- Repo: https://github.com/theprodsde/faultiq
- Architecture: https://github.com/theprodsde/faultiq/blob/main/docs/architecture.md
- Local demo guide: https://github.com/theprodsde/faultiq/blob/main/docs/LOCAL_DEMO.md

**Known issues:**
- RCA Ranker has a SIGSEGV on Docker/macOS (workaround documented)
- 1-2s delay before first incident appears in graph
- Go 1.24 + Next.js 16 = bleeding edge, smaller contributor pool

Happy to answer questions about the architecture, the BFS traversal logic, or why I chose Neo4j over Memgraph.
