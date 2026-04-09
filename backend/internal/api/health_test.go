package api_test

import (
	"context"
	"encoding/json"
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
func (s *stubStorage) Delete(_ context.Context, _ string) error { return nil }
func (s *stubStorage) Probe(_ context.Context) error            { return nil }

func newHealthHandler() *api.HealthHandler {
	return api.NewHealthHandler(testutil.Pool(), &stubStorage{})
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

func TestHealth_Readiness_DBReachable(t *testing.T) {
	h := newHealthHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/health/deep", nil)
	rec := httptest.NewRecorder()
	h.Readiness(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("want 200 when DB is reachable, got %d — body: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	checks, _ := resp["checks"].(map[string]any)
	if checks["postgres"] != "ok" {
		t.Errorf("want postgres=ok, got %v", checks["postgres"])
	}
}
