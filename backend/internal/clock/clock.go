// Package clock provides a time abstraction that can be replaced in tests.
//
// Production code takes a clock.Clock and calls Now/NewTimer/AfterFunc through
// it instead of the stdlib time package directly. Tests inject a *Fake whose
// Advance method fires pending timers deterministically, which makes
// time-dependent logic (rate-limit windows, hub round deadlines, session
// renewal cadence) testable without real sleeps or flaky scheduling.
package clock

import "time"

// Clock is the minimal time surface used by the backend. It mirrors the parts
// of the stdlib time package that production code actually needs.
type Clock interface {
	Now() time.Time
	NewTimer(d time.Duration) Timer
	NewTicker(d time.Duration) Ticker
	After(d time.Duration) <-chan time.Time
	AfterFunc(d time.Duration, f func()) Timer
	Sleep(d time.Duration)
}

// Timer abstracts *time.Timer so a fake implementation can be substituted.
// The C() method exposes the fire channel; AfterFunc timers return a nil
// channel because their callback runs inline when the deadline is crossed.
type Timer interface {
	C() <-chan time.Time
	Stop() bool
	Reset(d time.Duration) bool
}

// Ticker abstracts *time.Ticker the same way Timer abstracts *time.Timer.
type Ticker interface {
	C() <-chan time.Time
	Stop()
}
