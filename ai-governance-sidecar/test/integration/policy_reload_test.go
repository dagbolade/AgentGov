package integration

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagbolade/ai-governance-sidecar/internal/policy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPolicyHotReload tests that the policy engine can reload policies
// without restarting the server
func TestPolicyHotReload(t *testing.T) {
	env := SetupTestEnvironment(t)

	// Try to copy real WASM policies from workspace
	err := env.CopyPolicyFromWorkspace("passthrough")
	if err != nil {
		t.Logf("Could not load passthrough policy: %v", err)
		t.Skip("Real WASM policies not available, skipping hot-reload test")
	}

	require.NoError(t, env.InitializePolicyEngine())

	t.Run("initial_policy_load", func(t *testing.T) {
		// Make a test request to verify initial policy works
		req := policy.Request{
			ToolName: "test_tool",
			Args:     json.RawMessage(`{"action":"read"}`),
		}

		resp, err := env.PolicyEngine.Evaluate(context.Background(), req)
		require.NoError(t, err)

		// Passthrough policy should allow everything
		assert.True(t, resp.Allow, "Passthrough policy should allow requests")
	})

	t.Run("add_new_policy_runtime", func(t *testing.T) {
		// Try to add another policy at runtime
		err := env.CopyPolicyFromWorkspace("rate_limit")
		if err != nil {
			t.Logf("Could not load rate_limit policy: %v", err)
			return
		}

		// Give the file watcher time to detect the change
		time.Sleep(2 * time.Second)

		// Trigger manual reload to be sure
		err = env.PolicyEngine.Reload()
		require.NoError(t, err)

		// Verify policy is loaded by making a request
		req := policy.Request{
			ToolName: "bulk_operation",
			Args:     json.RawMessage(`{"count":10000}`),
		}

		resp, err := env.PolicyEngine.Evaluate(context.Background(), req)
		require.NoError(t, err)

		// Rate limit policy might require approval or deny large operations
		t.Logf("Policy response: allow=%v, humanRequired=%v, reason=%s",
			resp.Allow, resp.HumanRequired, resp.Reason)
	})

	t.Run("modify_policy_runtime", func(t *testing.T) {
		// Simulate policy modification by copying a different policy
		originalPath := filepath.Join(env.PolicyDir, "passthrough.wasm")
		
		// Check if policy exists
		if _, err := os.Stat(originalPath); os.IsNotExist(err) {
			t.Skip("Policy file not available for modification test")
		}

		// Save original modification time
		originalInfo, err := os.Stat(originalPath)
		require.NoError(t, err)
		originalModTime := originalInfo.ModTime()

		// Touch the file to trigger a reload
		time.Sleep(1 * time.Second)
		now := time.Now()
		err = os.Chtimes(originalPath, now, now)
		require.NoError(t, err)

		// Wait for file watcher to detect change
		time.Sleep(2 * time.Second)

		// Verify the file was modified
		newInfo, err := os.Stat(originalPath)
		require.NoError(t, err)
		assert.True(t, newInfo.ModTime().After(originalModTime),
			"File modification time should have changed")

		// Policy should still work after reload
		req := policy.Request{
			ToolName: "test_tool",
			Args:     json.RawMessage(`{"action":"test"}`),
		}

		resp, err := env.PolicyEngine.Evaluate(context.Background(), req)
		require.NoError(t, err)
		t.Logf("Policy still functional after reload: allow=%v", resp.Allow)
	})

	t.Run("remove_policy_runtime", func(t *testing.T) {
		// List current policies
		policyFiles, err := filepath.Glob(filepath.Join(env.PolicyDir, "*.wasm"))
		require.NoError(t, err)

		if len(policyFiles) == 0 {
			t.Skip("No policies to remove")
		}

		// Remove one policy
		policyToRemove := policyFiles[0]
		err = os.Remove(policyToRemove)
		require.NoError(t, err)

		t.Logf("Removed policy: %s", filepath.Base(policyToRemove))

		// Wait for file watcher
		time.Sleep(2 * time.Second)

		// Reload policies
		err = env.PolicyEngine.Reload()
		require.NoError(t, err)

		// Engine should still work (might deny if no policies loaded)
		req := policy.Request{
			ToolName: "test_tool",
			Args:     json.RawMessage(`{"action":"test"}`),
		}

		resp, err := env.PolicyEngine.Evaluate(context.Background(), req)
		require.NoError(t, err)
		t.Logf("Policy engine still functional after removal: allow=%v, reason=%s",
			resp.Allow, resp.Reason)
	})
}

// TestPolicyReloadConcurrency tests that policy reload doesn't interfere
// with ongoing policy evaluations
func TestPolicyReloadConcurrency(t *testing.T) {
	env := SetupTestEnvironment(t)

	// Load initial policy
	err := env.CopyPolicyFromWorkspace("passthrough")
	if err != nil {
		t.Skip("WASM policies not available, skipping")
	}

	require.NoError(t, env.InitializePolicyEngine())

	// Start concurrent evaluations
	done := make(chan bool)
	errors := make(chan error, 100)

	// Run evaluations in background
	go func() {
		for i := 0; i < 50; i++ {
			req := policy.Request{
				ToolName: "concurrent_test",
				Args:     json.RawMessage(`{"iteration":` + string(rune(i)) + `}`),
			}

			_, err := env.PolicyEngine.Evaluate(context.Background(), req)
			if err != nil {
				errors <- err
			}

			time.Sleep(50 * time.Millisecond)
		}
		done <- true
	}()

	// Trigger reloads while evaluations are running
	go func() {
		for i := 0; i < 5; i++ {
			time.Sleep(200 * time.Millisecond)
			if err := env.PolicyEngine.Reload(); err != nil {
				errors <- err
			}
		}
	}()

	// Wait for completion
	select {
	case <-done:
		// Success
	case <-time.After(10 * time.Second):
		t.Fatal("Test timeout")
	}

	// Check for errors
	close(errors)
	var evalErrors []error
	for err := range errors {
		evalErrors = append(evalErrors, err)
	}

	if len(evalErrors) > 0 {
		t.Logf("Encountered %d errors during concurrent reload:", len(evalErrors))
		for _, err := range evalErrors[:min(5, len(evalErrors))] {
			t.Logf("  - %v", err)
		}
	}

	// Some errors might be acceptable during reload, but most should succeed
	assert.Less(t, len(evalErrors), 10,
		"Too many errors during concurrent reload (%d errors)", len(evalErrors))
}

// TestPolicyWatcherReload tests the file watcher mechanism
func TestPolicyWatcherReload(t *testing.T) {
	env := SetupTestEnvironment(t)

	// Start with empty policy directory
	require.NoError(t, env.InitializePolicyEngine())

	// Add a policy file and watch for automatic reload
	err := env.CopyPolicyFromWorkspace("passthrough")
	if err != nil {
		t.Skip("WASM policies not available, skipping")
	}

	// Wait for file watcher to detect the new file
	time.Sleep(3 * time.Second)

	// The policy should now be loaded
	req := policy.Request{
		ToolName: "test_tool",
		Args:     json.RawMessage(`{"test":"data"}`),
	}

	resp, err := env.PolicyEngine.Evaluate(context.Background(), req)
	require.NoError(t, err)

	t.Logf("Policy evaluation after hot-reload: allow=%v, reason=%s",
		resp.Allow, resp.Reason)
}

// TestPolicyReloadErrors tests error handling during policy reload
func TestPolicyReloadErrors(t *testing.T) {
	env := SetupTestEnvironment(t)

	// Load valid policy first
	err := env.CopyPolicyFromWorkspace("passthrough")
	if err != nil {
		t.Skip("WASM policies not available, skipping")
	}

	require.NoError(t, env.InitializePolicyEngine())

	t.Run("invalid_wasm_file", func(t *testing.T) {
		// Write invalid WASM content
		invalidWASM := []byte("not a valid WASM file")
		err := env.WritePolicy("invalid.wasm", invalidWASM)
		require.NoError(t, err)

		// Try to reload - should handle error gracefully
		err = env.PolicyEngine.Reload()
		// Error is expected, but system should remain stable
		if err != nil {
			t.Logf("Expected error loading invalid WASM: %v", err)
		}

		// Original policy should still work
		req := policy.Request{
			ToolName: "test_tool",
			Args:     json.RawMessage(`{"test":"data"}`),
		}

		_, err = env.PolicyEngine.Evaluate(context.Background(), req)
		require.NoError(t, err, "Original policy should still work after failed reload")
	})

	t.Run("corrupted_policy_file", func(t *testing.T) {
		// Create a file with partial WASM content (corrupted)
		corruptedWASM := []byte{0x00, 0x61, 0x73, 0x6d} // WASM magic number only
		err := env.WritePolicy("corrupted.wasm", corruptedWASM)
		require.NoError(t, err)

		time.Sleep(1 * time.Second)

		// Reload should handle corruption
		err = env.PolicyEngine.Reload()
		if err != nil {
			t.Logf("Expected error with corrupted WASM: %v", err)
		}
	})
}

// TestMultiplePolicyInteraction tests how multiple policies interact
func TestMultiplePolicyInteraction(t *testing.T) {
	env := SetupTestEnvironment(t)

	// Load multiple policies
	policies := []string{"passthrough", "rate_limit", "sensitive_data"}
	loadedCount := 0

	for _, policyName := range policies {
		err := env.CopyPolicyFromWorkspace(policyName)
		if err == nil {
			loadedCount++
		}
	}

	if loadedCount == 0 {
		t.Skip("No policies could be loaded, skipping")
	}

	require.NoError(t, env.InitializePolicyEngine())

	t.Run("all_policies_allow", func(t *testing.T) {
		// Request that should pass all policies
		req := policy.Request{
			ToolName: "simple_read",
			Args:     json.RawMessage(`{"action":"read","count":1}`),
		}

		resp, err := env.PolicyEngine.Evaluate(context.Background(), req)
		require.NoError(t, err)
		t.Logf("All policies result: allow=%v, humanRequired=%v, reason=%s",
			resp.Allow, resp.HumanRequired, resp.Reason)
	})

	t.Run("one_policy_denies", func(t *testing.T) {
		// Request with sensitive data (should be caught by sensitive_data policy)
		req := policy.Request{
			ToolName: "database_query",
			Args:     json.RawMessage(`{"query":"SELECT password FROM users"}`),
		}

		resp, err := env.PolicyEngine.Evaluate(context.Background(), req)
		require.NoError(t, err)
		t.Logf("Sensitive data result: allow=%v, humanRequired=%v, reason=%s",
			resp.Allow, resp.HumanRequired, resp.Reason)

		// If sensitive_data policy is loaded, it should require approval or deny
		if loadedCount > 1 {
			assert.True(t, !resp.Allow || resp.HumanRequired,
				"Sensitive data should be blocked or require approval")
		}
	})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
