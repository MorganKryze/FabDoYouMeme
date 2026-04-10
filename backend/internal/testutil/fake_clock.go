package testutil

import (
	"testing"
	"time"

	"github.com/MorganKryze/FabDoYouMeme/backend/internal/clock"
)

// DefaultClockEpoch is the starting time every FakeClock uses by default.
// Picking a fixed, rounded time keeps test output stable across runs.
var DefaultClockEpoch = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// FakeClock returns a *clock.Fake anchored at DefaultClockEpoch. Pass it into
// any constructor that takes a clock.Clock. The t parameter is unused today
// but reserved for attaching t.Cleanup behaviour later.
func FakeClock(t *testing.T) *clock.Fake {
	t.Helper()
	return clock.NewFake(DefaultClockEpoch)
}

// FakeClockAt returns a *clock.Fake anchored at the given start time.
func FakeClockAt(t *testing.T, start time.Time) *clock.Fake {
	t.Helper()
	return clock.NewFake(start)
}
