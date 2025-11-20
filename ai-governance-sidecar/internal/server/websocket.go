package server

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/dagbolade/ai-governance-sidecar/internal/approval"
	"github.com/dagbolade/ai-governance-sidecar/internal/auth"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = 30 * time.Second
	maxMessageSize = 512 * 1024 // 512KB
)

// WSMessage represents messages sent to clients
type WSMessage struct {
	Type       string      `json:"type"`
	ApprovalID string      `json:"approval_id,omitempty"`
	Status     string      `json:"status,omitempty"`
	Data       interface{} `json:"data,omitempty"`
}

// Client represents a WebSocket client
type Client struct {
	id       string
	conn     *websocket.Conn
	send     chan WSMessage
	hub      *Hub
	user     *auth.User
	closedMu sync.Mutex
	closed   bool
}

// Hub maintains active WebSocket connections and broadcasts messages
type Hub struct {
	clients      map[*Client]bool
	broadcast    chan WSMessage
	register     chan *Client
	unregister   chan *Client
	mu           sync.RWMutex
	queue        approval.Queue
	authManager  *auth.Manager
	ctx          context.Context
	cancel       context.CancelFunc
	shutdownOnce sync.Once
}

// NewHub creates a new WebSocket hub
func NewHub(queue approval.Queue, authManager *auth.Manager) *Hub {
	ctx, cancel := context.WithCancel(context.Background())
	h := &Hub{
		clients:     make(map[*Client]bool),
		broadcast:   make(chan WSMessage, 256),
		register:    make(chan *Client),
		unregister:  make(chan *Client),
		queue:       queue,
		authManager: authManager,
		ctx:         ctx,
		cancel:      cancel,
	}
	go h.run()
	go h.watchApprovalQueue()
	return h
}

// Shutdown gracefully shuts down the hub
func (h *Hub) Shutdown() {
	h.shutdownOnce.Do(func() {
		log.Info().Msg("shutting down websocket hub")
		h.cancel()
		close(h.register)
		close(h.unregister)
		close(h.broadcast)
		
		h.mu.Lock()
		for client := range h.clients {
			client.safeClose()
		}
		h.mu.Unlock()
	})
}

// run handles hub operations
func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Info().Str("client_id", client.id).Int("total", len(h.clients)).Msg("client connected")

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				client.safeClose()
			}
			h.mu.Unlock()
			log.Info().Str("client_id", client.id).Int("total", len(h.clients)).Msg("client disconnected")

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					// Client send buffer full, disconnect
					go func(c *Client) {
						h.unregister <- c
					}(client)
				}
			}
			h.mu.RUnlock()

		case <-h.ctx.Done():
			return
		}
	}
}

// watchApprovalQueue monitors the approval queue for changes
func (h *Hub) watchApprovalQueue() {
	notifyCh := h.queue.NotifyChannel()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-notifyCh:
			h.broadcastPendingUpdate()
		case <-ticker.C:
			// Periodic refresh to catch any missed notifications
			h.broadcastPendingUpdate()
		case <-h.ctx.Done():
			return
		}
	}
}

// broadcastPendingUpdate sends current pending approvals to all clients
func (h *Hub) broadcastPendingUpdate() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pending, err := h.queue.GetPending(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("failed to get pending approvals for broadcast")
		return
	}

	msg := WSMessage{
		Type: "approval_update",
		Data: map[string]interface{}{
			"total":   len(pending),
			"pending": pending,
		},
	}

	select {
	case h.broadcast <- msg:
	case <-h.ctx.Done():
	}
}

// BroadcastApprovalDecision notifies all clients of an approval decision
func (h *Hub) BroadcastApprovalDecision(approvalID string, status string) {
	msg := WSMessage{
		Type:       "approval_update",
		ApprovalID: approvalID,
		Status:     status,
	}

	select {
	case h.broadcast <- msg:
	case <-h.ctx.Done():
	}
}

// Client methods

func (c *Client) safeClose() {
	c.closedMu.Lock()
	defer c.closedMu.Unlock()
	
	if c.closed {
		return
	}
	c.closed = true
	
	close(c.send)
	_ = c.conn.Close()
}

// readPump reads messages from the WebSocket connection
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Warn().Err(err).Str("client_id", c.id).Msg("websocket read error")
			}
			break
		}
	}
}

// writePump sends messages to the WebSocket connection
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
	}()

	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub closed the channel
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteJSON(message); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// WSHandler handles WebSocket connections
type WSHandler struct {
	hub      *Hub
	upgrader websocket.Upgrader
}

// NewWSHandler creates a WebSocket handler
func NewWSHandler(queue approval.Queue, authManager *auth.Manager) *WSHandler {
	hub := NewHub(queue, authManager)
	return &WSHandler{
		hub: hub,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Auth is handled via token validation
			},
		},
	}
}

// GetHub returns the hub (for server shutdown)
func (h *WSHandler) GetHub() *Hub {
	return h.hub
}

// HandleWebSocket handles WebSocket upgrade and client management
func (h *WSHandler) HandleWebSocket(c echo.Context) error {
	// Extract and validate token from query parameter
	token := c.QueryParam("token")
	if token == "" {
		// Fall back to Authorization header
		authHeader := c.Request().Header.Get("Authorization")
		if authHeader != "" {
			// Strip "Bearer " prefix
			if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
				token = authHeader[7:]
			}
		}
	}

	if token == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "missing authentication token")
	}

	// Validate token
	user, err := h.hub.authManager.ValidateToken(token)
	if err != nil {
		log.Warn().Err(err).Msg("websocket auth failed")
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid token")
	}

	// Upgrade connection
	conn, err := h.upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		log.Error().Err(err).Msg("websocket upgrade failed")
		return err
	}

	// Create client
	client := &Client{
		id:   user.ID + "-" + time.Now().Format("20060102150405"),
		conn: conn,
		send: make(chan WSMessage, 256),
		hub:  h.hub,
		user: user,
	}

	// Register client
	h.hub.register <- client

	// Send initial snapshot
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pending, err := h.hub.queue.GetPending(ctx)
	if err == nil {
		initialMsg := WSMessage{
			Type: "approval_update",
			Data: map[string]interface{}{
				"total":   len(pending),
				"pending": pending,
			},
		}
		client.send <- initialMsg
	}

	// Start client pumps
	go client.writePump()
	go client.readPump()

	return nil
}