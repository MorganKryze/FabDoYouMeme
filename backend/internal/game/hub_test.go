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
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/clock"
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

// hubEnvOpts tunes room/hub timings for timing-sensitive tests. Zero-valued
// fields fall back to the newHubEnv defaults, which match the pre-refactor
// 60s/30s/300ms values.
type hubEnvOpts struct {
	roundDurationSeconds  int           // room config: round_duration_seconds (DB CHECK: 15..300)
	votingDurationSeconds int           // room config: voting_duration_seconds (DB CHECK: 10..120)
	roundCount            int           // room config: round_count (DB CHECK: 1..50)
	reconnectGrace        time.Duration // hub cfg: ReconnectGraceWindow
	packItemCount         int           // item versions seeded in the pack so runRounds can pick one
	clk                   clock.Clock   // optional clock.Fake — required for tests that need sub-DB-minimum round timings
	// skipPreCreate omits the manager.GetOrCreate() call in the helper so the
	// WSHandler must lazy-create the hub on first connect. P1.1 acceptance
	// test uses this to reproduce the production 404 bug.
	skipPreCreate bool
}

// newHubEnv is the default, happy-path test environment (60s rounds, 30s
// voting, 300ms grace). Most hub tests don't care about timings and use this
// directly so they stay insulated from churn in the options struct.
func newHubEnv(t *testing.T) *hubEnv {
	t.Helper()
	return newHubEnvWith(t, hubEnvOpts{})
}

// newHubEnvWith seeds a room in the database, starts its hub via the manager,
// and wraps the WS endpoint in an httptest.Server that authenticates
// via X-Test-User-ID / X-Test-Username headers (test-only — never in production).
// Cleanup is registered with t.Cleanup. Any zero-valued opts fall back to
// defaults that match the pre-options baseline.
func newHubEnvWith(t *testing.T, opts hubEnvOpts) *hubEnv {
	t.Helper()
	ctx := context.Background()
	q := db.New(testutil.Pool())
	slug := testutil.SeedName(t)
	if len(slug) > 20 {
		slug = slug[:20]
	}

	if opts.roundDurationSeconds == 0 {
		opts.roundDurationSeconds = 60
	}
	if opts.votingDurationSeconds == 0 {
		opts.votingDurationSeconds = 30
	}
	if opts.roundCount == 0 {
		opts.roundCount = 3
	}
	if opts.reconnectGrace == 0 {
		opts.reconnectGrace = 300 * time.Millisecond
	}

	host, err := q.CreateUser(ctx, db.CreateUserParams{
		Username:  slug + "_h",
		Email:     slug + "_h@test.com",
		Role:      "player",
		IsActive:  true,
		ConsentAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("newHubEnvWith: create host: %v", err)
	}

	gt, err := q.GetGameTypeBySlug(ctx, "meme-caption")
	if err != nil {
		t.Fatalf("newHubEnvWith: get game type: %v", err)
	}

	pack, err := q.CreatePack(ctx, db.CreatePackParams{
		Name:       slug + "_pk",
		Visibility: "private",
	})
	if err != nil {
		t.Fatalf("newHubEnvWith: create pack: %v", err)
	}

	// Seed item versions into the pack so runRounds can actually start a
	// round. GetRandomUnplayedItems JOINs on gi.current_version_id, so each
	// item needs both a version row AND a SetCurrentVersion follow-up —
	// otherwise the JOIN excludes the item and runRounds immediately
	// short-circuits to roundCtrlEndGame, masking the bug we're hunting.
	for i := 0; i < opts.packItemCount; i++ {
		item, err := q.CreateItem(ctx, db.CreateItemParams{
			PackID:         pack.ID,
			PayloadVersion: 1,
		})
		if err != nil {
			t.Fatalf("newHubEnvWith: create item %d: %v", i, err)
		}
		key := fmt.Sprintf("test/%s/%d.png", pack.ID, i)
		payload := json.RawMessage(fmt.Sprintf(`{"caption":"item %d"}`, i))
		version, err := q.CreateItemVersion(ctx, db.CreateItemVersionParams{
			ItemID:   item.ID,
			MediaKey: &key,
			Payload:  payload,
		})
		if err != nil {
			t.Fatalf("newHubEnvWith: create item version %d: %v", i, err)
		}
		if _, err := q.SetCurrentVersion(ctx, db.SetCurrentVersionParams{
			ID:               item.ID,
			CurrentVersionID: pgtype.UUID{Bytes: version.ID, Valid: true},
		}); err != nil {
			t.Fatalf("newHubEnvWith: set current version %d: %v", i, err)
		}
	}

	code := hubCode()
	roomConfig := fmt.Sprintf(
		`{"round_count":%d,"round_duration_seconds":%d,"voting_duration_seconds":%d}`,
		opts.roundCount, opts.roundDurationSeconds, opts.votingDurationSeconds,
	)
	room, err := q.CreateRoom(ctx, db.CreateRoomParams{
		Code:       code,
		GameTypeID: gt.ID,
		PackID:     pack.ID,
		HostID:     pgtype.UUID{Bytes: host.ID, Valid: true},
		Mode:       "multiplayer",
		Config:     json.RawMessage(roomConfig),
	})
	if err != nil {
		t.Fatalf("newHubEnvWith: create room: %v", err)
	}

	// Short grace default keeps reconnect tests fast without flakiness.
	cfg := &config.Config{
		ReconnectGraceWindow: opts.reconnectGrace,
		WSPingInterval:       60 * time.Second, // long to avoid ping noise
		WSReadLimitBytes:     4096,
	}
	registry := game.NewRegistry()
	registry.Register(memecaption.New())
	clk := opts.clk
	if clk == nil {
		clk = clock.Real{}
	}
	manager := game.NewManager(context.Background(), registry, q, cfg, slog.Default(), clk)
	if !opts.skipPreCreate {
		manager.GetOrCreate(ctx, room.Code, room.ID, gt.Slug, host.ID.String())
	}

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

// TestE2E_HostDisconnectFinishesGame is the P1.2 acceptance test from the
// 2026-04-10 review punch list. When the host disconnects mid-game and the
// reconnect grace expires, finishRoom must also cancel the runRounds
// goroutine. Previously, runRounds kept ticking into the hub after the room
// was "finished" and emitted ghost round-lifecycle broadcasts. We detect the
// leak by:
//
//  1. Driving the hub through a clock.Fake so we control when runRounds'
//     After(roundDuration) would fire. The DB CHECK constraint enforces a
//     15-second minimum on round_duration_seconds, which would make a real-
//     clock version of this test unusable for CI.
//  2. Advancing fake time past the round's deadline after the host has died
//     and game_ended has been broadcast. If runRounds is still alive it will
//     then send roundCtrlCloseSubmissions, which handleRoundCtrl broadcasts
//     as submissions_closed — we catch that on p2's socket and fail.
func TestE2E_HostDisconnectFinishesGame(t *testing.T) {
	// Anchor the fake clock to real now so its readings look plausible to
	// code that formats ends_at into RFC 3339 strings (the tests assert on
	// game_ended, not those, but keep the calendar sane anyway).
	fake := clock.NewFake(time.Now().UTC())

	env := newHubEnvWith(t, hubEnvOpts{
		roundCount:            3,
		roundDurationSeconds:  15, // DB CHECK minimum
		votingDurationSeconds: 10, // DB CHECK minimum
		reconnectGrace:        500 * time.Millisecond,
		packItemCount:         3,
		clk:                   fake,
	})

	hostConn := dial(t, env, env.hostID, "host")
	defer hostConn.Close()
	readUntilType(t, hostConn, "player_joined")

	p2ID := "00000000-0000-0000-0000-000000000055"
	p2Conn := dial(t, env, p2ID, "player2")
	defer p2Conn.Close()
	readUntilType(t, hostConn, "player_joined") // host sees p2 joining
	readUntilType(t, p2Conn, "player_joined")   // p2 sees own join

	sendMsg(t, hostConn, "start", nil)
	// game_started proves startGame ran; round_started proves runRounds has
	// definitely launched and is now blocked on fake.After(roundDuration).
	readUntilType(t, hostConn, "game_started")
	readUntilType(t, p2Conn, "game_started")
	readUntilType(t, p2Conn, "round_started")

	// Kill the host's connection. readPump → unregister → grace timer starts.
	hostConn.Close()

	// p2 sees "reconnecting" as soon as the server picks up the unregister.
	readUntilType(t, p2Conn, "reconnecting")

	// Advance fake time past the reconnect grace window so the AfterFunc
	// callback fires and sends to h.graceExpired. handleGraceExpired then
	// notices the host is gone, runs finishRoom, and broadcasts game_ended.
	fake.Advance(501 * time.Millisecond)

	readUntilType(t, p2Conn, "player_left")
	gameEnded := readUntilType(t, p2Conn, "game_ended")
	data, _ := gameEnded["data"].(map[string]any)
	if reason, _ := data["reason"].(string); reason != "host_disconnected" {
		t.Fatalf("want game_ended.reason=host_disconnected, got %v", data["reason"])
	}

	// Now the critical assertion. Advance fake time WAY past the remaining
	// round_duration so that runRounds' pending After(15s) is due. Post-fix
	// the goroutine is cancelled and exits before emitting any further
	// roundCtrl message. Pre-fix it wakes up and sends
	// roundCtrlCloseSubmissions, which handleRoundCtrl broadcasts as
	// submissions_closed to p2.
	fake.Advance(30 * time.Second)

	// Read p2 once with a single 500ms deadline — plenty for goroutine
	// scheduling. Gorilla websocket panics on a second ReadMessage after any
	// read error (including a deadline timeout), so we set one deadline and
	// drain in a single loop until we either see a ghost or hit the deadline.
	p2Conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	for {
		_, raw, err := p2Conn.ReadMessage()
		if err != nil {
			// Timeout or close: hub is quiet post-finishRoom — success.
			return
		}
		var m map[string]any
		if jerr := json.Unmarshal(raw, &m); jerr != nil {
			continue
		}
		switch m["type"] {
		case "submissions_closed", "round_started", "vote_results":
			t.Fatalf("ghost runRounds message %q after game_ended — runRounds was not cancelled", m["type"])
		}
	}
}

// TestE2E_WebSocketHappyPath is the P1.1 acceptance test from the 2026-04-10
// review punch list. In the production wiring nothing calls
// manager.GetOrCreate — rooms are created via POST /api/rooms and the hub is
// supposed to be lazy-created when the first WebSocket client connects. The
// current WSHandler uses manager.Get which returns (nil,false) for any
// freshly-seeded room, so every join flow 404s end-to-end and unit tests
// miss it because they call GetOrCreate directly.
//
// This test intentionally SKIPS the helper's pre-create call, dials the WS
// endpoint, and expects a successful join broadcast — reproducing the
// production bug and, post-fix, locking in lazy creation.
func TestE2E_WebSocketHappyPath(t *testing.T) {
	env := newHubEnvWith(t, hubEnvOpts{skipPreCreate: true})

	conn := dial(t, env, env.hostID, "host")
	defer conn.Close()

	// The hub must exist (lazy-created) AND have accepted the join broadcast.
	m := readUntilType(t, conn, "player_joined")
	data, _ := m["data"].(map[string]any)
	if data["user_id"] != env.hostID {
		t.Errorf("want player_joined.user_id=%s, got %v", env.hostID, data["user_id"])
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
