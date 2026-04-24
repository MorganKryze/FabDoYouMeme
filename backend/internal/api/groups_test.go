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
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/api"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/config"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/testutil"
)

// newChiCtxMulti is the multi-param sibling of newChiCtx (defined in
// packs_test.go). The packs variant only takes one param; routes like
// /groups/{id}/members/{userID} need both.
func newChiCtxMulti(params map[string]string) func(*http.Request) *http.Request {
	return func(r *http.Request) *http.Request {
		rctx := chi.NewRouteContext()
		for k, v := range params {
			rctx.URLParams.Add(k, v)
		}
		return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	}
}

// pgUUID wraps uuid.UUID in the pgtype.UUID shape sqlc expects for nullable
// FK params (CreatedBy, BannedBy, etc.). Centralised here so test bodies
// stay readable.
func pgUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

// newGroupHandler builds a handler with the given per-user create cap
// (default 5 — matches production default). Pass 0 to keep the default;
// otherwise the cap field overrides it.
func newGroupHandler(t *testing.T, maxGroupsPerUser int) (*api.GroupHandler, *db.Queries) {
	t.Helper()
	pool := testutil.Pool()
	cfg := &config.Config{
		MaxGroupsPerUser:           5,
		MaxGroupMembershipsPerUser: 20,
	}
	if maxGroupsPerUser > 0 {
		cfg.MaxGroupsPerUser = maxGroupsPerUser
	}
	return api.NewGroupHandler(pool, cfg), db.New(pool)
}

// seedGroupUser creates a fresh user with a unique slug per call so multiple
// users can coexist in the same test run. Pass a numeric suffix to
// distinguish callers within one test.
func seedGroupUser(t *testing.T, q *db.Queries, suffix string) db.User {
	t.Helper()
	slug := strings.ToLower(strings.NewReplacer("/", "_", " ", "_").Replace(t.Name()))
	if len(slug) > 25 {
		slug = slug[:25]
	}
	slug = slug + "_" + suffix
	u, err := q.CreateUser(context.Background(), db.CreateUserParams{
		Username:  slug,
		Email:     slug + "@test.com",
		Role:      "player",
		IsActive:  true,
		ConsentAt: time.Now().UTC(),
		Locale:    "en",
	})
	if err != nil {
		t.Fatalf("seedGroupUser(%s): %v", suffix, err)
	}
	return u
}

// createGroupAs builds and submits a POST /api/groups request as the given
// user, returning the response and decoded body for assertions.
func createGroupAs(t *testing.T, h *api.GroupHandler, u db.User, body map[string]any) (*httptest.ResponseRecorder, db.Group) {
	t.Helper()
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/groups", bytes.NewBuffer(raw))
	req = withUser(req, u.ID.String(), u.Username, u.Email, u.Role)
	rec := httptest.NewRecorder()
	h.Create(rec, req)
	var out db.Group
	if rec.Code == http.StatusCreated {
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
	}
	return rec, out
}

// ─── Create ──────────────────────────────────────────────────────────────────

func TestCreateGroup_Success(t *testing.T) {
	h, q := newGroupHandler(t, 0)
	u := seedGroupUser(t, q, "a")

	rec, g := createGroupAs(t, h, u, map[string]any{
		"name":           "Movie Night " + u.Username,
		"description":    "Weekly meme showdown",
		"language":       "en",
		"classification": "sfw",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d — body: %s", rec.Code, rec.Body.String())
	}
	if g.Name != "Movie Night "+u.Username {
		t.Fatalf("want name 'Movie Night %s', got %q", u.Username, g.Name)
	}
	mem, err := q.GetMembership(context.Background(), db.GetMembershipParams{
		GroupID: g.ID, UserID: u.ID,
	})
	if err != nil {
		t.Fatalf("creator membership not found: %v", err)
	}
	if mem.Role != "admin" {
		t.Fatalf("want creator role admin, got %q", mem.Role)
	}
}

func TestCreateGroup_DuplicateNameCaseInsensitive(t *testing.T) {
	h, q := newGroupHandler(t, 0)
	u1 := seedGroupUser(t, q, "a")
	u2 := seedGroupUser(t, q, "b")
	name := "DupName" + strings.ReplaceAll(uuid.NewString(), "-", "")[:8]

	rec, _ := createGroupAs(t, h, u1, map[string]any{
		"name": name, "description": "first", "language": "en", "classification": "sfw",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("first create want 201, got %d — body: %s", rec.Code, rec.Body.String())
	}
	rec2, _ := createGroupAs(t, h, u2, map[string]any{
		"name": strings.ToLower(name), "description": "second", "language": "en", "classification": "sfw",
	})
	if rec2.Code != http.StatusConflict {
		t.Fatalf("dup-case create want 409, got %d", rec2.Code)
	}
	var env map[string]any
	_ = json.Unmarshal(rec2.Body.Bytes(), &env)
	if env["code"] != "group_name_taken" {
		t.Fatalf("want code group_name_taken, got %v", env["code"])
	}
}

func TestCreateGroup_CapReached(t *testing.T) {
	h, q := newGroupHandler(t, 1) // cap = 1
	u := seedGroupUser(t, q, "a")

	rec, _ := createGroupAs(t, h, u, map[string]any{
		"name": "first" + uuid.NewString()[:8], "description": "x", "language": "en", "classification": "sfw",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("first want 201, got %d", rec.Code)
	}
	rec2, _ := createGroupAs(t, h, u, map[string]any{
		"name": "second" + uuid.NewString()[:8], "description": "y", "language": "en", "classification": "sfw",
	})
	if rec2.Code != http.StatusConflict {
		t.Fatalf("second want 409, got %d", rec2.Code)
	}
	var env map[string]any
	_ = json.Unmarshal(rec2.Body.Bytes(), &env)
	if env["code"] != "group_cap_reached" {
		t.Fatalf("want code group_cap_reached, got %v", env["code"])
	}
}

func TestCreateGroup_InvalidClassification(t *testing.T) {
	h, q := newGroupHandler(t, 0)
	u := seedGroupUser(t, q, "a")
	rec, _ := createGroupAs(t, h, u, map[string]any{
		"name": "x" + uuid.NewString()[:8], "description": "y", "language": "en", "classification": "maybe",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rec.Code)
	}
}

func TestCreateGroup_InvalidLanguage(t *testing.T) {
	h, q := newGroupHandler(t, 0)
	u := seedGroupUser(t, q, "a")
	rec, _ := createGroupAs(t, h, u, map[string]any{
		"name": "x" + uuid.NewString()[:8], "description": "y", "language": "klingon", "classification": "sfw",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rec.Code)
	}
}

func TestCreateGroup_DescriptionTooLong(t *testing.T) {
	h, q := newGroupHandler(t, 0)
	u := seedGroupUser(t, q, "a")
	rec, _ := createGroupAs(t, h, u, map[string]any{
		"name":           "x" + uuid.NewString()[:8],
		"description":    strings.Repeat("y", 501),
		"language":       "en",
		"classification": "sfw",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rec.Code)
	}
}

func TestCreateGroup_EmptyDescription(t *testing.T) {
	h, q := newGroupHandler(t, 0)
	u := seedGroupUser(t, q, "a")
	rec, _ := createGroupAs(t, h, u, map[string]any{
		"name": "x" + uuid.NewString()[:8], "description": "  ", "language": "en", "classification": "sfw",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for empty description, got %d", rec.Code)
	}
}

// ─── List + Get ──────────────────────────────────────────────────────────────

func TestListGroups_OnlyIncludesUserMemberships(t *testing.T) {
	h, q := newGroupHandler(t, 0)
	a := seedGroupUser(t, q, "a")
	b := seedGroupUser(t, q, "b")
	createGroupAs(t, h, a, map[string]any{
		"name": "GroupA" + uuid.NewString()[:8], "description": "x", "language": "en", "classification": "sfw",
	})
	createGroupAs(t, h, b, map[string]any{
		"name": "GroupB" + uuid.NewString()[:8], "description": "y", "language": "en", "classification": "sfw",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/groups", nil)
	req = withUser(req, a.ID.String(), a.Username, a.Email, a.Role)
	rec := httptest.NewRecorder()
	h.List(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	var rows []map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &rows)
	if len(rows) != 1 {
		t.Fatalf("want 1 group for A, got %d (body: %s)", len(rows), rec.Body.String())
	}
}

func TestGetGroup_NonMemberForbidden(t *testing.T) {
	h, q := newGroupHandler(t, 0)
	a := seedGroupUser(t, q, "a")
	b := seedGroupUser(t, q, "b")
	_, g := createGroupAs(t, h, a, map[string]any{
		"name": "Cats" + uuid.NewString()[:8], "description": "x", "language": "en", "classification": "sfw",
	})

	applyCtx := newChiCtx("id", g.ID.String())
	req := applyCtx(httptest.NewRequest(http.MethodGet, "/api/groups/"+g.ID.String(), nil))
	req = withUser(req, b.ID.String(), b.Username, b.Email, b.Role)
	rec := httptest.NewRecorder()
	h.Get(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d", rec.Code)
	}
	var env map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	if env["code"] != "not_group_member" {
		t.Fatalf("want code not_group_member, got %v", env["code"])
	}
}

func TestGetGroup_SoftDeletedReturns404(t *testing.T) {
	h, q := newGroupHandler(t, 0)
	a := seedGroupUser(t, q, "a")
	_, g := createGroupAs(t, h, a, map[string]any{
		"name": "Trash" + uuid.NewString()[:8], "description": "x", "language": "en", "classification": "sfw",
	})
	if err := q.SoftDeleteGroup(context.Background(), g.ID); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	applyCtx := newChiCtx("id", g.ID.String())
	req := applyCtx(httptest.NewRequest(http.MethodGet, "/api/groups/"+g.ID.String(), nil))
	req = withUser(req, a.ID.String(), a.Username, a.Email, a.Role)
	rec := httptest.NewRecorder()
	h.Get(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", rec.Code)
	}
}

// ─── Update ──────────────────────────────────────────────────────────────────

func TestUpdateGroup_AdminChangesNameAndDescription(t *testing.T) {
	h, q := newGroupHandler(t, 0)
	a := seedGroupUser(t, q, "a")
	_, g := createGroupAs(t, h, a, map[string]any{
		"name": "Old" + uuid.NewString()[:8], "description": "x", "language": "en", "classification": "sfw",
	})
	newName := "New" + uuid.NewString()[:8]
	body, _ := json.Marshal(map[string]any{"name": newName, "description": "z"})

	applyCtx := newChiCtx("id", g.ID.String())
	req := applyCtx(httptest.NewRequest(http.MethodPatch, "/api/groups/"+g.ID.String(), bytes.NewBuffer(body)))
	req = withUser(req, a.ID.String(), a.Username, a.Email, a.Role)
	rec := httptest.NewRecorder()
	h.Update(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d — body: %s", rec.Code, rec.Body.String())
	}
	var out db.Group
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if out.Name != newName || out.Description != "z" {
		t.Fatalf("update mismatch: got name=%q desc=%q", out.Name, out.Description)
	}
}

func TestUpdateGroup_NonAdminForbidden(t *testing.T) {
	h, q := newGroupHandler(t, 0)
	a := seedGroupUser(t, q, "a")
	b := seedGroupUser(t, q, "b")
	_, g := createGroupAs(t, h, a, map[string]any{
		"name": "Lounge" + uuid.NewString()[:8], "description": "x", "language": "en", "classification": "sfw",
	})
	if _, err := q.CreateMembership(context.Background(), db.CreateMembershipParams{
		GroupID: g.ID, UserID: b.ID, Role: "member",
	}); err != nil {
		t.Fatalf("seed member: %v", err)
	}
	body, _ := json.Marshal(map[string]any{"name": "newname"})
	applyCtx := newChiCtx("id", g.ID.String())
	req := applyCtx(httptest.NewRequest(http.MethodPatch, "/api/groups/"+g.ID.String(), bytes.NewBuffer(body)))
	req = withUser(req, b.ID.String(), b.Username, b.Email, b.Role)
	rec := httptest.NewRecorder()
	h.Update(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d", rec.Code)
	}
	var env map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	if env["code"] != "not_group_admin" {
		t.Fatalf("want code not_group_admin, got %v", env["code"])
	}
}

func TestUpdateGroup_DuplicateNameCollision(t *testing.T) {
	h, q := newGroupHandler(t, 0)
	a := seedGroupUser(t, q, "a")
	b := seedGroupUser(t, q, "b")
	nameA := "Cats" + uuid.NewString()[:8]
	nameB := "Dogs" + uuid.NewString()[:8]
	createGroupAs(t, h, a, map[string]any{
		"name": nameA, "description": "x", "language": "en", "classification": "sfw",
	})
	_, gB := createGroupAs(t, h, b, map[string]any{
		"name": nameB, "description": "y", "language": "en", "classification": "sfw",
	})
	body, _ := json.Marshal(map[string]any{"name": strings.ToLower(nameA)})
	applyCtx := newChiCtx("id", gB.ID.String())
	req := applyCtx(httptest.NewRequest(http.MethodPatch, "/api/groups/"+gB.ID.String(), bytes.NewBuffer(body)))
	req = withUser(req, b.ID.String(), b.Username, b.Email, b.Role)
	rec := httptest.NewRecorder()
	h.Update(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d", rec.Code)
	}
	var env map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	if env["code"] != "group_name_taken" {
		t.Fatalf("want code group_name_taken, got %v", env["code"])
	}
}

// ─── Delete + Restore ────────────────────────────────────────────────────────

func TestDeleteGroup_AdminSoftDeletes(t *testing.T) {
	h, q := newGroupHandler(t, 0)
	a := seedGroupUser(t, q, "a")
	_, g := createGroupAs(t, h, a, map[string]any{
		"name": "Bin" + uuid.NewString()[:8], "description": "x", "language": "en", "classification": "sfw",
	})
	applyCtx := newChiCtx("id", g.ID.String())
	req := applyCtx(httptest.NewRequest(http.MethodDelete, "/api/groups/"+g.ID.String(), nil))
	req = withUser(req, a.ID.String(), a.Username, a.Email, a.Role)
	rec := httptest.NewRecorder()
	h.Delete(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d", rec.Code)
	}
	row, err := q.GetGroupByIDIncludingDeleted(context.Background(), g.ID)
	if err != nil {
		t.Fatalf("re-fetch: %v", err)
	}
	if !row.DeletedAt.Valid {
		t.Fatal("expected deleted_at to be set")
	}
}

func TestDeleteGroup_NonAdminForbidden(t *testing.T) {
	h, q := newGroupHandler(t, 0)
	a := seedGroupUser(t, q, "a")
	b := seedGroupUser(t, q, "b")
	_, g := createGroupAs(t, h, a, map[string]any{
		"name": "Lounge" + uuid.NewString()[:8], "description": "x", "language": "en", "classification": "sfw",
	})
	if _, err := q.CreateMembership(context.Background(), db.CreateMembershipParams{
		GroupID: g.ID, UserID: b.ID, Role: "member",
	}); err != nil {
		t.Fatalf("seed member: %v", err)
	}
	applyCtx := newChiCtx("id", g.ID.String())
	req := applyCtx(httptest.NewRequest(http.MethodDelete, "/api/groups/"+g.ID.String(), nil))
	req = withUser(req, b.ID.String(), b.Username, b.Email, b.Role)
	rec := httptest.NewRecorder()
	h.Delete(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d", rec.Code)
	}
}

func TestRestoreGroup_WithinWindow(t *testing.T) {
	h, q := newGroupHandler(t, 0)
	a := seedGroupUser(t, q, "a")
	_, g := createGroupAs(t, h, a, map[string]any{
		"name": "Phoenix" + uuid.NewString()[:8], "description": "x", "language": "en", "classification": "sfw",
	})
	if err := q.SoftDeleteGroup(context.Background(), g.ID); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	applyCtx := newChiCtx("id", g.ID.String())
	req := applyCtx(httptest.NewRequest(http.MethodPost, "/api/groups/"+g.ID.String()+"/restore", nil))
	req = withUser(req, a.ID.String(), a.Username, a.Email, a.Role)
	rec := httptest.NewRecorder()
	h.Restore(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d — body: %s", rec.Code, rec.Body.String())
	}
}

func TestRestoreGroup_OutsideWindowFails(t *testing.T) {
	h, q := newGroupHandler(t, 0)
	a := seedGroupUser(t, q, "a")
	_, g := createGroupAs(t, h, a, map[string]any{
		"name": "Stale" + uuid.NewString()[:8], "description": "x", "language": "en", "classification": "sfw",
	})
	// Force deleted_at to 31 days ago via direct UPDATE.
	if _, err := testutil.Pool().Exec(context.Background(),
		"UPDATE groups SET deleted_at = now() - interval '31 days' WHERE id = $1", g.ID); err != nil {
		t.Fatalf("force-stale delete: %v", err)
	}
	applyCtx := newChiCtx("id", g.ID.String())
	req := applyCtx(httptest.NewRequest(http.MethodPost, "/api/groups/"+g.ID.String()+"/restore", nil))
	req = withUser(req, a.ID.String(), a.Username, a.Email, a.Role)
	rec := httptest.NewRecorder()
	h.Restore(rec, req)
	if rec.Code != http.StatusGone {
		t.Fatalf("want 410 outside window, got %d — body: %s", rec.Code, rec.Body.String())
	}
}
