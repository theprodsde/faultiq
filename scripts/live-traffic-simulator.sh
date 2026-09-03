#!/bin/bash
# =============================================================================
# FaultIQ — Live Traffic Simulator
# =============================================================================
#
# Simulates a realistic distributed system with 15 services sending health
# signals continuously. Randomly injects realistic failure scenarios that
# cascade through the dependency graph — exactly like production outages.
#
# This makes the graph come alive:
#   - Nodes pulse green during normal operation
#   - Random failures turn nodes RED with cascading effects
#   - Failures auto-recover after a realistic duration
#   - You see real incidents, real RCA, real fix recommendations
#
# USAGE:
#   docker compose up -d       # start all services first
#   ./scripts/live-traffic-simulator.sh
#
# To stop: Ctrl+C
#
# =============================================================================

set -e

# ── Config ─────────────────────────────────────────────────────────────────────

API_GATEWAY="http://localhost:8080"
KEYCLOAK_URL="http://localhost:8081"
REALM="faultiq"
CLIENT_ID="faultiq-ui"
USERNAME="org-admin"
PASSWORD="orgadminpass"
TENANT="acme-corp"
PROJECT_ID="proj-payments-prod"

# Timing
HEALTHY_INTERVAL=5          # seconds between healthy heartbeats
FAILURE_CHECK_INTERVAL=10   # seconds between checking if we should inject a failure
FAILURE_PROBABILITY=25      # % chance of a failure per check (adjust for demo speed)
RECOVERY_MIN=20             # minimum seconds before auto-recovery
RECOVERY_MAX=60             # maximum seconds before auto-recovery

# ── Colors ─────────────────────────────────────────────────────────────────────

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
GRAY='\033[0;37m'
NC='\033[0m'

# ── Service Topology ───────────────────────────────────────────────────────────
# Defines the realistic distributed system with proper dependencies.
# When a service fails, its dependents may also degrade (realistic cascade).

declare -A SERVICE_TYPE
SERVICE_TYPE=(
    [svc_api_gateway]="GATEWAY"
    [svc_payment_api]="SERVICE"
    [svc_ledger_service]="SERVICE"
    [svc_auth_service]="SERVICE"
    [svc_settlement_worker]="SERVICE"
    [svc_notification_service]="SERVICE"
    [svc_fraud_detector]="SERVICE"
    [svc_compliance_check]="SERVICE"
    [db_postgres]="DATABASE"
    [db_redis_cache]="DATABASE"
    [queue_ledger_events]="QUEUE"
    [queue_notifications]="QUEUE"
    [ext_stripe]="EXTERNAL"
)

# Normal latency baselines (ms)
declare -A SERVICE_BASELINE_LATENCY
SERVICE_BASELINE_LATENCY=(
    [svc_api_gateway]=45
    [svc_payment_api]=120
    [svc_ledger_service]=95
    [svc_auth_service]=35
    [svc_settlement_worker]=500
    [svc_notification_service]=200
    [svc_fraud_detector]=280
    [svc_compliance_check]=150
    [db_postgres]=15
    [db_redis_cache]=3
    [queue_ledger_events]=50
    [queue_notifications]=40
    [ext_stripe]=450
)

# Services list for iteration
SERVICES=(
    svc_api_gateway
    svc_payment_api
    svc_ledger_service
    svc_auth_service
    svc_settlement_worker
    svc_notification_service
    svc_fraud_detector
    svc_compliance_check
    db_postgres
    db_redis_cache
    queue_ledger_events
    queue_notifications
    ext_stripe
)

# ── Failure Scenarios ──────────────────────────────────────────────────────────
# Each scenario is a realistic production incident with cascading effects.

# Scenario: Database primary goes down → cascading writes fail
SCENARIO_DB_PRIMARY=(
    "db_postgres:down:0.99:5000"
    "svc_ledger_service:5xx:0.75:3200"
    "svc_payment_api:5xx:0.60:2800"
    "svc_settlement_worker:timeout:0.45:4500"
    "svc_compliance_check:5xx:0.30:1800"
)

# Scenario: Redis cache failure → auth and payment service degrade
SCENARIO_REDIS_DOWN=(
    "db_redis_cache:connection_error:0.95:8000"
    "svc_auth_service:timeout:0.70:5200"
    "svc_fraud_detector:5xx:0.55:3500"
    "svc_payment_api:5xx:0.40:4100"
    "svc_api_gateway:5xx:0.25:2200"
)

# Scenario: External Stripe API timeout → payment processing fails
SCENARIO_STRIPE_TIMEOUT=(
    "ext_stripe:timeout:0.85:12000"
    "svc_payment_api:timeout:0.65:8500"
    "svc_api_gateway:5xx:0.30:4200"
    "svc_ledger_service:5xx:0.20:2100"
)

# Scenario: Settlement queue backs up → workers timeout
SCENARIO_QUEUE_BACKUP=(
    "queue_ledger_events:5xx:0.80:6000"
    "svc_settlement_worker:timeout:0.90:15000"
    "svc_ledger_service:timeout:0.40:8000"
    "svc_notification_service:5xx:0.35:4500"
)

# Scenario: Auth service failure → everything behind auth wall fails
SCENARIO_AUTH_FAILURE=(
    "svc_auth_service:5xx:0.92:1200"
    "svc_api_gateway:5xx:0.80:3500"
    "svc_payment_api:5xx:0.70:2800"
    "svc_fraud_detector:5xx:0.45:1500"
)

# Scenario: Latency spike in fraud detector (ML model slow)
SCENARIO_FRAUD_SLOW=(
    "svc_fraud_detector:timeout:0.60:9500"
    "svc_payment_api:timeout:0.35:6200"
    "svc_api_gateway:5xx:0.15:3800"
)

ALL_SCENARIOS=(
    SCENARIO_DB_PRIMARY
    SCENARIO_REDIS_DOWN
    SCENARIO_STRIPE_TIMEOUT
    SCENARIO_QUEUE_BACKUP
    SCENARIO_AUTH_FAILURE
    SCENARIO_FRAUD_SLOW
)

SCENARIO_NAMES=(
    "Database Primary Down → Cascading Write Failures"
    "Redis Cache Crash → Auth & Payment Degradation"
    "Stripe API Timeout → Payment Processing Stalled"
    "Settlement Queue Backup → Worker Timeouts"
    "Auth Service Crash → System-Wide 5xx"
    "Fraud Detector Slow → Payment Latency Spike"
)

# ── State ──────────────────────────────────────────────────────────────────────

ACTIVE_FAILURE=""
FAILURE_START=0
FAILURE_DURATION=0
TOKEN=""

# ── Functions ──────────────────────────────────────────────────────────────────

get_token() {
    TOKEN=$(curl -sf -X POST \
        "$KEYCLOAK_URL/realms/$REALM/protocol/openid-connect/token" \
        -H "Content-Type: application/x-www-form-urlencoded" \
        -d "grant_type=password&client_id=$CLIENT_ID&username=$USERNAME&password=$PASSWORD" \
        | python3 -c "import sys,json; print(json.load(sys.stdin)['access_token'])" 2>/dev/null)
}

send_signal() {
    local service=$1
    local status_class=$2
    local error_rate=$3
    local latency=$4

    curl -sf -X POST "$API_GATEWAY/api/v1/signals" \
        -H "Authorization: Bearer $TOKEN" \
        -H "Content-Type: application/json" \
        -d "{
            \"tenantId\": \"$TENANT\",
            \"projectId\": \"$PROJECT_ID\",
            \"environment\": \"prod\",
            \"service\": \"$service\",
            \"statusClass\": \"$status_class\",
            \"errorRate\": $error_rate,
            \"latencyP95\": $latency,
            \"timestamp\": $(date +%s%3N),
            \"source\": \"traffic-simulator\"
        }" > /dev/null 2>&1
}

send_healthy_heartbeat() {
    # Send healthy signals for all services (with slight random jitter)
    for svc in "${SERVICES[@]}"; do
        local base_latency=${SERVICE_BASELINE_LATENCY[$svc]}
        # Add ±20% random jitter to latency
        local jitter=$(( (RANDOM % (base_latency / 5 + 1)) - base_latency / 10 ))
        local latency=$(( base_latency + jitter ))
        if [ $latency -lt 1 ]; then latency=1; fi
        
        # Normal error rate: 0.001 to 0.01 (0.1% to 1%)
        local err_rate="0.00$(( RANDOM % 9 + 1 ))"
        
        send_signal "$svc" "2xx" "$err_rate" "$latency"
    done
}

inject_failure() {
    # Pick a random scenario
    local idx=$(( RANDOM % ${#ALL_SCENARIOS[@]} ))
    local scenario_var="${ALL_SCENARIOS[$idx]}"
    local scenario_name="${SCENARIO_NAMES[$idx]}"
    
    FAILURE_DURATION=$(( RANDOM % (RECOVERY_MAX - RECOVERY_MIN + 1) + RECOVERY_MIN ))
    FAILURE_START=$(date +%s)
    ACTIVE_FAILURE="$scenario_var"

    echo ""
    echo -e "${RED}╔══════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${RED}║  🚨 FAILURE INJECTED                                       ║${NC}"
    echo -e "${RED}║  Scenario: ${scenario_name}${NC}"
    echo -e "${RED}║  Duration: ~${FAILURE_DURATION}s before auto-recovery       ║${NC}"
    echo -e "${RED}╚══════════════════════════════════════════════════════════════╝${NC}"
    echo ""

    # Send the failure signals (get the array by nameref)
    local -n scenario_ref="$scenario_var"
    for entry in "${scenario_ref[@]}"; do
        IFS=':' read -r svc status err_rate latency <<< "$entry"
        echo -e "  ${RED}↯${NC} $svc → ${RED}$status${NC} (error=${err_rate}, latency=${latency}ms)"
        send_signal "$svc" "$status" "$err_rate" "$latency"
        # Stagger signals slightly (50-200ms) to simulate real cascading
        sleep 0.$(( RANDOM % 15 + 5 ))
    done
    echo ""
}

send_failure_signals() {
    # Re-send failure signals to keep the incident active (detection engine deduplicates)
    local -n scenario_ref="$ACTIVE_FAILURE"
    for entry in "${scenario_ref[@]}"; do
        IFS=':' read -r svc status err_rate latency <<< "$entry"
        # Add some randomness to make it look realistic
        local jitter=$(( RANDOM % 500 ))
        local actual_latency=$(( latency + jitter ))
        local actual_err=$(python3 -c "import random; print(round($err_rate * random.uniform(0.85, 1.0), 3))" 2>/dev/null || echo "$err_rate")
        send_signal "$svc" "$status" "$actual_err" "$actual_latency"
    done
}

recover_failure() {
    echo ""
    echo -e "${GREEN}╔══════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║  ✓ AUTO-RECOVERY — Failure resolved                         ║${NC}"
    echo -e "${GREEN}╚══════════════════════════════════════════════════════════════╝${NC}"
    echo ""

    # Send healthy signals for all affected services
    local -n scenario_ref="$ACTIVE_FAILURE"
    for entry in "${scenario_ref[@]}"; do
        IFS=':' read -r svc status err_rate latency <<< "$entry"
        local base_latency=${SERVICE_BASELINE_LATENCY[$svc]:-100}
        echo -e "  ${GREEN}✓${NC} $svc → ${GREEN}2xx${NC} (recovered)"
        send_signal "$svc" "2xx" "0.002" "$base_latency"
    done
    echo ""
    
    ACTIVE_FAILURE=""
    FAILURE_START=0
}

# ── Main Loop ──────────────────────────────────────────────────────────────────

echo ""
echo -e "${CYAN}╔══════════════════════════════════════════════════════════════╗${NC}"
echo -e "${CYAN}║   FaultIQ — Live Traffic Simulator                          ║${NC}"
echo -e "${CYAN}║   15 services • Realistic failures • Auto-recovery          ║${NC}"
echo -e "${CYAN}╚══════════════════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "${GRAY}  Press Ctrl+C to stop${NC}"
echo ""

# Authenticate
echo -n "  Authenticating... "
get_token
if [ -z "$TOKEN" ]; then
    echo -e "${RED}FAILED${NC}"
    echo "  Could not get token from Keycloak. Is docker compose up?"
    exit 1
fi
echo -e "${GREEN}OK${NC} (tenant=$TENANT)"
echo ""

# Token refresh timer
TOKEN_REFRESH_AT=$(( $(date +%s) + 3000 ))  # refresh every ~50 minutes

CYCLE=0

echo -e "${BLUE}  Starting traffic simulation...${NC}"
echo -e "${GRAY}  Healthy heartbeats every ${HEALTHY_INTERVAL}s | Failure check every ${FAILURE_CHECK_INTERVAL}s${NC}"
echo ""

while true; do
    CYCLE=$((CYCLE + 1))
    NOW=$(date +%s)

    # Refresh token if needed
    if [ $NOW -ge $TOKEN_REFRESH_AT ]; then
        get_token
        TOKEN_REFRESH_AT=$(( NOW + 3000 ))
    fi

    # Check if active failure should recover
    if [ -n "$ACTIVE_FAILURE" ]; then
        local_elapsed=$(( NOW - FAILURE_START ))
        if [ $local_elapsed -ge $FAILURE_DURATION ]; then
            recover_failure
        else
            # Re-send failure signals to keep it active
            send_failure_signals
            remaining=$(( FAILURE_DURATION - local_elapsed ))
            printf "\r  ${RED}⚡ Failure active${NC} — recovering in ${remaining}s | cycle #${CYCLE}  "
        fi
    else
        # Normal operation: send healthy heartbeats
        send_healthy_heartbeat
        printf "\r  ${GREEN}●${NC} Healthy — cycle #${CYCLE} (${#SERVICES[@]} services) | next failure check in ~${FAILURE_CHECK_INTERVAL}s  "
        
        # Maybe inject a failure
        if [ $((CYCLE % (FAILURE_CHECK_INTERVAL / HEALTHY_INTERVAL) )) -eq 0 ]; then
            roll=$(( RANDOM % 100 ))
            if [ $roll -lt $FAILURE_PROBABILITY ]; then
                inject_failure
            fi
        fi
    fi

    sleep $HEALTHY_INTERVAL
done
