package main

import (
	"context"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/dagbolade/ai-governance-sidecar/internal/approval"
	"github.com/dagbolade/ai-governance-sidecar/internal/audit"
	"github.com/dagbolade/ai-governance-sidecar/internal/auth"
	"github.com/dagbolade/ai-governance-sidecar/internal/policy"
	"github.com/dagbolade/ai-governance-sidecar/internal/server"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	setupLogger()

	log.Info().Msg("starting AI Governance Sidecar")

	ctx, cancel := setupSignalHandler()
	defer cancel()

	if err := run(ctx); err != nil {
		log.Fatal().Err(err).Msg("application error")
	}

	log.Info().Msg("sidecar stopped successfully")
}

func run(ctx context.Context) error {
	auditStore, err := initAuditStore()
	if err != nil {
		return err
	}
	defer func() {
		if err := auditStore.Close(); err != nil {
			log.Warn().Err(err).Msg("failed to close audit store")
		}
	}()

	policyEngine, err := initPolicyEngine()
	if err != nil {
		return err
	}
	defer func() {
		if err := policyEngine.Close(); err != nil {
			log.Warn().Err(err).Msg("failed to close policy engine")
		}
	}()

	approvalQueue := initApprovalQueue()
	defer func() {
		if err := approvalQueue.Close(); err != nil {
			log.Warn().Err(err).Msg("failed to close approval queue")
		}
	}()

	authManager := initAuthManager()

	cfg := server.LoadConfig()
	srv := server.New(cfg, policyEngine, auditStore, approvalQueue, authManager)

	return runServer(ctx, srv)
}

// Initialize auth manager
func initAuthManager() *auth.Manager {
	requireAuth := getEnv("REQUIRE_AUTH", "false") == "true"
	
	log.Info().Bool("required", requireAuth).Msg("initializing auth manager")
	
	manager := auth.NewManager(auth.Config{
		JWTSecret:       os.Getenv("JWT_SECRET"),
		TokenExpiration: 24 * time.Hour,
		RequireAuth:     requireAuth,
	})
	
	log.Info().Msg("auth manager initialized")
	return manager
}

func setupLogger() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})

	level, err := zerolog.ParseLevel(getEnv("LOG_LEVEL", "info"))
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)
}

func setupSignalHandler() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)

	go func() {
		sig := <-sigChan
		log.Info().Str("signal", sig.String()).Msg("shutdown signal received")
		cancel()
	}()

	return ctx, cancel
}

func initAuditStore() (audit.Store, error) {
	dbPath := getEnv("DB_PATH", "./db/audit.db")
	
	log.Info().Str("path", dbPath).Msg("initializing audit store")
	
	store, err := audit.NewSQLiteStore(dbPath)
	if err != nil {
		return nil, err
	}

	log.Info().Msg("audit store initialized")
	return store, nil
}

func initPolicyEngine() (policy.Evaluator, error) {
	policyDir := getEnv("POLICY_DIR", "./policies")
	
	log.Info().Str("dir", policyDir).Msg("initializing policy engine")
	
	engine, err := policy.NewEngine(policyDir)
	if err != nil {
		return nil, err
	}

	log.Info().Msg("policy engine initialized")
	return engine, nil
}

func initApprovalQueue() approval.Queue {
	timeoutSec := getEnvInt("APPROVAL_TIMEOUT", 300)
	timeout := time.Duration(timeoutSec) * time.Second
	
	log.Info().Dur("timeout", timeout).Msg("initializing approval queue")
	
	queue := approval.NewInMemoryQueue(timeout)
	
	log.Info().Msg("approval queue initialized")
	return queue
}

func runServer(ctx context.Context, srv *server.Server) error {
	errChan := make(chan error, 1)

	go func() {
		if err := srv.Start(); err != nil {
			errChan <- err
		}
	}()

	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		return srv.Shutdown(context.Background())
	}
}

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