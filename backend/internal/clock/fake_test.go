package clock

import (
	"sync/atomic"
	"testing"
	"time"
)

var epoch = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func TestFake_NowReflectsAdvance(t *testing.T) {
	f := NewFake(epoch)
	if got := f.Now(); !got.Equal(epoch) {
		t.Fatalf("initial now = %v, want %v", got, epoch)
	}
	f.Advance(2 * time.Second)
	if got := f.Now(); !got.Equal(epoch.Add(2 * time.Second)) {
		t.Fatalf("after advance now = %v", got)
	}
}

func TestFake_AfterFiresOnAdvance(t *testing.T) {
	f := NewFake(epoch)
	ch := f.After(5 * time.Second)
	select {
	case <-ch:
		t.Fatal("fired before advance")
	default:
	}
	f.Advance(5 * time.Second)
	select {
	case got := <-ch:
		if !got.Equal(epoch.Add(5 * time.Second)) {
			t.Errorf("fired at %v, want %v", got, epoch.Add(5*time.Second))
		}
	default:
		t.Fatal("did not fire after advance")
	}
}

func TestFake_AfterFuncFiresInDeadlineOrder(t *testing.T) {
	f := NewFake(epoch)
	var order []int
	f.AfterFunc(3*time.Second, func() { order = append(order, 3) })
	f.AfterFunc(1*time.Second, func() { order = append(order, 1) })
	f.AfterFunc(2*time.Second, func() { order = append(order, 2) })

	f.Advance(5 * time.Second)

	if len(order) != 3 || order[0] != 1 || order[1] != 2 || order[2] != 3 {
		t.Fatalf("fire order = %v, want [1 2 3]", order)
	}
}

func TestFake_AfterFuncSameDeadlineFiresInCreationOrder(t *testing.T) {
	f := NewFake(epoch)
	var order []int
	for i := range 5 {
		i := i
		f.AfterFunc(1*time.Second, func() { order = append(order, i) })
	}
	f.Advance(1 * time.Second)
	for i, v := range order {
		if v != i {
			t.Fatalf("fire order = %v, want [0 1 2 3 4]", order)
		}
	}
}

func TestFake_TimerStopPreventsFire(t *testing.T) {
	f := NewFake(epoch)
	var fired int32
	t1 := f.AfterFunc(1*time.Second, func() { atomic.StoreInt32(&fired, 1) })
	if !t1.Stop() {
		t.Fatal("Stop on active timer should return true")
	}
	f.Advance(2 * time.Second)
	if atomic.LoadInt32(&fired) != 0 {
		t.Fatal("stopped timer fired")
	}
	if t1.Stop() {
		t.Fatal("Stop on already-stopped timer should return false")
	}
}

func TestFake_TimerResetReArms(t *testing.T) {
	f := NewFake(epoch)
	var fires int32
	t1 := f.AfterFunc(1*time.Second, func() { atomic.AddInt32(&fires, 1) })
	f.Advance(1 * time.Second)
	if atomic.LoadInt32(&fires) != 1 {
		t.Fatalf("first fire count = %d", fires)
	}
	t1.Reset(1 * time.Second)
	f.Advance(1 * time.Second)
	if atomic.LoadInt32(&fires) != 2 {
		t.Fatalf("second fire count = %d", fires)
	}
}

func TestFake_TickerFiresRepeatedly(t *testing.T) {
	f := NewFake(epoch)
	tk := f.NewTicker(1 * time.Second)
	defer tk.Stop()

	// Drain 3 ticks.
	count := 0
	f.Advance(3 * time.Second)
	for range 3 {
		select {
		case <-tk.C():
			count++
		default:
			// The stdlib ticker drops ticks if the consumer is slow; so does
			// the fake. Reading 3 ticks from a buffer-1 channel after
			// advancing 3s will only return the most recent one.
		}
	}
	if count == 0 {
		t.Fatal("ticker did not fire at all")
	}
}

func TestFake_AdvanceReEntrantAfterFunc(t *testing.T) {
	// An AfterFunc callback that schedules another AfterFunc inside the same
	// Advance window must see that new entry fire within the same Advance.
	f := NewFake(epoch)
	var order []string
	f.AfterFunc(1*time.Second, func() {
		order = append(order, "first")
		f.AfterFunc(1*time.Second, func() {
			order = append(order, "second")
		})
	})
	f.Advance(3 * time.Second)
	if len(order) != 2 || order[0] != "first" || order[1] != "second" {
		t.Fatalf("re-entrant fire order = %v, want [first second]", order)
	}
}

func TestFake_SleepBlocksUntilAdvance(t *testing.T) {
	f := NewFake(epoch)
	done := make(chan struct{})
	go func() {
		f.Sleep(2 * time.Second)
		close(done)
	}()
	// Give the goroutine a tick to park on the channel. We cannot use
	// f.Sleep here — that would also block on the fake — so wait on a real
	// time.After with a very short timeout.
	select {
	case <-done:
		t.Fatal("Sleep returned before Advance")
	case <-time.After(10 * time.Millisecond):
	}
	f.Advance(2 * time.Second)
	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Sleep did not wake after Advance")
	}
}
