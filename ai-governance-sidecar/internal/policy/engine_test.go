package policy

import (
	"context"
	"encoding/json"
	"testing"
)

type mockEvaluator struct {
	response Response
	err      error
}

func (m *mockEvaluator) Evaluate(ctx context.Context, req Request) (Response, error) {
	if m.err != nil {
		return Response{}, m.err
	}
	return m.response, nil
}

func (m *mockEvaluator) Reload() error { return nil }
func (m *mockEvaluator) Close() error  { return nil }

func TestEngineEvaluation(t *testing.T) {
	engine := &Engine{
		evaluators: map[string]*WASMEvaluator{},
	}

	ctx := context.Background()
	req := Request{
		ToolName: "test_tool",
		Args:     json.RawMessage(`{"key":"value"}`),
	}

	resp, err := engine.Evaluate(ctx, req)
	if err != nil {
		t.Fatalf("evaluation failed: %v", err)
	}

	if resp.Allow {
		t.Error("expected deny when no policies loaded")
	}

	if resp.Reason != "no policies loaded" {
		t.Errorf("unexpected reason: %s", resp.Reason)
	}
}

func TestEngineReload(t *testing.T) {
	policyDir := t.TempDir()

	engine := &Engine{
		evaluators: make(map[string]*WASMEvaluator),
		loader:     NewWASMLoader(),
	}

	err := engine.loadPolicies(policyDir)
	if err == nil {
		t.Error("expected error when loading from empty directory")
	}

	if err.Error() != "no valid WASM policies found in directory: "+policyDir {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDenyResponse(t *testing.T) {
	engine := &Engine{}

	resp := engine.denyResponse("test reason")

	if resp.Allow {
		t.Error("expected Allow to be false")
	}

	if resp.Reason != "test reason" {
		t.Errorf("expected reason 'test reason', got '%s'", resp.Reason)
	}

	if resp.HumanRequired {
		t.Error("expected HumanRequired to be false")
	}
}
