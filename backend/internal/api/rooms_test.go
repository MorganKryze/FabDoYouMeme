package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/api"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/clock"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/config"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/game"
	memecaption "github.com/MorganKryze/FabDoYouMeme/backend/internal/game/types/meme_caption"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/testutil"
)

func newRoomHandler(t *testing.T) (*api.RoomHandler, *db.Queries) {
	t.Helper()
	pool := testutil.Pool()
	registry := game.NewRegistry()
	registry.Register(memecaption.New())
	q := db.New(pool)
	cfg := &config.Config{}
	manager := game.NewManager(context.Background(), registry, q, cfg, slog.Default(), clock.Real{})
	return api.NewRoomHandler(pool, cfg, manager, slog.Default()), q
}

func seedRoomUser(t *testing.T, q *db.Queries) db.User {
	t.Helper()
	slug := testutil.SeedName(t)
	u, err := q.CreateUser(context.Background(), db.CreateUserParams{
		Username:  slug,
		Email:     slug + "@test.com",
		Role:      "player",
		IsActive:  true,
		ConsentAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("seedRoomUser: %v", err)
	}
	return u
}

func TestCreateRoom_PackNoSupportedItems(t *testing.T) {
	h, q := newRoomHandler(t)
	user := seedRoomUser(t, q)

	// Get the seeded meme-caption game type.
	gt, err := q.GetGameTypeBySlug(context.Background(), "meme-caption")
	if err != nil {
		t.Fatalf("get game type: %v", err)
	}

	// Create a pack with zero items.
	pack, err := q.CreatePack(context.Background(), db.CreatePackParams{
		Name:       testutil.SeedName(t) + "_pk",
		Visibility: "private",
	})
	if err != nil {
		t.Fatalf("create pack: %v", err)
	}

	body, _ := json.Marshal(map[string]any{
		"game_type_id": gt.ID.String(),
		"pack_id":      pack.ID.String(),
		"mode":         "multiplayer",
		"config":       json.RawMessage(`{"round_count":3,"round_duration_seconds":60,"voting_duration_seconds":30}`),
	})
	req := httptest.NewRequest(http.MethodPost, "/api/rooms", bytes.NewReader(body))
	req = withUser(req, user.ID.String(), user.Username, user.Email, user.Role)
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("want 422, got %d — body: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "pack_no_supported_items" {
		t.Errorf("want pack_no_supported_items, got %s", resp["code"])
	}
}

func TestCreateRoom_PackInsufficientItems(t *testing.T) {
	h, q := newRoomHandler(t)
	user := seedRoomUser(t, q)
	ctx := context.Background()

	gt, _ := q.GetGameTypeBySlug(ctx, "meme-caption")

	// Create a pack with 1 item (fewer than round_count=3).
	pack, _ := q.CreatePack(ctx, db.CreatePackParams{
		Name:       testutil.SeedName(t) + "_insuf",
		Visibility: "private",
	})
	q.CreateItem(ctx, db.CreateItemParams{PackID: pack.ID, PayloadVersion: 1})

	body, _ := json.Marshal(map[string]any{
		"game_type_id": gt.ID.String(),
		"pack_id":      pack.ID.String(),
		"mode":         "multiplayer",
		"config":       json.RawMessage(`{"round_count":3,"round_duration_seconds":60,"voting_duration_seconds":30}`),
	})
	req := httptest.NewRequest(http.MethodPost, "/api/rooms", bytes.NewReader(body))
	req = withUser(req, user.ID.String(), user.Username, user.Email, user.Role)
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("want 422, got %d — body: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "pack_insufficient_items" {
		t.Errorf("want pack_insufficient_items, got %s", resp["code"])
	}
}

func TestGetRoom_NotFound(t *testing.T) {
	h, q := newRoomHandler(t)
	user := seedRoomUser(t, q)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("code", "ZZZZ")
	req := httptest.NewRequest(http.MethodGet, "/api/rooms/ZZZZ", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = withUser(req, user.ID.String(), user.Username, user.Email, user.Role)
	rec := httptest.NewRecorder()
	h.GetByCode(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", rec.Code)
	}
}

func TestGetRoom_ByCode_Found(t *testing.T) {
	h, q := newRoomHandler(t)
	user := seedRoomUser(t, q)
	ctx := context.Background()

	gt, _ := q.GetGameTypeBySlug(ctx, "meme-caption")
	pack, _ := q.CreatePack(ctx, db.CreatePackParams{
		Name:       testutil.SeedName(t) + "_gc",
		Visibility: "private",
	})
	code := fmt.Sprintf("T%03d", time.Now().UnixNano()%1000)
	room, err := q.CreateRoom(ctx, db.CreateRoomParams{
		Code:       code,
		GameTypeID: gt.ID,
		PackID:     pack.ID,
		HostID:     pgtype.UUID{Bytes: user.ID, Valid: true},
		Mode:       "multiplayer",
		Config:     json.RawMessage(`{"round_count":3,"round_duration_seconds":60,"voting_duration_seconds":30}`),
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("code", room.Code)
	req := httptest.NewRequest(http.MethodGet, "/api/rooms/"+room.Code, nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = withUser(req, user.ID.String(), user.Username, user.Email, user.Role)
	rec := httptest.NewRecorder()
	h.GetByCode(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d — body: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != room.Code {
		t.Errorf("want room code %s, got %v", room.Code, resp["code"])
	}
}
