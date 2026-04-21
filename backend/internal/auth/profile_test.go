// backend/internal/auth/profile_test.go

package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/middleware"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/testutil"
)

func requestWithUser(r *http.Request, userID, username, email, role string) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), middleware.SessionUserContextKey, middleware.SessionUser{
		UserID:   userID,
		Username: username,
		Email:    email,
		Role:     role,
	}))
}

func TestPatchMe_UpdateUsername(t *testing.T) {
	h, q := newTestHandler(t)
	user := seedUser(t, q, "helen", "helen@test.com")

	body := `{"username":"helen2"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/users/me", bytes.NewBufferString(body))
	req = requestWithUser(req, user.ID.String(), user.Username, user.Email, user.Role)
	rec := httptest.NewRecorder()
	h.PatchMe(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d — body: %s", rec.Code, rec.Body.String())
	}
}

func TestPatchMe_UsernameConflict(t *testing.T) {
	h, q := newTestHandler(t)
	seedUser(t, q, "ivan", "ivan@test.com")
	user2 := seedUser(t, q, "judy", "judy@test.com")

	body := `{"username":"ivan"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/users/me", bytes.NewBufferString(body))
	req = requestWithUser(req, user2.ID.String(), user2.Username, user2.Email, user2.Role)
	rec := httptest.NewRecorder()
	h.PatchMe(rec, req)
	if rec.Code != http.StatusConflict {
		t.Errorf("want 409, got %d", rec.Code)
	}
	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["code"] != "username_taken" {
		t.Errorf("want username_taken, got %s", resp["code"])
	}
}

func TestGetHistory_Empty(t *testing.T) {
	h, q := newTestHandler(t)
	user := seedUser(t, q, "kate", "kate@test.com")

	req := httptest.NewRequest(http.MethodGet, "/api/users/me/history", nil)
	req = requestWithUser(req, user.ID.String(), user.Username, user.Email, user.Role)
	rec := httptest.NewRecorder()
	h.GetHistory(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	rooms, ok := resp["rooms"]
	if !ok {
		t.Error("response missing rooms key")
	}
	if rooms == nil {
		t.Error("rooms should be an empty array, not null")
	}
}

// seedFinishedRoomForUser inserts a finished room the user is a member of.
// When withGameplay is true, one round is created, started, and one submission
// is inserted — enough to pass the "something actually happened" filter on
// GetUserGameHistory. When false, the room exists but carries no gameplay, so
// history should skip it.
func seedFinishedRoomForUser(t *testing.T, q *db.Queries, user db.User, withGameplay bool) db.Room {
	t.Helper()
	ctx := context.Background()

	pack, err := q.CreatePack(ctx, db.CreatePackParams{
		Name:       testutil.SeedName(t) + "_" + user.Username + "_pk",
		Visibility: "private",
		Language:   "en",
	})
	if err != nil {
		t.Fatalf("seedFinishedRoomForUser: create pack: %v", err)
	}
	item, err := q.CreateItem(ctx, db.CreateItemParams{
		PackID: pack.ID, Name: "prompt", PayloadVersion: 1,
	})
	if err != nil {
		t.Fatalf("seedFinishedRoomForUser: create item: %v", err)
	}
	gt, err := q.GetGameTypeBySlug(ctx, "meme-freestyle")
	if err != nil {
		t.Fatalf("seedFinishedRoomForUser: game type: %v", err)
	}
	code := fmt.Sprintf("H-%s-%s-%d", testutil.SeedName(t), user.Username, time.Now().UnixNano())
	room, err := q.CreateRoom(ctx, db.CreateRoomParams{
		Code:       code,
		GameTypeID: gt.ID,
		PackID:     pack.ID,
		HostID:     pgtype.UUID{Bytes: user.ID, Valid: true},
		Mode:       "multiplayer",
		Config:     json.RawMessage(`{"round_count":3,"round_duration_seconds":30,"voting_duration_seconds":15}`),
	})
	if err != nil {
		t.Fatalf("seedFinishedRoomForUser: create room: %v", err)
	}
	if err := q.UpsertRoomPlayer(ctx, db.UpsertRoomPlayerParams{
		RoomID: room.ID,
		UserID: pgtype.UUID{Bytes: user.ID, Valid: true},
	}); err != nil {
		t.Fatalf("seedFinishedRoomForUser: upsert player: %v", err)
	}
	if withGameplay {
		rnd, err := q.CreateRound(ctx, db.CreateRoundParams{
			RoomID: room.ID, ItemID: item.ID,
		})
		if err != nil {
			t.Fatalf("seedFinishedRoomForUser: create round: %v", err)
		}
		if _, err := q.StartRound(ctx, rnd.ID); err != nil {
			t.Fatalf("seedFinishedRoomForUser: start round: %v", err)
		}
		if _, err := q.CreateSubmission(ctx, db.CreateSubmissionParams{
			RoundID: rnd.ID,
			UserID:  pgtype.UUID{Bytes: user.ID, Valid: true},
			Payload: json.RawMessage(`{"caption":"x"}`),
		}); err != nil {
			t.Fatalf("seedFinishedRoomForUser: create submission: %v", err)
		}
	}
	if _, err := q.SetRoomState(ctx, db.SetRoomStateParams{
		ID: room.ID, State: "finished",
	}); err != nil {
		t.Fatalf("seedFinishedRoomForUser: set finished: %v", err)
	}
	return room
}

func TestGetHistory_ExcludesRoomsWithNoGameplay(t *testing.T) {
	h, q := newTestHandler(t)
	user := seedUser(t, q, "mona", "mona@test.com")

	played := seedFinishedRoomForUser(t, q, user, true)
	abandoned := seedFinishedRoomForUser(t, q, user, false)

	req := httptest.NewRequest(http.MethodGet, "/api/users/me/history", nil)
	req = requestWithUser(req, user.ID.String(), user.Username, user.Email, user.Role)
	rec := httptest.NewRecorder()
	h.GetHistory(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d — body: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Rooms []struct {
			Code string `json:"code"`
		} `json:"rooms"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	var sawPlayed, sawAbandoned bool
	for _, r := range resp.Rooms {
		if r.Code == played.Code {
			sawPlayed = true
		}
		if r.Code == abandoned.Code {
			sawAbandoned = true
		}
	}
	if !sawPlayed {
		t.Errorf("played room %q missing from history", played.Code)
	}
	if sawAbandoned {
		t.Errorf("abandoned room %q leaked into history", abandoned.Code)
	}
}

func TestGetExport_ContainsUser(t *testing.T) {
	h, q := newTestHandler(t)
	user := seedUser(t, q, "laura", "laura@test.com")

	req := httptest.NewRequest(http.MethodGet, "/api/users/me/export", nil)
	req = requestWithUser(req, user.ID.String(), user.Username, user.Email, user.Role)
	rec := httptest.NewRecorder()
	h.GetExport(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	userField, ok := resp["user"].(map[string]any)
	if !ok {
		t.Fatalf("expected user object in export, got %T", resp["user"])
	}
	if userField["email"] != user.Email {
		t.Errorf("export email mismatch: got %v", userField["email"])
	}
}
