package game_test

import (
	"context"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// TestHub_EndRoom_BroadcastsRoomClosedAndDisconnects asserts the contract
// for external room termination: every connected player receives a
// room_closed message with the supplied reason, and every connection is
// closed after the broadcast. Unlike finishRoom (game_ended), there is no
// rematch window — a killed room is terminal.
func TestHub_EndRoom_BroadcastsRoomClosedAndDisconnects(t *testing.T) {
	env := newHubEnv(t)

	hostConn := dial(t, env, env.hostID, "host")
	defer hostConn.Close()

	p1ID := "00000000-0000-0000-0000-000000000077"
	p1Conn := dial(t, env, p1ID, "p1")
	defer p1Conn.Close()

	// Drain the register traffic so the next reads only see the kill.
	readUntilType(t, hostConn, "player_joined")
	readUntilType(t, p1Conn, "player_joined")

	if err := env.hub.EndRoom(context.Background(), "ended_by_host"); err != nil {
		t.Fatalf("EndRoom returned error: %v", err)
	}

	// Both connections must receive room_closed with the expected reason.
	for _, c := range []*websocket.Conn{hostConn, p1Conn} {
		msg := readUntilType(t, c, "room_closed")
		data, ok := msg["data"].(map[string]any)
		if !ok {
			t.Fatalf("room_closed missing data: %v", msg)
		}
		if data["reason"] != "ended_by_host" {
			t.Fatalf("room_closed reason = %v, want ended_by_host", data["reason"])
		}
	}

	// Both connections must then be closed by the server. ReadMessage will
	// error (either CloseMessage or a generic read error). Use a short
	// deadline to avoid hanging if EndRoom forgot to close.
	for name, c := range map[string]*websocket.Conn{"host": hostConn, "p1": p1Conn} {
		c.SetReadDeadline(time.Now().Add(2 * time.Second))
		if _, _, err := c.ReadMessage(); err == nil {
			t.Fatalf("%s: expected connection to be closed after EndRoom, got nil error", name)
		}
	}
}

// TestHub_EndRoom_IdempotentSecondCall asserts that calling EndRoom twice
// in quick succession does not panic and is a no-op the second time.
// The first call drains the players map (via the unregister path triggered
// by each Close), so the second call simply has no one to broadcast to.
func TestHub_EndRoom_IdempotentSecondCall(t *testing.T) {
	env := newHubEnv(t)

	hostConn := dial(t, env, env.hostID, "host")
	defer hostConn.Close()
	readUntilType(t, hostConn, "player_joined")

	if err := env.hub.EndRoom(context.Background(), "ended_by_host"); err != nil {
		t.Fatalf("first EndRoom: %v", err)
	}
	// Give the Run loop time to drain the first message.
	time.Sleep(50 * time.Millisecond)

	if err := env.hub.EndRoom(context.Background(), "ended_by_host"); err != nil {
		t.Fatalf("second EndRoom: %v", err)
	}
}
