// backend/internal/api/danger_test.go
package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/api"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/testutil"
)

// postDangerRequest builds a POST /api/admin/danger/<action> request with
// a JSON {confirmation} body. The session user is injected separately via
// withUser so each test can choose a real vs fake UUID independently.
func postDangerRequest(path string, body map[string]string) *http.Request {
	buf, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func TestDangerWipeGameHistory_RejectsWrongConfirmation(t *testing.T) {
	pool := testutil.Pool()
	q := db.New(pool)

	// Seed a real admin so the context carries a parseable UUID — writeAuditAndRespond
	// may try to log even on 400s in future revisions, and we want this test robust.
	admin := seedAdmin(t, q)

	h := api.NewDangerHandler(pool, testutil.NewFakeStorage())

	req := postDangerRequest("/api/admin/danger/wipe-game-history", map[string]string{"confirmation": "nope"})
	req = withUser(req, admin.ID.String(), admin.Username, admin.Email, admin.Role)
	rec := httptest.NewRecorder()
	h.WipeGameHistory(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d — body: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["code"] != "invalid_confirmation" {
		t.Errorf("want code=invalid_confirmation, got %v", resp["code"])
	}
}

func TestDangerWipeGameHistory_Succeeds(t *testing.T) {
	ctx := context.Background()
	pool := testutil.Pool()
	q := db.New(pool)

	// Seed an admin whose ID we'll reuse as the acting user in the session context.
	slug := testutil.SeedName(t)
	user, err := q.CreateUser(ctx, db.CreateUserParams{
		Username:  slug,
		Email:     slug + "@test.com",
		Role:      "admin",
		IsActive:  true,
		ConsentAt: time.Now().UTC(),
		Locale:    "en",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM audit_logs WHERE admin_id = $1", user.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", user.ID)
	})

	h := api.NewDangerHandler(pool, testutil.NewFakeStorage())
	req := postDangerRequest("/api/admin/danger/wipe-game-history", map[string]string{"confirmation": "wipe game history"})
	req = withUser(req, user.ID.String(), user.Username, user.Email, user.Role)
	rec := httptest.NewRecorder()
	h.WipeGameHistory(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d — body: %s", rec.Code, rec.Body.String())
	}

	// Audit log entry must exist with action=danger_wipe_game_history.
	rows, err := q.ListRecentAuditLogs(ctx, 20)
	if err != nil {
		t.Fatalf("ListRecentAuditLogs: %v", err)
	}
	var found bool
	for _, row := range rows {
		if row.Action == "danger_wipe_game_history" && row.Resource == "system" {
			found = true
			break
		}
	}
	if !found {
		t.Error("audit log entry for danger_wipe_game_history not found")
	}
}

func TestDangerWipeInvites_Succeeds(t *testing.T) {
	ctx := context.Background()
	pool := testutil.Pool()
	q := db.New(pool)

	slug := testutil.SeedName(t)
	admin, err := q.CreateUser(ctx, db.CreateUserParams{
		Username:  slug,
		Email:     slug + "@test.com",
		Role:      "admin",
		IsActive:  true,
		ConsentAt: time.Now().UTC(),
		Locale:    "en",
	})
	if err != nil {
		t.Fatalf("create admin: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM audit_logs WHERE admin_id = $1", admin.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", admin.ID)
	})

	h := api.NewDangerHandler(pool, testutil.NewFakeStorage())
	req := postDangerRequest("/api/admin/danger/wipe-invites", map[string]string{"confirmation": "wipe invites"})
	req = withUser(req, admin.ID.String(), admin.Username, admin.Email, admin.Role)
	rec := httptest.NewRecorder()
	h.WipeInvites(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d — body: %s", rec.Code, rec.Body.String())
	}
}
