#!/bin/bash

set -e

BOLD='\033[1m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BOLD}${BLUE}AI Governance Sidecar - Bootstrap${NC}"
echo "===================================="
echo ""

# Check prerequisites
check_prerequisites() {
    echo -e "${BOLD}Checking prerequisites...${NC}"
    
    missing=()
    
    if ! command -v docker &> /dev/null; then
        missing+=("docker")
    fi
    
    if ! command -v docker-compose &> /dev/null; then
        missing+=("docker-compose")
    fi
    
    if ! command -v make &> /dev/null; then
        missing+=("make")
    fi
    
    if ! command -v rustc &> /dev/null; then
        echo -e "${YELLOW}⚠ Rust not found. Install with: curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh${NC}"
        missing+=("rust")
    fi
    
    if [ ${#missing[@]} -gt 0 ]; then
        echo -e "${YELLOW}Missing prerequisites: ${missing[*]}${NC}"
        echo "Please install missing tools and re-run this script."
        exit 1
    fi
    
    echo -e "${GREEN}✓ All prerequisites met${NC}"
}

# Setup project structure
setup_structure() {
    echo -e "\n${BOLD}Setting up project structure...${NC}"
    
    # Create directories
    mkdir -p policies/wasm
    mkdir -p db
    
    # Create .env if it doesn't exist
    if [ ! -f .env ]; then
        cat > .env << EOF
TOOL_UPSTREAM=http://host.docker.internal:9000
ENABLE_GRPC=false
LOG_LEVEL=info
APPROVAL_TIMEOUT_MINUTES=60
EOF
        echo -e "${GREEN}✓ Created .env file${NC}"
    else
        echo -e "${YELLOW}⚠ .env already exists, skipping${NC}"
    fi
    
    echo -e "${GREEN}✓ Project structure ready${NC}"
}

# Build WASM policies
build_policies() {
    echo -e "\n${BOLD}Building WASM policies...${NC}"
    
    if ! command -v cargo &> /dev/null; then
        echo -e "${YELLOW}⚠ Cargo not found, skipping policy build${NC}"
        echo "You can build policies later with: make policies"
        return
    fi
    
    # Add wasm target
    rustup target add wasm32-unknown-unknown 2>/dev/null || true
    
    # Build policies
    if [ -d "policies" ] && [ -f "policies/Cargo.toml" ]; then
        cd policies
        cargo build --release --target wasm32-unknown-unknown
        
        # Copy WASM files
        mkdir -p wasm
        find target/wasm32-unknown-unknown/release -name "*.wasm" -exec cp {} wasm/ \;
        cd ..
        
        echo -e "${GREEN}✓ Policies built successfully${NC}"
        ls -lh policies/wasm/
    else
        echo -e "${YELLOW}⚠ Policy source files not found${NC}"
    fi
}

# Build and start Docker services
start_services() {
    echo -e "\n${BOLD}Building and starting services...${NC}"
    
    # Build Docker image
    docker-compose build
    
    # Start services
    docker-compose up -d
    
    echo -e "${GREEN}✓ Services started${NC}"
}

# Wait for service to be healthy
wait_for_health() {
    echo -e "\n${BOLD}Waiting for service to be healthy...${NC}"
    
    max_attempts=30
    attempt=0
    
    while [ $attempt -lt $max_attempts ]; do
        if curl -s http://localhost:8080/health > /dev/null 2>&1; then
            echo -e "${GREEN}✓ Service is healthy${NC}"
            return 0
        fi
        
        echo -n "."
        sleep 1
        attempt=$((attempt + 1))
    done
    
    echo -e "\n${YELLOW}⚠ Service not responding, but may still be starting${NC}"
    echo "Check logs with: docker-compose logs -f"
}

# Run basic tests
run_tests() {
    echo -e "\n${BOLD}Running basic tests...${NC}"
    
    # Health check
    echo "1. Health check:"
    curl -s http://localhost:8080/health | jq . || echo "jq not installed"
    
    # Simple policy check
    echo -e "\n2. Policy check:"
    curl -s -X POST http://localhost:8080/check \
        -H "Content-Type: application/json" \
        -d '{"tool":"test","action":"read","parameters":{}}' | jq . || echo "jq not installed"
    
    echo -e "\n${GREEN}✓ Basic tests completed${NC}"
}

# Print next steps
print_next_steps() {
    echo -e "\n${BOLD}${GREEN}============================================${NC}"
    echo -e "${BOLD}${GREEN}    Setup Complete! 🚀${NC}"
    echo -e "${BOLD}${GREEN}============================================${NC}"
    echo ""
    echo -e "${BOLD}Service is running at:${NC}"
    echo "  • HTTP API: http://localhost:8080"
    echo "  • Health: http://localhost:8080/health"
    echo "  • WebSocket: ws://localhost:8080/ws"
    echo ""
    echo -e "${BOLD}Next steps:${NC}"
    echo "  1. Run comprehensive tests:"
    echo "     ${BLUE}./test-approval-flow.sh${NC}"
    echo ""
    echo "  2. View logs:"
    echo "     ${BLUE}make logs${NC}"
    echo ""
    echo "  3. Check pending approvals:"
    echo "     ${BLUE}curl http://localhost:8080/approvals/pending${NC}"
    echo ""
    echo "  4. Stop services:"
    echo "     ${BLUE}make docker-down${NC}"
    echo ""
    echo -e "${BOLD}Documentation:${NC}"
    echo "  • README.md - Quick start guide"
    echo "  • SETUP.md - Detailed setup instructions"
    echo ""
}

# Main execution
main() {
    check_prerequisites
    setup_structure
    build_policies
    start_services
    wait_for_health
    run_tests
    print_next_steps
}

# Run main function
main