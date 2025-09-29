package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Forwarder struct {
	client *http.Client
}

func NewForwarder(timeoutSec int) *Forwarder {
	return &Forwarder{
		client: &http.Client{
			Timeout: time.Duration(timeoutSec) * time.Second,
		},
	}
}

func (f *Forwarder) Forward(ctx context.Context, upstream string, req *ToolCallRequest) (json.RawMessage, error) {
	payload, err := f.buildPayload(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := f.buildRequest(ctx, upstream, payload)
	if err != nil {
		return nil, err
	}

	resp, err := f.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upstream returned %d", resp.StatusCode)
	}

	return f.readResponse(resp.Body)
}

func (f *Forwarder) buildPayload(req *ToolCallRequest) ([]byte, error) {
	payload := map[string]interface{}{
		"tool_name": req.ToolName,
		"args":      json.RawMessage(req.Args),
	}

	return json.Marshal(payload)
}

func (f *Forwarder) buildRequest(ctx context.Context, upstream string, payload []byte) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, upstream, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	return req, nil
}

func (f *Forwarder) readResponse(body io.Reader) (json.RawMessage, error) {
	data, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	return json.RawMessage(data), nil
}