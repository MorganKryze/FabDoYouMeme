// backend/internal/game/manager.go
package game

import (
	"context"
	"log/slog"
	"sync"

	"github.com/google/uuid"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/config"
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
