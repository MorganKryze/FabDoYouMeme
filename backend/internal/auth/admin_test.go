// backend/internal/auth/admin_test.go

package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
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
