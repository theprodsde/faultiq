# SOP – Sequenced Faults for Automation
## Completion Tracker & System Reference

> **Status as of 2026-08-31**
> Automation layer complete. SOP is fully operational.

---

## What "SOP" Means Here

SOP is not a static runbook. It is a live, evidence-driven pipeline that:

1. **Detects** a fault from real service signals (health polling or pushed metrics)
2. **Narrows** the blast radius by accumulating evidence across multiple signals
3. **Confirms** the root cause once confidence exceeds 0.75
4. **Sequences** a structured playbook — ordered steps, each with an expected outcome
5. **Fixes** guided by the operator, with destructive steps requiring explicit confirmation
6. **Verifies** by watching for the expected signal condition after each auto-verify step
7. **Resolves** automatically when the confirmation signal arrives — no manual close needed

The key property: **every state transition is driven by a real signal, not a timer or human click**. The system advances itself when evidence warrants it.

---

## Phase State Machine

```
DETECTING ──► NARROWING ──► CONFIRMED ──► TRIAGING ──► FIXING ──► VERIFYING ──► RESOLVED
    │              │              │                                      │
    │      (new signals        (confidence                        (healthy signal
    │       accumulate)         > 0.75)                           from root service)
    │                                                                    │
    └────────────────────── any phase ──────────── RESOLVED ◄───────────┘
                                                (auto-resolve if quiet 2+ min)
```

| Phase | Trigger | What happens |
|-------|---------|--------------|
| DETECTING | First fault signal arrives | BFS traversal, suspects identified |
| NARROWING | Each additional signal for this incident | Evidence count increments, re-ranking |
| CONFIRMED | Top candidate confidence ≥ 0.75 | Root cause locked, SOP playbook generated |
| TRIAGING | Immediately after CONFIRMED | Operator notified, playbook shown, first step ACTIVE |
| FIXING | Operator marks step 1 DONE | Each subsequent step activates in sequence |
| VERIFYING | Step with `autoVerify: true` marked DONE | System watches for `expectedSignal` |
| RESOLVED | `expectedSignal` condition met on root service | Incident closed, audit record written |

---

## Completion Status

### ✅ DONE — Framework

| Component | File(s) | What it does |
|-----------|---------|--------------|
| Full DB schema | `detection-engine/main.go` ensureSchema + migration `008_sop_phase.sql` | Correct tables with phase, evidence_count, sop_steps |
| Real RCA scoring | `rca-ranker/ranker.go` | Evidence × 0.5 + blast-radius impact × 0.3 + error-rate × 0.2 |
| Evidence accumulation | `detection-engine/main.go` accumulateEvidence() | Re-ranks on every new signal, advances phase when confident |
| SOP phase state machine | `detection-engine/main.go` advancePhase() | DETECTING → NARROWING → CONFIRMED → TRIAGING |
| SOP playbook generation | `detection-engine/recommendations.go` GenerateSOPPlaybook() | 7 fault types × 3-5 ordered steps with AutoVerify flags |
| Auto-verify (basic) | `detection-engine/main.go` handleHealthySignal() | Any 2xx from root service resolves VERIFYING incident |
| SOP API endpoints | `api-gateway/main.go` sopHandler(), incidentPhaseHandler() | GET/PUT sop steps, GET phase |
| Tenant incidents API | `api-gateway/main.go` listTenantIncidentsHandler() | Cross-project incident list |
| Phase stepper UI | `SOPStepsPanel.tsx` | Visual phase progress + playbook steps |
| SOP incident detail | `incidents/[incidentId]/page.tsx` | Phase banner + step actions |
| Service health dashboard | `dashboard/page.tsx` | Per-service tiles with SOP phase badges |
| Multi-tenant fixes | `Sidebar.tsx`, `graph/page.tsx`, `ContextBar.tsx` | No hardcoded tenant fallbacks |
| Static seed data removed | `graph-manager/main.go`, `api-gateway/main.go` | No demo data injected at startup |

---

### ✅ DONE — Automation Layer

| Component | File(s) | What it does |
|-----------|---------|--------------|
| Health poller | `services/health-poller/` | Polls every service's `healthUrl` every 30s, auto-POSTs signals — no human needed |
| Real onboarding worker | `services/onboarding-worker/main.go` | Reads `service-map.yaml`, writes graph nodes + edges to graph-manager, marks project PUBLISHED |
| Demo mock services | `services/demo-services/main.go` | Configurable HTTP server simulating all demo services; admin API to inject faults |
| Service map config | `scripts/service-map.yaml` | Defines topology for 2 demo projects (payments + orders) |
| Targeted auto-verify | `detection-engine/main.go` meetsExpectedSignal() | Checks `2xx`, `latency<Nms`, `errorRate<N.NN` conditions before resolving |
| SOP audit trail | `sop_audit` table + writes in detection-engine + api-gateway | Every phase change and step action logged with actor + signal snapshot |
| Webhook notifications | `detection-engine/main.go` sendWebhookNotification() | Posts to `NOTIFICATION_WEBHOOK_URL` on TRIAGING phase (root cause confirmed) |
| docker-compose.yml | `docker-compose.yml` | Includes demo-services + health-poller; both wired to service-map.yaml |

---

## Completion Definition

The system is "SOP – Sequenced Faults for Automation":

- [x] A fault in a registered service is detected **automatically** (health-poller polls every 30s)
- [x] The system advances through all 7 phases **without human intervention** except for non-auto-verify steps
- [x] Auto-verify steps resolve based on the **specific expected condition** (`2xx`, `latency<Nms`, `errorRate<N`)
- [x] Every phase change and step action is written to the **audit log** (`sop_audit` table)
- [x] The operator is **notified** when root cause is confirmed (`NOTIFICATION_WEBHOOK_URL`)
- [x] A new tenant can **onboard a project** in under 5 minutes using `service-map.yaml`

---

## Items Remaining (nice-to-have, not blockers)

| # | Item | Value |
|---|------|-------|
| 1 | Audit history UI — view sop_audit records in incident detail | Retrospectives, compliance reports |
| 2 | Custom playbooks per service (not just per fault type) | More precise SOPs for known services |
| 3 | Playbook learning — SOPs that resolved successfully improve future playbooks | Continuous improvement |
| 4 | Kubernetes health endpoints — discover services from K8s API instead of YAML | Zero-config onboarding |
