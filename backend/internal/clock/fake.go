package clock

import (
	"sync"
	"time"
)

// Fake is a controllable Clock for tests.
//
// Calls to Advance move the fake time forward and deterministically fire any
// Timer / Ticker / AfterFunc / Sleep entries whose deadline is crossed. Entries
// with the same deadline fire in creation order — so the same sequence of
// Advance calls always produces the same sequence of fires, regardless of
// scheduler noise.
//
// The design rule: every operation that can block on the wall clock in
// production (time.After, time.NewTimer, time.NewTicker, time.AfterFunc,
// time.Sleep) is routed through the fake, so tests never actually sleep.
type Fake struct {
	mu      sync.Mutex
	now     time.Time
	nextID  int
	entries []*fakeEntry
}

// NewFake returns a Fake initialised to the given start time. Pass a fixed
// time.Time so that golden-output tests remain stable across runs.
func NewFake(start time.Time) *Fake { return &Fake{now: start} }

// Now returns the current fake time.
func (f *Fake) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

// Sleep blocks the caller until the fake clock advances by at least d.
// In tests this is typically triggered by another goroutine calling Advance.
func (f *Fake) Sleep(d time.Duration) {
	<-f.After(d)
}

// After returns a channel that will receive a value once the fake clock has
// advanced by d.
func (f *Fake) After(d time.Duration) <-chan time.Time {
	ch := make(chan time.Time, 1)
	f.addEntry(&fakeEntry{
		deadline: f.addNow(d),
		ch:       ch,
	})
	return ch
}

// NewTimer returns a Timer that fires once, d into the future.
func (f *Fake) NewTimer(d time.Duration) Timer {
	ch := make(chan time.Time, 1)
	e := &fakeEntry{deadline: f.addNow(d), ch: ch}
	f.addEntry(e)
	return &fakeTimer{fake: f, entry: e}
}

// NewTicker returns a Ticker that fires every d.
func (f *Fake) NewTicker(d time.Duration) Ticker {
	if d <= 0 {
		panic("clock: non-positive interval for NewTicker")
	}
	ch := make(chan time.Time, 1)
	e := &fakeEntry{deadline: f.addNow(d), ch: ch, period: d}
	f.addEntry(e)
	return &fakeTicker{fake: f, entry: e}
}

// AfterFunc schedules fn to run after d. The returned Timer can Stop the
// pending call. Reset re-arms it.
func (f *Fake) AfterFunc(d time.Duration, fn func()) Timer {
	e := &fakeEntry{deadline: f.addNow(d), fn: fn}
	f.addEntry(e)
	return &fakeTimer{fake: f, entry: e}
}

// Advance moves the clock forward by d, firing every entry whose deadline is
// at or before the new time, in deadline order (creation order on ties).
// Tickers are re-scheduled in place. It is safe — and deterministic — to call
// Advance from the test goroutine even when fired callbacks register new
// entries with the fake; those entries are picked up within the same Advance
// call if their deadline falls inside the window.
func (f *Fake) Advance(d time.Duration) {
	f.mu.Lock()
	target := f.now.Add(d)
	for {
		due := f.popDueLocked(target)
		if due == nil {
			break
		}
		f.now = due.firedAt
		fn := due.fn
		ch := due.ch
		firedAt := due.firedAt
		f.mu.Unlock()
		if ch != nil {
			// Non-blocking send matches stdlib Ticker semantics: if the test
			// has not drained the previous tick, the old one stays pending.
			select {
			case ch <- firedAt:
			default:
			}
		}
		if fn != nil {
			fn()
		}
		f.mu.Lock()
	}
	f.now = target
	f.mu.Unlock()
}

// --- internals ---

func (f *Fake) addNow(d time.Duration) time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now.Add(d)
}

func (f *Fake) addEntry(e *fakeEntry) {
	f.mu.Lock()
	f.nextID++
	e.id = f.nextID
	f.entries = append(f.entries, e)
	f.mu.Unlock()
}

// popDueLocked finds the next entry whose deadline is <= target, consumes it
// (re-scheduling tickers, stopping one-shots), and returns a snapshot with
// firedAt set. Caller must hold f.mu and will receive the mutex still held.
// Returns nil when no more entries are due.
func (f *Fake) popDueLocked(target time.Time) *fakeEntry {
	// Sweep stopped entries and find the earliest due one.
	var best *fakeEntry
	bestIdx := -1
	write := 0
	for _, e := range f.entries {
		if e.stopped {
			continue
		}
		f.entries[write] = e
		if !e.deadline.After(target) {
			if best == nil ||
				e.deadline.Before(best.deadline) ||
				(e.deadline.Equal(best.deadline) && e.id < best.id) {
				best = e
				bestIdx = write
			}
		}
		write++
	}
	f.entries = f.entries[:write]
	if best == nil {
		return nil
	}
	snapshot := &fakeEntry{
		id:      best.id,
		firedAt: best.deadline,
		fn:      best.fn,
		ch:      best.ch,
	}
	if best.period > 0 {
		best.deadline = best.deadline.Add(best.period)
	} else {
		best.stopped = true
		// Remove it eagerly to keep the slice short.
		f.entries = append(f.entries[:bestIdx], f.entries[bestIdx+1:]...)
	}
	return snapshot
}

// fakeEntry is a single scheduled item. firedAt is set only on the snapshot
// returned from popDueLocked.
type fakeEntry struct {
	id       int
	deadline time.Time
	period   time.Duration // 0 = one-shot
	ch       chan time.Time
	fn       func()
	stopped  bool
	firedAt  time.Time
}

// fakeTimer is the Timer returned by Fake.NewTimer / Fake.AfterFunc.
type fakeTimer struct {
	fake  *Fake
	entry *fakeEntry
}

func (t *fakeTimer) C() <-chan time.Time {
	// AfterFunc timers have no channel, matching the Real behaviour.
	return t.entry.ch
}

func (t *fakeTimer) Stop() bool {
	t.fake.mu.Lock()
	defer t.fake.mu.Unlock()
	wasActive := !t.entry.stopped
	t.entry.stopped = true
	return wasActive
}

func (t *fakeTimer) Reset(d time.Duration) bool {
	t.fake.mu.Lock()
	wasActive := !t.entry.stopped
	t.entry.stopped = false
	t.entry.deadline = t.fake.now.Add(d)
	// If the entry was already removed from f.entries (one-shot that already
	// fired) we re-insert it so Advance can see it again.
	found := false
	for _, e := range t.fake.entries {
		if e == t.entry {
			found = true
			break
		}
	}
	if !found {
		t.fake.nextID++
		t.entry.id = t.fake.nextID
		t.fake.entries = append(t.fake.entries, t.entry)
	}
	t.fake.mu.Unlock()
	return wasActive
}

// fakeTicker is the Ticker returned by Fake.NewTicker.
type fakeTicker struct {
	fake  *Fake
	entry *fakeEntry
}

func (t *fakeTicker) C() <-chan time.Time { return t.entry.ch }

func (t *fakeTicker) Stop() {
	t.fake.mu.Lock()
	t.entry.stopped = true
	t.fake.mu.Unlock()
}
