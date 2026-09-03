# Service Map Format
## How to onboard your project into FaultIQ

The `service-map.yaml` is the single config file that tells FaultIQ:
1. What services exist in your project (graph topology)
2. Where to poll each service for health data (autonomous fault detection)

---

## Full Schema

```yaml
version: "1"                     # always "1" for now

projects:
  - id: <string>                 # unique project ID — must match a Postgres projects.id
    name: <string>               # display name
    namespace: <string>          # Neo4j namespace — format: "<slug>:<environment>"
    tenant: <string>             # tenant ID from Keycloak JWT
    environment: <string>        # prod | staging | dev

    services:
      - id: <string>             # unique service ID within this project
        name: <string>           # display name shown in UI
        type: <NodeType>         # SERVICE | DATABASE | QUEUE | GATEWAY | EXTERNAL
        healthUrl: <string>      # HTTP URL to poll — must return JSON (see Health Response below)
        pollIntervalSeconds: 30  # optional, default 30
        timeoutSeconds: 5        # optional, default 5
        calls:                   # IDs of services this service calls (defines edges)
          - <service-id>
          - <service-id>
        tags:                    # optional — shown in UI
          - <string>
```

---

## Health Response Format

The health poller calls each `healthUrl` with `GET`. Your service should return:

```json
{
  "status": "ok",           # "ok" | "degraded" | "down"
  "latency_p95_ms": 45,     # optional — P95 latency in ms (used for latency-anomaly detection)
  "error_rate": 0.002        # optional — error rate 0.0–1.0 (used for high-error-rate detection)
}
```

If your service returns a simple `200 OK` with any body (or even no body), the poller treats it as `statusClass: "2xx"`. If it returns `5xx`, it's treated as failing.

The poller computes `statusClass` as:
- `2xx` → HTTP 200–299 and body `status != "down"` and `status != "degraded"`
- `degraded` → HTTP 200–299 and body `status = "degraded"`
- `5xx` → HTTP 500–599
- `timeout` → no response within `timeoutSeconds`
- `connection_error` → DNS/TCP failure

---

## Example: Payments Platform

```yaml
version: "1"

projects:
  - id: proj-payments-prod
    name: Payments Platform
    namespace: payments-platform:prod
    tenant: acme-corp
    environment: prod

    services:
      - id: svc_api_gateway
        name: api-gateway
        type: GATEWAY
        healthUrl: http://demo-services:8091/services/api-gateway/health
        calls: [svc_payment_api, svc_auth_service]
        tags: [public-facing, ingress]

      - id: svc_payment_api
        name: payment-api
        type: SERVICE
        healthUrl: http://demo-services:8091/services/payment-api/health
        calls: [svc_ledger_service, svc_auth_service, ext_stripe, db_redis_cache]

      - id: svc_auth_service
        name: auth-service
        type: SERVICE
        healthUrl: http://demo-services:8091/services/auth-service/health
        calls: [db_redis_cache]

      - id: svc_ledger_service
        name: ledger-service
        type: SERVICE
        healthUrl: http://demo-services:8091/services/ledger-service/health
        calls: [db_postgres, queue_ledger_events]

      - id: svc_settlement_worker
        name: settlement-worker
        type: SERVICE
        healthUrl: http://demo-services:8091/services/settlement-worker/health
        calls: [svc_ledger_service, queue_ledger_events]

      - id: db_postgres
        name: postgres-db
        type: DATABASE
        healthUrl: http://demo-services:8091/services/postgres-db/health

      - id: db_redis_cache
        name: redis-cache
        type: DATABASE
        healthUrl: http://demo-services:8091/services/redis-cache/health

      - id: queue_ledger_events
        name: ledger-events-topic
        type: QUEUE
        healthUrl: http://demo-services:8091/services/ledger-events-topic/health

      - id: ext_stripe
        name: stripe-api
        type: EXTERNAL
        healthUrl: http://demo-services:8091/services/stripe-api/health
```

---

## Example: Real Kubernetes Services

If your services are running in Kubernetes with a proper health endpoint:

```yaml
version: "1"

projects:
  - id: proj-my-app-prod
    name: My Application
    namespace: my-app:prod
    tenant: my-tenant
    environment: prod

    services:
      - id: svc_frontend
        name: frontend
        type: GATEWAY
        healthUrl: http://frontend.default.svc.cluster.local/health
        calls: [svc_api, svc_auth]

      - id: svc_api
        name: api-service
        type: SERVICE
        healthUrl: http://api.default.svc.cluster.local/health
        calls: [svc_db, svc_cache]

      - id: svc_auth
        name: auth-service
        type: SERVICE
        healthUrl: http://auth.default.svc.cluster.local/health
        calls: [svc_db]

      - id: svc_db
        name: database
        type: DATABASE
        healthUrl: http://db-exporter.default.svc.cluster.local/health

      - id: svc_cache
        name: redis
        type: DATABASE
        healthUrl: http://redis.default.svc.cluster.local/health
```

---

## Adding a New Project

1. Create the service-map entry
2. Create the project in FaultIQ UI or API
3. Trigger onboarding: `POST /api/v1/projects/{id}/onboarding/import` with `{"mode":"service-map"}`
4. The onboarding worker reads the YAML, writes nodes + edges to Neo4j
5. The health poller picks up the new services on its next config reload (or restart)
6. You're live — faults detected automatically from this point

---

## Pointing at Real Services (not demo-services)

Replace `http://demo-services:8091/services/<name>/health` with your actual health endpoint URLs.

The health poller runs **inside Docker Compose network**, so it can reach:
- Other Docker services by container name: `http://my-service:8080/health`  
- Host machine services via Docker host gateway: `http://host.docker.internal:8080/health`
- External URLs: `https://api.mycompany.com/health`

If running the health poller outside Docker (local Go process), use `localhost` for local services.

---

## Auth Patterns for Health Endpoints

### Pattern 1: Open health endpoint (recommended)

Keep `/health` open. This is the Kubernetes standard — probes, load balancers, and monitoring tools all need access without auth.

```yaml
- id: svc_payment_api
  healthUrl: http://payment-api:8080/health   # no auth needed
```

Your service should expose TWO separate endpoints:
- `/health` → open, no auth
- `/api/v1/*` → JWT/OAuth2 required

FaultIQ **never calls your business API endpoints** — only `/health`. The SOP playbook instructions are shown to the operator, not executed by FaultIQ.

---

### Pattern 2: Static API key for health endpoint

If your security policy requires auth even on `/health`, use a dedicated non-expiring health check key. **Never put a user JWT here** — it expires and breaks monitoring.

```yaml
- id: svc_payment_api
  healthUrl: http://payment-api:8080/health
  healthCheck:
    headers:
      X-Health-Check-Key: "${HEALTH_CHECK_KEY}"   # reference env var
```

Set `HEALTH_CHECK_KEY=your-secret` in `.env` or as a Docker secret. The `${VAR}` syntax is expanded automatically.

---

### Pattern 3: OAuth2 client credentials (enterprise)

For services that require a real OAuth2 Bearer token (e.g., Keycloak-protected health endpoints). FaultIQ fetches a fresh token before each check and caches it until near-expiry.

```yaml
- id: svc_payment_api
  healthUrl: http://payment-api:8080/health
  healthCheck:
    oauth2:
      tokenUrl: http://keycloak:8080/realms/myrealm/protocol/openid-connect/token
      clientId: faultiq-health-checker       # machine client, not a user
      clientSecret: "${OAUTH2_HEALTH_SECRET}"   # reference env var — never hardcode
      scope: "health:read"                       # minimal scope
```

Create a dedicated Keycloak service account with only `health:read` permissions. Add to `.env`:
```
OAUTH2_HEALTH_SECRET=your-client-secret-here
```

---

### Pattern 4: HTTPS with self-signed certs

```yaml
- id: svc_internal_api
  healthUrl: https://internal-api:8443/health
  healthCheck:
    tlsSkipVerify: true   # only for internal services with your own CA
```

---

### Pattern 5: TCP check (databases — no HTTP auth needed)

Databases don't expose HTTP endpoints. FaultIQ just checks if the port is open:

```yaml
- id: db_postgres
  healthUrl: tcp://postgres:5432   # no auth at all
- id: cache_redis  
  healthUrl: tcp://redis:6379
```

---

### Summary

| Your setup | FaultIQ config |
|-----------|-----------------|
| Open `/health` | Just `healthUrl:` |
| Static API key | `healthCheck.headers.X-Health-Key` + `${ENV_VAR}` |
| OAuth2 required | `healthCheck.oauth2.tokenUrl` + `clientId` + `${SECRET}` |
| Self-signed TLS | `healthCheck.tlsSkipVerify: true` |
| Database (TCP) | `healthUrl: tcp://host:port` |
