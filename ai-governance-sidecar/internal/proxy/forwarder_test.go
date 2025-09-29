package proxy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestForwarder_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("expected Content-Type: application/json")
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"result":"ok"}`))
	}))
	defer server.Close()

	forwarder := NewForwarder(10)
	req := &ToolCallRequest{
		ToolName: "test",
		Args:     json.RawMessage(`{"key":"value"}`),
	}

	result, err := forwarder.Forward(context.Background(), server.URL, req)
	if err != nil {
		t.Fatalf("forward failed: %v", err)
	}

	var data map[string]string
	if err := json.Unmarshal(result, &data); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	if data["result"] != "ok" {
		t.Errorf("unexpected result: %v", data)
	}
}

func TestForwarder_UpstreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	forwarder := NewForwarder(10)
	req := &ToolCallRequest{ToolName: "test", Args: json.RawMessage(`{}`)}

	_, err := forwarder.Forward(context.Background(), server.URL, req)
	if err == nil {
		t.Error("expected error from upstream failure")
	}
}

func TestForwarder_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	forwarder := NewForwarder(1) // 1 second timeout
	req := &ToolCallRequest{ToolName: "test", Args: json.RawMessage(`{}`)}

	_, err := forwarder.Forward(context.Background(), server.URL, req)
	if err == nil {
		t.Error("expected timeout error")
	}
}