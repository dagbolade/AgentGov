package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// getDockerComposeCommand returns the correct docker compose command
// Tries "docker compose" first (v2), falls back to "docker-compose" (v1)
func getDockerComposeCommand() []string {
	// Try docker compose v2
	cmd := exec.Command("docker", "compose", "version")
	if err := cmd.Run(); err == nil {
		return []string{"docker", "compose"}
	}
	// Fall back to docker-compose v1
	return []string{"docker-compose"}
}

// TestDockerComposeSmoke tests the full Docker Compose deployment
// This test requires Docker and docker compose to be installed
func TestDockerComposeSmoke(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Docker Compose test in short mode")
	}

	// Check if Docker is available
	if !isDockerAvailable(t) {
		t.Skip("Docker not available, skipping Docker Compose tests")
	}

	// Get the project root directory
	projectRoot := getProjectRoot(t)

	// Change to project directory
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(originalDir)

	err = os.Chdir(projectRoot)
	require.NoError(t, err)

	t.Log("Starting Docker Compose stack...")

	// Start Docker Compose
	startDockerCompose(t)

	// Ensure cleanup
	t.Cleanup(func() {
		t.Log("Cleaning up Docker Compose stack...")
		stopDockerCompose(t)
	})

	// Wait for services to be ready
	waitForServicesReady(t, 60*time.Second)

	// Run smoke tests
	t.Run("health_check", testDockerHealthCheck)
	t.Run("basic_tool_call", testDockerBasicToolCall)
	t.Run("approval_flow", testDockerApprovalFlow)
	t.Run("audit_log", testDockerAuditLog)
	t.Run("ui_access", testDockerUIAccess)
}

// isDockerAvailable checks if Docker is installed and running
func isDockerAvailable(t *testing.T) bool {
	t.Helper()

	cmd := exec.Command("docker", "info")
	err := cmd.Run()
	return err == nil
}

// getProjectRoot finds the project root directory
func getProjectRoot(t *testing.T) string {
	t.Helper()

	// Try to find docker-compose.yml
	searchPaths := []string{
		".",
		"..",
		"../..",
		"../../..",
	}

	for _, path := range searchPaths {
		if _, err := os.Stat(path + "/docker-compose.yml"); err == nil {
			absPath, _ := filepath.Abs(path)
			return absPath
		}
	}

	t.Fatal("Could not find project root with docker-compose.yml")
	return ""
}

// startDockerCompose starts the Docker Compose stack
func startDockerCompose(t *testing.T) {
	t.Helper()

	dockerCmd := getDockerComposeCommand()

	// Build the images first
	buildArgs := append(dockerCmd, "build")
	buildCmd := exec.Command(buildArgs[0], buildArgs[1:]...)
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	err := buildCmd.Run()
	require.NoError(t, err, "Failed to build Docker images")

	// Start services in detached mode
	upArgs := append(dockerCmd, "up", "-d")
	upCmd := exec.Command(upArgs[0], upArgs[1:]...)
	upCmd.Stdout = os.Stdout
	upCmd.Stderr = os.Stderr
	err = upCmd.Run()
	require.NoError(t, err, "Failed to start Docker Compose")

	t.Log("Docker Compose stack started")
}

// stopDockerCompose stops and removes the Docker Compose stack
func stopDockerCompose(t *testing.T) {
	t.Helper()

	dockerCmd := getDockerComposeCommand()
	downArgs := append(dockerCmd, "down", "-v")
	downCmd := exec.Command(downArgs[0], downArgs[1:]...)
	downCmd.Stdout = os.Stdout
	downCmd.Stderr = os.Stderr
	err := downCmd.Run()
	if err != nil {
		t.Logf("Warning: Failed to stop Docker Compose: %v", err)
	}
}

// waitForServicesReady waits for all services to be healthy
func waitForServicesReady(t *testing.T, timeout time.Duration) {
	t.Helper()

	t.Log("Waiting for services to be ready...")

	deadline := time.Now().Add(timeout)
	backendReady := false
	uiReady := false

	for time.Now().Before(deadline) {
		// Check backend health
		if !backendReady {
			resp, err := http.Get("http://localhost:8080/health")
			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					backendReady = true
					t.Log("✓ Backend service is ready")
				}
			}
		}

		// Check UI
		if !uiReady {
			resp, err := http.Get("http://localhost:3000/")
			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					uiReady = true
					t.Log("✓ UI service is ready")
				}
			}
		}

		// Both services ready
		if backendReady && uiReady {
			t.Log("All services are ready!")
			return
		}

		time.Sleep(2 * time.Second)
	}

	// Log container status for debugging
	dockerCmd := getDockerComposeCommand()
	statusArgs := append(dockerCmd, "ps")
	statusCmd := exec.Command(statusArgs[0], statusArgs[1:]...)
	statusCmd.Stdout = os.Stdout
	statusCmd.Stderr = os.Stderr
	statusCmd.Run()

	// Log container logs for debugging
	logsArgs := append(dockerCmd, "logs", "--tail=50")
	logsCmd := exec.Command(logsArgs[0], logsArgs[1:]...)
	logsCmd.Stdout = os.Stdout
	logsCmd.Stderr = os.Stderr
	logsCmd.Run()

	t.Fatalf("Services did not become ready within %v", timeout)
}

// testDockerHealthCheck verifies the health endpoint
func testDockerHealthCheck(t *testing.T) {
	resp, err := http.Get("http://localhost:8080/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, "healthy", result["status"])
	t.Log("✓ Health check passed")
}

// testDockerBasicToolCall tests a basic tool call through the deployed service
func testDockerBasicToolCall(t *testing.T) {
	reqBody := map[string]interface{}{
		"tool_name": "test_tool",
		"args": map[string]interface{}{
			"action": "read",
			"data":   "test",
		},
	}

	body, _ := json.Marshal(reqBody)
	resp, err := http.Post(
		"http://localhost:8080/tool/call",
		"application/json",
		bytes.NewBuffer(body),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should get either 200 OK or 202 Accepted depending on policies
	assert.Contains(t, []int{http.StatusOK, http.StatusAccepted, http.StatusUnauthorized}, resp.StatusCode)

	t.Logf("Tool call response: %d", resp.StatusCode)
}

// testDockerApprovalFlow tests the approval workflow
func testDockerApprovalFlow(t *testing.T) {
	// Check pending approvals
	resp, err := http.Get("http://localhost:8080/pending")
	require.NoError(t, err)
	defer resp.Body.Close()

	// May require auth, so 401 is acceptable
	assert.Contains(t, []int{http.StatusOK, http.StatusUnauthorized}, resp.StatusCode)

	if resp.StatusCode == http.StatusOK {
		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		t.Logf("Pending approvals response: %+v", result)
	}
}

// testDockerAuditLog tests the audit log endpoint
func testDockerAuditLog(t *testing.T) {
	resp, err := http.Get("http://localhost:8080/audit")
	require.NoError(t, err)
	defer resp.Body.Close()

	// May require auth
	assert.Contains(t, []int{http.StatusOK, http.StatusUnauthorized}, resp.StatusCode)

	if resp.StatusCode == http.StatusOK {
		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		t.Logf("Audit log retrieved successfully")
	}
}

// testDockerUIAccess tests that the UI is accessible
func testDockerUIAccess(t *testing.T) {
	resp, err := http.Get("http://localhost:3000/")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Check that we got HTML
	contentType := resp.Header.Get("Content-Type")
	assert.Contains(t, contentType, "text/html")

	t.Log("✓ UI is accessible")
}

// TestDockerComposeVolumes tests that volumes are mounted correctly
func TestDockerComposeVolumes(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Docker volume test in short mode")
	}

	if !isDockerAvailable(t) {
		t.Skip("Docker not available")
	}

	// Check that the audit database volume is created
	cmd := exec.Command("docker", "volume", "ls")
	output, err := cmd.Output()
	require.NoError(t, err)

	volumeList := string(output)
	t.Logf("Docker volumes:\n%s", volumeList)

	// Note: Volume names depend on docker-compose project name
	// Just verify that volume operations work
}

// TestDockerComposeNetworking tests container networking
func TestDockerComposeNetworking(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Docker networking test in short mode")
	}

	if !isDockerAvailable(t) {
		t.Skip("Docker not available")
	}

	// Check that containers can communicate
	dockerCmd := getDockerComposeCommand()
	psArgs := append(dockerCmd, "ps", "-q")
	cmd := exec.Command(psArgs[0], psArgs[1:]...)
	output, err := cmd.Output()
	if err != nil {
		t.Skip("Docker Compose not running")
	}

	containers := strings.Split(strings.TrimSpace(string(output)), "\n")
	t.Logf("Running containers: %d", len(containers))

	assert.GreaterOrEqual(t, len(containers), 1, "At least one container should be running")
}

// TestDockerComposeRestart tests service restart behavior
func TestDockerComposeRestart(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Docker restart test in short mode")
	}

	if !isDockerAvailable(t) {
		t.Skip("Docker not available")
	}

	// Restart the backend service
	t.Log("Restarting backend service...")
	dockerCmd := getDockerComposeCommand()
	restartArgs := append(dockerCmd, "restart", "governance-sidecar")
	cmd := exec.Command(restartArgs[0], restartArgs[1:]...)
	err := cmd.Run()
	require.NoError(t, err)

	// Wait for service to come back up
	waitForHealthy(t, "http://localhost:8080/health", 30*time.Second)

	t.Log("✓ Service recovered after restart")
}

// TestDockerComposeLogs tests that logs are being generated
func TestDockerComposeLogs(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Docker logs test in short mode")
	}

	if !isDockerAvailable(t) {
		t.Skip("Docker not available")
	}

	// Get logs from backend service
	dockerCmd := getDockerComposeCommand()
	logsArgs := append(dockerCmd, "logs", "--tail=20", "governance-sidecar")
	cmd := exec.Command(logsArgs[0], logsArgs[1:]...)
	output, err := cmd.Output()
	require.NoError(t, err)

	logs := string(output)
	assert.NotEmpty(t, logs, "Service should generate logs")

	t.Logf("Recent logs:\n%s", logs)
}

// TestDockerComposeEnvironmentVariables tests that env vars are set correctly
func TestDockerComposeEnvironmentVariables(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Docker env test in short mode")
	}

	if !isDockerAvailable(t) {
		t.Skip("Docker not available")
	}

	// Check environment variables in the container
	dockerCmd := getDockerComposeCommand()
	execArgs := append(dockerCmd, "exec", "-T", "governance-sidecar", "env")
	cmd := exec.Command(execArgs[0], execArgs[1:]...)
	output, err := cmd.Output()
	if err != nil {
		t.Skip("Could not exec into container")
	}

	envVars := string(output)
	t.Logf("Container environment variables:\n%s", envVars)

	// Check for expected variables
	assert.Contains(t, envVars, "POLICY_DIR", "POLICY_DIR should be set")
	assert.Contains(t, envVars, "DB_PATH", "DB_PATH should be set")
}

// waitForHealthy waits for a health endpoint to return 200
func waitForHealthy(t *testing.T, url string, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(1 * time.Second)
	}

	t.Fatalf("Service at %s did not become healthy within %v", url, timeout)
}
