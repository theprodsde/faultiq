#!/usr/bin/env bash
set -euo pipefail
ROOT_DIR=$(cd "$(dirname "$0")/../.." && pwd)
cd "$ROOT_DIR"

echo "Bringing up postgres container..."
docker-compose up -d postgres db-migrate

echo "Waiting for Postgres to be ready..."
until docker-compose exec -T postgres pg_isready -U postgres -q; do
  sleep 1
done

INCIDENT_ID=${INCIDENT_ID:-rca-test-1}

echo "Seeding incident and rca_candidates..."
docker-compose exec -T postgres psql -U postgres -d postgres -c "INSERT INTO incidents (id, project_id) VALUES ('${INCIDENT_ID}', 'proj-1') ON CONFLICT (id) DO NOTHING;"
docker-compose exec -T postgres psql -U postgres -d postgres -c "INSERT INTO rca_candidates (incident_id, node_id, evidence) VALUES ('${INCIDENT_ID}','svc-b', '{\"evidence_count\":10,\"impact\":1.2}'), ('${INCIDENT_ID}','svc-c', '{\"evidence_count\":5,\"impact\":2.5}'), ('${INCIDENT_ID}','svc-d', '{\"evidence_count\":8,\"impact\":0.7}') ;"

echo "Running rca-ranker inside a Go container on the compose network..."
POSTGRES_CID=$(docker-compose ps -q postgres)
if [ -z "$POSTGRES_CID" ]; then
  echo "could not find postgres container id" >&2
  exit 1
fi
NETWORK=$(docker inspect -f '{{range $k,$v := .NetworkSettings.Networks}}{{println $k}}{{end}}' "$POSTGRES_CID" | head -n1)
if [ -z "$NETWORK" ]; then
  echo "could not determine docker network" >&2
  exit 1
fi
docker run --rm --network "$NETWORK" -v "$ROOT_DIR":/src -w /src/services/rca-ranker golang:1.24 sh -c "cd /src/services/rca-ranker && go mod tidy && PG_DSN='postgres://postgres:postgres@postgres:5432/postgres?sslmode=disable' go run . ${INCIDENT_ID}"

echo "Checking updated scores..."
docker-compose exec -T postgres psql -U postgres -d postgres -c "SELECT id, node_id, confidence, rank FROM rca_candidates WHERE incident_id='${INCIDENT_ID}' ORDER BY confidence DESC;"

echo "RC A Ranker integration completed."
