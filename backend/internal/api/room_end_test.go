package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/testutil"
)

// extraRoomUser seeds a second/third user for tests that need more than
// one account in the same test (seedRoomUser uses testutil.SeedName, which
// is deterministic per-test and collides on the unique username index when
// called twice).
func extraRoomUser(t *testing.T, q *db.Queries, suffix string, role string) db.User {
	t.Helper()
	slug := testutil.SeedName(t) + suffix
	u, err := q.CreateUser(context.Background(), db.CreateUserParams{
		Username:  slug,
		Email:     slug + "@test.com",
		Role:      role,
		IsActive:  true,
		ConsentAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("extraRoomUser(%s): %v", suffix, err)
	}
	return u
}

// endRequest builds an http.Request with a chi route context for the
// /end endpoint. Keeps the assertion tests below terse.
func endRequest(t *testing.T, room db.Room, caller db.User) *http.Request {
	t.Helper()
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("code", room.Code)
	req := httptest.NewRequest(http.MethodPost, "/api/rooms/"+room.Code+"/end", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = withUser(req, caller.ID.String(), caller.Username, caller.Email, caller.Role)
	return req
}

func TestAPI_EndRoom_HostInLobby_Succeeds(t *testing.T) {
	h, q := newRoomHandler(t)
	host := seedRoomUser(t, q)
	room := roomInState(t, q, host, "lobby")

	rec := httptest.NewRecorder()
	h.End(rec, endRequest(t, room, host))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d — body: %s", rec.Code, rec.Body.String())
	}
	updated, err := q.GetRoomByCode(context.Background(), room.Code)
	if err != nil {
		t.Fatalf("GetRoomByCode: %v", err)
	}
	if updated.State != "finished" {
		t.Fatalf("want room.state=finished, got %q", updated.State)
	}
}

func TestAPI_EndRoom_HostDuringPlaying_Succeeds(t *testing.T) {
	h, q := newRoomHandler(t)
	host := seedRoomUser(t, q)
	room := roomInState(t, q, host, "playing")

	rec := httptest.NewRecorder()
	h.End(rec, endRequest(t, room, host))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d — body: %s", rec.Code, rec.Body.String())
	}
	updated, _ := q.GetRoomByCode(context.Background(), room.Code)
	if updated.State != "finished" {
		t.Fatalf("want room.state=finished, got %q", updated.State)
	}
}

func TestAPI_EndRoom_AdminNonHost_Succeeds(t *testing.T) {
	h, q := newRoomHandler(t)
	host := seedRoomUser(t, q)
	room := roomInState(t, q, host, "playing")

	admin := extraRoomUser(t, q, "_adm", "admin")

	rec := httptest.NewRecorder()
	h.End(rec, endRequest(t, room, admin))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d — body: %s", rec.Code, rec.Body.String())
	}
}

func TestAPI_EndRoom_NonHostNonAdmin_Returns403(t *testing.T) {
	h, q := newRoomHandler(t)
	host := seedRoomUser(t, q)
	room := roomInState(t, q, host, "lobby")

	intruder := extraRoomUser(t, q, "_int", "player")

	rec := httptest.NewRecorder()
	h.End(rec, endRequest(t, room, intruder))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d — body: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "forbidden" {
		t.Fatalf("want code=forbidden, got %q", resp["code"])
	}
}

func TestAPI_EndRoom_AlreadyFinished_Returns409(t *testing.T) {
	h, q := newRoomHandler(t)
	host := seedRoomUser(t, q)
	room := roomInState(t, q, host, "finished")

	rec := httptest.NewRecorder()
	h.End(rec, endRequest(t, room, host))

	if rec.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d — body: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "room_already_finished" {
		t.Fatalf("want code=room_already_finished, got %q", resp["code"])
	}
}

func TestAPI_EndRoom_RoomNotFound_Returns404(t *testing.T) {
	h, _ := newRoomHandler(t)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("code", "NOT-A-REAL-CODE")
	req := httptest.NewRequest(http.MethodPost, "/api/rooms/NOT-A-REAL-CODE/end", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = withUser(req, "00000000-0000-0000-0000-000000000000", "ghost", "g@x", "player")

	rec := httptest.NewRecorder()
	h.End(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d — body: %s", rec.Code, rec.Body.String())
	}
}

func TestAPI_EndRoom_Unauthenticated_Returns401(t *testing.T) {
	h, q := newRoomHandler(t)
	host := seedRoomUser(t, q)
	room := roomInState(t, q, host, "lobby")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("code", room.Code)
	// Intentionally do NOT inject a user — withUser is skipped.
	req := httptest.NewRequest(http.MethodPost, "/api/rooms/"+room.Code+"/end", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.End(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d — body: %s", rec.Code, rec.Body.String())
	}
}
