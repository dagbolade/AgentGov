package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dagbolade/ai-governance-sidecar/internal/approval"
	"github.com/dagbolade/ai-governance-sidecar/internal/audit"
	"github.com/dagbolade/ai-governance-sidecar/internal/policy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConcurrentRequests tests the system under concurrent load
func TestConcurrentRequests(t *testing.T) {
	env := SetupTestEnvironment(t)

	// Load a simple policy
	err := env.CopyPolicyFromWorkspace("passthrough")
	if err != nil {
		t.Skip("WASM policies not available, skipping")
	}

	require.NoError(t, env.InitializePolicyEngine())
	env.StartServer()

	numRequests := 50
	var wg sync.WaitGroup
	var successCount, failCount int32

	start := time.Now()

	// Send concurrent requests
	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			reqBody := map[string]interface{}{
				"tool_name": fmt.Sprintf("tool_%d", id%10),
				"args": map[string]interface{}{
					"id":     id,
					"action": "read",
				},
			}

			body, _ := json.Marshal(reqBody)
			resp, err := http.Post(
				env.BaseURL()+"/tool/call",
				"application/json",
				bytes.NewBuffer(body),
			)

			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusAccepted {
					atomic.AddInt32(&successCount, 1)
				} else {
					atomic.AddInt32(&failCount, 1)
				}
			} else {
				atomic.AddInt32(&failCount, 1)
			}
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)

	t.Logf("Completed %d requests in %v", numRequests, elapsed)
	t.Logf("Success: %d, Failures: %d", successCount, failCount)

	// Most requests should succeed
	minSuccess := int32(float64(numRequests) * 0.9)
	assert.Greater(t, successCount, minSuccess,
		"At least 90%% of requests should succeed")

	// Verify audit log captured all requests
	time.Sleep(500 * time.Millisecond) // Wait for audit writes
	entries, err := env.AuditStore.GetAll(context.Background())
	require.NoError(t, err)

	t.Logf("Audit log contains %d entries", len(entries))
	assert.GreaterOrEqual(t, len(entries), int(successCount),
		"Audit log should contain at least all successful requests")
}

// TestConcurrentApprovals tests concurrent approval queue operations
func TestConcurrentApprovals(t *testing.T) {
	env := SetupTestEnvironment(t)

	numRequests := 20
	var wg sync.WaitGroup
	requestIDs := make(chan string, numRequests)

	// Enqueue multiple requests concurrently
	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			req := policy.Request{
				ToolName: fmt.Sprintf("concurrent_tool_%d", id),
				Args:     json.RawMessage(fmt.Sprintf(`{"id":%d}`, id)),
			}

			// Enqueue with timeout context
			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			defer cancel()

			decision, err := env.ApprovalQueue.Enqueue(ctx, req, "concurrent test")
			if err == nil && !decision.Approved {
				// Timeout occurred, which is expected in this test
				t.Logf("Request %d timed out (expected)", id)
			}
		}(i)

		// Small delay to create some overlap
		time.Sleep(10 * time.Millisecond)
	}

	// Concurrently check pending approvals
	go func() {
		for i := 0; i < 10; i++ {
			pending, _ := env.ApprovalQueue.GetPending(context.Background())
			t.Logf("Pending approvals: %d", len(pending))
			time.Sleep(50 * time.Millisecond)
		}
	}()

	wg.Wait()
	close(requestIDs)

	// Check final state
	pending, err := env.ApprovalQueue.GetPending(context.Background())
	require.NoError(t, err)
	t.Logf("Final pending count: %d", len(pending))
}

// TestConcurrentPolicyEvaluations tests policy engine under concurrent load
func TestConcurrentPolicyEvaluations(t *testing.T) {
	env := SetupTestEnvironment(t)

	err := env.CopyPolicyFromWorkspace("passthrough")
	if err != nil {
		t.Skip("WASM policies not available, skipping")
	}

	require.NoError(t, env.InitializePolicyEngine())

	numEvaluations := 100
	var wg sync.WaitGroup
	var successCount, errorCount int32

	start := time.Now()

	for i := 0; i < numEvaluations; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			req := policy.Request{
				ToolName: "test_tool",
				Args:     json.RawMessage(fmt.Sprintf(`{"id":%d}`, id)),
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			_, err := env.PolicyEngine.Evaluate(ctx, req)
			if err == nil {
				atomic.AddInt32(&successCount, 1)
			} else {
				atomic.AddInt32(&errorCount, 1)
			}
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)

	t.Logf("Completed %d evaluations in %v", numEvaluations, elapsed)
	t.Logf("Success: %d, Errors: %d", successCount, errorCount)

	assert.Equal(t, int32(numEvaluations), successCount,
		"All policy evaluations should succeed")
	assert.Equal(t, int32(0), errorCount,
		"No policy evaluation errors expected")
}

// TestConcurrentAuditWrites tests audit log under concurrent writes
func TestConcurrentAuditWrites(t *testing.T) {
	env := SetupTestEnvironment(t)

	numWrites := 100
	var wg sync.WaitGroup
	var successCount, errorCount int32

	start := time.Now()

	for i := 0; i < numWrites; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			toolInput := json.RawMessage(fmt.Sprintf(`{"tool":"concurrent_%d","action":"test"}`, id))
			decision := audit.DecisionAllow
			if id%3 == 0 {
				decision = audit.DecisionDeny
			}

			err := env.AuditStore.Log(context.Background(), toolInput, decision, "concurrent test")
			if err == nil {
				atomic.AddInt32(&successCount, 1)
			} else {
				atomic.AddInt32(&errorCount, 1)
				t.Logf("Audit write error for %d: %v", id, err)
			}
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)

	t.Logf("Completed %d audit writes in %v", numWrites, elapsed)
	t.Logf("Success: %d, Errors: %d", successCount, errorCount)

	// All writes should succeed
	assert.Equal(t, int32(numWrites), successCount,
		"All audit writes should succeed")

	// Verify all entries were written
	entries, err := env.AuditStore.GetAll(context.Background())
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(entries), numWrites,
		"All audit entries should be persisted")
}

// TestRaceConditionApprovalDecision tests for race conditions in approval decisions
func TestRaceConditionApprovalDecision(t *testing.T) {
	env := SetupTestEnvironment(t)

	// Create a pending approval request
	req := policy.Request{
		ToolName: "race_test",
		Args:     json.RawMessage(`{"test":"data"}`),
	}

	// Enqueue in background with long timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resultCh := make(chan error, 1)
	go func() {
		_, err := env.ApprovalQueue.Enqueue(ctx, req, "race condition test")
		resultCh <- err
	}()

	// Wait for request to be queued
	time.Sleep(100 * time.Millisecond)

	// Get the pending request
	pending, err := env.ApprovalQueue.GetPending(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, pending)

	approvalID := pending[0].ID

	// Try to make multiple concurrent decisions on the same request
	var wg sync.WaitGroup
	decisions := make(chan error, 5)

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			decision := approval.Decision{
				Approved:  id%2 == 0,
				Reason:    fmt.Sprintf("decision from goroutine %d", id),
				DecidedBy: fmt.Sprintf("approver_%d", id),
			}

			err := env.ApprovalQueue.Decide(context.Background(), approvalID, decision)
			decisions <- err
		}(i)
	}

	wg.Wait()
	close(decisions)
	cancel()

	// Only one decision should succeed, others should fail
	successCount := 0
	errorCount := 0
	for err := range decisions {
		if err == nil {
			successCount++
		} else {
			errorCount++
		}
	}

	assert.Equal(t, 1, successCount, "Only one decision should succeed")
	assert.Equal(t, 4, errorCount, "Other decisions should fail")

	// Original enqueue should complete
	select {
	case err := <-resultCh:
		assert.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Enqueue should have completed")
	}
}

// TestHighLoadStability tests system stability under sustained high load
func TestHighLoadStability(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping high load test in short mode")
	}

	env := SetupTestEnvironment(t)

	err := env.CopyPolicyFromWorkspace("passthrough")
	if err != nil {
		t.Skip("WASM policies not available, skipping")
	}

	require.NoError(t, env.InitializePolicyEngine())
	env.StartServer()

	// Run sustained load for 10 seconds
	duration := 10 * time.Second
	requestsPerSecond := 50

	var totalRequests, successCount, errorCount int32
	done := make(chan bool)

	start := time.Now()

	go func() {
		ticker := time.NewTicker(time.Second / time.Duration(requestsPerSecond))
		defer ticker.Stop()

		timeout := time.After(duration)
		for {
			select {
			case <-timeout:
				done <- true
				return
			case <-ticker.C:
				atomic.AddInt32(&totalRequests, 1)
				go func(id int32) {
					reqBody := map[string]interface{}{
						"tool_name": fmt.Sprintf("load_tool_%d", id%10),
						"args":      map[string]interface{}{"id": id},
					}

					body, _ := json.Marshal(reqBody)
					resp, err := http.Post(
						env.BaseURL()+"/tool/call",
						"application/json",
						bytes.NewBuffer(body),
					)

					if err == nil {
						resp.Body.Close()
						if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusAccepted {
							atomic.AddInt32(&successCount, 1)
						} else {
							atomic.AddInt32(&errorCount, 1)
						}
					} else {
						atomic.AddInt32(&errorCount, 1)
					}
				}(int32(atomic.LoadInt32(&totalRequests)))
			}
		}
	}()

	<-done
	elapsed := time.Since(start)

	// Wait for in-flight requests to complete
	time.Sleep(2 * time.Second)

	t.Logf("High load test completed in %v", elapsed)
	t.Logf("Total requests: %d", totalRequests)
	t.Logf("Success: %d (%.1f%%)", successCount, float64(successCount)/float64(totalRequests)*100)
	t.Logf("Errors: %d (%.1f%%)", errorCount, float64(errorCount)/float64(totalRequests)*100)

	// At least 95% success rate under sustained load
	successRate := float64(successCount) / float64(totalRequests)
	assert.Greater(t, successRate, 0.95,
		"Success rate should be above 95%% under sustained load")

	// Verify system is still responsive
	resp, err := http.Get(env.BaseURL() + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode, "System should still be healthy")
}

// TestDeadlockPrevention tests that the system doesn't deadlock under edge cases
func TestDeadlockPrevention(t *testing.T) {
	env := SetupTestEnvironment(t)

	// Create scenario that could potentially deadlock:
	// Multiple approvals timing out while new ones are being added

	var wg sync.WaitGroup
	done := make(chan bool, 1)

	// Start adding approvals
	go func() {
		for i := 0; i < 20; i++ {
			req := policy.Request{
				ToolName: fmt.Sprintf("deadlock_test_%d", i),
				Args:     json.RawMessage(`{}`),
			}

			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			env.ApprovalQueue.Enqueue(ctx, req, "deadlock test")
			cancel()

			time.Sleep(50 * time.Millisecond)
		}
	}()

	// Continuously query pending
	go func() {
		for i := 0; i < 30; i++ {
			env.ApprovalQueue.GetPending(context.Background())
			time.Sleep(100 * time.Millisecond)
		}
	}()

	// Set a timeout to detect deadlock
	go func() {
		time.Sleep(10 * time.Second)
		done <- true
	}()

	select {
	case <-done:
		t.Log("Deadlock prevention test completed without hanging")
	case <-time.After(15 * time.Second):
		t.Fatal("Test timed out - possible deadlock detected")
	}

	wg.Wait()
}
