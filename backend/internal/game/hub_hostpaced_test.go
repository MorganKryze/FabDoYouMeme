package game_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/MorganKryze/FabDoYouMeme/backend/internal/clock"
)

// TestE2E_ServerPaced_AutoAdvances verifies the default (non-host-paced) flow:
// after vote_results the server auto-advances to the next round (or ends the
// game) without waiting for a next_round message from the host.
func TestE2E_ServerPaced_AutoAdvances(t *testing.T) {
	fake := clock.NewFake(time.Now().UTC())
	env := newHubEnvWith(t, hubEnvOpts{
		roundCount:            1, // single round → game_ended after auto-advance
		roundDurationSeconds:  15,
		votingDurationSeconds: 10,
		packItemCount:         1,
		clk:                   fake,
		hostPaced:             false, // explicit, but matches the zero-value default
	})

	p2ID := "00000000-0000-0000-0000-0000000000a1"
	hostConn := dial(t, env, env.hostID, "host")
	defer hostConn.Close()
	p2Conn := dial(t, env, p2ID, "p2")
	defer p2Conn.Close()

	readUntilType(t, hostConn, "player_joined")
	readUntilType(t, p2Conn, "player_joined")

	sendMsg(t, hostConn, "start", nil)
	readUntilType(t, hostConn, "game_started")
	readUntilType(t, hostConn, "round_started")

	// Advance past the submission window → submissions_closed (or skipped voting).
	fake.Advance(16 * time.Second)
	// No submissions → voting skipped; vote_results arrives immediately.
	readUntilType(t, hostConn, "vote_results")

	// In server-paced mode, the inter-round pause is 10s — advance past it
	// so the auto-advance fires. Single round → game_ended.
	fake.Advance(11 * time.Second)
	m := readUntilType(t, hostConn, "game_ended")
	data, _ := m["data"].(map[string]any)
	if reason, _ := data["reason"].(string); reason != "game_complete" {
		t.Fatalf("want game_ended.reason=game_complete, got %v", data["reason"])
	}
}

// TestE2E_HostPaced_WaitsForNextRound verifies that in host-paced mode the
// server stays on the results phase until the host sends next_round, and only
// then advances to the next round (or ends the game).
func TestE2E_HostPaced_WaitsForNextRound(t *testing.T) {
	fake := clock.NewFake(time.Now().UTC())
	env := newHubEnvWith(t, hubEnvOpts{
		roundCount:            1,
		roundDurationSeconds:  15,
		votingDurationSeconds: 10,
		packItemCount:         1,
		clk:                   fake,
		hostPaced:             true,
	})

	p2ID := "00000000-0000-0000-0000-0000000000a2"
	hostConn := dial(t, env, env.hostID, "host")
	defer hostConn.Close()
	p2Conn := dial(t, env, p2ID, "p2")
	defer p2Conn.Close()

	readUntilType(t, hostConn, "player_joined")
	readUntilType(t, p2Conn, "player_joined")

	sendMsg(t, hostConn, "start", nil)
	readUntilType(t, hostConn, "game_started")
	readUntilType(t, hostConn, "round_started")

	// Submission window closes → vote_results.
	fake.Advance(16 * time.Second)
	readUntilType(t, hostConn, "vote_results")

	// Advance well past the server-paced 3-second window. In host-paced mode
	// the game must NOT advance on its own.
	fake.Advance(10 * time.Second)

	// Assert no game_ended arrives on p2Conn in the brief window after the time
	// advance. We deliberately use p2Conn here (not hostConn) because gorilla
	// websocket connections become non-recoverable after a deadline error, and
	// we need hostConn to stay healthy for the next_round / game_ended exchange.
	p2Conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	for {
		_, rawMsg, err := p2Conn.ReadMessage()
		if err != nil {
			break // deadline exceeded — nothing arrived, as expected
		}
		var m map[string]any
		if jsonErr := json.Unmarshal(rawMsg, &m); jsonErr == nil {
			if m["type"] == "game_ended" {
				t.Fatal("host-paced: server auto-advanced when it should wait for next_round")
			}
		}
	}
	// p2Conn is now unusable after the deadline error; hostConn is unaffected.

	// Host sends next_round → game_ended (single round).
	sendMsg(t, hostConn, "next_round", nil)
	m := readUntilType(t, hostConn, "game_ended")
	data, _ := m["data"].(map[string]any)
	if reason, _ := data["reason"].(string); reason != "game_complete" {
		t.Fatalf("want game_ended.reason=game_complete, got %v", data["reason"])
	}
}

// TestE2E_HostPaced_SafetyTimeout verifies that when host-paced mode is on
// but the host never sends next_round, the server auto-advances after the
// 5-minute safety ceiling.
func TestE2E_HostPaced_SafetyTimeout(t *testing.T) {
	fake := clock.NewFake(time.Now().UTC())
	env := newHubEnvWith(t, hubEnvOpts{
		roundCount:            1,
		roundDurationSeconds:  15,
		votingDurationSeconds: 10,
		packItemCount:         1,
		clk:                   fake,
		hostPaced:             true,
	})

	p2ID := "00000000-0000-0000-0000-0000000000a3"
	hostConn := dial(t, env, env.hostID, "host")
	defer hostConn.Close()
	p2Conn := dial(t, env, p2ID, "p2")
	defer p2Conn.Close()

	readUntilType(t, hostConn, "player_joined")
	readUntilType(t, p2Conn, "player_joined")

	sendMsg(t, hostConn, "start", nil)
	readUntilType(t, hostConn, "game_started")
	readUntilType(t, hostConn, "round_started")

	// Submission + voting windows close.
	fake.Advance(16 * time.Second)
	readUntilType(t, hostConn, "vote_results")

	// Host never sends next_round. Advance past the 5-minute safety ceiling.
	fake.Advance(6 * time.Minute)

	// The safety auto-advance fires → game_ended (single round).
	m := readUntilType(t, hostConn, "game_ended")
	data, _ := m["data"].(map[string]any)
	if reason, _ := data["reason"].(string); reason != "game_complete" {
		t.Fatalf("want game_ended.reason=game_complete, got %v", data["reason"])
	}
}
