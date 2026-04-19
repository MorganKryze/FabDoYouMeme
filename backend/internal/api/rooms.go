// backend/internal/api/rooms.go
package api

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/config"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/game"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/middleware"
)

// RoomHandler handles /api/rooms/* routes.
type RoomHandler struct {
	db      *db.Queries
	cfg     *config.Config
	manager *game.Manager
	log     *slog.Logger
}

func NewRoomHandler(pool *pgxpool.Pool, cfg *config.Config, manager *game.Manager, log *slog.Logger) *RoomHandler {
	return &RoomHandler{db: db.New(pool), cfg: cfg, manager: manager, log: log}
}

// Create handles POST /api/rooms.
func (h *RoomHandler) Create(w http.ResponseWriter, r *http.Request) {
	u, ok := middleware.GetSessionUser(r)
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	var req struct {
		GameTypeID string          `json:"game_type_id"`
		PackID     string          `json:"pack_id"`
		TextPackID string          `json:"text_pack_id"`
		Mode       string          `json:"mode"`
		Config     json.RawMessage `json:"config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid JSON")
		return
	}
	gameTypeID, err := uuid.Parse(req.GameTypeID)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid game_type_id")
		return
	}
	packID, err := uuid.Parse(req.PackID)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid pack_id")
		return
	}
	var textPackID uuid.UUID
	if req.TextPackID != "" {
		textPackID, err = uuid.Parse(req.TextPackID)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid text_pack_id")
			return
		}
	}
	// Validate mode
	switch req.Mode {
	case "":
		req.Mode = "multiplayer"
	case "multiplayer", "solo":
		// valid
	default:
		writeError(w, r, http.StatusBadRequest, "bad_request",
			fmt.Sprintf("invalid mode %q: must be multiplayer or solo", req.Mode))
		return
	}

	// Check game type exists and solo support
	gameType, err := h.db.GetGameTypeByID(r.Context(), gameTypeID)
	if err != nil {
		writeError(w, r, http.StatusNotFound, "not_found", "Game type not found")
		return
	}
	handler, ok := h.manager.Registry().Get(gameType.Slug)
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, "unknown_game_type", "Game type handler not registered")
		return
	}
	if req.Mode == "solo" && !handler.SupportsSolo() {
		writeError(w, r, http.StatusUnprocessableEntity, "solo_mode_not_supported", "This game type does not support solo mode")
		return
	}

	// Normalize and validate the room config against the handler's manifest
	// bounds. ValidateAndFill fills any missing fields from the manifest
	// defaults and rejects out-of-range values, so the shape stored in
	// rooms.config is always canonical and within bounds.
	roomConfig, err := handler.Manifest().Config.ValidateAndFill(req.Config)
	if err != nil {
		var verr *game.ValidationError
		if errors.As(err, &verr) {
			writeError(w, r, http.StatusUnprocessableEntity, "invalid_config", verr.Error())
			return
		}
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid room config JSON")
		return
	}

	// Role-aware pack validation. The handler declares what packs it needs
	// (image, text, …) and ValidatePackRequirements enforces presence + item
	// counts across every declared role. Error codes are scoped per role so
	// the client can map them to the right picker.
	var normalized game.RoomConfig
	if err := json.Unmarshal(roomConfig, &normalized); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to parse normalized config")
		return
	}
	packRefs := map[game.PackRole][16]byte{
		game.PackRoleImage: packID,
	}
	if req.TextPackID != "" {
		packRefs[game.PackRoleText] = textPackID
	}
	maxPlayers := handler.MaxPlayers()
	if maxPlayers == 0 {
		maxPlayers = 12
	}
	if perr := game.ValidatePackRequirements(
		r.Context(),
		sqlcCounter{q: h.db},
		handler,
		normalized,
		packRefs,
		maxPlayers,
	); perr != nil {
		status := http.StatusUnprocessableEntity
		switch {
		case perr.Code == "internal_error":
			status = http.StatusInternalServerError
		case strings.HasSuffix(perr.Code, "_required"),
			strings.HasSuffix(perr.Code, "_not_applicable"):
			status = http.StatusBadRequest
		}
		writeError(w, r, status, perr.Code, perr.Message)
		return
	}

	hostUUID, _ := uuid.Parse(u.UserID)
	hostPG := pgtype.UUID{Bytes: hostUUID, Valid: true}

	// Single-room enforcement: a user can be a participant in at most one
	// lobby/playing room. A free user returns pgx.ErrNoRows; anything else is
	// a real database error.
	active, err := h.db.GetActiveRoomForUser(r.Context(), hostPG)
	if err == nil {
		writeError(w, r, http.StatusConflict, "already_in_active_room",
			fmt.Sprintf("You are already in room %s. Leave it before creating a new one.", active.Code))
		return
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to check active room")
		return
	}
	textPackParam := pgtype.UUID{}
	if req.TextPackID != "" {
		textPackParam = pgtype.UUID{Bytes: textPackID, Valid: true}
	}
	room, err := h.db.CreateRoom(r.Context(), db.CreateRoomParams{
		Code:       generateRoomCode(),
		GameTypeID: gameTypeID,
		PackID:     packID,
		TextPackID: textPackParam,
		HostID:     hostPG,
		Mode:       req.Mode,
		Config:     roomConfig,
	})
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to create room")
		return
	}
	// Persist the host into room_players so GetActiveRoomForUser and
	// UpdatePlayerScore / leaderboard / history all agree on membership. The
	// upsert is idempotent — safe to re-run on retried requests.
	if err := h.db.UpsertRoomPlayer(r.Context(), db.UpsertRoomPlayerParams{
		RoomID: room.ID,
		UserID: hostPG,
	}); err != nil && h.log != nil {
		h.log.Warn("failed to persist host into room_players", "room", room.Code, "err", err)
	}
	writeJSON(w, http.StatusCreated, room)
}

// GetByCode handles GET /api/rooms/:code.
// GetByCode handles GET /api/rooms/{code}. Intentionally unauthenticated so
// the frontend's /rooms/{code}?as=guest SSR load works for visitors arriving
// via a shared link (their guest_token lives in client storage and cannot be
// forwarded from the server-side load). Returns only public room metadata.
// The WS handshake is where real identity is enforced (see ws.go).
func (h *RoomHandler) GetByCode(w http.ResponseWriter, r *http.Request) {
	room, err := h.db.GetRoomByCode(r.Context(), chi.URLParam(r, "code"))
	if err != nil {
		writeError(w, r, http.StatusNotFound, "not_found", "Room not found")
		return
	}
	writeJSON(w, http.StatusOK, room)
}

func generateRoomCode() string {
	const chars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, 4)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			panic("crypto/rand unavailable: " + err.Error())
		}
		b[i] = chars[n.Int64()]
	}
	return string(b)
}

func int32SliceToI32Arr(versions []int) []int32 {
	out := make([]int32, len(versions))
	for i, v := range versions {
		out[i] = int32(v)
	}
	return out
}

// sqlcCounter adapts *db.Queries to game.PackItemCounter so the game package
// can validate pack item counts without importing sqlc types directly. Keeps
// the interface boundary in the game package free of db-layer concerns.
type sqlcCounter struct{ q *db.Queries }

func (c sqlcCounter) CountItemsForPack(ctx context.Context, packID [16]byte, versions []int) (int64, error) {
	return c.q.CountCompatibleItems(ctx, db.CountCompatibleItemsParams{
		PackID:   packID,
		Versions: int32SliceToI32Arr(versions),
	})
}
