// backend/internal/api/rooms.go
package api

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
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
}

func NewRoomHandler(pool *pgxpool.Pool, cfg *config.Config, manager *game.Manager) *RoomHandler {
	return &RoomHandler{db: db.New(pool), cfg: cfg, manager: manager}
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
	// Validate config JSON structure if provided
	var roomCfgValidate struct {
		RoundCount            int `json:"round_count"`
		RoundDurationSeconds  int `json:"round_duration_seconds"`
		VotingDurationSeconds int `json:"voting_duration_seconds"`
	}
	if req.Config != nil {
		if err := json.Unmarshal(req.Config, &roomCfgValidate); err != nil {
			writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid room config JSON")
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

	// Validate pack compatibility
	versions := handler.SupportedPayloadVersions()
	count, err := h.db.CountCompatibleItems(r.Context(), db.CountCompatibleItemsParams{
		PackID: packID, Versions: int32SliceToI32Arr(versions),
	})
	if err != nil || count == 0 {
		writeError(w, r, http.StatusUnprocessableEntity, "pack_no_supported_items", "Pack has no items compatible with this game type")
		return
	}

	// Parse config to get round_count for minimum check
	var roomCfg struct{ RoundCount int `json:"round_count"` }
	if req.Config != nil {
		json.Unmarshal(req.Config, &roomCfg)
	}
	if roomCfg.RoundCount == 0 {
		roomCfg.RoundCount = 10
	}
	if int64(roomCfg.RoundCount) > count {
		writeError(w, r, http.StatusUnprocessableEntity, "pack_insufficient_items", "Pack does not have enough items for the requested round count")
		return
	}

	hostUUID, _ := uuid.Parse(u.UserID)
	roomConfig := req.Config
	if roomConfig == nil {
		roomConfig = json.RawMessage(`{"round_duration_seconds":60,"voting_duration_seconds":30,"round_count":10}`)
	}
	room, err := h.db.CreateRoom(r.Context(), db.CreateRoomParams{
		Code:       generateRoomCode(),
		GameTypeID: gameTypeID,
		PackID:     packID,
		HostID:     pgtype.UUID{Bytes: hostUUID, Valid: true},
		Mode:       req.Mode,
		Config:     roomConfig,
	})
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to create room")
		return
	}
	writeJSON(w, http.StatusCreated, room)
}

// GetByCode handles GET /api/rooms/:code.
func (h *RoomHandler) GetByCode(w http.ResponseWriter, r *http.Request) {
	_, ok := middleware.GetSessionUser(r)
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
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
