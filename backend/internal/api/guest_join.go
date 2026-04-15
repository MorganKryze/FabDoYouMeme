// backend/internal/api/guest_join.go
package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/auth"
)

// GuestJoin handles POST /api/rooms/{code}/guest-join.
//
// Pre-auth entry point for players who received a room code via chat/link
// and haven't bothered to create an account. Mints an opaque guest token
// scoped to this one room, inserts a guest_players row, and adds a
// room_players entry with the polymorphic guest_player_id FK.
//
// The raw token is returned exactly once in the response body — only its
// SHA-256 hash is persisted, matching the magic-link token pattern so a
// DB leak cannot be used to impersonate guests.
func (h *RoomHandler) GuestJoin(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")

	var req struct {
		DisplayName string `json:"display_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid JSON")
		return
	}
	req.DisplayName = strings.TrimSpace(req.DisplayName)
	if n := utf8.RuneCountInString(req.DisplayName); n < 1 || n > 32 {
		writeError(w, r, http.StatusBadRequest, "bad_request", "display_name must be 1-32 characters")
		return
	}

	room, err := h.db.GetRoomByCode(r.Context(), code)
	if err != nil {
		writeError(w, r, http.StatusNotFound, "not_found", "Room not found")
		return
	}
	if room.State != "lobby" {
		writeError(w, r, http.StatusConflict, "room_not_in_lobby", "Room is no longer accepting new players")
		return
	}

	rawToken, err := auth.GenerateRawToken()
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to mint guest token")
		return
	}
	tokenHash := auth.HashToken(rawToken)

	guest, err := h.db.CreateGuestPlayer(r.Context(), db.CreateGuestPlayerParams{
		RoomID:      room.ID,
		DisplayName: req.DisplayName,
		TokenHash:   tokenHash,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" &&
			pgErr.ConstraintName == "guest_players_room_id_display_name_key" {
			writeError(w, r, http.StatusConflict, "display_name_taken",
				"That display name is already taken in this room")
			return
		}
		if h.log != nil {
			h.log.Warn("guest_join: create guest player failed",
				"error", err, "room", code)
		}
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to create guest player")
		return
	}

	if _, err := h.db.AddGuestRoomPlayer(r.Context(), db.AddGuestRoomPlayerParams{
		RoomID:        room.ID,
		GuestPlayerID: pgtype.UUID{Bytes: guest.ID, Valid: true},
	}); err != nil {
		if h.log != nil {
			h.log.Warn("guest_join: add guest to room failed",
				"error", err, "room", code, "guest_id", guest.ID.String())
		}
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to add guest to room")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"guest_token":  rawToken,
		"player_id":    guest.ID.String(),
		"display_name": guest.DisplayName,
		"room_code":    room.Code,
	})
}
