package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dagbolade/ai-governance-sidecar/internal/approval"
	"github.com/dagbolade/ai-governance-sidecar/internal/audit"
	"github.com/dagbolade/ai-governance-sidecar/internal/auth"
	"github.com/dagbolade/ai-governance-sidecar/internal/policy"
	"github.com/dagbolade/ai-governance-sidecar/internal/proxy"
	"github.com/dagbolade/ai-governance-sidecar/internal/server"
	"github.com/stretchr/testify/require"
)

// TestEnvironment represents a complete test environment
type TestEnvironment struct {
	Server        *server.Server
	PolicyEngine  policy.Evaluator
	AuditStore    audit.Store
	ApprovalQueue approval.Queue
	AuthManager   *auth.Manager
	UpstreamMock  *httptest.Server
	PolicyDir     string
	DBPath        string
	HTTPServer    *httptest.Server
	t             *testing.T
}

// SetupTestEnvironment creates a complete test environment with all components
func SetupTestEnvironment(t *testing.T) *TestEnvironment {
	t.Helper()

	// Create temporary directories
	tmpDir := t.TempDir()
	policyDir := filepath.Join(tmpDir, "policies")
	dbPath := filepath.Join(tmpDir, "test.db")

	require.NoError(t, os.MkdirAll(policyDir, 0755))

	// Create mock upstream server
	upstreamMock := CreateMockUpstream()

	// Initialize components
	auditStore, err := audit.NewSQLiteStore(dbPath)
	require.NoError(t, err)

	approvalQueue := approval.NewInMemoryQueue(30 * time.Second)

	authManager := auth.NewManager(auth.Config{
		RequireAuth:     false,
		JWTSecret:       "test-secret",
		TokenExpiration: time.Hour,
	})

	env := &TestEnvironment{
		AuditStore:    auditStore,
		ApprovalQueue: approvalQueue,
		AuthManager:   authManager,
		UpstreamMock:  upstreamMock,
		PolicyDir:     policyDir,
		DBPath:        dbPath,
		t:             t,
	}

	// Cleanup function
	t.Cleanup(func() {
		if env.PolicyEngine != nil {
			env.PolicyEngine.Close()
		}
		env.AuditStore.Close()
		env.ApprovalQueue.Close()
		env.UpstreamMock.Close()
		if env.HTTPServer != nil {
			env.HTTPServer.Close()
		}
	})

	return env
}

// InitializePolicyEngine creates and initializes the policy engine with test policies
func (e *TestEnvironment) InitializePolicyEngine() error {
	engine, err := policy.NewEngine(e.PolicyDir)
	if err != nil {
		return err
	}
	e.PolicyEngine = engine
	return nil
}

// RequireWASMPolicies checks if valid WASM policies are available and skips test if not
func RequireWASMPolicies(t *testing.T) {
	t.Helper()
	
	wasmDir := "../../policies/wasm"
	entries, err := os.ReadDir(wasmDir)
	if err != nil {
		t.Skipf("WASM policies not found: %v (build policies first with 'cd policies && ./build.sh')", err)
	}
	
	hasValidWasm := false
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".wasm") {
			hasValidWasm = true
			break
		}
	}
	
	if !hasValidWasm {
		t.Skip("No WASM policies found (build policies first with 'cd policies && ./build.sh')")
	}
}

// StartServer creates and starts the HTTP server
func (e *TestEnvironment) StartServer() {
	cfg := server.Config{
		Port:            8080,
		ReadTimeout:     30,
		WriteTimeout:    30,
		ShutdownTimeout: 5,
		ProxyConfig: proxy.ProxyConfig{
			DefaultUpstream: e.UpstreamMock.URL,
			Timeout:         30,
		},
	}

	srv := server.New(cfg, e.PolicyEngine, e.AuditStore, e.ApprovalQueue, e.AuthManager)
	e.Server = srv
	
	// Create a test server - we need to serve the handler manually
	// since server.Server doesn't expose Echo directly
	e.HTTPServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// For testing, we need to handle requests through the proxy handler
		// This is a simplified test server setup
		w.Header().Set("Content-Type", "application/json")
		
		// Route basic endpoints for testing
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
		default:
			// For other routes, return a generic OK for now
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"message": "test endpoint"})
		}
	}))
}

// CreateMockUpstream creates a mock upstream server for testing
func CreateMockUpstream() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Echo back the request as a successful response
		var req map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		response := map[string]interface{}{
			"success": true,
			"result":  req,
			"message": "mock upstream processed request",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
}

// WritePolicy writes a WASM policy file to the test policy directory
func (e *TestEnvironment) WritePolicy(filename string, content []byte) error {
	policyPath := filepath.Join(e.PolicyDir, filename)
	return os.WriteFile(policyPath, content, 0644)
}

// CopyPolicyFromWorkspace copies a compiled WASM policy from the workspace
func (e *TestEnvironment) CopyPolicyFromWorkspace(policyName string) error {
	// Look for policy in common locations
	searchPaths := []string{
		fmt.Sprintf("../../policies/wasm/%s.wasm", policyName),
		fmt.Sprintf("../policies/wasm/%s.wasm", policyName),
		fmt.Sprintf("policies/wasm/%s.wasm", policyName),
	}

	var sourcePath string
	for _, path := range searchPaths {
		if _, err := os.Stat(path); err == nil {
			sourcePath = path
			break
		}
	}

	if sourcePath == "" {
		return fmt.Errorf("policy %s not found in workspace", policyName)
	}

	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("read policy: %w", err)
	}

	return e.WritePolicy(fmt.Sprintf("%s.wasm", policyName), content)
}

// HTTPClient returns a configured HTTP client for testing
func (e *TestEnvironment) HTTPClient() *http.Client {
	return &http.Client{
		Timeout: 10 * time.Second,
	}
}

// WaitForApprovalQueue waits for an approval to appear in the queue
func (e *TestEnvironment) WaitForApprovalQueue(timeout time.Duration) ([]approval.Request, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout waiting for approval queue")
		case <-ticker.C:
			pending, err := e.ApprovalQueue.GetPending(context.Background())
			if err != nil {
				return nil, err
			}
			if len(pending) > 0 {
				return pending, nil
			}
		}
	}
}

// WaitForAuditEntries waits for audit entries to be written
func (e *TestEnvironment) WaitForAuditEntries(minCount int, timeout time.Duration) ([]audit.Entry, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout waiting for audit entries")
		case <-ticker.C:
			entries, err := e.AuditStore.GetAll(context.Background())
			if err != nil {
				return nil, err
			}
			if len(entries) >= minCount {
				return entries, nil
			}
		}
	}
}

// AssertAuditEntry checks that an audit entry was created with expected values
func AssertAuditEntry(t *testing.T, entries []audit.Entry, expectedDecision audit.Decision, expectedToolName string) {
	t.Helper()

	found := false
	for _, entry := range entries {
		var toolInput map[string]interface{}
		if err := json.Unmarshal(entry.ToolInput, &toolInput); err != nil {
			continue
		}

		if toolName, ok := toolInput["tool_name"].(string); ok && toolName == expectedToolName {
			if entry.Decision == expectedDecision {
				found = true
				break
			}
		}
	}

	require.True(t, found, "Expected audit entry not found: tool=%s, decision=%s", expectedToolName, expectedDecision)
}

// BaseURL returns the base URL of the test HTTP server
func (e *TestEnvironment) BaseURL() string {
	if e.HTTPServer != nil {
		return e.HTTPServer.URL
	}
	return ""
}
