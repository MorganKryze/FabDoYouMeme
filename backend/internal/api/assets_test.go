package api_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MorganKryze/FabDoYouMeme/backend/internal/api"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/config"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/testutil"
)

func newAssetHandler(t *testing.T) *api.AssetHandler {
	t.Helper()
	cfg := &config.Config{MaxUploadSizeBytes: 1024 * 1024} // 1 MB limit for tests
	return api.NewAssetHandler(testutil.Pool(), cfg, nil)  // nil storage — only testing validation
}

// pngBase64 returns a minimal valid PNG as a base64 string.
func pngBase64(t *testing.T) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{255, 0, 0, 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func uploadURLRequest(t *testing.T, mime string, sizeBytes int64, previewBase64 string) *http.Request {
	t.Helper()
	body := map[string]any{
		"pack_id":       "00000000-0000-0000-0000-000000000001",
		"item_id":       "00000000-0000-0000-0000-000000000002",
		"version_number": 1,
		"filename":       "test.png",
		"mime_type":      mime,
		"size_bytes":     sizeBytes,
		"preview_bytes":  previewBase64,
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/assets/upload-url", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req = withUser(req, "00000000-0000-0000-0000-000000000003", "assetuser", "asset@t.com", "player")
	return req
}

func TestUploadURL_InvalidMIME_NoPreview(t *testing.T) {
	// Allowlist-only check: disallowed MIME without preview bytes → 422 invalid_mime_type
	h := newAssetHandler(t)
	req := uploadURLRequest(t, "application/pdf", 512, "")
	rec := httptest.NewRecorder()
	h.UploadURL(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("want 422, got %d", rec.Code)
	}
	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "invalid_mime_type" {
		t.Errorf("want invalid_mime_type, got %s", resp["code"])
	}
}

func TestUploadURL_MagicBytesMismatch(t *testing.T) {
	// PNG bytes declared as image/jpeg → ValidateMIME returns error → 422 invalid_mime_type
	h := newAssetHandler(t)
	req := uploadURLRequest(t, "image/jpeg", 512, pngBase64(t))
	rec := httptest.NewRecorder()
	h.UploadURL(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("want 422 for MIME mismatch, got %d", rec.Code)
	}
	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "invalid_mime_type" {
		t.Errorf("want invalid_mime_type, got %s", resp["code"])
	}
}

func TestUploadURL_TooLarge(t *testing.T) {
	// File size exceeds MaxUploadSizeBytes → 422 file_too_large
	h := newAssetHandler(t)
	req := uploadURLRequest(t, "image/png", 2*1024*1024, "") // 2 MB > 1 MB limit
	rec := httptest.NewRecorder()
	h.UploadURL(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("want 422, got %d", rec.Code)
	}
	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "file_too_large" {
		t.Errorf("want file_too_large, got %s", resp["code"])
	}
}

func TestUploadURL_Unauthenticated(t *testing.T) {
	h := newAssetHandler(t)
	body := `{"mime_type":"image/png","size_bytes":100}`
	req := httptest.NewRequest(http.MethodPost, "/api/assets/upload-url", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	h.UploadURL(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rec.Code)
	}
}
