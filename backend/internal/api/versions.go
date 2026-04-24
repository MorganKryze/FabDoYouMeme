// backend/internal/api/versions.go
package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/middleware"
)

// enrichedVersion is the wire shape for GET /api/packs/{id}/items/{item_id}/versions.
// Embeds the sqlc row so existing fields keep their json tags and adds
// media_url — a short-lived pre-signed GET URL derived from media_key.
type enrichedVersion struct {
	db.GameItemVersion
	MediaURL *string `json:"media_url,omitempty"`
}

// ListVersions handles GET /api/packs/{id}/items/{item_id}/versions.
func (h *PackHandler) ListVersions(w http.ResponseWriter, r *http.Request) {
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
	itemID, err := uuid.Parse(chi.URLParam(r, "item_id"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid item ID")
		return
	}
	pack, err := h.db.GetPackByID(r.Context(), packID)
	if err != nil {
		writeError(w, r, http.StatusNotFound, "not_found", "Pack not found")
		return
	}
	if !canReadPack(r, h.db, pack, u) {
		writeError(w, r, http.StatusForbidden, "forbidden", "Access denied")
		return
	}
	versions, err := h.db.ListVersionsForItem(r.Context(), itemID)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list versions")
		return
	}

	enriched := make([]enrichedVersion, 0, len(versions))
	for _, v := range versions {
		ev := enrichedVersion{GameItemVersion: v}
		if v.MediaKey != nil && *v.MediaKey != "" {
			u := MediaURL(*v.MediaKey)
			ev.MediaURL = &u
		}
		enriched = append(enriched, ev)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": enriched})
}

// CreateVersion handles POST /api/packs/{id}/items/{item_id}/versions.
func (h *PackHandler) CreateVersion(w http.ResponseWriter, r *http.Request) {
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
	itemID, err := uuid.Parse(chi.URLParam(r, "item_id"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid item ID")
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
	var req struct {
		MediaKey string          `json:"media_key"`
		Payload  json.RawMessage `json:"payload"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid JSON")
		return
	}
	if req.Payload == nil {
		req.Payload = json.RawMessage(`{}`)
	}
	var mediaKey *string
	if req.MediaKey != "" {
		mk := req.MediaKey
		mediaKey = &mk
	}
	version, err := h.db.CreateItemVersion(r.Context(), db.CreateItemVersionParams{
		ItemID:   itemID,
		MediaKey: mediaKey,
		Payload:  req.Payload,
	})
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to create version")
		return
	}
	bumpGroupEditor(r, h.db, pack, itemID, u)
	writeJSON(w, http.StatusCreated, version)
}

// RestoreVersion handles POST /api/packs/{id}/items/{item_id}/versions/{vid}/restore.
func (h *PackHandler) RestoreVersion(w http.ResponseWriter, r *http.Request) {
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
	itemID, err := uuid.Parse(chi.URLParam(r, "item_id"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid item ID")
		return
	}
	versionID, err := uuid.Parse(chi.URLParam(r, "vid"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid version ID")
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
	item, err := h.db.SetCurrentVersion(r.Context(), db.SetCurrentVersionParams{
		ID:               itemID,
		CurrentVersionID: pgtype.UUID{Bytes: versionID, Valid: true},
	})
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Restore failed")
		return
	}
	bumpGroupEditor(r, h.db, pack, itemID, u)
	writeJSON(w, http.StatusOK, item)
}

// SoftDeleteVersion handles DELETE /api/packs/{id}/items/{item_id}/versions/{vid}.
func (h *PackHandler) SoftDeleteVersion(w http.ResponseWriter, r *http.Request) {
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
	itemID, err := uuid.Parse(chi.URLParam(r, "item_id"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid item ID")
		return
	}
	versionID, err := uuid.Parse(chi.URLParam(r, "vid"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid version ID")
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
	if err := h.db.SoftDeleteVersion(r.Context(), versionID); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Delete failed")
		return
	}
	bumpGroupEditor(r, h.db, pack, itemID, u)
	w.WriteHeader(http.StatusNoContent)
}

// PurgeVersion handles DELETE /api/packs/{id}/items/{item_id}/versions/{vid}/purge.
func (h *PackHandler) PurgeVersion(w http.ResponseWriter, r *http.Request) {
	u, ok := middleware.GetSessionUser(r)
	if !ok || u.Role != "admin" {
		writeError(w, r, http.StatusForbidden, "forbidden", "Admin role required")
		return
	}
	versionID, err := uuid.Parse(chi.URLParam(r, "vid"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid version ID")
		return
	}
	if err := h.db.HardDeleteVersion(r.Context(), versionID); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Purge failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
