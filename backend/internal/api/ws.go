// backend/internal/api/ws.go
package api

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"

	"github.com/MorganKryze/FabDoYouMeme/backend/internal/config"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/game"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/middleware"
)

// WSHandler handles WS /api/ws/rooms/:code.
type WSHandler struct {
	manager        *game.Manager
	allowedOrigins []string
}

// NewWSHandler takes the pre-normalized allowlist produced by
// config.buildAllowedOrigins. Passing raw strings here is fine too — the
// constructor re-normalizes defensively so tests and main() share one rule.
func NewWSHandler(manager *game.Manager, allowedOrigins []string) *WSHandler {
	normalized := make([]string, 0, len(allowedOrigins))
	for _, o := range allowedOrigins {
		normalized = append(normalized, config.NormalizeOrigin(o))
	}
	return &WSHandler{manager: manager, allowedOrigins: normalized}
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

// ServeHTTP upgrades to WebSocket, authenticates via session cookie,
// looks up the room, and adds the connection to the hub.
func (h *WSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	u, ok := middleware.GetSessionUser(r)
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	roomCode := chi.URLParam(r, "code")
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
	hub.Join(joinCtx, u.UserID, u.Username, conn)
}
