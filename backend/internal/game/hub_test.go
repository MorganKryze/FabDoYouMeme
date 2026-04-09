package game_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/api"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/config"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/game"
	memecaption "github.com/MorganKryze/FabDoYouMeme/backend/internal/game/types/meme_caption"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/middleware"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/testutil"
)

// hubCodeCounter ensures unique room codes within a test run.
var hubCodeCounter atomic.Int64

// hubCode returns a unique 4-character room code.
func hubCode() string {
	n := hubCodeCounter.Add(1)
	return fmt.Sprintf("H%03d", n)
}

// hubEnv groups everything a hub test needs.
type hubEnv struct {
	serverURL string
	roomCode  string
	hostID    string
}

// newHubEnv seeds a room in the database, starts its hub via the manager,
// and wraps the WS endpoint in an httptest.Server that authenticates
// via X-Test-User-ID / X-Test-Username headers (test-only — never in production).
// Cleanup is registered with t.Cleanup.
func newHubEnv(t *testing.T) *hubEnv {
	t.Helper()
	ctx := context.Background()
	q := db.New(testutil.Pool())
	slug := testutil.SeedName(t)
	if len(slug) > 20 {
		slug = slug[:20]
	}

	host, err := q.CreateUser(ctx, db.CreateUserParams{
		Username:  slug + "_h",
		Email:     slug + "_h@test.com",
		Role:      "player",
		IsActive:  true,
		ConsentAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("newHubEnv: create host: %v", err)
	}

	gt, err := q.GetGameTypeBySlug(ctx, "meme-caption")
	if err != nil {
		t.Fatalf("newHubEnv: get game type: %v", err)
	}

	pack, err := q.CreatePack(ctx, db.CreatePackParams{
		Name:       slug + "_pk",
		Visibility: "private",
	})
	if err != nil {
		t.Fatalf("newHubEnv: create pack: %v", err)
	}

	code := hubCode()
	room, err := q.CreateRoom(ctx, db.CreateRoomParams{
		Code:       code,
		GameTypeID: gt.ID,
		PackID:     pack.ID,
		HostID:     pgtype.UUID{Bytes: host.ID, Valid: true},
		Mode:       "multiplayer",
		Config:     json.RawMessage(`{"round_count":3,"round_duration_seconds":60,"voting_duration_seconds":30}`),
	})
	if err != nil {
		t.Fatalf("newHubEnv: create room: %v", err)
	}

	// Short durations make timing-sensitive tests fast without flakiness.
	cfg := &config.Config{
		ReconnectGraceWindow: 300 * time.Millisecond,
		WSPingInterval:       60 * time.Second, // long to avoid ping noise
		WSReadLimitBytes:     4096,
	}
	registry := game.NewRegistry()
	registry.Register(memecaption.New())
	manager := game.NewManager(registry, q, cfg, slog.Default())
	manager.GetOrCreate(ctx, room.Code, room.ID, gt.Slug, host.ID.String())

	wsHandler := api.NewWSHandler(manager, "") // empty allowedOrigin: test uses no Origin header
	r := chi.NewRouter()
	r.Get("/ws/{code}", func(w http.ResponseWriter, req *http.Request) {
		userID := req.Header.Get("X-Test-User-ID")
		username := req.Header.Get("X-Test-Username")
		if userID != "" {
			req = req.WithContext(context.WithValue(req.Context(),
				middleware.SessionUserContextKey,
				middleware.SessionUser{UserID: userID, Username: username, Role: "player"},
			))
		}
		wsHandler.ServeHTTP(w, req)
	})

	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)

	return &hubEnv{
		serverURL: ts.URL,
		roomCode:  room.Code,
		hostID:    host.ID.String(),
	}
}

// dial opens a WebSocket connection to the hub using test identity headers.
func dial(t *testing.T, env *hubEnv, userID, username string) *websocket.Conn {
	t.Helper()
	u := "ws" + strings.TrimPrefix(env.serverURL, "http") + "/ws/" + env.roomCode
	h := http.Header{}
	h.Set("X-Test-User-ID", userID)
	h.Set("X-Test-Username", username)
	conn, _, err := websocket.DefaultDialer.Dial(u, h)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	return conn
}

// readMsg reads the next WebSocket message with a short deadline.
func readMsg(t *testing.T, conn *websocket.Conn) map[string]any {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, b, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("readMsg: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("readMsg unmarshal: %v", err)
	}
	return m
}

// readUntilType reads messages, discarding any that are not of wantType,
// and returns the first matching message. Fails if none is found in 20 reads.
func readUntilType(t *testing.T, conn *websocket.Conn, wantType string) map[string]any {
	t.Helper()
	for i := 0; i < 20; i++ {
		m := readMsg(t, conn)
		if m["type"] == wantType {
			return m
		}
	}
	t.Fatalf("readUntilType: no %q message received in 20 reads", wantType)
	return nil
}

// sendMsg serialises and sends a WebSocket message.
func sendMsg(t *testing.T, conn *websocket.Conn, msgType string, data any) {
	t.Helper()
	payload := map[string]any{"type": msgType}
	if data != nil {
		payload["data"] = data
	}
	b, _ := json.Marshal(payload)
	if err := conn.WriteMessage(websocket.TextMessage, b); err != nil {
		t.Fatalf("sendMsg: %v", err)
	}
}

// ─── Tests ───────────────────────────────────────────────────────────────────

func TestHub_Join_BroadcastsPlayerJoined(t *testing.T) {
	env := newHubEnv(t)

	c1 := dial(t, env, env.hostID, "host")
	defer c1.Close()

	// c1 should receive its own player_joined broadcast.
	m := readUntilType(t, c1, "player_joined")
	data, _ := m["data"].(map[string]any)
	if data["user_id"] != env.hostID {
		t.Errorf("want player_joined.user_id=%s, got %v", env.hostID, data["user_id"])
	}

	// A second player joins; c1 should see their player_joined broadcast.
	p2ID := "00000000-0000-0000-0000-000000000099"
	c2 := dial(t, env, p2ID, "player2")
	defer c2.Close()

	m2 := readUntilType(t, c1, "player_joined")
	data2, _ := m2["data"].(map[string]any)
	if data2["user_id"] != p2ID {
		t.Errorf("want player_joined.user_id=%s for p2, got %v", p2ID, data2["user_id"])
	}
}

func TestHub_Start_NonHost_ReturnsError(t *testing.T) {
	env := newHubEnv(t)

	nonHostID := "00000000-0000-0000-0000-000000000088"
	conn := dial(t, env, nonHostID, "nothost")
	defer conn.Close()

	readUntilType(t, conn, "player_joined") // drain own join

	sendMsg(t, conn, "start", nil)

	m := readUntilType(t, conn, "error")
	data, _ := m["data"].(map[string]any)
	if data["code"] != "not_host" {
		t.Errorf("want code=not_host, got %v", data["code"])
	}
}

func TestHub_Start_Host_BroadcastsGameStarted(t *testing.T) {
	env := newHubEnv(t)

	conn := dial(t, env, env.hostID, "host")
	defer conn.Close()

	readUntilType(t, conn, "player_joined") // drain own join

	sendMsg(t, conn, "start", nil)

	m := readUntilType(t, conn, "game_started")
	data, _ := m["data"].(map[string]any)
	count, _ := data["player_count"].(float64)
	if count < 1 {
		t.Errorf("want game_started.player_count >= 1, got %v", data["player_count"])
	}
}

func TestHub_Reconnect_WithinGrace_ReceivesRoomState(t *testing.T) {
	env := newHubEnv(t)

	c1 := dial(t, env, env.hostID, "host")
	readUntilType(t, c1, "player_joined") // drain own join

	// Disconnect — readPump will send to h.unregister, starting the grace timer.
	c1.Close()

	// Give the hub time to process the unregister (well inside the 300ms grace).
	time.Sleep(80 * time.Millisecond)

	// Reconnect with the same userID — hub should recognise the reconnect.
	c1b := dial(t, env, env.hostID, "host")
	defer c1b.Close()

	// Reconnecting player receives a room_state snapshot first.
	m := readUntilType(t, c1b, "room_state")
	data, _ := m["data"].(map[string]any)
	if data["state"] == nil {
		t.Error("expected room_state.data.state to be present in reconnect snapshot")
	}
}

func TestHub_Disconnect_GraceExpired_BroadcastsPlayerLeft(t *testing.T) {
	env := newHubEnv(t)

	// Host connects.
	c1 := dial(t, env, env.hostID, "host")
	defer c1.Close()
	readUntilType(t, c1, "player_joined")

	// Second player joins.
	p2ID := "00000000-0000-0000-0000-000000000077"
	c2 := dial(t, env, p2ID, "player2")
	readUntilType(t, c1, "player_joined") // c1 sees p2's join broadcast
	readUntilType(t, c2, "player_joined") // drain c2's own join

	// Disconnect p2.
	c2.Close()

	// c1 sees the immediate "reconnecting" broadcast while grace window is open.
	readUntilType(t, c1, "reconnecting")

	// Wait for the grace window to expire (300ms) plus margin.
	time.Sleep(450 * time.Millisecond)

	// c1 should now receive player_left.
	m := readUntilType(t, c1, "player_left")
	data, _ := m["data"].(map[string]any)
	if data["user_id"] != p2ID {
		t.Errorf("want player_left.user_id=%s, got %v", p2ID, data["user_id"])
	}
}
