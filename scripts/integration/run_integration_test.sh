#!/usr/bin/env bash
set -euo pipefail

# Simple integration test:
# 1. Start compose stack
# 2. Seed neo4j with a small graph (svc-a -> svc-b with svc-b 5xx)
# 3. POST a signal for svc-a to /api/v1/signals
# 4. Wait up to 180s for an incident record in Postgres

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
COMPOSE_FILE="$ROOT_DIR/docker-compose.yml"

echo "Starting integration stack..."
docker-compose -f "$COMPOSE_FILE" up -d neo4j postgres redis graph-manager signal-ingestion detection-engine db-migrate

echo "Waiting for Postgres to be ready..."
# wait for postgres
until docker-compose -f "$COMPOSE_FILE" exec -T postgres pg_isready -U postgres > /dev/null 2>&1; do
  echo "waiting for postgres..."
  sleep 1
done

# wait briefly for graph-manager to be ready
echo "Waiting for graph-manager to be ready..."
# simple sleep to allow services to come up; graph-manager readiness may vary
sleep 3

# Seed Neo4j directly using cypher-shell inside the neo4j container
# Read .env for NEO4J_AUTH if present (docker-compose uses .env by default)
if [ -f "$ROOT_DIR/.env" ]; then
  # shellcheck disable=SC1090
  set -a
  # shell will expand referenced vars like ${NEO4J_PASSWORD}
  . "$ROOT_DIR/.env"
  set +a
fi
# Derive password from NEO4J_AUTH if present (format neo4j/password), else use NEO4J_PASS or default
if [ -n "${NEO4J_AUTH:-}" ]; then
  NEO4J_PASS="${NEO4J_AUTH#*/}"
else
  NEO4J_PASS="${NEO4J_PASS:-test}"
fi
NAMESPACE="${NAMESPACE:-payments-platform:prod}"

echo "Seeding Neo4j with test graph..."
docker-compose -f "$COMPOSE_FILE" exec -T neo4j bin/cypher-shell -u neo4j -p "$NEO4J_PASS" <<CYPHER
CREATE (a:Service {id:'svc-a', namespace:'$NAMESPACE', statusClass:'2xx', latencyP95:10});
CREATE (b:Service {id:'svc-b', namespace:'$NAMESPACE', statusClass:'5xx', latencyP95:200});
CREATE (a)-[:CALLS]->(b);
CYPHER

# Post a signal to ingestion
echo "Posting signal to ingestion..."
curl -s -X POST http://localhost:8085/api/v1/signals -H 'Content-Type: application/json' -d '{"tenant":"t","project":"p","environment":"e","service":"svc-a","metric":"error","value":1.0}' || true

# Poll Postgres for incidents
echo "Waiting for incident in Postgres..."
END=180
START=$(date +%s)
while [ $(date +%s) -le $((START+END)) ]; do
  CNT=$(docker-compose -f "$COMPOSE_FILE" exec -T postgres psql -U postgres -d postgres -t -c "SELECT count(*) FROM incidents WHERE project_id = '$NAMESPACE';" | tr -d ' \n') || true
  CNT=${CNT:-0}
  echo "incident count=$CNT"
  if [ "$CNT" -ge "1" ]; then
    echo "Integration test passed: incident found"
    exit 0
  fi
  sleep 2
done

echo "Integration test failed: no incident found within timeout"
exit 2
