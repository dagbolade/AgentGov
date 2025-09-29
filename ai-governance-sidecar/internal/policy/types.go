package policy

import (
	"context"
	"encoding/json"
)

// Request represents a tool call to be evaluated
type Request struct {
	ToolName string          `json:"tool_name"`
	Args     json.RawMessage `json:"args"`
	Metadata map[string]any  `json:"metadata,omitempty"`
}

// Response represents the policy decision
type Response struct {
	Allow          bool   `json:"allow"`
	Reason         string `json:"reason"`
	HumanRequired  bool   `json:"human_required"`
}

// Evaluator evaluates tool call requests against policies
type Evaluator interface {
	Evaluate(ctx context.Context, req Request) (Response, error)
	Reload() error
	Close() error
}