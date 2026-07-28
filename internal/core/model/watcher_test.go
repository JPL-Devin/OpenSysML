package model

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatcherPostsModifyEvent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.sysml")
	if err := os.WriteFile(path, []byte("package P { namespace N; }"), 0o644); err != nil {
		t.Fatal(err)
	}

	ws := NewWorkspace()
	loop := NewEventLoop(ws)
	go loop.Run()
	defer loop.Close()

	w, err := NewWatcher(loop, 10*time.Millisecond, func(string) bool { return false })
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	if err := w.Add(dir); err != nil {
		t.Fatal(err)
	}
	go w.Run()

	// Modify the file; watcher should debounce then post an EventModify.
	if err := os.WriteFile(path, []byte("package P { namespace M; }"), 0o644); err != nil {
		t.Fatal(err)
	}

	deadline := time.After(2 * time.Second)
	for {
		if syms := ws.LookupQualified("P::M"); len(syms) == 1 {
			return // converged
		}
		select {
		case <-deadline:
			t.Fatalf("P::M not indexed after fs modify")
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func TestWatcherIgnoresOpenDocuments(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "b.sysml")
	if err := os.WriteFile(path, []byte("package P { namespace OnDisk; }"), 0o644); err != nil {
		t.Fatal(err)
	}

	ws := NewWorkspace()
	// Open with buffer content that differs from disk; buffer is authoritative.
	ws.Open("b.sysml", []byte("package P { namespace Buffered; }"), 1)

	loop := NewEventLoop(ws)
	go loop.Run()
	defer loop.Close()

	// isOpen reports the doc as open, so watcher drops its disk events.
	w, err := NewWatcher(loop, 10*time.Millisecond, func(name string) bool { return name == "b.sysml" })
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	if err := w.Add(dir); err != nil {
		t.Fatal(err)
	}
	go w.Run()

	if err := os.WriteFile(path, []byte("package P { namespace Changed; }"), 0o644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond) // allow any (wrongly) posted event to apply

	if syms := ws.LookupQualified("P::Buffered"); len(syms) != 1 {
		t.Fatalf("P::Buffered = %d, want 1 (buffer must stay authoritative)", len(syms))
	}
	if syms := ws.LookupQualified("P::Changed"); len(syms) != 0 {
		t.Fatalf("P::Changed = %d, want 0 (disk event must be ignored while open)", len(syms))
	}
}
