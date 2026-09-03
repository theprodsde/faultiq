# FaultIQ Environment Configuration Guide

## Overview

This document explains all environment variables used by FaultIQ services and how to configure them for different deployment environments.

---

## Environment Files

### Development
- **File**: `.env.local` (git-ignored)
- **Usage**: `npm run dev` and `docker-compose up`
- **Purpose**: Local development with localhost URLs

### Production
- **File**: `.env.production`
- **Usage**: Production deployment with external URLs
- **Purpose**: Production environment with real domain names

### Example Template
- **File**: `.env.example`
- **Usage**: Template for creating new environment files
- **Purpose**: Documentation and validation

---

## Configuration by Deployment Target

### 1. Development (Local Docker Compose)

```bash
# Copy and customize
cp .env.example .env.local

# Set localhost URLs
NEXT_PUBLIC_API_GATEWAY_URL=http://localhost:8080/api/v1
NEXT_PUBLIC_AUTH_URL=http://localhost:8081
KEYCLOAK_ADMIN_PASSWORD=admin
POSTGRES_PASSWORD=postgres
```

**Testing Credentials** (after Keycloak realm imported):
```
Username: super              Password: superpass             Role: super_admin
Username: org-admin          Password: orgadminpass          Role: tenant_admin
Username: project-admin      Password: projadminpass         Role: project_admin
Username: analyst            Password: analystpass           Role: analyst
Username: viewer             Password: viewerpass            Role: viewer
Username: signal-publisher   Password: signalpublisherpass   Role: signal_publisher
```

### 2. Docker Compose Deployment

**File**: `docker-compose.yml` (already configured)

All services are configured to use service-to-service DNS names:
- `postgres:5432` (PostgreSQL)
- `neo4j:7687` (Neo4j)
- `redis:6379` (Redis)
- `keycloak:8080` (Keycloak)
- `api-gateway:8080` (API Gateway)

**Frontend URLs in docker-compose**:
```yaml
environment:
  - NEXT_PUBLIC_API_GATEWAY_URL=http://api-gateway:8080/api/v1
  - NEXT_PUBLIC_AUTH_URL=http://localhost:8081  # Browser access
  - NEXT_PUBLIC_KEYCLOAK_REDIRECT_URI=http://localhost:4001
```

### 3. Kubernetes Production Deployment

**Storage**: K8s ConfigMaps and Secrets
```bash
# Create production ConfigMap
kubectl create configmap faultiq-config \
  --from-file=.env.production

# Create Secrets (sensitive data)
kubectl create secret generic faultiq-secrets \
  --from-literal=POSTGRES_PASSWORD=$(openssl rand -base64 32) \
  --from-literal=KEYCLOAK_ADMIN_PASSWORD=$(openssl rand -base64 32)
```

**Pod Environment**:
```yaml
env:
  - name: POSTGRES_PASSWORD
    valueFrom:
      secretKeyRef:
        name: faultiq-secrets
        key: POSTGRES_PASSWORD
  - name: KEYCLOAK_ADMIN_PASSWORD
    valueFrom:
      secretKeyRef:
        name: faultiq-secrets
        key: KEYCLOAK_ADMIN_PASSWORD
```

---

## Environment Variables Reference

### Database Configuration

| Variable | Service | Required | Example | Notes |
|----------|---------|----------|---------|-------|
| `PG_DSN` | detection-engine, onboarding-worker | ✅ | `postgres://user:pass@host:5432/db` | Full connection string |
| `POSTGRES_PASSWORD` | PostgreSQL, db-migrate | ✅ | `postgres` | Database password |
| `NEO4J_USER` | graph-manager | ✅ | `neo4j` | Neo4j username |
| `NEO4J_PASS` | graph-manager | ✅ | `testpass123` | Neo4j password |
| `NEO4J_URI` | graph-manager, detection-engine | ✅ | `neo4j://neo4j:7687` | Bolt connection URI |
| `REDIS_ADDR` | signal-ingestion, detection-engine | ✅ | `redis:6379` | Redis host:port |

### Keycloak Authentication

| Variable | Service | Required | Example | Notes |
|----------|---------|----------|---------|-------|
| `KEYCLOAK_ADMIN_USERNAME` | keycloak | ✅ | `admin` | For admin console |
| `KEYCLOAK_ADMIN_PASSWORD` | keycloak | ✅ | `admin` | **CHANGE IN PRODUCTION** |
| `KEYCLOAK_REALM` | all | ✅ | `faultiq` | Realm name |
| `JWKS_URL` | api-gateway | ✅ | `http://keycloak:8080/realms/...` | JWT key source |
| `JWKS_REFRESH_INTERVAL` | api-gateway | ⚠️ | `1m` or `5m` | Prod: increase to 5m |
| `AUTH_MODE` | api-gateway | ✅ | `jwks` | Authentication mode |

### Frontend URLs (Client-Side, NEXT_PUBLIC_)

| Variable | Browser Accessible | Production Example |
|----------|-------------------|-------------------|
| `NEXT_PUBLIC_API_GATEWAY_URL` | ✅ | `https://api.faultiq.your-domain.com/api/v1` |
| `NEXT_PUBLIC_AUTH_URL` | ✅ | `https://auth.faultiq.your-domain.com` |
| `NEXT_PUBLIC_KEYCLOAK_REDIRECT_URI` | ✅ | `https://app.faultiq.your-domain.com` |
| `NEXT_PUBLIC_WEBSOCKET_URL` | ✅ | `wss://api.faultiq.your-domain.com/signals` |
| `NEXT_PUBLIC_KEYCLOAK_CLIENT_ID` | - | `faultiq-ui` |
| `NEXT_PUBLIC_KEYCLOAK_REALM` | - | `faultiq` |

### Service Ports

| Variable | Service | Default | Dev | Production |
|----------|---------|---------|-----|------------|
| `API_GATEWAY_PORT` | api-gateway | 8080 | 8080 | 8080 |
| `INGESTION_PORT` | signal-ingestion | 8085 | 8085 | 8085 |
| `GRAPH_MANAGER_ADDR` | graph-manager | `:8086` | `:8086` | `:8086` |
| `DETECTION_ENGINE_PORT` | detection-engine | 8084 | 8084 | 8084 |

---

## Startup & Testing Checklist

### 1. Start Services
```bash
# Development
docker-compose up

# Wait for health checks
sleep 10

# Verify all services are healthy
docker-compose ps
```

### 2. Test Keycloak Login

**Via Browser**:
```
1. Open http://localhost:4001
2. Click "Login"
3. Enter credentials:
   - Username: super
   - Password: superpass
4. Accept consent
5. Should redirect to dashboard
```

**Via API**:
```bash
# Get token
curl -X POST http://localhost:8081/realms/faultiq/protocol/openid-connect/token \
  -d 'client_id=faultiq-ui' \
  -d 'username=super' \
  -d 'password=superpass' \
  -d 'grant_type=password'

# Use token to access API
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/projects
```

### 3. Verify User-Project Mappings

**Check Token Claims**:
```bash
# Decode JWT (use jwt.io)
# Should see in token:
{
  "tenant": "acme-corp",
  "projects": "payments-platform,orders-platform",
  "roles": ["super_admin"]
}
```

### 4. Test Different Roles

**Log in as each user**:
```bash
# Analyst (read incidents)
Username: analyst
Password: analystpass

# Viewer (read-only)
Username: viewer
Password: viewerpass

# Signal Publisher (post metrics)
Username: signal-publisher
Password: signalpublisherpass
```

---

## Configuration for Different Users

### Super Admin (Full Access)
```json
{
  "username": "super",
  "role": "super_admin",
  "projects": "all",
  "permissions": [
    "create_tenant",
    "manage_users",
    "manage_all_projects",
    "view_all_incidents"
  ]
}
```

### Organization Admin (Tenant-Level)
```json
{
  "username": "org-admin",
  "role": "tenant_admin",
  "tenant": "acme-corp",
  "projects": "payments-platform,orders-platform",
  "permissions": [
    "create_project",
    "manage_team",
    "view_tenant_incidents"
  ]
}
```

### Project Admin (Project-Level)
```json
{
  "username": "project-admin",
  "role": "project_admin",
  "project": "payments-platform",
  "permissions": [
    "manage_graph",
    "manage_incidents",
    "view_recommendations"
  ]
}
```

### Analyst (View & Analyze)
```json
{
  "username": "analyst",
  "role": "analyst",
  "projects": "payments-platform,orders-platform",
  "permissions": [
    "view_incidents",
    "view_graphs",
    "submit_feedback"
  ]
}
```

### Viewer (Read-Only)
```json
{
  "username": "viewer",
  "role": "viewer",
  "project": "payments-platform",
  "permissions": [
    "view_incidents",
    "view_graphs"
  ]
}
```

---

## Environment Variable Validation

### At Startup

Services validate required environment variables:

```go
// Example from api-gateway
func validateConfig() {
  required := []string{
    "JWKS_URL",
    "NEO4J_URI",
    "PG_DSN",
    "REDIS_ADDR",
  }
  
  for _, v := range required {
    if os.Getenv(v) == "" {
      log.Fatalf("Missing required env var: %s", v)
    }
  }
}
```

### Pre-Deployment Checklist

```bash
# Verify all required vars are set
required_vars=(
  "POSTGRES_PASSWORD"
  "NEO4J_PASSWORD"
  "KEYCLOAK_ADMIN_PASSWORD"
  "NEXT_PUBLIC_API_GATEWAY_URL"
  "NEXT_PUBLIC_AUTH_URL"
)

for var in "${required_vars[@]}"; do
  if [ -z "${!var}" ]; then
    echo "ERROR: Missing $var"
    exit 1
  fi
done
```

---

## Secrets Management

### Development (NOT FOR PRODUCTION)
```bash
# Store in .env.local (git-ignored)
POSTGRES_PASSWORD=postgres
KEYCLOAK_ADMIN_PASSWORD=admin
```

### Production (Recommended)
```bash
# Use Kubernetes Secrets
kubectl create secret generic faultiq-secrets \
  --from-literal=POSTGRES_PASSWORD=$(openssl rand -base64 32) \
  --from-literal=KEYCLOAK_ADMIN_PASSWORD=$(openssl rand -base64 32) \
  --from-literal=NEO4J_PASSWORD=$(openssl rand -base64 32)

# Use Vault
vault kv put secret/faultiq/postgres password=$(openssl rand -base64 32)
vault kv put secret/faultiq/keycloak password=$(openssl rand -base64 32)
```

---

## Troubleshooting

### Issue: "Invalid JWT signature"
**Cause**: JWKS URL is stale or unreachable
**Fix**:
```bash
# Check JWKS_REFRESH_INTERVAL
echo $JWKS_REFRESH_INTERVAL  # Should be 1m or more

# Verify Keycloak is accessible
curl http://keycloak:8080/realms/faultiq/protocol/openid-connect/certs
```

### Issue: "User has no projects"
**Cause**: `projects` attribute not in token
**Fix**:
1. Verify `projects` protocol mapper in Keycloak realm.json
2. Check user `attributes` section includes `projects`
3. Re-import realm after fixing

### Issue: "Connection refused" to PostgreSQL
**Cause**: Wrong `PG_DSN` or service not running
**Fix**:
```bash
# Check service is running
docker ps | grep postgres

# Verify DSN format
echo $PG_DSN
# Should be: postgres://user:pass@host:port/db

# Test connection
psql $PG_DSN
```

---

## Additional Resources

- [Keycloak Configuration](../docs/KEYCLOAK_SETUP.md)
- [Docker Compose Reference](https://docs.docker.com/compose/)
- [Environment Best Practices](../docs/ENVIRONMENT_BEST_PRACTICES.md)
