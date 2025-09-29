package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dagbolade/ai-governance-sidecar/internal/audit"
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
	defer closeResource("audit store", auditStore.Close)

	policyEngine, err := initPolicyEngine()
	if err != nil {
		return err
	}
	defer closeResource("policy engine", policyEngine.Close)

	cfg := server.LoadConfig()
	srv := server.New(cfg, policyEngine, auditStore)

	return runServer(ctx, srv)
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

func closeResource(name string, closeFn func() error) {
	if err := closeFn(); err != nil {
		log.Warn().Err(err).Str("resource", name).Msg("failed to close resource")
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}