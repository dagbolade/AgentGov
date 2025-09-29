package server

import (
	"net/http"

	"github.com/dagbolade/ai-governance-sidecar/internal/approval"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

type ApprovalHandler struct {
	queue approval.Queue
}

func NewApprovalHandler(queue approval.Queue) *ApprovalHandler {
	return &ApprovalHandler{queue: queue}
}

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
		"success": true,
		"id":      id,
		"decision": decision,
	})
}