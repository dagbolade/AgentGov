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
	queue          approval.Queue
	// optional: if you want to surface expires_at, pass this in from config
	// and compute created_at + timeout. If 0, we omit expires_at.
	approvalTimeout time.Duration
}

func NewApprovalHandler(queue approval.Queue) *ApprovalHandler {
	return &ApprovalHandler{queue: queue}
}

// If you want expires_at, use this ctor instead and call it from server.go:
// func NewApprovalHandlerWithTimeout(queue approval.Queue, timeout time.Duration) *ApprovalHandler {
// 	return &ApprovalHandler{queue: queue, approvalTimeout: timeout}
// }

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

// ---------- V2/UI-friendly endpoints ----------

// UI shape for an approval card
type uiApproval struct {
	ApprovalID string                 `json:"approval_id"`
	CreatedAt  time.Time              `json:"created_at"`
	ExpiresAt  *time.Time             `json:"expires_at,omitempty"`
	Reason     string                 `json:"reason,omitempty"`
	Confidence *float64               `json:"confidence,omitempty"`
	Request    map[string]interface{} `json:"request,omitempty"`
	Status     string                 `json:"status,omitempty"`
}

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

		// expires_at (optional)
		if h.approvalTimeout > 0 {
			t := it.CreatedAt.Add(h.approvalTimeout)
			u.ExpiresAt = &t
		}

		// build a minimal request object for the UI labels
		req := map[string]interface{}{
			"tool": it.ToolName,
		}
		// try to extract common fields from Args
		if len(it.Args) > 0 {
			var m map[string]interface{}
			if json.Unmarshal(it.Args, &m) == nil {
				// Pass through recognizable keys if present
				if v, ok := m["action"]; ok {
					req["action"] = v
				}
				if v, ok := m["parameters"]; ok {
					req["parameters"] = v
				} else {
					// otherwise include raw args for the "details" drawer
					req["parameters"] = m
				}
			}
		}
		u.Request = req
		resp = append(resp, u)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"total":     len(resp),
		"approvals": resp,
	})
}

// GET /approvals?status=pending   (only "pending" supported for now)
func (h *ApprovalHandler) ListApprovals(c echo.Context) error {
	status := c.QueryParam("status")
	if status == "" || status == "pending" {
		return h.GetPendingV2(c)
	}
	// not implemented yet for other statuses
	return c.JSON(http.StatusNotFound, map[string]string{
		"error": "status not supported",
	})
}

// POST /approvals/:id/approve   { approver, comment }
func (h *ApprovalHandler) Approve(c echo.Context) error {
	return h.decideV2(c, true)
}

// POST /approvals/:id/deny      { approver, comment }
func (h *ApprovalHandler) Deny(c echo.Context) error {
	return h.decideV2(c, false)
}

func (h *ApprovalHandler) decideV2(c echo.Context, approved bool) error {
	ctx := c.Request().Context()
	id := c.Param("id")

	var req struct {
		Approver string `json:"approver"`
		Comment  string `json:"comment"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	// For deny we require a reason; for approve it's optional in the UI but we still allow empty.
	if !approved && req.Comment == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "comment is required to deny"})
	}

	decision := approval.Decision{
		Approved:  approved,
		Reason:    req.Comment,
		DecidedBy: req.Approver,
	}

	if err := h.queue.Decide(ctx, id, decision); err != nil {
		log.Error().Err(err).Str("id", id).Msg("failed to decide approval (v2)")
		return c.JSON(http.StatusNotFound, map[string]string{"error": "approval request not found"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"id":      id,
		"status":  map[bool]string{true: "approved", false: "denied"}[approved],
	})
}

// ---------- Back-compat endpoint you already had ----------

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

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success":  true,
		"id":       id,
		"decision": decision,
	})
}
