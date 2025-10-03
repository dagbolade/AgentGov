package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/dagbolade/ai-governance-sidecar/internal/approval"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins in development
	},
}

type WSHandler struct {
	queue   approval.Queue
	clients map[*websocket.Conn]bool
	mu      sync.RWMutex
}

func NewWSHandler(queue approval.Queue) *WSHandler {
	handler := &WSHandler{
		queue:   queue,
		clients: make(map[*websocket.Conn]bool),
	}
	
	go handler.watchApprovals()
	
	return handler
}

func (h *WSHandler) HandleWebSocket(c echo.Context) error {
	ws, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		log.Error().Err(err).Msg("websocket upgrade failed")
		return err
	}
	defer ws.Close()

	h.addClient(ws)
	defer h.removeClient(ws)

	log.Info().Msg("websocket client connected")

	// Send current pending approvals
	if err := h.sendPending(ws); err != nil {
		log.Error().Err(err).Msg("failed to send pending approvals")
		return err
	}

	// Keep connection alive and handle client messages
	for {
		_, _, err := ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Error().Err(err).Msg("websocket read error")
			}
			break
		}
	}

	return nil
}

func (h *WSHandler) watchApprovals() {
	if q, ok := h.queue.(*approval.InMemoryQueue); ok {
		notifyCh := q.NotifyChannel()
		for range notifyCh {
			h.broadcastPending()
		}
	}
}

func (h *WSHandler) broadcastPending() {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		if err := h.sendPending(client); err != nil {
			log.Warn().Err(err).Str("remote_addr", client.RemoteAddr().String()).Msg("failed to broadcast to websocket client")
		}
	}
}

func (h *WSHandler) sendPending(ws *websocket.Conn) error {
	pending, err := h.queue.GetPending(context.Background())
	if err != nil {
		log.Error().Err(err).Msg("failed to get pending approvals for websocket")
		return fmt.Errorf("get pending: %w", err)
	}

	msg := map[string]interface{}{
		"type":    "pending_update",
		"total":   len(pending),
		"pending": pending,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		log.Error().Err(err).Msg("failed to marshal pending approvals")
		return fmt.Errorf("marshal message: %w", err)
	}

	if err := ws.WriteMessage(websocket.TextMessage, data); err != nil {
		return fmt.Errorf("write websocket message: %w", err)
	}

	return nil
}

func (h *WSHandler) addClient(ws *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[ws] = true
}

func (h *WSHandler) removeClient(ws *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, ws)
	log.Info().Msg("websocket client disconnected")
}