.PHONY: help build test test-unit test-integration test-docker test-all coverage coverage-integration clean run docker-build docker-up docker-down policies lint fmt

# Default target
help:
	@echo "AI Governance Sidecar - Available targets:"
	@echo ""
	@echo "  make build              - Build the Go binary"
	@echo "  make test               - Run all tests (unit + integration)"
	@echo "  make test-unit          - Run unit tests only"
	@echo "  make test-integration   - Run integration tests"
	@echo "  make test-docker        - Run Docker Compose smoke tests"
	@echo "  make test-all           - Run all tests with race detector"
	@echo "  make coverage           - Generate test coverage report"
	@echo "  make coverage-integration - Integration test coverage"
	@echo "  make bench              - Run benchmarks"
	@echo "  make clean              - Clean build artifacts"
	@echo "  make run                - Run the sidecar locally"
	@echo "  make docker-build       - Build Docker images"
	@echo "  make docker-up          - Start Docker Compose stack"
	@echo "  make docker-down        - Stop Docker Compose stack"
	@echo "  make policies           - Build WASM policies"
	@echo "  make lint               - Run linter"
	@echo "  make fmt                - Format code"
	@echo ""

# Build the Go binary
build:
	@echo "Building AI Governance Sidecar..."
	go build -o bin/sidecar ./cmd/sidecar
	@echo "Build complete: bin/sidecar"

# Run all tests
test: test-unit test-integration
	@echo "All tests completed!"

# Run unit tests only
test-unit:
	@echo "Running unit tests..."
	go test -v -short ./internal/...
	@echo "Unit tests completed!"

# Run integration tests (excluding Docker tests)
test-integration: policies
	@echo "Running integration tests..."
	go test -v -short -timeout 5m ./test/integration/...
	@echo "Integration tests completed!"

# Run Docker Compose smoke tests
test-docker: policies
	@echo "Running Docker Compose smoke tests..."
	go test -v -run TestDocker -timeout 10m ./test/integration/...
	@echo "Docker tests completed!"

# Run all tests with race detector
test-all: policies
	@echo "Running all tests with race detector..."
	go test -v -race -timeout 10m ./...
	@echo "All tests with race detector completed!"

# Run tests with coverage
coverage:
	@echo "Generating test coverage..."
	go test -v -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"
	go tool cover -func=coverage.out | grep total:

# Integration test coverage
coverage-integration: policies
	@echo "Generating integration test coverage..."
	go test -v -coverprofile=coverage-integration.out -covermode=atomic ./test/integration/...
	go tool cover -html=coverage-integration.out -o coverage-integration.html
	@echo "Integration coverage report: coverage-integration.html"
	go tool cover -func=coverage-integration.out | grep total:

# Run benchmarks
bench:
	@echo "Running benchmarks..."
	go test -bench=. -benchmem ./...

# Clean build artifacts
clean:
	@echo "Cleaning up..."
	rm -rf bin/
	rm -f coverage.out coverage.html
	rm -f coverage-integration.out coverage-integration.html
	rm -rf test/integration/tmp_*
	go clean -cache -testcache
	@echo "Clean complete!"

# Run the sidecar locally
run: policies
	@echo "Starting AI Governance Sidecar..."
	@if [ ! -f .env ]; then echo "Warning: .env file not found"; fi
	@mkdir -p db
	go run ./cmd/sidecar

# Build Docker images
docker-build:
	@echo "Building Docker images..."
	docker-compose build
	@echo "Docker build complete!"

# Start Docker Compose stack
docker-up: policies docker-build
	@echo "Starting Docker Compose stack..."
	docker-compose up -d
	@echo "Stack started. Services available at:"
	@echo "  Backend: http://localhost:8080"
	@echo "  UI:      http://localhost:3000"
	@echo ""
	@echo "Check status: docker-compose ps"
	@echo "View logs:    docker-compose logs -f"

# Stop Docker Compose stack
docker-down:
	@echo "Stopping Docker Compose stack..."
	docker-compose down -v
	@echo "Stack stopped!"

# Build WASM policies
policies:
	@echo "Building WASM policies..."
	@cd policies && ./build.sh
	@echo "Policies built!"

# Run linter
lint:
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed. Install with:"; \
		echo "  curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin"; \
		exit 1; \
	fi

# Format code
fmt:
	@echo "Formatting code..."
	gofmt -s -w .
	@echo "Code formatted!"

# Verify all tests pass (for CI)
ci: lint test-all
	@echo "CI checks passed!"

# Quick test (unit tests only, no build)
quick:
	@echo "Running quick tests..."
	go test -short ./internal/...
	@echo "Quick tests completed!"

# Install development dependencies
deps:
	@echo "Installing dependencies..."
	go mod download
	go mod tidy
	@echo "Dependencies installed!"

# Initialize development environment
init: deps policies
	@echo "Initializing development environment..."
	@mkdir -p db
	@mkdir -p policies/wasm
	@if [ ! -f .env ]; then \
		echo "Creating .env file..."; \
		cp .env.example .env 2>/dev/null || echo "LOG_LEVEL=info" > .env; \
	fi
	@echo "Development environment ready!"
	@echo "Run 'make run' to start the sidecar"

# Watch for changes and run tests (requires entr)
watch:
	@echo "Watching for changes..."
	@if command -v entr >/dev/null 2>&1; then \
		find . -name '*.go' | entr -c make test-unit; \
	else \
		echo "entr not installed. Install with:"; \
		echo "  apt-get install entr  (Ubuntu/Debian)"; \
		echo "  brew install entr     (macOS)"; \
		exit 1; \
	fi

# Show test logs
logs:
	@echo "Docker Compose logs:"
	docker-compose logs -f

# Database shell
db:
	@echo "Opening database shell..."
	@if [ -f db/audit.db ]; then \
		sqlite3 db/audit.db; \
	else \
		echo "Database not found. Run the sidecar first."; \
	fi

# Health check
health:
	@echo "Checking service health..."
	@curl -s http://localhost:8080/health | jq . || echo "Service not running"

# Integration test specific
test-approval:
	@echo "Running approval flow tests..."
	go test -v -run TestApproval ./test/integration/

test-policy:
	@echo "Running policy reload tests..."
	go test -v -run TestPolicy ./test/integration/

test-concurrent:
	@echo "Running concurrent tests..."
	go test -v -run TestConcurrent ./test/integration/

# Stress test
stress:
	@echo "Running stress tests..."
	go test -v -run TestHighLoad -timeout 15m ./test/integration/

# Security scan
security:
	@echo "Running security scan..."
	@if command -v gosec >/dev/null 2>&1; then \
		gosec ./...; \
	else \
		echo "gosec not installed. Install with:"; \
		echo "  go install github.com/securego/gosec/v2/cmd/gosec@latest"; \
		exit 1; \
	fi

# Check Go module dependencies for vulnerabilities
vuln:
	@echo "Checking for vulnerabilities..."
	go list -json -m all | go run golang.org/x/vuln/cmd/govulncheck@latest -mode=convert -

.DEFAULT_GOAL := help
