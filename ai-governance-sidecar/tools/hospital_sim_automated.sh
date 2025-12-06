#!/usr/bin/env bash
set -euo pipefail

# --- Configuration ---
API_BASE="${API_BASE:-http://localhost:8080}"
EMAIL="${ADMIN_EMAIL:-admin@example.com}"
PASSWORD="${ADMIN_PASSWORD:-admin}"
APPROVER="${APPROVER_NAME:-GovernanceBot}"
ITERATIONS=50  # Extensive run

# --- Colors ---
GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

echo -e "${BLUE}╔══════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║  EXTENSIVE AI AGENT SIMULATION: Mixed & Randomized Workload  ║${NC}"
echo -e "${BLUE}╚══════════════════════════════════════════════════════════════╝${NC}"

# 1. Health Check
echo -n "Checking Backend... "
for i in {1..30}; do
  if curl -sf "$API_BASE/health" >/dev/null 2>&1; then
    echo -e "${GREEN}Online${NC}"
    break
  fi
  sleep 1
done

# 2. Login
echo -n "Authenticating... "
TOKEN=$(curl -s -X POST "$API_BASE/login" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"${EMAIL}\",\"password\":\"${PASSWORD}\"}" | jq -r '.token // empty')

if [ -z "$TOKEN" ]; then echo -e "${RED}Failed${NC}"; exit 1; fi
echo -e "${GREEN}Success${NC}"
AUTH=(-H "Authorization: Bearer ${TOKEN}")

# --- Helper: Generate Random ID ---
rand_id() { echo "REQ-$((1000 + RANDOM % 9000))"; }

# --- Helper: Execute ---
process_transaction() {
    local risk=$1
    local tool=$2
    local reason=$3
    local payload=$4
    local decision=$5 # approve, deny, pending
    
    # 1. Enqueue
    echo -n "  Processing: $tool "
    ENQ=$(curl -s -X POST "$API_BASE/simulate/enqueue" \
        -H "Content-Type: application/json" "${AUTH[@]}" \
        -d "{\"tool_name\":\"$tool\",\"reason\":\"$reason\",\"args\":$payload}")
    ID=$(echo "$ENQ" | jq -r '.approval_id // empty')
    
    if [ -z "$ID" ]; then echo -e "${RED}ERROR${NC}"; return; fi
    
    # 2. Handle Decision
    if [ "$decision" == "pending" ]; then
        echo -e "--> ${BLUE}LEFT PENDING${NC} ($risk)"
        return
    fi
    
    # Auto Decide
    local comment="Automated governance decision"
    local color=$GREEN
    local status="APPROVED"
    
    if [ "$decision" == "deny" ]; then
        color=$RED
        status="BLOCKED"
        comment="Security Policy Violation: $risk risk detected"
    fi
    
    curl -s -X POST "$API_BASE/approvals/$ID/$decision" \
        -H "Content-Type: application/json" "${AUTH[@]}" \
        -d "{\"approver\":\"$APPROVER\",\"comment\":\"$comment\"}" > /dev/null
        
    echo -e "--> ${color}${status}${NC}"
}

# --- Tool Dictionaries ---
SAFE_TOOLS=("read_logs" "check_health" "get_metrics" "list_locations")
SENSITIVE_TOOLS=("access_patient_pii" "export_report" "update_config")
CRITICAL_TOOLS=("transfer_funds" "grant_admin" "decrypt_database")
DESTRUCTIVE_TOOLS=("drop_table" "delete_user" "factory_reset")

echo ""
echo "Starting Traffic Generation ($ITERATIONS requests)..."
echo "--------------------------------------------------------"

for ((i=1; i<=ITERATIONS; i++)); do
    # Vary traffic speed (Burst simulation)
    if [ $((i % 10)) -eq 0 ]; then
        echo -e "${YELLOW}...Traffic Spike...${NC}"
        sleep 0.1
    else
        sleep 0.5
    fi

    ROLL=$((RANDOM % 100))
    
    # --- LOGIC ENGINE ---
    
    # 40% SAFE -> Auto Approve
    if [ $ROLL -lt 40 ]; then
        TOOL=${SAFE_TOOLS[$((RANDOM % ${#SAFE_TOOLS[@]}))]}
        process_transaction "LOW" "$TOOL" "Routine Operation" \
            "{\"id\":\"$(rand_id)\",\"lines\":50}" "approve"

    # 20% SENSITIVE -> Leave Pending
    elif [ $ROLL -lt 60 ]; then
        TOOL=${SENSITIVE_TOOLS[$((RANDOM % ${#SENSITIVE_TOOLS[@]}))]}
        process_transaction "MEDIUM" "$TOOL" "Requires Oversight" \
            "{\"patient_id\":\"$(rand_id)\",\"access\":\"read\"}" "pending"

    # 20% CRITICAL -> Leave Pending
    elif [ $ROLL -lt 80 ]; then
        TOOL=${CRITICAL_TOOLS[$((RANDOM % ${#CRITICAL_TOOLS[@]}))]}
        process_transaction "HIGH" "$TOOL" "High Value Transaction" \
            "{\"amount\":50000,\"currency\":\"USD\"}" "pending"

    # 20% DESTRUCTIVE -> Auto Deny
    else
        TOOL=${DESTRUCTIVE_TOOLS[$((RANDOM % ${#DESTRUCTIVE_TOOLS[@]}))]}
        process_transaction "EXTREME" "$TOOL" "Malicious Activity" \
            "{\"target\":\"all\",\"force\":true}" "deny"
    fi
done

echo ""
echo "--------------------------------------------------------"
echo -e "${GREEN}Simulation Complete.${NC}"
echo "Check the 'Pending Approvals' tab in your UI for the items left for review."