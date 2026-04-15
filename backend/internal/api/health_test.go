package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/MorganKryze/FabDoYouMeme/backend/internal/api"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/testutil"
)

// stubStorage is a no-op Storage stub for health checks.
type stubStorage struct{}

func (s *stubStorage) PresignUpload(_ context.Context, _ string, _ time.Duration) (string, error) {
	return "", nil
}
func (s *stubStorage) PresignDownload(_ context.Context, _ string, _ time.Duration) (string, error) {
	return "", nil
}
func (s *stubStorage) Upload(_ context.Context, _ string, _ io.Reader, _ string, _ int64) error {
	return nil
}
func (s *stubStorage) Download(_ context.Context, _ string) (io.ReadCloser, string, int64, error) {
	return io.NopCloser(&bytes.Buffer{}), "", 0, nil
}
func (s *stubStorage) Delete(_ context.Context, _ string) error         { return nil }
func (s *stubStorage) Purge(_ context.Context, _ string) (int64, error) { return 0, nil }
func (s *stubStorage) Stats(_ context.Context, _ string) (int64, int64, error) {
	return 0, 0, nil
}
func (s *stubStorage) Probe(_ context.Context) error { return nil }

func newHealthHandler() *api.HealthHandler {
	return api.NewHealthHandler(testutil.Pool(), &stubStorage{}, nil)
}

func newHealthHandlerWithSMTP(probe func(context.Context) error) *api.HealthHandler {
	return api.NewHealthHandler(testutil.Pool(), &stubStorage{}, probe)
}

func TestHealth_Liveness(t *testing.T) {
	h := newHealthHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()
	h.Liveness(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("want status=ok, got %s", resp["status"])
	}
}

// decodeDeep parses a deep-health response into its structured shape.
func decodeDeep(t *testing.T, body []byte) (status string, checks map[string]map[string]any) {
	t.Helper()
	var resp map[string]any
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	status, _ = resp["status"].(string)
	raw, _ := resp["checks"].(map[string]any)
	checks = map[string]map[string]any{}
	for k, v := range raw {
		if m, ok := v.(map[string]any); ok {
			checks[k] = m
		}
	}
	return status, checks
}

func TestHealth_Readiness_DBReachable_SMTPSkipped(t *testing.T) {
	h := newHealthHandler() // no email probe → "skipped"
	req := httptest.NewRequest(http.MethodGet, "/api/health/deep", nil)
	rec := httptest.NewRecorder()
	h.Readiness(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("want 200 when DB+storage reachable, got %d — body: %s", rec.Code, rec.Body.String())
	}
	status, checks := decodeDeep(t, rec.Body.Bytes())
	if status != "ok" {
		t.Errorf("want top-level status=ok, got %q", status)
	}
	if pg := checks["postgres"]; pg["status"] != "ok" {
		t.Errorf("want postgres.status=ok, got %v", pg["status"])
	}
	if rf := checks["rustfs"]; rf["status"] != "ok" {
		t.Errorf("want rustfs.status=ok, got %v", rf["status"])
	}
	if smtp := checks["smtp"]; smtp["status"] != "skipped" {
		t.Errorf("want smtp.status=skipped (no probe configured), got %v", smtp["status"])
	}
}

func TestHealth_Readiness_SMTPDegraded(t *testing.T) {
	longMsg := "connection refused: " // base prefix
	for i := 0; i < 50; i++ {
		longMsg += "x"
	}
	probe := func(_ context.Context) error { return errors.New(longMsg) }

	h := newHealthHandlerWithSMTP(probe)
	req := httptest.NewRequest(http.MethodGet, "/api/health/deep", nil)
	rec := httptest.NewRecorder()
	h.Readiness(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("want 503 when smtp degraded, got %d — body: %s", rec.Code, rec.Body.String())
	}
	status, checks := decodeDeep(t, rec.Body.Bytes())
	if status != "degraded" {
		t.Errorf("want top-level status=degraded, got %q", status)
	}
	smtp := checks["smtp"]
	if smtp["status"] != "degraded" {
		t.Errorf("want smtp.status=degraded, got %v", smtp["status"])
	}
	errStr, _ := smtp["error"].(string)
	if errStr == "" {
		t.Error("want smtp.error to be non-empty")
	}
	// truncate(s, 120) appends "…" (3 bytes in UTF-8) so the max is 123 bytes.
	if len(errStr) > 123 {
		t.Errorf("smtp.error exceeds 120+1 char budget: len=%d", len(errStr))
	}
}
