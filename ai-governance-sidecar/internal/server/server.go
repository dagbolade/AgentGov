package server

import (
	"context"
	"fmt"
	"net/http"
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
}

type Config struct {
	Port            int
	ReadTimeout     int
	WriteTimeout    int
	ShutdownTimeout int
	ProxyConfig     proxy.ProxyConfig
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

	s.echo.Server.ReadTimeout = time.Duration(s.config.ReadTimeout) * time.Second
	s.echo.Server.WriteTimeout = time.Duration(s.config.WriteTimeout) * time.Second

	if err := s.echo.Start(addr); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server failed: %w", err)
	}

	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	log.Info().Msg("shutting down server")

	shutdownCtx, cancel := context.WithTimeout(ctx, time.Duration(s.config.ShutdownTimeout)*time.Second)
	defer cancel()

	if err := s.echo.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown failed: %w", err)
	}

	return nil
}

func (s *Server) setupMiddleware() {
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

	s.echo.Use(middleware.Recover())

	s.echo.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete},
		AllowHeaders: []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
	}))
}

func (s *Server) setupRoutes(pol policy.Evaluator, aud audit.Store, appr approval.Queue, authManager *auth.Manager) {
	proxyHandler := proxy.NewHandler(s.config.ProxyConfig, pol, aud, appr)
	auditHandler := NewAuditHandler(aud)
	approvalHandler := NewApprovalHandler(appr)
	wsHandler := NewWSHandler(appr)
	authHandler := auth.NewHandler(authManager)

	// Public endpoints (no auth required)
	s.echo.GET("/health", s.handleHealth)
	s.echo.POST("/login", authHandler.Login) 

	// Apply auth middleware to protected routes
	protected := s.echo.Group("")
	protected.Use(authManager.Middleware())
	
	// Protected endpoints
	protected.GET("/me", authHandler.Me)
	protected.POST("/tool/call", proxyHandler.HandleToolCall)
	protected.GET("/audit", auditHandler.GetAuditLog)
	protected.GET("/pending", approvalHandler.GetPending)
	protected.POST("/approve/:id", approvalHandler.Decide)
	protected.GET("/ws", wsHandler.HandleWebSocket)
	
	// UI routes
	protected.GET("/ui", s.handleUI)
	protected.GET("/ui/*", s.handleUI)
}

func (s *Server) handleHealth(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{
		"status": "healthy",
	})
}

func (s *Server) handleUI(c echo.Context) error {
	// TODO: Serve embedded React UI
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
					<li>GET /pending - View pending approvals (auth required)</li>
					<li>POST /approve/:id - Approve/deny requests (auth required)</li>
					<li>GET /audit - View audit log (auth required)</li>
				</ul>
			</div>
		</body>
		</html>
	`)
}