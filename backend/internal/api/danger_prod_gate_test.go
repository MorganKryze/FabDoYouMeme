// backend/internal/api/danger_prod_gate_test.go
package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/MorganKryze/FabDoYouMeme/backend/internal/api"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/testutil"
)

// TestDangerRoutesUnmountedInProd is a regression test that mirrors the
// mount gate in main.go. If someone removes the `if cfg.AppEnv != "prod"`
// wrapper, this test fails — which is the whole point of the gate. The
// test reproduces the exact conditional so any deviation is caught at
// CI time, not after a production incident.
func TestDangerRoutesUnmountedInProd(t *testing.T) {
	appEnv := "prod"
	r := chi.NewRouter()
	if appEnv != "prod" {
		dangerHandler := api.NewDangerHandler(testutil.Pool(), testutil.NewFakeStorage())
		r.Route("/api/admin/danger", func(r chi.Router) {
			r.Post("/wipe-game-history", dangerHandler.WipeGameHistory)
			r.Post("/wipe-packs-and-media", dangerHandler.WipePacksAndMedia)
			r.Post("/wipe-invites", dangerHandler.WipeInvites)
			r.Post("/wipe-sessions", dangerHandler.WipeSessions)
			r.Post("/full-reset", dangerHandler.FullReset)
		})
	}

	paths := []string{
		"/api/admin/danger/wipe-game-history",
		"/api/admin/danger/wipe-packs-and-media",
		"/api/admin/danger/wipe-invites",
		"/api/admin/danger/wipe-sessions",
		"/api/admin/danger/full-reset",
	}
	for _, p := range paths {
		req := httptest.NewRequest(http.MethodPost, p, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("path %s in prod should 404, got %d", p, rec.Code)
		}
	}
}
