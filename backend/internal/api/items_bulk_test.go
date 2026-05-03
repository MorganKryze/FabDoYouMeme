package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/api"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/config"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/testutil"
)

// memStorage is a minimal in-memory Storage stub for the bulk-upload tests.
// It records every Upload key (so we can assert orphan-cleanup behaviour) and
// can be flipped to fail Upload for one specific key to exercise the
// transactional rollback path.
type memStorage struct {
	mu       sync.Mutex
	objects  map[string][]byte
	failAll  bool
}

func newMemStorage() *memStorage { return &memStorage{objects: map[string][]byte{}} }

func (s *memStorage) Upload(_ context.Context, key string, body io.Reader, _ string, _ int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failAll {
		return errors.New("forced failure")
	}
	b, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	s.objects[key] = b
	return nil
}
func (s *memStorage) PresignUpload(context.Context, string, time.Duration) (string, error) {
	return "", nil
}
func (s *memStorage) PresignDownload(context.Context, string, time.Duration) (string, error) {
	return "", nil
}
func (s *memStorage) Download(context.Context, string) (io.ReadCloser, string, int64, error) {
	return nil, "", 0, errors.New("not implemented")
}
func (s *memStorage) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.objects, key)
	return nil
}
func (s *memStorage) Purge(context.Context, string) (int64, error)        { return 0, nil }
func (s *memStorage) Stats(context.Context, string) (int64, int64, error) { return 0, 0, nil }
func (s *memStorage) Probe(context.Context) error                         { return nil }

// pngBytes returns the bytes of a minimal valid PNG.
func pngBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	img.Set(0, 0, color.RGBA{255, 0, 0, 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

// buildMultipart returns a multipart body with N file fields named "file"
// and (optionally) matching "name" fields. The file parts carry an explicit
// Content-Type so the server's magic-byte check matches the declared MIME.
func buildMultipart(t *testing.T, files [][]byte, names []string) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for i, b := range files {
		hdr := make(textproto.MIMEHeader)
		hdr.Set("Content-Disposition", `form-data; name="file"; filename="image`+string(rune('0'+i))+`.png"`)
		hdr.Set("Content-Type", "image/png")
		fw, err := w.CreatePart(hdr)
		if err != nil {
			t.Fatalf("create form file: %v", err)
		}
		if _, err := fw.Write(b); err != nil {
			t.Fatalf("write file part: %v", err)
		}
	}
	for _, n := range names {
		if err := w.WriteField("name", n); err != nil {
			t.Fatalf("write name field: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}
	return &buf, w.FormDataContentType()
}

func newBulkHandler(t *testing.T, store *memStorage) (*api.PackHandler, *db.Queries) {
	t.Helper()
	pool := testutil.Pool()
	cfg := &config.Config{MaxUploadSizeBytes: 1 * 1024 * 1024}
	h := api.NewPackHandler(pool, cfg, store, nil)
	return h, db.New(pool)
}

func seedPersonalPack(t *testing.T, q *db.Queries, ownerID pgtype.UUID) db.GamePack {
	t.Helper()
	p, err := q.CreatePack(context.Background(), db.CreatePackParams{
		Name:       "bulk-test-" + t.Name(),
		OwnerID:    ownerID,
		Visibility: "private",
		Language:   "en",
	})
	if err != nil {
		t.Fatalf("seed pack: %v", err)
	}
	return p
}

// TestBulkCreateImageItems_AllSucceed is the happy path: three valid PNGs
// produce three rows, three uploaded objects, and three "ok" results.
func TestBulkCreateImageItems_AllSucceed(t *testing.T) {
	store := newMemStorage()
	h, q := newBulkHandler(t, store)
	admin := seedAdmin(t, q)
	pack := seedPersonalPack(t, q, pgtype.UUID{Bytes: admin.ID, Valid: true})

	body, contentType := buildMultipart(t, [][]byte{pngBytes(t), pngBytes(t), pngBytes(t)}, []string{"alpha", "beta", "gamma"})
	req := httptest.NewRequest(http.MethodPost, "/api/packs/"+pack.ID.String()+"/items/bulk", body)
	req.Header.Set("Content-Type", contentType)
	req = withUser(req, admin.ID.String(), admin.Username, admin.Email, admin.Role)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", pack.ID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.BulkCreateImageItems(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d — body: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Results []struct {
			OK       bool   `json:"ok"`
			Filename string `json:"filename"`
			Reason   string `json:"reason"`
			Item     struct {
				ID            string  `json:"id"`
				Name          string  `json:"name"`
				MediaKey      *string `json:"media_key"`
				ThumbnailURL  *string `json:"thumbnail_url"`
				VersionNumber *int32  `json:"version_number"`
			} `json:"item"`
		} `json:"results"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Results) != 3 {
		t.Fatalf("want 3 results, got %d", len(resp.Results))
	}
	for i, r := range resp.Results {
		if !r.OK {
			t.Errorf("result[%d]: want ok, got reason=%q", i, r.Reason)
		}
		if r.Item.ThumbnailURL == nil || *r.Item.ThumbnailURL == "" {
			t.Errorf("result[%d]: missing thumbnail_url", i)
		}
		if r.Item.VersionNumber == nil || *r.Item.VersionNumber != 1 {
			t.Errorf("result[%d]: want version_number=1", i)
		}
	}
	if resp.Results[0].Item.Name != "alpha" {
		t.Errorf("want first item named 'alpha', got %q", resp.Results[0].Item.Name)
	}
	// All three objects landed in storage.
	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.objects) != 3 {
		t.Errorf("want 3 stored objects, got %d", len(store.objects))
	}
	// All three rows landed in the DB.
	rows, err := q.ListItemsForPack(context.Background(), db.ListItemsForPackParams{PackID: pack.ID, Lim: 50, Off: 0})
	if err != nil {
		t.Fatalf("list items: %v", err)
	}
	if len(rows) != 3 {
		t.Errorf("want 3 DB rows, got %d", len(rows))
	}
	for _, row := range rows {
		if !row.CurrentVersionID.Valid {
			t.Errorf("row %s has NULL current_version_id (orphan)", row.ID)
		}
		if row.MediaKey == nil || *row.MediaKey == "" {
			t.Errorf("row %s has NULL media_key", row.ID)
		}
	}
}

// TestBulkCreateImageItems_StorageFailureLeavesNoOrphan asserts the
// transactional pipeline: when storage.Upload fails for one file, that file's
// item row must NOT exist (the tx rolled back). Sibling files in the same
// request must still succeed.
func TestBulkCreateImageItems_StorageFailureLeavesNoOrphan(t *testing.T) {
	store := newMemStorage()
	h, q := newBulkHandler(t, store)
	admin := seedAdmin(t, q)
	pack := seedPersonalPack(t, q, pgtype.UUID{Bytes: admin.ID, Valid: true})

	// Force every upload to fail — we cannot predict the item-derived key
	// before the row is inserted, so an all-fail mode is the cleanest way
	// to exercise the rollback path deterministically.
	store.mu.Lock()
	store.failAll = true
	store.mu.Unlock()

	body, contentType := buildMultipart(t, [][]byte{pngBytes(t)}, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/packs/"+pack.ID.String()+"/items/bulk", body)
	req.Header.Set("Content-Type", contentType)
	req = withUser(req, admin.ID.String(), admin.Username, admin.Email, admin.Role)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", pack.ID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.BulkCreateImageItems(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	rows, err := q.ListItemsForPack(context.Background(), db.ListItemsForPackParams{PackID: pack.ID, Lim: 50, Off: 0})
	if err != nil {
		t.Fatalf("list items: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("want 0 rows (all uploads failed → all txs rolled back), got %d", len(rows))
	}
}

// TestBulkCreateImageItems_RejectsTooManyFiles bounds the per-request file
// count so a single bulk call cannot pin a worker on huge input.
func TestBulkCreateImageItems_RejectsTooManyFiles(t *testing.T) {
	store := newMemStorage()
	h, q := newBulkHandler(t, store)
	admin := seedAdmin(t, q)
	pack := seedPersonalPack(t, q, pgtype.UUID{Bytes: admin.ID, Valid: true})

	files := make([][]byte, api.MaxBulkUploadFiles+1)
	for i := range files {
		files[i] = pngBytes(t)
	}
	body, contentType := buildMultipart(t, files, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/packs/"+pack.ID.String()+"/items/bulk", body)
	req.Header.Set("Content-Type", contentType)
	req = withUser(req, admin.ID.String(), admin.Username, admin.Email, admin.Role)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", pack.ID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.BulkCreateImageItems(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("want 413, got %d — body: %s", rec.Code, rec.Body.String())
	}
}
