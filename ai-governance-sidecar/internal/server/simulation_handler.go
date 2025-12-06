package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/dagbolade/ai-governance-sidecar/internal/approval"
	"github.com/dagbolade/ai-governance-sidecar/internal/policy"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// SimulationHandler handles test/demo approval creation
type SimulationHandler struct {
	queue approval.Queue
	wsHub *Hub
}

// NewSimulationHandler creates simulation handler
func NewSimulationHandler(queue approval.Queue, wsHub *Hub) *SimulationHandler {
	return &SimulationHandler{
		queue: queue,
		wsHub: wsHub,
	}
}

// SimulateEnqueueRequest for testing
type SimulateEnqueueRequest struct {
	ToolName string                 `json:"tool_name"`
	Args     map[string]interface{} `json:"args"`
	Reason   string                 `json:"reason"`
}

// SimulateEnqueueResponse returns approval ID
type SimulateEnqueueResponse struct {
	ApprovalID string `json:"approval_id"`
	Status     string `json:"status"`
}

// Enqueue creates a test approval request
func (h *SimulationHandler) Enqueue(c echo.Context) error {
	// 1. Use Background Context to prevent "Context Canceled" errors
	// This ensures the DB write completes even if the HTTP request ends quickly.
	bgCtx := context.Background()
	
	var req SimulateEnqueueRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid request body",
		})
	}

	// Set defaults
	if req.ToolName == "" {
		req.ToolName = "demo_tool"
	}
	if req.Reason == "" {
		req.Reason = "simulation"
	}
	if req.Args == nil {
		req.Args = make(map[string]interface{})
	}

	// Convert args to JSON
	argsJSON, err := json.Marshal(req.Args)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid args format",
		})
	}

	// Create policy request
	policyReq := policy.Request{
		ToolName: req.ToolName,
		Args:     argsJSON,
	}

	// 2. Enqueue in background using the SAFE background context
	go func() {
		_, err := h.queue.Enqueue(bgCtx, policyReq, req.Reason)
		if err != nil {
			log.Warn().Err(err).Msg("simulation enqueue failed")
		}
	}()

	// 3. RETRY LOGIC: Wait briefly for the DB write to finish
	// This fixes the "Race Condition" where we looked for the ID before it existed.
	var approvalID string
	var pending []approval.Request
	
	// Try 5 times (total 500ms wait)
	for i := 0; i < 5; i++ {
		time.Sleep(100 * time.Millisecond) 
		
		// We use the request context here for reading, which is fine
		pending, err = h.queue.GetPending(c.Request().Context())
		if err == nil && len(pending) > 0 {
			// Check if the last item matches our tool
			lastItem := pending[len(pending)-1]
			if lastItem.ToolName == req.ToolName {
				approvalID = lastItem.ID
				break
			}
		}
	}

	// If still not found after retries, return error
	if approvalID == "" {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "enqueue_timeout_db_write_slow",
		})
	}

	// Broadcast update via WebSocket
	if h.wsHub != nil {
		h.wsHub.broadcastPendingUpdate()
	}

	log.Info().
		Str("approval_id", approvalID).
		Str("tool", req.ToolName).
		Str("reason", req.Reason).
		Msg("simulation approval enqueued")

	return c.JSON(http.StatusOK, SimulateEnqueueResponse{
		ApprovalID: approvalID,
		Status:     "pending",
	})
}