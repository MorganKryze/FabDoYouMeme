// backend/internal/reset/reset_test.go
package reset_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/reset"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/testutil"
)

func TestMain(m *testing.M) {
	os.Exit(testutil.SetupSuite(m))
}

// historySeed captures the ids created by seedGameHistory so assertions
// can check which rows survived a wipe and which got deleted.
type historySeed struct {
	user db.User
	pack db.GamePack
	item db.GameItem
	room db.Room
	sub  db.Submission
	vote db.Vote
}

// seedGameHistory creates one full room graph — user, pack, item, room,
// round, submission, vote — and returns the ids used. Cleanup is NOT
// registered because these tests run inside a rolled-back transaction.
func seedGameHistory(t *testing.T, ctx context.Context, q *db.Queries, slug string) historySeed {
	t.Helper()
	gt, err := q.GetGameTypeBySlug(ctx, "meme-freestyle")
	if err != nil {
		t.Fatalf("GetGameTypeBySlug: %v", err)
	}
	user, err := q.CreateUser(ctx, db.CreateUserParams{
		Username:  slug,
		Email:     slug + "@test.com",
		Role:      "player",
		IsActive:  true,
		ConsentAt: time.Now().UTC(),
		Locale:    "en",
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	pack, err := q.CreatePack(ctx, db.CreatePackParams{
		Name:       slug + "_pack",
		Visibility: "private",
		Language:   "en",
	})
	if err != nil {
		t.Fatalf("CreatePack: %v", err)
	}
	item, err := q.CreateItem(ctx, db.CreateItemParams{
		PackID:         pack.ID,
		Name:           "item_" + slug,
		PayloadVersion: 1,
	})
	if err != nil {
		t.Fatalf("CreateItem: %v", err)
	}
	// Room codes must be unique in the schema — derive from a fresh UUID
	// instead of the slug so multiple seed calls don't collide.
	code := uuid.NewString()[:8]
	room, err := q.CreateRoom(ctx, db.CreateRoomParams{
		Code:       code,
		GameTypeID: gt.ID,
		HostID:     pgtype.UUID{Bytes: user.ID, Valid: true},
		Mode:       "multiplayer",
		Config:     json.RawMessage(`{"round_count":3,"round_duration_seconds":60,"voting_duration_seconds":30}`),
	})
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	if err := q.InsertRoomPack(ctx, db.InsertRoomPackParams{
		RoomID: room.ID, Role: "image", PackID: pack.ID, Weight: 1,
	}); err != nil {
		t.Fatalf("InsertRoomPack: %v", err)
	}
	round, err := q.CreateRound(ctx, db.CreateRoundParams{
		RoomID: room.ID,
		ItemID: item.ID,
	})
	if err != nil {
		t.Fatalf("CreateRound: %v", err)
	}
	userPG := pgtype.UUID{Bytes: user.ID, Valid: true}
	sub, err := q.CreateSubmission(ctx, db.CreateSubmissionParams{
		RoundID: round.ID,
		UserID:  userPG,
		Payload: json.RawMessage(`{"caption":"funny"}`),
	})
	if err != nil {
		t.Fatalf("CreateSubmission: %v", err)
	}
	vote, err := q.CreateVote(ctx, db.CreateVoteParams{
		SubmissionID: sub.ID,
		VoterID:      userPG,
		Value:        json.RawMessage(`{"points":1}`),
	})
	if err != nil {
		t.Fatalf("CreateVote: %v", err)
	}
	return historySeed{user: user, pack: pack, item: item, room: room, sub: sub, vote: vote}
}

func TestWipeGameHistory_DeletesHistoryOnly(t *testing.T) {
	ctx := context.Background()
	pool := testutil.Pool()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	q := db.New(tx)
	seed := seedGameHistory(t, ctx, q, testutil.SeedName(t))

	report, err := reset.WipeGameHistory(ctx, tx)
	if err != nil {
		t.Fatalf("WipeGameHistory: %v", err)
	}
	if report.RoomsDeleted < 1 {
		t.Errorf("want >=1 rooms deleted, got %d", report.RoomsDeleted)
	}
	if report.SubmissionsDeleted < 1 {
		t.Errorf("want >=1 submissions deleted, got %d", report.SubmissionsDeleted)
	}
	if report.VotesDeleted < 1 {
		t.Errorf("want >=1 votes deleted, got %d", report.VotesDeleted)
	}

	// Pack and user should survive.
	if _, err := q.GetPackByID(ctx, seed.pack.ID); err != nil {
		t.Errorf("pack should survive WipeGameHistory: %v", err)
	}
	if _, err := q.GetUserByID(ctx, seed.user.ID); err != nil {
		t.Errorf("user should survive WipeGameHistory: %v", err)
	}
	// Room should be gone.
	if _, err := q.GetRoomByCode(ctx, seed.room.Code); err == nil {
		t.Error("room should be deleted by WipeGameHistory")
	}
}

func TestWipeInvites_DeletesOnlyInvites(t *testing.T) {
	ctx := context.Background()
	pool := testutil.Pool()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	q := db.New(tx)
	slug := testutil.SeedName(t)
	user, err := q.CreateUser(ctx, db.CreateUserParams{
		Username:  slug + "_u",
		Email:     slug + "_u@test.com",
		Role:      "player",
		IsActive:  true,
		ConsentAt: time.Now().UTC(),
		Locale:    "en",
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	for i := 0; i < 2; i++ {
		_, err := q.CreateInvite(ctx, db.CreateInviteParams{
			Token:   "INV_" + uuid.New().String(),
			MaxUses: 1,
			Locale:  "en",
		})
		if err != nil {
			t.Fatalf("CreateInvite %d: %v", i, err)
		}
	}

	report, err := reset.WipeInvites(ctx, tx)
	if err != nil {
		t.Fatalf("WipeInvites: %v", err)
	}
	if report.InvitesDeleted < 2 {
		t.Errorf("want >=2 invites deleted, got %d", report.InvitesDeleted)
	}

	// User must survive.
	if _, err := q.GetUserByID(ctx, user.ID); err != nil {
		t.Errorf("user should survive WipeInvites: %v", err)
	}
}

func TestWipeSessions_ExcludesActingAdmin(t *testing.T) {
	ctx := context.Background()
	pool := testutil.Pool()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	q := db.New(tx)
	slug := testutil.SeedName(t)

	mk := func(suffix string) db.User {
		u, err := q.CreateUser(ctx, db.CreateUserParams{
			Username:  slug + "_" + suffix,
			Email:     slug + "_" + suffix + "@test.com",
			Role:      "player",
			IsActive:  true,
			ConsentAt: time.Now().UTC(),
			Locale:    "en",
		})
		if err != nil {
			t.Fatalf("CreateUser %s: %v", suffix, err)
		}
		_, err = q.CreateSession(ctx, db.CreateSessionParams{
			UserID:    u.ID,
			TokenHash: "hash_" + u.ID.String(),
			ExpiresAt: time.Now().UTC().Add(time.Hour),
		})
		if err != nil {
			t.Fatalf("CreateSession %s: %v", suffix, err)
		}
		return u
	}
	acting := mk("acting")
	victimA := mk("vicA")
	victimB := mk("vicB")

	// Also create one magic link token — it should be wiped regardless.
	_, err = q.CreateMagicLinkToken(ctx, db.CreateMagicLinkTokenParams{
		UserID:    victimA.ID,
		TokenHash: "mlhash_" + victimA.ID.String(),
		Purpose:   "login",
		ExpiresAt: time.Now().UTC().Add(15 * time.Minute),
	})
	if err != nil {
		t.Fatalf("CreateMagicLinkToken: %v", err)
	}

	report, err := reset.WipeSessions(ctx, tx, acting.ID)
	if err != nil {
		t.Fatalf("WipeSessions: %v", err)
	}
	if report.SessionsDeleted < 2 {
		t.Errorf("want >=2 sessions deleted, got %d", report.SessionsDeleted)
	}
	if report.MagicTokensDeleted < 1 {
		t.Errorf("want >=1 magic tokens deleted, got %d", report.MagicTokensDeleted)
	}
	if !report.ExcludedSelf {
		t.Error("want ExcludedSelf=true when a non-nil UUID is passed")
	}

	// Acting admin's session must survive.
	if _, err := q.GetSessionByTokenHash(ctx, "hash_"+acting.ID.String()); err != nil {
		t.Errorf("acting session should survive: %v", err)
	}
	// Victim sessions must be gone.
	if _, err := q.GetSessionByTokenHash(ctx, "hash_"+victimA.ID.String()); err == nil {
		t.Error("victim A session should be wiped")
	}
	if _, err := q.GetSessionByTokenHash(ctx, "hash_"+victimB.ID.String()); err == nil {
		t.Error("victim B session should be wiped")
	}
}

func TestWipePacksAndMedia_DeletesPacksAndCallsPurge(t *testing.T) {
	ctx := context.Background()
	pool := testutil.Pool()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	q := db.New(tx)
	slug := testutil.SeedName(t)

	// Seed a full history graph so we exercise the internal history
	// wipe path (FK prerequisite for pack deletion).
	seed := seedGameHistory(t, ctx, q, slug)

	st := testutil.NewFakeStorage()
	// Pretend the bucket had 7 keys so Purge returns a meaningful count.
	st.PurgeCount = 7

	report, err := reset.WipePacksAndMedia(ctx, tx, st)
	if err != nil {
		t.Fatalf("WipePacksAndMedia: %v", err)
	}
	if report.PacksDeleted < 1 {
		t.Errorf("want >=1 packs deleted, got %d", report.PacksDeleted)
	}
	if report.RoomsDeleted < 1 {
		t.Errorf("history was not wiped first — got %d rooms deleted", report.RoomsDeleted)
	}
	if report.S3ObjectsDeleted != 7 {
		t.Errorf("want s3_objects_deleted=7, got %d", report.S3ObjectsDeleted)
	}
	if len(st.Purges) != 1 {
		t.Fatalf("want 1 Purge call, got %d", len(st.Purges))
	}
	if st.Purges[0].Prefix != "" {
		t.Errorf("want empty prefix (full bucket wipe), got %q", st.Purges[0].Prefix)
	}

	// Pack should be gone; user should survive.
	if _, err := q.GetPackByID(ctx, seed.pack.ID); err == nil {
		t.Error("pack should be deleted")
	}
	if _, err := q.GetUserByID(ctx, seed.user.ID); err != nil {
		t.Errorf("user should survive WipePacksAndMedia: %v", err)
	}
}

func TestFullReset_PreservesSentinelAndBootstrapAndActing(t *testing.T) {
	ctx := context.Background()
	pool := testutil.Pool()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	q := db.New(tx)
	slug := testutil.SeedName(t)

	// Seed: a bootstrap-style protected admin, an acting admin, a regular
	// user with full history graph, an invite, and a session for the
	// acting admin.
	bootstrap, err := q.CreateUser(ctx, db.CreateUserParams{
		Username:  slug + "_boot",
		Email:     slug + "_boot@test.com",
		Role:      "admin",
		IsActive:  true,
		ConsentAt: time.Now().UTC(),
		Locale:    "en",
	})
	if err != nil {
		t.Fatalf("CreateUser bootstrap: %v", err)
	}
	if _, err := tx.Exec(ctx, "UPDATE users SET is_protected = true WHERE id = $1", bootstrap.ID); err != nil {
		t.Fatalf("mark bootstrap protected: %v", err)
	}

	acting, err := q.CreateUser(ctx, db.CreateUserParams{
		Username:  slug + "_act",
		Email:     slug + "_act@test.com",
		Role:      "admin",
		IsActive:  true,
		ConsentAt: time.Now().UTC(),
		Locale:    "en",
	})
	if err != nil {
		t.Fatalf("CreateUser acting: %v", err)
	}
	_, err = q.CreateSession(ctx, db.CreateSessionParams{
		UserID:    acting.ID,
		TokenHash: "acting_hash_" + uuid.New().String(),
		ExpiresAt: time.Now().UTC().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("acting session: %v", err)
	}

	_ = seedGameHistory(t, ctx, q, slug+"_seed")

	_, err = q.CreateInvite(ctx, db.CreateInviteParams{
		Token:   "FR_" + uuid.New().String(),
		MaxUses: 1,
		Locale:  "en",
	})
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}

	st := testutil.NewFakeStorage()
	st.PurgeCount = 3

	report, err := reset.FullReset(ctx, tx, st, acting.ID)
	if err != nil {
		t.Fatalf("FullReset: %v", err)
	}

	// Invariant 1: sentinel survives.
	sentinelID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	if _, err := q.GetUserByID(ctx, sentinelID); err != nil {
		t.Errorf("sentinel user should survive FullReset: %v", err)
	}
	// Invariant 2: bootstrap (is_protected=true) survives.
	if _, err := q.GetUserByID(ctx, bootstrap.ID); err != nil {
		t.Errorf("bootstrap admin should survive FullReset: %v", err)
	}
	// Invariant 3: acting admin survives.
	if _, err := q.GetUserByID(ctx, acting.ID); err != nil {
		t.Errorf("acting admin should survive FullReset: %v", err)
	}
	// Invariant 4: game_types seed data is untouched.
	if _, err := q.GetGameTypeBySlug(ctx, "meme-freestyle"); err != nil {
		t.Errorf("game_types should survive FullReset: %v", err)
	}
	// Invariant 5: acting admin's session survives.
	var actingSessionCount int
	if err := tx.QueryRow(ctx, "SELECT COUNT(*) FROM sessions WHERE user_id = $1", acting.ID).Scan(&actingSessionCount); err != nil {
		t.Fatalf("count acting sessions: %v", err)
	}
	if actingSessionCount < 1 {
		t.Errorf("acting admin's session should survive, got count=%d", actingSessionCount)
	}

	// Destructive expectations:
	if report.InvitesDeleted < 1 {
		t.Errorf("want invites wiped, got %d", report.InvitesDeleted)
	}
	if report.PacksDeleted < 1 {
		t.Errorf("want packs wiped, got %d", report.PacksDeleted)
	}
	if report.UsersDeleted < 1 {
		t.Errorf("want >=1 non-protected user deleted, got %d", report.UsersDeleted)
	}
	if report.S3ObjectsDeleted != 3 {
		t.Errorf("want s3_objects_deleted=3 (from PurgeCount), got %d", report.S3ObjectsDeleted)
	}
	if !report.ExcludedSelf {
		t.Error("want ExcludedSelf=true")
	}
}
