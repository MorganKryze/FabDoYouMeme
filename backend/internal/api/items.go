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
		Lim:    int32(limit),
		Off:    int32(offset),
	})
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list items")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": items})
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
	var req struct {
		PayloadVersion int `json:"payload_version"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.PayloadVersion = 1
	}
	if req.PayloadVersion == 0 {
		req.PayloadVersion = 1
	}
	item, err := h.db.CreateItem(r.Context(), db.CreateItemParams{
		PackID:         packID,
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
