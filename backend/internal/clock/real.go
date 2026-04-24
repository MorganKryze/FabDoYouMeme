package clock

import "time"

// Real is the production Clock. Every method delegates straight to the stdlib
// time package. It is a zero-sized value so it costs nothing to pass around
// and can be embedded or stored without concern.
type Real struct{}

func (Real) Now() time.Time                         { return time.Now() }
func (Real) After(d time.Duration) <-chan time.Time { return time.After(d) }
func (Real) Sleep(d time.Duration)                  { time.Sleep(d) }

func (Real) NewTimer(d time.Duration) Timer {
	return &realTimer{t: time.NewTimer(d)}
}

func (Real) NewTicker(d time.Duration) Ticker {
	return &realTicker{t: time.NewTicker(d)}
}

func (Real) AfterFunc(d time.Duration, f func()) Timer {
	return &realTimer{t: time.AfterFunc(d, f), afterFunc: true}
}

type realTimer struct {
	t         *time.Timer
	afterFunc bool
}

func (r *realTimer) C() <-chan time.Time {
	if r.afterFunc {
		// AfterFunc timers expose no fire channel; return nil so a caller
		// that mistakenly selects on it blocks forever rather than reading
		// zero values.
		return nil
	}
	return r.t.C
}

func (r *realTimer) Stop() bool                 { return r.t.Stop() }
func (r *realTimer) Reset(d time.Duration) bool { return r.t.Reset(d) }

type realTicker struct{ t *time.Ticker }

func (r *realTicker) C() <-chan time.Time { return r.t.C }
func (r *realTicker) Stop()               { r.t.Stop() }
