// backend/internal/api/ws.go
package api

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"

	"github.com/MorganKryze/FabDoYouMeme/backend/internal/game"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/middleware"
)

// WSHandler handles WS /api/ws/rooms/:code.
type WSHandler struct {
	manager       *game.Manager
	allowedOrigin string
}

func NewWSHandler(manager *game.Manager, allowedOrigin string) *WSHandler {
	return &WSHandler{manager: manager, allowedOrigin: allowedOrigin}
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
	hub, hubOK := h.manager.Get(roomCode)
	if !hubOK {
		writeError(w, r, http.StatusNotFound, "room_not_found", "No active hub for this room")
		return
	}

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return r.Header.Get("Origin") == h.allowedOrigin
		},
	}
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
