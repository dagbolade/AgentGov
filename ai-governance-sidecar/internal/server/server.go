package server

import (
	"context"
	"embed"
	"fmt"
	"net/http"
	"time"

	"github.com/dagbolade/ai-governance-sidecar/internal/approval"
	"github.com/dagbolade/ai-governance-sidecar/internal/audit"
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

func New(cfg Config, pol policy.Evaluator, aud audit.Store, appr approval.Queue, assets embed.FS) *Server {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	s := &Server{
		echo:   e,
		config: cfg,
	}

	s.setupMiddleware()
	s.setupRoutes(pol, aud, appr, assets)

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
	}))
}

func (s *Server) setupRoutes(pol policy.Evaluator, aud audit.Store, appr approval.Queue, assets embed.FS) {
	proxyHandler := proxy.NewHandler(s.config.ProxyConfig, pol, aud, appr)
	auditHandler := NewAuditHandler(aud)
	approvalHandler := NewApprovalHandler(appr)
	wsHandler := NewWSHandler(appr)
	uiHandler := NewUIHandler(assets)

	// Core endpoints
	s.echo.GET("/health", s.handleHealth)
	s.echo.POST("/tool/call", proxyHandler.HandleToolCall)
	s.echo.GET("/audit", auditHandler.GetAuditLog)

	// Approval endpoints
	s.echo.GET("/pending", approvalHandler.GetPending)
	s.echo.POST("/approve/:id", approvalHandler.Decide)

	// WebSocket for real-time updates
	s.echo.GET("/ws", wsHandler.HandleWebSocket)

	// Serve static assets for UI
	s.echo.GET("/ui/assets/*", uiHandler.ServeAsset)

	// UI routes - serve React SPA
	s.echo.GET("/", uiHandler.ServeUI)
	s.echo.GET("/ui", uiHandler.ServeUI)
	s.echo.GET("/ui/*", uiHandler.ServeUI)
}

func (s *Server) handleHealth(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{
		"status": "healthy",
	})
}

