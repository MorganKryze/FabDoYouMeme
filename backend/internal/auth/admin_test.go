// backend/internal/auth/admin_test.go

package auth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/auth"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/clock"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/config"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/testutil"
)

func newChiRequest(method, path, paramKey, paramVal string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(paramKey, paramVal)
	req := httptest.NewRequest(method, path, nil)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func TestDeleteUser_HardDeletes(t *testing.T) {
	h, q := newTestHandler(t)
	admin := seedUser(t, q, "admin_del", "admin_del@test.com")
	target := seedUser(t, q, "victim", "victim@test.com")

	req := newChiRequest(http.MethodDelete, "/api/admin/users/"+target.ID.String(), "id", target.ID.String())
	req = requestWithUser(req, admin.ID.String(), admin.Username, admin.Email, "admin")
	rec := httptest.NewRecorder()
	h.DeleteUser(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("want 204, got %d — body: %s", rec.Code, rec.Body.String())
	}

	// User must no longer exist
	if _, err := q.GetUserByID(context.Background(), target.ID); err == nil {
		t.Error("expected user to be deleted")
	}
}

func TestDeleteUser_CannotDeleteSentinel(t *testing.T) {
	h, q := newTestHandler(t)
	admin := seedUser(t, q, "admin_s", "admin_s@test.com")
	sentinelID := "00000000-0000-0000-0000-000000000001"

	req := newChiRequest(http.MethodDelete, "/api/admin/users/"+sentinelID, "id", sentinelID)
	req = requestWithUser(req, admin.ID.String(), admin.Username, admin.Email, "admin")
	rec := httptest.NewRecorder()
	h.DeleteUser(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("want 409, got %d", rec.Code)
	}
}

func TestDeleteUser_AuditLogCreated(t *testing.T) {
	h, q := newTestHandler(t)
	admin := seedUser(t, q, "admin_al", "admin_al@test.com")
	target := seedUser(t, q, "victim2", "victim2@test.com")

	req := newChiRequest(http.MethodDelete, "/api/admin/users/"+target.ID.String(), "id", target.ID.String())
	req = requestWithUser(req, admin.ID.String(), admin.Username, admin.Email, "admin")
	rec := httptest.NewRecorder()
	h.DeleteUser(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d", rec.Code)
	}

	// Check audit log was written
	logs, err := q.ListAuditLogs(context.Background(), db.ListAuditLogsParams{Lim: 10, Off: 0})
	if err != nil {
		t.Fatalf("list audit logs: %v", err)
	}
	var found bool
	for _, l := range logs {
		if l.Action == "hard_delete_user" && l.Resource == "user:"+target.ID.String() {
			found = true
		}
	}
	if !found {
		t.Error("expected audit log entry for hard_delete_user")
	}
}

func TestDeleteUser_CannotDeleteSelf(t *testing.T) {
	h, q := newTestHandler(t)
	admin := seedUser(t, q, "admin_self", "admin_self@test.com")

	req := newChiRequest(http.MethodDelete, "/api/admin/users/"+admin.ID.String(), "id", admin.ID.String())
	req = requestWithUser(req, admin.ID.String(), admin.Username, admin.Email, "admin")
	rec := httptest.NewRecorder()
	h.DeleteUser(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("want 409 for self-delete, got %d", rec.Code)
	}
}

func TestDeleteUser_InvalidID(t *testing.T) {
	h, q := newTestHandler(t)
	admin := seedUser(t, q, "admin_inv", "admin_inv@test.com")

	req := newChiRequest(http.MethodDelete, "/api/admin/users/not-a-uuid", "id", "not-a-uuid")
	req = requestWithUser(req, admin.ID.String(), admin.Username, admin.Email, "admin")
	rec := httptest.NewRecorder()
	h.DeleteUser(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400 for invalid UUID, got %d", rec.Code)
	}
}

func TestDeleteUser_NotFound(t *testing.T) {
	h, q := newTestHandler(t)
	admin := seedUser(t, q, "admin_nf", "admin_nf@test.com")
	nonexistent := uuid.New().String()

	req := newChiRequest(http.MethodDelete, "/api/admin/users/"+nonexistent, "id", nonexistent)
	req = requestWithUser(req, admin.ID.String(), admin.Username, admin.Email, "admin")
	rec := httptest.NewRecorder()
	h.DeleteUser(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("want 404 for nonexistent user, got %d", rec.Code)
	}
}

func TestSendMagicLink_HappyPath(t *testing.T) {
	h, q := newTestHandler(t)
	admin := seedUser(t, q, "admin_sml_ok", "admin_sml_ok@test.com")
	target := seedUser(t, q, "target_sml_ok", "target_sml_ok@test.com")

	req := newChiRequest(http.MethodPost,
		"/api/admin/users/"+target.ID.String()+"/magic-link",
		"id", target.ID.String())
	req = requestWithUser(req, admin.ID.String(), admin.Username, admin.Email, "admin")
	rec := httptest.NewRecorder()
	h.SendMagicLink(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d — body: %s", rec.Code, rec.Body.String())
	}

	// Token row was persisted.
	if _, err := q.GetLatestMagicLinkToken(context.Background(),
		db.GetLatestMagicLinkTokenParams{UserID: target.ID, Purpose: "login"}); err != nil {
		t.Errorf("expected magic link token row for target, got err: %v", err)
	}

	// Audit row was written.
	logs, err := q.ListAuditLogs(context.Background(), db.ListAuditLogsParams{Lim: 50, Off: 0})
	if err != nil {
		t.Fatalf("list audit logs: %v", err)
	}
	var found bool
	for _, l := range logs {
		if l.Action == "admin_force_magic_link" && l.Resource == "user:"+target.ID.String() {
			found = true
		}
	}
	if !found {
		t.Error("expected audit log entry for admin_force_magic_link")
	}
}

func TestSendMagicLink_NotFound(t *testing.T) {
	h, q := newTestHandler(t)
	admin := seedUser(t, q, "admin_sml_nf", "admin_sml_nf@test.com")
	missing := uuid.New().String()

	req := newChiRequest(http.MethodPost, "/api/admin/users/"+missing+"/magic-link", "id", missing)
	req = requestWithUser(req, admin.ID.String(), admin.Username, admin.Email, "admin")
	rec := httptest.NewRecorder()
	h.SendMagicLink(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", rec.Code)
	}
}

func TestSendMagicLink_Sentinel(t *testing.T) {
	h, q := newTestHandler(t)
	admin := seedUser(t, q, "admin_sml_se", "admin_sml_se@test.com")
	sentinelID := "00000000-0000-0000-0000-000000000001"

	req := newChiRequest(http.MethodPost, "/api/admin/users/"+sentinelID+"/magic-link", "id", sentinelID)
	req = requestWithUser(req, admin.ID.String(), admin.Username, admin.Email, "admin")
	rec := httptest.NewRecorder()
	h.SendMagicLink(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("want 409 for sentinel, got %d", rec.Code)
	}
}

func TestSendMagicLink_Inactive(t *testing.T) {
	h, q := newTestHandler(t)
	admin := seedUser(t, q, "admin_sml_in", "admin_sml_in@test.com")
	target := seedUser(t, q, "target_sml_in", "target_sml_in@test.com")

	if _, err := q.SetUserActive(context.Background(), db.SetUserActiveParams{
		ID: target.ID, IsActive: false,
	}); err != nil {
		t.Fatalf("deactivate target: %v", err)
	}

	req := newChiRequest(http.MethodPost,
		"/api/admin/users/"+target.ID.String()+"/magic-link",
		"id", target.ID.String())
	req = requestWithUser(req, admin.ID.String(), admin.Username, admin.Email, "admin")
	rec := httptest.NewRecorder()
	h.SendMagicLink(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("want 409 for inactive user, got %d — body: %s", rec.Code, rec.Body.String())
	}
}

func TestSendMagicLink_InvalidID(t *testing.T) {
	h, q := newTestHandler(t)
	admin := seedUser(t, q, "admin_sml_iv", "admin_sml_iv@test.com")

	req := newChiRequest(http.MethodPost, "/api/admin/users/not-a-uuid/magic-link", "id", "not-a-uuid")
	req = requestWithUser(req, admin.ID.String(), admin.Username, admin.Email, "admin")
	rec := httptest.NewRecorder()
	h.SendMagicLink(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400 for invalid UUID, got %d", rec.Code)
	}
}

func TestSendMagicLink_Cooldown(t *testing.T) {
	pool := testutil.Pool()
	cfg := &config.Config{
		FrontendURL:      "http://localhost:3000",
		MagicLinkBaseURL: "http://localhost:3000",
		MagicLinkTTL:     15 * time.Minute,
		SessionTTL:       720 * time.Hour,
	}
	clk := clock.NewFake(time.Now())
	h := auth.New(pool, cfg, &stubEmail{}, nil, clk)
	q := db.New(pool)

	admin := seedUser(t, q, "admin_sml_cd", "admin_sml_cd@test.com")
	target := seedUser(t, q, "target_sml_cd", "target_sml_cd@test.com")

	send := func() *httptest.ResponseRecorder {
		req := newChiRequest(http.MethodPost,
			"/api/admin/users/"+target.ID.String()+"/magic-link",
			"id", target.ID.String())
		req = requestWithUser(req, admin.ID.String(), admin.Username, admin.Email, "admin")
		rec := httptest.NewRecorder()
		h.SendMagicLink(rec, req)
		return rec
	}

	// First send: 204.
	if rec := send(); rec.Code != http.StatusNoContent {
		t.Fatalf("first send: want 204, got %d", rec.Code)
	}

	// Second send within cooldown: 429 with retry_after.
	rec := send()
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("during cooldown: want 429, got %d", rec.Code)
	}
	var body struct {
		Code       string `json:"code"`
		RetryAfter int    `json:"retry_after"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode 429 body: %v", err)
	}
	if body.Code != "cooldown_active" {
		t.Errorf("want code=cooldown_active, got %q", body.Code)
	}
	if body.RetryAfter <= 0 || body.RetryAfter > 16 {
		t.Errorf("want 1 <= retry_after <= 16, got %d", body.RetryAfter)
	}

	// Advance past cooldown — now it should succeed again.
	clk.Advance(16 * time.Second)
	if rec := send(); rec.Code != http.StatusNoContent {
		t.Errorf("after cooldown: want 204, got %d", rec.Code)
	}
}
