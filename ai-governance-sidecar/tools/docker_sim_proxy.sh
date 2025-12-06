#!/usr/bin/env bash
set -euo pipefail

API_BASE="${API_BASE:-http://governance-sidecar:8080}"
EMAIL="${ADMIN_EMAIL:-admin@example.com}"
PASSWORD="${ADMIN_PASSWORD:-admin}"
APPROVER="${APPROVER_NAME:-Automated System}"
STARTUP_DELAY="${STARTUP_DELAY:-20}"
SIMULATION_DELAY="${SIMULATION_DELAY:-20}"

echo "================================================"
echo "Hospital Simulation (Using /simulate/enqueue)"
echo "================================================"
echo "API: $API_BASE"
echo ""

# Wait for services
echo "Waiting ${STARTUP_DELAY}s for services..."
sleep "$STARTUP_DELAY"

# Check health
echo "Checking backend health..."
for i in {1..30}; do
  if curl -sf "$API_BASE/health" >/dev/null 2>&1; then
    echo "✓ Backend healthy"
    break
  fi
  sleep 2
done

# Delay before simulation
echo ""
echo "Waiting ${SIMULATION_DELAY}s before starting..."
sleep "$SIMULATION_DELAY"
echo "✓ Starting simulation now!"
echo ""

# Authenticate
LOGIN_RESPONSE=$(curl -sf -X POST "$API_BASE/login" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"${EMAIL}\",\"password\":\"${PASSWORD}\"}" 2>&1 || echo '{}')

TOKEN=$(echo "$LOGIN_RESPONSE" | jq -r '.token // empty' 2>/dev/null || echo "")

if [ -z "$TOKEN" ] || [ "$TOKEN" = "null" ]; then
  echo "✗ Authentication failed" >&2
  exit 1
fi

echo "✓ Authenticated"
echo ""

# Process workflow using /simulate/enqueue
process_workflow() {
  local tool=$1
  local args=$2
  local description=$3
  
  echo "Processing: $description"
  
  # Enqueue approval request
  ENQUEUE_RESPONSE=$(curl -sf -X POST "$API_BASE/simulate/enqueue" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer ${TOKEN}" \
    -d "{\"tool_name\":\"${tool}\",\"args\":${args},\"reason\":\"${description}\"}" 2>&1 || echo '{"error":"enqueue_failed"}')
  
  APPROVAL_ID=$(echo "$ENQUEUE_RESPONSE" | jq -r '.approval_id // empty' 2>/dev/null || echo "")
  
  if [ -z "$APPROVAL_ID" ] || [ "$APPROVAL_ID" = "null" ]; then
    echo "  ✗ Enqueue failed"
    echo "  Response: $ENQUEUE_RESPONSE"
    return 1
  fi
  
  echo "  ✓ Enqueued: $APPROVAL_ID"
  
  sleep 0.5
  
  # Auto-approve
  APPROVE_RESPONSE=$(curl -sf -X POST "$API_BASE/approvals/${APPROVAL_ID}/approve" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer ${TOKEN}" \
    -d "{\"approver\":\"${APPROVER}\",\"comment\":\"Automated: $description\"}" 2>&1 || echo '{}')
  
  if echo "$APPROVE_RESPONSE" | jq -e '.success' >/dev/null 2>&1; then
    STATUS=$(echo "$APPROVE_RESPONSE" | jq -r '.status // empty' 2>/dev/null || echo "")
    echo "  ✓ ${STATUS^}"
  else
    echo "  ⚠ Approval response: $APPROVE_RESPONSE"
  fi
  
  sleep 1
}

echo "================================================"
echo "Hospital Workflow"
echo "================================================"
echo ""

# EHR Access
process_workflow "get_patient_record" \
  '{"patient_id":"HOSP-1001","action":"read","query":"full_record"}' \
  "EHR Access"

# Prescription
process_workflow "send_prescription" \
  '{"patient_id":"HOSP-1001","medication":"Amoxicillin","dose":"500mg","note":"standard course"}' \
  "Prescription"

# Billing
process_workflow "charge_insurance" \
  '{"patient_id":"HOSP-1001","claim_id":"CLM-9001","amount":1250.00}' \
  "Insurance Billing"

# Lab Results
process_workflow "fetch_lab_results" \
  '{"patient_id":"HOSP-1001","test":"CBC"}' \
  "Lab Results"

# Summary
echo ""
echo "================================================"
echo "Simulation Complete ✓"
echo "================================================"

AUDIT=$(curl -sf "$API_BASE/audit?limit=10" \
  -H "Authorization: Bearer ${TOKEN}" 2>&1 || echo '{"entries":[]}')

AUDIT_COUNT=$(echo "$AUDIT" | jq '.entries | length' 2>/dev/null || echo "0")
echo ""
echo "Audit entries: $AUDIT_COUNT"

if [ "$AUDIT_COUNT" -gt 0 ]; then
  echo "$AUDIT" | jq -r '.entries[] | "  [\(.decision)] \(.tool_input.tool_name // "unknown")"' 2>/dev/null | head -10 || true
fi

echo ""
echo "✓ Simulation finished"
echo "View at: http://localhost:3000"