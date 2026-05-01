// backend/internal/game/manager.go
package game

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/clock"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/config"
)

// ErrRoomFinished is returned by GetOrLoad when the room exists but is in
// the "finished" state — joining it is never valid and must 404 at the API
// boundary rather than spin up a ghost hub.
var ErrRoomFinished = errors.New("room is finished")

// Manager tracks all active hubs (one per active room).
//
// Hubs are created lazily: the WebSocket upgrade handler calls GetOrLoad,
// which reads the room row from the database and, for any non-finished room,
// constructs a Hub and runs it in a goroutine. The hub goroutine is scoped to
// Manager.serverCtx (NOT the HTTP request context) so that request cancellation
// does not kill the hub, while Manager.Shutdown can cancel every hub in one
// shot by cancelling serverCtx.
type Manager struct {
	mu   sync.RWMutex
	hubs map[string]*Hub // roomCode → Hub

	registry *Registry
	db       *db.Queries
	cfg      *config.Config
	log      *slog.Logger
	clock    clock.Clock

	// serverCtx scopes every hub's Run goroutine to the server's lifetime.
	// Cancelled by Shutdown so all hubs exit their select-loops cleanly.
	serverCtx    context.Context
	serverCancel context.CancelFunc
}

func NewManager(parent context.Context, registry *Registry, queries *db.Queries, cfg *config.Config, log *slog.Logger, clk clock.Clock) *Manager {
	if clk == nil {
		clk = clock.Real{}
	}
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	return &Manager{
		hubs:         make(map[string]*Hub),
		registry:     registry,
		db:           queries,
		cfg:          cfg,
		log:          log,
		clock:        clk,
		serverCtx:    ctx,
		serverCancel: cancel,
	}
}

// GetOrCreate returns the hub for roomCode, creating it if it does not exist.
// roomID, gameTypeSlug, and hostUserID are only used when creating a new hub.
// effectiveMaxPlayers is the per-room cap from rooms.config.max_players;
// pass 0 to fall back to the handler's manifest cap.
//
// The ctx parameter is retained for API compatibility with existing test and
// REST handler call sites, but the hub's Run goroutine is scoped to
// Manager.serverCtx — not to ctx — so a request-scoped cancellation cannot
// inadvertently terminate an in-flight game.
func (m *Manager) GetOrCreate(ctx context.Context, roomCode string, roomID uuid.UUID, gameTypeSlug, hostUserID string, effectiveMaxPlayers int) *Hub {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.createLocked(roomCode, roomID, gameTypeSlug, hostUserID, effectiveMaxPlayers)
}

// createLocked does the actual hub construction; caller must already hold m.mu.
func (m *Manager) createLocked(roomCode string, roomID uuid.UUID, gameTypeSlug, hostUserID string, effectiveMaxPlayers int) *Hub {
	if h, ok := m.hubs[roomCode]; ok {
		return h
	}

	h := NewHub(HubConfig{
		RoomCode:            roomCode,
		RoomID:              roomID,
		GameTypeSlug:        gameTypeSlug,
		HostUserID:          hostUserID,
		EffectiveMaxPlayers: effectiveMaxPlayers,
		Registry:            m.registry,
		DB:                  m.db,
		Cfg:                 m.cfg,
		Log:                 m.log,
		Clock:               m.clock,
	})
	m.hubs[roomCode] = h

	runCtx := m.serverCtx
	go func() {
		h.Run(runCtx)
		m.mu.Lock()
		delete(m.hubs, roomCode)
		m.mu.Unlock()
	}()

	return h
}

// GetOrLoad returns the hub for roomCode, lazy-loading it from the database
// if no goroutine is currently running one. It is the canonical entry point
// from the WebSocket upgrade handler and is what fixes the production 404.
//
// The DB lookup uses the caller's ctx (typically r.Context()) so a request
// cancellation aborts the lookup cleanly, but the hub goroutine itself is
// scoped to Manager.serverCtx inside createLocked. Finished rooms return
// ErrRoomFinished — callers map this to HTTP 404 at the API boundary.
func (m *Manager) GetOrLoad(ctx context.Context, roomCode string) (*Hub, error) {
	// Fast path — hub already running. No DB round trip.
	m.mu.RLock()
	if h, ok := m.hubs[roomCode]; ok {
		m.mu.RUnlock()
		return h, nil
	}
	m.mu.RUnlock()

	// Slow path — hydrate from the rooms table.
	row, err := m.db.GetRoomByCode(ctx, roomCode)
	if err != nil {
		return nil, fmt.Errorf("lookup room %s: %w", roomCode, err)
	}
	if row.State == "finished" {
		return nil, ErrRoomFinished
	}

	hostUserID := ""
	if row.HostID.Valid {
		hostUserID = uuid.UUID(row.HostID.Bytes).String()
	}

	// Decode rooms.config.max_players so the hub can enforce the host's
	// chosen lobby size at join time. A decode error is non-fatal (legacy
	// or hand-edited rows fall back to the manifest cap inside the hub).
	effectiveMaxPlayers := 0
	if len(row.Config) > 0 {
		var cfg struct {
			MaxPlayers int `json:"max_players"`
		}
		if err := json.Unmarshal(row.Config, &cfg); err == nil {
			effectiveMaxPlayers = cfg.MaxPlayers
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	return m.createLocked(row.Code, row.ID, row.GameTypeSlug, hostUserID, effectiveMaxPlayers), nil
}

// Get returns the hub for roomCode, or (nil, false) if no hub is running.
func (m *Manager) Get(roomCode string) (*Hub, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	h, ok := m.hubs[roomCode]
	return h, ok
}

// Registry returns the game type registry.
func (m *Manager) Registry() *Registry {
	return m.registry
}

// ActiveCount returns the number of rooms with an active hub.
func (m *Manager) ActiveCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.hubs)
}

// Shutdown broadcasts "server_restarting" to every active hub, sleeps for one
// second so the messages flush through each hub's writePump, then cancels
// serverCtx — which causes every hub's Run() loop to return and each hub
// goroutine to exit. Without the cancel, the hub goroutines would continue
// running after srv.Shutdown() returned, which was the root cause of 4.C in
// the review ("manager.Shutdown() doesn't cancel hubs, magic sleep").
func (m *Manager) Shutdown() {
	m.mu.RLock()
	hubs := make([]*Hub, 0, len(m.hubs))
	for _, h := range m.hubs {
		hubs = append(hubs, h)
	}
	m.mu.RUnlock()

	for _, h := range hubs {
		h.broadcast(buildMessage("server_restarting", map[string]string{
			"message": "Server is restarting. Please reconnect in a few moments.",
		}))
	}
	m.clock.Sleep(1 * time.Second)
	m.serverCancel()
}
