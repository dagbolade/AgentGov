package server

import (
	"net/http"
	"time"

	"github.com/dagbolade/ai-governance-sidecar/internal/approval"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

type WSHandler struct {
	queue    approval.Queue
	upgrader websocket.Upgrader
}

func NewWSHandler(q approval.Queue) *WSHandler {
	return &WSHandler{
		queue: q,
		upgrader: websocket.Upgrader{
			// We’re already behind your auth middleware; allow the UI origin.
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

const (
	writeWait  = 10 * time.Second
    pongWait   = 30 * time.Second      // how long we wait for a pong
    pingPeriod = 10 * time.Second      // send a ping every 10 seconds (must be < pongWait)
	// maxMessageSize optional; browsers don’t send big frames by default
	// maxMessageSize = 1 << 20
)

func (h *WSHandler) HandleWebSocket(c echo.Context) error {
	// Upgrade
	conn, err := h.upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}
	log.Info().Msg("websocket client connected")
	defer func() {
		_ = conn.Close()
		log.Info().Msg("websocket client disconnected")
	}()

	// ---- Keepalive (server-driven ping; extend read deadline on pong) ----
	// conn.SetReadLimit(maxMessageSize)
	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	// Send initial snapshot so the UI has something immediately
	if err := h.sendPendingSnapshot(conn, c); err != nil {
		return nil // client likely gone; don’t spam logs
	}

	// Reader: just drain messages to keep the connection alive (browser pongs automatically)
	errCh := make(chan error, 1)
	go func() {
		for {
			// We don’t expect client messages; just read to detect close/pong
			if _, _, err := conn.ReadMessage(); err != nil {
				errCh <- err
				return
			}
		}
	}()

	// Writer: periodic pings; you can also push “pending_update” here on your own tick
	ping := time.NewTicker(pingPeriod)
	defer ping.Stop()

	for {
		select {
		case <-ping.C:
			if err := writePing(conn); err != nil {
				return nil
			}
			// OPTIONAL: push periodic snapshot(s) to update the UI
			// _ = h.sendPendingSnapshot(conn, c)

		case err := <-errCh:
			// Close 1005 (no status) is normal when timeouts/refreshes happen; don’t error log loudly
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Error().Err(err).Msg("websocket read error")
			}
			return nil

		// case <-c.Request().Context().Done():
		//	return nil
		}
	}
}

func (h *WSHandler) sendPendingSnapshot(conn *websocket.Conn, c echo.Context) error {
	ctx := c.Request().Context()
	items, err := h.queue.GetPending(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("failed to load pending approvals for ws snapshot")
		return nil
	}
	msg := map[string]any{
		"type":    "pending_update",
		"total":   len(items),
		"pending": items, // your UI already understands this shape
	}
	_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
	return conn.WriteJSON(msg)
}

func writePing(conn *websocket.Conn) error {
	_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
	return conn.WriteMessage(websocket.PingMessage, nil)
}