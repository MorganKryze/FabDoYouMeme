package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/testutil"
)

// seedGroupAndMember creates a fresh group + makes `user` an admin. Returns
// the group. Used by phase-4 room tests that need a group to scope into.
func seedGroupAndMember(t *testing.T, q *db.Queries, user db.User, classification string) db.Group {
	t.Helper()
	g, err := q.CreateGroup(context.Background(), db.CreateGroupParams{
		Name:           "RG_" + uuid.NewString()[:10],
		Description:    "seed",
		Language:       "en",
		Classification: classification,
		QuotaBytes:     500 * 1024 * 1024,
		MemberCap:      100,
		CreatedBy:      pgtype.UUID{Bytes: user.ID, Valid: true},
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	if _, err := q.CreateMembership(context.Background(), db.CreateMembershipParams{
		GroupID: g.ID, UserID: user.ID, Role: "admin",
	}); err != nil {
		t.Fatalf("create membership: %v", err)
	}
	return g
}

// seedGroupOwnedPackWithItems creates a group-owned pack with N items (same
// shape as seedPackWithItems but marked group-owned via direct UPDATE because
// the existing CreatePack is user-owned). Returns the pack.
func seedGroupOwnedPackWithItems(t *testing.T, q *db.Queries, group db.Group, count int) db.GamePack {
	t.Helper()
	ctx := context.Background()
	pack, err := q.CreateGroupPack(ctx, db.CreateGroupPackParams{
		Name:           "gp_" + uuid.NewString()[:8],
		Language:       "en",
		GroupID:        pgtype.UUID{Bytes: group.ID, Valid: true},
		Classification: group.Classification,
	})
	if err != nil {
		t.Fatalf("create group pack: %v", err)
	}
	for i := 0; i < count; i++ {
		item, err := q.CreateItem(ctx, db.CreateItemParams{
			PackID:         pack.ID,
			Name:           "item",
			PayloadVersion: 1,
		})
		if err != nil {
			t.Fatalf("create item: %v", err)
		}
		mediaKey := "group/test/" + uuid.NewString()[:8] + ".png"
		ver, err := q.CreateItemVersion(ctx, db.CreateItemVersionParams{
			ItemID: item.ID, MediaKey: &mediaKey, Payload: json.RawMessage(`{"caption":"x"}`),
		})
		if err != nil {
			t.Fatalf("create version: %v", err)
		}
		if _, err := q.SetCurrentVersion(ctx, db.SetCurrentVersionParams{
			ID: item.ID, CurrentVersionID: pgtype.UUID{Bytes: ver.ID, Valid: true},
		}); err != nil {
			t.Fatalf("set current: %v", err)
		}
	}
	return pack
}

func TestCreateRoom_GroupScopedHappyPath(t *testing.T) {
	h, q := newRoomHandler(t)
	user := seedRoomUser(t, q)
	group := seedGroupAndMember(t, q, user, "sfw")
	pack := seedGroupOwnedPackWithItems(t, q, group, 5)

	gt, _ := q.GetGameTypeBySlug(context.Background(), "meme-freestyle")
	body, _ := json.Marshal(map[string]any{
		"game_type_id": gt.ID.String(),
		"packs": []map[string]any{
			{"role": "image", "pack_id": pack.ID.String(), "weight": 1},
		},
		"group_id":     group.ID.String(),
		"mode":         "multiplayer",
		"config":       json.RawMessage(`{"round_count":3,"round_duration_seconds":60,"voting_duration_seconds":30}`),
	})
	req := httptest.NewRequest(http.MethodPost, "/api/rooms", bytes.NewReader(body))
	req = withUser(req, user.ID.String(), user.Username, user.Email, user.Role)
	rec := httptest.NewRecorder()
	h.Create(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d — body: %s", rec.Code, rec.Body.String())
	}
	var room db.Room
	_ = json.Unmarshal(rec.Body.Bytes(), &room)
	if !room.GroupID.Valid || room.GroupID.Bytes != group.ID {
		t.Fatalf("expected room.group_id=%v, got %+v", group.ID, room.GroupID)
	}
}

func TestCreateRoom_GroupScoped_NonMemberForbidden(t *testing.T) {
	h, q := newRoomHandler(t)
	admin := seedRoomUser(t, q)
	group := seedGroupAndMember(t, q, admin, "sfw")
	pack := seedGroupOwnedPackWithItems(t, q, group, 5)

	// Second user is NOT a member.
	slug := testutil.SeedName(t) + "_nm"
	other, err := q.CreateUser(context.Background(), db.CreateUserParams{
		Username: slug, Email: slug + "@test.com", Role: "player", IsActive: true,
		ConsentAt: admin.CreatedAt, Locale: "en",
	})
	if err != nil {
		t.Fatalf("create other: %v", err)
	}

	gt, _ := q.GetGameTypeBySlug(context.Background(), "meme-freestyle")
	body, _ := json.Marshal(map[string]any{
		"game_type_id": gt.ID.String(),
		"packs": []map[string]any{
			{"role": "image", "pack_id": pack.ID.String(), "weight": 1},
		},
		"group_id":     group.ID.String(),
		"mode":         "multiplayer",
		"config":       json.RawMessage(`{"round_count":3,"round_duration_seconds":60,"voting_duration_seconds":30}`),
	})
	req := httptest.NewRequest(http.MethodPost, "/api/rooms", bytes.NewReader(body))
	req = withUser(req, other.ID.String(), other.Username, other.Email, other.Role)
	rec := httptest.NewRecorder()
	h.Create(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d — body: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateRoom_GroupScoped_PackFromOtherGroupRejected(t *testing.T) {
	h, q := newRoomHandler(t)
	user := seedRoomUser(t, q)
	g1 := seedGroupAndMember(t, q, user, "sfw")
	g2 := seedGroupAndMember(t, q, user, "sfw")
	// Pack belongs to g2 but the room is created against g1.
	pack := seedGroupOwnedPackWithItems(t, q, g2, 5)

	gt, _ := q.GetGameTypeBySlug(context.Background(), "meme-freestyle")
	body, _ := json.Marshal(map[string]any{
		"game_type_id": gt.ID.String(),
		"packs": []map[string]any{
			{"role": "image", "pack_id": pack.ID.String(), "weight": 1},
		},
		"group_id":     g1.ID.String(),
		"mode":         "multiplayer",
		"config":       json.RawMessage(`{"round_count":3,"round_duration_seconds":60,"voting_duration_seconds":30}`),
	})
	req := httptest.NewRequest(http.MethodPost, "/api/rooms", bytes.NewReader(body))
	req = withUser(req, user.ID.String(), user.Username, user.Email, user.Role)
	rec := httptest.NewRecorder()
	h.Create(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d — body: %s", rec.Code, rec.Body.String())
	}
	var env map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	if env["code"] != "pack_not_in_group" {
		t.Fatalf("want pack_not_in_group, got %v", env["code"])
	}
}
