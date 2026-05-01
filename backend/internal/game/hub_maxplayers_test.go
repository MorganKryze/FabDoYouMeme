package game_test

import (
	"fmt"
	"testing"
	"time"
)

// TestHub_RejectJoinWhenFull is the P2.3 acceptance test. The meme-freestyle
// handler caps lobbies at 12 players (Handler.MaxPlayers). Before the fix
// handleRegister only checked hub state, not population, so the 13th player
// would silently join and bloat room_state. After the fix the 13th dial
// receives a single `error` frame with code=room_full and the connection is
// closed by the hub.
//
// We rely on the existing newHubEnv helper (which registers meme_freestyle)
// plus the test-only X-Test-User-ID header path to dial 13 distinct users
// without running auth. 12 joins must succeed, the 13th must error out.
func TestHub_RejectJoinWhenFull(t *testing.T) {
	env := newHubEnv(t)

	// Dial the host plus 11 more players: 12 successes.
	conns := make([]*dialCloser, 0, 12)
	t.Cleanup(func() {
		for _, c := range conns {
			c.Close()
		}
	})

	c := dial(t, env, env.hostID, "host")
	conns = append(conns, &dialCloser{c})
	readUntilType(t, c, "player_joined") // drain own join

	for i := 1; i < 12; i++ {
		uid := fmt.Sprintf("00000000-0000-0000-0000-0000000001%02d", i)
		c := dial(t, env, uid, fmt.Sprintf("p%d", i))
		conns = append(conns, &dialCloser{c})
	}

	// 13th join: must receive error{code:room_full} and then the hub closes.
	c13 := dial(t, env, "00000000-0000-0000-0000-00000000aa13", "p13")
	defer c13.Close()

	c13.SetReadDeadline(time.Now().Add(3 * time.Second))
	m := readMsg(t, c13)
	if m["type"] != "error" {
		t.Fatalf("want type=error, got %v", m["type"])
	}
	data, _ := m["data"].(map[string]any)
	if data["code"] != "room_full" {
		t.Fatalf("want code=room_full, got %v", data["code"])
	}
}

// TestHub_RejectJoinWhenPerRoomCapReached covers Solution B's hub side:
// a host who picked max_players=3 must see player 4 bounced even though
// the meme-freestyle manifest allows 12. The cap travels via
// rooms.config.max_players → manager → HubConfig.EffectiveMaxPlayers and
// is consulted in handleRegister before the manifest fallback.
func TestHub_RejectJoinWhenPerRoomCapReached(t *testing.T) {
	env := newHubEnvWith(t, hubEnvOpts{effectiveMaxPlayers: 3})

	conns := make([]*dialCloser, 0, 3)
	t.Cleanup(func() {
		for _, c := range conns {
			c.Close()
		}
	})

	c := dial(t, env, env.hostID, "host")
	conns = append(conns, &dialCloser{c})
	readUntilType(t, c, "player_joined")

	for i := 1; i < 3; i++ {
		uid := fmt.Sprintf("00000000-0000-0000-0000-0000000003%02d", i)
		c := dial(t, env, uid, fmt.Sprintf("p%d", i))
		conns = append(conns, &dialCloser{c})
	}

	// 4th join: must hit room_full from the per-room cap, not the manifest.
	c4 := dial(t, env, "00000000-0000-0000-0000-00000000bb04", "p4")
	defer c4.Close()

	c4.SetReadDeadline(time.Now().Add(3 * time.Second))
	m := readMsg(t, c4)
	if m["type"] != "error" {
		t.Fatalf("want type=error, got %v", m["type"])
	}
	data, _ := m["data"].(map[string]any)
	if data["code"] != "room_full" {
		t.Fatalf("want code=room_full, got %v", data["code"])
	}
}

// dialCloser wraps *websocket.Conn for defer-friendly batch cleanup.
type dialCloser struct{ inner interface{ Close() error } }

func (d *dialCloser) Close() { _ = d.inner.Close() }
