# We Built an Open-Source Alternative to Dynatrace AIOps — Here's How the Root Cause Algorithm Works

> **TL;DR:** We open-sourced FaultIQ — a graph-based fault analysis engine that uses BFS traversal, PageRank-style evidence propagation, and a structured SOP state machine to automatically find the root cause of distributed system failures and guide remediation. It's free, self-hosted, and the algorithm is fully transparent.

---

## The Problem With Every Monitoring Tool You're Using

You have Datadog, or Grafana, or New Relic. Your services are instrumented. When something breaks, alerts fire.

Here's what happens when `ledger-service` times out in a payment platform with 10 microservices:

```
🔴 ALERT: payment-api — error rate 18%
🔴 ALERT: settlement-worker — error rate 31%
🔴 ALERT: ledger-service — timeout
🔴 ALERT: api-gateway — elevated latency
```

Four separate alerts. Four separate Slack notifications. Four separate PagerDuty pages.

Your on-call engineer wakes up at 3 AM, looks at four red dashboards, and spends 45 minutes manually tracing dependency paths to figure out that **all four alerts came from one single failure**: `ledger-service` was slow, and everything upstream of it cascaded.

This is the fundamental problem with threshold-based alerting: **it fires per service in isolation, with no understanding of causality**.

---

## Why This Happens: The Cascade Problem

When `ledger-service` times out, here's what actually happens through your call graph:

```mermaid
flowchart TD
    GW[api-gateway\n2xx ✓] --> PA[payment-api\n5xx ✗]
    GW --> AS[auth-service\n2xx ✓]
    PA --> LS[ledger-service\ntimeout ✗ 🎯 ROOT CAUSE]
    PA --> AS
    SW[settlement-worker\n5xx ✗] --> LS
    PA --> EXT[stripe-api\n2xx ✓]
    LS --> DB[(postgres-db\n2xx ✓)]

    classDef failing fill:#fee2e2,stroke:#dc2626,color:#991b1b
    classDef healthy fill:#dcfce7,stroke:#16a3a8,color:#166534
    classDef root fill:#fef3c7,stroke:#d97706,color:#92400e

    class PA,SW failing
    class GW,AS,EXT,DB healthy
    class LS root
```

Notice what's happening:
- `ledger-service` is timing out → it's the **source**
- `payment-api` fails because it calls `ledger-service` → it's a **victim**
- `settlement-worker` fails for the same reason → also a **victim**
- `auth-service` and `stripe-api` are completely unaffected → **healthy**

Every monitoring tool fires separate alerts for `payment-api`, `settlement-worker`, and `ledger-service`. **None of them tell you which is the root cause.** That's the job your on-call engineer is doing manually at 3 AM.

---

## The Algorithm: Graph Traversal + Evidence Accumulation

FaultIQ solves this with a two-part approach:

### Part 1: BFS Traversal With Pruning

When a fault signal arrives for `payment-api`, we don't alert immediately. We run a Breadth-First Search across the service dependency graph:

```mermaid
flowchart LR
    subgraph BFS["BFS Traversal from payment-api"]
        direction TB
        A["Start: payment-api\n5xx signal"] --> B["Check dependencies\nledger-service, auth-service, stripe-api"]
        B --> C{"ledger-service\ntimeout?"}
        C -->|YES - suspect| D["Add to suspect queue\nExpand its dependencies"]
        B --> E{"auth-service\n2xx healthy?"}
        E -->|NO - prune| F["Stop here\nDon't traverse further"]
        D --> G["ledger-service deps:\npostgres-db — 2xx healthy"]
        G --> H["Stop — found boundary\nledger-service is root"]
    end
```

The key insight: **healthy nodes prune the search**. When we hit `auth-service` (healthy 2xx), we stop traversing that branch. When we hit `ledger-service` (timeout), we expand it. This converges quickly on the actual root.

### Part 2: Evidence Accumulation With Time Decay

A single bad health check doesn't create an incident. That's alert fatigue. Instead, FaultIQ accumulates evidence across multiple signals and applies **exponential time decay**:

```mermaid
xychart-beta
    title "Signal Weight Over Time (λ=0.1/min)"
    x-axis ["0 min", "5 min", "10 min", "20 min", "30 min"]
    y-axis "Signal Weight" 0 --> 1
    bar [1.0, 0.60, 0.37, 0.14, 0.05]
```

A signal from 30 minutes ago contributes only 5% to the confidence score. A signal from 2 minutes ago contributes 82%. **Stale evidence decays automatically** — the system won't stay triggered because of a blip that happened an hour ago.

The confidence formula:

```
score = (Σ decay(signal_i) / 5.0) × 0.40   ← time-decayed evidence
      + (failing_callers / total_nodes)  × 0.35   ← blast radius (PageRank-style)
      + error_rate                       × 0.10   ← observed error rate
```

When the score crosses **0.35** (calibrated for a typical 10-node graph), we confirm the root cause.

---

## The Full Detection Pipeline

```mermaid
sequenceDiagram
    participant HP as Health Poller
    participant SI as Signal Ingestion
    participant DE as Detection Engine
    participant RCA as RCA Ranker
    participant DB as Postgres

    HP->>SI: POST /signals<br/>{service: ledger-service, statusClass: timeout}
    SI->>SI: Validate + rate limit
    SI->>DE: Redis PUB "signals"
    
    DE->>DE: Load graph from cache (<1ms)
    DE->>DE: BFS traversal → suspects found
    DE->>DE: Classify fault: latency-anomaly
    DE->>SI: Publish incident_created event (SSE to UI)
    
    Note over DE: Async — UI already notified
    
    DE->>RCA: POST /rank {candidates, evidence, errorRates}
    RCA->>RCA: PageRank scoring → rank candidates
    RCA->>DE: {rootCause: ledger-service, confidence: 0.47}
    DE->>DB: INSERT incidents, rca_candidates
    
    loop Next signals (every 30s)
        HP->>DE: More signals
        DE->>DE: Accumulate evidence\nRe-rank candidates
        DE->>DE: Score crosses 0.35 → CONFIRMED
        DE->>DE: Generate SOP playbook
    end
```

**Key performance characteristic:** The detection event is published to the UI *before* the database write. The operator sees the incident in under 5ms from signal receipt. The DB write, RCA ranking, and playbook generation happen asynchronously.

---

## How It Compares to Existing Tools

```mermaid
quadrantChart
    title Fault Analysis Tools: Transparency vs Automation
    x-axis Low Transparency --> High Transparency
    y-axis Low Automation --> High Automation
    quadrant-1 Ideal
    quadrant-2 Black-Box AI
    quadrant-3 Manual Process
    quadrant-4 Transparent but Manual
    Dynatrace Davis AI: [0.2, 0.7]
    Moogsoft: [0.15, 0.65]
    PagerDuty: [0.4, 0.3]
    Grafana Alerts: [0.5, 0.2]
    FaultIQ: [0.85, 0.75]
```

Dynatrace Davis AI is highly automated but opaque — when it's wrong, you can't understand why. FaultIQ shows you **every decision**:

```json
{
  "event": "root_cause_confirmed",
  "score": 0.474,
  "reasoning": "evidence(7 signals, decayed)×0.40 + propagation(2/10 callers)×0.35 + errorRate(0.42)×0.10 = 0.474 ≥ threshold 0.35",
  "bfs_path": "api-gateway → payment-api → ledger-service (root)"
}
```

This is the reasoning chain that every decision produces. Machine-parseable JSON, stored in Postgres, queryable via API.

---

## Getting Started

```bash
git clone https://github.com/[your-org]/FaultIQ
cd FaultIQ
cp .env.example .env
docker compose up --build
# Open http://localhost:4001
```

Or try the demo in 3 commands without installing anything — see [docs/LOCAL_DEMO.md](../docs/LOCAL_DEMO.md).

**GitHub:** [github.com/your-org/FaultIQ](https://github.com/your-org/FaultIQ)  
**License:** Apache 2.0

---

*In the next post, we go deeper on the BFS algorithm and show how graph topology affects which service gets identified as root cause across different failure patterns.*
