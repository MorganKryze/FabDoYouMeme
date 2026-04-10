package testutil

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// HTTPTest wraps an http.Handler (typically the main chi router) so tests can
// issue JSON requests without booting a TCP server. It also carries an
// optional cookie that is attached to every request — plug in a session cookie
// from MakeSession to simulate an authenticated user.
//
// Usage:
//
//	h := testutil.NewHTTPTest(t, router)
//	sess, cookie := testutil.MakeSession(t, user)
//	h = h.WithCookie(cookie)
//	rec := h.POST(t, "/api/rooms", map[string]any{"pack_id": packID})
//	h.AssertStatus(t, rec, http.StatusCreated)
type HTTPTest struct {
	Handler http.Handler
	Cookie  *http.Cookie
}

// NewHTTPTest returns an HTTPTest bound to the given handler.
func NewHTTPTest(_ *testing.T, handler http.Handler) *HTTPTest {
	return &HTTPTest{Handler: handler}
}

// WithCookie returns a copy of h that attaches cookie to every request. The
// original is not mutated so multiple personas can share one HTTPTest.
func (h *HTTPTest) WithCookie(cookie *http.Cookie) *HTTPTest {
	return &HTTPTest{Handler: h.Handler, Cookie: cookie}
}

// Do builds a request, attaches the current cookie, serves it against the
// handler, and returns the recorder for inspection.
func (h *HTTPTest) Do(t *testing.T, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != nil {
		switch v := body.(type) {
		case string:
			reader = bytes.NewBufferString(v)
		case []byte:
			reader = bytes.NewBuffer(v)
		default:
			b, err := json.Marshal(v)
			if err != nil {
				t.Fatalf("HTTPTest.Do: marshal body: %v", err)
			}
			reader = bytes.NewBuffer(b)
		}
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		if req.Header.Get("Content-Type") == "" {
			req.Header.Set("Content-Type", "application/json")
		}
	}
	if h.Cookie != nil {
		req.AddCookie(h.Cookie)
	}
	rec := httptest.NewRecorder()
	h.Handler.ServeHTTP(rec, req)
	return rec
}

// POST is a convenience wrapper around Do.
func (h *HTTPTest) POST(t *testing.T, path string, body any) *httptest.ResponseRecorder {
	return h.Do(t, http.MethodPost, path, body)
}

// GET is a convenience wrapper around Do.
func (h *HTTPTest) GET(t *testing.T, path string) *httptest.ResponseRecorder {
	return h.Do(t, http.MethodGet, path, nil)
}

// PATCH is a convenience wrapper around Do.
func (h *HTTPTest) PATCH(t *testing.T, path string, body any) *httptest.ResponseRecorder {
	return h.Do(t, http.MethodPatch, path, body)
}

// DELETE is a convenience wrapper around Do.
func (h *HTTPTest) DELETE(t *testing.T, path string) *httptest.ResponseRecorder {
	return h.Do(t, http.MethodDelete, path, nil)
}

// AssertStatus fails the test if rec.Code != want, including the body in the
// failure message to speed up debugging.
func (h *HTTPTest) AssertStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("want status %d, got %d — body: %s", want, rec.Code, rec.Body.String())
	}
}

// DecodeJSON unmarshals rec.Body into out. Fails on decode error.
func (h *HTTPTest) DecodeJSON(t *testing.T, rec *httptest.ResponseRecorder, out any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
		t.Fatalf("HTTPTest.DecodeJSON: %v — body: %s", err, rec.Body.String())
	}
}

// AssertError asserts that rec has the given status and a JSON body with
// {"code": wantCode}. Used for API error envelope assertions.
func (h *HTTPTest) AssertError(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantCode string) {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("AssertError: want status %d, got %d — body: %s", wantStatus, rec.Code, rec.Body.String())
	}
	var env map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("AssertError: decode body: %v — body: %s", err, rec.Body.String())
	}
	if code, _ := env["code"].(string); code != wantCode {
		t.Fatalf("AssertError: want code=%q, got %q (full=%v)", wantCode, code, env)
	}
}
