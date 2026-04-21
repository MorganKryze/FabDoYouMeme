package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/testutil"
)

// roomCodeSeq gives each roomInState call a process-unique suffix so
// rapid-fire test calls cannot collide on the rooms.code unique index.
// Relying on UnixNano()%1000 (as some older helpers do) is fine when a
// single test seeds one room, but this file seeds four in quick
// succession and the mod-1000 window collapses.
var roomCodeSeq uint64

// roomInState seeds a room in the requested state. It reuses the room handler
// helpers from rooms_test.go (same package) and flips the state via a direct
// SetRoomState call so we don't have to simulate the full lifecycle just to
// assert a guard.
func roomInState(t *testing.T, q *db.Queries, host db.User, state string) db.Room {
	t.Helper()
	ctx := context.Background()
	gt, err := q.GetGameTypeBySlug(ctx, "meme-freestyle")
	if err != nil {
		t.Fatalf("roomInState: get game type: %v", err)
	}
	pack, err := q.CreatePack(ctx, db.CreatePackParams{
		Name:       testutil.SeedName(t) + "_rs",
		Visibility: "private",
		Language:   "en",
	})
	if err != nil {
		t.Fatalf("roomInState: create pack: %v", err)
	}
	code := fmt.Sprintf("R%d-%d", time.Now().UnixNano(), atomic.AddUint64(&roomCodeSeq, 1))
	room, err := q.CreateRoom(ctx, db.CreateRoomParams{
		Code:       code,
		GameTypeID: gt.ID,
		PackID:     pack.ID,
		HostID:     pgtype.UUID{Bytes: host.ID, Valid: true},
		Mode:       "multiplayer",
		Config:     json.RawMessage(`{"round_count":3,"round_duration_seconds":60,"voting_duration_seconds":30}`),
	})
	if err != nil {
		t.Fatalf("roomInState: create room: %v", err)
	}
	if state != "lobby" {
		updated, err := q.SetRoomState(ctx, db.SetRoomStateParams{
			ID:    room.ID,
			State: state,
		})
		if err != nil {
			t.Fatalf("roomInState: SetRoomState(%s): %v", state, err)
		}
		room.State = updated.State
	}
	return room
}

// TestAPI_LeaveRejectsPlaying is the P2.4 acceptance test (finding 3.E). Docs
// say Leave is lobby-only but the pre-fix handler accepted leave in any state.
// After the fix, leaving a `playing` room returns 409 room_not_in_lobby.
func TestAPI_LeaveRejectsPlaying(t *testing.T) {
	h, q := newRoomHandler(t)
	host := seedRoomUser(t, q)
	room := roomInState(t, q, host, "playing")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("code", room.Code)
	req := httptest.NewRequest(http.MethodPost, "/api/rooms/"+room.Code+"/leave", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = withUser(req, host.ID.String(), host.Username, host.Email, host.Role)

	rec := httptest.NewRecorder()
	h.Leave(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d — body: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "room_not_in_lobby" {
		t.Fatalf("want code=room_not_in_lobby, got %q", resp["code"])
	}
}

// TestAPI_LeaveHostClosesRoom documents the other half of 3.E: when the host
// leaves a lobby, the room transitions to finished so no later join can
// race into an orphaned lobby.
func TestAPI_LeaveHostClosesRoom(t *testing.T) {
	h, q := newRoomHandler(t)
	host := seedRoomUser(t, q)
	room := roomInState(t, q, host, "lobby")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("code", room.Code)
	req := httptest.NewRequest(http.MethodPost, "/api/rooms/"+room.Code+"/leave", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = withUser(req, host.ID.String(), host.Username, host.Email, host.Role)

	rec := httptest.NewRecorder()
	h.Leave(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d — body: %s", rec.Code, rec.Body.String())
	}

	updated, err := q.GetRoomByCode(context.Background(), room.Code)
	if err != nil {
		t.Fatalf("GetRoomByCode: %v", err)
	}
	if updated.State != "finished" {
		t.Fatalf("want room.state=finished after host leave, got %q", updated.State)
	}
}

// TestAPI_LeaderboardRejectsUnfinished is the P2.5 acceptance test (finding
// 3.F). The leaderboard query runs fine in any state; the guard is at the
// handler. Pre-fix, a mid-game request leaked live scores.
func TestAPI_LeaderboardRejectsUnfinished(t *testing.T) {
	h, q := newRoomHandler(t)
	host := seedRoomUser(t, q)
	room := roomInState(t, q, host, "playing")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("code", room.Code)
	req := httptest.NewRequest(http.MethodGet, "/api/rooms/"+room.Code+"/leaderboard", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = withUser(req, host.ID.String(), host.Username, host.Email, host.Role)

	rec := httptest.NewRecorder()
	h.Leaderboard(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d — body: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "room_not_finished" {
		t.Fatalf("want code=room_not_finished, got %q", resp["code"])
	}
}

// TestAPI_LeaderboardAllowsFinished is the happy-path counter-test for 3.F —
// confirms the guard lets finished rooms through unchanged.
func TestAPI_LeaderboardAllowsFinished(t *testing.T) {
	h, q := newRoomHandler(t)
	host := seedRoomUser(t, q)
	room := roomInState(t, q, host, "finished")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("code", room.Code)
	req := httptest.NewRequest(http.MethodGet, "/api/rooms/"+room.Code+"/leaderboard", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = withUser(req, host.ID.String(), host.Username, host.Email, host.Role)

	rec := httptest.NewRecorder()
	h.Leaderboard(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200 on finished room, got %d — body: %s", rec.Code, rec.Body.String())
	}
}
