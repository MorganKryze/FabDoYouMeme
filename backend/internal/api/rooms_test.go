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
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/api"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/clock"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/config"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/game"
	memefreestyle "github.com/MorganKryze/FabDoYouMeme/backend/internal/game/types/meme_freestyle"
	memeshowdown "github.com/MorganKryze/FabDoYouMeme/backend/internal/game/types/meme_showdown"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/testutil"
)

func newRoomHandler(t *testing.T) (*api.RoomHandler, *db.Queries) {
	t.Helper()
	pool := testutil.Pool()
	registry := game.NewRegistry()
	registry.Register(memefreestyle.New())
	registry.Register(memeshowdown.New())
	q := db.New(pool)
	// Sync handlers into game_types so tests that POST /api/rooms with the
	// meme-showdown slug find the row — matches the production boot flow.
	if err := game.SyncGameTypes(context.Background(), q, registry, slog.Default()); err != nil {
		t.Fatalf("sync game types: %v", err)
	}
	cfg := &config.Config{}
	manager := game.NewManager(context.Background(), registry, q, cfg, slog.Default(), clock.Real{})
	return api.NewRoomHandler(pool, cfg, manager, slog.Default()), q
}

// seedPackWithItems creates a pack and N items whose current version payload
// is `{"text":"tN"}` for payload_version=2 or `{"caption":"cN"}` for v1 (the
// shape doesn't matter to the count check — only payload_version does).
func seedPackWithItems(t *testing.T, q *db.Queries, ctx context.Context, namePrefix string, payloadVersion int32, count int) db.GamePack {
	t.Helper()
	pack, err := q.CreatePack(ctx, db.CreatePackParams{
		Name:       testutil.SeedName(t) + "_" + namePrefix,
		Visibility: "private",
		Language:   "en",
	})
	if err != nil {
		t.Fatalf("create pack: %v", err)
	}
	for i := 0; i < count; i++ {
		item, err := q.CreateItem(ctx, db.CreateItemParams{
			PackID:         pack.ID,
			Name:           fmt.Sprintf("%s-%d", namePrefix, i),
			PayloadVersion: payloadVersion,
		})
		if err != nil {
			t.Fatalf("create item: %v", err)
		}
		var payload json.RawMessage
		if payloadVersion == 2 {
			payload = json.RawMessage(fmt.Sprintf(`{"text":"t%d"}`, i))
		} else {
			payload = json.RawMessage(fmt.Sprintf(`{"caption":"c%d"}`, i))
		}
		key := fmt.Sprintf("test/%s/%d.png", pack.ID, i)
		ver, err := q.CreateItemVersion(ctx, db.CreateItemVersionParams{
			ItemID: item.ID, MediaKey: &key, Payload: payload,
		})
		if err != nil {
			t.Fatalf("create item version: %v", err)
		}
		if _, err := q.SetCurrentVersion(ctx, db.SetCurrentVersionParams{
			ID:               item.ID,
			CurrentVersionID: pgtype.UUID{Bytes: ver.ID, Valid: true},
		}); err != nil {
			t.Fatalf("set current version: %v", err)
		}
	}
	return pack
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
		Locale:    "en",
	})
	if err != nil {
		t.Fatalf("seedRoomUser: %v", err)
	}
	return u
}

func TestCreateRoom_ImagePackNoSupportedItems(t *testing.T) {
	h, q := newRoomHandler(t)
	user := seedRoomUser(t, q)

	// Get the seeded meme-freestyle game type.
	gt, err := q.GetGameTypeBySlug(context.Background(), "meme-freestyle")
	if err != nil {
		t.Fatalf("get game type: %v", err)
	}

	// Create a pack with zero items.
	pack, err := q.CreatePack(context.Background(), db.CreatePackParams{
		Name:       testutil.SeedName(t) + "_pk",
		Visibility: "private",
		Language:   "en",
	})
	if err != nil {
		t.Fatalf("create pack: %v", err)
	}

	body, _ := json.Marshal(map[string]any{
		"game_type_id": gt.ID.String(),
		"packs": []map[string]any{
			{"role": "image", "pack_id": pack.ID.String(), "weight": 1},
		},
		"mode":   "multiplayer",
		"config": json.RawMessage(`{"round_count":3,"round_duration_seconds":60,"voting_duration_seconds":30}`),
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
	if resp["code"] != "image_pack_no_supported_items" {
		t.Errorf("want image_pack_no_supported_items, got %s", resp["code"])
	}
}

func TestCreateRoom_ImagePackInsufficient(t *testing.T) {
	h, q := newRoomHandler(t)
	user := seedRoomUser(t, q)
	ctx := context.Background()

	gt, _ := q.GetGameTypeBySlug(ctx, "meme-freestyle")

	// Create a pack with 1 item (fewer than round_count=3).
	pack, _ := q.CreatePack(ctx, db.CreatePackParams{
		Name:       testutil.SeedName(t) + "_insuf",
		Visibility: "private",
		Language:   "en",
	})
	q.CreateItem(ctx, db.CreateItemParams{PackID: pack.ID, Name: "test item", PayloadVersion: 1})

	body, _ := json.Marshal(map[string]any{
		"game_type_id": gt.ID.String(),
		"packs": []map[string]any{
			{"role": "image", "pack_id": pack.ID.String(), "weight": 1},
		},
		"mode":   "multiplayer",
		"config": json.RawMessage(`{"round_count":3,"round_duration_seconds":60,"voting_duration_seconds":30}`),
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
	if resp["code"] != "image_pack_insufficient" {
		t.Errorf("want image_pack_insufficient, got %s", resp["code"])
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
	gt, _ := q.GetGameTypeBySlug(ctx, "meme-freestyle")
	pack, _ := q.CreatePack(ctx, db.CreatePackParams{
		Name:       testutil.SeedName(t) + "_cfg",
		Visibility: "private",
		Language:   "en",
	})
	code := fmt.Sprintf("C-%s-%d", testutil.SeedName(t), time.Now().UnixNano())
	room, err := q.CreateRoom(ctx, db.CreateRoomParams{
		Code:       code,
		GameTypeID: gt.ID,
		HostID:     pgtype.UUID{Bytes: user.ID, Valid: true},
		Mode:       "multiplayer",
		Config:     json.RawMessage(configJSON),
	})
	if err != nil {
		t.Fatalf("seedLobbyRoom: create room: %v", err)
	}
	if err := q.InsertRoomPack(ctx, db.InsertRoomPackParams{
		RoomID: room.ID, Role: "image", PackID: pack.ID, Weight: 1,
	}); err != nil {
		t.Fatalf("seedLobbyRoom: insert room_pack: %v", err)
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

	gt, _ := q.GetGameTypeBySlug(ctx, "meme-freestyle")

	// Seed a pack with enough items to satisfy image_pack_insufficient.
	pack, _ := q.CreatePack(ctx, db.CreatePackParams{
		Name:       testutil.SeedName(t) + "_dflt",
		Visibility: "private",
		Language:   "en",
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
		"packs": []map[string]any{
			{"role": "image", "pack_id": pack.ID.String(), "weight": 1},
		},
		"mode":   "multiplayer",
		"config": json.RawMessage(`{"round_count":10,"round_duration_seconds":60,"voting_duration_seconds":30}`),
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

	gt, _ := q.GetGameTypeBySlug(ctx, "meme-freestyle")
	pack, _ := q.CreatePack(ctx, db.CreatePackParams{
		Name:       testutil.SeedName(t) + "_gc",
		Visibility: "private",
		Language:   "en",
	})
	code := fmt.Sprintf("T%03d", time.Now().UnixNano()%1000)
	room, err := q.CreateRoom(ctx, db.CreateRoomParams{
		Code:       code,
		GameTypeID: gt.ID,
		HostID:     pgtype.UUID{Bytes: user.ID, Valid: true},
		Mode:       "multiplayer",
		Config:     json.RawMessage(`{"round_count":3,"round_duration_seconds":60,"voting_duration_seconds":30}`),
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	if err := q.InsertRoomPack(ctx, db.InsertRoomPackParams{
		RoomID: room.ID, Role: "image", PackID: pack.ID, Weight: 1,
	}); err != nil {
		t.Fatalf("insert room_pack: %v", err)
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

func TestCreateRoom_MemeVote_RequiresTextPack(t *testing.T) {
	h, q := newRoomHandler(t)
	user := seedRoomUser(t, q)
	ctx := context.Background()

	gt, _ := q.GetGameTypeBySlug(ctx, "meme-showdown")
	imgPack := seedPackWithItems(t, q, ctx, "img", 1, 10)

	body, _ := json.Marshal(map[string]any{
		"game_type_id": gt.ID.String(),
		"packs": []map[string]any{
			{"role": "image", "pack_id": imgPack.ID.String(), "weight": 1},
		},
		"mode":   "multiplayer",
		"config": json.RawMessage(`{"round_count":5,"round_duration_seconds":45,"voting_duration_seconds":30,"hand_size":5}`),
	})
	req := httptest.NewRequest(http.MethodPost, "/api/rooms", bytes.NewReader(body))
	req = withUser(req, user.ID.String(), user.Username, user.Email, user.Role)
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d — body: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "text_pack_required" {
		t.Errorf("want text_pack_required, got %s", resp["code"])
	}
}

func TestCreateRoom_MemeCaption_RejectsTextPack(t *testing.T) {
	h, q := newRoomHandler(t)
	user := seedRoomUser(t, q)
	ctx := context.Background()

	gt, _ := q.GetGameTypeBySlug(ctx, "meme-freestyle")
	imgPack := seedPackWithItems(t, q, ctx, "img", 1, 10)
	textPack := seedPackWithItems(t, q, ctx, "txt", 2, 5)

	body, _ := json.Marshal(map[string]any{
		"game_type_id": gt.ID.String(),
		"packs": []map[string]any{
			{"role": "image", "pack_id": imgPack.ID.String(), "weight": 1},
			{"role": "text", "pack_id": textPack.ID.String(), "weight": 1},
		},
		"mode":   "multiplayer",
		"config": json.RawMessage(`{"round_count":3,"round_duration_seconds":60,"voting_duration_seconds":30}`),
	})
	req := httptest.NewRequest(http.MethodPost, "/api/rooms", bytes.NewReader(body))
	req = withUser(req, user.ID.String(), user.Username, user.Email, user.Role)
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d — body: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "text_pack_not_applicable" {
		t.Errorf("want text_pack_not_applicable, got %s", resp["code"])
	}
}

func TestCreateRoom_MemeVote_TextPackInsufficient(t *testing.T) {
	h, q := newRoomHandler(t)
	user := seedRoomUser(t, q)
	ctx := context.Background()

	gt, _ := q.GetGameTypeBySlug(ctx, "meme-showdown")
	imgPack := seedPackWithItems(t, q, ctx, "img", 1, 10)
	// meme-showdown worst-case: hand_size * max_players + (round_count-1) *
	// max_players = 5*12 + 4*12 = 108. Seed only 2 to trigger insufficient.
	textPack := seedPackWithItems(t, q, ctx, "txt", 2, 2)

	body, _ := json.Marshal(map[string]any{
		"game_type_id": gt.ID.String(),
		"packs": []map[string]any{
			{"role": "image", "pack_id": imgPack.ID.String(), "weight": 1},
			{"role": "text", "pack_id": textPack.ID.String(), "weight": 1},
		},
		"mode":   "multiplayer",
		"config": json.RawMessage(`{"round_count":5,"round_duration_seconds":45,"voting_duration_seconds":30,"hand_size":5}`),
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
	if resp["code"] != "text_pack_insufficient" {
		t.Errorf("want text_pack_insufficient, got %s", resp["code"])
	}
}

// TestCreateRoom_MaxPlayers_FullCapRejects mirrors the
// existing "insufficient" test but seeds a sub-worst-case pack (28 items)
// and explicitly leaves max_players at the manifest cap (12). With a 4-round
// game the requirement is hand_size×12 + (4-1)×12 = 5×12 + 3×12 = 96 > 28,
// so the validator should reject. Pairs with the next test, which keeps the
// pack the same but lowers max_players to 4 and expects success.
func TestCreateRoom_MaxPlayers_FullCapRejects(t *testing.T) {
	h, q := newRoomHandler(t)
	user := seedRoomUser(t, q)
	ctx := context.Background()

	gt, _ := q.GetGameTypeBySlug(ctx, "meme-showdown")
	imgPack := seedPackWithItems(t, q, ctx, "img", 1, 5)
	textPack := seedPackWithItems(t, q, ctx, "txt", 2, 28)

	body, _ := json.Marshal(map[string]any{
		"game_type_id": gt.ID.String(),
		"packs": []map[string]any{
			{"role": "image", "pack_id": imgPack.ID.String(), "weight": 1},
			{"role": "text", "pack_id": textPack.ID.String(), "weight": 1},
		},
		"mode":   "multiplayer",
		"config": json.RawMessage(`{"round_count":4,"round_duration_seconds":45,"voting_duration_seconds":30,"hand_size":5,"max_players":12}`),
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
	if resp["code"] != "text_pack_insufficient" {
		t.Errorf("want text_pack_insufficient, got %s", resp["code"])
	}
}

// TestCreateRoom_MaxPlayers_SmallCapAccepts proves the new
// per-room sizing: same 28-item pack as the test above, same 4-round config,
// but the host caps the room at 4 players. Requirement becomes 5×4 + 3×4 = 32
// — still > 28, so we need 5 players' worth. Use max_players=3:
// 5×3 + 3×3 = 24 ≤ 28 → accepted.
func TestCreateRoom_MaxPlayers_SmallCapAccepts(t *testing.T) {
	h, q := newRoomHandler(t)
	user := seedRoomUser(t, q)
	ctx := context.Background()

	gt, _ := q.GetGameTypeBySlug(ctx, "meme-showdown")
	imgPack := seedPackWithItems(t, q, ctx, "img", 1, 5)
	textPack := seedPackWithItems(t, q, ctx, "txt", 2, 28)

	body, _ := json.Marshal(map[string]any{
		"game_type_id": gt.ID.String(),
		"packs": []map[string]any{
			{"role": "image", "pack_id": imgPack.ID.String(), "weight": 1},
			{"role": "text", "pack_id": textPack.ID.String(), "weight": 1},
		},
		"mode":   "multiplayer",
		"config": json.RawMessage(`{"round_count":4,"round_duration_seconds":45,"voting_duration_seconds":30,"hand_size":5,"max_players":3}`),
	})
	req := httptest.NewRequest(http.MethodPost, "/api/rooms", bytes.NewReader(body))
	req = withUser(req, user.ID.String(), user.Username, user.Email, user.Role)
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("want 201 with smaller cap, got %d — body: %s", rec.Code, rec.Body.String())
	}
}

// TestCreateRoom_MaxPlayers_RejectAboveCap asserts the new
// max_players field is bounds-checked by ValidateAndFill. Sending 99 against
// a manifest that caps at 12 returns invalid_config + a max_players hint.
func TestCreateRoom_MaxPlayers_RejectAboveCap(t *testing.T) {
	h, q := newRoomHandler(t)
	user := seedRoomUser(t, q)
	ctx := context.Background()

	gt, _ := q.GetGameTypeBySlug(ctx, "meme-showdown")
	imgPack := seedPackWithItems(t, q, ctx, "img", 1, 5)
	textPack := seedPackWithItems(t, q, ctx, "txt", 2, 5)

	body, _ := json.Marshal(map[string]any{
		"game_type_id": gt.ID.String(),
		"packs": []map[string]any{
			{"role": "image", "pack_id": imgPack.ID.String(), "weight": 1},
			{"role": "text", "pack_id": textPack.ID.String(), "weight": 1},
		},
		"mode":   "multiplayer",
		"config": json.RawMessage(`{"round_count":4,"round_duration_seconds":45,"voting_duration_seconds":30,"hand_size":5,"max_players":99}`),
	})
	req := httptest.NewRequest(http.MethodPost, "/api/rooms", bytes.NewReader(body))
	req = withUser(req, user.ID.String(), user.Username, user.Email, user.Role)
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422 invalid_config, got %d — body: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "invalid_config" {
		t.Errorf("want invalid_config, got %s", resp["code"])
	}
}

func TestCreateRoom_MemeVote_Success(t *testing.T) {
	h, q := newRoomHandler(t)
	user := seedRoomUser(t, q)
	ctx := context.Background()

	gt, _ := q.GetGameTypeBySlug(ctx, "meme-showdown")
	imgPack := seedPackWithItems(t, q, ctx, "img", 1, 10)
	textPack := seedPackWithItems(t, q, ctx, "txt", 2, 120)

	body, _ := json.Marshal(map[string]any{
		"game_type_id": gt.ID.String(),
		"packs": []map[string]any{
			{"role": "image", "pack_id": imgPack.ID.String(), "weight": 1},
			{"role": "text", "pack_id": textPack.ID.String(), "weight": 1},
		},
		"mode":   "multiplayer",
		"config": json.RawMessage(`{"round_count":5,"round_duration_seconds":45,"voting_duration_seconds":30,"hand_size":5}`),
	})
	req := httptest.NewRequest(http.MethodPost, "/api/rooms", bytes.NewReader(body))
	req = withUser(req, user.ID.String(), user.Username, user.Email, user.Role)
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d — body: %s", rec.Code, rec.Body.String())
	}
	var room db.Room
	if err := json.NewDecoder(rec.Body).Decode(&room); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// room_packs is the new source of truth (ADR-016) — verify the text role
	// landed in the join table with the expected pack id and weight.
	rps, err := q.ListRoomPacks(ctx, room.ID)
	if err != nil {
		t.Fatalf("list room_packs: %v", err)
	}
	foundText := false
	for _, rp := range rps {
		if rp.Role == "text" && rp.PackID == textPack.ID {
			foundText = true
		}
	}
	if !foundText {
		t.Errorf("text pack not persisted in room_packs: %+v (want %s)", rps, textPack.ID)
	}
}

// TestCreateRoom_MultiPack_PoolModelAccepted exercises ADR-016 end-to-end:
// two image packs in a 3:1 mix, with neither pack alone large enough but the
// pool sum sufficient. Verifies (a) the room is created and (b) both rows
// land in room_packs with the supplied weights.
func TestCreateRoom_MultiPack_PoolModelAccepted(t *testing.T) {
	h, q := newRoomHandler(t)
	user := seedRoomUser(t, q)
	ctx := context.Background()

	gt, _ := q.GetGameTypeBySlug(ctx, "meme-freestyle")
	// round_count=10. Each pack has 6 items — neither alone covers 10, the
	// pool sum (12) does. Earlier single-pack validation would reject either.
	packA := seedPackWithItems(t, q, ctx, "imgA", 1, 6)
	packB := seedPackWithItems(t, q, ctx, "imgB", 1, 6)

	body, _ := json.Marshal(map[string]any{
		"game_type_id": gt.ID.String(),
		"packs": []map[string]any{
			{"role": "image", "pack_id": packA.ID.String(), "weight": 3},
			{"role": "image", "pack_id": packB.ID.String(), "weight": 1},
		},
		"mode":   "multiplayer",
		"config": json.RawMessage(`{"round_count":10,"round_duration_seconds":60,"voting_duration_seconds":30}`),
	})
	req := httptest.NewRequest(http.MethodPost, "/api/rooms", bytes.NewReader(body))
	req = withUser(req, user.ID.String(), user.Username, user.Email, user.Role)
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("want 201 (pool sum covers MinItemsFn), got %d — body: %s", rec.Code, rec.Body.String())
	}
	var room db.Room
	_ = json.NewDecoder(rec.Body).Decode(&room)

	rps, err := q.ListRoomPacks(ctx, room.ID)
	if err != nil {
		t.Fatalf("list room_packs: %v", err)
	}
	weights := map[uuid.UUID]int32{}
	for _, rp := range rps {
		weights[rp.PackID] = rp.Weight
	}
	if weights[packA.ID] != 3 {
		t.Errorf("packA weight = %d, want 3", weights[packA.ID])
	}
	if weights[packB.ID] != 1 {
		t.Errorf("packB weight = %d, want 1", weights[packB.ID])
	}
}

// TestCreateRoom_MultiPack_DuplicatePackRejected hits the validator's
// per-role uniqueness rule: the same pack id listed twice for one role is
// a config error.
func TestCreateRoom_MultiPack_DuplicatePackRejected(t *testing.T) {
	h, q := newRoomHandler(t)
	user := seedRoomUser(t, q)
	ctx := context.Background()

	gt, _ := q.GetGameTypeBySlug(ctx, "meme-freestyle")
	pack := seedPackWithItems(t, q, ctx, "img", 1, 10)

	body, _ := json.Marshal(map[string]any{
		"game_type_id": gt.ID.String(),
		"packs": []map[string]any{
			{"role": "image", "pack_id": pack.ID.String(), "weight": 1},
			{"role": "image", "pack_id": pack.ID.String(), "weight": 2},
		},
		"mode":   "multiplayer",
		"config": json.RawMessage(`{"round_count":3,"round_duration_seconds":60,"voting_duration_seconds":30}`),
	})
	req := httptest.NewRequest(http.MethodPost, "/api/rooms", bytes.NewReader(body))
	req = withUser(req, user.ID.String(), user.Username, user.Email, user.Role)
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d — body: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "image_pack_invalid" {
		t.Errorf("want image_pack_invalid, got %s", resp["code"])
	}
}
