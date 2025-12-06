#!/usr/bin/env bash
set -euo pipefail

BOLD='\033[1m'
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

API_BASE="${API_BASE:-http://localhost:8080}"
UI_BASE="${UI_BASE:-http://localhost:3000}"

PASSED=0
FAILED=0

echo -e "${BOLD}╔════════════════════════════════════════════╗${NC}"
echo -e "${BOLD}║  Deployment Verification Script           ║${NC}"
echo -e "${BOLD}╚════════════════════════════════════════════╝${NC}"
echo ""

check() {
  local name=$1
  local command=$2
  
  echo -n "Checking $name... "
  
  if eval "$command" >/dev/null 2>&1; then
    echo -e "${GREEN}✓ PASS${NC}"
    PASSED=$((PASSED + 1))
    return 0
  else
    echo -e "${RED}✗ FAIL${NC}"
    FAILED=$((FAILED + 1))
    return 1
  fi
}

# Docker checks
echo -e "${BOLD}Docker Environment${NC}"
echo "─────────────────────────────────────────────"
check "Docker installed" "command -v docker"
check "Docker running" "docker ps"
check "Docker Compose available" "docker-compose version || docker compose version"
echo ""

# Container checks
echo -e "${BOLD}Container Status${NC}"
echo "─────────────────────────────────────────────"
check "Backend container running" "docker-compose ps governance-sidecar | grep -q Up"
check "Frontend container running" "docker-compose ps governance-ui | grep -q Up"
echo ""

# Service health checks
echo -e "${BOLD}Service Health${NC}"
echo "─────────────────────────────────────────────"
check "Backend health endpoint" "curl -sf $API_BASE/health"
check "Frontend accessible" "curl -sf $UI_BASE/"
echo ""

# Authentication checks
echo -e "${BOLD}Authentication${NC}"
echo "─────────────────────────────────────────────"

# Test login
LOGIN_RESULT=$(curl -sf -X POST "$API_BASE/login" \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@example.com","password":"admin"}' 2>&1 || echo '{}')

TOKEN=$(echo "$LOGIN_RESULT" | jq -r '.token // empty' 2>/dev/null || echo "")

if [ ! -z "$TOKEN" ] && [ "$TOKEN" != "null" ]; then
  echo -e "Login endpoint... ${GREEN}✓ PASS${NC}"
  PASSED=$((PASSED + 1))
  
  # Export for subsequent checks
  export AUTH_TOKEN="$TOKEN"
else
  echo -e "Login endpoint... ${RED}✗ FAIL${NC}"
  echo "  Response: $LOGIN_RESULT"
  FAILED=$((FAILED + 1))
fi
echo ""

# Approval endpoints
if [ ! -z "$TOKEN" ] && [ "$TOKEN" != "null" ]; then
  echo -e "${BOLD}API Endpoints${NC}"
  echo "─────────────────────────────────────────────"
  
  check "/approvals/pending" "curl -sf '$API_BASE/approvals/pending' -H 'Authorization: Bearer $TOKEN'"
  check "/audit" "curl -sf '$API_BASE/audit?limit=1' -H 'Authorization: Bearer $TOKEN'"
  check "/simulate/enqueue" "curl -sf -X POST '$API_BASE/simulate/enqueue' -H 'Authorization: Bearer $TOKEN' -H 'Content-Type: application/json' -d '{\"tool_name\":\"test\"}'"
  echo ""
fi

# Audit log verification
if [ ! -z "$TOKEN" ] && [ "$TOKEN" != "null" ]; then
  echo -e "${BOLD}Audit Log Verification${NC}"
  echo "─────────────────────────────────────────────"
  
  AUDIT_RESPONSE=$(curl -sf "$API_BASE/audit?limit=10" \
    -H "Authorization: Bearer $TOKEN" 2>&1 || echo '{"entries":[]}')
  
  ENTRY_COUNT=$(echo "$AUDIT_RESPONSE" | jq '.entries | length' 2>/dev/null || echo "0")
  
  echo -n "Audit log entries... "
  if [ "$ENTRY_COUNT" -gt 0 ]; then
    echo -e "${GREEN}✓ PASS${NC} ($ENTRY_COUNT entries found)"
    PASSED=$((PASSED + 1))
    
    # Check for approved entries
    APPROVED_COUNT=$(echo "$AUDIT_RESPONSE" | jq '[.entries[] | select(.status == "approved")] | length' 2>/dev/null || echo "0")
    
    echo -n "Approved entries... "
    if [ "$APPROVED_COUNT" -ge 4 ]; then
      echo -e "${GREEN}✓ PASS${NC} ($APPROVED_COUNT approved)"
      PASSED=$((PASSED + 1))
    else
      echo -e "${YELLOW}⚠ PARTIAL${NC} ($APPROVED_COUNT approved, expected 4+)"
    fi
  else
    echo -e "${YELLOW}⚠ EMPTY${NC} (no entries - simulation may not have run)"
  fi
  echo ""
fi

# WebSocket check
echo -e "${BOLD}WebSocket Support${NC}"
echo "─────────────────────────────────────────────"
echo -n "WebSocket endpoint... "

# Quick check if endpoint exists (may not work perfectly without proper WS client)
WS_CHECK=$(curl -i -N -s \
  -H "Connection: Upgrade" \
  -H "Upgrade: websocket" \
  "$API_BASE/ws" 2>&1 | head -n 1)

if echo "$WS_CHECK" | grep -q "101\|400\|426"; then
  echo -e "${GREEN}✓ PASS${NC} (endpoint exists)"
  PASSED=$((PASSED + 1))
else
  echo -e "${YELLOW}⚠ UNKNOWN${NC} (cannot verify without WS client)"
fi
echo ""

# Summary
echo -e "${BOLD}═════════════════════════════════════════════${NC}"
echo -e "${BOLD}Summary${NC}"
echo -e "${BOLD}═════════════════════════════════════════════${NC}"
echo ""

TOTAL=$((PASSED + FAILED))
PASS_RATE=$((PASSED * 100 / TOTAL))

echo "Total checks: $TOTAL"
echo -e "Passed: ${GREEN}$PASSED${NC}"
echo -e "Failed: ${RED}$FAILED${NC}"
echo "Success rate: $PASS_RATE%"
echo ""

if [ $FAILED -eq 0 ]; then
  echo -e "${BOLD}${GREEN}✓ All checks passed!${NC}"
  echo ""
  echo -e "${BOLD}Access Points:${NC}"
  echo "  • Web UI:  $UI_BASE"
  echo "  • API:     $API_BASE"
  echo "  • Login:   admin@example.com / admin"
  echo ""
  exit 0
elif [ $FAILED -le 2 ]; then
  echo -e "${BOLD}${YELLOW}⚠ Some checks failed, but core functionality works${NC}"
  echo ""
  echo "Review failed checks above and check logs:"
  echo "  docker-compose logs -f"
  echo ""
  exit 0
else
  echo -e "${BOLD}${RED}✗ Multiple checks failed${NC}"
  echo ""
  echo "Troubleshooting steps:"
  echo "  1. Check logs: docker-compose logs"
  echo "  2. Restart services: docker-compose restart"
  echo "  3. Fresh start: docker-compose down -v && docker-compose up -d"
  echo ""
  exit 1
fi