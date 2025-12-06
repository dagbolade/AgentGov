#!/usr/bin/env bash
set -euo pipefail

BOLD='\033[1m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${BOLD}${BLUE}╔════════════════════════════════════════════╗${NC}"
echo -e "${BOLD}${BLUE}║  AI Governance Sidecar - Quick Deploy     ║${NC}"
echo -e "${BOLD}${BLUE}╚════════════════════════════════════════════╝${NC}"
echo ""

# Check Docker
if ! command -v docker &> /dev/null; then
    echo -e "${RED}✗ Docker not found${NC}"
    echo "Install Docker from: https://docs.docker.com/get-docker/"
    exit 1
fi

if ! command -v docker-compose &> /dev/null && ! docker compose version &> /dev/null 2>&1; then
    echo -e "${RED}✗ Docker Compose not found${NC}"
    echo "Install Docker Compose or use Docker with compose plugin"
    exit 1
fi

# Detect docker-compose command
if docker compose version &> /dev/null 2>&1; then
    DOCKER_COMPOSE="docker compose"
else
    DOCKER_COMPOSE="docker-compose"
fi

echo -e "${GREEN}✓ Docker and Docker Compose detected${NC}"
echo ""

# Menu
echo -e "${BOLD}Select deployment mode:${NC}"
echo ""
echo "  1) ${GREEN}Full Auto${NC} - Services + Automated simulation (no clicks)"
echo "  2) ${YELLOW}Services Only${NC} - UI + Backend (manual testing)"
echo "  3) ${BLUE}Stop All${NC} - Stop and remove containers"
echo "  4) ${BLUE}View Logs${NC} - Show service logs"
echo ""
read -p "Enter choice [1-4]: " choice

case $choice in
  1)
    echo ""
    echo -e "${BOLD}${GREEN}Starting Full Automated Deployment${NC}"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    
    # Stop any existing containers
    echo "Cleaning up existing containers..."
    $DOCKER_COMPOSE down 2>/dev/null || true
    
    # Start services
    echo ""
    echo -e "${BOLD}Starting services...${NC}"
    $DOCKER_COMPOSE up -d governance-sidecar governance-ui
    
    # Wait for services
    echo ""
    echo "Waiting for services to be healthy..."
    sleep 5
    
    # Check health
    for i in {1..30}; do
      if curl -sf http://localhost:8080/health >/dev/null 2>&1; then
        echo -e "${GREEN}✓ Backend healthy${NC}"
        break
      fi
      if [ $i -eq 30 ]; then
        echo -e "${RED}✗ Backend health check timeout${NC}"
        exit 1
      fi
      sleep 1
    done
    
    for i in {1..30}; do
      if curl -sf http://localhost:3000/ >/dev/null 2>&1; then
        echo -e "${GREEN}✓ Frontend healthy${NC}"
        break
      fi
      if [ $i -eq 30 ]; then
        echo -e "${RED}✗ Frontend health check timeout${NC}"
        exit 1
      fi
      sleep 1
    done
    
    # Run simulation
    echo ""
    echo -e "${BOLD}Running hospital simulation...${NC}"
    
    if [ -f "./tools/hospital_sim_automated.sh" ]; then
      chmod +x ./tools/hospital_sim_automated.sh
      ./tools/hospital_sim_automated.sh
    else
      echo -e "${YELLOW}⚠ Simulation script not found at ./tools/hospital_sim_automated.sh${NC}"
      echo "Services are running. You can test manually at http://localhost:3000"
    fi
    
    echo ""
    echo -e "${BOLD}${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BOLD}${GREEN}✓ Deployment Complete${NC}"
    echo -e "${BOLD}${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""
    echo -e "${BOLD}Access Points:${NC}"
    echo "  • Web UI:    ${BLUE}http://localhost:3000${NC}"
    echo "  • API:       ${BLUE}http://localhost:8080${NC}"
    echo "  • Login:     ${GREEN}admin@example.com${NC} / ${GREEN}admin${NC}"
    echo ""
    echo -e "${BOLD}Useful Commands:${NC}"
    echo "  • View logs:       ${YELLOW}$DOCKER_COMPOSE logs -f${NC}"
    echo "  • Stop services:   ${YELLOW}$DOCKER_COMPOSE down${NC}"
    echo "  • Restart:         ${YELLOW}$DOCKER_COMPOSE restart${NC}"
    ;;
    
  2)
    echo ""
    echo -e "${BOLD}${YELLOW}Starting Services Only${NC}"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    
    $DOCKER_COMPOSE down 2>/dev/null || true
    $DOCKER_COMPOSE up -d governance-sidecar governance-ui
    
    echo ""
    echo "Waiting for services..."
    sleep 10
    
    echo ""
    echo -e "${BOLD}${GREEN}✓ Services Started${NC}"
    echo ""
    echo -e "${BOLD}Access Points:${NC}"
    echo "  • Web UI:    ${BLUE}http://localhost:3000${NC}"
    echo "  • API:       ${BLUE}http://localhost:8080${NC}"
    echo "  • Login:     ${GREEN}admin@example.com${NC} / ${GREEN}admin${NC}"
    echo ""
    echo -e "${BOLD}Manual Testing:${NC}"
    echo "  1. Open http://localhost:3000"
    echo "  2. Login with credentials above"
    echo "  3. Create test approvals via API or UI"
    ;;
    
  3)
    echo ""
    echo -e "${BOLD}${BLUE}Stopping All Services${NC}"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    
    $DOCKER_COMPOSE down -v
    
    echo ""
    echo -e "${GREEN}✓ All services stopped${NC}"
    ;;
    
  4)
    echo ""
    echo -e "${BOLD}${BLUE}Service Logs${NC}"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    echo "Press Ctrl+C to exit logs"
    echo ""
    sleep 2
    
    $DOCKER_COMPOSE logs -f
    ;;
    
  *)
    echo ""
    echo -e "${RED}Invalid choice${NC}"
    exit 1
    ;;
esac