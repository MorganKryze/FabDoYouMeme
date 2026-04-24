// backend/internal/api/ws.go
package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/auth"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/config"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/game"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/middleware"
)

// WSHandler handles WS /api/ws/rooms/:code.
type WSHandler struct {
	manager        *game.Manager
	queries        *db.Queries
	allowedOrigins []string
}

// NewWSHandler takes the pre-normalized allowlist produced by
// config.buildAllowedOrigins. Passing raw strings here is fine too — the
// constructor re-normalizes defensively so tests and main() share one rule.
// The queries handle may be nil in tests that don't exercise the guest path;
// in that case a guest_token query param will be rejected with an auth error.
func NewWSHandler(manager *game.Manager, queries *db.Queries, allowedOrigins []string) *WSHandler {
	normalized := make([]string, 0, len(allowedOrigins))
	for _, o := range allowedOrigins {
		normalized = append(normalized, config.NormalizeOrigin(o))
	}
	return &WSHandler{manager: manager, queries: queries, allowedOrigins: normalized}
}

// checkOrigin is the exact policy used by the upgrader. Extracted as a
// method so tests can exercise it without spinning up an HTTP server.
// Finding 5.C in the 2026-04-10 review: the pre-fix version compared the
// raw Origin header against a single string, so `https://app.example.com/`
// vs `https://app.example.com` silently failed.
func (h *WSHandler) checkOrigin(r *http.Request) bool {
	origin := config.NormalizeOrigin(r.Header.Get("Origin"))
	for _, a := range h.allowedOrigins {
		if origin == a {
			return true
		}
	}
	return false
}

// ServeHTTP upgrades to WebSocket, resolves the caller's identity (registered
// user via session cookie OR guest via ?guest_token=), looks up the room, and
// adds the connection to the hub. Guests must have obtained their token via
// POST /api/rooms/{code}/guest-join — the token is validated against the
// guest_players row and must be scoped to the same room.
func (h *WSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	roomCode := chi.URLParam(r, "code")

	ident, err := h.resolveIdentity(r, roomCode)
	if err != nil {
		writeError(w, r, http.StatusUnauthorized, "unauthorized", err.Error())
		return
	}

	// Single-room enforcement for registered users: reject with a plain JSON
	// 409 before the upgrade so the SvelteKit join form can surface it.
	// Guests are already scoped to one room by token mint (see guest_join.go).
	if !ident.IsGuest && h.queries != nil {
		if uid, parseErr := uuid.Parse(ident.ID); parseErr == nil {
			active, aerr := h.queries.GetActiveRoomForUser(r.Context(), pgtype.UUID{Bytes: uid, Valid: true})
			if aerr == nil && active.Code != roomCode {
				writeError(w, r, http.StatusConflict, "already_in_active_room",
					"You are already in room "+active.Code)
				return
			}
			if aerr != nil && !errors.Is(aerr, pgx.ErrNoRows) {
				writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to check active room")
				return
			}
		}
	}

	// Phase 4 (groups) — group-scoped room gate. When rooms.group_id is set,
	// guests are rejected outright (no platform account) and registered
	// users must be a live member of the group. Mirrors the ban-gate shape
	// below: runs before the upgrade so the JSON error reaches the client.
	if h.queries != nil {
		room, rerr := h.queries.GetRoomByCode(r.Context(), roomCode)
		if rerr == nil && room.GroupID.Valid {
			if ident.IsGuest {
				writeError(w, r, http.StatusForbidden, "group_scoped_room_requires_account",
					"This room is restricted to registered group members.")
				return
			}
			if uid, perr := uuid.Parse(ident.ID); perr == nil {
				if _, merr := h.queries.GetMembership(r.Context(), db.GetMembershipParams{
					GroupID: room.GroupID.Bytes, UserID: uid,
				}); merr != nil {
					writeError(w, r, http.StatusForbidden, "not_group_member",
						"You are not a member of this group.")
					return
				}
			}
		}
	}

	// Ban gate. Runs after identity resolution and the single-room check so a
	// banned identity sees the same "you can't be here" signal as a legitimate
	// mismatch would. Placed BEFORE the upgrade so the JSON error reaches the
	// join form (mirrors the already_in_active_room pattern above).
	if h.queries != nil {
		banRoom, bErr := h.queries.GetRoomByCode(r.Context(), roomCode)
		if bErr == nil {
			if ident.IsGuest {
				if gpUUID, perr := uuid.Parse(ident.ID); perr == nil {
					banned, berr := h.queries.IsGuestBannedFromRoom(r.Context(), db.IsGuestBannedFromRoomParams{
						RoomID:        banRoom.ID,
						GuestPlayerID: pgtype.UUID{Bytes: gpUUID, Valid: true},
					})
					if berr == nil && banned {
						writeError(w, r, http.StatusConflict, "banned_from_room",
							"You were removed from this room")
						return
					}
				}
			} else {
				if uUUID, perr := uuid.Parse(ident.ID); perr == nil {
					banned, berr := h.queries.IsUserBannedFromRoom(r.Context(), db.IsUserBannedFromRoomParams{
						RoomID: banRoom.ID,
						UserID: pgtype.UUID{Bytes: uUUID, Valid: true},
					})
					if berr == nil && banned {
						writeError(w, r, http.StatusConflict, "banned_from_room",
							"You were removed from this room")
						return
					}
				}
			}
		}
	}

	hub, err := h.manager.GetOrLoad(r.Context(), roomCode)
	if err != nil {
		// Room lookup failed OR the room exists but is finished. Both map to
		// 404 at the HTTP layer — we do not leak the distinction between
		// "never existed" and "already ended" to unauthenticated observers.
		writeError(w, r, http.StatusNotFound, "room_not_found", "No active hub for this room")
		return
	}

	upgrader := websocket.Upgrader{CheckOrigin: h.checkOrigin}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		// Upgrade failure is logged by gorilla; response already written
		return
	}

	// 5s timeout for hub join — prevents slow hubs from blocking goroutines.
	joinCtx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	hub.Join(joinCtx, ident, conn)
}

// resolveIdentity returns a PlayerIdentity for the caller, consulting the
// session user first and falling back to a guest token. The guest token must
// match a guest_players row scoped to the requested room.
func (h *WSHandler) resolveIdentity(r *http.Request, roomCode string) (game.PlayerIdentity, error) {
	if u, ok := middleware.GetSessionUser(r); ok {
		return game.PlayerIdentity{
			ID:          u.UserID,
			DisplayName: u.Username,
			IsGuest:     false,
		}, nil
	}

	token := r.URL.Query().Get("guest_token")
	if token == "" {
		return game.PlayerIdentity{}, errAuthRequired
	}
	if h.queries == nil {
		return game.PlayerIdentity{}, errAuthRequired
	}

	hash := auth.HashToken(token)
	gp, err := h.queries.GetGuestPlayerByTokenHash(r.Context(), hash)
	if err != nil {
		return game.PlayerIdentity{}, errAuthRequired
	}

	// Cross-room token replay protection: the guest_players row is scoped to
	// a single room at mint time. The WS handshake must match that room.
	room, err := h.queries.GetRoomByCode(r.Context(), roomCode)
	if err != nil || room.ID != gp.RoomID {
		return game.PlayerIdentity{}, errAuthRequired
	}

	return game.PlayerIdentity{
		ID:          gp.ID.String(),
		DisplayName: gp.DisplayName,
		IsGuest:     true,
	}, nil
}

// errAuthRequired is a stable sentinel so the handler returns a single
// opaque error message regardless of which gate (cookie, token, room scope)
// actually failed. Leaking the distinction would help attackers enumerate
// live guest sessions.
var errAuthRequired = wsAuthError("Authentication required")

type wsAuthError string

func (e wsAuthError) Error() string { return string(e) }
