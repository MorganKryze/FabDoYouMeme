package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/api"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/auth"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/config"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/testutil"
)

func newGroupInviteHandler(t *testing.T) (*api.GroupInviteHandler, *db.Queries) {
	t.Helper()
	pool := testutil.Pool()
	cfg := &config.Config{MaxGroupsPerUser: 50, MaxGroupMembershipsPerUser: 50}
	return api.NewGroupInviteHandler(pool, cfg), db.New(pool)
}

// seedGroupInvite inserts a ready-to-use group_join invite with the given
// max_uses + ttl. Returns the row + raw token so callers can redeem.
func seedGroupInvite(t *testing.T, q *db.Queries, gid, creatorID uuid.UUID, kind string, maxUses int32, ttl time.Duration, restrictedEmail *string) (db.GroupInvite, string) {
	t.Helper()
	tok, _ := auth.GenerateRawToken()
	row, err := q.CreateGroupInvite(context.Background(), db.CreateGroupInviteParams{
		Token:           tok,
		GroupID:         gid,
		CreatedBy:       pgUUID(creatorID),
		Kind:            kind,
		RestrictedEmail: restrictedEmail,
		MaxUses:         maxUses,
		ExpiresAt:       pgTime(time.Now().Add(ttl)),
	})
	if err != nil {
		t.Fatalf("seedGroupInvite: %v", err)
	}
	return row, tok
}

// pgTime builds a valid pgtype.Timestamptz from a time.Time. The factories
// in groups_test.go use pgUUID; this is the timestamptz sibling.
func pgTime(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

// ─── Mint group_join ─────────────────────────────────────────────────────────

func TestMintGroupJoinInvite_AdminMints(t *testing.T) {
	h, q := newGroupInviteHandler(t)
	a, g := seedGroupWithAdmin(t, q, "mgj")

	body, _ := json.Marshal(map[string]any{"max_uses": 3, "ttl_seconds": 86400})
	applyCtx := newChiCtx("id", g.ID.String())
	req := applyCtx(httptest.NewRequest(http.MethodPost, "/api/groups/"+g.ID.String()+"/invites", bytes.NewBuffer(body)))
	req = withUser(req, a.ID.String(), a.Username, a.Email, a.Role)
	rec := httptest.NewRecorder()
	h.MintGroupJoin(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d — body: %s", rec.Code, rec.Body.String())
	}
	var inv db.GroupInvite
	_ = json.Unmarshal(rec.Body.Bytes(), &inv)
	if inv.Kind != "group_join" || inv.MaxUses != 3 {
		t.Fatalf("unexpected invite shape: %+v", inv)
	}
}

func TestMintGroupJoinInvite_NonAdminForbidden(t *testing.T) {
	h, q := newGroupInviteHandler(t)
	_, g := seedGroupWithAdmin(t, q, "mgjn")
	b := seedGroupUser(t, q, "mgjn_b")
	addMember(t, q, g, b, "member")

	body, _ := json.Marshal(map[string]any{"max_uses": 1})
	applyCtx := newChiCtx("id", g.ID.String())
	req := applyCtx(httptest.NewRequest(http.MethodPost, "/api/groups/"+g.ID.String()+"/invites", bytes.NewBuffer(body)))
	req = withUser(req, b.ID.String(), b.Username, b.Email, b.Role)
	rec := httptest.NewRecorder()
	h.MintGroupJoin(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d", rec.Code)
	}
}

func TestMintGroupJoinInvite_TTLExceedsMax(t *testing.T) {
	h, q := newGroupInviteHandler(t)
	a, g := seedGroupWithAdmin(t, q, "mgjttl")

	body, _ := json.Marshal(map[string]any{"ttl_seconds": int64(31 * 24 * 3600)}) // 31 days
	applyCtx := newChiCtx("id", g.ID.String())
	req := applyCtx(httptest.NewRequest(http.MethodPost, "/api/groups/"+g.ID.String()+"/invites", bytes.NewBuffer(body)))
	req = withUser(req, a.ID.String(), a.Username, a.Email, a.Role)
	rec := httptest.NewRecorder()
	h.MintGroupJoin(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rec.Code)
	}
}

// ─── List + Revoke ──────────────────────────────────────────────────────────

func TestListGroupInvites_AdminSees(t *testing.T) {
	h, q := newGroupInviteHandler(t)
	a, g := seedGroupWithAdmin(t, q, "lgi")
	seedGroupInvite(t, q, g.ID, a.ID, "group_join", 1, time.Hour, nil)
	seedGroupInvite(t, q, g.ID, a.ID, "group_join", 1, time.Hour, nil)

	applyCtx := newChiCtx("id", g.ID.String())
	req := applyCtx(httptest.NewRequest(http.MethodGet, "/api/groups/"+g.ID.String()+"/invites", nil))
	req = withUser(req, a.ID.String(), a.Username, a.Email, a.Role)
	rec := httptest.NewRecorder()
	h.List(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	var out []db.GroupInvite
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if len(out) != 2 {
		t.Fatalf("want 2 invites, got %d", len(out))
	}
}

func TestRevokeGroupInvite_AdminRevokes(t *testing.T) {
	h, q := newGroupInviteHandler(t)
	a, g := seedGroupWithAdmin(t, q, "rgi")
	inv, _ := seedGroupInvite(t, q, g.ID, a.ID, "group_join", 1, time.Hour, nil)

	applyCtx := newChiCtxMulti(map[string]string{"id": g.ID.String(), "inviteID": inv.ID.String()})
	req := applyCtx(httptest.NewRequest(http.MethodDelete, "/api/groups/"+g.ID.String()+"/invites/"+inv.ID.String(), nil))
	req = withUser(req, a.ID.String(), a.Username, a.Email, a.Role)
	rec := httptest.NewRecorder()
	h.Revoke(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d", rec.Code)
	}
	row, _ := q.GetGroupInviteByToken(context.Background(), inv.Token)
	if !row.RevokedAt.Valid {
		t.Fatal("expected revoked_at set after revoke")
	}
}

// ─── Redeem (group_join) ────────────────────────────────────────────────────

// redeemAs builds and submits a redeem request as the given user.
func redeemAs(t *testing.T, h *api.GroupInviteHandler, u db.User, token string) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"token": token})
	req := httptest.NewRequest(http.MethodPost, "/api/groups/invites/redeem", bytes.NewBuffer(body))
	req = withUser(req, u.ID.String(), u.Username, u.Email, u.Role)
	rec := httptest.NewRecorder()
	h.Redeem(rec, req)
	return rec
}

func TestRedeemGroupInvite_HappyPath(t *testing.T) {
	h, q := newGroupInviteHandler(t)
	a, g := seedGroupWithAdmin(t, q, "rdm")
	b := seedGroupUser(t, q, "rdm_b")
	_, tok := seedGroupInvite(t, q, g.ID, a.ID, "group_join", 5, time.Hour, nil)

	rec := redeemAs(t, h, b, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d — body: %s", rec.Code, rec.Body.String())
	}
	mem, err := q.GetMembership(context.Background(), db.GetMembershipParams{GroupID: g.ID, UserID: b.ID})
	if err != nil {
		t.Fatalf("expected membership, got err %v", err)
	}
	if mem.Role != "member" {
		t.Fatalf("want role member, got %q", mem.Role)
	}
}

func TestRedeemGroupInvite_AlreadyMember(t *testing.T) {
	h, q := newGroupInviteHandler(t)
	a, g := seedGroupWithAdmin(t, q, "ramem")
	b := seedGroupUser(t, q, "ramem_b")
	addMember(t, q, g, b, "member")
	_, tok := seedGroupInvite(t, q, g.ID, a.ID, "group_join", 5, time.Hour, nil)

	rec := redeemAs(t, h, b, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("already-member redeem want 200, got %d", rec.Code)
	}
}

func TestRedeemGroupInvite_BannedUserRejected(t *testing.T) {
	h, q := newGroupInviteHandler(t)
	a, g := seedGroupWithAdmin(t, q, "rban")
	b := seedGroupUser(t, q, "rban_b")
	if err := q.CreateBan(context.Background(), db.CreateBanParams{
		GroupID: g.ID, UserID: b.ID, BannedBy: pgUUID(a.ID),
	}); err != nil {
		t.Fatalf("seed ban: %v", err)
	}
	_, tok := seedGroupInvite(t, q, g.ID, a.ID, "group_join", 1, time.Hour, nil)

	rec := redeemAs(t, h, b, tok)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d", rec.Code)
	}
	var env map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	if env["code"] != "user_banned_from_group" {
		t.Fatalf("want code user_banned_from_group, got %v", env["code"])
	}
}

func TestRedeemGroupInvite_Revoked(t *testing.T) {
	h, q := newGroupInviteHandler(t)
	a, g := seedGroupWithAdmin(t, q, "rrev")
	b := seedGroupUser(t, q, "rrev_b")
	inv, tok := seedGroupInvite(t, q, g.ID, a.ID, "group_join", 1, time.Hour, nil)
	if _, err := q.RevokeGroupInvite(context.Background(), db.RevokeGroupInviteParams{
		ID: inv.ID, GroupID: g.ID,
	}); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	rec := redeemAs(t, h, b, tok)
	if rec.Code != http.StatusGone {
		t.Fatalf("want 410, got %d", rec.Code)
	}
}

func TestRedeemGroupInvite_Expired(t *testing.T) {
	h, q := newGroupInviteHandler(t)
	a, g := seedGroupWithAdmin(t, q, "rexp")
	b := seedGroupUser(t, q, "rexp_b")
	_, tok := seedGroupInvite(t, q, g.ID, a.ID, "group_join", 1, time.Hour, nil)
	// Force expired via direct UPDATE.
	if _, err := testutil.Pool().Exec(context.Background(),
		"UPDATE group_invites SET expires_at = now() - interval '1 minute' WHERE token = $1", tok); err != nil {
		t.Fatalf("expire: %v", err)
	}
	rec := redeemAs(t, h, b, tok)
	if rec.Code != http.StatusGone {
		t.Fatalf("want 410, got %d", rec.Code)
	}
}

func TestRedeemGroupInvite_RestrictedEmailMismatch(t *testing.T) {
	h, q := newGroupInviteHandler(t)
	a, g := seedGroupWithAdmin(t, q, "rrem")
	b := seedGroupUser(t, q, "rrem_b")
	other := "someone-else@test.com"
	_, tok := seedGroupInvite(t, q, g.ID, a.ID, "group_join", 1, time.Hour, &other)

	rec := redeemAs(t, h, b, tok)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d", rec.Code)
	}
}

func TestRedeemGroupInvite_PlatformPlusKindRejected(t *testing.T) {
	h, q := newGroupInviteHandler(t)
	a, g := seedGroupWithAdmin(t, q, "rppk")
	b := seedGroupUser(t, q, "rppk_b")
	_, tok := seedGroupInvite(t, q, g.ID, a.ID, "platform_plus_group", 1, time.Hour, nil)

	rec := redeemAs(t, h, b, tok)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for platform+group via redeem, got %d", rec.Code)
	}
}

// ─── Mint platform_plus ─────────────────────────────────────────────────────

func TestMintPlatformPlus_NoQuotaRejected(t *testing.T) {
	h, q := newGroupInviteHandler(t)
	a, g := seedGroupWithAdmin(t, q, "mpp_nq")
	body, _ := json.Marshal(map[string]any{"max_uses": 1})
	applyCtx := newChiCtx("id", g.ID.String())
	req := applyCtx(httptest.NewRequest(http.MethodPost, "/api/groups/"+g.ID.String()+"/invites/platform_plus", bytes.NewBuffer(body)))
	req = withUser(req, a.ID.String(), a.Username, a.Email, a.Role)
	rec := httptest.NewRecorder()
	h.MintPlatformPlus(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("want 409 platform_plus_quota_exhausted, got %d", rec.Code)
	}
	var env map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	if env["code"] != "platform_plus_quota_exhausted" {
		t.Fatalf("want code platform_plus_quota_exhausted, got %v", env["code"])
	}
}

func TestMintPlatformPlus_WithQuotaSucceeds(t *testing.T) {
	h, q := newGroupInviteHandler(t)
	a, g := seedGroupWithAdmin(t, q, "mpp_ok")
	if _, err := q.UpsertUserInviteQuota(context.Background(), db.UpsertUserInviteQuotaParams{
		UserID: a.ID, Allocated: 3,
	}); err != nil {
		t.Fatalf("seed quota: %v", err)
	}
	body, _ := json.Marshal(map[string]any{})
	applyCtx := newChiCtx("id", g.ID.String())
	req := applyCtx(httptest.NewRequest(http.MethodPost, "/api/groups/"+g.ID.String()+"/invites/platform_plus", bytes.NewBuffer(body)))
	req = withUser(req, a.ID.String(), a.Username, a.Email, a.Role)
	rec := httptest.NewRecorder()
	h.MintPlatformPlus(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d — body: %s", rec.Code, rec.Body.String())
	}
	row, _ := q.GetUserInviteQuota(context.Background(), a.ID)
	if row.Used != 1 {
		t.Fatalf("want used=1 after mint, got %d", row.Used)
	}
}

func TestMintPlatformPlus_MaxUsesGreaterThanOneRejected(t *testing.T) {
	h, q := newGroupInviteHandler(t)
	a, g := seedGroupWithAdmin(t, q, "mpp_mu")
	if _, err := q.UpsertUserInviteQuota(context.Background(), db.UpsertUserInviteQuotaParams{
		UserID: a.ID, Allocated: 5,
	}); err != nil {
		t.Fatalf("seed quota: %v", err)
	}
	body, _ := json.Marshal(map[string]any{"max_uses": 2})
	applyCtx := newChiCtx("id", g.ID.String())
	req := applyCtx(httptest.NewRequest(http.MethodPost, "/api/groups/"+g.ID.String()+"/invites/platform_plus", bytes.NewBuffer(body)))
	req = withUser(req, a.ID.String(), a.Username, a.Email, a.Role)
	rec := httptest.NewRecorder()
	h.MintPlatformPlus(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rec.Code)
	}
}
