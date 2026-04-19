package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/MorganKryze/FabDoYouMeme/backend/internal/api"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/game"
	memefreestyle "github.com/MorganKryze/FabDoYouMeme/backend/internal/game/types/meme_freestyle"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/testutil"
)

func newGameTypeHandler() *api.GameTypeHTTPHandler {
	registry := game.NewRegistry()
	registry.Register(memefreestyle.New())
	return api.NewGameTypeHandler(testutil.Pool(), registry)
}

func TestListGameTypes_ReturnsMemeCaption(t *testing.T) {
	h := newGameTypeHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/game-types", nil)
	req = withUser(req, "00000000-0000-0000-0000-000000000002", "testuser", "t@t.com", "player")
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d — body: %s", rec.Code, rec.Body.String())
	}
	var types []map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&types); err != nil {
		t.Fatalf("decode: %v", err)
	}
	var found bool
	for _, gt := range types {
		if gt["slug"] == "meme-freestyle" {
			found = true
		}
	}
	if !found {
		t.Error("expected meme-freestyle in game types list")
	}
}

func TestListGameTypes_Unauthenticated(t *testing.T) {
	h := newGameTypeHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/game-types", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rec.Code)
	}
}

func TestGetGameType_BySlug(t *testing.T) {
	h := newGameTypeHandler()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("slug", "meme-freestyle")
	req := httptest.NewRequest(http.MethodGet, "/api/game-types/meme-freestyle", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = withUser(req, "00000000-0000-0000-0000-000000000002", "testuser", "t@t.com", "player")
	rec := httptest.NewRecorder()
	h.GetBySlug(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d — body: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["slug"] != "meme-freestyle" {
		t.Errorf("want slug=meme-freestyle, got %v", resp["slug"])
	}
	if _, ok := resp["required_packs"]; !ok {
		t.Error("expected required_packs in response")
	}
}

func TestGetGameType_NotFound(t *testing.T) {
	h := newGameTypeHandler()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("slug", "no-such-game")
	req := httptest.NewRequest(http.MethodGet, "/api/game-types/no-such-game", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = withUser(req, "00000000-0000-0000-0000-000000000002", "testuser", "t@t.com", "player")
	rec := httptest.NewRecorder()
	h.GetBySlug(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", rec.Code)
	}
	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if !strings.Contains(resp["code"], "not_found") {
		t.Errorf("want not_found code, got %s", resp["code"])
	}
}
