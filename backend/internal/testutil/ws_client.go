package testutil

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// WSTestClient is a thin wrapper around *websocket.Conn that adds ergonomics
// for handler tests: URL rewriting (http → ws), JSON envelope send/expect with
// deadlines, and a Close that swallows benign shutdown errors.
//
// Usage:
//
//	srv := httptest.NewServer(router)
//	defer srv.Close()
//	ws := testutil.DialWS(t, srv.URL+"/ws/rooms/"+roomID, cookie)
//	defer ws.Close()
//	ws.Send(t, "join", map[string]any{"role": "host"})
//	msg := ws.ExpectMessage(t, "player_joined", 2*time.Second)
type WSTestClient struct {
	Conn *websocket.Conn
}

// DialWS opens a WebSocket connection to url. If url starts with http:// or
// https:// (typical for httptest.Server), the scheme is rewritten to ws:// /
// wss://. If cookie is non-nil, it is attached to the upgrade request so the
// server's session middleware sees an authenticated user.
func DialWS(t *testing.T, url string, cookie *http.Cookie) *WSTestClient {
	t.Helper()
	wsURL := url
	switch {
	case strings.HasPrefix(wsURL, "http://"):
		wsURL = "ws://" + strings.TrimPrefix(wsURL, "http://")
	case strings.HasPrefix(wsURL, "https://"):
		wsURL = "wss://" + strings.TrimPrefix(wsURL, "https://")
	}

	hdr := http.Header{}
	if cookie != nil {
		hdr.Set("Cookie", cookie.String())
	}

	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, hdr)
	if err != nil {
		status := ""
		if resp != nil {
			status = resp.Status
		}
		t.Fatalf("DialWS(%s): %v (response=%s)", wsURL, err, status)
	}
	return &WSTestClient{Conn: conn}
}

// Send marshals {type, data} and writes it as a text frame.
func (c *WSTestClient) Send(t *testing.T, msgType string, data any) {
	t.Helper()
	payload := map[string]any{"type": msgType}
	if data != nil {
		payload["data"] = data
	}
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("WSTestClient.Send: marshal: %v", err)
	}
	if err := c.Conn.WriteMessage(websocket.TextMessage, b); err != nil {
		t.Fatalf("WSTestClient.Send: write: %v", err)
	}
}

// ExpectMessage reads frames until one of type msgType arrives or within
// elapses. Other message types are discarded. Returns the raw data field.
func (c *WSTestClient) ExpectMessage(t *testing.T, msgType string, within time.Duration) json.RawMessage {
	t.Helper()
	deadline := time.Now().Add(within)
	if err := c.Conn.SetReadDeadline(deadline); err != nil {
		t.Fatalf("WSTestClient.ExpectMessage: set deadline: %v", err)
	}
	for {
		if time.Now().After(deadline) {
			t.Fatalf("WSTestClient.ExpectMessage: timed out waiting for %q", msgType)
		}
		_, raw, err := c.Conn.ReadMessage()
		if err != nil {
			t.Fatalf("WSTestClient.ExpectMessage: read (waiting for %q): %v", msgType, err)
		}
		var env struct {
			Type string          `json:"type"`
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(raw, &env); err != nil {
			t.Fatalf("WSTestClient.ExpectMessage: unmarshal envelope: %v (body=%s)", err, string(raw))
		}
		if env.Type == msgType {
			return env.Data
		}
	}
}

// ExpectError reads until an "error" envelope arrives, asserts its code field
// matches wantCode, and returns the full data object. Fails on timeout or code
// mismatch.
func (c *WSTestClient) ExpectError(t *testing.T, wantCode string, within time.Duration) map[string]any {
	t.Helper()
	data := c.ExpectMessage(t, "error", within)
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatalf("WSTestClient.ExpectError: unmarshal data: %v", err)
	}
	if code, _ := obj["code"].(string); code != wantCode {
		t.Fatalf("WSTestClient.ExpectError: want code=%q, got %q (full=%v)", wantCode, code, obj)
	}
	return obj
}

// Close sends a close frame and then closes the underlying conn. Errors from
// an already-closed connection are ignored — tests don't care.
func (c *WSTestClient) Close() {
	if c == nil || c.Conn == nil {
		return
	}
	_ = c.Conn.WriteMessage(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	_ = c.Conn.Close()
}
