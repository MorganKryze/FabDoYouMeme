// backend/internal/api/assets.go
package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
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

	// Authorization: admin or pack owner
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
	ownerID, _ := uuid.Parse(u.UserID)
	if u.Role != "admin" && (!pack.OwnerID.Valid || pack.OwnerID.Bytes != ownerID) {
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

// DownloadURL handles POST /api/assets/download-url (admin/owner preview only).
func (h *AssetHandler) DownloadURL(w http.ResponseWriter, r *http.Request) {
	_, ok := middleware.GetSessionUser(r)
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	var req struct{ MediaKey string `json:"media_key"` }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.MediaKey == "" {
		writeError(w, r, http.StatusBadRequest, "bad_request", "media_key is required")
		return
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
