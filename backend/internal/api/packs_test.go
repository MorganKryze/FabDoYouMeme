
package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/api"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/config"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/middleware"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/testutil"
)

func newPackHandler(t *testing.T) (*api.PackHandler, *db.Queries) {
	t.Helper()
	pool := testutil.Pool()
	cfg := &config.Config{MaxUploadSizeBytes: 2097152}
	h := api.NewPackHandler(pool, cfg, nil) // nil storage — not needed for pack CRUD
	return h, db.New(pool)
}

func seedAdmin(t *testing.T, q *db.Queries) db.User {
	t.Helper()
	// Derive unique credentials from the test name to prevent unique-constraint
	// collisions when multiple tests run against the same shared database.
	slug := strings.ToLower(strings.NewReplacer("/", "_", " ", "_").Replace(t.Name()))
	if len(slug) > 30 {
		slug = slug[:30]
	}
	u, err := q.CreateUser(context.Background(), db.CreateUserParams{
		Username:  slug,
		Email:     slug + "@test.com",
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

func newChiCtx(key, val string) func(*http.Request) *http.Request {
	return func(r *http.Request) *http.Request {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add(key, val)
		return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
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

func TestDeletePack_Success(t *testing.T) {
	h, q := newPackHandler(t)
	admin := seedAdmin(t, q)

	pack, err := q.CreatePack(context.Background(), db.CreatePackParams{
		Name:       testutil.SeedName(t),
		OwnerID:    pgtype.UUID{Bytes: admin.ID, Valid: true},
		Visibility: "private",
	})
	if err != nil {
		t.Fatalf("create pack: %v", err)
	}

	applyCtx := newChiCtx("id", pack.ID.String())
	req := applyCtx(httptest.NewRequest(http.MethodDelete, "/api/packs/"+pack.ID.String(), nil))
	req = withUser(req, admin.ID.String(), admin.Username, admin.Email, admin.Role)
	rec := httptest.NewRecorder()
	h.Delete(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("want 204, got %d — body: %s", rec.Code, rec.Body.String())
	}
}

func TestDeletePack_NotOwner(t *testing.T) {
	h, q := newPackHandler(t)
	admin := seedAdmin(t, q)
	slug2 := testutil.SeedName(t) + "2"
	player, err := q.CreateUser(context.Background(), db.CreateUserParams{
		Username:  slug2,
		Email:     slug2 + "@test.com",
		Role:      "player",
		IsActive:  true,
		ConsentAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("create player: %v", err)
	}

	// Pack owned by admin.
	pack, _ := q.CreatePack(context.Background(), db.CreatePackParams{
		Name:       testutil.SeedName(t) + "_adm",
		OwnerID:    pgtype.UUID{Bytes: admin.ID, Valid: true},
		Visibility: "private",
	})

	applyCtx := newChiCtx("id", pack.ID.String())
	req := applyCtx(httptest.NewRequest(http.MethodDelete, "/api/packs/"+pack.ID.String(), nil))
	req = withUser(req, player.ID.String(), player.Username, player.Email, player.Role)
	rec := httptest.NewRecorder()
	h.Delete(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", rec.Code)
	}
}

func TestSetStatus_InvalidStatus(t *testing.T) {
	h, q := newPackHandler(t)
	admin := seedAdmin(t, q)

	pack, _ := q.CreatePack(context.Background(), db.CreatePackParams{
		Name:       testutil.SeedName(t) + "_st",
		OwnerID:    pgtype.UUID{Bytes: admin.ID, Valid: true},
		Visibility: "private",
	})

	body := `{"status":"invalid_value"}`
	applyCtx := newChiCtx("id", pack.ID.String())
	req := applyCtx(httptest.NewRequest(http.MethodPatch, "/api/packs/"+pack.ID.String()+"/status", bytes.NewBufferString(body)))
	req = withUser(req, admin.ID.String(), admin.Username, admin.Email, admin.Role)
	rec := httptest.NewRecorder()
	h.SetStatus(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400 for invalid status, got %d", rec.Code)
	}
}

func TestSetStatus_Flagged(t *testing.T) {
	h, q := newPackHandler(t)
	admin := seedAdmin(t, q)

	pack, _ := q.CreatePack(context.Background(), db.CreatePackParams{
		Name:       testutil.SeedName(t) + "_fl",
		OwnerID:    pgtype.UUID{Bytes: admin.ID, Valid: true},
		Visibility: "private",
	})

	body := `{"status":"flagged"}`
	applyCtx := newChiCtx("id", pack.ID.String())
	req := applyCtx(httptest.NewRequest(http.MethodPatch, "/api/packs/"+pack.ID.String()+"/status", bytes.NewBufferString(body)))
	req = withUser(req, admin.ID.String(), admin.Username, admin.Email, admin.Role)
	rec := httptest.NewRecorder()
	h.SetStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d — body: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["status"] != "flagged" {
		t.Errorf("want status=flagged, got %v", resp["status"])
	}
}
