// backend/internal/api/ws.go
package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"

	"github.com/MorganKryze/FabDoYouMeme/backend/internal/game"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/middleware"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// Origin validated by the ALLOWED_ORIGIN middleware in main.go
		return true
	},
}

// WSHandler handles WS /api/ws/rooms/:code.
type WSHandler struct {
	manager *game.Manager
}

func NewWSHandler(manager *game.Manager) *WSHandler {
	return &WSHandler{manager: manager}
}

// ServeHTTP upgrades to WebSocket, authenticates via session cookie,
// looks up the room, and adds the connection to the hub.
func (h *WSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	u, ok := middleware.GetSessionUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	roomCode := chi.URLParam(r, "code")
	hub, hubOK := h.manager.Get(roomCode)
	if !hubOK {
		writeError(w, http.StatusNotFound, "room_not_found", "No active hub for this room")
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		// Upgrade failure is logged by gorilla; response already written
		return
	}

	hub.Join(r.Context(), u.UserID, u.Username, conn)
}
