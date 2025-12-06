#!/usr/bin/env bash
set -euo pipefail

# Docker-specific configuration
API_BASE="${API_BASE:-http://governance-sidecar:8080}"
EMAIL="${ADMIN_EMAIL:-admin@example.com}"
PASSWORD="${ADMIN_PASSWORD:-admin}"
APPROVER="${APPROVER_NAME:-Automated System}"
WAIT_TIME="${WAIT_TIME:-10}"

echo "================================================"
echo "Hospital Simulation (Docker Mode)"
echo "================================================"
echo "API: $API_BASE"
echo "Wait time before starting: ${WAIT_TIME}s"
echo ""

# Wait for services to be fully ready
echo "Waiting ${WAIT_TIME}s for services to stabilize..."
sleep "$WAIT_TIME"

# Wait for backend health with retries
echo "Checking backend health..."
MAX_RETRIES=60
RETRY_COUNT=0

while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
  if curl -sf "$API_BASE/health" >/dev/null 2>&1; then
    echo "✓ Backend is healthy"
    break
  fi
  
  RETRY_COUNT=$((RETRY_COUNT + 1))
  
  if [ $RETRY_COUNT -eq $MAX_RETRIES ]; then
    echo "✗ Backend health check timeout after $MAX_RETRIES attempts" >&2
    exit 1
  fi
  
  echo "  Attempt $RETRY_COUNT/$MAX_RETRIES..."
  sleep 2
done

# Login
echo ""
echo "Logging in..."
LOGIN_RESPONSE=$(curl -sf -X POST "$API_BASE/login" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"${EMAIL}\",\"password\":\"${PASSWORD}\"}" 2>&1 || echo '{"error":"login_failed"}')

TOKEN=$(echo "$LOGIN_RESPONSE" | jq -r '.token // empty' 2>/dev/null || echo "")

if [ -z "$TOKEN" ] || [ "$TOKEN" = "null" ]; then
  echo "✗ Login failed" >&2
  echo "Response: $LOGIN_RESPONSE" >&2
  exit 1
fi

echo "✓ Logged in"

# Process hospital workflow
process_workflow() {
  local payload=$1
  local description=$2
  
  echo ""
  echo "Processing: $description"
  
  # Enqueue
  ENQUEUE_RESPONSE=$(curl -sf -X POST "$API_BASE/simulate/enqueue" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer ${TOKEN}" \
    -d "$payload" 2>&1 || echo '{"error":"enqueue_failed"}')
  
  APPROVAL_ID=$(echo "$ENQUEUE_RESPONSE" | jq -r '.approval_id // empty' 2>/dev/null || echo "")
  
  if [ -z "$APPROVAL_ID" ] || [ "$APPROVAL_ID" = "null" ]; then
    echo "  ✗ Enqueue failed" >&2
    return 1
  fi
  
  echo "  ✓ Enqueued: $APPROVAL_ID"
  
  # Auto-approve
  APPROVE_RESPONSE=$(curl -sf -X POST "$API_BASE/approvals/${APPROVAL_ID}/approve" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer ${TOKEN}" \
    -d "{\"approver\":\"${APPROVER}\",\"comment\":\"Automated: $description\"}" 2>&1 || echo '{"error":"approve_failed"}')
  
  STATUS=$(echo "$APPROVE_RESPONSE" | jq -r '.status // empty' 2>/dev/null || echo "")
  
  if [ "$STATUS" = "approved" ]; then
    echo "  ✓ Approved"
  else
    echo "  ⚠ Approval status: $STATUS"
  fi
  
  sleep 1
}

echo ""
echo "================================================"
echo "Hospital Workflow"
echo "================================================"

# Execute workflow
process_workflow '{"tool_name":"get_patient_record","args":{"patient_id":"HOSP-1001","action":"read","query":"full_record"},"reason":"ehr-access"}' "EHR Access"
process_workflow '{"tool_name":"send_prescription","args":{"patient_id":"HOSP-1001","medication":"Amoxicillin","dose":"500mg"},"reason":"prescription"}' "Prescription"
process_workflow '{"tool_name":"charge_insurance","args":{"patient_id":"HOSP-1001","claim_id":"CLM-9001","amount":1250.00},"reason":"billing"}' "Insurance Billing"
process_workflow '{"tool_name":"fetch_lab_results","args":{"patient_id":"HOSP-1001","test":"CBC"},"reason":"lab-fetch"}' "Lab Results"

# Summary
echo ""
echo "================================================"
echo "Simulation Complete"
echo "================================================"

# Get final audit log
echo ""
echo "Audit Summary:"
AUDIT=$(curl -sf "$API_BASE/audit?limit=10" \
  -H "Authorization: Bearer ${TOKEN}" 2>&1 || echo '{"entries":[]}')

AUDIT_COUNT=$(echo "$AUDIT" | jq '.entries | length' 2>/dev/null || echo "0")
echo "Total entries: $AUDIT_COUNT"

if [ "$AUDIT_COUNT" -gt 0 ]; then
  echo "$AUDIT" | jq -r '.entries[] | "  [\(.status)] \(.request.tool // "unknown") - \(.approver // "system")"' 2>/dev/null || true
fi

echo ""
echo "✓ Simulation finished successfully"
echo "View details at: http://localhost:3000"