// backend/internal/api/room_actions.go
package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/middleware"
)

// UpdateConfig handles PATCH /api/rooms/{code}/config.
func (h *RoomHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	u, ok := middleware.GetSessionUser(r)
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	room, err := h.db.GetRoomByCode(r.Context(), chi.URLParam(r, "code"))
	if err != nil {
		writeError(w, r, http.StatusNotFound, "not_found", "Room not found")
		return
	}
	hostID, _ := uuid.Parse(u.UserID)
	if u.Role != "admin" && (!room.HostID.Valid || room.HostID.Bytes != hostID) {
		writeError(w, r, http.StatusForbidden, "forbidden", "Only the host can update config")
		return
	}
	var req struct {
		Config json.RawMessage `json:"config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid JSON")
		return
	}
	if req.Config == nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "config is required")
		return
	}
	updated, err := h.db.UpdateRoomConfig(r.Context(), db.UpdateRoomConfigParams{
		ID:     room.ID,
		Config: req.Config,
	})
	if err != nil {
		writeError(w, r, http.StatusConflict, "room_not_in_lobby", "Room config can only be updated while in lobby state")
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

// Leave handles POST /api/rooms/{code}/leave.
func (h *RoomHandler) Leave(w http.ResponseWriter, r *http.Request) {
	u, ok := middleware.GetSessionUser(r)
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	room, err := h.db.GetRoomByCode(r.Context(), chi.URLParam(r, "code"))
	if err != nil {
		writeError(w, r, http.StatusNotFound, "not_found", "Room not found")
		return
	}
	userID, _ := uuid.Parse(u.UserID)
	if err := h.db.RemoveRoomPlayer(r.Context(), db.RemoveRoomPlayerParams{
		RoomID: room.ID,
		UserID: userID,
	}); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to leave room")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Kick handles POST /api/rooms/{code}/kick.
func (h *RoomHandler) Kick(w http.ResponseWriter, r *http.Request) {
	u, ok := middleware.GetSessionUser(r)
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	room, err := h.db.GetRoomByCode(r.Context(), chi.URLParam(r, "code"))
	if err != nil {
		writeError(w, r, http.StatusNotFound, "not_found", "Room not found")
		return
	}
	hostID, _ := uuid.Parse(u.UserID)
	if u.Role != "admin" && (!room.HostID.Valid || room.HostID.Bytes != hostID) {
		writeError(w, r, http.StatusForbidden, "forbidden", "Only the host can kick players")
		return
	}
	var req struct {
		UserID string `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UserID == "" {
		writeError(w, r, http.StatusBadRequest, "bad_request", "user_id is required")
		return
	}
	targetID, err := uuid.Parse(req.UserID)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid user ID")
		return
	}
	if err := h.db.RemoveRoomPlayer(r.Context(), db.RemoveRoomPlayerParams{
		RoomID: room.ID,
		UserID: targetID,
	}); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to kick player")
		return
	}
	// Notify connected WS clients
	if hub, ok := h.manager.Get(room.Code); ok {
		hub.KickPlayer(req.UserID)
	}
	w.WriteHeader(http.StatusNoContent)
}

// Leaderboard handles GET /api/rooms/{code}/leaderboard.
func (h *RoomHandler) Leaderboard(w http.ResponseWriter, r *http.Request) {
	_, ok := middleware.GetSessionUser(r)
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	room, err := h.db.GetRoomByCode(r.Context(), chi.URLParam(r, "code"))
	if err != nil {
		writeError(w, r, http.StatusNotFound, "not_found", "Room not found")
		return
	}
	leaderboard, err := h.db.GetRoomLeaderboard(r.Context(), room.ID)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to get leaderboard")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": leaderboard})
}
