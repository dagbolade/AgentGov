package server

import (
	"net/http"

	"github.com/dagbolade/ai-governance-sidecar/internal/audit"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

type AuditHandler struct {
	store audit.Store
}

func NewAuditHandler(store audit.Store) *AuditHandler {
	return &AuditHandler{store: store}
}

func (h *AuditHandler) GetAuditLog(c echo.Context) error {
	ctx := c.Request().Context()

	entries, err := h.store.GetAll(ctx)
	if err != nil {
		log.Error().Err(err).Str("remote_addr", c.Request().RemoteAddr).Msg("failed to retrieve audit log")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to retrieve audit log",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"total":   len(entries),
		"entries": entries,
	})
}