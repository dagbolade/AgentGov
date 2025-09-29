package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

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

func New(cfg Config, pol policy.Evaluator, aud audit.Store) *Server {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	s := &Server{
		echo:   e,
		config: cfg,
	}

	s.setupMiddleware()
	s.setupRoutes(pol, aud)

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
		LogURI:    true,
		LogStatus: true,
		LogMethod: true,
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

func (s *Server) setupRoutes(pol policy.Evaluator, aud audit.Store) {
	proxyHandler := proxy.NewHandler(s.config.ProxyConfig, pol, aud)
	auditHandler := NewAuditHandler(aud)

	s.echo.GET("/health", s.handleHealth)
	s.echo.POST("/tool/call", proxyHandler.HandleToolCall)
	s.echo.GET("/audit", auditHandler.GetAuditLog)

	// TODO: Approval routes will be added in Phase 2
	// s.echo.GET("/pending", approvalHandler.GetPending)
	// s.echo.POST("/approve/:id", approvalHandler.Approve)

	// TODO: UI routes will be added in Phase 2
	// s.echo.GET("/ui", uiHandler.Serve)
	// s.echo.GET("/ui/*", uiHandler.ServeStatic)
}

func (s *Server) handleHealth(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{
		"status": "healthy",
	})
}