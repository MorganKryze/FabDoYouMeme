// backend/internal/api/ws_ban_test.go
package api_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/api"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/auth"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/clock"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/config"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/game"
	memecaption "github.com/MorganKryze/FabDoYouMeme/backend/internal/game/types/meme_caption"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/testutil"
)

// newWSHandlerForTest builds a WSHandler backed by the real test pool so the
// ban-gate queries (IsUserBannedFromRoom / IsGuestBannedFromRoom) run against
// actual rows. The "" allowlist entry matches requests that carry no Origin
// header, which is exactly what httptest.NewRequest produces.
func newWSHandlerForTest(t *testing.T) (*api.WSHandler, *db.Queries) {
	t.Helper()
	pool := testutil.Pool()
	q := db.New(pool)
	registry := game.NewRegistry()
	registry.Register(memecaption.New())
	cfg := &config.Config{}
	manager := game.NewManager(context.Background(), registry, q, cfg, slog.Default(), clock.Real{})
	return api.NewWSHandler(manager, q, []string{""}), q
}

// TestWS_BannedUser_Rejected — a registered user on the ban list cannot
// complete the WS handshake; we return 409 banned_from_room before upgrade.
func TestWS_BannedUser_Rejected(t *testing.T) {
	h, q := newWSHandlerForTest(t)

	host := seedRoomUser(t, q)
	victim := extraRoomUser(t, q, "_banned_user", "player")
	room := roomInState(t, q, host, "lobby")
	if err := q.CreateUserRoomBan(context.Background(), db.CreateUserRoomBanParams{
		RoomID:   room.ID,
		UserID:   pgtype.UUID{Bytes: victim.ID, Valid: true},
		BannedBy: pgtype.UUID{Bytes: host.ID, Valid: true},
	}); err != nil {
		t.Fatalf("seed ban: %v", err)
	}

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("code", room.Code)
	req := httptest.NewRequest(http.MethodGet, "/api/ws/rooms/"+room.Code, nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = withUser(req, victim.ID.String(), victim.Username, victim.Email, victim.Role)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d — body: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "banned_from_room") {
		t.Fatalf("want banned_from_room in body, got %q", rec.Body.String())
	}
}

// TestWS_BannedGuest_Rejected — a guest whose token resolves to a banned
// guest_player_id cannot complete the WS handshake; we return 409
// banned_from_room before upgrade.
func TestWS_BannedGuest_Rejected(t *testing.T) {
	h, q := newWSHandlerForTest(t)

	host := seedRoomUser(t, q)
	room := roomInState(t, q, host, "lobby")

	rawToken, err := auth.GenerateRawToken()
	if err != nil {
		t.Fatalf("GenerateRawToken: %v", err)
	}
	gp, err := q.CreateGuestPlayer(context.Background(), db.CreateGuestPlayerParams{
		RoomID:      room.ID,
		DisplayName: "Gwen",
		TokenHash:   auth.HashToken(rawToken),
	})
	if err != nil {
		t.Fatalf("CreateGuestPlayer: %v", err)
	}
	if err := q.CreateGuestRoomBan(context.Background(), db.CreateGuestRoomBanParams{
		RoomID:        room.ID,
		GuestPlayerID: pgtype.UUID{Bytes: gp.ID, Valid: true},
		BannedBy:      pgtype.UUID{Bytes: host.ID, Valid: true},
	}); err != nil {
		t.Fatalf("seed guest ban: %v", err)
	}

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("code", room.Code)
	req := httptest.NewRequest(http.MethodGet, "/api/ws/rooms/"+room.Code+"?guest_token="+rawToken, nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d — body: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "banned_from_room") {
		t.Fatalf("want banned_from_room in body, got %q", rec.Body.String())
	}
}
