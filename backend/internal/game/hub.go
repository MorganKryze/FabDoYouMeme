// backend/internal/game/hub.go
package game

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/config"
)

// HubState tracks in-memory room lifecycle.
type HubState string

const (
	HubLobby   HubState = "lobby"
	HubPlaying HubState = "playing"
)

// Hub manages one room's WebSocket connections and game lifecycle.
// All state mutations happen in the main Run() goroutine — no locking needed
// for internal state. Incoming connections are registered via a channel.
type Hub struct {
	roomCode     string
	roomID       uuid.UUID
	gameTypeSlug string
	hostUserID   string

	registry *Registry
	db       *db.Queries
	cfg      *config.Config
	log      *slog.Logger

	state   HubState
	players map[string]*connectedPlayer // userID → player

	register   chan *connectedPlayer
	unregister chan *connectedPlayer
	incoming   chan playerMessage

	mu sync.Mutex // only for accessing hub from outside Run()
}

// connectedPlayer is hub-internal player state.
type connectedPlayer struct {
	userID       string
	username     string
	conn         *websocket.Conn
	send         chan []byte
	reconnecting bool
	graceTimer   *time.Timer
}

// playerMessage is a message arriving from a player connection.
type playerMessage struct {
	player  *connectedPlayer
	msgType string
	data    json.RawMessage
}

// HubConfig groups the dependencies Hub needs.
type HubConfig struct {
	RoomCode     string
	RoomID       uuid.UUID
	GameTypeSlug string
	HostUserID   string
	Registry     *Registry
	DB           *db.Queries
	Cfg          *config.Config
	Log          *slog.Logger
}

// NewHub creates a Hub but does not start it. Call hub.Run() in a goroutine.
func NewHub(hc HubConfig) *Hub {
	return &Hub{
		roomCode:     hc.RoomCode,
		roomID:       hc.RoomID,
		gameTypeSlug: hc.GameTypeSlug,
		hostUserID:   hc.HostUserID,
		registry:     hc.Registry,
		db:           hc.DB,
		cfg:          hc.Cfg,
		log:          hc.Log,
		state:        HubLobby,
		players:      make(map[string]*connectedPlayer),
		register:     make(chan *connectedPlayer, 8),
		unregister:   make(chan *connectedPlayer, 8),
		incoming:     make(chan playerMessage, 64),
	}
}

// Join is called from the HTTP handler (outside Run goroutine) to add a new WS connection.
// It blocks until the player is registered or the context is cancelled.
func (h *Hub) Join(ctx context.Context, userID, username string, conn *websocket.Conn) {
	p := &connectedPlayer{
		userID:   userID,
		username: username,
		conn:     conn,
		send:     make(chan []byte, 64),
	}
	select {
	case h.register <- p:
	case <-ctx.Done():
		conn.Close()
	}
}

// Run is the hub's main event loop. Call in a goroutine; it exits when the room ends.
func (h *Hub) Run(ctx context.Context) {
	h.log.Info("hub started", "room", h.roomCode)
	defer h.log.Info("hub stopped", "room", h.roomCode)

	for {
		select {
		case <-ctx.Done():
			h.broadcast(buildMessage("game_ended", map[string]string{"reason": "server_shutdown"}))
			return

		case p := <-h.register:
			h.handleRegister(p)

		case p := <-h.unregister:
			h.handleUnregister(ctx, p)

		case msg := <-h.incoming:
			h.handleMessage(ctx, msg)
		}
	}
}

func (h *Hub) handleRegister(p *connectedPlayer) {
	existing, reconnecting := h.players[p.userID]
	if reconnecting && existing.reconnecting {
		// Player reconnecting within grace window
		if existing.graceTimer != nil {
			existing.graceTimer.Stop()
		}
		existing.conn = p.conn
		existing.send = p.send
		existing.reconnecting = false
		h.sendTo(existing, buildMessage("room_state", h.buildRoomState()))
		h.broadcast(buildMessage("player_joined", map[string]string{
			"user_id": p.userID, "username": p.username,
		}))
		go h.readPump(existing)
		go h.writePump(existing)
		return
	}

	if h.state != HubLobby {
		writeWS(p.conn, buildMessage("error", map[string]string{
			"code": "game_already_started", "message": "Game is already in progress",
		}))
		p.conn.Close()
		return
	}

	h.players[p.userID] = p
	h.broadcast(buildMessage("player_joined", map[string]string{
		"user_id": p.userID, "username": p.username,
	}))
	go h.readPump(p)
	go h.writePump(p)
}

func (h *Hub) handleUnregister(ctx context.Context, p *connectedPlayer) {
	if _, ok := h.players[p.userID]; !ok {
		return
	}
	p.reconnecting = true
	h.broadcast(buildMessage("reconnecting", map[string]string{
		"user_id": p.userID, "username": p.username,
	}))
	// Start grace window timer
	p.graceTimer = time.AfterFunc(h.cfg.ReconnectGraceWindow, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if cp, ok := h.players[p.userID]; ok && cp.reconnecting {
			delete(h.players, p.userID)
			h.broadcastLocked(buildMessage("player_left", map[string]string{
				"user_id": p.userID, "username": p.username,
			}))
			if p.userID == h.hostUserID && h.state == HubPlaying {
				h.finishRoom(ctx, "host_disconnected")
			}
		}
	})
}

func (h *Hub) handleMessage(ctx context.Context, msg playerMessage) {
	switch msg.msgType {
	case "start":
		if msg.player.userID != h.hostUserID {
			h.sendTo(msg.player, buildMessage("error", map[string]string{
				"code": "not_host", "message": "Only the host can start the game",
			}))
			return
		}
		h.startGame(ctx)

	case "ping":
		h.sendTo(msg.player, buildMessage("pong", nil))

	default:
		// Game-type-specific messages are prefixed with the slug
		expected := h.gameTypeSlug + ":"
		if len(msg.msgType) > len(expected) && msg.msgType[:len(expected)] == expected {
			h.handleGameMessage(ctx, msg)
		} else {
			h.sendTo(msg.player, buildMessage("error", map[string]string{
				"code": "unknown_message_type", "message": "Unknown message type",
			}))
		}
	}
}

func (h *Hub) startGame(ctx context.Context) {
	if h.state != HubLobby {
		return
	}
	h.state = HubPlaying
	if _, err := h.db.SetRoomState(ctx, db.SetRoomStateParams{
		ID: h.roomID, State: "playing",
	}); err != nil {
		h.log.Error("hub: set room state playing", "error", err)
	}
	h.broadcast(buildMessage("game_started", map[string]any{
		"player_count": len(h.players),
	}))
	go h.runRounds(ctx)
}

func (h *Hub) runRounds(_ context.Context) {
	// Rounds loop — simplified; full implementation in game type handler
	// See design/04-protocol.md for complete round lifecycle
}

func (h *Hub) finishRoom(ctx context.Context, reason string) {
	if _, err := h.db.SetRoomState(ctx, db.SetRoomStateParams{
		ID: h.roomID, State: "finished",
	}); err != nil {
		h.log.Error("hub: set room state finished", "error", err)
	}
	h.broadcast(buildMessage("game_ended", map[string]string{"reason": reason}))
}

func (h *Hub) handleGameMessage(ctx context.Context, msg playerMessage) {
	handler, ok := h.registry.Get(h.gameTypeSlug)
	if !ok {
		h.sendTo(msg.player, buildMessage("error", map[string]string{
			"code": "unknown_game_type",
		}))
		return
	}
	_ = handler
	// Dispatch to handler methods based on msg.msgType suffix (submit/vote)
	// Full implementation: Phase 7 wires this with the round and submission DB calls
}

// readPump reads messages from a player's connection and forwards them to the hub.
func (h *Hub) readPump(p *connectedPlayer) {
	defer func() { h.unregister <- p }()
	p.conn.SetReadLimit(h.cfg.WSReadLimitBytes)

	for {
		_, raw, err := p.conn.ReadMessage()
		if err != nil {
			return
		}
		var m Message
		if err := json.Unmarshal(raw, &m); err != nil {
			continue
		}
		h.incoming <- playerMessage{player: p, msgType: m.Type, data: m.Data}
	}
}

// writePump drains the player's send channel to the WebSocket connection.
func (h *Hub) writePump(p *connectedPlayer) {
	pingTicker := time.NewTicker(h.cfg.WSPingInterval)
	defer func() {
		pingTicker.Stop()
		p.conn.Close()
	}()
	for {
		select {
		case msg, ok := <-p.send:
			if !ok {
				p.conn.WriteMessage(websocket.CloseMessage, nil)
				return
			}
			if err := p.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-pingTicker.C:
			if err := p.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (h *Hub) broadcast(msg []byte) {
	for _, p := range h.players {
		if !p.reconnecting {
			select {
			case p.send <- msg:
			default:
				// Slow consumer — drop message
			}
		}
	}
}

func (h *Hub) broadcastLocked(msg []byte) {
	h.broadcast(msg) // mu is held by caller
}

func (h *Hub) sendTo(p *connectedPlayer, msg []byte) {
	select {
	case p.send <- msg:
	default:
	}
}

func (h *Hub) buildRoomState() map[string]any {
	players := make([]map[string]string, 0, len(h.players))
	for _, p := range h.players {
		players = append(players, map[string]string{
			"user_id": p.userID, "username": p.username,
		})
	}
	return map[string]any{
		"state":   string(h.state),
		"players": players,
	}
}

func buildMessage(msgType string, data any) []byte {
	payload := map[string]any{"type": msgType}
	if data != nil {
		payload["data"] = data
	}
	b, _ := json.Marshal(payload)
	return b
}

func writeWS(conn *websocket.Conn, msg []byte) {
	conn.WriteMessage(websocket.TextMessage, msg) //nolint:errcheck
}
