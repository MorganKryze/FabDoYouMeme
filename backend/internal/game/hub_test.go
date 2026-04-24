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
	memefreestyle "github.com/MorganKryze/FabDoYouMeme/backend/internal/game/types/meme_freestyle"
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
	hub       *game.Hub // nil when opts.skipPreCreate is true
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
	hostPaced             bool          // room config: host_paced — host manually advances rounds
	// skipPreCreate omits the manager.GetOrCreate() call in the helper so the
	// WSHandler must lazy-create the hub on first connect. P1.1 acceptance
	// test uses this to reproduce the production 404 bug.
	skipPreCreate bool
}

// defaultJokerCount mirrors the ValidateAndFill ceil(round_count/5) default so
// skip-feature tests can rely on the harness emitting a realistic, non-zero
// joker budget without explicitly opting in. Kept here (not in hub_skip_test.go)
// because newHubEnvWith writes the config row directly and would otherwise
// serialize zero values for the two new RoomConfig fields.
func defaultJokerCount(roundCount int) int {
	return (roundCount + 4) / 5
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
	// Add a nanosecond suffix so two tests whose slugified names share the
	// first 20 characters (e.g. TestHub_SkipSubmit_Exhausted_Rejected vs
	// TestHub_SkipSubmit_EarlyClose…) don't collide on the unique
	// `users.username` constraint when the package runs them back-to-back.
	slug = fmt.Sprintf("%s%d", slug, time.Now().UnixNano()%100000)

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
		Locale:    "en",
	})
	if err != nil {
		t.Fatalf("newHubEnvWith: create host: %v", err)
	}

	gt, err := q.GetGameTypeBySlug(ctx, "meme-freestyle")
	if err != nil {
		t.Fatalf("newHubEnvWith: get game type: %v", err)
	}

	pack, err := q.CreatePack(ctx, db.CreatePackParams{
		Name:       slug + "_pk",
		Visibility: "private",
		Language:   "en",
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
			Name:           fmt.Sprintf("item %d", i),
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
		`{"round_count":%d,"round_duration_seconds":%d,"voting_duration_seconds":%d,"host_paced":%t,"joker_count":%d,"allow_skip_vote":true}`,
		opts.roundCount, opts.roundDurationSeconds, opts.votingDurationSeconds, opts.hostPaced,
		defaultJokerCount(opts.roundCount),
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
	registry.Register(memefreestyle.New())
	clk := opts.clk
	if clk == nil {
		clk = clock.Real{}
	}
	manager := game.NewManager(context.Background(), registry, q, cfg, slog.Default(), clk)
	var createdHub *game.Hub
	if !opts.skipPreCreate {
		createdHub = manager.GetOrCreate(ctx, room.Code, room.ID, gt.Slug, host.ID.String())
	}

	// Passing [""] means the empty Origin (no header set by the test dialer)
	// normalizes to "" and matches — same semantic as the old empty-string
	// constructor argument before P2.6.
	wsHandler := api.NewWSHandler(manager, q, []string{""})
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
		hub:       createdHub,
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

// TestE2E_PackExhausted_GameEndedReason verifies that when the pack has no
// unplayed items, the game ends with reason "pack_exhausted".
func TestE2E_PackExhausted_GameEndedReason(t *testing.T) {
	// packItemCount defaults to 0: runRounds immediately fails to fetch items.
	env := newHubEnv(t)

	conn := dial(t, env, env.hostID, "host")
	defer conn.Close()
	readUntilType(t, conn, "player_joined")

	sendMsg(t, conn, "start", nil)
	readUntilType(t, conn, "game_started")

	m := readUntilType(t, conn, "game_ended")
	data, _ := m["data"].(map[string]any)
	if reason, _ := data["reason"].(string); reason != "pack_exhausted" {
		t.Fatalf("want game_ended.reason=pack_exhausted, got %v", data["reason"])
	}
}

// TestE2E_AllGrace_Ceiling_EndsGame verifies that when every player is in the
// reconnect grace window for longer than the ceiling, the game ends and the hub
// transitions to HubFinished. A late-connecting viewer sees state="finished".
func TestE2E_AllGrace_Ceiling_EndsGame(t *testing.T) {
	fake := clock.NewFake(time.Now().UTC())
	env := newHubEnvWith(t, hubEnvOpts{
		roundCount:            1,
		roundDurationSeconds:  15,
		votingDurationSeconds: 10,
		reconnectGrace:        1 * time.Second,
		packItemCount:         1,
		clk:                   fake,
	})

	hostConn := dial(t, env, env.hostID, "host")
	defer hostConn.Close()
	readUntilType(t, hostConn, "player_joined")

	p2ID := "00000000-0000-0000-0000-000000000061"
	p2Conn := dial(t, env, p2ID, "player2")
	defer p2Conn.Close()
	readUntilType(t, hostConn, "player_joined")
	readUntilType(t, p2Conn, "player_joined")

	sendMsg(t, hostConn, "start", nil)
	readUntilType(t, hostConn, "game_started")
	readUntilType(t, p2Conn, "game_started")
	readUntilType(t, hostConn, "round_started")
	readUntilType(t, p2Conn, "round_started")

	// Disconnect host; p2 confirms the hub processed it.
	hostConn.Close()
	readUntilType(t, p2Conn, "reconnecting")

	// Disconnect p2 — all players are now in the grace window.
	p2Conn.Close()
	// Allow the hub to process p2's unregister and queue the inGrace=true
	// signal so waitPhase records pauseStart before we advance the clock.
	time.Sleep(80 * time.Millisecond)

	// Advance past reconnectGrace (1s) ceiling. waitPhase detects all-in-grace
	// for ≥1s and ends the game with "all_players_disconnected".
	fake.Advance(2 * time.Second)

	// Allow hub goroutines to drain roundCtrlEndGame and call finishRoom.
	time.Sleep(200 * time.Millisecond)

	// A late viewer should see the room is finished.
	viewerConn := dial(t, env, env.hostID, "host")
	defer viewerConn.Close()
	m := readUntilType(t, viewerConn, "room_state")
	roomData, _ := m["data"].(map[string]any)
	if state := roomData["state"]; state != "finished" {
		t.Fatalf("want room state=finished after all-disconnected ceiling, got %v", state)
	}
}

// TestE2E_AllGrace_Reconnect_TimerResumes verifies that when all players
// disconnect and one reconnects within the grace ceiling, the round timer
// resumes (round_resumed broadcast) and the round eventually closes.
func TestE2E_AllGrace_Reconnect_TimerResumes(t *testing.T) {
	fake := clock.NewFake(time.Now().UTC())
	env := newHubEnvWith(t, hubEnvOpts{
		roundCount:            1,
		roundDurationSeconds:  15,
		votingDurationSeconds: 10,
		reconnectGrace:        2 * time.Second,
		packItemCount:         1,
		clk:                   fake,
	})

	hostConn := dial(t, env, env.hostID, "host")
	defer hostConn.Close()
	readUntilType(t, hostConn, "player_joined")

	p2ID := "00000000-0000-0000-0000-000000000062"
	p2Conn := dial(t, env, p2ID, "player2")
	defer p2Conn.Close()
	readUntilType(t, hostConn, "player_joined")
	readUntilType(t, p2Conn, "player_joined")

	sendMsg(t, hostConn, "start", nil)
	readUntilType(t, hostConn, "game_started")
	readUntilType(t, p2Conn, "game_started")
	readUntilType(t, hostConn, "round_started")
	readUntilType(t, p2Conn, "round_started")

	// Disconnect host; p2 confirms hub processed it.
	hostConn.Close()
	readUntilType(t, p2Conn, "reconnecting")

	// Disconnect p2 — all players now in grace; timer pauses.
	p2Conn.Close()
	time.Sleep(80 * time.Millisecond)

	// Reconnect host within the grace window. waitPhase receives inGrace=false
	// and resumes the timer, broadcasting round_resumed.
	newHostConn := dial(t, env, env.hostID, "host")
	defer newHostConn.Close()

	readUntilType(t, newHostConn, "round_resumed")

	// Advance past the remaining round duration (15s, none elapsed).
	// Zero submissions → voting skipped → vote_results broadcast directly.
	fake.Advance(16 * time.Second)
	readUntilType(t, newHostConn, "vote_results")
}

// TestE2E_ZeroSubmissions_SkipsVotingPhase verifies that when nobody submits
// during a round, the voting phase is skipped and vote_results is broadcast
// with empty data rather than the hub waiting for votes that never arrive.
func TestE2E_ZeroSubmissions_SkipsVotingPhase(t *testing.T) {
	fake := clock.NewFake(time.Now().UTC())
	env := newHubEnvWith(t, hubEnvOpts{
		roundCount:            1,
		roundDurationSeconds:  15,
		votingDurationSeconds: 10,
		reconnectGrace:        300 * time.Millisecond,
		packItemCount:         1,
		clk:                   fake,
	})

	hostConn := dial(t, env, env.hostID, "host")
	defer hostConn.Close()
	readUntilType(t, hostConn, "player_joined")

	p2ID := "00000000-0000-0000-0000-000000000063"
	p2Conn := dial(t, env, p2ID, "player2")
	defer p2Conn.Close()
	readUntilType(t, hostConn, "player_joined")
	readUntilType(t, p2Conn, "player_joined")

	sendMsg(t, hostConn, "start", nil)
	readUntilType(t, hostConn, "game_started")
	readUntilType(t, hostConn, "round_started")

	// Nobody submits. Advance past the submission window.
	fake.Advance(16 * time.Second)

	// vote_results should arrive without any prior submissions_closed
	// (voting phase is skipped when there are no submissions).
	m := readUntilType(t, hostConn, "vote_results")
	data, _ := m["data"].(map[string]any)
	// results may be nil (empty payload) or contain an empty submissions list.
	if results, ok := data["results"].(map[string]any); ok {
		if subs, ok := results["submissions"].([]any); ok && len(subs) != 0 {
			t.Errorf("want empty submissions in vote_results, got %v", subs)
		}
	}

	// Advance past the 10-second inter-round delay; game ends (1 round configured).
	fake.Advance(11 * time.Second)
	readUntilType(t, hostConn, "game_ended")
}

// TestHub_Reconnect_MidRound_SnapshotContainsPhase verifies that a player
// who refreshes during an active round receives enough state in the
// room_state snapshot to render the in-flight phase. Without phase, item,
// and ends_at, the frontend render tree has no matching branch and the
// page appears blank until the next round-lifecycle broadcast arrives.
func TestHub_Reconnect_MidRound_SnapshotContainsPhase(t *testing.T) {
	env := newHubEnvWith(t, hubEnvOpts{
		roundCount:            1,
		roundDurationSeconds:  60,
		votingDurationSeconds: 30,
		reconnectGrace:        2 * time.Second,
		packItemCount:         1,
	})

	hostConn := dial(t, env, env.hostID, "host")
	defer hostConn.Close()
	readUntilType(t, hostConn, "player_joined")

	p2ID := "00000000-0000-0000-0000-000000000071"
	p2Conn := dial(t, env, p2ID, "player2")
	defer p2Conn.Close()
	readUntilType(t, hostConn, "player_joined")
	readUntilType(t, p2Conn, "player_joined")

	sendMsg(t, hostConn, "start", nil)
	readUntilType(t, hostConn, "game_started")
	readUntilType(t, hostConn, "round_started")
	readUntilType(t, p2Conn, "round_started")

	// Force a reconnect within the grace window. readPump on the server will
	// observe the Close and call unregister, marking the player reconnecting.
	hostConn.Close()
	time.Sleep(80 * time.Millisecond)

	hostConn2 := dial(t, env, env.hostID, "host")
	defer hostConn2.Close()

	m := readUntilType(t, hostConn2, "room_state")
	data, _ := m["data"].(map[string]any)

	if phase := data["phase"]; phase != "submitting" {
		t.Fatalf("want phase=submitting in reconnect snapshot, got %v", phase)
	}
	if rn, _ := data["round_number"].(float64); rn != 1 {
		t.Errorf("want round_number=1, got %v", data["round_number"])
	}
	if _, ok := data["ends_at"].(string); !ok {
		t.Errorf("want ends_at string in reconnect snapshot, got %v", data["ends_at"])
	}
	if _, ok := data["duration_seconds"].(float64); !ok {
		t.Errorf("want duration_seconds number, got %v", data["duration_seconds"])
	}
	item, ok := data["item"].(map[string]any)
	if !ok {
		t.Fatalf("want item object in reconnect snapshot, got %v", data["item"])
	}
	if _, ok := item["payload"]; !ok {
		t.Errorf("want item.payload in reconnect snapshot")
	}
}
