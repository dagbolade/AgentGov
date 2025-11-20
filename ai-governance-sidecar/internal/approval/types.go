package approval

import (
	"context"
	"encoding/json"
	"time"

	"github.com/dagbolade/ai-governance-sidecar/internal/policy"
)

type Status string

const (
	StatusPending  Status = "pending"
	StatusApproved Status = "approved"
	StatusDenied   Status = "denied"
	StatusTimeout  Status = "timeout"
)

type Request struct {
	ID        string              `json:"id"`
	ToolName  string              `json:"tool_name"`
	Args      json.RawMessage     `json:"args"`
	Reason    string              `json:"reason"`
	CreatedAt time.Time           `json:"created_at"`
	Status    Status              `json:"status"`
	decidedBy string              `json:"-"`
	resultCh  chan<- Decision     `json:"-"`
}

type Decision struct {
	Approved bool   `json:"approved"`
	Reason   string `json:"reason"`
	DecidedBy string `json:"decided_by,omitempty"`
}

type Queue interface {
	Enqueue(ctx context.Context, req policy.Request, reason string) (Decision, error)
	GetPending(ctx context.Context) ([]Request, error)
	Decide(ctx context.Context, id string, decision Decision) error
	NotifyChannel() <-chan struct{} //Added for the WebSocket handler
	Close() error
}