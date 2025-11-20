package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/dagbolade/ai-governance-sidecar/internal/approval"
	"github.com/dagbolade/ai-governance-sidecar/internal/audit"
	"github.com/dagbolade/ai-governance-sidecar/internal/auth"
	"github.com/dagbolade/ai-governance-sidecar/internal/policy"
	"github.com/dagbolade/ai-governance-sidecar/internal/proxy"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog/log"
)

type Server struct {
	echo   *echo.Echo
	config Config
	wsHub  *Hub // WebSocket hub for graceful shutdown
}

type Config struct {
	Port            int
	ReadTimeout     int
	WriteTimeout    int
	ShutdownTimeout int
	ProxyConfig     proxy.ProxyConfig
	ApprovalTimeout time.Duration // Added for approval expiry calculation
}

func LoadConfig() Config {
	approvalTimeoutMin := getEnvInt("APPROVAL_TIMEOUT_MINUTES", 60)
	
	return Config{
		Port:            getEnvInt("PORT", 8080),
		ReadTimeout:     getEnvInt("READ_TIMEOUT", 30),
		WriteTimeout:    getEnvInt("WRITE_TIMEOUT", 30),
		ShutdownTimeout: getEnvInt("SHUTDOWN_TIMEOUT", 10),
		ApprovalTimeout: time.Duration(approvalTimeoutMin) * time.Minute,
		ProxyConfig: proxy.ProxyConfig{
			DefaultUpstream: getEnv("TOOL_UPSTREAM", "http://localhost:9000"),
			Timeout:         getEnvInt("UPSTREAM_TIMEOUT", 30),
		},
	}
}

func New(cfg Config, pol policy.Evaluator, aud audit.Store, appr approval.Queue, authManager *auth.Manager) *Server {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	s := &Server{
		echo:   e,
		config: cfg,
	}

	s.setupMiddleware()
	s.setupRoutes(pol, aud, appr, authManager)

	return s
}

func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.config.Port)
	log.Info().Int("port", s.config.Port).Msg("starting HTTP server")

	// Disable default timeouts (we handle them via context)
	s.echo.Server.ReadTimeout = 0
	s.echo.Server.WriteTimeout = 0
	s.echo.Server.IdleTimeout = 120 * time.Second

	if err := s.echo.Start(addr); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server failed: %w", err)
	}

	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	log.Info().Msg("shutting down server")

	// Shutdown WebSocket hub first
	if s.wsHub != nil {
		s.wsHub.Shutdown()
	}

	// Then shutdown HTTP server
	shutdownCtx, cancel := context.WithTimeout(ctx, time.Duration(s.config.ShutdownTimeout)*time.Second)
	defer cancel()

	if err := s.echo.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown failed: %w", err)
	}

	return nil
}

func (s *Server) setupMiddleware() {
	// Request logging
	s.echo.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogURI:     true,
		LogStatus:  true,
		LogMethod:  true,
		LogLatency: true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			log.Info().
				Str("method", v.Method).
				Str("uri", v.URI).
				Int("status", v.Status).
				Dur("latency", v.Latency).
				Msg("request")
			return nil
		},
	}))

	// Panic recovery
	s.echo.Use(middleware.Recover())

	// CORS
	s.echo.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete},
		AllowHeaders:     []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
	}))
}

func (s *Server) setupRoutes(pol policy.Evaluator, aud audit.Store, appr approval.Queue, authManager *auth.Manager) {
	// Initialize WebSocket handler with hub
	wsHandler := NewWSHandler(appr, authManager)
	s.wsHub = wsHandler.GetHub() // Store for graceful shutdown

	// Initialize handlers
	proxyHandler := proxy.NewHandler(s.config.ProxyConfig, pol, aud, appr)
	auditHandler := NewAuditHandler(aud)
	approvalHandler := NewApprovalHandler(appr, s.config.ApprovalTimeout, s.wsHub)
	authHandler := auth.NewHandler(authManager)

	// Public endpoints (no auth required)
	s.echo.GET("/health", s.handleHealth)
	s.echo.POST("/login", authHandler.Login)

	// Protected endpoints
	protected := s.echo.Group("")
	protected.Use(authManager.Middleware())

	// Auth endpoints
	protected.GET("/me", authHandler.Me)

	// Tool proxy
	protected.POST("/tool/call", proxyHandler.HandleToolCall)

	// Audit log
	protected.GET("/audit", auditHandler.GetAuditLog)

	// Approval endpoints (v1 - legacy)
	protected.GET("/pending", approvalHandler.GetPending)
	protected.POST("/approve/:id", approvalHandler.Decide)

	// Approval endpoints (v2 - UI-friendly)
	protected.GET("/approvals", approvalHandler.ListApprovals)
	protected.GET("/approvals/pending", approvalHandler.GetPendingV2)
	protected.POST("/approvals/:id/approve", approvalHandler.Approve)
	protected.POST("/approvals/:id/deny", approvalHandler.Deny)

	// WebSocket endpoint
	protected.GET("/ws", wsHandler.HandleWebSocket)

	// UI routes (placeholder)
	protected.GET("/ui", s.handleUI)
	protected.GET("/ui/*", s.handleUI)
}

func (s *Server) handleHealth(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":  "healthy",
		"version": "1.0.0",
		"uptime":  time.Since(startTime).String(),
	})
}

func (s *Server) handleUI(c echo.Context) error {
	return c.HTML(http.StatusOK, `
		<!DOCTYPE html>
		<html>
		<head>
			<title>AI Governance Sidecar</title>
			<meta charset="UTF-8">
			<meta name="viewport" content="width=device-width, initial-scale=1.0">
		</head>
		<body>
			<div id="root">
				<h1>AI Governance Sidecar</h1>
				<p>UI is being built. For now, use the API endpoints:</p>
				<ul>
					<li>POST /login - Login to get JWT token</li>
					<li>GET /me - Get current user info</li>
					<li>GET /approvals?status=pending - View pending approvals (auth required)</li>
					<li>POST /approvals/:id/approve - Approve requests (auth required)</li>
					<li>POST /approvals/:id/deny - Deny requests (auth required)</li>
					<li>GET /audit - View audit log (auth required)</li>
					<li>GET /ws?token=YOUR_JWT - WebSocket connection (auth required)</li>
				</ul>
			</div>
		</body>
		</html>
	`)
}

// Helper functions

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return fallback
}

var startTime = time.Now()