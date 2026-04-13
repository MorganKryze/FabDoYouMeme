package api

import (
	"net/http"
	"testing"
)

// TestWS_CheckOriginNormalizesTrailingSlash is the P2.6 acceptance test for
// finding 5.C. Pre-fix the WS upgrader compared the raw Origin header
// against a single string, so a browser sending `https://meme.example.com/`
// would fail against a configured `https://meme.example.com`. After the fix,
// both sides are normalized (trim whitespace + a trailing slash) and the
// allowlist supports multiple origins via TRUSTED_WS_ORIGINS.
//
// This is a pure unit test against the exported constructor + the private
// checkOrigin method (same package) — no upgrader or server required.
func TestWS_CheckOriginNormalizesTrailingSlash(t *testing.T) {
	h := NewWSHandler(nil, nil, []string{
		"https://meme.example.com",           // primary
		"https://admin.meme.example.com/",    // secondary, with a stray slash
		"  https://mobile.meme.example.com ", // whitespace around it
	})

	cases := []struct {
		name   string
		origin string
		want   bool
	}{
		{"exact match", "https://meme.example.com", true},
		{"trailing slash on incoming", "https://meme.example.com/", true},
		{"trailing slash on allowed side", "https://admin.meme.example.com", true},
		{"both trailing slashes", "https://admin.meme.example.com/", true},
		{"whitespace-wrapped allowed entry normalized at load", "https://mobile.meme.example.com", true},
		{"different scheme rejected", "http://meme.example.com", false},
		{"different host rejected", "https://evil.example.com", false},
		{"subdomain not a wildcard match", "https://www.meme.example.com", false},
		{"empty origin rejected when not in allowlist", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodGet, "/api/ws/rooms/ABC", nil)
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			got := h.checkOrigin(req)
			if got != tc.want {
				t.Fatalf("checkOrigin(%q) = %v, want %v", tc.origin, got, tc.want)
			}
		})
	}
}

// TestWS_CheckOriginAllowsEmptyWhenConfigured guards the legacy behaviour
// used by game/hub_test.go: passing an explicit empty string in the
// allowlist means "accept connections with no Origin header" (non-browser
// clients, test dialers). Dropping this contract would break every existing
// hub integration test.
func TestWS_CheckOriginAllowsEmptyWhenConfigured(t *testing.T) {
	h := NewWSHandler(nil, nil, []string{""})
	req, _ := http.NewRequest(http.MethodGet, "/api/ws/rooms/ABC", nil)
	// No Origin header set — the normalized value must match the "" entry.
	if !h.checkOrigin(req) {
		t.Fatal("empty Origin should be accepted when [\"\"] is configured")
	}
}
