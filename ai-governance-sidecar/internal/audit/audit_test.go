package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSQLiteStore(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	ctx := context.Background()
	toolInput := json.RawMessage(`{"tool":"test","args":{"key":"value"}}`)

	// Log first entry
	if err := store.Log(ctx, toolInput, DecisionAllow, "test allowed"); err != nil {
		t.Fatalf("failed to log allow: %v", err)
	}

	// Wait to ensure different timestamp
	time.Sleep(1 * time.Second)

	// Log second entry
	if err := store.Log(ctx, toolInput, DecisionDeny, "test denied"); err != nil {
		t.Fatalf("failed to log deny: %v", err)
	}

	entries, err := store.GetAll(ctx)
	if err != nil {
		t.Fatalf("failed to get all: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	// Verify both decisions exist
	foundAllow := false
	foundDeny := false
	for _, entry := range entries {
		if entry.Decision == DecisionAllow && entry.Reason == "test allowed" {
			foundAllow = true
		}
		if entry.Decision == DecisionDeny && entry.Reason == "test denied" {
			foundDeny = true
		}
	}

	if !foundAllow {
		t.Error("DecisionAllow entry not found")
	}
	if !foundDeny {
		t.Error("DecisionDeny entry not found")
	}

	// Verify DESC ordering - most recent (deny) should be first
	if entries[0].Decision != DecisionDeny {
		t.Logf("First entry timestamp: %v, decision: %s", entries[0].Timestamp, entries[0].Decision)
		t.Logf("Second entry timestamp: %v, decision: %s", entries[1].Timestamp, entries[1].Decision)
		t.Errorf("expected most recent entry (deny) first, got %s", entries[0].Decision)
	}

	// Verify timestamps are different
	if entries[0].Timestamp.Equal(entries[1].Timestamp) {
		t.Error("expected different timestamps for entries")
	}
}

func TestImmutability(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	ctx := context.Background()
	toolInput := json.RawMessage(`{"tool":"test"}`)

	if err := store.Log(ctx, toolInput, DecisionAllow, "original"); err != nil {
		t.Fatalf("failed to log: %v", err)
	}

	// Attempt UPDATE - should fail
	_, err := store.db.ExecContext(ctx, "UPDATE audit_log SET reason = 'modified' WHERE id = 1")
	if err == nil {
		t.Error("expected UPDATE to fail, but it succeeded")
	}
	if !strings.Contains(err.Error(), "not allowed") && !strings.Contains(err.Error(), "FAIL") {
		t.Errorf("expected trigger error, got: %v", err)
	}

	// Attempt DELETE - should fail
	_, err = store.db.ExecContext(ctx, "DELETE FROM audit_log WHERE id = 1")
	if err == nil {
		t.Error("expected DELETE to fail, but it succeeded")
	}
	if !strings.Contains(err.Error(), "not allowed") && !strings.Contains(err.Error(), "FAIL") {
		t.Errorf("expected trigger error, got: %v", err)
	}

	// Verify original entry unchanged
	entries, _ := store.GetAll(ctx)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Reason != "original" {
		t.Errorf("expected reason 'original', got '%s'", entries[0].Reason)
	}
}

func TestConcurrentWrites(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	ctx := context.Background()
	toolInput := json.RawMessage(`{"tool":"concurrent"}`)

	const numWrites = 20
	errChan := make(chan error, numWrites)
	doneChan := make(chan struct{})

	// Stagger writes slightly to reduce lock contention
	for i := 0; i < numWrites; i++ {
		go func(id int) {
			time.Sleep(time.Duration(id) * time.Millisecond)
			errChan <- store.Log(ctx, toolInput, DecisionAllow, "concurrent test")
		}(i)
	}

	go func() {
		for i := 0; i < numWrites; i++ {
			<-errChan
		}
		close(doneChan)
	}()

	select {
	case <-doneChan:
		// All writes completed
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for concurrent writes")
	}

	// Verify all writes succeeded
	entries, err := store.GetAll(ctx)
	if err != nil {
		t.Fatalf("failed to get entries: %v", err)
	}

	if len(entries) != numWrites {
		t.Errorf("expected %d entries, got %d", numWrites, len(entries))
	}
}

func TestSequentialWrites(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	ctx := context.Background()

	// Test rapid sequential writes (more realistic for real usage)
	for i := 0; i < 100; i++ {
		toolInput := json.RawMessage(fmt.Sprintf(`{"tool":"seq","id":%d}`, i))
		if err := store.Log(ctx, toolInput, DecisionAllow, "sequential"); err != nil {
			t.Fatalf("write %d failed: %v", i, err)
		}
	}

	entries, err := store.GetAll(ctx)
	if err != nil {
		t.Fatalf("failed to get entries: %v", err)
	}

	if len(entries) != 100 {
		t.Errorf("expected 100 entries, got %d", len(entries))
	}
}

func TestValidation(t *testing.T) {
	tests := []struct {
		name      string
		input     json.RawMessage
		decision  Decision
		reason    string
		expectErr bool
	}{
		{"valid", json.RawMessage(`{}`), DecisionAllow, "test", false},
		{"empty input", json.RawMessage(``), DecisionAllow, "test", true},
		{"invalid json", json.RawMessage(`{bad`), DecisionAllow, "test", true},
		{"invalid decision", json.RawMessage(`{}`), "invalid", "test", true},
		{"empty reason", json.RawMessage(`{}`), DecisionAllow, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateLogInput(tt.input, tt.decision, tt.reason)
			if (err != nil) != tt.expectErr {
				t.Errorf("expected error: %v, got: %v", tt.expectErr, err)
			}
		})
	}
}

func setupTestStore(t *testing.T) *SQLiteStore {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	return store
}