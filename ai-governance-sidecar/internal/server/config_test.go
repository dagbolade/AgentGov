package server

import (
	"os"
	"testing"
)

func TestGetEnv(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		fallback string
		setValue string
		expected string
	}{
		{
			name:     "uses env value",
			key:      "TEST_VAR",
			fallback: "default",
			setValue: "custom",
			expected: "custom",
		},
		{
			name:     "uses fallback",
			key:      "MISSING_VAR",
			fallback: "default",
			setValue: "",
			expected: "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setValue != "" {
				os.Setenv(tt.key, tt.setValue)
				defer os.Unsetenv(tt.key)
			}

			result := getEnv(tt.key, tt.fallback)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestGetEnvInt(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		fallback int
		setValue string
		expected int
	}{
		{
			name:     "parses int",
			key:      "TEST_INT",
			fallback: 100,
			setValue: "200",
			expected: 200,
		},
		{
			name:     "uses fallback on invalid",
			key:      "TEST_INT",
			fallback: 100,
			setValue: "invalid",
			expected: 100,
		},
		{
			name:     "uses fallback when missing",
			key:      "MISSING_INT",
			fallback: 100,
			setValue: "",
			expected: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setValue != "" {
				os.Setenv(tt.key, tt.setValue)
				defer os.Unsetenv(tt.key)
			} else {
				os.Unsetenv(tt.key)
			}

			result := getEnvInt(tt.key, tt.fallback)
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	os.Setenv("PORT", "9090")
	os.Setenv("TOOL_UPSTREAM", "http://custom:8000")
	defer func() {
		os.Unsetenv("PORT")
		os.Unsetenv("TOOL_UPSTREAM")
	}()

	cfg := LoadConfig()

	if cfg.Port != 9090 {
		t.Errorf("expected port 9090, got %d", cfg.Port)
	}

	if cfg.ProxyConfig.DefaultUpstream != "http://custom:8000" {
		t.Errorf("expected custom upstream, got %s", cfg.ProxyConfig.DefaultUpstream)
	}

	if cfg.ReadTimeout != 30 {
		t.Errorf("expected default read timeout 30, got %d", cfg.ReadTimeout)
	}
}