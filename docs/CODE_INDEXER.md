# Code Indexer — How It Works

Connects the runtime fault layer (service signals, SOP phases) to the code layer (git commits, functions).

---

## Lifecycle per Repository

```
Trigger (cron every 60min / CI webhook on push / on-demand API)
  │
  ▼
git clone --depth=50 repo  →  /tmp/ci-{serviceId}/     ← shallow, ~2-50MB, seconds
  │
  ▼
git log <lastHash>..HEAD --name-only                    ← only NEW commits (incremental)
  │                                                       first run: last 50 commits
  │                                                       subsequent: only since last run
  ▼
For each changed file:
  ├── Go:         match "func Name(" or "func (recv) Name("
  ├── TypeScript: match "function foo(" or "const foo = ("
  ├── Python:     match "def foo(" or "async def foo("
  ├── Java/Kotlin: match visibility modifier + return type + name(
  └── Rust:       match "fn foo(" or "pub fn foo("
  │
  ▼
Write to Neo4j via graph-manager API:
  ├── GitCommit node  {hash, message, author, timestamp}
  ├── CodeFunction node  {name, file, line, serviceId}
  ├── IMPLEMENTS edge:  CodeFunction → Service node
  └── CHANGED_IN edge:  CodeFunction → GitCommit
  │
  ▼
UPDATE code_index_state SET last_commit_hash = <newest>  ← Postgres (incremental state)
  │
  ▼
rm -rf /tmp/ci-{serviceId}/                              ← disk freed immediately
```

**Disk used only during analysis** — seconds to minutes.
After `rm -rf`, zero disk usage until next run.

---

## Incremental Design

```
Run 1 (first time):
  lastHash = ""
  → git log -n50  (50 commits)
  → process all 50
  → save lastHash = abc123

Run 2 (1 hour later):
  lastHash = abc123
  → git log abc123..HEAD  (maybe 3 new commits)
  → process only those 3
  → save lastHash = def456

Run N:
  Only ever processes commits newer than last run.
  Clone is freshly downloaded and deleted each time.
```

**Why clone fresh each time instead of keeping the clone?**

- Docker containers restart, disk is ephemeral — persistent clones get corrupted
- Shallow clone (depth=50) is tiny (2-50MB) and takes 1-5s
- Always clean, no stale state, no disk accumulation
- Multiple services indexed in parallel — temp dirs don't collide

---

## Trigger Mechanisms

### 1. Scheduled (default — every 60 minutes)

Configured via `INDEX_INTERVAL_MINUTES=60` in docker-compose.
Good for background indexing with minimal overhead.

### 2. CI/CD Webhook (recommended — fires on every push)

Add to your GitHub Actions / GitLab CI:

```yaml
# .github/workflows/deploy.yml
- name: Trigger FaultIQ code index
  run: |
    curl -sf -X POST http://your-faultiq-host:8092/index/service/svc_payment_api
  continue-on-error: true   # don't fail deploy if indexer is down
```

This fires immediately after deploy — by the time a fault is detected, code context is already indexed.

### 3. On-demand (via API)

```bash
# Re-index ALL services now
curl -X POST http://localhost:8092/index

# Re-index one specific service
curl -X POST http://localhost:8092/index/service/svc_ledger_service
```

---

## Monolith vs Microservices

### Microservices (10 services, 10 repos)

Each service has its own repo — full repo maps to one service:

```yaml
services:
  - id: svc_payment_api
    repo: https://github.com/org/payment-service
    # codePath not needed — whole repo belongs to this service
```

### Monolith (1 repo, many logical services)

One repo, scoped by directory:

```yaml
services:
  - id: svc_payment_module
    repo: https://github.com/org/monolith   # same repo for all
    codePath: src/payments/                  # scope to this subdirectory

  - id: svc_ledger_module
    repo: https://github.com/org/monolith   # same repo
    codePath: src/ledger/                    # different scope
```

The code-indexer clones once per unique repo and re-uses it across all services sharing that repo (in the same indexing run), then deletes.

---

## Private Repos

For private GitHub/GitLab repos, mount an SSH key or personal access token:

### GitHub token (HTTPS)

In docker-compose.yml, set the repo URL with credentials:
```yaml
# In service-map.yaml:
repo: https://TOKEN@github.com/org/private-repo
```

Or use a Git credential helper via environment variable (safer — no token in YAML):
```yaml
environment:
  - GIT_CREDENTIALS=https://USERNAME:TOKEN@github.com
```

### SSH key (deploy key)

```yaml
code-indexer:
  environment:
    - GIT_SSH_COMMAND=ssh -i /etc/ssh/deploy_key -o StrictHostKeyChecking=no
  volumes:
    - ./secrets/deploy_key:/etc/ssh/deploy_key:ro
```

---

## What Gets Written to Neo4j

After indexing `payment-service`, the graph has:

```
Service node: svc_payment_api
      ↑ IMPLEMENTS (from)
CodeFunction: processPayment  [payments/handler.go:42]
      ↓ CHANGED_IN
GitCommit: abc123  "fix: retry logic"  john@company.com  2h ago
```

When detection-engine finds `svc_payment_api` as root cause →
`GET /api/v1/projects/proj-payments/services/svc_payment_api/code-context` returns:

```json
{
  "serviceId": "svc_payment_api",
  "repoUrl": "https://github.com/org/payment-service",
  "recentCommits": [
    {
      "hash": "abc123",
      "message": "fix: retry logic for DB timeout",
      "author": "john@company.com",
      "timestamp": "2026-09-01T02:00:00Z",
      "changedFiles": ["payments/handler.go", "payments/db.go"]
    }
  ],
  "suspectFunctions": ["processPayment (payments/handler.go)", "retryQuery (payments/db.go)"],
  "summary": "2 recent commits indexed — suspect functions highlighted"
}
```

This appears in the **Code Context** panel in the incident detail page, above the RCA candidates.

---

## Disk Usage Summary

| Phase | Disk used | Duration |
|-------|-----------|----------|
| Idle (between runs) | 0 bytes | 23h 59m / day |
| Cloning (depth=50) | 2–50 MB | 1–5 seconds |
| Analysis | 2–50 MB | 1–30 seconds |
| After cleanup | 0 bytes | Immediate |

Per service per run: peak ~50MB, then freed. 10 services in parallel: peak ~500MB, then freed.
