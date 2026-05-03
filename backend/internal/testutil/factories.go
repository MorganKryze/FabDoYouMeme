package testutil

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/auth"
)

// Factory helpers that build realistic domain rows directly via sqlc against
// the shared pool. Each factory uses SeedName(t) as the uniqueness base so
// parallel tests don't collide on UNIQUE constraints (usernames, emails,
// invite tokens, room codes).
//
// These bypass HTTP handlers on purpose — they're the fastest way to stand up
// preconditions for tests that target something else. Tests that exercise the
// handler path (e.g. TestRegister_Success) should still go through the
// handler.

// ─── Users ───────────────────────────────────────────────────────────────────

// MakeUser inserts a user row with role (e.g. "player", "admin") and a
// collision-safe username/email derived from t.Name(). Consent is set to now.
func MakeUser(t *testing.T, role string) db.User {
	t.Helper()
	q := db.New(Pool())
	slug := SeedName(t) + "_" + randSuffix()
	user, err := q.CreateUser(context.Background(), db.CreateUserParams{
		Username:  "u_" + slug,
		Email:     slug + "@test.local",
		Role:      role,
		IsActive:  true,
		InvitedBy: pgtype.UUID{Valid: false},
		ConsentAt: time.Now().UTC(),
		Locale:    "en",
	})
	if err != nil {
		t.Fatalf("MakeUser: %v", err)
	}
	return user
}

// ─── Sessions ────────────────────────────────────────────────────────────────

// MakeSession creates a real sessions row for user and returns the session row
// plus an http.Cookie carrying the raw (unhashed) token. Attach the cookie to
// test requests via HTTPTest.WithCookie or WSTestClient's Dial.
//
// The expiry is set 24h into the future — override by calling
// MakeSessionWithTTL for custom windows (e.g. renewal cadence tests).
func MakeSession(t *testing.T, user db.User) (db.Session, *http.Cookie) {
	t.Helper()
	return MakeSessionWithTTL(t, user, 24*time.Hour)
}

// MakeSessionWithTTL is MakeSession with a caller-supplied TTL.
func MakeSessionWithTTL(t *testing.T, user db.User, ttl time.Duration) (db.Session, *http.Cookie) {
	t.Helper()
	raw, err := auth.GenerateRawToken()
	if err != nil {
		t.Fatalf("MakeSession: generate token: %v", err)
	}
	q := db.New(Pool())
	sess, err := q.CreateSession(context.Background(), db.CreateSessionParams{
		UserID:    user.ID,
		TokenHash: auth.HashToken(raw),
		ExpiresAt: time.Now().UTC().Add(ttl),
	})
	if err != nil {
		t.Fatalf("MakeSession: insert: %v", err)
	}
	cookie := &http.Cookie{
		Name:  "session",
		Value: raw,
		Path:  "/",
	}
	return sess, cookie
}

// ─── Invites ─────────────────────────────────────────────────────────────────

// MakeInvite creates a usable invite row with max_uses=10. Pass a zero-value
// db.User for createdBy to leave the creator NULL (useful for bootstrap tests).
func MakeInvite(t *testing.T, createdBy db.User) db.Invite {
	t.Helper()
	q := db.New(Pool())
	params := db.CreateInviteParams{
		Token:   "INV_" + SeedName(t) + "_" + randSuffix(),
		MaxUses: 10,
		Locale:  "en",
	}
	if createdBy.ID != uuid.Nil {
		params.CreatedBy = pgtype.UUID{Bytes: createdBy.ID, Valid: true}
	}
	inv, err := q.CreateInvite(context.Background(), params)
	if err != nil {
		t.Fatalf("MakeInvite: %v", err)
	}
	return inv
}

// ─── Packs & items ───────────────────────────────────────────────────────────

// MakePack creates a public, active game pack owned by owner. If withItems > 0,
// N items are inserted with throwaway payloads so meme-freestyle rounds can pick
// a prompt.
func MakePack(t *testing.T, owner db.User, withItems int) db.GamePack {
	t.Helper()
	q := db.New(Pool())
	pack, err := q.CreatePack(context.Background(), db.CreatePackParams{
		Name:       "pack_" + SeedName(t) + "_" + randSuffix(),
		OwnerID:    pgtype.UUID{Bytes: owner.ID, Valid: true},
		IsOfficial: false,
		Visibility: "public",
		Language:   "en",
	})
	if err != nil {
		t.Fatalf("MakePack: create: %v", err)
	}
	for i := 0; i < withItems; i++ {
		item, err := q.CreateItem(context.Background(), db.CreateItemParams{
			PackID:         pack.ID,
			Name:           fmt.Sprintf("item %d", i),
			PayloadVersion: 1,
		})
		if err != nil {
			t.Fatalf("MakePack: create item %d: %v", i, err)
		}
		payload := json.RawMessage(fmt.Sprintf(`{"caption":"item %d"}`, i))
		key := fmt.Sprintf("test/%s/%d.png", pack.ID, i)
		if _, err := q.CreateItemVersion(context.Background(), db.CreateItemVersionParams{
			ItemID:   item.ID,
			MediaKey: &key,
			Payload:  payload,
		}); err != nil {
			t.Fatalf("MakePack: create item version %d: %v", i, err)
		}
	}
	return pack
}

// ─── Rooms ───────────────────────────────────────────────────────────────────

// MakeRoom creates a lobby-state room for gameTypeSlug (default "meme-freestyle"
// if empty) hosted by host and backed by pack. The room config is a minimal
// valid JSON object — callers that need specific round counts should update
// the row themselves.
func MakeRoom(t *testing.T, host db.User, pack db.GamePack, gameTypeSlug string) db.Room {
	t.Helper()
	if gameTypeSlug == "" {
		gameTypeSlug = "meme-freestyle"
	}
	q := db.New(Pool())
	gt, err := q.GetGameTypeBySlug(context.Background(), gameTypeSlug)
	if err != nil {
		t.Fatalf("MakeRoom: lookup game type %q: %v", gameTypeSlug, err)
	}
	// Room codes are 6 chars upper-case alpha — keep it simple and unique.
	code := strings.ToUpper(randSuffix())[:6]
	room, err := q.CreateRoom(context.Background(), db.CreateRoomParams{
		Code:       code,
		GameTypeID: gt.ID,
		HostID:     pgtype.UUID{Bytes: host.ID, Valid: true},
		Mode:       "multiplayer",
		Config:     json.RawMessage(`{"round_count":3,"round_duration_seconds":30,"voting_duration_seconds":15}`),
	})
	if err != nil {
		t.Fatalf("MakeRoom: %v", err)
	}
	// Persist a single-pack room mix so the hub finds the pack via room_packs
	// when it boots. Tests that need multi-pack mixes call InsertRoomPack
	// directly on top of this baseline.
	if err := q.InsertRoomPack(context.Background(), db.InsertRoomPackParams{
		RoomID: room.ID,
		Role:   "image",
		PackID: pack.ID,
		Weight: 1,
	}); err != nil {
		t.Fatalf("MakeRoom: insert room_pack: %v", err)
	}
	return room
}

// ─── internals ───────────────────────────────────────────────────────────────

// randSuffix returns a short hex string to disambiguate factory rows inside a
// single test that needs multiple users/packs/etc.
func randSuffix() string {
	return strings.ReplaceAll(uuid.NewString(), "-", "")[:8]
}
