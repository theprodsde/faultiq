#!/usr/bin/env bash
set -euo pipefail

# Environment defaults
KEYCLOAK_HOST=${KEYCLOAK_HOST:-localhost:8081}
API_HOST=${API_HOST:-localhost:8080}
PG_DSN=${PG_DSN:-postgres://postgres:postgres@postgres:5432/postgres?sslmode=disable}

# get token
TOK=$(curl -s -X POST "http://${KEYCLOAK_HOST}/realms/faultiq/protocol/openid-connect/token" -d 'grant_type=password&client_id=faultiq-ui&username=super&password=superpass&scope=openid' | sed -n 's/.*"access_token":"\([^\"]*\)".*/\1/p')
if [ -z "$TOK" ]; then
  echo "failed to get token"
  exit 2
fi

# call onboarding
RESP=$(curl -s -X POST "http://${API_HOST}/api/v1/projects/onboarding/import" -H "Authorization: Bearer ${TOK}" -H "Content-Type: application/json" -d '{"project_id":"proj-e2e-ci","mode":"full","sources":{"items":[{"type":"openapi","url":"http://example.com"}]}}')
echo "api response: $RESP"
JOB_ID=$(echo "$RESP" | sed -n 's/.*"job_id":"\([^"]*\)".*/\1/p')
if [ -z "$JOB_ID" ]; then
  echo "no job id returned"
  exit 3
fi

# wait for Postgres entry
for i in {1..30}; do
  if docker compose exec -T postgres psql -U postgres -d postgres -c "select id, status from onboarding_jobs where id='${JOB_ID}';" | grep -q "${JOB_ID}"; then
    docker compose exec -T postgres psql -U postgres -d postgres -c "select id, status, created_at from onboarding_jobs where id='${JOB_ID}';"
    exit 0
  fi
  sleep 2
done

echo "job ${JOB_ID} not found in Postgres after timeout"
exit 4
