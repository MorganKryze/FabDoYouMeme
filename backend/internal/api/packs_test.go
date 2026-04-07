//go:build integration

package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/api"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/config"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/middleware"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/testutil"
)

func newPackHandler(t *testing.T) (*api.PackHandler, *db.Queries) {
	t.Helper()
	pool, q := testutil.NewDB(t)
	cfg := &config.Config{MaxUploadSizeBytes: 2097152}
	h := api.NewPackHandler(pool, cfg, nil) // nil storage — not needed for pack CRUD
	return h, q
}

func seedAdmin(t *testing.T, q *db.Queries) db.User {
	t.Helper()
	u, err := q.CreateUser(context.Background(), db.CreateUserParams{
		Username:  "admin_pack",
		Email:     "admin_pack@test.com",
		Role:      "admin",
		IsActive:  true,
		ConsentAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("seedAdmin: %v", err)
	}
	return u
}

func withUser(r *http.Request, userID, username, email, role string) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), middleware.SessionUserContextKey, middleware.SessionUser{
		UserID: userID, Username: username, Email: email, Role: role,
	}))
}

func TestCreatePack_Success(t *testing.T) {
	h, q := newPackHandler(t)
	admin := seedAdmin(t, q)

	body := `{"name":"Test Pack","description":"desc","visibility":"private"}`
	req := httptest.NewRequest(http.MethodPost, "/api/packs", bytes.NewBufferString(body))
	req = withUser(req, admin.ID.String(), admin.Username, admin.Email, admin.Role)
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("want 201, got %d — body: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["id"] == nil {
		t.Error("expected id in response")
	}
}

func TestListPacks_ReturnsOwnPacks(t *testing.T) {
	h, q := newPackHandler(t)
	admin := seedAdmin(t, q)

	// Create a pack — OwnerID is pgtype.UUID; admin.ID is uuid.UUID ([16]byte)
	q.CreatePack(context.Background(), db.CreatePackParams{
		Name:       "My Pack",
		OwnerID:    pgtype.UUID{Bytes: admin.ID, Valid: true},
		Visibility: "private",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/packs", nil)
	req = withUser(req, admin.ID.String(), admin.Username, admin.Email, admin.Role)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	data, _ := resp["data"].([]any)
	if len(data) == 0 {
		t.Error("expected at least one pack in response")
	}
}
