package model

import (
	"sync"
	"time"
)

// Debouncer coalesces bursts of triggers per key into a single delayed call.
type Debouncer struct {
	window time.Duration
	mu     sync.Mutex
	timers map[string]*time.Timer
}

// NewDebouncer returns a debouncer with the given coalescing window.
func NewDebouncer(window time.Duration) *Debouncer {
	return &Debouncer{window: window, timers: map[string]*time.Timer{}}
}

// Trigger (re)starts key's timer; fn runs once when the window elapses with no
// further trigger for that key. A trigger within the window resets the timer.
func (d *Debouncer) Trigger(key string, fn func()) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if t, ok := d.timers[key]; ok {
		t.Stop()
	}
	d.timers[key] = time.AfterFunc(d.window, func() {
		d.mu.Lock()
		delete(d.timers, key)
		d.mu.Unlock()
		fn()
	})
}
