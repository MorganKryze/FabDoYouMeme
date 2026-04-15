// backend/internal/api/items.go
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

// enrichedItem is the wire shape for GET /api/packs/{id}/items.
// It embeds the sqlc row so existing fields keep their json tags, and adds
// thumbnail_url — a short-lived pre-signed GET URL derived from the current
// version's media_key.
type enrichedItem struct {
	db.ListItemsForPackRow
	ThumbnailURL *string `json:"thumbnail_url,omitempty"`
}

// ListItems handles GET /api/packs/{id}/items.
func (h *PackHandler) ListItems(w http.ResponseWriter, r *http.Request) {
	_, ok := middleware.GetSessionUser(r)
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	packID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid pack ID")
		return
	}
	limit, offset := parsePagination(r)
	items, err := h.db.ListItemsForPack(r.Context(), db.ListItemsForPackParams{
		PackID: packID,
		Lim:    limit,
		Off:    offset,
	})
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list items")
		return
	}

	// Each item's thumbnail_url is a backend-relative URL served by
	// GET /api/assets/media — the browser fetches the image through the
	// backend (same origin as the app), bypassing any RustFS CORS / DNS
	// reachability issues.
	enriched := make([]enrichedItem, 0, len(items))
	for _, it := range items {
		ei := enrichedItem{ListItemsForPackRow: it}
		if it.MediaKey != nil && *it.MediaKey != "" {
			u := MediaURL(*it.MediaKey)
			ei.ThumbnailURL = &u
		}
		enriched = append(enriched, ei)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": enriched})
}

// CreateItem handles POST /api/packs/{id}/items.
func (h *PackHandler) CreateItem(w http.ResponseWriter, r *http.Request) {
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
	ownerID, _ := uuid.Parse(u.UserID)
	if u.Role != "admin" && (!pack.OwnerID.Valid || pack.OwnerID.Bytes != ownerID) {
		writeError(w, r, http.StatusForbidden, "forbidden", "Access denied")
		return
	}
	if !h.ensureNotSystem(w, r, pack) {
		return
	}
	var req struct {
		Name           string `json:"name"`
		PayloadVersion int    `json:"payload_version"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.PayloadVersion = 1
	}
	if req.PayloadVersion == 0 {
		req.PayloadVersion = 1
	}
	if req.Name == "" {
		writeError(w, r, http.StatusBadRequest, "bad_request", "name is required")
		return
	}
	item, err := h.db.CreateItem(r.Context(), db.CreateItemParams{
		PackID:         packID,
		Name:           req.Name,
		PayloadVersion: int32(req.PayloadVersion),
	})
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to create item")
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

// UpdateItem handles PATCH /api/packs/{id}/items/{item_id}.
func (h *PackHandler) UpdateItem(w http.ResponseWriter, r *http.Request) {
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
	ownerID, _ := uuid.Parse(u.UserID)
	if u.Role != "admin" && (!pack.OwnerID.Valid || pack.OwnerID.Bytes != ownerID) {
		writeError(w, r, http.StatusForbidden, "forbidden", "Access denied")
		return
	}
	if !h.ensureNotSystem(w, r, pack) {
		return
	}
	var req struct {
		CurrentVersionID *string `json:"current_version_id,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid JSON")
		return
	}
	if req.CurrentVersionID != nil {
		versionID, err := uuid.Parse(*req.CurrentVersionID)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid version ID")
			return
		}
		item, err := h.db.SetCurrentVersion(r.Context(), db.SetCurrentVersionParams{
			ID:               itemID,
			CurrentVersionID: pgtype.UUID{Bytes: versionID, Valid: true},
		})
		if err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal_error", "Update failed")
			return
		}
		writeJSON(w, http.StatusOK, item)
		return
	}
	writeError(w, r, http.StatusBadRequest, "bad_request", "No updatable fields provided")
}

// DeleteItem handles DELETE /api/packs/{id}/items/{item_id}.
func (h *PackHandler) DeleteItem(w http.ResponseWriter, r *http.Request) {
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
	ownerID, _ := uuid.Parse(u.UserID)
	if u.Role != "admin" && (!pack.OwnerID.Valid || pack.OwnerID.Bytes != ownerID) {
		writeError(w, r, http.StatusForbidden, "forbidden", "Access denied")
		return
	}
	if !h.ensureNotSystem(w, r, pack) {
		return
	}
	if err := h.db.DeleteItem(r.Context(), itemID); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Delete failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ReorderItems handles PATCH /api/packs/{id}/items/reorder.
func (h *PackHandler) ReorderItems(w http.ResponseWriter, r *http.Request) {
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
	ownerID, _ := uuid.Parse(u.UserID)
	if u.Role != "admin" && (!pack.OwnerID.Valid || pack.OwnerID.Bytes != ownerID) {
		writeError(w, r, http.StatusForbidden, "forbidden", "Access denied")
		return
	}
	if !h.ensureNotSystem(w, r, pack) {
		return
	}
	var req struct {
		Positions []struct {
			ID       string `json:"id"`
			Position int    `json:"position"`
		} `json:"positions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid JSON")
		return
	}
	for _, p := range req.Positions {
		itemID, err := uuid.Parse(p.ID)
		if err != nil {
			continue
		}
		if err := h.db.ReorderItems(r.Context(), db.ReorderItemsParams{
			ID:       itemID,
			PackID:   packID,
			Position: int32(p.Position),
		}); err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal_error", "Reorder failed")
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}
