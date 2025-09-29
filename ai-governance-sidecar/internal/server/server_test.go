package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dagbolade/ai-governance-sidecar/internal/audit"
	"github.com/dagbolade/ai-governance-sidecar/internal/policy"
	"github.com/dagbolade/ai-governance-sidecar/internal/proxy"
)

type mockPolicyEvaluator struct{}

func (m *mockPolicyEvaluator) Evaluate(ctx context.Context, req policy.Request) (policy.Response, error) {
	return policy.Response{Allow: true, Reason: "test"}, nil
}

func (m *mockPolicyEvaluator) Reload() error { return nil }
func (m *mockPolicyEvaluator) Close() error  { return nil }

type mockAuditStore struct {
	entries []audit.Entry
}

func (m *mockAuditStore) Log(ctx context.Context, toolInput json.RawMessage, decision audit.Decision, reason string) error {
	m.entries = append(m.entries, audit.Entry{
		ToolInput: toolInput,
		Decision:  decision,
		Reason:    reason,
	})
	return nil
}

func (m *mockAuditStore) GetAll(ctx context.Context) ([]audit.Entry, error) {
	return m.entries, nil
}

func (m *mockAuditStore) Close() error { return nil }

func TestHealthEndpoint(t *testing.T) {
	cfg := Config{
		Port:         8080,
		ReadTimeout:  30,
		WriteTimeout: 30,
		ProxyConfig: proxy.ProxyConfig{
			DefaultUpstream: "http://localhost:9000",
			Timeout:         30,
		},
	}

	mockPolicy := &mockPolicyEvaluator{}
	mockAudit := &mockAuditStore{}

	srv := New(cfg, mockPolicy, mockAudit)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	srv.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var response map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response["status"] != "healthy" {
		t.Errorf("expected status 'healthy', got '%s'", response["status"])
	}
}

func TestAuditEndpoint(t *testing.T) {
	cfg := Config{
		Port: 8080,
		ProxyConfig: proxy.ProxyConfig{
			DefaultUpstream: "http://localhost:9000",
			Timeout:         30,
		},
	}

	mockPolicy := &mockPolicyEvaluator{}
	mockAudit := &mockAuditStore{
		entries: []audit.Entry{
			{
				ID:        1,
				Timestamp: time.Now(),
				ToolInput: json.RawMessage(`{"tool":"test"}`),
				Decision:  audit.DecisionAllow,
				Reason:    "test",
			},
		},
	}

	srv := New(cfg, mockPolicy, mockAudit)

	req := httptest.NewRequest(http.MethodGet, "/audit", nil)
	rec := httptest.NewRecorder()

	srv.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	total := int(response["total"].(float64))
	if total != 1 {
		t.Errorf("expected 1 entry, got %d", total)
	}
}

func TestServerShutdown(t *testing.T) {
	cfg := Config{
		Port:            8888,
		ShutdownTimeout: 2,
		ProxyConfig: proxy.ProxyConfig{
			DefaultUpstream: "http://localhost:9000",
			Timeout:         30,
		},
	}

	mockPolicy := &mockPolicyEvaluator{}
	mockAudit := &mockAuditStore{}

	srv := New(cfg, mockPolicy, mockAudit)

	go func() {
		srv.Start()
	}()

	time.Sleep(100 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		t.Errorf("shutdown failed: %v", err)
	}
}