// backend/internal/api/packs.go
package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/config"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/game"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/middleware"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/storage"
)

// PackHandler handles all /api/packs/* routes.
type PackHandler struct {
	db       *db.Queries
	cfg      *config.Config
	storage  storage.Storage
	registry *game.Registry
}

// NewPackHandler wires the pack routes. Registry may be nil in unit tests that
// only exercise CRUD paths where role/game-type filtering isn't needed; the
// List handler degrades to its unfiltered behaviour when the registry is nil
// or the requested game type isn't registered.
func NewPackHandler(pool *pgxpool.Pool, cfg *config.Config, store storage.Storage, registry *game.Registry) *PackHandler {
	return &PackHandler{db: db.New(pool), cfg: cfg, storage: store, registry: registry}
}

// ensureNotSystem writes a 403 system_pack_readonly response and returns false
// when the given pack is managed by systempack. Every mutating pack/item
// handler must call this after loading the pack and before any write.
// There is NO admin bypass — the filesystem is the only way to update.
func (h *PackHandler) ensureNotSystem(w http.ResponseWriter, r *http.Request, pack db.GamePack) bool {
	if pack.IsSystem {
		writeError(w, r, http.StatusForbidden, "system_pack_readonly",
			"This pack is managed by the server and cannot be modified.")
		return false
	}
	return true
}

// Create handles POST /api/packs.
func (h *PackHandler) Create(w http.ResponseWriter, r *http.Request) {
	u, ok := middleware.GetSessionUser(r)
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Visibility  string `json:"visibility"`
		IsOfficial  bool   `json:"is_official"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid JSON")
		return
	}
	if req.Name == "" {
		writeError(w, r, http.StatusBadRequest, "bad_request", "name is required")
		return
	}
	if req.Visibility == "" {
		req.Visibility = "private"
	}
	if req.IsOfficial && u.Role != "admin" {
		writeError(w, r, http.StatusForbidden, "forbidden", "Only admins can create official packs")
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
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to create pack")
		return
	}
	writeJSON(w, http.StatusCreated, pack)
}

// List handles GET /api/packs.
//
// Optional query params:
//   - game_type=<slug>  scope picker to a single game type
//   - role=image|text   scope picker to one of that game type's required pack
//     roles (meme-showdown needs both image and text, rendered as two pickers)
//
// When both are present and the slug is registered, packs with zero items of
// the role's supported payload versions are dropped. Missing or unregistered
// values fall back to the unfiltered list so /api/packs remains useful for
// studio-style browsing.
func (h *PackHandler) List(w http.ResponseWriter, r *http.Request) {
	u, ok := middleware.GetSessionUser(r)
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	limit, offset := parsePagination(r)
	userID, _ := uuid.Parse(u.UserID)

	var packs []db.GamePack
	var err error
	if u.Role == "admin" {
		packs, err = h.db.ListAllPacksAdmin(r.Context(), db.ListAllPacksAdminParams{
			Lim: limit, Off: offset,
		})
	} else {
		packs, err = h.db.ListPacksForUser(r.Context(), db.ListPacksForUserParams{
			UserID: pgtype.UUID{Bytes: userID, Valid: true},
			Lim:    limit,
			Off:    offset,
		})
	}
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list packs")
		return
	}

	if versions := h.roleVersions(r.URL.Query().Get("game_type"), r.URL.Query().Get("role")); versions != nil {
		filtered := make([]db.GamePack, 0, len(packs))
		for _, p := range packs {
			count, cerr := h.db.CountCompatibleItems(r.Context(), db.CountCompatibleItemsParams{
				PackID:   p.ID,
				Versions: int32SliceToI32Arr(versions),
			})
			if cerr != nil {
				writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to count pack items")
				return
			}
			if count > 0 {
				filtered = append(filtered, p)
			}
		}
		packs = filtered
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data":        packs,
		"next_cursor": nextCursor(len(packs), limit, offset),
	})
}

// roleVersions returns the payload_version whitelist for a (gameTypeSlug,
// role) pair, or nil when filtering is not applicable — missing params,
// unregistered slug, or role not declared by the handler.
func (h *PackHandler) roleVersions(gameTypeSlug, role string) []int {
	if gameTypeSlug == "" || role == "" || h.registry == nil {
		return nil
	}
	handler, ok := h.registry.Get(gameTypeSlug)
	if !ok {
		return nil
	}
	for _, pr := range handler.RequiredPacks() {
		if string(pr.Role) == role {
			return pr.PayloadVersions
		}
	}
	return nil
}

// GetByID handles GET /api/packs/:id.
func (h *PackHandler) GetByID(w http.ResponseWriter, r *http.Request) {
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
	isOwner := pack.OwnerID.Valid && pack.OwnerID.Bytes == ownerID
	isPublicActive := pack.Visibility == "public" && pack.Status == "active" && !pack.DeletedAt.Valid
	if u.Role != "admin" && !isOwner && !isPublicActive {
		writeError(w, r, http.StatusForbidden, "forbidden", "Access denied")
		return
	}
	writeJSON(w, http.StatusOK, pack)
}

// Update handles PATCH /api/packs/:id.
func (h *PackHandler) Update(w http.ResponseWriter, r *http.Request) {
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
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Update failed")
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
	if err := h.db.SoftDeletePack(r.Context(), packID); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Delete failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// SetStatus handles PATCH /api/packs/:id/status (admin only).
func (h *PackHandler) SetStatus(w http.ResponseWriter, r *http.Request) {
	admin, ok := middleware.GetSessionUser(r)
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
	if !h.ensureNotSystem(w, r, pack) {
		return
	}
	var req struct{ Status string `json:"status"` }
	json.NewDecoder(r.Body).Decode(&req) //nolint:errcheck
	if req.Status != "active" && req.Status != "flagged" && req.Status != "banned" {
		writeError(w, r, http.StatusBadRequest, "bad_request", "status must be active, flagged, or banned")
		return
	}
	updated, err := h.db.SetPackStatus(r.Context(), db.SetPackStatusParams{
		ID: packID, Status: req.Status,
	})
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Update failed")
		return
	}
	writeAuditLog(r.Context(), h.db, admin.UserID, "set_pack_status",
		fmt.Sprintf("pack:%s", packID),
		map[string]string{"status": req.Status})
	writeJSON(w, http.StatusOK, updated)
}

// Helpers

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// cursor is the opaque pagination token, base64-encoded JSON.
type cursor struct {
	CreatedAt time.Time `json:"created_at"`
	ID        string    `json:"id"`
}

func encodeCursor(c cursor) string {
	b, _ := json.Marshal(c)
	return base64.StdEncoding.EncodeToString(b)
}

func decodeCursor(s string) (cursor, error) {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return cursor{}, err
	}
	var c cursor
	return c, json.Unmarshal(b, &c)
}

func parsePagination(r *http.Request) (limit, offset int32) {
	limit = 50
	offset = 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.ParseInt(l, 10, 32); err == nil && v > 0 && v <= 100 {
			limit = int32(v)
		}
	}
	if c := r.URL.Query().Get("after"); c != "" {
		if decoded, err := decodeCursor(c); err == nil {
			if v, err := strconv.ParseInt(decoded.ID, 10, 32); err == nil && v >= 0 {
				offset = int32(v)
			}
		} else if v, err := strconv.ParseInt(c, 10, 32); err == nil && v >= 0 {
			// Fallback: accept plain integer offset for backwards compatibility
			offset = int32(v)
		}
	}
	return
}

func nextCursor(count int, limit, offset int32) string {
	if count < int(limit) {
		return ""
	}
	c := cursor{CreatedAt: time.Now(), ID: strconv.Itoa(int(offset) + count)}
	return encodeCursor(c)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	body := map[string]string{"error": message, "code": code}
	if r != nil {
		if reqID := chiMiddleware.GetReqID(r.Context()); reqID != "" {
			body["request_id"] = reqID
		}
	}
	writeJSON(w, status, body)
}
