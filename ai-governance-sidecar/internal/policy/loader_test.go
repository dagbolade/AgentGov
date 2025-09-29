package policy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoaderFileDetection(t *testing.T) {
	loader := NewWASMLoader()

	tests := []struct {
		filename string
		expected bool
	}{
		{"policy.wasm", true},
		{"policy.WASM", true},
		{"policy.txt", false},
		{"policy.wasm.bak", false},
		{"wasm", false},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := loader.isWASMFile(tt.filename)
			if result != tt.expected {
				t.Errorf("isWASMFile(%s) = %v, want %v", tt.filename, result, tt.expected)
			}
		})
	}
}

func TestLoaderPolicyNameExtraction(t *testing.T) {
	loader := NewWASMLoader()

	tests := []struct {
		filename string
		expected string
	}{
		{"my_policy.wasm", "my_policy"},
		{"ALLOW_ALL.WASM", "allow_all"},
		{"test.wasm", "test"},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := loader.extractPolicyName(tt.filename)
			if result != tt.expected {
				t.Errorf("extractPolicyName(%s) = %s, want %s", tt.filename, result, tt.expected)
			}
		})
	}
}

func TestLoaderEmptyDirectory(t *testing.T) {
	loader := NewWASMLoader()
	dir := t.TempDir()

	_, err := loader.LoadFromDir(dir)
	if err == nil {
		t.Error("expected error when loading from empty directory")
	}
}

func TestLoaderInvalidWASM(t *testing.T) {
	loader := NewWASMLoader()
	dir := t.TempDir()

	// Create invalid WASM file
	invalidPath := filepath.Join(dir, "invalid.wasm")
	if err := os.WriteFile(invalidPath, []byte("not wasm"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := loader.LoadFromDir(dir)
	if err == nil {
		t.Error("expected error when loading invalid WASM")
	}
}