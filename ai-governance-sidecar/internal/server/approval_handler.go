package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/dagbolade/ai-governance-sidecar/internal/approval"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

type ApprovalHandler struct {
	queue           approval.Queue
	approvalTimeout time.Duration
	wsHub           *Hub // Reference to broadcast decisions
}

// NewApprovalHandler creates approval handler with timeout
func NewApprovalHandler(queue approval.Queue, timeout time.Duration, wsHub *Hub) *ApprovalHandler {
	return &ApprovalHandler{
		queue:           queue,
		approvalTimeout: timeout,
		wsHub:           wsHub,
	}
}

// UI shape for an approval card
type uiApproval struct {
	ApprovalID string                 `json:"approval_id"`
	CreatedAt  time.Time              `json:"created_at"`
	ExpiresAt  *time.Time             `json:"expires_at,omitempty"`
	Reason     string                 `json:"reason,omitempty"`
	Confidence *float64               `json:"confidence,omitempty"`
	Request    map[string]interface{} `json:"request"`
	Status     string                 `json:"status"`
}

// GetPending returns pending approvals (legacy format)
func (h *ApprovalHandler) GetPending(c echo.Context) error {
	ctx := c.Request().Context()

	pending, err := h.queue.GetPending(ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to get pending approvals")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to retrieve pending approvals",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"total":   len(pending),
		"pending": pending,
	})
}

// GetPendingV2 returns pending approvals in UI-friendly format
func (h *ApprovalHandler) GetPendingV2(c echo.Context) error {
	ctx := c.Request().Context()

	items, err := h.queue.GetPending(ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to get pending approvals (v2)")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to retrieve pending approvals",
		})
	}

	resp := make([]uiApproval, 0, len(items))
	for _, it := range items {
		u := uiApproval{
			ApprovalID: it.ID,
			CreatedAt:  it.CreatedAt,
			Reason:     it.Reason,
			Status:     string(it.Status),
		}

		// Calculate expires_at
		if h.approvalTimeout > 0 {
			expiresAt := it.CreatedAt.Add(h.approvalTimeout)
			u.ExpiresAt = &expiresAt
		}

		// Build request object with proper structure
		req := h.buildRequestObject(it)
		u.Request = req

		resp = append(resp, u)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"total":     len(resp),
		"approvals": resp,
	})
}

// buildRequestObject constructs UI-friendly request structure
func (h *ApprovalHandler) buildRequestObject(item approval.Request) map[string]interface{} {
	req := map[string]interface{}{
		"tool": item.ToolName,
	}

	// Try to extract action and parameters from Args
	if len(item.Args) > 0 {
		var argsMap map[string]interface{}
		if json.Unmarshal(item.Args, &argsMap) == nil {
			// If args has "action" field, extract it
			if action, ok := argsMap["action"]; ok {
				req["action"] = action
			}

			// If args has "parameters" field, use it; otherwise use entire args as parameters
			if params, ok := argsMap["parameters"]; ok {
				req["parameters"] = params
			} else {
				req["parameters"] = argsMap
			}
		} else {
			// If not valid JSON object, include raw args
			req["parameters"] = json.RawMessage(item.Args)
		}
	}

	return req
}

// ListApprovals handles GET /approvals?status=pending
func (h *ApprovalHandler) ListApprovals(c echo.Context) error {
	status := c.QueryParam("status")
	if status == "" || status == "pending" {
		return h.GetPendingV2(c)
	}

	// Future: support other statuses (approved, denied, expired)
	return c.JSON(http.StatusBadRequest, map[string]string{
		"error": "only status=pending is currently supported",
	})
}

// Approve handles POST /approvals/:id/approve
func (h *ApprovalHandler) Approve(c echo.Context) error {
	return h.decideV2(c, true)
}

// Deny handles POST /approvals/:id/deny
func (h *ApprovalHandler) Deny(c echo.Context) error {
	return h.decideV2(c, false)
}

// decideV2 handles approval/denial with WebSocket notification
func (h *ApprovalHandler) decideV2(c echo.Context, approved bool) error {
	ctx := c.Request().Context()
	id := c.Param("id")

	var req struct {
		Approver string `json:"approver"`
		Comment  string `json:"comment"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
		})
	}

	// Validate inputs
	if req.Approver == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "approver is required",
		})
	}

	if !approved && req.Comment == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "comment is required for denial",
		})
	}

	// Create decision
	decision := approval.Decision{
		Approved:  approved,
		Reason:    req.Comment,
		DecidedBy: req.Approver,
	}

	// Apply decision
	if err := h.queue.Decide(ctx, id, decision); err != nil {
		log.Error().Err(err).Str("id", id).Bool("approved", approved).Msg("failed to decide approval")
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "approval request not found or already processed",
		})
	}

	// Broadcast decision via WebSocket
	statusStr := "denied"
	if approved {
		statusStr = "approved"
	}
	
	if h.wsHub != nil {
		h.wsHub.BroadcastApprovalDecision(id, statusStr)
	}

	log.Info().
		Str("id", id).
		Bool("approved", approved).
		Str("approver", req.Approver).
		Msg("approval decision made")

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"id":      id,
		"status":  statusStr,
	})
}

// Decide handles legacy POST /approve/:id format
func (h *ApprovalHandler) Decide(c echo.Context) error {
	ctx := c.Request().Context()
	id := c.Param("id")

	var req struct {
		Approved  bool   `json:"approved"`
		Reason    string `json:"reason"`
		DecidedBy string `json:"decided_by,omitempty"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
		})
	}

	if req.Reason == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "reason is required",
		})
	}

	decision := approval.Decision{
		Approved:  req.Approved,
		Reason:    req.Reason,
		DecidedBy: req.DecidedBy,
	}

	if err := h.queue.Decide(ctx, id, decision); err != nil {
		log.Error().Err(err).Str("id", id).Msg("failed to decide approval")
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "approval request not found",
		})
	}

	// Broadcast via WebSocket
	if h.wsHub != nil {
		statusStr := "denied"
		if req.Approved {
			statusStr = "approved"
		}
		h.wsHub.BroadcastApprovalDecision(id, statusStr)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success":  true,
		"id":       id,
		"decision": decision,
	})
}