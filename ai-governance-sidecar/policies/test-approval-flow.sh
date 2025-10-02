#!/bin/bash

set -e

BASE_URL="http://localhost:8080"
BOLD='\033[1m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${BOLD}AI Governance Sidecar - Approval Flow Test${NC}"
echo "=============================================="
echo ""

# Function to print section headers
print_section() {
    echo -e "\n${BOLD}${1}${NC}"
    echo "-------------------------------------------"
}

# Function to check if service is ready
check_health() {
    print_section "1. Health Check"
    
    response=$(curl -s "$BASE_URL/health")
    echo "$response" | jq .
    
    status=$(echo "$response" | jq -r .status)
    if [ "$status" = "healthy" ]; then
        echo -e "${GREEN}✓ Service is healthy${NC}"
    else
        echo -e "${RED}✗ Service is not healthy${NC}"
        exit 1
    fi
}

# Function to test passthrough (no approval needed)
test_passthrough() {
    print_section "2. Test Passthrough Request (No Approval)"
    
    echo "Sending simple calculator request..."
    response=$(curl -s -X POST "$BASE_URL/proxy" \
        -H "Content-Type: application/json" \
        -d '{
            "tool": "calculator",
            "action": "add",
            "parameters": {"a": 5, "b": 3}
        }')
    
    echo "$response" | jq .
    
    if echo "$response" | jq -e '.result' > /dev/null; then
        echo -e "${GREEN}✓ Request completed without approval${NC}"
    else
        echo -e "${YELLOW}⚠ Request behavior differs from expected${NC}"
    fi
}

# Function to test request requiring approval
test_approval_required() {
    print_section "3. Test Request Requiring Approval"
    
    echo "Sending request with sensitive data (password)..."
    response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X POST "$BASE_URL/proxy" \
        -H "Content-Type: application/json" \
        -d '{
            "tool": "database",
            "action": "query",
            "parameters": {"query": "SELECT password FROM users WHERE id=1"}
        }')
    
    http_status=$(echo "$response" | grep "HTTP_STATUS:" | cut -d: -f2)
    body=$(echo "$response" | sed '/HTTP_STATUS:/d')
    
    echo "$body" | jq .
    
    if [ "$http_status" = "202" ]; then
        echo -e "${GREEN}✓ Request properly queued for approval (HTTP 202)${NC}"
        
        approval_id=$(echo "$body" | jq -r '.approval_id')
        echo -e "Approval ID: ${YELLOW}$approval_id${NC}"
        
        # Save approval_id for later steps
        echo "$approval_id" > /tmp/approval_id.txt
        return 0
    else
        echo -e "${RED}✗ Expected HTTP 202, got $http_status${NC}"
        return 1
    fi
}

# Function to check pending approvals
test_pending_approvals() {
    print_section "4. Check Pending Approvals"
    
    response=$(curl -s "$BASE_URL/approvals/pending")
    echo "$response" | jq .
    
    count=$(echo "$response" | jq '.total')
    if [ "$count" -gt 0 ]; then
        echo -e "${GREEN}✓ Found $count pending approval(s)${NC}"
    else
        echo -e "${YELLOW}⚠ No pending approvals found${NC}"
    fi
}

# Function to approve request
test_approve_request() {
    print_section "5. Approve Pending Request"
    
    if [ ! -f /tmp/approval_id.txt ]; then
        echo -e "${YELLOW}⚠ No approval_id from previous test${NC}"
        return 1
    fi
    
    approval_id=$(cat /tmp/approval_id.txt)
    echo "Approving request: $approval_id"
    
    response=$(curl -s -X POST "$BASE_URL/approvals/$approval_id/approve" \
        -H "Content-Type: application/json" \
        -d '{
            "approver": "test-admin@example.com",
            "comment": "Approved for testing purposes"
        }')
    
    echo "$response" | jq .
    
    if echo "$response" | jq -e '.approved' | grep -q true; then
        echo -e "${GREEN}✓ Request approved successfully${NC}"
    else
        echo -e "${RED}✗ Approval failed${NC}"
        return 1
    fi
}

# Function to test deny
test_deny_request() {
    print_section "6. Test Deny Request"
    
    # First, create a new request to deny
    echo "Creating new request to deny..."
    response=$(curl -s -X POST "$BASE_URL/proxy" \
        -H "Content-Type: application/json" \
        -d '{
            "tool": "database",
            "action": "query",
            "parameters": {"query": "SELECT credit_card FROM payments"}
        }')
    
    approval_id=$(echo "$response" | jq -r '.approval_id')
    
    if [ "$approval_id" != "null" ] && [ -n "$approval_id" ]; then
        echo "Denying request: $approval_id"
        
        deny_response=$(curl -s -X POST "$BASE_URL/approvals/$approval_id/deny" \
            -H "Content-Type: application/json" \
            -d '{
                "approver": "test-admin@example.com",
                "comment": "Denied - too sensitive for testing"
            }')
        
        echo "$deny_response" | jq .
        
        if echo "$deny_response" | jq -e '.denied' | grep -q true; then
            echo -e "${GREEN}✓ Request denied successfully${NC}"
        else
            echo -e "${RED}✗ Deny failed${NC}"
        fi
    else
        echo -e "${YELLOW}⚠ Could not create request to deny${NC}"
    fi
}

# Function to check audit trail
test_audit_trail() {
    print_section "7. Check Audit Trail"
    
    response=$(curl -s "$BASE_URL/audit?limit=5")
    echo "$response" | jq .
    
    count=$(echo "$response" | jq '.total')
    if [ "$count" -gt 0 ]; then
        echo -e "${GREEN}✓ Found $count audit entries${NC}"
    else
        echo -e "${YELLOW}⚠ No audit entries found${NC}"
    fi
}

# Function to test rate limiting
test_high_volume() {
    print_section "8. Test High Volume Request (Rate Limit Policy)"
    
    echo "Sending bulk operation request..."
    response=$(curl -s -X POST "$BASE_URL/proxy" \
        -H "Content-Type: application/json" \
        -d '{
            "tool": "database",
            "action": "bulk_delete",
            "parameters": {"count": 5000, "table": "logs"}
        }')
    
    echo "$response" | jq .
    
    if echo "$response" | jq -e '.approval_id' > /dev/null 2>&1; then
        echo -e "${GREEN}✓ High volume request properly requires approval${NC}"
    else
        echo -e "${YELLOW}⚠ High volume request behavior differs${NC}"
    fi
}

# Main test execution
main() {
    echo "Starting comprehensive approval flow test..."
    echo "Target: $BASE_URL"
    echo ""
    
    # Wait for service to be ready
    echo "Waiting for service to start..."
    sleep 2
    
    # Run tests in sequence
    check_health
    test_passthrough
    test_approval_required
    test_pending_approvals
    test_approve_request
    test_deny_request
    test_audit_trail
    test_high_volume
    
    # Summary
    print_section "Test Summary"
    echo -e "${GREEN}✓ All core approval workflow tests completed${NC}"
    echo ""
    echo "Next steps:"
    echo "  - Connect to WebSocket at ws://localhost:8080/ws"
    echo "  - Build React UI for approval management"
    echo "  - Deploy to production with proper authentication"
    echo ""
}

# Run main function
main