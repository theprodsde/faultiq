#!/bin/bash

# Node-Down Detection & RCA Resolution Flow - Automated Test Script
# This script walks through the complete signal → incident → RCA → resolution flow

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
API_URL="http://localhost:8080"
SIGNAL_URL="http://localhost:8085"
DB_HOST="localhost"
POSTGRES_PORT=5434
POSTGRES_DB="faultiq"
POSTGRES_USER="postgres"
POSTGRES_PASSWORD="postgres"

# Get a token (for auth-required endpoints)
# For now using service-to-service auth or public endpoints
TOKEN=""

# Helper functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Step 1: Verify services are running
verify_services() {
    log_info "Step 1: Verifying services are running..."
    
    # Check API Gateway
    if ! curl -s http://localhost:8080/health > /dev/null; then
        log_error "API Gateway (8080) is not responding"
        exit 1
    fi
    log_success "API Gateway (8080) ✓"
    
    # Check Signal Ingestion
    if ! curl -s http://localhost:8085/health > /dev/null; then
        log_error "Signal Ingestion (8085) is not responding"
        exit 1
    fi
    log_success "Signal Ingestion (8085) ✓"
    
    # Check Detection Engine
    if ! curl -s http://localhost:8084/health > /dev/null; then
        log_error "Detection Engine (8084) is not responding"
        exit 1
    fi
    log_success "Detection Engine (8084) ✓"
    
    # Check Database
    if ! docker-compose exec -T postgres psql -U $POSTGRES_USER -d $POSTGRES_DB -c "SELECT 1" > /dev/null 2>&1; then
        log_error "PostgreSQL is not responding"
        exit 1
    fi
    log_success "PostgreSQL ✓"
    
    echo ""
}

# Step 2: Get current incident count
get_incident_count() {
    docker-compose exec -T postgres psql -U $POSTGRES_USER -d $POSTGRES_DB -t -c \
        "SELECT COUNT(*) FROM incidents WHERE project_id = 'proj-payments-prod' AND status = 'OPEN';" | tr -d ' '
}

# Step 3: Send timeout signal
send_signal() {
    local service=$1
    local status_class=$2
    local error_rate=$3
    local latency=$4
    
    log_info "Sending signal: service=$service, statusClass=$status_class, errorRate=$error_rate, latency=$latency"
    
    local timestamp=$(date +%s%N | cut -b1-13)
    local response=$(curl -s -X POST $SIGNAL_URL/api/v1/signals \
        -H 'Content-Type: application/json' \
        -d "{
            \"tenant\": \"acme-corp\",
            \"tenantId\": \"acme-corp\",
            \"project\": \"payments-platform\",
            \"projectId\": \"proj-payments-prod\",
            \"environment\": \"prod\",
            \"service\": \"$service\",
            \"statusClass\": \"$status_class\",
            \"errorRate\": $error_rate,
            \"latencyP95\": $latency,
            \"timestamp\": $timestamp
        }")
    
    local http_code=$(curl -s -o /dev/null -w "%{http_code}" -X POST $SIGNAL_URL/api/v1/signals \
        -H 'Content-Type: application/json' \
        -d "{
            \"tenant\": \"acme-corp\",
            \"tenantId\": \"acme-corp\",
            \"project\": \"payments-platform\",
            \"projectId\": \"proj-payments-prod\",
            \"environment\": \"prod\",
            \"service\": \"$service\",
            \"statusClass\": \"$status_class\",
            \"errorRate\": $error_rate,
            \"latencyP95\": $latency,
            \"timestamp\": $timestamp
        }")
    
    if [ "$http_code" = "202" ] || [ "$http_code" = "200" ]; then
        log_success "Signal accepted (HTTP $http_code)"
    else
        log_warning "Signal response: HTTP $http_code"
    fi
}

# Step 4: Wait for incident creation
wait_for_incident() {
    local service=$1
    local max_wait=10
    local waited=0
    
    log_info "Waiting for incident to be created..."
    
    while [ $waited -lt $max_wait ]; do
        local count=$(docker-compose exec -T postgres psql -U $POSTGRES_USER -d $POSTGRES_DB -t -c \
            "SELECT COUNT(*) FROM incidents WHERE service = '$service' AND status = 'OPEN';" | tr -d ' ')
        
        if [ "$count" -gt 0 ]; then
            log_success "Incident created for $service"
            return 0
        fi
        
        echo -ne "."
        sleep 1
        ((waited++))
    done
    
    log_error "Timeout waiting for incident creation"
    return 1
}

# Step 5: Verify incident in database
verify_incident() {
    local service=$1
    
    log_info "Verifying incident in database..."
    
    local incident=$(docker-compose exec -T postgres psql -U $POSTGRES_USER -d $POSTGRES_DB -t -c \
        "SELECT id, status, confidence FROM incidents WHERE service = '$service' AND status = 'OPEN' ORDER BY detected_at DESC LIMIT 1;")
    
    if [ -z "$incident" ]; then
        log_error "No incident found in database"
        return 1
    fi
    
    log_success "Incident found: $incident"
    return 0
}

# Step 6: Acknowledge incident
acknowledge_incident() {
    local incident_id=$(docker-compose exec -T postgres psql -U $POSTGRES_USER -d $POSTGRES_DB -t -c \
        "SELECT id FROM incidents WHERE status = 'OPEN' ORDER BY detected_at DESC LIMIT 1;" | tr -d ' ')
    
    if [ -z "$incident_id" ]; then
        log_error "No OPEN incident found"
        return 1
    fi
    
    log_info "Acknowledging incident: $incident_id"
    
    local response=$(curl -s -X PUT "$API_URL/api/v1/incidents/$incident_id/status" \
        -H 'Content-Type: application/json' \
        -d '{"status": "ACKNOWLEDGED"}')
    
    log_success "Incident acknowledged"
    
    # Verify status changed
    sleep 1
    local status=$(docker-compose exec -T postgres psql -U $POSTGRES_USER -d $POSTGRES_DB -t -c \
        "SELECT status FROM incidents WHERE id = '$incident_id';" | tr -d ' ')
    
    if [ "$status" = "ACKNOWLEDGED" ]; then
        log_success "Status verified: ACKNOWLEDGED"
    else
        log_warning "Status may not have updated: $status"
    fi
}

# Step 7: Resolve incident
resolve_incident() {
    local incident_id=$(docker-compose exec -T postgres psql -U $POSTGRES_USER -d $POSTGRES_DB -t -c \
        "SELECT id FROM incidents WHERE status = 'ACKNOWLEDGED' ORDER BY updated_at DESC LIMIT 1;" | tr -d ' ')
    
    if [ -z "$incident_id" ]; then
        log_error "No ACKNOWLEDGED incident found"
        return 1
    fi
    
    log_info "Resolving incident: $incident_id"
    
    curl -s -X PUT "$API_URL/api/v1/incidents/$incident_id/status" \
        -H 'Content-Type: application/json' \
        -d '{"status": "RESOLVED"}' > /dev/null
    
    log_success "Incident resolved"
    
    # Verify status changed
    sleep 1
    local status=$(docker-compose exec -T postgres psql -U $POSTGRES_USER -d $POSTGRES_DB -t -c \
        "SELECT status FROM incidents WHERE id = '$incident_id';" | tr -d ' ')
    
    if [ "$status" = "RESOLVED" ]; then
        log_success "Status verified: RESOLVED"
    else
        log_warning "Status may not have updated: $status"
    fi
}

# Step 8: Check incident counts
check_counts() {
    log_info "Checking incident counts by status..."
    
    local open=$(docker-compose exec -T postgres psql -U $POSTGRES_USER -d $POSTGRES_DB -t -c \
        "SELECT COUNT(*) FROM incidents WHERE project_id = 'proj-payments-prod' AND status = 'OPEN';" | tr -d ' ')
    local acked=$(docker-compose exec -T postgres psql -U $POSTGRES_USER -d $POSTGRES_DB -t -c \
        "SELECT COUNT(*) FROM incidents WHERE project_id = 'proj-payments-prod' AND status = 'ACKNOWLEDGED';" | tr -d ' ')
    local resolved=$(docker-compose exec -T postgres psql -U $POSTGRES_USER -d $POSTGRES_DB -t -c \
        "SELECT COUNT(*) FROM incidents WHERE project_id = 'proj-payments-prod' AND status = 'RESOLVED';" | tr -d ' ')
    
    log_success "OPEN: $open, ACKNOWLEDGED: $acked, RESOLVED: $resolved"
}

# Main test flow
run_test() {
    echo ""
    echo "╔════════════════════════════════════════════════════════════════════════╗"
    echo "║  Node-Down Detection & RCA Resolution Flow - Automated Test Suite      ║"
    echo "╚════════════════════════════════════════════════════════════════════════╝"
    echo ""
    
    # Step 1
    verify_services
    
    # Step 2
    log_info "Step 2: Recording initial incident count..."
    initial_count=$(get_incident_count)
    log_success "Initial incident count: $initial_count"
    echo ""
    
    # Step 3
    log_info "Step 3: Sending timeout signal for payment-api..."
    send_signal "svc_payment_api" "timeout" "0.95" "5000"
    echo ""
    
    # Step 4
    log_info "Step 4: Waiting for incident creation..."
    wait_for_incident "svc_payment_api" || exit 1
    sleep 2
    echo ""
    
    # Step 5
    log_info "Step 5: Verifying incident..."
    verify_incident "svc_payment_api" || exit 1
    echo ""
    
    # Step 6
    check_counts
    echo ""
    
    # Step 7
    log_info "Step 6: Acknowledging incident..."
    acknowledge_incident
    echo ""
    
    # Step 8
    check_counts
    echo ""
    
    # Step 9
    log_info "Step 7: Resolving incident..."
    resolve_incident
    echo ""
    
    # Step 10
    check_counts
    echo ""
    
    # Summary
    log_info "Step 8: Final verification..."
    final_count=$(get_incident_count)
    expected_count=$((initial_count))  # Should return to initial (RESOLVED incidents don't count as OPEN)
    
    if [ "$final_count" = "$expected_count" ]; then
        log_success "Final incident count matches expected: $final_count"
    else
        log_warning "Final count ($final_count) differs from initial ($expected_count)"
    fi
    
    echo ""
    echo "╔════════════════════════════════════════════════════════════════════════╗"
    echo "║  Test Complete! Verify in Frontend:                                    ║"
    echo "║  1. Dashboard: http://localhost:4000/dashboard                          ║"
    echo "║  2. Graph: http://localhost:4000/dashboard/graph                        ║"
    echo "║  3. Incidents: http://localhost:4000/dashboard/incidents                ║"
    echo "╚════════════════════════════════════════════════════════════════════════╝"
    echo ""
}

# Run the test
run_test
