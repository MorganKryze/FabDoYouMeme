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
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/config"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/testutil"
)

func newAdminQuotaHandler(t *testing.T) (*api.AdminQuotaHandler, *db.Queries) {
	t.Helper()
	pool := testutil.Pool()
	cfg := &config.Config{}
	return api.NewAdminQuotaHandler(pool, cfg), db.New(pool)
}

func TestListUserQuotas_Empty(t *testing.T) {
	h, _ := newAdminQuotaHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/admin/user-invite-quotas", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d — body: %s", rec.Code, rec.Body.String())
	}
}

func TestSetUserQuota_CreatesIfAbsent(t *testing.T) {
	h, q := newAdminQuotaHandler(t)
	u := seedGroupUser(t, q, "qc")

	body, _ := json.Marshal(map[string]any{"allocated": 10})
	applyCtx := newChiCtx("userID", u.ID.String())
	req := applyCtx(httptest.NewRequest(http.MethodPut, "/api/admin/user-invite-quotas/"+u.ID.String(), bytes.NewBuffer(body)))
	rec := httptest.NewRecorder()
	h.Set(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d — body: %s", rec.Code, rec.Body.String())
	}
	row, err := q.GetUserInviteQuota(context.Background(), u.ID)
	if err != nil {
		t.Fatalf("re-fetch: %v", err)
	}
	if row.Allocated != 10 || row.Used != 0 {
		t.Fatalf("want allocated=10 used=0, got %d/%d", row.Allocated, row.Used)
	}
}

func TestSetUserQuota_Updates(t *testing.T) {
	h, q := newAdminQuotaHandler(t)
	u := seedGroupUser(t, q, "qu")

	if _, err := q.UpsertUserInviteQuota(context.Background(), db.UpsertUserInviteQuotaParams{
		UserID: u.ID, Allocated: 5,
	}); err != nil {
		t.Fatalf("seed quota: %v", err)
	}
	body, _ := json.Marshal(map[string]any{"allocated": 25})
	applyCtx := newChiCtx("userID", u.ID.String())
	req := applyCtx(httptest.NewRequest(http.MethodPut, "/api/admin/user-invite-quotas/"+u.ID.String(), bytes.NewBuffer(body)))
	rec := httptest.NewRecorder()
	h.Set(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	row, _ := q.GetUserInviteQuota(context.Background(), u.ID)
	if row.Allocated != 25 {
		t.Fatalf("want allocated=25, got %d", row.Allocated)
	}
}

func TestSetUserQuota_NegativeRejected(t *testing.T) {
	h, q := newAdminQuotaHandler(t)
	u := seedGroupUser(t, q, "qn")
	body, _ := json.Marshal(map[string]any{"allocated": -1})
	applyCtx := newChiCtx("userID", u.ID.String())
	req := applyCtx(httptest.NewRequest(http.MethodPut, "/api/admin/user-invite-quotas/"+u.ID.String(), bytes.NewBuffer(body)))
	rec := httptest.NewRecorder()
	h.Set(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rec.Code)
	}
}

func TestSetUserQuota_BelowUsedRejected(t *testing.T) {
	h, q := newAdminQuotaHandler(t)
	u := seedGroupUser(t, q, "qb")

	// Seed quota with allocated=10, then bump used=5 directly so the
	// allocated < used check has something to trip on.
	if _, err := q.UpsertUserInviteQuota(context.Background(), db.UpsertUserInviteQuotaParams{
		UserID: u.ID, Allocated: 10,
	}); err != nil {
		t.Fatalf("seed quota: %v", err)
	}
	if _, err := testutil.Pool().Exec(context.Background(),
		"UPDATE user_invite_quotas SET used = 5 WHERE user_id = $1", u.ID); err != nil {
		t.Fatalf("seed used: %v", err)
	}

	body, _ := json.Marshal(map[string]any{"allocated": 2}) // below used
	applyCtx := newChiCtx("userID", u.ID.String())
	req := applyCtx(httptest.NewRequest(http.MethodPut, "/api/admin/user-invite-quotas/"+u.ID.String(), bytes.NewBuffer(body)))
	rec := httptest.NewRecorder()
	h.Set(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d", rec.Code)
	}
	var env map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	if env["code"] != "quota_below_used" {
		t.Fatalf("want code quota_below_used, got %v", env["code"])
	}
}
