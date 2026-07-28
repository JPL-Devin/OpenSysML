package model

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestDebouncerCoalescesBurst(t *testing.T) {
	var calls int32
	d := NewDebouncer(30 * time.Millisecond)
	for i := 0; i < 5; i++ {
		d.Trigger("a.sysml", func() { atomic.AddInt32(&calls, 1) })
		time.Sleep(3 * time.Millisecond)
	}
	time.Sleep(80 * time.Millisecond)
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("calls = %d, want 1 (burst coalesced)", got)
	}
}

func TestDebouncerDistinctKeysIndependent(t *testing.T) {
	var a, b int32
	d := NewDebouncer(20 * time.Millisecond)
	d.Trigger("a.sysml", func() { atomic.AddInt32(&a, 1) })
	d.Trigger("b.sysml", func() { atomic.AddInt32(&b, 1) })
	time.Sleep(70 * time.Millisecond)
	if atomic.LoadInt32(&a) != 1 || atomic.LoadInt32(&b) != 1 {
		t.Fatalf("a=%d b=%d, want 1 and 1", a, b)
	}
}
