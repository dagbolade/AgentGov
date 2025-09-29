package server

import (
	"os"
	"strconv"

	"github.com/dagbolade/ai-governance-sidecar/internal/proxy"
)

func LoadConfig() Config {
	return Config{
		Port:            getEnvInt("PORT", 8080),
		ReadTimeout:     getEnvInt("READ_TIMEOUT", 30),
		WriteTimeout:    getEnvInt("WRITE_TIMEOUT", 30),
		ShutdownTimeout: getEnvInt("SHUTDOWN_TIMEOUT", 10),
		ProxyConfig: proxy.ProxyConfig{
			DefaultUpstream: getEnv("TOOL_UPSTREAM", "http://localhost:9000"),
			Timeout:         getEnvInt("UPSTREAM_TIMEOUT", 30),
		},
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