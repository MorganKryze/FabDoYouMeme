package game_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/clock"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/testutil"
)

// skipEnv is the standard skip-test setup: 2 rounds by default, 60s submit,
// 30s vote, enough items for all rounds. joker_count defaults to
// ceil(round_count/5) and allow_skip_vote defaults to true — both come from
// the shared newHubEnvWith config emitter.
func skipEnv(t *testing.T, opts hubEnvOpts) *hubEnv {
	t.Helper()
	if opts.roundCount == 0 {
		opts.roundCount = 2
	}
	if opts.packItemCount == 0 {
		opts.packItemCount = opts.roundCount + 1
	}
	if opts.clk == nil {
		opts.clk = clock.NewFake(time.Unix(1700000000, 0))
	}
	return newHubEnvWith(t, opts)
}

// forceJokerCount rewrites rooms.config.joker_count for the test room.
// Bypasses ValidateAndFill so tests can seed a state that would otherwise
// be unreachable via the API (e.g. explicit 0 when round_count > 0).
func forceJokerCount(t *testing.T, env *hubEnv, n int) {
	t.Helper()
	ctx := context.Background()
	pool := testutil.Pool()
	q := db.New(pool)
	room, err := q.GetRoomByCode(ctx, env.roomCode)
	if err != nil {
		t.Fatalf("forceJokerCount: get room: %v", err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(room.Config, &cfg); err != nil {
		t.Fatalf("forceJokerCount: unmarshal cfg: %v", err)
	}
	cfg["joker_count"] = n
	patched, _ := json.Marshal(cfg)
	if _, err := pool.Exec(ctx, `UPDATE rooms SET config = $1 WHERE id = $2`, patched, room.ID); err != nil {
		t.Fatalf("forceJokerCount: update: %v", err)
	}
}

func forceAllowSkipVote(t *testing.T, env *hubEnv, v bool) {
	t.Helper()
	ctx := context.Background()
	pool := testutil.Pool()
	q := db.New(pool)
	room, err := q.GetRoomByCode(ctx, env.roomCode)
	if err != nil {
		t.Fatalf("forceAllowSkipVote: get room: %v", err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(room.Config, &cfg); err != nil {
		t.Fatalf("forceAllowSkipVote: unmarshal cfg: %v", err)
	}
	cfg["allow_skip_vote"] = v
	patched, _ := json.Marshal(cfg)
	if _, err := pool.Exec(ctx, `UPDATE rooms SET config = $1 WHERE id = $2`, patched, room.ID); err != nil {
		t.Fatalf("forceAllowSkipVote: update: %v", err)
	}
}

func TestHub_SkipSubmit_DecrementsJokers_BroadcastsPlayerSkipped(t *testing.T) {
	env := skipEnv(t, hubEnvOpts{roundCount: 2})

	host := dial(t, env, env.hostID, "host")
	defer host.Close()
	readUntilType(t, host, "room_state")

	// Second player joins so a host-paced or early-close path doesn't trip.
	p2 := dial(t, env, "00000000-0000-0000-0000-000000000002", "p2")
	defer p2.Close()
	readUntilType(t, p2, "room_state")
	readUntilType(t, host, "player_joined")

	sendMsg(t, host, "start", nil)
	readUntilType(t, host, "game_started")
	readUntilType(t, p2, "game_started")
	readUntilType(t, host, "round_started")
	readUntilType(t, p2, "round_started")

	sendMsg(t, host, "skip_submit", nil)
	m := readUntilType(t, host, "player_skipped_submit")
	data, _ := m["data"].(map[string]any)
	if data["user_id"] != env.hostID {
		t.Fatalf("user_id: want %q, got %v", env.hostID, data["user_id"])
	}
	// joker_count default for 2 rounds is ceil(2/5)=1, so after one use
	// the host's remaining count is 0.
	if got, ok := data["jokers_remaining"].(float64); !ok || int(got) != 0 {
		t.Fatalf("jokers_remaining: want 0, got %v", data["jokers_remaining"])
	}
}

func TestHub_SkipSubmit_Exhausted_Rejected(t *testing.T) {
	env := skipEnv(t, hubEnvOpts{roundCount: 1}) // joker_count default = ceil(1/5)=1

	host := dial(t, env, env.hostID, "host")
	defer host.Close()
	readUntilType(t, host, "room_state")

	p2 := dial(t, env, "00000000-0000-0000-0000-000000000002", "p2")
	defer p2.Close()
	readUntilType(t, p2, "room_state")
	readUntilType(t, host, "player_joined")

	sendMsg(t, host, "start", nil)
	readUntilType(t, host, "round_started")
	readUntilType(t, p2, "round_started")

	// First skip consumes the only joker.
	sendMsg(t, host, "skip_submit", nil)
	readUntilType(t, host, "player_skipped_submit")

	// Second skip: host already in roundSkippedSubmits, so already_submitted
	// fires before jokers_exhausted. Spec orders duplicate-check above budget.
	sendMsg(t, host, "skip_submit", nil)
	m := readUntilType(t, host, "error")
	data, _ := m["data"].(map[string]any)
	if data["code"] != "already_submitted" {
		t.Fatalf("code: want already_submitted, got %v", data["code"])
	}
}

func TestHub_SkipSubmit_WhenDisabled_Rejected(t *testing.T) {
	// joker_count=0 explicitly disables the feature. Patch the DB row
	// directly since the default config emitter always produces a non-zero
	// joker budget for non-zero round counts.
	env := skipEnv(t, hubEnvOpts{roundCount: 2})
	forceJokerCount(t, env, 0)

	host := dial(t, env, env.hostID, "host")
	defer host.Close()
	readUntilType(t, host, "room_state")

	p2 := dial(t, env, "00000000-0000-0000-0000-000000000002", "p2")
	defer p2.Close()
	readUntilType(t, p2, "room_state")
	readUntilType(t, host, "player_joined")

	sendMsg(t, host, "start", nil)
	readUntilType(t, host, "round_started")
	readUntilType(t, p2, "round_started")

	sendMsg(t, host, "skip_submit", nil)
	m := readUntilType(t, host, "error")
	data, _ := m["data"].(map[string]any)
	if data["code"] != "skip_submit_disabled" {
		t.Fatalf("code: want skip_submit_disabled, got %v", data["code"])
	}
}

func TestHub_SkipVote_UnlimitedAndBroadcasts(t *testing.T) {
	env := skipEnv(t, hubEnvOpts{roundCount: 2})

	host := dial(t, env, env.hostID, "host")
	defer host.Close()
	readUntilType(t, host, "room_state")

	p2 := dial(t, env, "00000000-0000-0000-0000-000000000002", "p2")
	defer p2.Close()
	readUntilType(t, p2, "room_state")
	readUntilType(t, host, "player_joined")

	sendMsg(t, host, "start", nil)
	readUntilType(t, host, "round_started")
	readUntilType(t, p2, "round_started")

	// Both submit so submissions phase closes and we enter voting.
	sendMsg(t, host, "meme-freestyle:submit", map[string]string{"caption": "a"})
	sendMsg(t, p2, "meme-freestyle:submit", map[string]string{"caption": "b"})
	readUntilType(t, host, "submissions_closed")
	readUntilType(t, p2, "submissions_closed")

	sendMsg(t, host, "skip_vote", nil)
	m := readUntilType(t, host, "player_skipped_vote")
	data, _ := m["data"].(map[string]any)
	if data["user_id"] != env.hostID {
		t.Fatalf("user_id: want %q, got %v", env.hostID, data["user_id"])
	}
}

func TestHub_SkipVote_WhenDisabled_Rejected(t *testing.T) {
	env := skipEnv(t, hubEnvOpts{roundCount: 2})
	forceAllowSkipVote(t, env, false)

	host := dial(t, env, env.hostID, "host")
	defer host.Close()
	readUntilType(t, host, "room_state")

	p2 := dial(t, env, "00000000-0000-0000-0000-000000000002", "p2")
	defer p2.Close()
	readUntilType(t, p2, "room_state")
	readUntilType(t, host, "player_joined")

	sendMsg(t, host, "start", nil)
	readUntilType(t, host, "round_started")
	readUntilType(t, p2, "round_started")

	sendMsg(t, host, "meme-freestyle:submit", map[string]string{"caption": "a"})
	sendMsg(t, p2, "meme-freestyle:submit", map[string]string{"caption": "b"})
	readUntilType(t, host, "submissions_closed")

	sendMsg(t, host, "skip_vote", nil)
	m := readUntilType(t, host, "error")
	data, _ := m["data"].(map[string]any)
	if data["code"] != "skip_vote_disabled" {
		t.Fatalf("code: want skip_vote_disabled, got %v", data["code"])
	}
}

func TestHub_SkipVote_DuringSubmitPhase_Rejected(t *testing.T) {
	env := skipEnv(t, hubEnvOpts{roundCount: 2})

	host := dial(t, env, env.hostID, "host")
	defer host.Close()
	readUntilType(t, host, "room_state")

	p2 := dial(t, env, "00000000-0000-0000-0000-000000000002", "p2")
	defer p2.Close()
	readUntilType(t, p2, "room_state")
	readUntilType(t, host, "player_joined")

	sendMsg(t, host, "start", nil)
	readUntilType(t, host, "round_started")
	readUntilType(t, p2, "round_started")

	sendMsg(t, host, "skip_vote", nil)
	m := readUntilType(t, host, "error")
	data, _ := m["data"].(map[string]any)
	if data["code"] != "vote_closed" {
		t.Fatalf("code: want vote_closed, got %v", data["code"])
	}
}

func TestHub_Submit_AfterSkipSubmit_Rejected(t *testing.T) {
	env := skipEnv(t, hubEnvOpts{roundCount: 2})

	host := dial(t, env, env.hostID, "host")
	defer host.Close()
	readUntilType(t, host, "room_state")

	p2 := dial(t, env, "00000000-0000-0000-0000-000000000002", "p2")
	defer p2.Close()
	readUntilType(t, p2, "room_state")
	readUntilType(t, host, "player_joined")

	sendMsg(t, host, "start", nil)
	readUntilType(t, host, "round_started")
	readUntilType(t, p2, "round_started")

	sendMsg(t, host, "skip_submit", nil)
	readUntilType(t, host, "player_skipped_submit")

	sendMsg(t, host, "meme-freestyle:submit", map[string]string{"caption": "late"})
	m := readUntilType(t, host, "error")
	data, _ := m["data"].(map[string]any)
	if data["code"] != "already_submitted" {
		t.Fatalf("code: want already_submitted, got %v", data["code"])
	}
}

func TestHub_SkipSubmit_EarlyCloseMixedSubmitsAndSkips(t *testing.T) {
	env := skipEnv(t, hubEnvOpts{roundCount: 2, roundDurationSeconds: 300})

	host := dial(t, env, env.hostID, "host")
	defer host.Close()
	readUntilType(t, host, "room_state")

	p2 := dial(t, env, "00000000-0000-0000-0000-000000000002", "p2")
	defer p2.Close()
	readUntilType(t, p2, "room_state")
	readUntilType(t, host, "player_joined")

	sendMsg(t, host, "start", nil)
	readUntilType(t, host, "round_started")
	readUntilType(t, p2, "round_started")

	// host submits, p2 skips → the phase must close immediately without
	// waiting for the 300s timer.
	sendMsg(t, host, "meme-freestyle:submit", map[string]string{"caption": "a"})
	readUntilType(t, host, "submission_accepted")
	sendMsg(t, p2, "skip_submit", nil)
	readUntilType(t, p2, "player_skipped_submit")

	// Either client should see submissions_closed well before the 300s
	// deadline — a 3s read deadline is plenty.
	readUntilType(t, host, "submissions_closed")
}

func TestHub_Reconnect_CarriesMyJokersRemainingAndSkippedFlags(t *testing.T) {
	env := skipEnv(t, hubEnvOpts{roundCount: 2, reconnectGrace: 2 * time.Second})

	host := dial(t, env, env.hostID, "host")
	readUntilType(t, host, "room_state")

	p2 := dial(t, env, "00000000-0000-0000-0000-000000000002", "p2")
	defer p2.Close()
	readUntilType(t, p2, "room_state")
	readUntilType(t, host, "player_joined")

	sendMsg(t, host, "start", nil)
	readUntilType(t, host, "round_started")
	readUntilType(t, p2, "round_started")

	sendMsg(t, host, "skip_submit", nil)
	readUntilType(t, host, "player_skipped_submit")

	host.Close()

	// Reconnect within grace.
	host2 := dial(t, env, env.hostID, "host")
	defer host2.Close()
	m := readUntilType(t, host2, "room_state")
	data, _ := m["data"].(map[string]any)
	if got, ok := data["my_jokers_remaining"].(float64); !ok || int(got) != 0 {
		t.Fatalf("my_jokers_remaining: want 0, got %v", data["my_jokers_remaining"])
	}
	if got, _ := data["skipped_submit"].(bool); !got {
		t.Fatalf("skipped_submit: want true, got %v", data["skipped_submit"])
	}
}
