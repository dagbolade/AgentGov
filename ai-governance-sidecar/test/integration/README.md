# Integration Tests

This directory contains comprehensive end-to-end integration tests for the AI Governance Sidecar.

## Test Structure

```
test/integration/
├── testutil.go             # Common test utilities and helpers
├── approval_flow_test.go   # Full approval workflow tests
├── policy_reload_test.go   # Policy hot-reload tests
├── concurrent_test.go      # Concurrent request handling tests
├── docker_smoke_test.go    # Docker Compose smoke tests
└── README.md               # This file
```

## Test Categories

### 1. Approval Flow Tests (`approval_flow_test.go`)

Tests the complete approval workflow from request to decision:

- **TestApprovalFlowE2E**: End-to-end approval workflow
  - Policy evaluation triggers approval
  - Request queued for human review
  - Decision made via API
  - Audit trail verification
  - Upstream forwarding

- **TestApprovalTimeout**: Timeout handling for pending approvals

- **TestApprovalQueueConcurrency**: Multiple concurrent approval requests

- **TestAuditLogIntegrity**: Immutability and ordering of audit logs

**Run:** `go test -v -run TestApproval ./test/integration/...`

### 2. Policy Hot-Reload Tests (`policy_reload_test.go`)

Tests dynamic policy loading without service restart:

- **TestPolicyHotReload**: Complete hot-reload workflow
  - Initial policy load
  - Add policy at runtime
  - Modify existing policy
  - Remove policy at runtime

- **TestPolicyReloadConcurrency**: Policy reload during active evaluations

- **TestPolicyWatcherReload**: File watcher automatic reload

- **TestPolicyReloadErrors**: Error handling for invalid policies

- **TestMultiplePolicyInteraction**: Multiple policies evaluating same request

**Run:** `go test -v -run TestPolicy ./test/integration/...`

### 3. Concurrent Request Tests (`concurrent_test.go`)

Tests system behavior under concurrent load:

- **TestConcurrentRequests**: 50+ simultaneous HTTP requests

- **TestConcurrentApprovals**: Concurrent approval queue operations

- **TestConcurrentPolicyEvaluations**: 100+ concurrent policy evaluations

- **TestConcurrentAuditWrites**: 100+ concurrent audit log writes

- **TestRaceConditionApprovalDecision**: Race condition prevention

- **TestHighLoadStability**: Sustained load (10s @ 50 req/s)

- **TestDeadlockPrevention**: Deadlock detection and prevention

**Run:** `go test -v -run TestConcurrent ./test/integration/...`

### 4. Docker Compose Smoke Tests (`docker_smoke_test.go`)

Tests the complete Docker deployment:

- **TestDockerComposeSmoke**: Full stack deployment
  - Service startup
  - Health checks
  - Basic tool calls
  - Approval workflow
  - UI access

- **TestDockerComposeVolumes**: Volume mounting verification

- **TestDockerComposeNetworking**: Container communication

- **TestDockerComposeRestart**: Service restart behavior

- **TestDockerComposeLogs**: Log generation

**Run:** `go test -v -run TestDocker ./test/integration/...`

## Running Tests

### Prerequisites

1. **Go 1.21+** installed
2. **Docker & Docker Compose** (for Docker tests)
3. **Compiled WASM policies** in `policies/wasm/`

Build policies first:
```bash
cd policies
./build.sh
cd ..
```

### Run All Integration Tests

```bash
make test-integration
```

### Run Specific Test Suite

```bash
# Approval flow tests only
go test -v -run TestApproval ./test/integration/

# Policy reload tests only
go test -v -run TestPolicy ./test/integration/

# Concurrent tests only
go test -v -run TestConcurrent ./test/integration/

# Docker smoke tests only
go test -v -run TestDocker ./test/integration/
```

### Run Single Test

```bash
go test -v -run TestApprovalFlowE2E ./test/integration/
```

### Skip Long-Running Tests

```bash
go test -v -short ./test/integration/
```

This skips:
- Docker Compose tests
- High load stability tests
- Extended stress tests

### Run with Coverage

```bash
go test -v -coverprofile=coverage.out ./test/integration/
go tool cover -html=coverage.out
```

## Test Configuration

### Environment Variables

- `TEST_POLICY_DIR`: Custom policy directory for tests
- `TEST_UPSTREAM_URL`: Custom upstream URL for tests
- `TEST_TIMEOUT`: Default test timeout (default: 30s)

### Test Flags

- `-short`: Skip long-running tests
- `-v`: Verbose output
- `-race`: Enable race detector
- `-count=N`: Run each test N times

## Writing New Tests

### Use Test Environment Helper

```go
func TestMyFeature(t *testing.T) {
    env := SetupTestEnvironment(t)
    
    // Copy real WASM policy
    err := env.CopyPolicyFromWorkspace("passthrough")
    if err != nil {
        t.Skip("WASM policies not available")
    }
    
    // Initialize components
    require.NoError(t, env.InitializePolicyEngine())
    env.StartServer()
    
    // Your test code here
    resp, err := http.Get(env.BaseURL() + "/health")
    require.NoError(t, err)
    defer resp.Body.Close()
    
    assert.Equal(t, http.StatusOK, resp.StatusCode)
}
```

### Test Helper Functions

- `SetupTestEnvironment(t)`: Creates isolated test environment
- `env.CopyPolicyFromWorkspace(name)`: Copies compiled WASM policy
- `env.InitializePolicyEngine()`: Initializes policy engine
- `env.StartServer()`: Starts test HTTP server
- `env.WaitForApprovalQueue(timeout)`: Waits for approval
- `env.WaitForAuditEntries(count, timeout)`: Waits for audit entries

### Mock Upstream Server

The test environment includes a mock upstream server that echoes requests:

```go
env := SetupTestEnvironment(t)
// env.UpstreamMock is automatically created
// URL available at: env.UpstreamMock.URL
```

## CI/CD Integration

### GitHub Actions Example

```yaml
name: Integration Tests

on: [push, pull_request]

jobs:
  integration:
    runs-on: ubuntu-latest
    
    steps:
    - uses: actions/checkout@v3
    
    - name: Set up Go
      uses: actions/setup-go@v4
      with:
        go-version: '1.21'
    
    - name: Build WASM Policies
      run: |
        cd policies
        ./build.sh
    
    - name: Run Integration Tests
      run: make test-integration
    
    - name: Upload Coverage
      uses: codecov/codecov-action@v3
      with:
        files: ./coverage.out
```

### Docker Tests in CI

```yaml
    - name: Run Docker Smoke Tests
      run: |
        docker-compose up -d
        go test -v -run TestDockerCompose ./test/integration/
        docker-compose down -v
```

## Debugging Tests

### Enable Verbose Logging

```bash
LOG_LEVEL=debug go test -v ./test/integration/
```

### View Docker Logs

```bash
docker-compose logs -f governance-sidecar
```

### Inspect Test Database

```bash
# Find test DB in temporary directory
find /tmp -name "test.db" -type f -mmin -5

# Inspect with sqlite3
sqlite3 /tmp/.../test.db "SELECT * FROM audit_log;"
```

### Race Condition Detection

```bash
go test -race ./test/integration/
```

## Performance Benchmarks

Run benchmarks to measure performance:

```bash
go test -bench=. -benchmem ./test/integration/
```

## Troubleshooting

### Tests Fail: "WASM policies not available"

**Solution:** Build WASM policies first:
```bash
cd policies && ./build.sh
```

### Tests Fail: "Docker not available"

**Solution:** Ensure Docker is running:
```bash
docker info
```

### Tests Timeout

**Solution:** Increase timeout or skip with `-short`:
```bash
go test -v -short -timeout 5m ./test/integration/
```

### Port Already in Use

**Solution:** Tests use random ports. If issues persist:
```bash
# Kill processes using port 8080
lsof -ti:8080 | xargs kill -9
```

### Database Lock Errors

**Solution:** Tests use isolated temp databases. If issues persist:
```bash
# Clean up old test databases
find /tmp -name "test.db*" -type f -mtime +1 -delete
```

## Test Maintenance

### Update Test Policies

When policy format changes:

1. Rebuild WASM policies: `cd policies && ./build.sh`
2. Update test expectations in test files
3. Run full test suite: `make test-integration`

### Add New Test Suite

1. Create `my_feature_test.go` in `test/integration/`
2. Use `SetupTestEnvironment(t)` helper
3. Add documentation to this README
4. Update Makefile if needed

## Coverage Goals

- **Overall Coverage**: > 80%
- **Critical Paths**: > 95%
  - Approval workflow
  - Policy evaluation
  - Audit logging
- **Error Handling**: 100%

Check coverage:
```bash
make coverage-integration
```

## Related Documentation

- [Internal Architecture](../../internal/README.md)
- [Policy Development](../../policies/README.md)
- [Deployment Guide](../../COMPLETE_SYSTEM.md)
- [Unit Tests](../../internal/*/README.md)
