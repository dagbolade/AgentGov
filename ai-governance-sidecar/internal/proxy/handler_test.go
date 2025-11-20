package proxy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dagbolade/ai-governance-sidecar/internal/approval"
	"github.com/dagbolade/ai-governance-sidecar/internal/audit"
	"github.com/dagbolade/ai-governance-sidecar/internal/policy"
	"github.com/labstack/echo/v4"
)

type mockPolicyEvaluator struct {
	response policy.Response
	err      error
}

func (m *mockPolicyEvaluator) Evaluate(ctx context.Context, req policy.Request) (policy.Response, error) {
	return m.response, m.err
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

func TestHandleToolCall_Success(t *testing.T) {
	mockPolicy := &mockPolicyEvaluator{
		response: policy.Response{Allow: true, Reason: "approved"},
	}
	mockAudit := &mockAuditStore{}
	mockApproval := newMockApprovalQueue()
	defer mockApproval.Close()

	// Mock upstream server
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"success"}`))
	}))
	defer upstream.Close()

	config := ProxyConfig{
		DefaultUpstream: upstream.URL,
		Timeout:         10,
	}

	handler := NewHandler(config, mockPolicy, mockAudit, mockApproval)

	e := echo.New()
	reqBody := `{"tool_name":"test_tool","args":{"key":"value"}}`
	req := httptest.NewRequest(http.MethodPost, "/tool/call", strings.NewReader(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.HandleToolCall(c); err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp ToolCallResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if !resp.Success {
		t.Error("expected success response")
	}

	// Verify audit log
	if len(mockAudit.entries) != 1 {
		t.Errorf("expected 1 audit entry, got %d", len(mockAudit.entries))
	}
}

func TestHandleToolCall_Denied(t *testing.T) {
	mockPolicy := &mockPolicyEvaluator{
		response: policy.Response{Allow: false, Reason: "blocked by policy"},
	}
	mockAudit := &mockAuditStore{}
	mockApproval := newMockApprovalQueue()
	defer mockApproval.Close()

	config := ProxyConfig{
		DefaultUpstream: "http://localhost:9000",
		Timeout:         10,
	}

	handler := NewHandler(config, mockPolicy, mockAudit, mockApproval)

	e := echo.New()
	reqBody := `{"tool_name":"blocked_tool","args":{}}`
	req := httptest.NewRequest(http.MethodPost, "/tool/call", strings.NewReader(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.HandleToolCall(c); err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", rec.Code)
	}

	var resp ToolCallResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)

	if resp.Success {
		t.Error("expected failed response")
	}

	if resp.Error != "blocked by policy" {
		t.Errorf("unexpected error message: %s", resp.Error)
	}
}

func TestHandleToolCall_InvalidRequest(t *testing.T) {
	mockPolicy := &mockPolicyEvaluator{}
	mockAudit := &mockAuditStore{}
	mockApproval := newMockApprovalQueue()
	defer mockApproval.Close()

	config := ProxyConfig{DefaultUpstream: "http://localhost:9000", Timeout: 10}
	handler := NewHandler(config, mockPolicy, mockAudit, mockApproval)

	e := echo.New()
	reqBody := `{"args":{}}`
	req := httptest.NewRequest(http.MethodPost, "/tool/call", strings.NewReader(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.HandleToolCall(c); err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
}

func TestParseRequest(t *testing.T) {
	handler := &Handler{
		config: ProxyConfig{DefaultUpstream: "http://default:9000"},
	}

	tests := []struct {
		name        string
		body        string
		expectError bool
		expectValue string
	}{
		{
			name:        "valid request",
			body:        `{"tool_name":"test","args":{}}`,
			expectError: false,
			expectValue: "test",
		},
		{
			name:        "missing tool_name",
			body:        `{"args":{}}`,
			expectError: true,
		},
		{
			name:        "uses default upstream",
			body:        `{"tool_name":"test","args":{}}`,
			expectError: false,
			expectValue: "http://default:9000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tt.body))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			result, err := handler.parseRequest(c)

			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if !tt.expectError && tt.name == "uses default upstream" {
				if result.Upstream != tt.expectValue {
					t.Errorf("expected upstream %s, got %s", tt.expectValue, result.Upstream)
				}
			}
		})
	}
}