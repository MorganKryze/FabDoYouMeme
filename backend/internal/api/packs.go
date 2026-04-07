// backend/internal/api/packs.go
package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/config"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/middleware"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/storage"
)

// PackHandler handles all /api/packs/* routes.
type PackHandler struct {
	db      *db.Queries
	cfg     *config.Config
	storage storage.Storage
}

func NewPackHandler(pool *pgxpool.Pool, cfg *config.Config, store storage.Storage) *PackHandler {
	return &PackHandler{db: db.New(pool), cfg: cfg, storage: store}
}

// Create handles POST /api/packs.
func (h *PackHandler) Create(w http.ResponseWriter, r *http.Request) {
	u, ok := middleware.GetSessionUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Visibility  string `json:"visibility"`
		IsOfficial  bool   `json:"is_official"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Invalid JSON")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "name is required")
		return
	}
	if req.Visibility == "" {
		req.Visibility = "private"
	}
	if req.IsOfficial && u.Role != "admin" {
		writeError(w, http.StatusForbidden, "forbidden", "Only admins can create official packs")
		return
	}
	ownerID, _ := uuid.Parse(u.UserID)
	pack, err := h.db.CreatePack(r.Context(), db.CreatePackParams{
		Name:        req.Name,
		Description: strPtr(req.Description),
		OwnerID:     pgtype.UUID{Bytes: ownerID, Valid: true},
		IsOfficial:  req.IsOfficial,
		Visibility:  req.Visibility,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to create pack")
		return
	}
	writeJSON(w, http.StatusCreated, pack)
}

// List handles GET /api/packs.
func (h *PackHandler) List(w http.ResponseWriter, r *http.Request) {
	u, ok := middleware.GetSessionUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	limit, offset := parsePagination(r)
	userID, _ := uuid.Parse(u.UserID)

	var packs []db.GamePack
	var err error
	if u.Role == "admin" {
		packs, err = h.db.ListAllPacksAdmin(r.Context(), db.ListAllPacksAdminParams{
			Lim: int32(limit), Off: int32(offset),
		})
	} else {
		packs, err = h.db.ListPacksForUser(r.Context(), db.ListPacksForUserParams{
			UserID: pgtype.UUID{Bytes: userID, Valid: true},
			Lim:    int32(limit),
			Off:    int32(offset),
		})
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list packs")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data":        packs,
		"next_cursor": nextCursor(len(packs), limit, offset),
	})
}

// GetByID handles GET /api/packs/:id.
func (h *PackHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	u, ok := middleware.GetSessionUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	packID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Invalid pack ID")
		return
	}
	pack, err := h.db.GetPackByID(r.Context(), packID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Pack not found")
		return
	}
	ownerID, _ := uuid.Parse(u.UserID)
	if u.Role != "admin" && (!pack.OwnerID.Valid || pack.OwnerID.Bytes != ownerID) {
		writeError(w, http.StatusForbidden, "forbidden", "Access denied")
		return
	}
	writeJSON(w, http.StatusOK, pack)
}

// Update handles PATCH /api/packs/:id.
func (h *PackHandler) Update(w http.ResponseWriter, r *http.Request) {
	u, ok := middleware.GetSessionUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	packID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Invalid pack ID")
		return
	}
	pack, err := h.db.GetPackByID(r.Context(), packID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Pack not found")
		return
	}
	ownerID, _ := uuid.Parse(u.UserID)
	if u.Role != "admin" && (!pack.OwnerID.Valid || pack.OwnerID.Bytes != ownerID) {
		writeError(w, http.StatusForbidden, "forbidden", "Access denied")
		return
	}
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Visibility  string `json:"visibility"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Name == "" {
		req.Name = pack.Name
	}
	if req.Visibility == "" {
		req.Visibility = pack.Visibility
	}
	oldVisibility := pack.Visibility
	updated, err := h.db.UpdatePack(r.Context(), db.UpdatePackParams{
		ID:          packID,
		Name:        req.Name,
		Description: strPtr(req.Description),
		Visibility:  req.Visibility,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Update failed")
		return
	}
	// Notify admin when pack becomes public
	if oldVisibility != "public" && updated.Visibility == "public" {
		actorID, _ := uuid.Parse(u.UserID)
		h.db.CreateAdminNotification(r.Context(), db.CreateAdminNotificationParams{
			Type:    "pack_published",
			PackID:  pgtype.UUID{Bytes: packID, Valid: true},
			ActorID: pgtype.UUID{Bytes: actorID, Valid: true},
		})
	}
	writeJSON(w, http.StatusOK, updated)
}

// Delete handles DELETE /api/packs/:id (soft delete).
func (h *PackHandler) Delete(w http.ResponseWriter, r *http.Request) {
	u, ok := middleware.GetSessionUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	packID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Invalid pack ID")
		return
	}
	pack, err := h.db.GetPackByID(r.Context(), packID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Pack not found")
		return
	}
	ownerID, _ := uuid.Parse(u.UserID)
	if u.Role != "admin" && (!pack.OwnerID.Valid || pack.OwnerID.Bytes != ownerID) {
		writeError(w, http.StatusForbidden, "forbidden", "Access denied")
		return
	}
	if err := h.db.SoftDeletePack(r.Context(), packID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Delete failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// SetStatus handles PATCH /api/packs/:id/status (admin only).
func (h *PackHandler) SetStatus(w http.ResponseWriter, r *http.Request) {
	packID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Invalid pack ID")
		return
	}
	var req struct{ Status string `json:"status"` }
	json.NewDecoder(r.Body).Decode(&req)
	if req.Status != "active" && req.Status != "flagged" && req.Status != "banned" {
		writeError(w, http.StatusBadRequest, "bad_request", "status must be active, flagged, or banned")
		return
	}
	updated, err := h.db.SetPackStatus(r.Context(), db.SetPackStatusParams{
		ID: packID, Status: req.Status,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Update failed")
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

// Helpers

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func parsePagination(r *http.Request) (limit, offset int) {
	limit = 50
	offset = 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}
	if c := r.URL.Query().Get("after"); c != "" {
		if v, err := strconv.Atoi(c); err == nil && v > 0 {
			offset = v
		}
	}
	return
}

func nextCursor(count, limit, offset int) *string {
	if count < limit {
		return nil
	}
	s := strconv.Itoa(offset + count)
	return &s
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"error": message, "code": code})
}
