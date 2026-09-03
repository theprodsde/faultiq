#!/bin/bash

################################################################################
# FaultIQ Superadmin Authentication & Authorization Testing Script
# 
# This script verifies the complete authentication flow for a superadmin user:
# 1. Token generation from Keycloak
# 2. Token structure verification (claims)
# 3. API Gateway token validation
# 4. Project data visibility
# 5. Comparison with other roles
#
# Usage: ./test-superadmin-flow.sh
################################################################################

set -e

# Color codes for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
KEYCLOAK_URL="${KEYCLOAK_URL:-http://localhost:8081}"
API_GATEWAY_URL="${API_GATEWAY_URL:-http://localhost:8080}"
REALM="faultiq"
CLIENT_ID="faultiq-ui"

# Test data
SUPERADMIN_USERNAME="super"
SUPERADMIN_PASSWORD="superpass"
ANALYST_USERNAME="analyst"
ANALYST_PASSWORD="analystpass"
VIEWER_USERNAME="viewer"
VIEWER_PASSWORD="viewerpass"

# Counters
TESTS_PASSED=0
TESTS_FAILED=0
TESTS_TOTAL=0

################################################################################
# Helper Functions
################################################################################

print_section() {
    echo ""
    echo -e "${BLUE}╔════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║ $1${NC}"
    echo -e "${BLUE}╚════════════════════════════════════════════════════════════╝${NC}"
    echo ""
}

print_subsection() {
    echo -e "${YELLOW}[TEST] $1${NC}"
}

test_passed() {
    echo -e "${GREEN}✓ $1${NC}"
    ((TESTS_PASSED++))
    ((TESTS_TOTAL++))
}

test_failed() {
    echo -e "${RED}✗ $1${NC}"
    ((TESTS_FAILED++))
    ((TESTS_TOTAL++))
}

test_warning() {
    echo -e "${YELLOW}⚠ $1${NC}"
}

log_response() {
    echo -e "${BLUE}Response:${NC}"
    echo "$1" | jq . 2>/dev/null || echo "$1"
}

print_claims() {
    local token=$1
    local label=${2:-"Token Claims"}
    
    echo -e "${BLUE}${label}:${NC}"
    
    # Extract payload
    local payload=$(echo $token | cut -d'.' -f2)
    local padded="${payload}$(printf '=%.0s' {1..10})"
    
    # Decode and display
    echo $padded | base64 -D 2>/dev/null | jq . 2>/dev/null || {
        test_failed "Could not decode JWT payload"
        return 1
    }
}

verify_claim() {
    local token=$1
    local claim_key=$2
    local expected_value=$3
    
    # Extract and decode payload
    local payload=$(echo $token | cut -d'.' -f2)
    local padded="${payload}$(printf '=%.0s' {1..10})"
    local claims=$(echo $padded | base64 -D 2>/dev/null | jq . 2>/dev/null)
    
    if [ -z "$claims" ]; then
        test_failed "Could not parse JWT claims"
        return 1
    fi
    
    # Extract claim value
    local value=$(echo "$claims" | jq -r ".${claim_key} // empty")
    
    if [ -z "$value" ]; then
        test_failed "Claim '$claim_key' not found in token"
        return 1
    fi
    
    if [ "$value" = "$expected_value" ] || [[ "$value" == *"$expected_value"* ]]; then
        test_passed "Claim '$claim_key' = $value"
        return 0
    else
        test_failed "Claim '$claim_key' = $value (expected: $expected_value)"
        return 1
    fi
}

################################################################################
# Main Tests
################################################################################

main() {
    clear
    
    echo ""
    echo -e "${BLUE}╔═══════════════════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║   FaultIQ Authentication & Authorization Verification      ║${NC}"
    echo -e "${BLUE}╚═══════════════════════════════════════════════════════════╝${NC}"
    echo ""
    
    # Check if services are running
    print_section "STEP 1: Checking Service Availability"
    
    # Check Keycloak
    print_subsection "Testing Keycloak connectivity..."
    if curl -s "$KEYCLOAK_URL/realms/$REALM" > /dev/null; then
        test_passed "Keycloak is accessible at $KEYCLOAK_URL"
    else
        test_failed "Keycloak is NOT accessible at $KEYCLOAK_URL"
        echo "Try: docker-compose up -d keycloak"
        exit 1
    fi
    
    # Check API Gateway
    print_subsection "Testing API Gateway connectivity..."
    if curl -s "$API_GATEWAY_URL/health" > /dev/null; then
        test_passed "API Gateway is accessible at $API_GATEWAY_URL"
    else
        test_failed "API Gateway is NOT accessible at $API_GATEWAY_URL"
        echo "Try: docker-compose up -d api-gateway"
        exit 1
    fi
    
    ################################################################################
    # Token Generation Tests
    ################################################################################
    
    print_section "STEP 2: Token Generation"
    
    print_subsection "Requesting superadmin token (super:superpass)..."
    SUPER_TOKEN=$(curl -s -X POST "$KEYCLOAK_URL/realms/$REALM/protocol/openid-connect/token" \
        -d "client_id=$CLIENT_ID" \
        -d "username=$SUPERADMIN_USERNAME" \
        -d "password=$SUPERADMIN_PASSWORD" \
        -d "grant_type=password" | jq -r '.access_token // empty')
    
    if [ -z "$SUPER_TOKEN" ] || [ "$SUPER_TOKEN" = "null" ]; then
        test_failed "Failed to get superadmin token"
        exit 1
    fi
    
    test_passed "Superadmin token received"
    echo "Token (first 50 chars): ${SUPER_TOKEN:0:50}..."
    
    ################################################################################
    # Token Structure Tests
    ################################################################################
    
    print_section "STEP 3: Token Structure Verification"
    
    # Decode and display token
    print_subsection "Decoding token payload..."
    print_claims "$SUPER_TOKEN" "Superadmin Token Claims"
    
    # Verify required claims
    print_subsection "Verifying required claims..."
    
    verify_claim "$SUPER_TOKEN" "tenant" "acme-corp" || test_warning "tenant claim not as expected"
    verify_claim "$SUPER_TOKEN" "projects" "payments-platform" || test_warning "projects claim not as expected"
    verify_claim "$SUPER_TOKEN" "realm_access.roles" "super_admin" || test_warning "super_admin role not found"
    
    # Verify JWT structure
    print_subsection "Verifying JWT structure..."
    local header=$(echo $SUPER_TOKEN | cut -d'.' -f1)
    if [ ${#header} -gt 0 ]; then
        test_passed "JWT has header segment"
    else
        test_failed "JWT missing header segment"
    fi
    
    local payload=$(echo $SUPER_TOKEN | cut -d'.' -f2)
    if [ ${#payload} -gt 0 ]; then
        test_passed "JWT has payload segment"
    else
        test_failed "JWT missing payload segment"
    fi
    
    local signature=$(echo $SUPER_TOKEN | cut -d'.' -f3)
    if [ ${#signature} -gt 0 ]; then
        test_passed "JWT has signature segment"
    else
        test_failed "JWT missing signature segment"
    fi
    
    ################################################################################
    # API Gateway Token Validation Tests
    ################################################################################
    
    print_section "STEP 4: API Gateway Token Validation"
    
    print_subsection "Testing API Gateway health with superadmin token..."
    HEALTH_RESPONSE=$(curl -s -w "\n%{http_code}" -X GET "$API_GATEWAY_URL/health" \
        -H "Authorization: Bearer $SUPER_TOKEN")
    
    HEALTH_STATUS=$(echo "$HEALTH_RESPONSE" | tail -n1)
    if [ "$HEALTH_STATUS" = "200" ]; then
        test_passed "API Gateway accepted superadmin token (HTTP 200)"
    else
        test_failed "API Gateway rejected token (HTTP $HEALTH_STATUS)"
    fi
    
    ################################################################################
    # Project Data Access Tests
    ################################################################################
    
    print_section "STEP 5: Project Data Access"
    
    print_subsection "Fetching projects with superadmin token..."
    PROJECTS_RESPONSE=$(curl -s -X GET "$API_GATEWAY_URL/api/v1/projects" \
        -H "Authorization: Bearer $SUPER_TOKEN" \
        -H "Content-Type: application/json")
    
    PROJECT_COUNT=$(echo "$PROJECTS_RESPONSE" | jq 'length // 0' 2>/dev/null || echo "0")
    
    if [ "$PROJECT_COUNT" -gt 0 ]; then
        test_passed "Received $PROJECT_COUNT project(s) for superadmin"
        echo -e "${BLUE}Projects:${NC}"
        echo "$PROJECTS_RESPONSE" | jq '.[] | {id, name, tenant_id}' 2>/dev/null
    else
        test_warning "No projects returned or invalid response"
        echo -e "${BLUE}Response:${NC}"
        echo "$PROJECTS_RESPONSE"
    fi
    
    ################################################################################
    # Role Comparison Tests
    ################################################################################
    
    print_section "STEP 6: Role-Based Access Comparison"
    
    # Get analyst token
    print_subsection "Requesting analyst token (analyst:analystpass)..."
    ANALYST_TOKEN=$(curl -s -X POST "$KEYCLOAK_URL/realms/$REALM/protocol/openid-connect/token" \
        -d "client_id=$CLIENT_ID" \
        -d "username=$ANALYST_USERNAME" \
        -d "password=$ANALYST_PASSWORD" \
        -d "grant_type=password" | jq -r '.access_token // empty')
    
    if [ -z "$ANALYST_TOKEN" ]; then
        test_failed "Failed to get analyst token"
    else
        test_passed "Analyst token received"
        
        # Check analyst claims
        print_subsection "Analyst token claims:"
        print_claims "$ANALYST_TOKEN" "Analyst Token Claims"
    fi
    
    # Get viewer token
    print_subsection "Requesting viewer token (viewer:viewerpass)..."
    VIEWER_TOKEN=$(curl -s -X POST "$KEYCLOAK_URL/realms/$REALM/protocol/openid-connect/token" \
        -d "client_id=$CLIENT_ID" \
        -d "username=$VIEWER_USERNAME" \
        -d "password=$VIEWER_PASSWORD" \
        -d "grant_type=password" | jq -r '.access_token // empty')
    
    if [ -z "$VIEWER_TOKEN" ]; then
        test_failed "Failed to get viewer token"
    else
        test_passed "Viewer token received"
        
        # Check viewer claims
        print_subsection "Viewer token claims:"
        print_claims "$VIEWER_TOKEN" "Viewer Token Claims"
    fi
    
    # Compare project access
    print_subsection "Comparing project access across roles..."
    
    if [ ! -z "$ANALYST_TOKEN" ]; then
        ANALYST_PROJECTS=$(curl -s -X GET "$API_GATEWAY_URL/api/v1/projects" \
            -H "Authorization: Bearer $ANALYST_TOKEN" \
            -H "Content-Type: application/json")
        ANALYST_COUNT=$(echo "$ANALYST_PROJECTS" | jq 'length // 0' 2>/dev/null || echo "0")
        
        echo "Projects visible:"
        echo "  Superadmin: $PROJECT_COUNT"
        echo "  Analyst:    $ANALYST_COUNT"
    fi
    
    ################################################################################
    # Token Refresh Tests
    ################################################################################
    
    print_section "STEP 7: Token Management"
    
    print_subsection "Checking token expiration..."
    
    # Extract exp claim
    local payload=$(echo $SUPER_TOKEN | cut -d'.' -f2)
    local padded="${payload}$(printf '=%.0s' {1..10})"
    local exp=$(echo $padded | base64 -D 2>/dev/null | jq -r '.exp // empty')
    local current_time=$(date +%s)
    local remaining=$((exp - current_time))
    
    if [ $remaining -gt 0 ]; then
        test_passed "Token is valid (expires in $remaining seconds)"
    else
        test_failed "Token has expired"
    fi
    
    ################################################################################
    # Frontend Token Storage Tests
    ################################################################################
    
    print_section "STEP 8: Frontend Token Storage Verification"
    
    print_subsection "Frontend should store token in window.__faultiqToken"
    echo "To verify in browser console (http://localhost:4001):"
    echo ""
    echo "  1. Login with super:superpass"
    echo "  2. Open browser console (F12)"
    echo "  3. Run: console.log(window.__faultiqToken)"
    echo "  4. Should see: eyJhbGciOiJSUzI1NiIs..."
    echo ""
    echo "  5. Decode claims:"
    echo "     const parts = window.__faultiqToken.split('.')"
    echo "     const claims = JSON.parse(atob(parts[1]))"
    echo "     console.log(claims)"
    echo ""
    
    ################################################################################
    # Summary
    ################################################################################
    
    print_section "TEST SUMMARY"
    
    echo "Total Tests: $TESTS_TOTAL"
    echo -e "Passed: ${GREEN}$TESTS_PASSED${NC}"
    echo -e "Failed: ${RED}$TESTS_FAILED${NC}"
    echo ""
    
    if [ $TESTS_FAILED -eq 0 ]; then
        echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
        echo -e "${GREEN}✓ All tests passed! Superadmin authentication is working.${NC}"
        echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
        exit 0
    else
        echo -e "${RED}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
        echo -e "${RED}✗ Some tests failed. Check the output above for details.${NC}"
        echo -e "${RED}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
        exit 1
    fi
}

################################################################################
# Run Tests
################################################################################

main
