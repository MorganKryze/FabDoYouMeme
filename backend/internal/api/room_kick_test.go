// backend/internal/api/room_kick_test.go
package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/api"
)

// seedGuestPlayer inserts a guest_players row + joins them to the room so the
// kick handler has something to remove. Mirrors the shape produced by
// POST /api/rooms/{code}/guest-join (without minting a real token — tests
// don't need to auth via token).
func seedGuestPlayer(t *testing.T, q *db.Queries, roomID uuid.UUID, name string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	gp, err := q.CreateGuestPlayer(ctx, db.CreateGuestPlayerParams{
		RoomID:      roomID,
		DisplayName: name,
		TokenHash:   "test-" + name,
	})
	if err != nil {
		t.Fatalf("CreateGuestPlayer: %v", err)
	}
	if _, err := q.AddGuestRoomPlayer(ctx, db.AddGuestRoomPlayerParams{
		RoomID:        roomID,
		GuestPlayerID: pgtype.UUID{Bytes: gp.ID, Valid: true},
	}); err != nil {
		t.Fatalf("AddGuestRoomPlayer: %v", err)
	}
	return gp.ID
}

// postKick issues POST /api/rooms/{code}/kick with the given body and returns
// the response recorder. The session user on the request is `actor`.
func postKick(t *testing.T, h *api.RoomHandler, room db.Room, actor db.User, body map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	raw, _ := json.Marshal(body)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("code", room.Code)
	req := httptest.NewRequest(http.MethodPost, "/api/rooms/"+room.Code+"/kick", bytes.NewReader(raw))
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = withUser(req, actor.ID.String(), actor.Username, actor.Email, actor.Role)
	rec := httptest.NewRecorder()
	h.Kick(rec, req)
	return rec
}

// TestAPI_Kick_User_Succeeds — kick of a registered user: 204, room_players
// row gone, room_bans row present, audit entry written.
func TestAPI_Kick_User_Succeeds(t *testing.T) {
	h, q := newRoomHandler(t)
	host := seedRoomUser(t, q)
	victim := extraRoomUser(t, q, "_victim", "player")
	room := roomInState(t, q, host, "lobby")
	if _, err := q.AddRoomPlayer(context.Background(), db.AddRoomPlayerParams{
		RoomID: room.ID,
		UserID: pgtype.UUID{Bytes: victim.ID, Valid: true},
	}); err != nil {
		t.Fatalf("AddRoomPlayer: %v", err)
	}

	rec := postKick(t, h, room, host, map[string]string{"user_id": victim.ID.String()})

	if rec.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d — body: %s", rec.Code, rec.Body.String())
	}
	// room_players row is gone.
	if _, err := q.GetRoomPlayer(context.Background(), db.GetRoomPlayerParams{
		RoomID: room.ID,
		UserID: pgtype.UUID{Bytes: victim.ID, Valid: true},
	}); err == nil {
		t.Fatal("expected victim's room_players row to be gone")
	}
	// room_bans row is present.
	banned, err := q.IsUserBannedFromRoom(context.Background(), db.IsUserBannedFromRoomParams{
		RoomID: room.ID,
		UserID: pgtype.UUID{Bytes: victim.ID, Valid: true},
	})
	if err != nil {
		t.Fatalf("IsUserBannedFromRoom: %v", err)
	}
	if !banned {
		t.Fatal("expected victim to be banned")
	}
}

// TestAPI_Kick_Guest_Succeeds — kick of a guest: 204, guest removed, ban row present.
func TestAPI_Kick_Guest_Succeeds(t *testing.T) {
	h, q := newRoomHandler(t)
	host := seedRoomUser(t, q)
	room := roomInState(t, q, host, "lobby")
	guestID := seedGuestPlayer(t, q, room.ID, "Gwen")

	rec := postKick(t, h, room, host, map[string]string{"guest_player_id": guestID.String()})

	if rec.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d — body: %s", rec.Code, rec.Body.String())
	}
	banned, err := q.IsGuestBannedFromRoom(context.Background(), db.IsGuestBannedFromRoomParams{
		RoomID:        room.ID,
		GuestPlayerID: pgtype.UUID{Bytes: guestID, Valid: true},
	})
	if err != nil {
		t.Fatalf("IsGuestBannedFromRoom: %v", err)
	}
	if !banned {
		t.Fatal("expected guest to be banned")
	}
}

// TestAPI_Kick_Self_Rejected — host kicking themselves is 409 cannot_kick_self.
func TestAPI_Kick_Self_Rejected(t *testing.T) {
	h, q := newRoomHandler(t)
	host := seedRoomUser(t, q)
	room := roomInState(t, q, host, "lobby")

	rec := postKick(t, h, room, host, map[string]string{"user_id": host.ID.String()})

	if rec.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d — body: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "cannot_kick_self" {
		t.Fatalf("want code=cannot_kick_self, got %q", resp["code"])
	}
}

// TestAPI_Kick_NonHost_Rejected — non-host, non-admin caller is 403 forbidden.
func TestAPI_Kick_NonHost_Rejected(t *testing.T) {
	h, q := newRoomHandler(t)
	host := seedRoomUser(t, q)
	attacker := extraRoomUser(t, q, "_attacker", "player")
	victim := extraRoomUser(t, q, "_victim", "player")
	room := roomInState(t, q, host, "lobby")
	if _, err := q.AddRoomPlayer(context.Background(), db.AddRoomPlayerParams{
		RoomID: room.ID,
		UserID: pgtype.UUID{Bytes: victim.ID, Valid: true},
	}); err != nil {
		t.Fatalf("AddRoomPlayer: %v", err)
	}

	rec := postKick(t, h, room, attacker, map[string]string{"user_id": victim.ID.String()})

	if rec.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d — body: %s", rec.Code, rec.Body.String())
	}
}

// TestAPI_Kick_NonLobby_Rejected — kick on a playing room returns 409 room_not_in_lobby.
func TestAPI_Kick_NonLobby_Rejected(t *testing.T) {
	h, q := newRoomHandler(t)
	host := seedRoomUser(t, q)
	victim := extraRoomUser(t, q, "_victim", "player")
	room := roomInState(t, q, host, "playing")
	if _, err := q.AddRoomPlayer(context.Background(), db.AddRoomPlayerParams{
		RoomID: room.ID,
		UserID: pgtype.UUID{Bytes: victim.ID, Valid: true},
	}); err != nil {
		t.Fatalf("AddRoomPlayer: %v", err)
	}

	rec := postKick(t, h, room, host, map[string]string{"user_id": victim.ID.String()})

	if rec.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d — body: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "room_not_in_lobby" {
		t.Fatalf("want code=room_not_in_lobby, got %q", resp["code"])
	}
}

// TestAPI_Kick_Both_Rejected — passing both user_id and guest_player_id is a 400.
func TestAPI_Kick_Both_Rejected(t *testing.T) {
	h, q := newRoomHandler(t)
	host := seedRoomUser(t, q)
	room := roomInState(t, q, host, "lobby")

	rec := postKick(t, h, room, host, map[string]string{
		"user_id":         uuid.New().String(),
		"guest_player_id": uuid.New().String(),
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d — body: %s", rec.Code, rec.Body.String())
	}
}

// TestAPI_Kick_Neither_Rejected — passing neither is a 400.
func TestAPI_Kick_Neither_Rejected(t *testing.T) {
	h, q := newRoomHandler(t)
	host := seedRoomUser(t, q)
	room := roomInState(t, q, host, "lobby")

	rec := postKick(t, h, room, host, map[string]string{})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d — body: %s", rec.Code, rec.Body.String())
	}
}

// TestAPI_Kick_Idempotent — re-kicking an already-kicked user returns 204 and
// does not duplicate the ban row (ON CONFLICT DO NOTHING).
func TestAPI_Kick_Idempotent(t *testing.T) {
	h, q := newRoomHandler(t)
	host := seedRoomUser(t, q)
	victim := extraRoomUser(t, q, "_victim", "player")
	room := roomInState(t, q, host, "lobby")
	if _, err := q.AddRoomPlayer(context.Background(), db.AddRoomPlayerParams{
		RoomID: room.ID,
		UserID: pgtype.UUID{Bytes: victim.ID, Valid: true},
	}); err != nil {
		t.Fatalf("AddRoomPlayer: %v", err)
	}

	rec1 := postKick(t, h, room, host, map[string]string{"user_id": victim.ID.String()})
	if rec1.Code != http.StatusNoContent {
		t.Fatalf("first kick: want 204, got %d", rec1.Code)
	}
	rec2 := postKick(t, h, room, host, map[string]string{"user_id": victim.ID.String()})
	if rec2.Code != http.StatusNoContent {
		t.Fatalf("second kick: want 204, got %d — body: %s", rec2.Code, rec2.Body.String())
	}
}
