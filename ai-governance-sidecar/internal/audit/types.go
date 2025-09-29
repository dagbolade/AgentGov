package audit

import (
	"context"
	"encoding/json"
	"time"
)

type Decision string

const (
	DecisionAllow Decision = "allow"
	DecisionDeny  Decision = "deny"
)

type Entry struct {
	ID        int64           `json:"id"`
	Timestamp time.Time       `json:"timestamp"`
	ToolInput json.RawMessage `json:"tool_input"`
	Decision  Decision        `json:"decision"`
	Reason    string          `json:"reason"`
}

type Store interface {
	Log(ctx context.Context, toolInput json.RawMessage, decision Decision, reason string) error
	GetAll(ctx context.Context) ([]Entry, error)
	Close() error
}