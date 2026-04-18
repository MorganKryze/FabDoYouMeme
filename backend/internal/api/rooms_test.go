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
	q.CreateItem(ctx, db.CreateItemParams{PackID: pack.ID, Name: "test item", PayloadVersion: 1})

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

// seedLobbyRoom creates a lobby room with the given config JSON and returns
// the handler, user, and room. Shared by the config-write tests below.
func seedLobbyRoom(t *testing.T, configJSON string) (*api.RoomHandler, *db.Queries, db.User, db.Room) {
	t.Helper()
	h, q := newRoomHandler(t)
	user := seedRoomUser(t, q)
	ctx := context.Background()
	gt, _ := q.GetGameTypeBySlug(ctx, "meme-caption")
	pack, _ := q.CreatePack(ctx, db.CreatePackParams{
		Name:       testutil.SeedName(t) + "_cfg",
		Visibility: "private",
	})
	code := fmt.Sprintf("C-%s-%d", testutil.SeedName(t), time.Now().UnixNano())
	room, err := q.CreateRoom(ctx, db.CreateRoomParams{
		Code:       code,
		GameTypeID: gt.ID,
		PackID:     pack.ID,
		HostID:     pgtype.UUID{Bytes: user.ID, Valid: true},
		Mode:       "multiplayer",
		Config:     json.RawMessage(configJSON),
	})
	if err != nil {
		t.Fatalf("seedLobbyRoom: create room: %v", err)
	}
	return h, q, user, room
}

func TestPatchConfig_PartialPatchJokerCount(t *testing.T) {
	h, _, user, room := seedLobbyRoom(t,
		`{"round_count":10,"round_duration_seconds":60,"voting_duration_seconds":30,"joker_count":2,"allow_skip_vote":true}`)

	body := `{"config":{"joker_count":5}}`
	req := httptest.NewRequest(http.MethodPatch, "/api/rooms/"+room.Code+"/config", bytes.NewBufferString(body))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("code", room.Code)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = withUser(req, user.ID.String(), user.Username, user.Email, user.Role)
	rec := httptest.NewRecorder()
	h.UpdateConfig(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d — body: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Config map[string]any `json:"config"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got, _ := resp.Config["joker_count"].(float64); int(got) != 5 {
		t.Fatalf("joker_count: want 5, got %v", resp.Config["joker_count"])
	}
	if got, _ := resp.Config["round_count"].(float64); int(got) != 10 {
		t.Fatalf("round_count should survive the partial patch: want 10, got %v", resp.Config["round_count"])
	}
}

func TestPatchConfig_PartialPatchAllowSkipVote(t *testing.T) {
	h, _, user, room := seedLobbyRoom(t,
		`{"round_count":5,"round_duration_seconds":60,"voting_duration_seconds":30,"joker_count":1,"allow_skip_vote":true}`)

	body := `{"config":{"allow_skip_vote":false}}`
	req := httptest.NewRequest(http.MethodPatch, "/api/rooms/"+room.Code+"/config", bytes.NewBufferString(body))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("code", room.Code)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = withUser(req, user.ID.String(), user.Username, user.Email, user.Role)
	rec := httptest.NewRecorder()
	h.UpdateConfig(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d — body: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Config map[string]any `json:"config"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got, ok := resp.Config["allow_skip_vote"].(bool); !ok || got {
		t.Fatalf("allow_skip_vote: want false, got %v", resp.Config["allow_skip_vote"])
	}
}

func TestPatchConfig_JokerCountAboveRoundCount_Rejected(t *testing.T) {
	h, _, user, room := seedLobbyRoom(t,
		`{"round_count":5,"round_duration_seconds":60,"voting_duration_seconds":30,"joker_count":1,"allow_skip_vote":true}`)

	body := `{"config":{"joker_count":99}}`
	req := httptest.NewRequest(http.MethodPatch, "/api/rooms/"+room.Code+"/config", bytes.NewBufferString(body))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("code", room.Code)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = withUser(req, user.ID.String(), user.Username, user.Email, user.Role)
	rec := httptest.NewRecorder()
	h.UpdateConfig(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status: want 422, got %d — body: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "invalid_config" {
		t.Fatalf("code: want invalid_config, got %s", resp["code"])
	}
	if !bytes.Contains([]byte(resp["error"]), []byte("joker_count")) {
		t.Fatalf("error message should mention joker_count: %q", resp["error"])
	}
}

func TestCreateRoom_DefaultsJokerCountAndAllowSkipVote(t *testing.T) {
	h, q := newRoomHandler(t)
	user := seedRoomUser(t, q)
	ctx := context.Background()

	gt, _ := q.GetGameTypeBySlug(ctx, "meme-caption")

	// Seed a pack with enough items to satisfy pack_insufficient_items.
	pack, _ := q.CreatePack(ctx, db.CreatePackParams{
		Name:       testutil.SeedName(t) + "_dflt",
		Visibility: "private",
	})
	for i := 0; i < 11; i++ {
		item, _ := q.CreateItem(ctx, db.CreateItemParams{
			PackID:         pack.ID,
			Name:           fmt.Sprintf("i%d", i),
			PayloadVersion: 1,
		})
		key := fmt.Sprintf("test/%s/%d.png", pack.ID, i)
		payload := json.RawMessage(fmt.Sprintf(`{"caption":"c%d"}`, i))
		ver, _ := q.CreateItemVersion(ctx, db.CreateItemVersionParams{
			ItemID: item.ID, MediaKey: &key, Payload: payload,
		})
		q.SetCurrentVersion(ctx, db.SetCurrentVersionParams{
			ID:               item.ID,
			CurrentVersionID: pgtype.UUID{Bytes: ver.ID, Valid: true},
		})
	}

	// POST /api/rooms without specifying the two new fields.
	body, _ := json.Marshal(map[string]any{
		"game_type_id": gt.ID.String(),
		"pack_id":      pack.ID.String(),
		"mode":         "multiplayer",
		"config":       json.RawMessage(`{"round_count":10,"round_duration_seconds":60,"voting_duration_seconds":30}`),
	})
	req := httptest.NewRequest(http.MethodPost, "/api/rooms", bytes.NewReader(body))
	req = withUser(req, user.ID.String(), user.Username, user.Email, user.Role)
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusOK && rec.Code != http.StatusCreated {
		t.Fatalf("status: want 200/201, got %d — body: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Config map[string]any `json:"config"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got, _ := resp.Config["joker_count"].(float64); int(got) != 2 {
		t.Fatalf("joker_count: want 2 (ceil(10/5)), got %v", resp.Config["joker_count"])
	}
	if got, ok := resp.Config["allow_skip_vote"].(bool); !ok || !got {
		t.Fatalf("allow_skip_vote: want true, got %v", resp.Config["allow_skip_vote"])
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
