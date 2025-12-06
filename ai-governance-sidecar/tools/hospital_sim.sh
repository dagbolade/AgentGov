#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
API_BASE="http://localhost:3000/"

# Credentials
EMAIL="admin@example.com"
PASSWORD="admin"

echo "Running hospital simulation against $API_BASE"

# Wait for health
for i in {1..30}; do
  if curl -s "$API_BASE/health" >/dev/null 2>&1; then
    break
  fi
  echo "Waiting for the sidecar to be healthy ($i)..."
  sleep 1
done

echo "Logging in as $EMAIL"
TOKEN=$(curl -s -X POST "$API_BASE/login" -H "Content-Type: application/json" \
  -d "{\"email\":\"${EMAIL}\",\"password\":\"${PASSWORD}\"}")
TOKEN=$(echo "$TOKEN" | jq -r .token)
if [ -z "$TOKEN" ] || [ "$TOKEN" = "null" ]; then
  echo "Failed to get token from /login" >&2
  exit 1
fi
AUTH_HEADER=( -H "Authorization: Bearer ${TOKEN}" )

echo "Logged in. Enqueuing approvals..."

enqueue() {
  local payload=$1
  echo "Enqueueing: $payload"
  curl -s -X POST "$API_BASE/simulate/enqueue" -H "Content-Type: application/json" "${AUTH_HEADER[@]}" -d "$payload" | jq .
}

# 1) EHR access request
pay1='{"tool_name":"get_patient_record","args":{"patient_id":"HOSP-1001","action":"read","query":"full_record"},"reason":"ehr-access"}'
enqueue "$pay1"

# 2) Prescription send
pay2='{"tool_name":"send_prescription","args":{"patient_id":"HOSP-1001","medication":"Amoxicillin","dose":"500mg","note":"standard course"},"reason":"prescription"}'
enqueue "$pay2"

# 3) Billing charge
pay3='{"tool_name":"charge_insurance","args":{"patient_id":"HOSP-1001","claim_id":"CLM-9001","amount":1250.00},"reason":"billing"}'
enqueue "$pay3"

# 4) Lab results fetch
pay4='{"tool_name":"fetch_lab_results","args":{"patient_id":"HOSP-1001","test":"CBC"},"reason":"lab-fetch"}'
enqueue "$pay4"

echo
echo "Done. Pending approvals are listed below." 
echo "Open the web UI at http://localhost:3000 to review and approve them (user: admin@example.com / admin)."
echo "To list pending approvals from the command line run:"
echo "  curl -s -H \"Authorization: Bearer $TOKEN\" http://localhost:8080/approvals?status=pending | jq ."

echo "Pending approvals (brief):"
curl -sS -X GET "$API_BASE/approvals?status=pending" "${AUTH_HEADER[@]}" | jq .

echo
echo "Approve the items in the UI to continue the forwarding flow."
