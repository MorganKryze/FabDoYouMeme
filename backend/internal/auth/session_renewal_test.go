// backend/internal/auth/session_renewal_test.go
//
// Covers P1.3 from docs/review/2026-04-10/99-punch-list.md: SessionLookupFn
// must respect SessionRenewInterval instead of writing on every authenticated
// request. The test drives the handler through a clock.Fake so the renewal
// cadence is deterministic.

package auth_test

import (
	"context"
	"testing"
	"time"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/auth"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/clock"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/config"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/testutil"
)

// TestSession_RenewAtMostOncePerInterval is the P1.3 acceptance test: across a
// sequence of lookups driven by a fake clock, SessionLookupFn must renew the
// expires_at only when the notional new expiry would extend the current one
// by at least SessionRenewInterval. Repeated lookups inside the interval must
// leave the row untouched.
func TestSession_RenewAtMostOncePerInterval(t *testing.T) {
	// Anchor the fake clock to real wall time so Postgres-side
	// `expires_at > now()` in GetSessionByTokenHash keeps matching the seeded
	// row regardless of how far we Advance the fake clock below.
	start := time.Now().UTC().Truncate(time.Second)
	fake := clock.NewFake(start)

	const (
		sessionTTL    = 24 * time.Hour
		renewInterval = 10 * time.Minute
	)

	cfg := &config.Config{
		FrontendURL:          "http://localhost:3000",
		MagicLinkBaseURL:     "http://localhost:3000",
		MagicLinkTTL:         15 * time.Minute,
		SessionTTL:           sessionTTL,
		SessionRenewInterval: renewInterval,
	}
	h := auth.New(testutil.Pool(), cfg, &stubEmail{}, nil, fake)
	q := db.New(testutil.Pool())

	user := testutil.MakeUser(t, "player")

	// Seed a session whose expires_at matches exactly what a fresh login would
	// have produced against the fake clock. First lookup must be a no-op: the
	// delta between (fake.Now() + SessionTTL) and the stored ExpiresAt is zero,
	// which is well under the 10-minute renewal interval.
	raw, _ := auth.GenerateRawToken()
	hash := auth.HashToken(raw)
	initialExpiry := fake.Now().Add(sessionTTL)
	sess, err := q.CreateSession(context.Background(), db.CreateSessionParams{
		UserID:    user.ID,
		TokenHash: hash,
		ExpiresAt: initialExpiry,
	})
	if err != nil {
		t.Fatalf("seed session: %v", err)
	}

	// readExpiry returns the current expires_at for the seeded session.
	readExpiry := func() time.Time {
		row, err := q.GetSessionByTokenHash(context.Background(), hash)
		if err != nil {
			t.Fatalf("GetSessionByTokenHash: %v", err)
		}
		return row.ExpiresAt.UTC()
	}

	// Helper: invoke SessionLookupFn and fail on any error.
	lookup := func() {
		t.Helper()
		if _, _, _, _, _, err := h.SessionLookupFn(context.Background(), hash); err != nil {
			t.Fatalf("SessionLookupFn: %v", err)
		}
	}

	// --- Step 1: immediate lookup — must NOT renew. ---
	lookup()
	if got := readExpiry(); !got.Equal(initialExpiry.UTC()) {
		t.Fatalf("immediate lookup renewed session: want %v, got %v", initialExpiry, got)
	}

	// --- Step 2: advance 9m (< 10m interval). Still must NOT renew. ---
	fake.Advance(9 * time.Minute)
	lookup()
	if got := readExpiry(); !got.Equal(initialExpiry.UTC()) {
		t.Fatalf("lookup inside renewal window renewed session: want %v, got %v", initialExpiry, got)
	}

	// --- Step 3: advance to exactly the 10m boundary. MUST renew. ---
	fake.Advance(1 * time.Minute) // total elapsed = 10m
	lookup()
	afterFirstRenew := readExpiry()
	expectedAfterRenew := fake.Now().Add(sessionTTL).UTC()
	if !afterFirstRenew.Equal(expectedAfterRenew) {
		t.Fatalf("renewal at interval boundary wrong: want %v, got %v", expectedAfterRenew, afterFirstRenew)
	}
	if !afterFirstRenew.After(initialExpiry.UTC()) {
		t.Fatalf("renewal did not push expires_at forward: before %v, after %v", initialExpiry, afterFirstRenew)
	}

	// --- Step 4: back-to-back lookup immediately after renewal. Must NOT renew again. ---
	lookup()
	if got := readExpiry(); !got.Equal(afterFirstRenew) {
		t.Fatalf("second lookup at same instant re-renewed session: want %v, got %v", afterFirstRenew, got)
	}

	// --- Step 5: advance less than the interval again. Must NOT renew. ---
	fake.Advance(5 * time.Minute)
	lookup()
	if got := readExpiry(); !got.Equal(afterFirstRenew) {
		t.Fatalf("lookup inside second renewal window renewed: want %v, got %v", afterFirstRenew, got)
	}

	// --- Step 6: advance past the interval. MUST renew a second time. ---
	fake.Advance(5 * time.Minute) // 5m + 5m = 10m since previous renewal
	lookup()
	afterSecondRenew := readExpiry()
	if !afterSecondRenew.After(afterFirstRenew) {
		t.Fatalf("second renewal did not push expires_at forward: before %v, after %v", afterFirstRenew, afterSecondRenew)
	}

	_ = sess // keep the seeded row alive for the lifetime of the test
}
