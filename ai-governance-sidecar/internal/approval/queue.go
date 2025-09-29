package approval

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/dagbolade/ai-governance-sidecar/internal/policy"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

type InMemoryQueue struct {
	mu       sync.RWMutex
	pending  map[string]*Request
	timeout  time.Duration
	notifyCh chan struct{}
}

func NewInMemoryQueue(timeout time.Duration) *InMemoryQueue {
	return &InMemoryQueue{
		pending:  make(map[string]*Request),
		timeout:  timeout,
		notifyCh: make(chan struct{}, 100),
	}
}

func (q *InMemoryQueue) Enqueue(ctx context.Context, req policy.Request, reason string) (Decision, error) {
	reqID := uuid.New().String()
	resultCh := make(chan Decision, 1)

	approvalReq := &Request{
		ID:        reqID,
		ToolName:  req.ToolName,
		Args:      req.Args,
		Reason:    reason,
		CreatedAt: time.Now(),
		Status:    StatusPending,
		resultCh:  resultCh,
	}

	q.addPending(approvalReq)
	q.notifyWatchers()

	log.Info().Str("id", reqID).Str("tool", req.ToolName).Msg("approval request enqueued")

	return q.waitForDecision(ctx, reqID, resultCh)
}

func (q *InMemoryQueue) GetPending(ctx context.Context) ([]Request, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	pending := make([]Request, 0, len(q.pending))
	for _, req := range q.pending {
		pending = append(pending, *req)
	}

	return pending, nil
}

func (q *InMemoryQueue) Decide(ctx context.Context, id string, decision Decision) error {
	q.mu.Lock()
	req, exists := q.pending[id]
	if !exists {
		q.mu.Unlock()
		return fmt.Errorf("request not found: %s", id)
	}

	delete(q.pending, id)
	q.mu.Unlock()

	req.Status = q.statusFromDecision(decision)
	req.decidedBy = decision.DecidedBy

	select {
	case req.resultCh <- decision:
		log.Info().Str("id", id).Bool("approved", decision.Approved).Msg("approval decision made")
	default:
		log.Warn().Str("id", id).Msg("result channel closed, decision dropped")
	}

	return nil
}

func (q *InMemoryQueue) NotifyChannel() <-chan struct{} {
	return q.notifyCh
}

func (q *InMemoryQueue) Close() error {
	q.mu.Lock()
	defer q.mu.Unlock()

	for id, req := range q.pending {
		close(req.resultCh)
		delete(q.pending, id)
	}

	close(q.notifyCh)
	return nil
}

func (q *InMemoryQueue) addPending(req *Request) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.pending[req.ID] = req
}

func (q *InMemoryQueue) waitForDecision(ctx context.Context, id string, resultCh <-chan Decision) (Decision, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, q.timeout)
	defer cancel()

	select {
	case decision := <-resultCh:
		return decision, nil
	case <-timeoutCtx.Done():
		q.handleTimeout(id)
		return Decision{Approved: false, Reason: "approval timeout"}, nil
	case <-ctx.Done():
		q.handleTimeout(id)
		return Decision{Approved: false, Reason: "request cancelled"}, ctx.Err()
	}
}

func (q *InMemoryQueue) handleTimeout(id string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if req, exists := q.pending[id]; exists {
		req.Status = StatusTimeout
		delete(q.pending, id)
		close(req.resultCh)
		log.Warn().Str("id", id).Msg("approval request timeout")
	}
}

func (q *InMemoryQueue) notifyWatchers() {
	select {
	case q.notifyCh <- struct{}{}:
	default:
	}
}

func (q *InMemoryQueue) statusFromDecision(d Decision) Status {
	if d.Approved {
		return StatusApproved
	}
	return StatusDenied
}