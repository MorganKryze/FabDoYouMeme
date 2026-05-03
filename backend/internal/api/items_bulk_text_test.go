package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/api"
)

func bulkTextReq(t *testing.T, packID string, items []map[string]string, admin db.User) *http.Request {
	t.Helper()
	body, err := json.Marshal(map[string]any{"items": items})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/packs/"+packID+"/items/bulk-text", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withUser(req, admin.ID.String(), admin.Username, admin.Email, admin.Role)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", packID)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

// TestBulkCreateTextItems_AllSucceed: three rows in, three rows out, every
// row carries a current_version_id and a {text} payload.
func TestBulkCreateTextItems_AllSucceed(t *testing.T) {
	h, q := newBulkHandler(t, newMemStorage())
	admin := seedAdmin(t, q)
	pack := seedPersonalPack(t, q, pgtype.UUID{Bytes: admin.ID, Valid: true})

	rec := httptest.NewRecorder()
	h.BulkCreateTextItems(rec, bulkTextReq(t, pack.ID.String(), []map[string]string{
		{"name": "alpha", "text": "first"},
		{"name": "beta", "text": "second"},
		{"name": "gamma", "text": `quotes " unicode é ok`},
	}, admin))

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d — body: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Results []struct {
			OK     bool   `json:"ok"`
			Reason string `json:"reason"`
			Item   struct {
				Name           string `json:"name"`
				PayloadVersion int    `json:"payload_version"`
				Payload        any    `json:"payload"`
			} `json:"item"`
		} `json:"results"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Results) != 3 {
		t.Fatalf("want 3 results, got %d", len(resp.Results))
	}
	for i, r := range resp.Results {
		if !r.OK {
			t.Errorf("result[%d]: want ok, got reason=%q", i, r.Reason)
		}
		if r.Item.PayloadVersion != 2 {
			t.Errorf("result[%d]: want payload_version=2, got %d", i, r.Item.PayloadVersion)
		}
		p, ok := r.Item.Payload.(map[string]any)
		if !ok || p["text"] == nil {
			t.Errorf("result[%d]: payload missing text field, got %#v", i, r.Item.Payload)
		}
	}
	rows, err := q.ListItemsForPack(context.Background(), db.ListItemsForPackParams{PackID: pack.ID, Lim: 50, Off: 0})
	if err != nil {
		t.Fatalf("list items: %v", err)
	}
	if len(rows) != 3 {
		t.Errorf("want 3 DB rows, got %d", len(rows))
	}
	for _, row := range rows {
		if !row.CurrentVersionID.Valid {
			t.Errorf("row %s has NULL current_version_id (orphan)", row.ID)
		}
		if row.PayloadVersion != 2 {
			t.Errorf("row %s payload_version=%d, want 2", row.ID, row.PayloadVersion)
		}
	}
}

// Per-row validation: empty name/text → ok=false, no row inserted, sibling
// rows still succeed independently.
func TestBulkCreateTextItems_PerRowValidation(t *testing.T) {
	h, q := newBulkHandler(t, newMemStorage())
	admin := seedAdmin(t, q)
	pack := seedPersonalPack(t, q, pgtype.UUID{Bytes: admin.ID, Valid: true})

	rec := httptest.NewRecorder()
	h.BulkCreateTextItems(rec, bulkTextReq(t, pack.ID.String(), []map[string]string{
		{"name": "ok-row", "text": "valid"},
		{"name": "", "text": "missing name"},
		{"name": "blank-text", "text": "   "},
		{"name": "too-long", "text": strings.Repeat("a", api.MaxBulkTextLength+1)},
	}, admin))

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200 (per-row failures don't fail the request), got %d", rec.Code)
	}
	var resp struct {
		Results []struct {
			OK     bool   `json:"ok"`
			Code   string `json:"code"`
			Reason string `json:"reason"`
		} `json:"results"`
	}
	json.NewDecoder(rec.Body).Decode(&resp)
	if len(resp.Results) != 4 {
		t.Fatalf("want 4 results, got %d", len(resp.Results))
	}
	if !resp.Results[0].OK {
		t.Errorf("result[0] should succeed: %q", resp.Results[0].Reason)
	}
	if resp.Results[1].OK || resp.Results[1].Code != "bad_request" {
		t.Errorf("result[1] should fail bad_request, got ok=%v code=%q", resp.Results[1].OK, resp.Results[1].Code)
	}
	if resp.Results[2].OK || resp.Results[2].Code != "bad_request" {
		t.Errorf("result[2] should fail bad_request (blank text), got ok=%v code=%q", resp.Results[2].OK, resp.Results[2].Code)
	}
	rows, _ := q.ListItemsForPack(context.Background(), db.ListItemsForPackParams{PackID: pack.ID, Lim: 50, Off: 0})
	if len(rows) != 1 {
		t.Errorf("want exactly 1 row inserted (only the valid one), got %d", len(rows))
	}
}

// Hard cap on items per request — frontend must chunk.
func TestBulkCreateTextItems_RejectsTooManyItems(t *testing.T) {
	h, q := newBulkHandler(t, newMemStorage())
	admin := seedAdmin(t, q)
	pack := seedPersonalPack(t, q, pgtype.UUID{Bytes: admin.ID, Valid: true})

	items := make([]map[string]string, api.MaxBulkTextItems+1)
	for i := range items {
		items[i] = map[string]string{"name": "n", "text": "t"}
	}
	rec := httptest.NewRecorder()
	h.BulkCreateTextItems(rec, bulkTextReq(t, pack.ID.String(), items, admin))

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("want 413, got %d — body: %s", rec.Code, rec.Body.String())
	}
}
