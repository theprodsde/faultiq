# Reddit Post Draft

## Subreddits
- r/devops
- r/golang
- r/kubernetes
- r/SRE

## Title
Show OSS: FaultIQ — Open-source root cause analysis using graph traversal

## Body

Hey r/devops,

I've been building FaultIQ — an open-source tool for root cause analysis in distributed systems. It's designed for the "everything is alerting and I don't know where to start" problem.

**How it works:**
1. Services report health (via poller, OTel, or webhook)
2. Detection engine does BFS traversal on the service dependency graph
3. RCA ranker scores candidates by evidence + propagation impact
4. SOP playbook guides the operator through remediation
5. Auto-verifies recovery when healthy signals return

**Stack:** 12 Go microservices, Next.js 16, Neo4j, PostgreSQL, Redis, Keycloak

**Why open source?** Most RCA tools (Datadog, Dynatrace, Causely) are expensive black boxes. FaultIQ shows the full reasoning chain — every suspect, every score, every phase transition is explainable.

**Quick start:**
```bash
git clone https://github.com/theprodsde/faultiq && cd faultiq
docker compose up --build
# Open http://localhost:4001/dashboard/demo — one-click fault simulation
```

**Repo:** https://github.com/theprodsde/faultiq

**Docs:**
- Architecture: https://github.com/theprodsde/faultiq/blob/main/docs/architecture.md
- Local demo: https://github.com/theprodsde/faultiq/blob/main/docs/LOCAL_DEMO.md
- Service map format: https://github.com/theprodsde/faultiq/blob/main/docs/SERVICE_MAP.md

**Known issues:**
- SIGSEGV on Docker/macOS (workaround in docs/KNOWN_ISSUES.md)
- 1-2s delay before first incident appears
- Bleeding-edge versions (Go 1.24, Next.js 16)

Feedback welcome — especially on the BFS traversal logic and the SOP state machine design.
