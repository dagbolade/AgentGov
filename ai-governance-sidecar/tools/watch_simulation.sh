#!/usr/bin/env bash

BOLD='\033[1m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

clear

echo -e "${BOLD}${BLUE}╔═══════════════════════════════════════════════════╗${NC}"
echo -e "${BOLD}${BLUE}║  Hospital Simulation - Live Status Monitor       ║${NC}"
echo -e "${BOLD}${BLUE}╚═══════════════════════════════════════════════════╝${NC}"
echo ""

# Check if simulator is running
if ! docker-compose ps hospital-simulator 2>/dev/null | grep -q "Up\|running"; then
  echo -e "${YELLOW}⚠ Simulator container not running${NC}"
  echo ""
  echo "To start with simulation:"
  echo "  docker-compose --profile simulation up -d"
  echo ""
  exit 1
fi

echo -e "${GREEN}✓ Simulator is running${NC}"
echo ""
echo "Monitoring simulator logs (Press Ctrl+C to exit)..."
echo ""
echo "─────────────────────────────────────────────────────"
echo ""

# Follow simulator logs with color
docker-compose logs -f --tail=50 hospital-simulator 2>&1 | while IFS= read -r line; do
  # Highlight countdown
  if echo "$line" | grep -q "Starting in"; then
    echo -e "${YELLOW}${line}${NC}"
  # Highlight success
  elif echo "$line" | grep -q "✓"; then
    echo -e "${GREEN}${line}${NC}"
  # Highlight errors
  elif echo "$line" | grep -q "✗\|error\|failed"; then
    echo -e "\033[0;31m${line}${NC}"
  # Highlight section headers
  elif echo "$line" | grep -q "===="; then
    echo -e "${BOLD}${BLUE}${line}${NC}"
  # Normal output
  else
    echo "$line"
  fi
done