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
		// Phase 4 (groups) — optional group scoping. When non-empty the
		// room inherits the group's classification, rejects non-members
		// and guests at WS join, and narrows pack sources to group-owned
		// or system packs.
		GroupID string `json:"group_id,omitempty"`
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
	// (image, text, prompt, filler…) and ValidatePackRequirements enforces
	// presence + item counts across every declared role. Error codes are
	// scoped per role so the client can map them to the right picker.
	//
	// The pack_id column holds the primary pack (whatever role is declared
	// first in RequiredPacks()), and text_pack_id holds the secondary if any.
	// Both are positional — the role each one fills is decided per game type.
	var normalized game.RoomConfig
	if err := json.Unmarshal(roomConfig, &normalized); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to parse normalized config")
		return
	}
	required := handler.RequiredPacks()
	packRefs := map[game.PackRole][16]byte{}
	if len(required) >= 1 {
		packRefs[required[0].Role] = packID
	}
	if req.TextPackID != "" {
		// A secondary pack was supplied. Assign it to whichever role the
		// handler declares second; if the handler only needs one pack, fall
		// back to the legacy "text" role so the validator surfaces a stable
		// text_pack_not_applicable error code.
		secondaryRole := game.PackRoleText
		if len(required) >= 2 {
			secondaryRole = required[1].Role
		}
		packRefs[secondaryRole] = textPackID
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

	// Phase 4 — group-scoped room validation. The three preconditions live
	// together so an invalid request shorts before pack-requirement checks
	// run their DB reads:
	//   1. Group exists and is not soft-deleted.
	//   2. Actor is a member of the group.
	//   3. The chosen image pack (and text pack, if any) belongs to this
	//      group OR is a system pack. Personal packs are not allowed — by
	//      design, so hosts duplicate into the group first.
	var groupIDParam pgtype.UUID
	if req.GroupID != "" {
		gid, gerr := uuid.Parse(req.GroupID)
		if gerr != nil {
			writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid group_id")
			return
		}
		group, gerr := h.db.GetGroupByID(r.Context(), gid)
		if gerr != nil {
			writeError(w, r, http.StatusNotFound, "group_not_found", "Group not found")
			return
		}
		if _, merr := h.db.GetMembership(r.Context(), db.GetMembershipParams{
			GroupID: group.ID, UserID: hostUUID,
		}); merr != nil {
			writeError(w, r, http.StatusForbidden, "not_group_member", "You are not a member of this group")
			return
		}
		// Pack source gate: each referenced pack must be either group-owned
		// by THIS group or a system pack. Anything else is a config error.
		checkPackSource := func(pid uuid.UUID) *string {
			p, perr := h.db.GetPackByID(r.Context(), pid)
			if perr != nil {
				s := "not_found"
				return &s
			}
			if p.IsSystem {
				return nil
			}
			if p.GroupID.Valid && p.GroupID.Bytes == group.ID {
				return nil
			}
			s := "pack_not_in_group"
			return &s
		}
		if code := checkPackSource(packID); code != nil {
			writeError(w, r, http.StatusConflict, *code,
				"Group-scoped rooms accept only group or system packs.")
			return
		}
		if req.TextPackID != "" {
			if code := checkPackSource(textPackID); code != nil {
				writeError(w, r, http.StatusConflict, *code,
					"Group-scoped rooms accept only group or system text packs.")
				return
			}
		}
		groupIDParam = pgtype.UUID{Bytes: group.ID, Valid: true}
	}

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
		GroupID:    groupIDParam,
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
