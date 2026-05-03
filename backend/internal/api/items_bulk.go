// backend/internal/api/items_bulk.go
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/middleware"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/storage"
)

// MaxBulkUploadFiles caps the number of files per bulk request. The frontend
// chunks larger imports into multiple requests so an 83-image import becomes
// roughly four HTTP calls (and four rate-limit tokens) rather than 332. The
// cap also bounds parser memory: at MaxUploadSizeBytes per file, peak
// per-request memory is ≤ MaxBulkUploadFiles × MaxUploadSizeBytes.
const MaxBulkUploadFiles = 25

// bulkUploadResult is the per-file shape returned by BulkCreateImageItems.
// Order matches the order of file fields in the multipart request so the
// client can correlate failures with its input list without filename
// matching (filenames are not unique within a batch).
type bulkUploadResult struct {
	OK       bool          `json:"ok"`
	Filename string        `json:"filename"`
	Item     *enrichedItem `json:"item,omitempty"`
	Reason   string        `json:"reason,omitempty"`
	Code     string        `json:"code,omitempty"`
}

// BulkCreateImageItems handles POST /api/packs/{id}/items/bulk.
//
// Why a single endpoint exists: the per-image flow used to do four sequential
// HTTP round-trips (createItem → upload → createVersion → promote). For an
// 83-image bulk import that's 332 requests, which saturates the per-user
// global limiter (default 100/min burst) within seconds and starves the rest
// of the page. Worse, when a step fails mid-chain the client-side cleanup
// DELETE is itself rate-limited, so the failure leaves an orphan item row
// with NULL current_version_id that renders without a thumbnail. This
// endpoint collapses the four steps into one server-side transaction per
// file and makes a whole batch cost one rate-limit token.
//
// Request: multipart/form-data
//
//	file        — the image bytes (repeat once per item; up to MaxBulkUploadFiles)
//	name        — optional display name (repeat once per item, same order as `file`).
//	              Empty / missing entries fall back to the filename without extension.
//
// Response: 200 OK, { "results": [ {ok, filename, item?, reason?, code?}, ... ] }
//
// The HTTP status is 200 even when individual files fail — per-file outcomes
// are reported in the body. The endpoint as a whole only returns non-2xx for
// authz/parsing failures that prevent any work from happening.
func (h *PackHandler) BulkCreateImageItems(w http.ResponseWriter, r *http.Request) {
	u, ok := middleware.GetSessionUser(r)
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	packID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid pack ID")
		return
	}
	pack, err := h.db.GetPackByID(r.Context(), packID)
	if err != nil {
		writeError(w, r, http.StatusNotFound, "not_found", "Pack not found")
		return
	}
	if !canEditItems(r, h.db, pack, u) {
		writeError(w, r, http.StatusForbidden, "forbidden", "Access denied")
		return
	}
	if !h.ensureNotSystem(w, r, pack) {
		return
	}

	// Hard cap on the request body so a single bulk call cannot pin a worker
	// on arbitrarily large input. MaxBulkUploadFiles × MaxUploadSizeBytes plus
	// a small slack for the multipart envelope.
	maxBody := int64(MaxBulkUploadFiles)*h.cfg.MaxUploadSizeBytes + 64*1024
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)

	if err := r.ParseMultipartForm(h.cfg.MaxUploadSizeBytes); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, r, http.StatusRequestEntityTooLarge, "request_too_large",
				fmt.Sprintf("Bulk request exceeds %d bytes", maxBody))
			return
		}
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid multipart form")
		return
	}

	files := r.MultipartForm.File["file"]
	if len(files) == 0 {
		writeError(w, r, http.StatusBadRequest, "bad_request", "At least one file field is required")
		return
	}
	if len(files) > MaxBulkUploadFiles {
		writeError(w, r, http.StatusRequestEntityTooLarge, "too_many_files",
			fmt.Sprintf("Bulk import accepts at most %d files per request", MaxBulkUploadFiles))
		return
	}

	// Optional names parallel to files. Missing entries fall back to the
	// filename minus extension to preserve the single-upload behaviour.
	names := r.MultipartForm.Value["name"]

	results := make([]bulkUploadResult, len(files))
	for i, fh := range files {
		name := ""
		if i < len(names) {
			name = strings.TrimSpace(names[i])
		}
		if name == "" {
			name = stripExt(fh.Filename)
		}
		results[i] = h.processBulkFile(r.Context(), packID, name, fh)
		// Audit-trail bump for group packs, mirroring CreateItem + UpdateItem.
		// Only fires when the row was successfully created. Best-effort: a
		// failed bump must not flip the per-file result.
		if results[i].OK && results[i].Item != nil {
			bumpGroupEditor(r, h.db, pack, results[i].Item.ID, u)
		}
		// Per-file failures get logged so the operator can correlate a
		// "Import failed" toast with a real cause (rate limit, S3 outage,
		// MIME mismatch, etc.). Successful files stay silent — this is
		// the studio's hot path and a bulk batch can be hundreds of files.
		if !results[i].OK {
			log.Printf("bulk_upload: pack=%s user=%s file=%q code=%s reason=%q",
				packID, u.UserID, results[i].Filename, results[i].Code, results[i].Reason)
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

// processBulkFile runs the full create+store+version+promote pipeline for one
// uploaded file. The DB writes happen in a single transaction so a partial
// failure leaves no orphan rows. The storage upload happens between the item
// insert and the version insert because the object key embeds the item ID;
// if the storage upload fails the DB transaction is rolled back, and if the
// DB transaction fails after a successful upload we best-effort delete the
// stored object so the bucket does not accumulate unreferenced blobs.
func (h *PackHandler) processBulkFile(
	ctx context.Context,
	packID uuid.UUID,
	name string,
	fh *multipart.FileHeader,
) bulkUploadResult {
	res := bulkUploadResult{Filename: fh.Filename}

	if fh.Size > h.cfg.MaxUploadSizeBytes {
		res.Code = "file_too_large"
		res.Reason = fmt.Sprintf("File exceeds maximum size of %d bytes", h.cfg.MaxUploadSizeBytes)
		return res
	}
	if name == "" {
		res.Code = "bad_request"
		res.Reason = "name is required"
		return res
	}

	f, err := fh.Open()
	if err != nil {
		res.Code = "bad_request"
		res.Reason = "Failed to open uploaded file"
		return res
	}
	defer f.Close()

	// Buffer the full file in memory — bounded by MaxUploadSizeBytes per
	// file. We need it in memory so we can sniff magic bytes once and then
	// re-read for the storage upload (the multipart Reader is not seekable
	// after the first read).
	var buf bytes.Buffer
	n, err := io.Copy(&buf, io.LimitReader(f, h.cfg.MaxUploadSizeBytes+1))
	if err != nil {
		res.Code = "bad_request"
		res.Reason = "Failed to read uploaded file"
		return res
	}
	if n > h.cfg.MaxUploadSizeBytes {
		res.Code = "file_too_large"
		res.Reason = fmt.Sprintf("File exceeds maximum size of %d bytes", h.cfg.MaxUploadSizeBytes)
		return res
	}

	mimeType := fh.Header.Get("Content-Type")
	sample := buf.Bytes()
	if len(sample) > 512 {
		sample = sample[:512]
	}
	if err := storage.ValidateMIME(mimeType, sample); err != nil {
		res.Code = "invalid_mime_type"
		res.Reason = err.Error()
		return res
	}

	tx, err := h.pool.Begin(ctx)
	if err != nil {
		res.Code = "internal_error"
		res.Reason = "Failed to start transaction"
		return res
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()
	qtx := h.db.WithTx(tx)

	item, err := qtx.CreateItem(ctx, db.CreateItemParams{
		PackID:         packID,
		Name:           name,
		PayloadVersion: 1, // image items
	})
	if err != nil {
		res.Code = "internal_error"
		res.Reason = "Failed to create item"
		return res
	}

	key := storage.ObjectKey(packID.String(), item.ID.String(), 1, sanitizeFilename(fh.Filename))
	if err := h.storage.Upload(ctx, key, bytes.NewReader(buf.Bytes()), mimeType, int64(buf.Len())); err != nil {
		res.Code = "storage_error"
		res.Reason = "Failed to store file"
		return res
	}

	// orientation is best-effort: detection failure is non-fatal. Frontends
	// that rely on the bucket fall back to a default container.
	payload := json.RawMessage(`{}`)
	if orient, err := storage.DetectOrientation(buf.Bytes()); err == nil {
		// orient is one of a closed set of identifier strings; safe to splice.
		payload = json.RawMessage(`{"orientation":"` + orient + `"}`)
	}

	mediaKey := key
	version, err := qtx.CreateItemVersion(ctx, db.CreateItemVersionParams{
		ItemID:   item.ID,
		MediaKey: &mediaKey,
		Payload:  payload,
	})
	if err != nil {
		// DB write failed after we'd already pushed bytes to the bucket;
		// orphan the object now so the bucket doesn't accumulate
		// unreferenced blobs. Best-effort: storage Delete failures are
		// logged on the storage layer, not surfaced to the user.
		_ = h.storage.Delete(ctx, key)
		res.Code = "internal_error"
		res.Reason = "Failed to create version"
		return res
	}

	promoted, err := qtx.SetCurrentVersion(ctx, db.SetCurrentVersionParams{
		ID:               item.ID,
		CurrentVersionID: pgtype.UUID{Bytes: version.ID, Valid: true},
	})
	if err != nil {
		_ = h.storage.Delete(ctx, key)
		res.Code = "internal_error"
		res.Reason = "Failed to promote version"
		return res
	}

	if err := tx.Commit(ctx); err != nil {
		_ = h.storage.Delete(ctx, key)
		res.Code = "internal_error"
		res.Reason = "Failed to commit transaction"
		return res
	}
	committed = true

	thumb := MediaURL(key)
	row := db.ListItemsForPackRow{
		ID:               promoted.ID,
		PackID:           promoted.PackID,
		Name:             promoted.Name,
		Position:         promoted.Position,
		PayloadVersion:   promoted.PayloadVersion,
		CurrentVersionID: promoted.CurrentVersionID,
		CreatedAt:        promoted.CreatedAt,
		DeletedAt:        promoted.DeletedAt,
		LastEditorUserID: promoted.LastEditorUserID,
		LastEditedAt:     promoted.LastEditedAt,
		MediaKey:         &mediaKey,
		Payload:          payload,
		VersionNumber:    &version.VersionNumber,
	}
	res.OK = true
	res.Item = &enrichedItem{ListItemsForPackRow: row, ThumbnailURL: &thumb}
	return res
}

// stripExt drops the final ".<ext>" from a filename. Matches the client-side
// derivation that uses /\.[^.]+$/ as a regex so the default name is the
// same whether the import comes from drag-and-drop or the bulk endpoint.
func stripExt(name string) string {
	ext := filepath.Ext(name)
	if ext == "" {
		return name
	}
	return strings.TrimSuffix(name, ext)
}
