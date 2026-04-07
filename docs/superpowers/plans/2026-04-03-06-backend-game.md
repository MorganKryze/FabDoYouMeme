# Backend — Game Engine (Registry, Hub, Meme-Caption) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the game type handler interface, the handler registry, the WebSocket hub (per-room goroutine managing lifecycle), and the `meme-caption` game type handler.

**Architecture:** `internal/game/handler.go` — `GameTypeHandler` interface. `internal/game/registry.go` — `Register()` + `Dispatch()`. `internal/game/hub.go` — WebSocket hub goroutine managing one room: join/reconnect/start/submit/vote/timer/disconnect. `internal/game/types/meme_caption/` — `meme-caption` implementation.

**Tech Stack:** `gorilla/websocket`, `pgx/v5`, sqlc queries, `google/uuid`.

**Prerequisite:** Phase 3 complete (config, middleware). Phase 2 complete (sqlc queries available).

---

### Task 1: GameTypeHandler interface + registry

**Files:**

- Create: `backend/internal/game/handler.go`
- Create: `backend/internal/game/registry.go`
- Create: `backend/internal/game/registry_test.go`

- [ ] **Step 1: Write the registry test**

```go
// backend/internal/game/registry_test.go
package game_test

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"github.com/MorganKryze/FabDoYouMeme/backend/internal/game"
)

// stubHandler is a minimal GameTypeHandler for registry tests.
type stubHandler struct{ slug string }

func (s *stubHandler) Slug() string                          { return s.slug }
func (s *stubHandler) SupportedPayloadVersions() []int       { return []int{1} }
func (s *stubHandler) SupportsSolo() bool                    { return false }
func (s *stubHandler) ValidateSubmission(_ game.Round, _ json.RawMessage) error { return nil }
func (s *stubHandler) ValidateVote(_ game.Round, _ game.Submission, _ uuid.UUID, _ json.RawMessage) error {
	return nil
}
func (s *stubHandler) CalculateRoundScores(_ []game.Submission, _ []game.Vote) map[uuid.UUID]int {
	return nil
}
func (s *stubHandler) BuildSubmissionsShownPayload(_ []game.Submission) (json.RawMessage, error) {
	return json.RawMessage(`{}`), nil
}
func (s *stubHandler) BuildVoteResultsPayload(_ []game.Submission, _ []game.Vote, _ map[uuid.UUID]int) (json.RawMessage, error) {
	return json.RawMessage(`{}`), nil
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := game.NewRegistry()
	r.Register(&stubHandler{slug: "test-game"})
	h, ok := r.Get("test-game")
	if !ok {
		t.Fatal("expected handler to be registered")
	}
	if h.Slug() != "test-game" {
		t.Errorf("wrong slug: %s", h.Slug())
	}
}

func TestRegistry_DuplicatePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on duplicate slug registration")
		}
	}()
	r := game.NewRegistry()
	r.Register(&stubHandler{slug: "dup"})
	r.Register(&stubHandler{slug: "dup"}) // must panic
}

func TestRegistry_GetMissing(t *testing.T) {
	r := game.NewRegistry()
	_, ok := r.Get("nonexistent")
	if ok {
		t.Error("expected ok=false for unregistered slug")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd backend && go test ./internal/game/... -run TestRegistry -v
```

Expected: compile error (package does not exist).

- [ ] **Step 3: Write `handler.go` — GameTypeHandler interface**

```go
// backend/internal/game/handler.go
package game

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Round is the data the hub provides to handler methods per round.
type Round struct {
	ID                 uuid.UUID
	RoomID             uuid.UUID
	ItemID             uuid.UUID
	RoundNumber        int
	StartedAt          *time.Time
	EndedAt            *time.Time
	ItemPayload        json.RawMessage
	ItemPayloadVersion int
}

// Submission is a player's answer for a round.
type Submission struct {
	ID      uuid.UUID
	UserID  uuid.UUID
	Payload json.RawMessage
}

// Vote is a player's vote for a submission.
type Vote struct {
	SubmissionID uuid.UUID
	VoterID      uuid.UUID
	Value        json.RawMessage
}

// GameTypeHandler is the interface every game type must implement.
// The hub calls these methods during gameplay; implementations must be safe for
// concurrent calls from the hub goroutine only (no additional locking needed
// since the hub is single-threaded per room).
type GameTypeHandler interface {
	// Slug returns the game type slug matching game_types.slug (e.g. "meme-caption").
	Slug() string

	// SupportedPayloadVersions returns which item payload versions this handler can process.
	SupportedPayloadVersions() []int

	// SupportsSolo returns true if solo mode (single player) is supported.
	SupportsSolo() bool

	// ValidateSubmission checks that the submission payload is valid for this game type and round.
	ValidateSubmission(round Round, payload json.RawMessage) error

	// ValidateVote checks that the vote payload is valid.
	// Hub has already verified: (a) voting phase open, (b) no duplicate vote.
	// Handler must additionally verify: voterID != submission.UserID (self-vote).
	ValidateVote(round Round, submission Submission, voterID uuid.UUID, payload json.RawMessage) error

	// CalculateRoundScores aggregates votes into per-user point awards.
	CalculateRoundScores(submissions []Submission, votes []Vote) map[uuid.UUID]int

	// BuildSubmissionsShownPayload returns data for the {slug}:submissions_shown event.
	BuildSubmissionsShownPayload(submissions []Submission) (json.RawMessage, error)

	// BuildVoteResultsPayload returns data for the {slug}:vote_results event.
	BuildVoteResultsPayload(submissions []Submission, votes []Vote, scores map[uuid.UUID]int) (json.RawMessage, error)
}
```

- [ ] **Step 4: Write `registry.go`**

```go
// backend/internal/game/registry.go
package game

import "fmt"

// Registry maps game type slugs to their handlers.
// Use NewRegistry() in main.go; call Register() for each game type.
type Registry struct {
	handlers map[string]GameTypeHandler
}

func NewRegistry() *Registry {
	return &Registry{handlers: make(map[string]GameTypeHandler)}
}

// Register adds a handler. Panics on duplicate slug (caught at startup, never at runtime).
func (r *Registry) Register(h GameTypeHandler) {
	if _, exists := r.handlers[h.Slug()]; exists {
		panic(fmt.Sprintf("game: duplicate handler for slug %q", h.Slug()))
	}
	r.handlers[h.Slug()] = h
}

// Get returns the handler for slug, or (nil, false) if not registered.
func (r *Registry) Get(slug string) (GameTypeHandler, bool) {
	h, ok := r.handlers[slug]
	return h, ok
}
```

- [ ] **Step 5: Run tests**

```bash
cd backend && go test ./internal/game/... -run TestRegistry -v
```

Expected: all `PASS`.

---

### Task 2: WebSocket hub

**Files:**

- Create: `backend/internal/game/hub.go`
- Create: `backend/internal/game/message.go`

- [ ] **Step 1: Write `message.go` — WS message types**

```go
// backend/internal/game/message.go
package game

import "encoding/json"

// Message is the envelope for all WebSocket messages.
type Message struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}
```

> **Deviation (implemented):** `Player` and `WSConn` were removed. The hub uses `*websocket.Conn` directly via `connectedPlayer` in `hub.go`, so `WSConn` was unused. The `WSConn` interface definition also contained invalid Go syntax (`SetReadDeadline(t interface{ ...interface{} }) error`) that would not compile.

- [ ] **Step 2: Write `hub.go`**

```go
// backend/internal/game/hub.go
package game

import (
	"context"
	"encoding/json"
	"fmt"
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
	roomCode    string
	roomID      uuid.UUID
	gameTypeSlug string
	hostUserID  string

	registry *Registry
	db       *db.Queries
	// Note: pool field removed — hub only needs *db.Queries for the queries it calls.
	cfg *config.Config
	log *slog.Logger

	state   HubState
	players map[string]*connectedPlayer // userID → player

	register   chan *connectedPlayer
	unregister chan *connectedPlayer
	incoming   chan playerMessage

	mu sync.Mutex // only for accessing hub from outside Run()
}

// connectedPlayer is hub-internal player state.
type connectedPlayer struct {
	userID      string
	username    string
	conn        *websocket.Conn
	send        chan []byte
	reconnecting bool
	graceTimer  *time.Timer
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

func (h *Hub) runRounds(ctx context.Context) {
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
	conn.WriteMessage(websocket.TextMessage, msg)
}

```

> **Deviation (implemented):** `setRoomStateAdapter` type alias removed — `db.SetRoomStateParams` is used directly. The alias was unnecessary once the actual sqlc output was confirmed.

- [ ] **Step 3: Build check**

```bash
cd backend && go build ./internal/game/...
```

Expected: no errors.

---

### Task 3: Hub manager (multiple rooms)

**Files:**

- Create: `backend/internal/game/manager.go`

- [ ] **Step 1: Write `manager.go`**

```go
// backend/internal/game/manager.go
package game

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/config"
	"log/slog"
)

// Manager tracks all active hubs (one per active room).
// The REST API creates a hub when a WS connection arrives for a room that has no hub.
type Manager struct {
	mu       sync.RWMutex
	hubs     map[string]*Hub // roomCode → Hub
	registry *Registry
	db       *db.Queries
	cfg      *config.Config
	log      *slog.Logger
}

func NewManager(registry *Registry, queries *db.Queries, cfg *config.Config, log *slog.Logger) *Manager {
	return &Manager{
		hubs:     make(map[string]*Hub),
		registry: registry,
		db:       queries,
		cfg:      cfg,
		log:      log,
	}
}

// GetOrCreate returns the hub for roomCode, creating it if it does not exist.
// roomID, gameTypeSlug, and hostUserID are only used when creating a new hub.
func (m *Manager) GetOrCreate(ctx context.Context, roomCode string, roomID uuid.UUID, gameTypeSlug, hostUserID string) *Hub {
	m.mu.Lock()
	defer m.mu.Unlock()

	if h, ok := m.hubs[roomCode]; ok {
		return h
	}

	h := NewHub(HubConfig{
		RoomCode:     roomCode,
		RoomID:       roomID,
		GameTypeSlug: gameTypeSlug,
		HostUserID:   hostUserID,
		Registry:     m.registry,
		DB:           m.db,
		Cfg:          m.cfg,
		Log:          m.log,
	})
	m.hubs[roomCode] = h

	go func() {
		h.Run(ctx)
		m.mu.Lock()
		delete(m.hubs, roomCode)
		m.mu.Unlock()
	}()

	return h
}

// Get returns the hub for roomCode, or (nil, false) if no hub is running.
func (m *Manager) Get(roomCode string) (*Hub, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	h, ok := m.hubs[roomCode]
	return h, ok
}

// ActiveCount returns the number of rooms with an active hub.
func (m *Manager) ActiveCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.hubs)
}

// Ensure Manager compiles with its fmt dependency
var _ = fmt.Sprintf
```

- [ ] **Step 2: Build check**

```bash
cd backend && go build ./internal/game/...
```

Expected: no errors.

---

### Task 4: Meme-caption game type handler

**Files:**

- Create: `backend/internal/game/types/meme_caption/handler.go`
- Create: `backend/internal/game/types/meme_caption/handler_test.go`

- [ ] **Step 1: Write the tests**

```go
// backend/internal/game/types/meme_caption/handler_test.go
package memecaption_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/MorganKryze/FabDoYouMeme/backend/internal/game"
	memecaption "github.com/MorganKryze/FabDoYouMeme/backend/internal/game/types/meme_caption"
)

func newHandler() *memecaption.Handler {
	return memecaption.New()
}

func TestSlug(t *testing.T) {
	if newHandler().Slug() != "meme-caption" {
		t.Error("expected slug 'meme-caption'")
	}
}

func TestSupportedPayloadVersions(t *testing.T) {
	versions := newHandler().SupportedPayloadVersions()
	if len(versions) == 0 {
		t.Error("expected at least one supported payload version")
	}
	found := false
	for _, v := range versions {
		if v == 1 {
			found = true
		}
	}
	if !found {
		t.Error("expected version 1 to be supported")
	}
}

func TestValidateSubmission_TooLong(t *testing.T) {
	h := newHandler()
	long := strings.Repeat("a", 301)
	payload, _ := json.Marshal(map[string]string{"caption": long})
	err := h.ValidateSubmission(game.Round{}, payload)
	if err == nil {
		t.Error("expected error for caption > 300 chars")
	}
}

func TestValidateSubmission_Empty(t *testing.T) {
	h := newHandler()
	payload, _ := json.Marshal(map[string]string{"caption": ""})
	err := h.ValidateSubmission(game.Round{}, payload)
	if err == nil {
		t.Error("expected error for empty caption")
	}
}

func TestValidateSubmission_OK(t *testing.T) {
	h := newHandler()
	payload, _ := json.Marshal(map[string]string{"caption": "This is funny"})
	if err := h.ValidateSubmission(game.Round{}, payload); err != nil {
		t.Errorf("expected no error: %v", err)
	}
}

func TestValidateVote_SelfVote(t *testing.T) {
	h := newHandler()
	voterID := uuid.New()
	submission := game.Submission{UserID: voterID}
	err := h.ValidateVote(game.Round{}, submission, voterID, json.RawMessage(`{}`))
	if err == nil {
		t.Error("expected error for self-vote")
	}
	if !errors.Is(err, memecaption.ErrSelfVote) {
		t.Errorf("expected ErrSelfVote, got %v", err)
	}
}

func TestValidateVote_OK(t *testing.T) {
	h := newHandler()
	voterID := uuid.New()
	authorID := uuid.New()
	submission := game.Submission{UserID: authorID}
	if err := h.ValidateVote(game.Round{}, submission, voterID, json.RawMessage(`{}`)); err != nil {
		t.Errorf("expected no error: %v", err)
	}
}

func TestCalculateRoundScores_OneVotePerSubmission(t *testing.T) {
	h := newHandler()
	authorA := uuid.New()
	authorB := uuid.New()
	voter1 := uuid.New()
	voter2 := uuid.New()
	subA := game.Submission{ID: uuid.New(), UserID: authorA}
	subB := game.Submission{ID: uuid.New(), UserID: authorB}
	votes := []game.Vote{
		{SubmissionID: subA.ID, VoterID: voter1},
		{SubmissionID: subA.ID, VoterID: voter2},
		{SubmissionID: subB.ID, VoterID: authorA}, // authorA votes for authorB
	}
	scores := h.CalculateRoundScores([]game.Submission{subA, subB}, votes)
	if scores[authorA] != 2 {
		t.Errorf("authorA should have 2 votes, got %d", scores[authorA])
	}
	if scores[authorB] != 1 {
		t.Errorf("authorB should have 1 vote, got %d", scores[authorB])
	}
}

func TestBuildSubmissionsShownPayload_HidesAuthors(t *testing.T) {
	h := newHandler()
	subs := []game.Submission{
		{ID: uuid.New(), UserID: uuid.New(), Payload: json.RawMessage(`{"caption":"funny"}`)},
	}
	payload, err := h.BuildSubmissionsShownPayload(subs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(string(payload), "author") {
		t.Error("submissions_shown payload should not reveal author information")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd backend && go test ./internal/game/types/meme_caption/... -v
```

Expected: compile error (package does not exist).

- [ ] **Step 3: Implement `handler.go`**

```go
// backend/internal/game/types/meme_caption/handler.go
package memecaption

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/MorganKryze/FabDoYouMeme/backend/internal/game"
)

// ErrSelfVote is returned when a player tries to vote for their own submission.
var ErrSelfVote = errors.New("cannot_vote_for_self")

// Handler implements game.GameTypeHandler for the meme-caption game type.
type Handler struct{}

func New() *Handler { return &Handler{} }

func (h *Handler) Slug() string                    { return "meme-caption" }
func (h *Handler) SupportedPayloadVersions() []int { return []int{1} }
func (h *Handler) SupportsSolo() bool              { return false }

type submitPayload struct {
	Caption string `json:"caption"`
}

// ValidateSubmission checks caption is non-empty and ≤300 chars.
func (h *Handler) ValidateSubmission(_ game.Round, raw json.RawMessage) error {
	var p submitPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("invalid submission payload: %w", err)
	}
	p.Caption = strings.TrimSpace(p.Caption)
	if p.Caption == "" {
		return fmt.Errorf("caption cannot be empty")
	}
	if len([]rune(p.Caption)) > 300 {
		return fmt.Errorf("caption exceeds 300 characters")
	}
	return nil
}

// ValidateVote prevents self-vote. The vote payload for meme-caption is { submission_id }.
func (h *Handler) ValidateVote(_ game.Round, submission game.Submission, voterID uuid.UUID, _ json.RawMessage) error {
	if submission.UserID == voterID {
		return ErrSelfVote
	}
	return nil
}

// CalculateRoundScores awards one point per vote received.
// Tied submissions each receive full points — no tiebreaker.
func (h *Handler) CalculateRoundScores(submissions []game.Submission, votes []game.Vote) map[uuid.UUID]int {
	// Map submission ID → author user ID
	authorBySubmission := make(map[uuid.UUID]uuid.UUID, len(submissions))
	for _, s := range submissions {
		authorBySubmission[s.ID] = s.UserID
	}

	scores := make(map[uuid.UUID]int)
	for _, v := range votes {
		if authorID, ok := authorBySubmission[v.SubmissionID]; ok {
			scores[authorID]++
		}
	}
	return scores
}

type submissionShown struct {
	ID      string `json:"id"`
	Caption string `json:"caption"`
	// Note: no author fields — authors are hidden during voting phase
}

// BuildSubmissionsShownPayload returns captions without author information.
func (h *Handler) BuildSubmissionsShownPayload(submissions []game.Submission) (json.RawMessage, error) {
	shown := make([]submissionShown, 0, len(submissions))
	for _, s := range submissions {
		var p submitPayload
		if err := json.Unmarshal(s.Payload, &p); err != nil {
			continue
		}
		shown = append(shown, submissionShown{
			ID:      s.ID.String(),
			Caption: strings.TrimSpace(p.Caption),
		})
	}
	payload, err := json.Marshal(map[string]any{"submissions": shown})
	return json.RawMessage(payload), err
}

type submissionResult struct {
	ID            string `json:"id"`
	Caption       string `json:"caption"`
	AuthorUsername string `json:"author_username,omitempty"` // revealed after voting
	VotesReceived int    `json:"votes_received"`
	PointsAwarded int    `json:"points_awarded"`
}

// BuildVoteResultsPayload reveals authors and scores after voting closes.
// Note: author username is not available in the Submission struct (only UserID).
// The hub must pass username-enriched submissions; for now we use user IDs.
// Phase 7 hub wiring enriches this with usernames from the DB.
func (h *Handler) BuildVoteResultsPayload(
	submissions []game.Submission,
	votes []game.Vote,
	scores map[uuid.UUID]int,
) (json.RawMessage, error) {
	// Count votes per submission
	votesPerSub := make(map[uuid.UUID]int)
	for _, v := range votes {
		votesPerSub[v.SubmissionID]++
	}

	results := make([]submissionResult, 0, len(submissions))
	for _, s := range submissions {
		var p submitPayload
		json.Unmarshal(s.Payload, &p)
		results = append(results, submissionResult{
			ID:            s.ID.String(),
			Caption:       strings.TrimSpace(p.Caption),
			VotesReceived: votesPerSub[s.ID],
			PointsAwarded: scores[s.UserID],
		})
	}

	// Round scores list
	roundScores := make([]map[string]any, 0, len(scores))
	for userID, pts := range scores {
		roundScores = append(roundScores, map[string]any{
			"user_id": userID.String(),
			"points":  pts,
		})
	}

	payload, err := json.Marshal(map[string]any{
		"submissions":  results,
		"round_scores": roundScores,
	})
	return json.RawMessage(payload), err
}

// Compile-time interface check
var _ game.GameTypeHandler = (*Handler)(nil)
```

- [ ] **Step 4: Run tests**

```bash
cd backend && go test ./internal/game/types/meme_caption/... -v
```

Expected: all `PASS`.

- [ ] **Step 5: Full game package build**

```bash
cd backend && go build ./internal/game/...
```

Expected: no errors.

---

### Verification

```bash
cd backend && go test ./internal/game/... -v
cd backend && go build ./...
```

Expected: all tests pass, build succeeds.

Mark phase 6 complete in `docs/implementation-status.md`.
