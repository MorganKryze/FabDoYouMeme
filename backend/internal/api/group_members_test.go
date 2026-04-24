package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/api"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/config"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/testutil"
)

// newGroupMemberHandler returns a member handler with FeatureGroups=true.
func newGroupMemberHandler(t *testing.T) (*api.GroupMemberHandler, *db.Queries) {
	t.Helper()
	pool := testutil.Pool()
	cfg := &config.Config{MaxGroupsPerUser: 50, MaxGroupMembershipsPerUser: 50}
	return api.NewGroupMemberHandler(pool, cfg), db.New(pool)
}

// seedGroupWithAdmin builds a fresh user, creates a group with that user as
// admin, and returns both. Test helpers downstream extend with extra members.
func seedGroupWithAdmin(t *testing.T, q *db.Queries, slug string) (db.User, db.Group) {
	t.Helper()
	a := seedGroupUser(t, q, slug)
	g, err := q.CreateGroup(context.Background(), db.CreateGroupParams{
		Name:           "G_" + slug + "_" + uuid.NewString()[:8],
		Description:    "seed",
		Language:       "en",
		Classification: "sfw",
		QuotaBytes:     500 * 1024 * 1024,
		MemberCap:      100,
		CreatedBy:      pgUUID(a.ID),
	})
	if err != nil {
		t.Fatalf("seedGroupWithAdmin: create group: %v", err)
	}
	if _, err := q.CreateMembership(context.Background(), db.CreateMembershipParams{
		GroupID: g.ID, UserID: a.ID, Role: "admin",
	}); err != nil {
		t.Fatalf("seedGroupWithAdmin: create admin membership: %v", err)
	}
	return a, g
}

func addMember(t *testing.T, q *db.Queries, g db.Group, u db.User, role string) {
	t.Helper()
	if _, err := q.CreateMembership(context.Background(), db.CreateMembershipParams{
		GroupID: g.ID, UserID: u.ID, Role: role,
	}); err != nil {
		t.Fatalf("addMember(%s): %v", role, err)
	}
}

// ─── List ────────────────────────────────────────────────────────────────────

func TestListGroupMembers_ReturnsRoleAndUsername(t *testing.T) {
	h, q := newGroupMemberHandler(t)
	a, g := seedGroupWithAdmin(t, q, "lgm")
	b := seedGroupUser(t, q, "lgm_b")
	addMember(t, q, g, b, "member")

	applyCtx := newChiCtx("id", g.ID.String())
	req := applyCtx(httptest.NewRequest(http.MethodGet, "/api/groups/"+g.ID.String()+"/members", nil))
	req = withUser(req, a.ID.String(), a.Username, a.Email, a.Role)
	rec := httptest.NewRecorder()
	h.List(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d — body: %s", rec.Code, rec.Body.String())
	}
	var rows []map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &rows)
	if len(rows) != 2 {
		t.Fatalf("want 2 members, got %d", len(rows))
	}
	if rows[0]["role"] != "admin" {
		t.Fatalf("admin should sort first, got %v", rows[0]["role"])
	}
}

// ─── Kick ────────────────────────────────────────────────────────────────────

func TestKickMember_AdminRemovesNonAdmin(t *testing.T) {
	h, q := newGroupMemberHandler(t)
	a, g := seedGroupWithAdmin(t, q, "kick")
	b := seedGroupUser(t, q, "kick_b")
	addMember(t, q, g, b, "member")

	applyCtx := newChiCtxMulti(map[string]string{"id": g.ID.String(), "userID": b.ID.String()})
	req := applyCtx(httptest.NewRequest(http.MethodDelete, "/api/groups/"+g.ID.String()+"/members/"+b.ID.String(), nil))
	req = withUser(req, a.ID.String(), a.Username, a.Email, a.Role)
	rec := httptest.NewRecorder()
	h.Kick(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d — body: %s", rec.Code, rec.Body.String())
	}
	if _, err := q.GetMembership(context.Background(), db.GetMembershipParams{
		GroupID: g.ID, UserID: b.ID,
	}); err == nil {
		t.Fatal("expected membership gone after kick")
	}
}

func TestKickMember_NonAdminForbidden(t *testing.T) {
	h, q := newGroupMemberHandler(t)
	_, g := seedGroupWithAdmin(t, q, "kna")
	b := seedGroupUser(t, q, "kna_b")
	c := seedGroupUser(t, q, "kna_c")
	addMember(t, q, g, b, "member")
	addMember(t, q, g, c, "member")

	applyCtx := newChiCtxMulti(map[string]string{"id": g.ID.String(), "userID": c.ID.String()})
	req := applyCtx(httptest.NewRequest(http.MethodDelete, "/api/groups/"+g.ID.String()+"/members/"+c.ID.String(), nil))
	req = withUser(req, b.ID.String(), b.Username, b.Email, b.Role)
	rec := httptest.NewRecorder()
	h.Kick(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d", rec.Code)
	}
}

func TestKickMember_CannotKickSelf(t *testing.T) {
	h, q := newGroupMemberHandler(t)
	a, g := seedGroupWithAdmin(t, q, "kself")

	applyCtx := newChiCtxMulti(map[string]string{"id": g.ID.String(), "userID": a.ID.String()})
	req := applyCtx(httptest.NewRequest(http.MethodDelete, "/api/groups/"+g.ID.String()+"/members/"+a.ID.String(), nil))
	req = withUser(req, a.ID.String(), a.Username, a.Email, a.Role)
	rec := httptest.NewRecorder()
	h.Kick(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("want 409 cannot_kick_self, got %d", rec.Code)
	}
	var env map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	if env["code"] != "cannot_kick_self" {
		t.Fatalf("want code cannot_kick_self, got %v", env["code"])
	}
}

func TestKickMember_TargetNotMember(t *testing.T) {
	h, q := newGroupMemberHandler(t)
	a, g := seedGroupWithAdmin(t, q, "knm")
	b := seedGroupUser(t, q, "knm_b") // never a member

	applyCtx := newChiCtxMulti(map[string]string{"id": g.ID.String(), "userID": b.ID.String()})
	req := applyCtx(httptest.NewRequest(http.MethodDelete, "/api/groups/"+g.ID.String()+"/members/"+b.ID.String(), nil))
	req = withUser(req, a.ID.String(), a.Username, a.Email, a.Role)
	rec := httptest.NewRecorder()
	h.Kick(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", rec.Code)
	}
}

// ─── Ban / Unban / ListBans ─────────────────────────────────────────────────

func TestBanMember_AdminBans(t *testing.T) {
	h, q := newGroupMemberHandler(t)
	a, g := seedGroupWithAdmin(t, q, "ban")
	b := seedGroupUser(t, q, "ban_b")
	addMember(t, q, g, b, "member")

	body, _ := json.Marshal(map[string]any{"user_id": b.ID.String()})
	applyCtx := newChiCtx("id", g.ID.String())
	req := applyCtx(httptest.NewRequest(http.MethodPost, "/api/groups/"+g.ID.String()+"/bans", bytes.NewBuffer(body)))
	req = withUser(req, a.ID.String(), a.Username, a.Email, a.Role)
	rec := httptest.NewRecorder()
	h.Ban(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d — body: %s", rec.Code, rec.Body.String())
	}
	if _, err := q.GetMembership(context.Background(), db.GetMembershipParams{GroupID: g.ID, UserID: b.ID}); err == nil {
		t.Fatal("expected membership gone after ban")
	}
	banned, err := q.IsUserBannedFromGroup(context.Background(), db.IsUserBannedFromGroupParams{GroupID: g.ID, UserID: b.ID})
	if err != nil || !banned {
		t.Fatalf("expected ban row present, got banned=%v err=%v", banned, err)
	}
}

func TestBanMember_NonAdminForbidden(t *testing.T) {
	h, q := newGroupMemberHandler(t)
	_, g := seedGroupWithAdmin(t, q, "bna")
	b := seedGroupUser(t, q, "bna_b")
	c := seedGroupUser(t, q, "bna_c")
	addMember(t, q, g, b, "member")
	addMember(t, q, g, c, "member")

	body, _ := json.Marshal(map[string]any{"user_id": c.ID.String()})
	applyCtx := newChiCtx("id", g.ID.String())
	req := applyCtx(httptest.NewRequest(http.MethodPost, "/api/groups/"+g.ID.String()+"/bans", bytes.NewBuffer(body)))
	req = withUser(req, b.ID.String(), b.Username, b.Email, b.Role)
	rec := httptest.NewRecorder()
	h.Ban(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d", rec.Code)
	}
}

func TestBanMember_CannotBanSelf(t *testing.T) {
	h, q := newGroupMemberHandler(t)
	a, g := seedGroupWithAdmin(t, q, "bself")

	body, _ := json.Marshal(map[string]any{"user_id": a.ID.String()})
	applyCtx := newChiCtx("id", g.ID.String())
	req := applyCtx(httptest.NewRequest(http.MethodPost, "/api/groups/"+g.ID.String()+"/bans", bytes.NewBuffer(body)))
	req = withUser(req, a.ID.String(), a.Username, a.Email, a.Role)
	rec := httptest.NewRecorder()
	h.Ban(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d", rec.Code)
	}
}

func TestUnbanMember_AdminRemovesBan(t *testing.T) {
	h, q := newGroupMemberHandler(t)
	a, g := seedGroupWithAdmin(t, q, "ub")
	b := seedGroupUser(t, q, "ub_b")
	if err := q.CreateBan(context.Background(), db.CreateBanParams{
		GroupID: g.ID, UserID: b.ID, BannedBy: pgUUID(a.ID),
	}); err != nil {
		t.Fatalf("seed ban: %v", err)
	}
	applyCtx := newChiCtxMulti(map[string]string{"id": g.ID.String(), "userID": b.ID.String()})
	req := applyCtx(httptest.NewRequest(http.MethodDelete, "/api/groups/"+g.ID.String()+"/bans/"+b.ID.String(), nil))
	req = withUser(req, a.ID.String(), a.Username, a.Email, a.Role)
	rec := httptest.NewRecorder()
	h.Unban(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d", rec.Code)
	}
	banned, _ := q.IsUserBannedFromGroup(context.Background(), db.IsUserBannedFromGroupParams{GroupID: g.ID, UserID: b.ID})
	if banned {
		t.Fatal("expected ban gone after unban")
	}
}

func TestListGroupBans(t *testing.T) {
	h, q := newGroupMemberHandler(t)
	a, g := seedGroupWithAdmin(t, q, "lb")
	b := seedGroupUser(t, q, "lb_b")
	if err := q.CreateBan(context.Background(), db.CreateBanParams{
		GroupID: g.ID, UserID: b.ID, BannedBy: pgUUID(a.ID),
	}); err != nil {
		t.Fatalf("seed ban: %v", err)
	}
	applyCtx := newChiCtx("id", g.ID.String())
	req := applyCtx(httptest.NewRequest(http.MethodGet, "/api/groups/"+g.ID.String()+"/bans", nil))
	req = withUser(req, a.ID.String(), a.Username, a.Email, a.Role)
	rec := httptest.NewRecorder()
	h.ListBans(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	var rows []map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &rows)
	if len(rows) != 1 {
		t.Fatalf("want 1 ban, got %d", len(rows))
	}
}

// ─── Promote / SelfDemote / Leave ───────────────────────────────────────────

func TestPromoteMember_AdminPromotes(t *testing.T) {
	h, q := newGroupMemberHandler(t)
	a, g := seedGroupWithAdmin(t, q, "pr")
	b := seedGroupUser(t, q, "pr_b")
	addMember(t, q, g, b, "member")

	applyCtx := newChiCtxMulti(map[string]string{"id": g.ID.String(), "userID": b.ID.String()})
	req := applyCtx(httptest.NewRequest(http.MethodPost, "/api/groups/"+g.ID.String()+"/members/"+b.ID.String()+"/promote", nil))
	req = withUser(req, a.ID.String(), a.Username, a.Email, a.Role)
	rec := httptest.NewRecorder()
	h.Promote(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d — body: %s", rec.Code, rec.Body.String())
	}
	mem, err := q.GetMembership(context.Background(), db.GetMembershipParams{GroupID: g.ID, UserID: b.ID})
	if err != nil || mem.Role != "admin" {
		t.Fatalf("expected promoted to admin, got role=%q err=%v", mem.Role, err)
	}
}

func TestPromoteMember_NonAdminForbidden(t *testing.T) {
	h, q := newGroupMemberHandler(t)
	_, g := seedGroupWithAdmin(t, q, "prnf")
	b := seedGroupUser(t, q, "prnf_b")
	c := seedGroupUser(t, q, "prnf_c")
	addMember(t, q, g, b, "member")
	addMember(t, q, g, c, "member")
	applyCtx := newChiCtxMulti(map[string]string{"id": g.ID.String(), "userID": c.ID.String()})
	req := applyCtx(httptest.NewRequest(http.MethodPost, "/api/groups/"+g.ID.String()+"/members/"+c.ID.String()+"/promote", nil))
	req = withUser(req, b.ID.String(), b.Username, b.Email, b.Role)
	rec := httptest.NewRecorder()
	h.Promote(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d", rec.Code)
	}
}

func TestSelfDemote_OtherAdminExists(t *testing.T) {
	h, q := newGroupMemberHandler(t)
	a, g := seedGroupWithAdmin(t, q, "sdo")
	b := seedGroupUser(t, q, "sdo_b")
	addMember(t, q, g, b, "admin")

	applyCtx := newChiCtx("id", g.ID.String())
	req := applyCtx(httptest.NewRequest(http.MethodPost, "/api/groups/"+g.ID.String()+"/members/self/demote", nil))
	req = withUser(req, a.ID.String(), a.Username, a.Email, a.Role)
	rec := httptest.NewRecorder()
	h.SelfDemote(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d — body: %s", rec.Code, rec.Body.String())
	}
	mem, _ := q.GetMembership(context.Background(), db.GetMembershipParams{GroupID: g.ID, UserID: a.ID})
	if mem.Role != "member" {
		t.Fatalf("want role member after demote, got %q", mem.Role)
	}
}

func TestSelfDemote_SoleAdminRejected(t *testing.T) {
	h, q := newGroupMemberHandler(t)
	a, g := seedGroupWithAdmin(t, q, "sdr")
	applyCtx := newChiCtx("id", g.ID.String())
	req := applyCtx(httptest.NewRequest(http.MethodPost, "/api/groups/"+g.ID.String()+"/members/self/demote", nil))
	req = withUser(req, a.ID.String(), a.Username, a.Email, a.Role)
	rec := httptest.NewRecorder()
	h.SelfDemote(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d", rec.Code)
	}
	var env map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	if env["code"] != "last_admin_cannot_leave" {
		t.Fatalf("want code last_admin_cannot_leave, got %v", env["code"])
	}
}

func TestLeaveGroup_MemberLeaves(t *testing.T) {
	h, q := newGroupMemberHandler(t)
	_, g := seedGroupWithAdmin(t, q, "lv")
	b := seedGroupUser(t, q, "lv_b")
	addMember(t, q, g, b, "member")
	applyCtx := newChiCtx("id", g.ID.String())
	req := applyCtx(httptest.NewRequest(http.MethodDelete, "/api/groups/"+g.ID.String()+"/members/self", nil))
	req = withUser(req, b.ID.String(), b.Username, b.Email, b.Role)
	rec := httptest.NewRecorder()
	h.Leave(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d", rec.Code)
	}
}

func TestLeaveGroup_SoleAdminRejected(t *testing.T) {
	h, q := newGroupMemberHandler(t)
	a, g := seedGroupWithAdmin(t, q, "lvsa")
	applyCtx := newChiCtx("id", g.ID.String())
	req := applyCtx(httptest.NewRequest(http.MethodDelete, "/api/groups/"+g.ID.String()+"/members/self", nil))
	req = withUser(req, a.ID.String(), a.Username, a.Email, a.Role)
	rec := httptest.NewRecorder()
	h.Leave(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d", rec.Code)
	}
}

func TestLeaveGroup_AdminWithPeerAdminSucceeds(t *testing.T) {
	h, q := newGroupMemberHandler(t)
	a, g := seedGroupWithAdmin(t, q, "lvpa")
	b := seedGroupUser(t, q, "lvpa_b")
	addMember(t, q, g, b, "admin")
	applyCtx := newChiCtx("id", g.ID.String())
	req := applyCtx(httptest.NewRequest(http.MethodDelete, "/api/groups/"+g.ID.String()+"/members/self", nil))
	req = withUser(req, a.ID.String(), a.Username, a.Email, a.Role)
	rec := httptest.NewRecorder()
	h.Leave(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d — body: %s", rec.Code, rec.Body.String())
	}
}
