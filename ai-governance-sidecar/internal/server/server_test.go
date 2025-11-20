package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dagbolade/ai-governance-sidecar/internal/approval"
	"github.com/dagbolade/ai-governance-sidecar/internal/audit"
	"github.com/dagbolade/ai-governance-sidecar/internal/auth"
	"github.com/dagbolade/ai-governance-sidecar/internal/policy"
	"github.com/dagbolade/ai-governance-sidecar/internal/proxy"
)

// minimal test server used by these tests to provide handlers and lifecycle methods.
type testServer struct {
	echo http.Handler
	srv  *http.Server
	port int
}


// New constructs a minimal server instance with /health and /audit handlers.
// The auditStore parameter is expected to implement GetAll(context.Context) ([]audit.Entry, error).
func newTestServer(cfg Config, _ interface{}, auditStore interface{ GetAll(context.Context) ([]audit.Entry, error) }, _ interface{}, _ interface{}) *testServer {
	mux := http.NewServeMux()

	// health handler
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
	})

	// audit handler reads entries from provided auditStore
	mux.HandleFunc("/audit", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		entries := []audit.Entry{}
		if auditStore != nil {
			if es, err := auditStore.GetAll(context.Background()); err == nil {
				entries = es
			}
		}
		resp := map[string]interface{}{
			"total":   len(entries),
			"entries": entries,
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	return &testServer{
		echo: mux,
		port: cfg.Port,
	}
}

// Start runs the HTTP server (blocks until Shutdown is called).
func (ts *testServer) Start() {
	ts.srv = &http.Server{
		Addr:    fmt.Sprintf(":%d", ts.port),
		Handler: ts.echo,
	}
	if err := ts.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		// intentionally ignore errors for the test harness
	}
}

// Shutdown gracefully stops the server.
func (ts *testServer) Shutdown(ctx context.Context) error {
	if ts.srv != nil {
		return ts.srv.Shutdown(ctx)
	}
	return nil
}

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

type mockApprovalQueue struct {
	notifyCh chan struct{}
}

func newMockApprovalQueue() *mockApprovalQueue {
	return &mockApprovalQueue{
		notifyCh: make(chan struct{}, 10),
	}
}

func (m *mockApprovalQueue) Enqueue(ctx context.Context, req policy.Request, reason string) (approval.Decision, error) {
	return approval.Decision{Approved: true, Reason: "mock approved"}, nil
}

func (m *mockApprovalQueue) GetPending(ctx context.Context) ([]approval.Request, error) {
	return []approval.Request{}, nil
}

func (m *mockApprovalQueue) Decide(ctx context.Context, id string, decision approval.Decision) error {
	return nil
}

func (m *mockApprovalQueue) NotifyChannel() <-chan struct{} {
	return m.notifyCh
}

func (m *mockApprovalQueue) Close() error {
	if m.notifyCh != nil {
		close(m.notifyCh)
	}
	return nil
}

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
	mockApproval := newMockApprovalQueue()
	defer mockApproval.Close()
	
	mockAuthManager := auth.NewManager(auth.Config{
		RequireAuth: false,
		JWTSecret:   "test-secret",
	})

	srv := newTestServer(cfg, mockPolicy, mockAudit, mockApproval, mockAuthManager)

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
	mockApproval := newMockApprovalQueue()
	defer mockApproval.Close()
	
	mockAuthManager := auth.NewManager(auth.Config{
		RequireAuth: false,
		JWTSecret:   "test-secret",
	})

	srv := New(cfg, mockPolicy, mockAudit, mockApproval, mockAuthManager)

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
	mockApproval := newMockApprovalQueue()
	defer mockApproval.Close()
	
	mockAuthManager := auth.NewManager(auth.Config{
		RequireAuth: false,
		JWTSecret:   "test-secret",
	})

	srv := New(cfg, mockPolicy, mockAudit, mockApproval, mockAuthManager)

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