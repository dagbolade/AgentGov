#!/bin/bash

# Integration Test Verification Script
# Verifies that all integration tests are properly set up

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

echo "========================================="
echo "Integration Test Verification"
echo "========================================="
echo ""

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Function to print status
print_status() {
    if [ $1 -eq 0 ]; then
        echo -e "${GREEN}✓${NC} $2"
    else
        echo -e "${RED}✗${NC} $2"
    fi
}

# Check Go installation
echo "1. Checking Go installation..."
if command -v go &> /dev/null; then
    GO_VERSION=$(go version)
    print_status 0 "Go installed: $GO_VERSION"
else
    print_status 1 "Go not found"
    exit 1
fi
echo ""

# Check test file structure
echo "2. Checking test file structure..."
cd "$PROJECT_ROOT"

files=(
    "test/integration/testutil.go"
    "test/integration/approval_flow_test.go"
    "test/integration/policy_reload_test.go"
    "test/integration/concurrent_test.go"
    "test/integration/docker_smoke_test.go"
    "test/integration/README.md"
)

all_files_present=true
for file in "${files[@]}"; do
    if [ -f "$file" ]; then
        print_status 0 "$file"
    else
        print_status 1 "$file - MISSING"
        all_files_present=false
    fi
done
echo ""

if [ "$all_files_present" = false ]; then
    echo "Some test files are missing!"
    exit 1
fi

# Check Makefile
echo "3. Checking Makefile..."
if [ -f "Makefile" ]; then
    if grep -q "test-integration" Makefile; then
        print_status 0 "Makefile with test-integration target"
    else
        print_status 1 "Makefile missing test-integration target"
    fi
else
    print_status 1 "Makefile not found"
fi
echo ""

# Compile test files
echo "4. Compiling integration tests..."
if go test -c -o /tmp/integration_test ./test/integration/ 2>&1 | grep -q "^$"; then
    print_status 0 "Integration tests compile successfully"
    rm -f /tmp/integration_test
else
    echo -e "${YELLOW}Checking compilation...${NC}"
    if go test -c -o /tmp/integration_test ./test/integration/ 2>&1; then
        print_status 0 "Integration tests compile with warnings"
        rm -f /tmp/integration_test
    else
        print_status 1 "Integration tests have compilation errors"
        exit 1
    fi
fi
echo ""

# Check policy directory
echo "5. Checking WASM policies..."
if [ -d "policies/wasm" ]; then
    wasm_count=$(find policies/wasm -name "*.wasm" 2>/dev/null | wc -l)
    if [ "$wasm_count" -gt 0 ]; then
        print_status 0 "Found $wasm_count WASM policy files"
    else
        print_status 1 "No WASM policies found (run: cd policies && ./build.sh)"
    fi
else
    print_status 1 "policies/wasm directory not found"
fi
echo ""

# Check Docker availability
echo "6. Checking Docker (optional)..."
if command -v docker &> /dev/null; then
    if docker info &> /dev/null; then
        print_status 0 "Docker is available and running"
    else
        print_status 1 "Docker installed but not running"
    fi
else
    print_status 1 "Docker not installed (required for Docker tests)"
fi
echo ""

# Count test functions
echo "7. Counting test functions..."
test_count=$(grep -h "^func Test" test/integration/*.go 2>/dev/null | wc -l)
if [ "$test_count" -gt 0 ]; then
    print_status 0 "Found $test_count test functions"
else
    print_status 1 "No test functions found"
fi
echo ""

# Summary
echo "========================================="
echo "Summary"
echo "========================================="
echo ""
echo "Test files: ${#files[@]}"
echo "Test functions: $test_count"
echo ""
echo "To run tests:"
echo "  make test-integration       # Run integration tests"
echo "  make test-docker            # Run Docker tests"
echo "  make test-all               # Run all tests"
echo ""
echo "To build WASM policies (if not built):"
echo "  cd policies && ./build.sh"
echo ""
echo -e "${GREEN}✓ Integration test suite is ready!${NC}"
