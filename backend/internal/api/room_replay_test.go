// backend/internal/api/room_replay_test.go
package api_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/testutil"
)

// seedFinishedReplayRoom builds a finished room the given user is a member of,
// with one started round — the minimum shape GetFinishedRoomByCode +
// GetReplayRounds expect. When startRound is false, no round is created (used
// by Task 1-style abandoned-room tests elsewhere; keep here for symmetry).
func seedFinishedReplayRoom(t *testing.T, q *db.Queries, user db.User, startRound bool) db.Room {
	t.Helper()
	ctx := context.Background()

	pack, err := q.CreatePack(ctx, db.CreatePackParams{
		Name:       testutil.SeedName(t) + "_" + user.Username + "_rpk",
		Visibility: "private",
		Language:   "en",
	})
	if err != nil {
		t.Fatalf("create pack: %v", err)
	}
	item, err := q.CreateItem(ctx, db.CreateItemParams{
		PackID: pack.ID, Name: "prompt", PayloadVersion: 1,
	})
	if err != nil {
		t.Fatalf("create item: %v", err)
	}
	// GetReplayRounds inner-joins game_item_versions via gi.current_version_id,
	// so abandoned items without a version drop the round silently. Attach one.
	mediaKey := fmt.Sprintf("test/%s/%s.png", pack.ID, user.Username)
	ver, err := q.CreateItemVersion(ctx, db.CreateItemVersionParams{
		ItemID: item.ID, MediaKey: &mediaKey, Payload: json.RawMessage(`{"caption":"prompt"}`),
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
	gt, err := q.GetGameTypeBySlug(ctx, "meme-freestyle")
	if err != nil {
		t.Fatalf("game type: %v", err)
	}
	code := fmt.Sprintf("R-%s-%s-%d", testutil.SeedName(t), user.Username, time.Now().UnixNano())
	room, err := q.CreateRoom(ctx, db.CreateRoomParams{
		Code:       code,
		GameTypeID: gt.ID,
		PackID:     pack.ID,
		HostID:     pgtype.UUID{Bytes: user.ID, Valid: true},
		Mode:       "multiplayer",
		Config:     json.RawMessage(`{"round_count":3,"round_duration_seconds":30,"voting_duration_seconds":15}`),
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	if err := q.UpsertRoomPlayer(ctx, db.UpsertRoomPlayerParams{
		RoomID: room.ID,
		UserID: pgtype.UUID{Bytes: user.ID, Valid: true},
	}); err != nil {
		t.Fatalf("upsert player: %v", err)
	}
	if startRound {
		rnd, err := q.CreateRound(ctx, db.CreateRoundParams{
			RoomID: room.ID, ItemID: item.ID,
		})
		if err != nil {
			t.Fatalf("create round: %v", err)
		}
		if _, err := q.StartRound(ctx, rnd.ID); err != nil {
			t.Fatalf("start round: %v", err)
		}
	}
	if _, err := q.SetRoomState(ctx, db.SetRoomStateParams{
		ID: room.ID, State: "finished",
	}); err != nil {
		t.Fatalf("set finished: %v", err)
	}
	return room
}

func replayRequest(code string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("code", code)
	req := httptest.NewRequest(http.MethodGet, "/api/rooms/"+code+"/replay", nil)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func TestGetReplay_RequiresSession(t *testing.T) {
	h, _ := newRoomHandler(t)
	req := replayRequest("ZZZZ") // no session injected
	rec := httptest.NewRecorder()
	h.GetReplay(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d — body: %s", rec.Code, rec.Body.String())
	}
}

func TestGetReplay_UnfinishedRoom_Returns404(t *testing.T) {
	// A lobby room is never returned by GetFinishedRoomByCode → 404 with
	// "not_found" is the correct behavior (replay is finished-only).
	h, q, user, room := seedLobbyRoom(t,
		`{"round_count":3,"round_duration_seconds":60,"voting_duration_seconds":30}`)
	_ = q

	req := replayRequest(room.Code)
	req = withUser(req, user.ID.String(), user.Username, user.Email, user.Role)
	rec := httptest.NewRecorder()
	h.GetReplay(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404 for lobby room, got %d — body: %s", rec.Code, rec.Body.String())
	}
	var body map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["code"] != "not_found" {
		t.Fatalf("want code=not_found, got %q", body["code"])
	}
}

func TestGetReplay_NonMember_Returns403(t *testing.T) {
	h, q := newRoomHandler(t)
	host := seedRoomUser(t, q)
	// seedRoomUser keys off t.Name(); make the outsider distinct.
	slug := testutil.SeedName(t) + "_outsider"
	outsider, err := q.CreateUser(context.Background(), db.CreateUserParams{
		Username:  slug,
		Email:     slug + "@test.com",
		Role:      "player",
		IsActive:  true,
		ConsentAt: time.Now().UTC(),
		Locale:    "en",
	})
	if err != nil {
		t.Fatalf("create outsider: %v", err)
	}
	room := seedFinishedReplayRoom(t, q, host, true)

	req := replayRequest(room.Code)
	req = withUser(req, outsider.ID.String(), outsider.Username, outsider.Email, outsider.Role)
	rec := httptest.NewRecorder()
	h.GetReplay(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d — body: %s", rec.Code, rec.Body.String())
	}
	var body map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["code"] != "not_a_player" {
		t.Fatalf("want code=not_a_player, got %q", body["code"])
	}
}

func TestGetReplay_AdminBypass_Returns200(t *testing.T) {
	h, q := newRoomHandler(t)
	host := seedRoomUser(t, q)
	slug := testutil.SeedName(t) + "_adm"
	admin, err := q.CreateUser(context.Background(), db.CreateUserParams{
		Username:  slug,
		Email:     slug + "@test.com",
		Role:      "admin",
		IsActive:  true,
		ConsentAt: time.Now().UTC(),
		Locale:    "en",
	})
	if err != nil {
		t.Fatalf("create admin: %v", err)
	}
	room := seedFinishedReplayRoom(t, q, host, true)

	req := replayRequest(room.Code)
	req = withUser(req, admin.ID.String(), admin.Username, admin.Email, admin.Role)
	rec := httptest.NewRecorder()
	h.GetReplay(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("admin should see replay, got %d — body: %s", rec.Code, rec.Body.String())
	}
}

func TestGetReplay_HappyPath_MemeFreestyle(t *testing.T) {
	h, q := newRoomHandler(t)
	ctx := context.Background()
	host := seedRoomUser(t, q)

	slug := testutil.SeedName(t) + "_p2"
	p2, err := q.CreateUser(ctx, db.CreateUserParams{
		Username: slug, Email: slug + "@test.com", Role: "player",
		IsActive: true, ConsentAt: time.Now().UTC(),
		Locale: "en",
	})
	if err != nil {
		t.Fatalf("create p2: %v", err)
	}

	room := seedFinishedReplayRoom(t, q, host, true)
	if err := q.UpsertRoomPlayer(ctx, db.UpsertRoomPlayerParams{
		RoomID: room.ID,
		UserID: pgtype.UUID{Bytes: p2.ID, Valid: true},
	}); err != nil {
		t.Fatalf("add p2 as player: %v", err)
	}

	rnd, err := q.GetCurrentRound(ctx, room.ID)
	if err != nil {
		t.Fatalf("get current round: %v", err)
	}
	subHost, err := q.CreateSubmission(ctx, db.CreateSubmissionParams{
		RoundID: rnd.ID,
		UserID:  pgtype.UUID{Bytes: host.ID, Valid: true},
		Payload: json.RawMessage(`{"caption":"hello"}`),
	})
	if err != nil {
		t.Fatalf("host submission: %v", err)
	}
	if _, err := q.CreateSubmission(ctx, db.CreateSubmissionParams{
		RoundID: rnd.ID,
		UserID:  pgtype.UUID{Bytes: p2.ID, Valid: true},
		Payload: json.RawMessage(`{"caption":"world"}`),
	}); err != nil {
		t.Fatalf("p2 submission: %v", err)
	}
	if _, err := q.CreateVote(ctx, db.CreateVoteParams{
		SubmissionID: subHost.ID,
		VoterID:      pgtype.UUID{Bytes: p2.ID, Valid: true},
		Value:        json.RawMessage(`{"points":1}`),
	}); err != nil {
		t.Fatalf("vote: %v", err)
	}
	if _, err := q.UpdatePlayerScore(ctx, db.UpdatePlayerScoreParams{
		RoomID: room.ID,
		UserID: pgtype.UUID{Bytes: host.ID, Valid: true},
		Score:  1,
	}); err != nil {
		t.Fatalf("score host: %v", err)
	}

	req := replayRequest(room.Code)
	req = withUser(req, host.ID.String(), host.Username, host.Email, host.Role)
	rec := httptest.NewRecorder()
	h.GetReplay(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d — body: %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Room struct {
			Code        string `json:"code"`
			PlayerCount int64  `json:"player_count"`
		} `json:"room"`
		Rounds []struct {
			RoundNumber int32           `json:"round_number"`
			Prompt      json.RawMessage `json:"prompt"`
			Submissions []struct {
				ID     string `json:"id"`
				Author struct {
					DisplayName string `json:"display_name"`
					Kind        string `json:"kind"`
				} `json:"author"`
				Payload       json.RawMessage `json:"payload"`
				VotesReceived int32           `json:"votes_received"`
				PointsAwarded int32           `json:"points_awarded"`
			} `json:"submissions"`
		} `json:"rounds"`
		Leaderboard []struct {
			Rank        int    `json:"rank"`
			DisplayName string `json:"display_name"`
			Score       int32  `json:"score"`
		} `json:"leaderboard"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Room.PlayerCount != 2 {
		t.Fatalf("player_count: want 2, got %d", body.Room.PlayerCount)
	}
	if len(body.Rounds) != 1 {
		t.Fatalf("want 1 round, got %d", len(body.Rounds))
	}
	if len(body.Rounds[0].Submissions) != 2 {
		t.Fatalf("want 2 submissions, got %d", len(body.Rounds[0].Submissions))
	}
	var hostSubIdx = -1
	for i, s := range body.Rounds[0].Submissions {
		if s.Author.DisplayName == host.Username {
			hostSubIdx = i
		}
	}
	if hostSubIdx < 0 {
		t.Fatal("host submission missing")
	}
	hs := body.Rounds[0].Submissions[hostSubIdx]
	if hs.VotesReceived != 1 || hs.PointsAwarded != 1 {
		t.Fatalf("host sub: want 1v/1p, got v=%d p=%d", hs.VotesReceived, hs.PointsAwarded)
	}
	// Prompt payload must include media_key + payload_version from the seeded version.
	var promptMap map[string]any
	_ = json.Unmarshal(body.Rounds[0].Prompt, &promptMap)
	if _, ok := promptMap["media_key"].(string); !ok {
		t.Errorf("prompt missing media_key: %s", string(body.Rounds[0].Prompt))
	}
	if _, ok := promptMap["payload_version"]; !ok {
		t.Errorf("prompt missing payload_version: %s", string(body.Rounds[0].Prompt))
	}
	if len(body.Leaderboard) != 2 {
		t.Fatalf("want 2 leaderboard rows, got %d", len(body.Leaderboard))
	}
	if body.Leaderboard[0].Rank != 1 || body.Leaderboard[0].DisplayName != host.Username {
		t.Fatalf("want host at rank 1, got %+v", body.Leaderboard[0])
	}
}

// seedFinishedShowdownRoom builds a finished meme-showdown room with one
// image pack (seeded as prompt item) + one text pack (caller provides the
// pre-seeded item). One started round using the image prompt. The test then
// attaches submissions referencing items in the text pack by card_id.
func seedFinishedShowdownRoom(t *testing.T, q *db.Queries, host db.User, textPackID [16]byte) db.Room {
	t.Helper()
	ctx := context.Background()

	imgPack, err := q.CreatePack(ctx, db.CreatePackParams{
		Name:       testutil.SeedName(t) + "_" + host.Username + "_img",
		Visibility: "private",
		Language:   "en",
	})
	if err != nil {
		t.Fatalf("create image pack: %v", err)
	}
	imgItem, err := q.CreateItem(ctx, db.CreateItemParams{
		PackID: imgPack.ID, Name: "prompt", PayloadVersion: 1,
	})
	if err != nil {
		t.Fatalf("create image item: %v", err)
	}
	mediaKey := fmt.Sprintf("test/%s/prompt.png", imgPack.ID)
	ver, err := q.CreateItemVersion(ctx, db.CreateItemVersionParams{
		ItemID: imgItem.ID, MediaKey: &mediaKey, Payload: json.RawMessage(`{"caption":"prompt"}`),
	})
	if err != nil {
		t.Fatalf("create image version: %v", err)
	}
	if _, err := q.SetCurrentVersion(ctx, db.SetCurrentVersionParams{
		ID: imgItem.ID, CurrentVersionID: pgtype.UUID{Bytes: ver.ID, Valid: true},
	}); err != nil {
		t.Fatalf("set image version: %v", err)
	}

	gt, err := q.GetGameTypeBySlug(ctx, "meme-showdown")
	if err != nil {
		t.Fatalf("showdown game type: %v", err)
	}
	code := fmt.Sprintf("S-%s-%s-%d", testutil.SeedName(t), host.Username, time.Now().UnixNano())
	room, err := q.CreateRoom(ctx, db.CreateRoomParams{
		Code:       code,
		GameTypeID: gt.ID,
		PackID:     imgPack.ID,
		TextPackID: pgtype.UUID{Bytes: textPackID, Valid: true},
		HostID:     pgtype.UUID{Bytes: host.ID, Valid: true},
		Mode:       "multiplayer",
		Config:     json.RawMessage(`{"round_count":1,"round_duration_seconds":30,"voting_duration_seconds":15}`),
	})
	if err != nil {
		t.Fatalf("create showdown room: %v", err)
	}
	if err := q.UpsertRoomPlayer(ctx, db.UpsertRoomPlayerParams{
		RoomID: room.ID, UserID: pgtype.UUID{Bytes: host.ID, Valid: true},
	}); err != nil {
		t.Fatalf("upsert host: %v", err)
	}
	rnd, err := q.CreateRound(ctx, db.CreateRoundParams{
		RoomID: room.ID, ItemID: imgItem.ID,
	})
	if err != nil {
		t.Fatalf("create round: %v", err)
	}
	if _, err := q.StartRound(ctx, rnd.ID); err != nil {
		t.Fatalf("start round: %v", err)
	}
	if _, err := q.SetRoomState(ctx, db.SetRoomStateParams{
		ID: room.ID, State: "finished",
	}); err != nil {
		t.Fatalf("set finished: %v", err)
	}
	return room
}

func TestGetReplay_MemeShowdown_ResolvesCardText(t *testing.T) {
	h, q := newRoomHandler(t)
	ctx := context.Background()
	host := seedRoomUser(t, q)

	// Text pack with one card whose text is "FUNNY". payload_version=2 is the
	// text-caption shape the handler declares; real item-creation path sets it
	// on game_items, not on game_item_versions.
	textPack, err := q.CreatePack(ctx, db.CreatePackParams{
		Name:       testutil.SeedName(t) + "_txt",
		Visibility: "private",
		Language:   "en",
	})
	if err != nil {
		t.Fatalf("create text pack: %v", err)
	}
	textItem, err := q.CreateItem(ctx, db.CreateItemParams{
		PackID: textPack.ID, Name: "card", PayloadVersion: 2,
	})
	if err != nil {
		t.Fatalf("create text item: %v", err)
	}
	ver, err := q.CreateItemVersion(ctx, db.CreateItemVersionParams{
		ItemID: textItem.ID, Payload: json.RawMessage(`{"text":"FUNNY"}`),
	})
	if err != nil {
		t.Fatalf("create text version: %v", err)
	}
	if _, err := q.SetCurrentVersion(ctx, db.SetCurrentVersionParams{
		ID: textItem.ID, CurrentVersionID: pgtype.UUID{Bytes: ver.ID, Valid: true},
	}); err != nil {
		t.Fatalf("set text version: %v", err)
	}

	room := seedFinishedShowdownRoom(t, q, host, textPack.ID)
	rnd, err := q.GetCurrentRound(ctx, room.ID)
	if err != nil {
		t.Fatalf("get round: %v", err)
	}
	cardID := textItem.ID.String()
	if _, err := q.CreateSubmission(ctx, db.CreateSubmissionParams{
		RoundID: rnd.ID,
		UserID:  pgtype.UUID{Bytes: host.ID, Valid: true},
		Payload: json.RawMessage(`{"card_id":"` + cardID + `"}`),
	}); err != nil {
		t.Fatalf("create submission: %v", err)
	}

	req := replayRequest(room.Code)
	req = withUser(req, host.ID.String(), host.Username, host.Email, host.Role)
	rec := httptest.NewRecorder()
	h.GetReplay(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d — body: %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Rounds []struct {
			Submissions []struct {
				Payload json.RawMessage `json:"payload"`
			} `json:"submissions"`
		} `json:"rounds"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Rounds) != 1 || len(body.Rounds[0].Submissions) != 1 {
		t.Fatalf("unexpected replay shape: rounds=%d subs=%d", len(body.Rounds),
			func() int {
				if len(body.Rounds) == 0 {
					return 0
				}
				return len(body.Rounds[0].Submissions)
			}())
	}
	var payload map[string]any
	_ = json.Unmarshal(body.Rounds[0].Submissions[0].Payload, &payload)
	if payload["text"] != "FUNNY" {
		t.Fatalf("want resolved text=FUNNY, got %v (payload: %s)", payload["text"], body.Rounds[0].Submissions[0].Payload)
	}
	if payload["card_id"] != cardID {
		t.Fatalf("card_id should be preserved, got %v", payload["card_id"])
	}
}

func TestGetReplay_Member_Returns200(t *testing.T) {
	h, q := newRoomHandler(t)
	user := seedRoomUser(t, q)
	room := seedFinishedReplayRoom(t, q, user, true)

	req := replayRequest(room.Code)
	req = withUser(req, user.ID.String(), user.Username, user.Email, user.Role)
	rec := httptest.NewRecorder()
	h.GetReplay(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d — body: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Room struct {
			Code         string `json:"code"`
			GameTypeSlug string `json:"game_type_slug"`
			PackName     string `json:"pack_name"`
		} `json:"room"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Room.Code != room.Code {
		t.Fatalf("want code=%s, got %q", room.Code, resp.Room.Code)
	}
	if resp.Room.GameTypeSlug != "meme-freestyle" {
		t.Fatalf("want game_type_slug=meme-freestyle, got %q", resp.Room.GameTypeSlug)
	}
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// decodeReplay is a local helper that decodes just the `rounds` slice from
// the replay response — enough to inspect submission authors in edge cases.
func decodeReplayRounds(t *testing.T, body []byte) []struct {
	Submissions []struct {
		Author struct {
			DisplayName string `json:"display_name"`
			Kind        string `json:"kind"`
		} `json:"author"`
	} `json:"submissions"`
} {
	t.Helper()
	var resp struct {
		Rounds []struct {
			Submissions []struct {
				Author struct {
					DisplayName string `json:"display_name"`
					Kind        string `json:"kind"`
				} `json:"author"`
			} `json:"submissions"`
		} `json:"rounds"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return resp.Rounds
}

func TestGetReplay_EmptyRound_RendersWithNoSubmissions(t *testing.T) {
	h, q := newRoomHandler(t)
	user := seedRoomUser(t, q)
	room := seedFinishedReplayRoom(t, q, user, true) // started, no submissions

	req := replayRequest(room.Code)
	req = withUser(req, user.ID.String(), user.Username, user.Email, user.Role)
	rec := httptest.NewRecorder()
	h.GetReplay(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d — body: %s", rec.Code, rec.Body.String())
	}

	rounds := decodeReplayRounds(t, rec.Body.Bytes())
	if len(rounds) != 1 {
		t.Fatalf("want 1 round, got %d", len(rounds))
	}
	if len(rounds[0].Submissions) != 0 {
		t.Fatalf("want 0 submissions, got %d", len(rounds[0].Submissions))
	}
}

func TestGetReplay_DeletedAuthor_RendersAsDeleted(t *testing.T) {
	h, q := newRoomHandler(t)
	ctx := context.Background()
	host := seedRoomUser(t, q)

	slug := testutil.SeedName(t) + "_victim"
	victim, err := q.CreateUser(ctx, db.CreateUserParams{
		Username: slug, Email: slug + "@test.com", Role: "player",
		IsActive: true, ConsentAt: time.Now().UTC(),
		Locale: "en",
	})
	if err != nil {
		t.Fatalf("create victim: %v", err)
	}

	room := seedFinishedReplayRoom(t, q, host, true)
	if err := q.UpsertRoomPlayer(ctx, db.UpsertRoomPlayerParams{
		RoomID: room.ID, UserID: pgtype.UUID{Bytes: victim.ID, Valid: true},
	}); err != nil {
		t.Fatalf("add victim: %v", err)
	}
	rnd, err := q.GetCurrentRound(ctx, room.ID)
	if err != nil {
		t.Fatalf("get round: %v", err)
	}
	if _, err := q.CreateSubmission(ctx, db.CreateSubmissionParams{
		RoundID: rnd.ID,
		UserID:  pgtype.UUID{Bytes: victim.ID, Valid: true},
		Payload: json.RawMessage(`{"caption":"bye"}`),
	}); err != nil {
		t.Fatalf("submission: %v", err)
	}

	// GDPR hard-delete path: sentinel-swap then drop the user row.
	if err := q.UpdateSubmissionsSentinel(ctx, pgtype.UUID{Bytes: victim.ID, Valid: true}); err != nil {
		t.Fatalf("sentinel submissions: %v", err)
	}
	if err := q.UpdateVotesSentinel(ctx, pgtype.UUID{Bytes: victim.ID, Valid: true}); err != nil {
		t.Fatalf("sentinel votes: %v", err)
	}
	if err := q.HardDeleteUser(ctx, victim.ID); err != nil {
		t.Fatalf("hard delete: %v", err)
	}

	req := replayRequest(room.Code)
	req = withUser(req, host.ID.String(), host.Username, host.Email, host.Role)
	rec := httptest.NewRecorder()
	h.GetReplay(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d — body: %s", rec.Code, rec.Body.String())
	}

	rounds := decodeReplayRounds(t, rec.Body.Bytes())
	if len(rounds) != 1 || len(rounds[0].Submissions) != 1 {
		t.Fatalf("want 1 round with 1 submission, got rounds=%d", len(rounds))
	}
	got := rounds[0].Submissions[0].Author
	if got.Kind != "deleted" || got.DisplayName != "[deleted]" {
		t.Fatalf("want kind=deleted name=[deleted], got %+v", got)
	}
}

func TestGetReplay_GuestAuthor_RendersAsGuest(t *testing.T) {
	h, q := newRoomHandler(t)
	ctx := context.Background()
	host := seedRoomUser(t, q)
	room := seedFinishedReplayRoom(t, q, host, true)

	guest, err := q.CreateGuestPlayer(ctx, db.CreateGuestPlayerParams{
		RoomID:      room.ID,
		DisplayName: "carol",
		TokenHash:   sha256Hex("t1"),
	})
	if err != nil {
		t.Fatalf("create guest: %v", err)
	}
	if _, err := q.AddGuestRoomPlayer(ctx, db.AddGuestRoomPlayerParams{
		RoomID:        room.ID,
		GuestPlayerID: pgtype.UUID{Bytes: guest.ID, Valid: true},
	}); err != nil {
		t.Fatalf("add guest: %v", err)
	}
	rnd, err := q.GetCurrentRound(ctx, room.ID)
	if err != nil {
		t.Fatalf("get round: %v", err)
	}
	if _, err := q.CreateGuestSubmission(ctx, db.CreateGuestSubmissionParams{
		RoundID:       rnd.ID,
		GuestPlayerID: pgtype.UUID{Bytes: guest.ID, Valid: true},
		Payload:       json.RawMessage(`{"caption":"guest says hi"}`),
	}); err != nil {
		t.Fatalf("guest submission: %v", err)
	}

	req := replayRequest(room.Code)
	req = withUser(req, host.ID.String(), host.Username, host.Email, host.Role)
	rec := httptest.NewRecorder()
	h.GetReplay(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d — body: %s", rec.Code, rec.Body.String())
	}

	rounds := decodeReplayRounds(t, rec.Body.Bytes())
	if len(rounds) != 1 || len(rounds[0].Submissions) != 1 {
		t.Fatalf("want 1 round with 1 submission, got rounds=%d", len(rounds))
	}
	got := rounds[0].Submissions[0].Author
	if got.Kind != "guest" || got.DisplayName != "carol" {
		t.Fatalf("want kind=guest name=carol, got %+v", got)
	}
}
