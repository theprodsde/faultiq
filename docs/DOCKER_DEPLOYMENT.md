# FaultIQ Docker Compose Deployment Guide

## Quick Start (5 Minutes)

### Prerequisites
- Docker & Docker Compose v2.0+
- 4GB free memory
- Ports 3000-9000 available

### Steps

```bash
# 1. Clone and enter directory
cd /path/to/FaultIQ

# 2. Create .env.local with correct Keycloak port
cp .env.example .env.local
# IMPORTANT: Update this line:
# NEXT_PUBLIC_AUTH_URL=http://localhost:8081  ← Matches docker-compose

# 3. Start all services
docker-compose up

# 4. Wait for health checks to pass (30-60 seconds)
docker-compose ps

# 5. Access applications
- Frontend:  http://localhost:4001
- API:       http://localhost:8080
- Keycloak:  http://localhost:8081
- Neo4j:     http://localhost:7474 (web)
```

### First Login

**Credentials** (from Keycloak realm.json):
- **Username**: `super`
- **Password**: `superpass`
- **Role**: Super Admin (full platform access)

---

## Docker Compose Architecture

### Service Topology

```
┌─────────────────────────────────────────────────────────────────┐
│                        Frontend Container                       │
│                   (http://localhost:4001)                        │
└────────────────────────┬──────────────────────────────────────┘
                         │
       ┌─────────────────┴─────────────────┐
       │                                   │
┌──────▼─────────────────┐      ┌─────────▼──────────────┐
│   API Gateway (8080)   │      │  Keycloak (8081)       │
│  - JWT Validation      │      │  - Token Service       │
│  - Request Routing     │      │  - User Management     │
└──┬───────────────────┬─┘      └────────────────────────┘
   │                  │
   │         ┌────────┴────────┐
   │         │                 │
┌──▼─────────▼────┐ ┌──────────▼──────────┐
│  PostgreSQL DB  │ │  Neo4j Graph DB    │
│    (5434)       │ │     (7687)         │
└─────────────────┘ └────────────────────┘
   │
┌──▼──────────────────────────────┐
│   Redis Cache (6379)            │
│   - Session cache               │
│   - Signal pub/sub              │
└─────────────────────────────────┘
```

### Service Details

| Service | Image | Port | Type | Status |
|---------|-------|------|------|--------|
| **frontend** | Node.js 20 (build) | 4001 | Container | ✅ |
| **api-gateway** | Go binary | 8080 | Container | ✅ |
| **keycloak** | Keycloak 21.1.1 | 8081 | Image | ✅ |
| **postgres** | PostgreSQL 16 | 5434 | Image | ✅ |
| **neo4j** | Neo4j 5.8 | 7687 | Image | ✅ |
| **redis** | Redis 7 | 6379 | Image | ✅ |
| **graph-manager** | Go binary | 8086 | Container | ✅ |
| **signal-ingestion** | Go binary | 8085 | Container | ✅ |
| **detection-engine** | Go binary | 8084 | Container | ✅ |
| **onboarding-worker** | Go binary | N/A | Container | ✅ |
| **db-migrate** | PostgreSQL 16 | N/A | Init Job | ✅ |

---

## Configuration

### Environment Variables

All services read from `.env` file. Key variables:

```bash
# Database URLs (service-to-service DNS names)
PG_DSN=postgres://postgres:postgres@postgres:5432/faultiq?sslmode=disable
NEO4J_URI=neo4j://neo4j:7687
REDIS_ADDR=redis:6379

# URLs accessible from services
GRAPH_MANAGER_URL=http://graph-manager:8086/api/v1/graphs

# Frontend environment (client-side, NEXT_PUBLIC_)
NEXT_PUBLIC_API_GATEWAY_URL=http://api-gateway:8080/api/v1
NEXT_PUBLIC_AUTH_URL=http://localhost:8081  ← Must match browser access!
NEXT_PUBLIC_KEYCLOAK_REDIRECT_URI=http://localhost:4001
```

### Port Mappings

```
Container Port → Host Port
─────────────────────────
4001 → 4001   (Frontend)
8080 → 8080   (API Gateway)
8081 → 8081   (Keycloak)
8084 → 8084   (Detection Engine)
8085 → 8085   (Signal Ingestion)
8086 → 8086   (Graph Manager)
5434 → 5432   (PostgreSQL)
7687 → 7687   (Neo4j Bolt)
7474 → 7474   (Neo4j Browser)
6379 → 6379   (Redis)
```

### Health Checks

All services have health checks configured:

```bash
# View health status
docker-compose ps

# Example output:
NAME                    STATUS
frontend                Up 2 min (healthy)
api-gateway             Up 2 min (healthy)
keycloak                Up 2 min (healthy)
postgres                Up 2 min (healthy)
neo4j                   Up 2 min (healthy)
redis                   Up 2 min (healthy)
```

---

## Data Persistence

### Volumes

```
faultiq_neo4j_data    → /data (Neo4j database files)
faultiq_postgres_data → /var/lib/postgresql/data (PostgreSQL)
faultiq_redis_data    → /data (Redis RDB snapshots)
```

### Backup & Restore

```bash
# Backup PostgreSQL
docker exec $(docker ps --filter "name=postgres" -q) \
  pg_dump -U postgres faultiq > backup-$(date +%s).sql

# Backup Neo4j
docker exec $(docker ps --filter "name=neo4j" -q) \
  neo4j-admin database dump faultiq > backup.dump

# Restore PostgreSQL
docker exec -i $(docker ps --filter "name=postgres" -q) \
  psql -U postgres faultiq < backup.sql

# Restore Neo4j
docker exec $(docker ps --filter "name=neo4j" -q) \
  neo4j-admin database load faultiq < backup.dump
```

---

## Common Operations

### Start Services

```bash
# Start all services
docker-compose up

# Start in background
docker-compose up -d

# Start specific service
docker-compose up api-gateway
```

### Stop Services

```bash
# Stop all services (keeps data)
docker-compose down

# Stop and remove volumes (DELETES DATA!)
docker-compose down -v

# Stop specific service
docker-compose stop api-gateway
```

### View Logs

```bash
# All services
docker-compose logs -f

# Specific service
docker-compose logs -f api-gateway

# Last 100 lines
docker-compose logs --tail=100 postgres

# With timestamps
docker-compose logs -f --timestamps
```

### Execute Commands

```bash
# PostgreSQL CLI
docker-compose exec postgres psql -U postgres -d faultiq

# Neo4j CLI
docker-compose exec neo4j cypher-shell -u neo4j -p testpass123

# Redis CLI
docker-compose exec redis redis-cli

# Keycloak Admin CLI
docker-compose exec keycloak /opt/keycloak/bin/kcadm.sh
```

### Check Service Health

```bash
# All services
docker-compose ps

# Specific service
docker-compose exec api-gateway curl http://localhost:8080/health

# Frontend
docker-compose exec frontend curl http://localhost:4001

# Database connectivity
docker-compose exec postgres pg_isready

# Redis connectivity
docker-compose exec redis redis-cli ping
```

---

## Troubleshooting

### Issue: Port Already in Use

```bash
# Find what's using port 8080
lsof -i :8080

# Kill process
kill -9 <PID>

# Or change port in docker-compose.yml:
# ports:
#   - "8081:8080"  ← Changed from 8080:8080
```

### Issue: Services Won't Start

```bash
# Check logs
docker-compose logs api-gateway

# Common causes:
# 1. Out of memory → Increase Docker resources
# 2. Port conflict → Free up ports
# 3. Missing env vars → Check .env file
# 4. Database not ready → Wait for health check
```

### Issue: Keycloak Login Fails

```bash
# Check realm.json is valid JSON
jq . scripts/keycloak/realm.json

# Verify Keycloak has imported realm
docker-compose logs keycloak | grep "imported"

# Test token endpoint
curl -X POST http://localhost:8081/realms/faultiq/protocol/openid-connect/token \
  -d 'client_id=faultiq-ui' \
  -d 'username=super' \
  -d 'password=superpass' \
  -d 'grant_type=password'
```

### Issue: API Gateway Can't Reach Services

```bash
# Check service is running
docker-compose ps graph-manager

# Check network connectivity between services
docker-compose exec api-gateway \
  curl http://graph-manager:8086/health

# Check DNS resolution
docker-compose exec api-gateway \
  nslookup neo4j
```

### Issue: Database Migration Failed

```bash
# Check migration logs
docker-compose logs db-migrate

# Re-run migrations
docker-compose up db-migrate --force-recreate

# Manual PostgreSQL access
docker-compose exec postgres psql -U postgres -d faultiq
\dt  ← List tables
\d incidents  ← Describe table
```

### Issue: Frontend Can't Connect to API

```bash
# Check frontend env vars
docker-compose exec frontend env | grep NEXT_PUBLIC

# Check API Gateway is accessible
curl http://localhost:8080/health

# Check frontend is accessible
curl http://localhost:4001

# Check browser console for errors
# (Open DevTools → Console)
```

---

## Performance Tuning

### Memory Configuration

For high-load environments, adjust container memory limits in docker-compose.yml:

```yaml
services:
  api-gateway:
    deploy:
      resources:
        limits:
          memory: 1G
        reservations:
          memory: 512M

  neo4j:
    environment:
      - NEO4J_dbms_memory_heap_maxSize=2G
      - NEO4J_dbms_memory_pagecache_size=2G
```

### Connection Pooling

Services already configured with connection pools:
- PostgreSQL: 10 connections per service
- Redis: Single connection + pub/sub
- Neo4j: Connection pool in bolt driver

### Caching

- Redis cache TTL: 300 seconds (5 minutes)
- Graph cache: Automatic invalidation on publish
- JWKS cache: 1 minute (5 minutes in prod)

---

## Monitoring

### Prometheus Metrics

Services expose metrics on port 9090 (future integration):

```bash
# View raw metrics
curl http://localhost:8080/metrics
```

### Log Aggregation

Collect all logs for analysis:

```bash
# Export logs
docker-compose logs > logs.txt

# Stream to file
docker-compose logs -f >> logs.txt &
```

### Health Dashboard

All services expose `/health` endpoint:

```bash
for service in api-gateway graph-manager detection-engine signal-ingestion; do
  echo "=== $service ==="
  curl -s http://localhost:$(docker-compose port $service 2>/dev/null | cut -d: -f2)/health
done
```

---

## Production Deployment

### Pre-deployment Checklist

- [ ] Update `.env.production` with real URLs and passwords
- [ ] Change all default passwords (Keycloak, PostgreSQL, etc.)
- [ ] Enable HTTPS with valid SSL certificates
- [ ] Set up persistent volume storage (K8s PVC or cloud storage)
- [ ] Configure backup schedule for databases
- [ ] Set resource limits (CPU, memory)
- [ ] Configure health check monitoring
- [ ] Set up log aggregation (ELK, Splunk, etc.)
- [ ] Enable authentication for Neo4j and PostgreSQL
- [ ] Rotate secrets regularly

### Kubernetes Migration

For production, migrate from Docker Compose to Kubernetes:

```bash
# Convert docker-compose to K8s manifests
kompose convert -f docker-compose.yml -o k8s/

# Or use existing Helm charts (future)
helm install faultiq ./charts/faultiq
```

---

## Cleanup

### Remove All Containers & Volumes

```bash
# WARNING: This deletes all data!
docker-compose down -v

# Remove images
docker-compose down -v --rmi all
```

### Selective Cleanup

```bash
# Remove unused volumes
docker volume prune

# Remove dangling images
docker image prune

# Full cleanup
docker system prune -a --volumes
```

---

## Additional Resources

- [Docker Compose Documentation](https://docs.docker.com/compose/)
- [Environment Variables Guide](./ENVIRONMENT_VARIABLES.md)
- [Keycloak Setup](./KEYCLOAK_SETUP.md)
- [Architecture Guide](./architecture.md)
