package policy

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatcherCreation(t *testing.T) {
	dir := t.TempDir()

	handler := func(path string) {
		// Handler will be tested in other tests
	}

	watcher, err := NewFileWatcher(dir, handler)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer watcher.Close()

	if watcher.dir != dir {
		t.Errorf("expected dir %s, got %s", dir, watcher.dir)
	}
}

func TestWatcherFileChange(t *testing.T) {
	dir := t.TempDir()
	changeChan := make(chan string, 1)

	handler := func(path string) {
		changeChan <- path
	}

	watcher, err := NewFileWatcher(dir, handler)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer watcher.Close()

	// Create a WASM file
	testFile := filepath.Join(dir, "test.wasm")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Wait for change detection (with timeout)
	select {
	case path := <-changeChan:
		if path != testFile {
			t.Errorf("expected change for %s, got %s", testFile, path)
		}
	case <-time.After(2 * time.Second):
		t.Error("timeout waiting for file change detection")
	}
}

func TestWatcherIgnoresNonWASM(t *testing.T) {
	dir := t.TempDir()
	changeChan := make(chan string, 1)

	handler := func(path string) {
		changeChan <- path
	}

	watcher, err := NewFileWatcher(dir, handler)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer watcher.Close()

	// Create a non-WASM file
	testFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Should NOT trigger handler
	select {
	case path := <-changeChan:
		t.Errorf("unexpected change detection for %s", path)
	case <-time.After(1 * time.Second):
		// Expected - no change should be detected
	}
}