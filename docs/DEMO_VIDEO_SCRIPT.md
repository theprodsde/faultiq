# FaultIQ — 2-Minute Demo Video Script

## Goal
Show the full fault lifecycle: inject → detect → rank → remediate → auto-resolve.

## Equipment
- Screen recording (OBS / QuickTime)
- Terminal window
- Browser window

---

## Scene 1: Hook (0:00–0:15)

**Visual:** Terminal with a failing service log

**Voiceover:**
> "Your payment API is failing. Four services are alerting. Which one caused it? Let me show you how FaultIQ answers that in milliseconds."

**Action:** Show a fake log stream with errors cascading

---

## Scene 2: Quick Start (0:15–0:30)

**Visual:** Terminal

**Voiceover:**
> "Clone, compose up, done."

**Action:**
```bash
git clone https://github.com/theprodsde/faultiq && cd faultiq
docker compose up --build
```

**Voiceover:**
> "12 Go services, Neo4j, Postgres, Redis, Keycloak — all orchestrated in one file."

---

## Scene 3: Dashboard Tour (0:30–0:50)

**Visual:** Browser at http://localhost:4001

**Voiceover:**
> "This is the FaultIQ dashboard. You can see the service graph — every node is a service, every edge is a dependency. Green means healthy."

**Action:** Click through Dashboard → Service Graph → Incidents

**Voiceover:**
> "The graph is live. When a fault happens, the affected nodes turn red, and the root cause gets a confidence score."

---

## Scene 4: Inject Fault (0:50–1:10)

**Visual:** Browser at http://localhost:4001/dashboard/demo

**Voiceover:**
> "Let me simulate a database slowdown — the most common root cause."

**Action:** Click "All healthy — continue" (Step 1), then "Fault ledger-service" (Step 2)

**Voiceover:**
> "Ledger service is now timing out. Watch what happens."

---

## Scene 5: Detection + RCA (1:10–1:35)

**Visual:** Browser — Incidents page

**Voiceover:**
> "Within seconds, FaultIQ detected the fault. The detection engine did a BFS traversal from the impacted service, scored every suspect, and ranked ledger-service as the root cause with 91% confidence."

**Action:** Click on the incident → show the suspect list with scores

**Voiceover:**
> "Every candidate is shown with its evidence — direct signal, shared impact, branch isolation, latency anomaly."

---

## Scene 6: SOP Playbook (1:35–1:50)

**Visual:** Browser — Incident detail with SOP steps

**Voiceover:**
> "The SOP playbook sequences the fix steps. Step 1: check recent deployments. Step 2: inspect the database connection pool. Step 3: compare canary vs stable."

**Action:** Click through the steps

**Voiceover:**
> "Each step has an auto-verify condition. When the service recovers, the system marks it resolved automatically."

---

## Scene 7: Auto-Resolve (1:50–2:00)

**Visual:** Browser — incident status changes to RESOLVED

**Voiceover:**
> "I just restored the service. Watch the phase advance: VERIFYING → RESOLVED. No manual click needed."

**Action:** Show the incident timeline

**Voiceover:**
> "FaultIQ. Graph-based root cause analysis. Open source. Try it at github.com/theprodsde/faultiq"

---

## End Screen (2:00)

**Visual:** Repo URL + demo GIF

**Text:**
- github.com/theprodsde/faultiq
- One-click demo: http://localhost:4001/dashboard/demo
- Apache 2.0 — free for commercial use

---

## Recording Tips

1. **Speed up the docker-compose build** — record that part separately or skip it
2. **Use the demo page** — no terminal commands needed for the fault injection
3. **Highlight the graph** — zoom in on the Cytoscape visualization when the fault hits
4. **Show the confidence bar** — it fills up as evidence accumulates
5. **Keep it under 2 minutes** — attention spans are short
