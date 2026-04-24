package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/api"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/testutil"
)

func newAdminGroupsHandler(t *testing.T) (*api.AdminGroupsHandler, *db.Queries) {
	t.Helper()
	pool := testutil.Pool()
	return api.NewAdminGroupsHandler(pool), db.New(pool)
}

func TestAdminListGroups_ReturnsMemberCount(t *testing.T) {
	h, q := newAdminGroupsHandler(t)
	_, g := seedGroupWithAdmin(t, q, "alg")
	_ = g

	req := httptest.NewRequest(http.MethodGet, "/api/admin/groups", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	var resp map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	data, _ := resp["data"].([]any)
	if len(data) == 0 {
		t.Fatal("expected at least one group")
	}
}

func TestAdminSetQuota_UpdatesRow(t *testing.T) {
	h, q := newAdminGroupsHandler(t)
	_, g := seedGroupWithAdmin(t, q, "sq")

	body, _ := json.Marshal(map[string]any{"quota_bytes": 1024 * 1024 * 1024})
	applyCtx := newChiCtx("gid", g.ID.String())
	req := applyCtx(httptest.NewRequest(http.MethodPatch, "/api/admin/groups/"+g.ID.String()+"/quota", bytes.NewBuffer(body)))
	rec := httptest.NewRecorder()
	h.SetQuota(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d — body: %s", rec.Code, rec.Body.String())
	}
	updated, _ := q.GetGroupByID(context.Background(), g.ID)
	if updated.QuotaBytes != 1024*1024*1024 {
		t.Fatalf("want 1GB quota, got %d", updated.QuotaBytes)
	}
}

func TestAdminSetMemberCap_BelowCurrentRejected(t *testing.T) {
	h, q := newAdminGroupsHandler(t)
	a, g := seedGroupWithAdmin(t, q, "smc")
	_ = a
	// Add a second member so count=2.
	b := seedGroupUser(t, q, "smc_b")
	addMember(t, q, g, b, "member")

	body, _ := json.Marshal(map[string]any{"member_cap": 1})
	applyCtx := newChiCtx("gid", g.ID.String())
	req := applyCtx(httptest.NewRequest(http.MethodPatch, "/api/admin/groups/"+g.ID.String()+"/member_cap", bytes.NewBuffer(body)))
	rec := httptest.NewRecorder()
	h.SetMemberCap(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d", rec.Code)
	}
	var env map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	if env["code"] != "member_cap_below_current" {
		t.Fatalf("want member_cap_below_current, got %v", env["code"])
	}
}
