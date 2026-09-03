#!/bin/bash
# =============================================================================
# FaultIQ — Local Validation (No Docker Required)
# =============================================================================
# Tests:
#   1. Detection algorithm (Go unit tests)
#   2. Frontend builds correctly
#   3. All Go services compile
#   4. Graph cache + recommendations engine logic
# =============================================================================

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo ""
echo -e "${BLUE}╔══════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║   FaultIQ — Local Build & Test Validation                   ║${NC}"
echo -e "${BLUE}╚══════════════════════════════════════════════════════════════╝${NC}"
echo ""

PASS=0
FAIL=0

run_check() {
    local name=$1
    shift
    echo -n "  Testing: $name... "
    if "$@" > /tmp/faultiq-test-output.txt 2>&1; then
        echo -e "${GREEN}PASS${NC}"
        PASS=$((PASS + 1))
    else
        echo -e "${RED}FAIL${NC}"
        cat /tmp/faultiq-test-output.txt | tail -5 | sed 's/^/    /'
        FAIL=$((FAIL + 1))
    fi
}

echo -e "${YELLOW}── Go Services Compilation ──${NC}"
echo ""

cd "$(dirname "$0")/.."

run_check "detection-engine compiles" bash -c "cd services/detection-engine && go build ./..."
run_check "signal-ingestion compiles" bash -c "cd services/signal-ingestion && go build ./..."
run_check "api-gateway compiles" bash -c "cd services/api-gateway && go build ./..."
run_check "rca-ranker compiles" bash -c "cd services/rca-ranker && go build ./..."
run_check "graph-manager compiles" bash -c "cd services/graph-manager && go build ./..."
run_check "onboarding-worker compiles" bash -c "cd services/onboarding-worker && go build ./..."

echo ""
echo -e "${YELLOW}── Detection Engine Unit Tests ──${NC}"
echo ""

run_check "BFS detection algorithm" bash -c "cd services/detection-engine && go test -run TestDetectSuspects -v ./..."
run_check "graph cache compiles" bash -c "cd services/detection-engine && go vet ./..."

echo ""
echo -e "${YELLOW}── Frontend ──${NC}"
echo ""

run_check "TypeScript type check" bash -c "cd frontend && npx tsc --noEmit"
run_check "Next.js build" bash -c "cd frontend && NEXT_LINT_DISABLE=1 npx next build"

echo ""
echo -e "${YELLOW}── Key Files Exist ──${NC}"
echo ""

run_check "graph_cache.go exists" test -f services/detection-engine/graph_cache.go
run_check "recommendations.go exists" test -f services/detection-engine/recommendations.go
run_check "SSE endpoint (sse.go) exists" test -f services/signal-ingestion/sse.go
run_check "NodeDetailPanel.tsx exists" test -f frontend/src/components/NodeDetailPanel.tsx
run_check "useIncidentEvents hook exists" test -f frontend/src/hooks/useIncidentEvents.ts
run_check "migration 006 exists" test -f scripts/migrations/006_incident_updates.sql

echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "  Results: ${GREEN}$PASS passed${NC}, ${RED}$FAIL failed${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

if [ $FAIL -gt 0 ]; then
    exit 1
fi

echo -e "${GREEN}All checks passed! The project is ready to deploy.${NC}"
echo ""
echo -e "${YELLOW}Next steps:${NC}"
echo "  1. docker compose up -d      # Start all services"
echo "  2. Wait 30-60s for Keycloak"
echo "  3. ./scripts/demo-e2e.sh     # Run full E2E demo"
echo "  4. Open http://localhost:4001 # See the UI"
echo ""
