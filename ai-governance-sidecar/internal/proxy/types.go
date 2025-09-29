package proxy

import (
	"encoding/json"

	"github.com/dagbolade/ai-governance-sidecar/internal/policy"
)

type ToolCallRequest struct {
	ToolName string          `json:"tool_name"`
	Args     json.RawMessage `json:"args"`
	Upstream string          `json:"upstream,omitempty"`
}

type ToolCallResponse struct {
	Success bool            `json:"success"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   string          `json:"error,omitempty"`
}

type ProxyConfig struct {
	DefaultUpstream string
	Timeout         int // seconds
}

func (r *ToolCallRequest) ToPolicyRequest() policy.Request {
	return policy.Request{
		ToolName: r.ToolName,
		Args:     r.Args,
		Metadata: map[string]any{
			"upstream": r.Upstream,
		},
	}
}