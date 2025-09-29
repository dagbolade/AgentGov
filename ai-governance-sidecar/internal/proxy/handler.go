package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/dagbolade/ai-governance-sidecar/internal/audit"
	"github.com/dagbolade/ai-governance-sidecar/internal/policy"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

type Handler struct {
	config    ProxyConfig
	policy    policy.Evaluator
	audit     audit.Store
	forwarder *Forwarder
}

func NewHandler(cfg ProxyConfig, pol policy.Evaluator, aud audit.Store) *Handler {
	return &Handler{
		config:    cfg,
		policy:    pol,
		audit:     aud,
		forwarder: NewForwarder(cfg.Timeout),
	}
}

func (h *Handler) HandleToolCall(c echo.Context) error {
	ctx := c.Request().Context()
	
	req, err := h.parseRequest(c)
	if err != nil {
		return h.errorResponse(c, http.StatusBadRequest, err.Error())
	}

	decision, err := h.evaluatePolicy(ctx, req)
	if err != nil {
		return h.errorResponse(c, http.StatusInternalServerError, "policy evaluation failed")
	}

	if err := h.logAudit(ctx, req, decision); err != nil {
		log.Warn().Err(err).Msg("audit logging failed")
	}

	if !decision.Allow {
		return h.denyResponse(c, decision.Reason)
	}

	if decision.HumanRequired {
		return h.errorResponse(c, http.StatusNotImplemented, "human approval not yet implemented")
	}

	return h.forwardRequest(ctx, c, req)
}

func (h *Handler) parseRequest(c echo.Context) (*ToolCallRequest, error) {
	var req ToolCallRequest
	if err := c.Bind(&req); err != nil {
		return nil, fmt.Errorf("invalid request body: %w", err)
	}

	if req.ToolName == "" {
		return nil, fmt.Errorf("tool_name is required")
	}

	if req.Upstream == "" {
		req.Upstream = h.config.DefaultUpstream
	}

	return &req, nil
}

func (h *Handler) evaluatePolicy(ctx context.Context, req *ToolCallRequest) (policy.Response, error) {
	evalCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return h.policy.Evaluate(evalCtx, req.ToPolicyRequest())
}

func (h *Handler) logAudit(ctx context.Context, req *ToolCallRequest, decision policy.Response) error {
	toolInput, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	auditDecision := audit.DecisionDeny
	if decision.Allow {
		auditDecision = audit.DecisionAllow
	}

	return h.audit.Log(ctx, toolInput, auditDecision, decision.Reason)
}

func (h *Handler) forwardRequest(ctx context.Context, c echo.Context, req *ToolCallRequest) error {
	result, err := h.forwarder.Forward(ctx, req.Upstream, req)
	if err != nil {
		log.Error().Err(err).Str("upstream", req.Upstream).Msg("forward failed")
		return h.errorResponse(c, http.StatusBadGateway, "upstream request failed")
	}

	return c.JSON(http.StatusOK, ToolCallResponse{
		Success: true,
		Result:  result,
	})
}

func (h *Handler) denyResponse(c echo.Context, reason string) error {
	return c.JSON(http.StatusForbidden, ToolCallResponse{
		Success: false,
		Error:   reason,
	})
}

func (h *Handler) errorResponse(c echo.Context, status int, message string) error {
	return c.JSON(status, ToolCallResponse{
		Success: false,
		Error:   message,
	})
}