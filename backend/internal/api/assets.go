// backend/internal/api/assets.go
package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/auth"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/config"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/middleware"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/storage"
)

// AssetHandler handles /api/assets/* routes.
type AssetHandler struct {
	db      *db.Queries
	cfg     *config.Config
	storage storage.Storage
}

func NewAssetHandler(pool *pgxpool.Pool, cfg *config.Config, store storage.Storage) *AssetHandler {
	return &AssetHandler{db: db.New(pool), cfg: cfg, storage: store}
}

// UploadURL handles POST /api/assets/upload-url.
// Request body: { pack_id, item_id, version_number, filename, mime_type, size_bytes, preview_bytes (base64) }
func (h *AssetHandler) UploadURL(w http.ResponseWriter, r *http.Request) {
	u, ok := middleware.GetSessionUser(r)
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	var req struct {
		PackID        string `json:"pack_id"`
		ItemID        string `json:"item_id"`
		VersionNumber int    `json:"version_number"`
		Filename      string `json:"filename"`
		MIMEType      string `json:"mime_type"`
		SizeBytes     int64  `json:"size_bytes"`
		PreviewBytes  string `json:"preview_bytes"` // base64-encoded first ~512 bytes for magic byte validation
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid JSON")
		return
	}

	// Size check
	if req.SizeBytes > h.cfg.MaxUploadSizeBytes {
		writeError(w, r, http.StatusUnprocessableEntity, "file_too_large",
			fmt.Sprintf("File exceeds maximum size of %d bytes", h.cfg.MaxUploadSizeBytes))
		return
	}

	// MIME validation — preview_bytes is required for magic byte check
	if req.PreviewBytes == "" {
		writeError(w, r, http.StatusBadRequest, "bad_request", "preview_bytes is required for MIME validation")
		return
	}
	sample, err := base64.StdEncoding.DecodeString(req.PreviewBytes)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "preview_bytes must be base64-encoded")
		return
	}
	if err := storage.ValidateMIME(req.MIMEType, sample); err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, "invalid_mime_type", err.Error())
		return
	}

	// Authorization: admin, personal owner, or any member of the owning group.
	packID, err := uuid.Parse(req.PackID)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid pack_id")
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

	// Generate object key and pre-signed URL
	key := storage.ObjectKey(req.PackID, req.ItemID, req.VersionNumber, sanitizeFilename(req.Filename))
	uploadURL, err := h.storage.PresignUpload(r.Context(), key, 15*time.Minute)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to generate upload URL")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"upload_url": uploadURL,
		"media_key":  key,
	})
}

// UploadDirect handles POST /api/assets/upload (multipart/form-data).
//
// This is the proxied-upload path: the browser POSTs the file bytes to our
// own backend, which validates them and forwards to RustFS via the AWS SDK.
// It exists because direct browser→RustFS PUTs fail CORS preflight when the
// bucket has no CORS config, and is now the default path used by the studio UI.
//
// Form fields:
//
//	pack_id         — UUID of the pack (authorization scope)
//	item_id         — UUID of the item (used only for the object key)
//	version_number  — integer (used only for the object key)
//	file            — the actual image bytes
//
// Sanitization / security checks — in this order:
//  1. http.MaxBytesReader caps the request body at cfg.MaxUploadSizeBytes+slack
//     so we can't be forced to buffer arbitrary data.
//  2. ParseMultipartForm with the same limit as the disk-spill threshold.
//  3. Session auth + pack ownership (admin OR owner_id == session user).
//  4. Declared Content-Type must be in the MIME allowlist AND magic bytes of
//     the file must match (storage.ValidateMIME).
//  5. Filename is sanitized via sanitizeFilename and the object key is
//     server-derived from pack_id/item_id/version_number — the client never
//     picks the storage path.
func (h *AssetHandler) UploadDirect(w http.ResponseWriter, r *http.Request) {
	u, ok := middleware.GetSessionUser(r)
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	// Hard cap the request body. We add 1 KiB of slack for the multipart
	// envelope (boundaries, headers) so a file exactly at MaxUploadSizeBytes
	// still fits.
	maxBody := h.cfg.MaxUploadSizeBytes + 1024
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)

	if err := r.ParseMultipartForm(h.cfg.MaxUploadSizeBytes); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, r, http.StatusRequestEntityTooLarge, "file_too_large",
				fmt.Sprintf("File exceeds maximum size of %d bytes", h.cfg.MaxUploadSizeBytes))
			return
		}
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid multipart form")
		return
	}

	packIDStr := r.FormValue("pack_id")
	itemIDStr := r.FormValue("item_id")
	versionStr := r.FormValue("version_number")
	if packIDStr == "" || itemIDStr == "" || versionStr == "" {
		writeError(w, r, http.StatusBadRequest, "bad_request",
			"pack_id, item_id, and version_number are required")
		return
	}

	packID, err := uuid.Parse(packIDStr)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid pack_id")
		return
	}
	if _, err := uuid.Parse(itemIDStr); err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid item_id")
		return
	}
	versionNumber, err := strconv.Atoi(versionStr)
	if err != nil || versionNumber < 1 {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid version_number")
		return
	}

	// Authorization: admin, personal owner, or any member of the owning group.
	pack, err := h.db.GetPackByID(r.Context(), packID)
	if err != nil {
		writeError(w, r, http.StatusNotFound, "not_found", "Pack not found")
		return
	}
	if !canEditItems(r, h.db, pack, u) {
		writeError(w, r, http.StatusForbidden, "forbidden", "Access denied")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "file field is required")
		return
	}
	defer file.Close()

	if header.Size > h.cfg.MaxUploadSizeBytes {
		writeError(w, r, http.StatusRequestEntityTooLarge, "file_too_large",
			fmt.Sprintf("File exceeds maximum size of %d bytes", h.cfg.MaxUploadSizeBytes))
		return
	}

	// Buffer the full file in memory — bounded by MaxUploadSizeBytes (default 2 MiB).
	// We need it in memory so we can (a) sniff magic bytes and (b) hand a
	// seekable Reader to the AWS SDK with an exact Content-Length.
	var buf bytes.Buffer
	n, err := io.Copy(&buf, io.LimitReader(file, h.cfg.MaxUploadSizeBytes+1))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Failed to read file")
		return
	}
	if n > h.cfg.MaxUploadSizeBytes {
		writeError(w, r, http.StatusRequestEntityTooLarge, "file_too_large",
			fmt.Sprintf("File exceeds maximum size of %d bytes", h.cfg.MaxUploadSizeBytes))
		return
	}

	mimeType := header.Header.Get("Content-Type")
	sample := buf.Bytes()
	if len(sample) > 512 {
		sample = sample[:512]
	}
	if err := storage.ValidateMIME(mimeType, sample); err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, "invalid_mime_type", err.Error())
		return
	}

	key := storage.ObjectKey(packIDStr, itemIDStr, versionNumber, sanitizeFilename(header.Filename))
	if err := h.storage.Upload(r.Context(), key, bytes.NewReader(buf.Bytes()), mimeType, int64(buf.Len())); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to store file")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"media_key": key})
}

// GetMedia handles GET /api/assets/media?key=...[&guest_token=...]
//
// Proxies the object at media_key through the backend so the browser never
// talks directly to RustFS. Used by <img> tags for thumbnails (studio, admin)
// and by in-game round prompts (gameplay). The handler self-resolves identity
// so both registered users (session cookie) and guests (guest_token query
// param, minted via POST /rooms/{code}/guest-join) can fetch media they are
// entitled to see. This mirrors WSHandler.resolveIdentity; the route is
// therefore mounted outside the RequireAuth middleware group.
//
// Authorization:
//   - admin: any media_key
//   - registered user: CanUserDownloadMedia (owner OR public+active pack)
//   - guest: CanGuestDownloadMedia (media used in a round of a room the
//     guest is currently joined to) — narrower than the pack-wide predicate
//     so a guest can't enumerate unshown items via media_key guessing.
//
// The media_key is passed as a query parameter (URL-encoded). We don't support
// path-style routing because the key itself contains slashes.
func (h *AssetHandler) GetMedia(w http.ResponseWriter, r *http.Request) {
	mediaKey := r.URL.Query().Get("key")
	if mediaKey == "" {
		writeError(w, r, http.StatusBadRequest, "bad_request", "key query parameter is required")
		return
	}

	if u, ok := middleware.GetSessionUser(r); ok {
		if u.Role != "admin" {
			uid, err := uuid.Parse(u.UserID)
			if err != nil {
				writeError(w, r, http.StatusForbidden, "forbidden", "Access denied")
				return
			}
			mk := mediaKey
			allowed, err := h.db.CanUserDownloadMedia(r.Context(), db.CanUserDownloadMediaParams{
				MediaKey: &mk,
				UserID:   uid,
			})
			if err != nil {
				writeError(w, r, http.StatusInternalServerError, "internal_error", "Authorization check failed")
				return
			}
			if !allowed {
				writeError(w, r, http.StatusForbidden, "forbidden", "Access denied")
				return
			}
		}
	} else {
		// No session — fall through to guest_token. We deliberately collapse
		// every failure below into a single 401 (missing, malformed, expired,
		// or wrong-media token all look alike) so a probing caller can't tell
		// which gate rejected them, same policy as WSHandler.resolveIdentity.
		token := r.URL.Query().Get("guest_token")
		if token == "" {
			writeError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
			return
		}
		gp, err := h.db.GetGuestPlayerByTokenHash(r.Context(), auth.HashToken(token))
		if err != nil {
			writeError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
			return
		}
		mk := mediaKey
		allowed, err := h.db.CanGuestDownloadMedia(r.Context(), db.CanGuestDownloadMediaParams{
			MediaKey:      &mk,
			GuestPlayerID: gp.ID,
		})
		if err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal_error", "Authorization check failed")
			return
		}
		if !allowed {
			writeError(w, r, http.StatusForbidden, "forbidden", "Access denied")
			return
		}
	}

	body, contentType, size, err := h.storage.Download(r.Context(), mediaKey)
	if err != nil {
		writeError(w, r, http.StatusNotFound, "not_found", "Media not found")
		return
	}
	defer body.Close()

	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	if size > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	}
	// Cache on the browser. These URLs include the media_key which is
	// effectively immutable (new versions use new keys), so long caching is safe.
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.WriteHeader(http.StatusOK)
	if _, err := io.Copy(w, body); err != nil {
		// Headers already sent; we can't return JSON — just log via the
		// connection being torn down. Swallow the error.
		return
	}
}

// MediaURL builds the backend-relative URL that /api/assets/media will serve
// for a given object key. Used by list handlers to inject thumbnail_url /
// media_url fields without calling an S3 presigner.
func MediaURL(key string) string {
	return "/api/assets/media?key=" + url.QueryEscape(key)
}

// DownloadURL handles POST /api/assets/download-url.
//
// Authorization (P1.4 / finding 5.A): a caller may download a media_key iff
//
//   - they are an admin, OR
//   - the media belongs to a pack they own, OR
//   - the media belongs to a public + active pack.
//
// The pack-side checks are pushed into a single sqlc query (CanUserDownloadMedia)
// so the predicate is atomic — there's no TOCTOU window between an existence
// check and the response. Pre-fix any logged-in user could download any
// media_key they could see or guess.
func (h *AssetHandler) DownloadURL(w http.ResponseWriter, r *http.Request) {
	u, ok := middleware.GetSessionUser(r)
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	var req struct {
		MediaKey string `json:"media_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.MediaKey == "" {
		writeError(w, r, http.StatusBadRequest, "bad_request", "media_key is required")
		return
	}

	if u.Role != "admin" {
		uid, err := uuid.Parse(u.UserID)
		if err != nil {
			writeError(w, r, http.StatusForbidden, "forbidden", "Access denied")
			return
		}
		mediaKey := req.MediaKey
		allowed, err := h.db.CanUserDownloadMedia(r.Context(), db.CanUserDownloadMediaParams{
			MediaKey: &mediaKey,
			UserID:   uid,
		})
		if err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal_error", "Authorization check failed")
			return
		}
		if !allowed {
			writeError(w, r, http.StatusForbidden, "forbidden", "Access denied")
			return
		}
	}

	downloadURL, err := h.storage.PresignDownload(r.Context(), req.MediaKey, 15*time.Minute)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to generate download URL")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"download_url": downloadURL})
}

func sanitizeFilename(name string) string {
	// Remove path components and dangerous characters
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "..", "_")
	if name == "" {
		name = "file"
	}
	return name
}
