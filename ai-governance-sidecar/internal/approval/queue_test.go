package approval

import (
	"context"
	"encoding/json"
	"testing"
	"time"
	"sync"

	"github.com/dagbolade/ai-governance-sidecar/internal/policy"
)

func TestEnqueueAndDecide(t *testing.T) {
	queue := NewInMemoryQueue(5 * time.Second)
	defer queue.Close()

	ctx := context.Background()
	req := policy.Request{
		ToolName: "test_tool",
		Args:     json.RawMessage(`{"key":"value"}`),
	}

	doneCh := make(chan Decision)
	go func() {
		decision, err := queue.Enqueue(ctx, req, "requires approval")
		if err != nil {
			t.Errorf("enqueue failed: %v", err)
		}
		doneCh <- decision
	}()

	time.Sleep(100 * time.Millisecond)

	pending, err := queue.GetPending(ctx)
	if err != nil {
		t.Fatalf("get pending failed: %v", err)
	}

	if len(pending) != 1 {
		t.Fatalf("expected 1 pending request, got %d", len(pending))
	}

	decision := Decision{
		Approved:  true,
		Reason:    "approved by test",
		DecidedBy: "tester",
	}

	if err := queue.Decide(ctx, pending[0].ID, decision); err != nil {
		t.Fatalf("decide failed: %v", err)
	}

	select {
	case result := <-doneCh:
		if !result.Approved {
			t.Error("expected approved decision")
		}
		if result.Reason != "approved by test" {
			t.Errorf("unexpected reason: %s", result.Reason)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for decision")
	}
}

func TestTimeout(t *testing.T) {
	queue := NewInMemoryQueue(100 * time.Millisecond)
	defer queue.Close()

	ctx := context.Background()
	req := policy.Request{
		ToolName: "test_tool",
		Args:     json.RawMessage(`{}`),
	}

	decision, err := queue.Enqueue(ctx, req, "will timeout")
	if err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}

	if decision.Approved {
		t.Error("expected timeout to result in denial")
	}

	if decision.Reason != "approval timeout" {
		t.Errorf("unexpected reason: %s", decision.Reason)
	}
}

func TestDecideNonExistent(t *testing.T) {
	queue := NewInMemoryQueue(5 * time.Second)
	defer queue.Close()

	ctx := context.Background()
	decision := Decision{Approved: true, Reason: "test"}

	err := queue.Decide(ctx, "nonexistent-id", decision)
	if err == nil {
		t.Error("expected error for non-existent request")
	}
}

func TestConcurrentEnqueue(t *testing.T) {
	queue := NewInMemoryQueue(5 * time.Second)
	ctx := context.Background()
	const numRequests = 10

	var wg sync.WaitGroup
	wg.Add(numRequests)

	for i := 0; i < numRequests; i++ {
		go func(id int) {
			defer wg.Done()
			req := policy.Request{
				ToolName: "concurrent_test",
				Args:     json.RawMessage(`{}`),
			}
			queue.Enqueue(ctx, req, "concurrent")
		}(i)
	}

	wg.Wait() // Wait for all goroutines to finish before checking and closing

	pending, _ := queue.GetPending(ctx)
	if len(pending) != numRequests {
		t.Errorf("expected %d pending requests, got %d", numRequests, len(pending))
	}

	queue.Close()
}
