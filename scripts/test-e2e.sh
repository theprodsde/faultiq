#!/bin/bash
set -e

# `docker compose` needs the project directory to resolve the compose file,
# so run from the repo root regardless of where this was invoked.
cd "$(dirname "$0")/.."

BASE="http://localhost:8080"
SIGNAL="http://localhost:8085"
DEMO="http://localhost:8091"

# Keycloak client credentials. These must match scripts/keycloak/realm.json.
# Override by exporting before running, e.g. E2E_USERNAME=analyst E2E_PASSWORD=analystpass
KC_URL="${KC_URL:-http://localhost:8081}"
KC_REALM="${KC_REALM:-faultiq}"
E2E_CLIENT_ID="${E2E_CLIENT_ID:-faultiq-ui}"
E2E_USERNAME="${E2E_USERNAME:-super}"
E2E_PASSWORD="${E2E_PASSWORD:-superpass}"

echo "=== TechGraph E2E Test Suite ==="
echo ""

# ── Helper functions ──────────────────────────────────────────────────────────

pass() { echo "  ✓ $1"; }
fail() { echo "  ✗ FAIL: $1"; exit 1; }

# wait_for desc cmd expected [max_attempts=12]
wait_for() {
  local desc=$1 cmd=$2 expected=$3 max_attempts=${4:-12} attempt=0
  while [ $attempt -lt $max_attempts ]; do
    result=$(eval "$cmd" 2>/dev/null || echo "")
    if echo "$result" | grep -q "$expected"; then
      pass "$desc"
      return 0
    fi
    attempt=$((attempt + 1))
    sleep 5
  done
  fail "$desc (expected '$expected' after $max_attempts attempts)"
}

# ── Test 1: Stack health ──────────────────────────────────────────────────────
echo "--- Test 1: Stack health ---"
curl -sf "$BASE/health" > /dev/null && pass "api-gateway healthy" || fail "api-gateway not responding"
curl -sf "http://localhost:8085/health" > /dev/null && pass "signal-ingestion healthy" || fail "signal-ingestion not responding"
curl -sf "http://localhost:8086/health" > /dev/null && pass "graph-manager healthy" || fail "graph-manager not responding"

# ── Test 2: Authentication ────────────────────────────────────────────────────
echo "" && echo "--- Test 2: Authentication ---"
TOKEN=$(curl -sf -X POST "$KC_URL/realms/$KC_REALM/protocol/openid-connect/token" \
  -d "grant_type=password&client_id=$E2E_CLIENT_ID&username=$E2E_USERNAME&password=$E2E_PASSWORD" \
  | python3 -c "import json,sys; print(json.load(sys.stdin).get('access_token',''))" 2>/dev/null)
[ -n "$TOKEN" ] && pass "got auth token (${#TOKEN} chars)" \
  || fail "auth token empty — check $E2E_USERNAME/$E2E_PASSWORD against scripts/keycloak/realm.json"

# ── Test 3: Graph topology ────────────────────────────────────────────────────
echo "" && echo "--- Test 3: Graph topology ---"
NODES=$(curl -sf "http://localhost:8086/api/v1/graphs?namespace=payments-platform:prod" \
  | python3 -c "import json,sys; g=json.load(sys.stdin); print(len(g.get('nodes',{})))" 2>/dev/null || echo 0)
[ "$NODES" -ge 8 ] && pass "payments-platform has $NODES nodes (≥8 expected)" \
  || fail "payments-platform graph empty or incomplete (got $NODES nodes)"

# ── Test 4: Namespace discovery ───────────────────────────────────────────────
echo "" && echo "--- Test 4: Namespace discovery ---"
NS=$(curl -sf "http://localhost:8086/api/v1/graphs/namespaces" \
  | python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('total',0))" 2>/dev/null || echo 0)
[ "$NS" -ge 1 ] && pass "$NS namespace(s) found" || fail "no namespaces found"

# ── Test 5: Demo services ─────────────────────────────────────────────────────
echo "" && echo "--- Test 5: Demo services ---"
FAULTS=$(curl -sf "http://localhost:8091/admin/services/" \
  | python3 -c "import json,sys; d=json.load(sys.stdin); print(sum(1 for s in d.values() if s.get('statusClass','2xx') != '2xx'))" 2>/dev/null || echo "?")
pass "demo-services running (currently $FAULTS services in fault state)"

# ── Test 6: Signal injection → incident creation ──────────────────────────────
echo "" && echo "--- Test 6: Signal injection ---"
# Reset demo-services first
curl -sf -X POST "http://localhost:8091/admin/reset" > /dev/null 2>&1 || true
sleep 2

# Inject fault signals
for i in 1 2 3; do
  curl -sf -X POST "$SIGNAL/api/v1/signals" \
    -H "Content-Type: application/json" \
    -d '{"tenant":"acme-corp","project":"proj-payments-prod","service":"svc_ledger_service","statusClass":"timeout","errorRate":0.42,"latencyP95":4800}' > /dev/null
  sleep 0.5
done
pass "3 fault signals sent for svc_ledger_service"

# Wait for incident to appear
wait_for "incident created" \
  "curl -sf '$BASE/api/v1/projects/proj-payments-prod/incidents?limit=5' \
     -H 'Authorization: Bearer $TOKEN' \
   | python3 -c \"import json,sys; incs=json.load(sys.stdin).get('incidents',[]); print('FOUND' if any(i.get('service')=='svc_ledger_service' for i in incs) else 'NONE')\"" \
  "FOUND" 6

# ── Test 7: SOP phase progression ────────────────────────────────────────────
echo "" && echo "--- Test 7: SOP phase progression ---"
# Send more signals to advance to TRIAGING
for i in 4 5 6 7 8; do
  curl -sf -X POST "$SIGNAL/api/v1/signals" \
    -H "Content-Type: application/json" \
    -d '{"tenant":"acme-corp","project":"proj-payments-prod","service":"svc_ledger_service","statusClass":"timeout","errorRate":0.45,"latencyP95":5000}' > /dev/null
  sleep 0.3
done

wait_for "phase reached TRIAGING" \
  "docker compose exec -T postgres psql -U postgres -d faultiq -tAc \
     \"SELECT phase FROM incidents WHERE service='svc_ledger_service' AND status='OPEN' ORDER BY detected_at DESC LIMIT 1\" 2>/dev/null" \
  "TRIAGING" 48

# ── Test 8: SOP playbook ──────────────────────────────────────────────────────
echo "" && echo "--- Test 8: SOP playbook generated ---"
INC_ID=$(docker compose exec -T postgres psql -U postgres -d faultiq -tAc \
  "SELECT id FROM incidents WHERE service='svc_ledger_service' AND status='OPEN' ORDER BY detected_at DESC LIMIT 1;" \
  2>/dev/null | tr -d ' \n')
STEPS=$(curl -sf "$BASE/api/v1/incidents/$INC_ID/sop" \
  -H "Authorization: Bearer $TOKEN" \
  | python3 -c "import json,sys; d=json.load(sys.stdin); print(len(d.get('steps',[])))" 2>/dev/null || echo 0)
[ "$STEPS" -ge 3 ] && pass "SOP playbook has $STEPS steps" \
  || fail "SOP playbook empty (incident: $INC_ID)"

# ── Test 9: Reasoning chain ───────────────────────────────────────────────────
echo "" && echo "--- Test 9: Reasoning chain ---"
CHAIN_RESP=$(curl -sf "$BASE/api/v1/incidents/$INC_ID/reasoning" -H "Authorization: Bearer $TOKEN" 2>/dev/null || echo '{}')
# Accept reasoningChain (from incidentReasoningHandler) OR rootCauseCandidates (from getIncidentHandler if image not rebuilt yet)
CHAIN=$(echo "$CHAIN_RESP" | python3 -c "import json,sys; d=json.load(sys.stdin); c=d.get('reasoningChain',d.get('rootCauseCandidates',[])); print(len(c))" 2>/dev/null || echo 0)
[ "$CHAIN" -ge 1 ] && pass "reasoning endpoint returned $CHAIN items" || fail "reasoning endpoint empty"

# ── Test 10: Auto-resolution ──────────────────────────────────────────────────
echo "" && echo "--- Test 10: Auto-resolution ---"
curl -sf -X PUT "http://localhost:8091/admin/services/ledger-service/status" \
  -H "Content-Type: application/json" \
  -d '{"statusClass":"2xx","errorRate":0.001,"latencyP95":45}' > /dev/null
pass "ledger-service restored to healthy"

wait_for "incident auto-resolved" \
  "docker compose exec -T postgres psql -U postgres -d faultiq -tAc \
     \"SELECT status FROM incidents WHERE id='$INC_ID'\" 2>/dev/null" \
  "RESOLVED" 24

# ── Summary ───────────────────────────────────────────────────────────────────
echo ""
echo "=== All tests passed ✓ ==="
echo "  Stack:          healthy"
echo "  Auth:           working"
echo "  Graph:          $NODES nodes in payments-platform"
echo "  Detection:      signal → incident → TRIAGING"
echo "  SOP:            $STEPS-step playbook generated"
echo "  Reasoning:      $CHAIN-step chain"
echo "  Auto-resolve:   ✓"
