package clock

import (
	"testing"
	"time"
)

func TestReal_NowIsMonotonic(t *testing.T) {
	var c Clock = Real{}
	a := c.Now()
	b := c.Now()
	if b.Before(a) {
		t.Fatalf("Now went backwards: %v -> %v", a, b)
	}
}

func TestReal_AfterFires(t *testing.T) {
	var c Clock = Real{}
	select {
	case <-c.After(5 * time.Millisecond):
	case <-time.After(1 * time.Second):
		t.Fatal("Real.After never fired")
	}
}

func TestReal_AfterFuncCallsCallback(t *testing.T) {
	var c Clock = Real{}
	done := make(chan struct{})
	c.AfterFunc(5*time.Millisecond, func() { close(done) })
	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("Real.AfterFunc callback never ran")
	}
}
