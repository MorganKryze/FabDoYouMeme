package db_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/testutil"
)

// queries returns a db.Queries bound to the shared pool.
// Suitable for read-only smoke tests; for write tests use testutil.WithTx or
// set up explicit cleanup via t.Cleanup.
func queries() *db.Queries {
	return db.New(testutil.Pool())
}

// createUser is a test helper that creates a user with the given name and
// cleans it up after the test.
func createUser(t *testing.T, q *db.Queries, username, email string) db.User {
	t.Helper()
	u, err := q.CreateUser(context.Background(), db.CreateUserParams{
		Username:  username,
		Email:     email,
		Role:      "player",
		IsActive:  true,
		ConsentAt: time.Now().UTC(),
		Locale:    "en",
	})
	if err != nil {
		t.Fatalf("createUser %s: %v", username, err)
	}
	return u
}

// ─── Smoke tests (migration-seeded data) ─────────────────────────────────────

func TestSentinelRowExists(t *testing.T) {
	q := queries()
	u, err := q.GetSentinelUser(context.Background())
	if err != nil {
		t.Fatalf("sentinel not found: %v", err)
	}
	if u.Username != "[deleted]" {
		t.Errorf("expected [deleted], got %s", u.Username)
	}
	if u.IsActive {
		t.Error("sentinel must be inactive")
	}
}

func TestMemeCaptionGameTypeSeeded(t *testing.T) {
	q := queries()
	gt, err := q.GetGameTypeBySlug(context.Background(), "meme-freestyle")
	if err != nil {
		t.Fatalf("game type not found: %v", err)
	}
	if gt.Slug != "meme-freestyle" {
		t.Errorf("unexpected slug: %s", gt.Slug)
	}
}

// ─── users.sql ───────────────────────────────────────────────────────────────

func TestCreateUser_Success(t *testing.T) {
	testutil.WithTx(t, func(q *db.Queries) {
		u, err := q.CreateUser(context.Background(), db.CreateUserParams{
			Username:  "txuser1",
			Email:     "txuser1@test.com",
			Role:      "player",
			IsActive:  true,
			ConsentAt: time.Now().UTC(),
			Locale:    "en",
		})
		if err != nil {
			t.Fatalf("CreateUser: %v", err)
		}
		if u.Username != "txuser1" {
			t.Errorf("got username %s", u.Username)
		}
		if u.Role != "player" {
			t.Errorf("got role %s", u.Role)
		}
	})
}

func TestGetUserByEmail_NotFound(t *testing.T) {
	testutil.WithTx(t, func(q *db.Queries) {
		_, err := q.GetUserByEmail(context.Background(), "nobody@nowhere.test")
		if err == nil {
			t.Error("expected error for unknown email")
		}
	})
}

func TestGetUserByID_NotFound(t *testing.T) {
	testutil.WithTx(t, func(q *db.Queries) {
		_, err := q.GetUserByID(context.Background(), uuid.New())
		if err == nil {
			t.Error("expected error for unknown ID")
		}
	})
}

func TestUpdateUsername_Conflict(t *testing.T) {
	testutil.WithTx(t, func(q *db.Queries) {
		ctx := context.Background()
		createUser(t, q, "taken_name", "taken@test.com")
		u2 := createUser(t, q, "other_name", "other@test.com")

		_, err := q.UpdateUserUsername(ctx, db.UpdateUserUsernameParams{
			ID:       u2.ID,
			Username: "taken_name",
		})
		if err == nil {
			t.Error("expected unique constraint violation")
		}
	})
}

func TestHardDeleteUser_ReplacesSubmissionsWithSentinel(t *testing.T) {
	ctx := context.Background()
	pool := testutil.Pool()
	q := db.New(pool)

	// Seed: user, game type (already seeded), pack, item, room, round, submission.
	gt, err := q.GetGameTypeBySlug(ctx, "meme-freestyle")
	if err != nil {
		t.Fatalf("get game type: %v", err)
	}
	slug := testutil.SeedName(t)
	user, err := q.CreateUser(ctx, db.CreateUserParams{
		Username:  slug,
		Email:     slug + "@test.com",
		Role:      "player",
		IsActive:  true,
		ConsentAt: time.Now().UTC(),
		Locale:    "en",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	pack, err := q.CreatePack(ctx, db.CreatePackParams{
		Name:       slug + "_pack",
		Visibility: "private",
		Language:   "en",
	})
	if err != nil {
		t.Fatalf("create pack: %v", err)
	}
	// No items needed for room; room just references pack_id.
	room, err := q.CreateRoom(ctx, db.CreateRoomParams{
		Code:       slug[:4],
		GameTypeID: gt.ID,
		PackID:     pack.ID,
		HostID:     pgtype.UUID{Bytes: user.ID, Valid: true},
		Mode:       "multiplayer",
		Config:     json.RawMessage(`{"round_count":3,"round_duration_seconds":60,"voting_duration_seconds":30}`),
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	// Need at least one item to create a round (item_id FK).
	item, err := q.CreateItem(ctx, db.CreateItemParams{
		PackID:         pack.ID,
		Name:           "test item",
		PayloadVersion: 1,
	})
	if err != nil {
		t.Fatalf("create item: %v", err)
	}
	round, err := q.CreateRound(ctx, db.CreateRoundParams{
		RoomID: room.ID,
		ItemID: item.ID,
	})
	if err != nil {
		t.Fatalf("create round: %v", err)
	}
	userPG := pgtype.UUID{Bytes: user.ID, Valid: true}
	sub, err := q.CreateSubmission(ctx, db.CreateSubmissionParams{
		RoundID: round.ID,
		UserID:  userPG,
		Payload: json.RawMessage(`{"caption":"funny"}`),
	})
	if err != nil {
		t.Fatalf("create submission: %v", err)
	}

	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM votes WHERE submission_id = $1", sub.ID)
		pool.Exec(ctx, "DELETE FROM submissions WHERE id = $1", sub.ID)
		pool.Exec(ctx, "DELETE FROM rounds WHERE id = $1", round.ID)
		pool.Exec(ctx, "DELETE FROM rooms WHERE id = $1", room.ID)
		pool.Exec(ctx, "DELETE FROM game_items WHERE id = $1", item.ID)
		pool.Exec(ctx, "DELETE FROM game_packs WHERE id = $1", pack.ID)
		pool.Exec(ctx, "DELETE FROM users WHERE id = $1", user.ID)
	})

	// Replace user refs in submissions/votes with the sentinel UUID.
	if err := q.UpdateSubmissionsSentinel(ctx, userPG); err != nil {
		t.Fatalf("UpdateSubmissionsSentinel: %v", err)
	}
	if err := q.UpdateVotesSentinel(ctx, userPG); err != nil {
		t.Fatalf("UpdateVotesSentinel: %v", err)
	}

	// Verify the sentinel replacement before we delete the rows.
	subs, err := q.GetSubmissionsForRound(ctx, round.ID)
	if err != nil {
		t.Fatalf("GetSubmissionsForRound: %v", err)
	}
	sentinelUUID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	sentinelPG := pgtype.UUID{Bytes: sentinelUUID, Valid: true}
	if len(subs) != 1 || subs[0].UserID != sentinelPG {
		t.Errorf("expected submission user_id == sentinel, got %v", subs[0].UserID)
	}

	// Remove the room graph rows so the host FK is released before deleting the user.
	pool.Exec(ctx, "DELETE FROM votes WHERE submission_id = $1", sub.ID)
	pool.Exec(ctx, "DELETE FROM submissions WHERE id = $1", sub.ID)
	pool.Exec(ctx, "DELETE FROM rounds WHERE id = $1", round.ID)
	pool.Exec(ctx, "DELETE FROM rooms WHERE id = $1", room.ID)
	if err := q.HardDeleteUser(ctx, user.ID); err != nil {
		t.Fatalf("HardDeleteUser: %v", err)
	}

	// User row must be gone.
	if _, err := q.GetUserByID(ctx, user.ID); err == nil {
		t.Error("expected user to be deleted")
	}
}

// ─── sessions.sql ─────────────────────────────────────────────────────────────

func TestCreateSession_AndLookupByHash(t *testing.T) {
	testutil.WithTx(t, func(q *db.Queries) {
		ctx := context.Background()
		u := createUser(t, q, "sess_user", "sess@test.com")
		hash := "fakehash_" + uuid.New().String()

		_, err := q.CreateSession(ctx, db.CreateSessionParams{
			UserID:    u.ID,
			TokenHash: hash,
			ExpiresAt: time.Now().UTC().Add(time.Hour),
		})
		if err != nil {
			t.Fatalf("CreateSession: %v", err)
		}

		row, err := q.GetSessionByTokenHash(ctx, hash)
		if err != nil {
			t.Fatalf("GetSessionByTokenHash: %v", err)
		}
		if row.UserID != u.ID {
			t.Errorf("session user_id mismatch")
		}
		if row.Username != u.Username {
			t.Errorf("session username mismatch")
		}
	})
}

func TestDeleteSession_ByHash(t *testing.T) {
	testutil.WithTx(t, func(q *db.Queries) {
		ctx := context.Background()
		u := createUser(t, q, "delsess_user", "delsess@test.com")
		hash := "delhash_" + uuid.New().String()

		q.CreateSession(ctx, db.CreateSessionParams{
			UserID:    u.ID,
			TokenHash: hash,
			ExpiresAt: time.Now().UTC().Add(time.Hour),
		})

		if err := q.DeleteSession(ctx, hash); err != nil {
			t.Fatalf("DeleteSession: %v", err)
		}

		_, err := q.GetSessionByTokenHash(ctx, hash)
		if err == nil {
			t.Error("expected session to be gone after delete")
		}
	})
}

func TestExpiredSession_NotReturned(t *testing.T) {
	testutil.WithTx(t, func(q *db.Queries) {
		ctx := context.Background()
		u := createUser(t, q, "expsess_user", "expsess@test.com")
		hash := "exphash_" + uuid.New().String()

		q.CreateSession(ctx, db.CreateSessionParams{
			UserID:    u.ID,
			TokenHash: hash,
			ExpiresAt: time.Now().UTC().Add(-time.Minute), // already expired
		})

		_, err := q.GetSessionByTokenHash(ctx, hash)
		if err == nil {
			t.Error("expected expired session to not be returned")
		}
	})
}

// ─── magic_link_tokens.sql ───────────────────────────────────────────────────

func TestMagicToken_ConsumeOnce(t *testing.T) {
	testutil.WithTx(t, func(q *db.Queries) {
		ctx := context.Background()
		u := createUser(t, q, "tok_user", "tok@test.com")
		hash := "tokenhash_" + uuid.New().String()

		_, err := q.CreateMagicLinkToken(ctx, db.CreateMagicLinkTokenParams{
			UserID:    u.ID,
			TokenHash: hash,
			Purpose:   "login",
			ExpiresAt: time.Now().UTC().Add(15 * time.Minute),
		})
		if err != nil {
			t.Fatalf("CreateMagicLinkToken: %v", err)
		}

		// First consume: succeeds.
		tok, err := q.ConsumeMagicLinkTokenAtomic(ctx, hash)
		if err != nil {
			t.Fatalf("first consume: %v", err)
		}
		if !tok.UsedAt.Valid {
			t.Error("expected used_at to be set after consume")
		}

		// Second consume: fails (already used).
		_, err = q.ConsumeMagicLinkTokenAtomic(ctx, hash)
		if err == nil {
			t.Error("expected error on second consume of same token")
		}
	})
}

func TestMagicToken_ExpiredNotReturned(t *testing.T) {
	testutil.WithTx(t, func(q *db.Queries) {
		ctx := context.Background()
		u := createUser(t, q, "exptok_user", "exptok@test.com")
		hash := "exptokenhash_" + uuid.New().String()

		q.CreateMagicLinkToken(ctx, db.CreateMagicLinkTokenParams{
			UserID:    u.ID,
			TokenHash: hash,
			Purpose:   "login",
			ExpiresAt: time.Now().UTC().Add(-time.Minute), // already expired
		})

		_, err := q.GetMagicLinkToken(ctx, hash)
		if err == nil {
			t.Error("expected expired token to not be returned")
		}
	})
}

// ─── invites.sql ─────────────────────────────────────────────────────────────

func TestCreateInvite_AndValidate(t *testing.T) {
	testutil.WithTx(t, func(q *db.Queries) {
		ctx := context.Background()
		inv, err := q.CreateInvite(ctx, db.CreateInviteParams{
			Token:   "INVITE_" + uuid.New().String(),
			MaxUses: 5,
			Locale:    "en",
		})
		if err != nil {
			t.Fatalf("CreateInvite: %v", err)
		}
		if inv.UsesCount != 0 {
			t.Errorf("expected uses_count=0, got %d", inv.UsesCount)
		}

		consumed, err := q.ConsumeInvite(ctx, inv.ID)
		if err != nil {
			t.Fatalf("ConsumeInvite: %v", err)
		}
		if consumed.UsesCount != 1 {
			t.Errorf("expected uses_count=1 after consume, got %d", consumed.UsesCount)
		}
	})
}

func TestInvite_MaxUsesExhausted(t *testing.T) {
	testutil.WithTx(t, func(q *db.Queries) {
		ctx := context.Background()
		inv, err := q.CreateInvite(ctx, db.CreateInviteParams{
			Token:   "EXHAUST_" + uuid.New().String(),
			MaxUses: 1,
			Locale:    "en",
		})
		if err != nil {
			t.Fatalf("CreateInvite: %v", err)
		}

		if _, err := q.ConsumeInvite(ctx, inv.ID); err != nil {
			t.Fatalf("first consume: %v", err)
		}

		// Second consume should fail (max_uses reached).
		_, err = q.ConsumeInvite(ctx, inv.ID)
		if err == nil {
			t.Error("expected error when invite is exhausted")
		}
	})
}

// ─── rooms.sql — startup cleanup queries ─────────────────────────────────────

func TestStartupCleanup_MarksPlayingAsFinished(t *testing.T) {
	testutil.WithTx(t, func(q *db.Queries) {
		ctx := context.Background()
		gt, _ := q.GetGameTypeBySlug(ctx, "meme-freestyle")
		slug := "cleanup_play_" + uuid.New().String()[:8]
		user := createUser(t, q, slug, slug+"@test.com")
		pack, _ := q.CreatePack(ctx, db.CreatePackParams{
			Name:       slug,
			Visibility: "private",
			Language:   "en",
		})
		room, err := q.CreateRoom(ctx, db.CreateRoomParams{
			Code:       slug[:4],
			GameTypeID: gt.ID,
			PackID:     pack.ID,
			HostID:     pgtype.UUID{Bytes: user.ID, Valid: true},
			Mode:       "multiplayer",
			Config:     json.RawMessage(`{"round_count":3,"round_duration_seconds":60,"voting_duration_seconds":30}`),
		})
		if err != nil {
			t.Fatalf("CreateRoom: %v", err)
		}

		// Put room into playing state.
		if _, err := q.SetRoomState(ctx, db.SetRoomStateParams{ID: room.ID, State: "playing"}); err != nil {
			t.Fatalf("SetRoomState: %v", err)
		}

		// Simulate crash-recovery startup cleanup.
		if _, err := q.FinishCrashedRooms(ctx); err != nil {
			t.Fatalf("FinishCrashedRooms: %v", err)
		}

		updated, err := q.GetRoomByCode(ctx, room.Code)
		if err != nil {
			t.Fatalf("GetRoomByCode: %v", err)
		}
		if updated.State != "finished" {
			t.Errorf("expected state=finished after crash cleanup, got %s", updated.State)
		}
		if !updated.FinishedAt.Valid {
			t.Error("expected finished_at to be set")
		}
	})
}

func TestStartupCleanup_ClosesStaleLobbies(t *testing.T) {
	ctx := context.Background()
	pool := testutil.Pool()
	q := db.New(pool)

	gt, _ := q.GetGameTypeBySlug(ctx, "meme-freestyle")
	slug := testutil.SeedName(t)
	user, err := q.CreateUser(ctx, db.CreateUserParams{
		Username:  slug,
		Email:     slug + "@test.com",
		Role:      "player",
		IsActive:  true,
		ConsentAt: time.Now().UTC(),
		Locale:    "en",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	pack, _ := q.CreatePack(ctx, db.CreatePackParams{
		Name:       slug + "_pk",
		Visibility: "private",
		Language:   "en",
	})
	room, err := q.CreateRoom(ctx, db.CreateRoomParams{
		Code:       slug[:4],
		GameTypeID: gt.ID,
		PackID:     pack.ID,
		HostID:     pgtype.UUID{Bytes: user.ID, Valid: true},
		Mode:       "multiplayer",
		Config:     json.RawMessage(`{"round_count":3,"round_duration_seconds":60,"voting_duration_seconds":30}`),
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}

	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM rooms WHERE id = $1", room.ID)
		pool.Exec(ctx, "DELETE FROM game_packs WHERE id = $1", pack.ID)
		pool.Exec(ctx, "DELETE FROM users WHERE id = $1", user.ID)
	})

	// Backdate created_at to simulate a 25-hour-old lobby.
	if _, err := pool.Exec(ctx, "UPDATE rooms SET created_at = now() - interval '25 hours' WHERE id = $1", room.ID); err != nil {
		t.Fatalf("backdate room: %v", err)
	}

	if _, err := q.FinishAbandonedLobbies(ctx); err != nil {
		t.Fatalf("FinishAbandonedLobbies: %v", err)
	}

	updated, err := q.GetRoomByCode(ctx, room.Code)
	if err != nil {
		t.Fatalf("GetRoomByCode: %v", err)
	}
	if updated.State != "finished" {
		t.Errorf("expected state=finished for stale lobby, got %s", updated.State)
	}
}

func TestCreateRoom_StateIsLobby(t *testing.T) {
	testutil.WithTx(t, func(q *db.Queries) {
		ctx := context.Background()
		gt, _ := q.GetGameTypeBySlug(ctx, "meme-freestyle")
		slug := "lobby_" + uuid.New().String()[:8]
		user := createUser(t, q, slug, slug+"@test.com")
		pack, _ := q.CreatePack(ctx, db.CreatePackParams{Name: slug, Visibility: "private",
		Language:   "en",})

		room, err := q.CreateRoom(ctx, db.CreateRoomParams{
			Code:       slug[:4],
			GameTypeID: gt.ID,
			PackID:     pack.ID,
			HostID:     pgtype.UUID{Bytes: user.ID, Valid: true},
			Mode:       "multiplayer",
			Config:     json.RawMessage(`{"round_count":3,"round_duration_seconds":60,"voting_duration_seconds":30}`),
		})
		if err != nil {
			t.Fatalf("CreateRoom: %v", err)
		}
		if room.State != "lobby" {
			t.Errorf("expected initial state=lobby, got %s", room.State)
		}
	})
}
