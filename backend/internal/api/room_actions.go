// backend/internal/api/room_actions.go
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
//
// Contract (docs/api.md): "Leave the room (lobby only; host leaving closes
// the room)". Finding 3.E in the 2026-04-10 review pointed out that the
// previous implementation enforced neither the lobby-only gate nor the
// host-closes-room semantics. This version does both so the documented state
// machine is actually observed.
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
	if room.State != "lobby" {
		writeError(w, r, http.StatusConflict, "room_not_in_lobby",
			"Leave is only available while the room is in lobby state")
		return
	}
	userID, _ := uuid.Parse(u.UserID)

	isHost := room.HostID.Valid && room.HostID.Bytes == userID
	if isHost {
		// Host-leaves-closes-room: flip the row to 'finished' before removing
		// players so concurrent joins stop immediately. RemoveRoomPlayer
		// cleanup then drops the host row too.
		if _, err := h.db.SetRoomState(r.Context(), db.SetRoomStateParams{
			ID:    room.ID,
			State: "finished",
		}); err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to close room")
			return
		}
	}
	if err := h.db.RemoveRoomPlayer(r.Context(), db.RemoveRoomPlayerParams{
		RoomID: room.ID,
		UserID: pgtype.UUID{Bytes: userID, Valid: true},
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
		UserID: pgtype.UUID{Bytes: targetID, Valid: true},
	}); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to kick player")
		return
	}
	// Notify connected WS clients. The DB row was already removed above, so
	// failure to enqueue here is recoverable — the player will be treated as
	// a non-member on their next WS message. We still bound the send with
	// the request context to avoid stalling the HTTP goroutine on a saturated
	// hub (finding 4.B).
	if hub, ok := h.manager.Get(room.Code); ok {
		if err := hub.KickPlayer(r.Context(), req.UserID); err != nil && h.log != nil {
			h.log.Warn("kick did not reach hub", "room", room.Code, "user_id", req.UserID, "err", err)
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// End handles POST /api/rooms/{code}/end.
//
// Host or admin can terminate a room in any non-finished state. The DB row
// is flipped to 'finished' before the hub is notified so late joiners hitting
// /join while the hub is tearing down get a clean 409 instead of a stale
// lobby row. See also Leave above which uses the same DB-first ordering for
// the lobby-only host-closes-room path.
func (h *RoomHandler) End(w http.ResponseWriter, r *http.Request) {
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
	isAdmin := u.Role == "admin"
	isHost := room.HostID.Valid && room.HostID.Bytes == hostID
	if !isAdmin && !isHost {
		writeError(w, r, http.StatusForbidden, "forbidden", "Only the host can end the room")
		return
	}
	if room.State == "finished" {
		writeError(w, r, http.StatusConflict, "room_already_finished",
			"Room is already finished")
		return
	}
	if _, err := h.db.SetRoomState(r.Context(), db.SetRoomStateParams{
		ID:    room.ID,
		State: "finished",
	}); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to end room")
		return
	}
	reason := "ended_by_host"
	if isAdmin && !isHost {
		reason = "ended_by_admin"
	}
	if hub, ok := h.manager.Get(room.Code); ok {
		if err := hub.EndRoom(r.Context(), reason); err != nil && h.log != nil {
			h.log.Warn("end room did not reach hub", "room", room.Code, "err", err)
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// Leaderboard handles GET /api/rooms/{code}/leaderboard.
//
// Contract (docs/api.md): "Final leaderboard (finished rooms only)". Finding
// 3.F in the 2026-04-10 review called out that the pre-fix handler returned
// a live score snapshot during a playing round, leaking signal to voters
// mid-round. The 409 matches the error model used elsewhere for state
// mismatches (see UpdateConfig's room_not_in_lobby).
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
	if room.State != "finished" {
		writeError(w, r, http.StatusConflict, "room_not_finished",
			"Leaderboard is only available after the game ends")
		return
	}
	leaderboard, err := h.db.GetRoomLeaderboard(r.Context(), room.ID)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to get leaderboard")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": leaderboard})
}
