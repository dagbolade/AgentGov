package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dagbolade/ai-governance-sidecar/internal/approval"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

// ------- fake queue for tests -------

type fakeQueue struct {
	pending []approval.Request
	decided []struct {
		id       string
		decision approval.Decision
	}
}

func (f *fakeQueue) Enqueue(_ interface{}, _ approval.Request, _ string) (approval.Decision, error) {
	return approval.Decision{}, nil
}
func (f *fakeQueue) GetPending(_ interface{}) ([]approval.Request, error) {
	// echo.Context passes request.Context(), we don't need it; keep signature with any type to avoid importing context in test
	return append([]approval.Request(nil), f.pending...), nil
}
func (f *fakeQueue) Decide(_ interface{}, id string, d approval.Decision) error {
	f.decided = append(f.decided, struct {
		id       string
		decision approval.Decision
	}{id: id, decision: d})
	return nil
}
func (f *fakeQueue) Close() error { return nil }

// ------- tests -------

func TestGetPendingV2(t *testing.T) {
	e := echo.New()
	fq := &fakeQueue{
		pending: []approval.Request{
			{
				ID:        "abc-123",
				ToolName:  "database",
				Reason:    "Bulk delete requires approval",
				CreatedAt: time.Unix(1_700_000_000, 0),
				Status:    approval.StatusPending,
			},
		},
	}
	h := &ApprovalHandler{queue: fq, approvalTimeout: 30 * time.Minute}

	req := httptest.NewRequest(http.MethodGet, "/approvals/pending", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	require.NoError(t, h.GetPendingV2(c))
	require.Equal(t, http.StatusOK, rec.Code)

	var body struct {
		Total     int `json:"total"`
		Approvals []struct {
			ApprovalID string `json:"approval_id"`
			CreatedAt  string `json:"created_at"`
		} `json:"approvals"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, 1, body.Total)
	require.Equal(t, "abc-123", body.Approvals[0].ApprovalID)
}

func TestApproveAndDeny(t *testing.T) {
	e := echo.New()
	fq := &fakeQueue{}
	h := &ApprovalHandler{queue: fq}

	// approve
	{
		payload := []byte(`{"approver":"Admin","comment":"looks good"}`)
		req := httptest.NewRequest(http.MethodPost, "/approvals/abc-123/approve", bytes.NewReader(payload))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues("abc-123")

		require.NoError(t, h.Approve(c))
		require.Equal(t, http.StatusOK, rec.Code)
		require.Len(t, fq.decided, 1)
		require.Equal(t, "abc-123", fq.decided[0].id)
		require.True(t, fq.decided[0].decision.Approved)
		require.Equal(t, "looks good", fq.decided[0].decision.Reason)
		require.Equal(t, "Admin", fq.decided[0].decision.DecidedBy)
	}

	// deny
	{
		payload := []byte(`{"approver":"Admin","comment":"unsafe"}`)
		req := httptest.NewRequest(http.MethodPost, "/approvals/xyz/deny", bytes.NewReader(payload))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues("xyz")

		require.NoError(t, h.Deny(c))
		require.Equal(t, http.StatusOK, rec.Code)
		require.Len(t, fq.decided, 2)
		require.Equal(t, "xyz", fq.decided[1].id)
		require.False(t, fq.decided[1].decision.Approved)
		require.Equal(t, "unsafe", fq.decided[1].decision.Reason)
		require.Equal(t, "Admin", fq.decided[1].decision.DecidedBy)
	}
}
