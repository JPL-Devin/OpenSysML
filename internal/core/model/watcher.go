package model

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watcher translates filesystem events for .sysml/.kerml files into
// ChangeEvents posted to an EventLoop, debounced per path. Open documents
// (per the isOpen predicate) have their on-disk events dropped, so the LSP
// buffer stays authoritative.
type Watcher struct {
	loop   *EventLoop
	fsw    *fsnotify.Watcher
	deb    *Debouncer
	isOpen func(name string) bool
	done   chan struct{}
}

// NewWatcher creates a Watcher posting to loop, coalescing bursts within
// window, and consulting isOpen(name) to skip open documents.
func NewWatcher(loop *EventLoop, window time.Duration, isOpen func(name string) bool) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	if isOpen == nil {
		isOpen = func(string) bool { return false }
	}
	return &Watcher{
		loop:   loop,
		fsw:    fsw,
		deb:    NewDebouncer(window),
		isOpen: isOpen,
		done:   make(chan struct{}),
	}, nil
}

// Add registers a directory to watch.
func (w *Watcher) Add(dir string) error { return w.fsw.Add(dir) }

// Close stops watching and releases resources.
func (w *Watcher) Close() error {
	close(w.done)
	return w.fsw.Close()
}

// Run consumes filesystem events until Close. It is the sole caller of the
// debouncer, which in turn posts to the EventLoop (the sole workspace writer).
func (w *Watcher) Run() {
	for {
		select {
		case <-w.done:
			return
		case ev, ok := <-w.fsw.Events:
			if !ok {
				return
			}
			w.handle(ev)
		case _, ok := <-w.fsw.Errors:
			if !ok {
				return
			}
			// Errors are non-fatal for v1; drop them.
		}
	}
}

func (w *Watcher) handle(ev fsnotify.Event) {
	if !IsModelSource(ev.Name) {
		return
	}
	name := filepath.Base(ev.Name)
	if w.isOpen(name) {
		return // buffer authoritative; ignore disk event
	}
	path := ev.Name
	switch {
	case ev.Op&fsnotify.Remove != 0 || ev.Op&fsnotify.Rename != 0:
		w.deb.Trigger(name, func() {
			w.loop.Post(ChangeEvent{Kind: EventDelete, Name: name})
		})
	case ev.Op&fsnotify.Create != 0:
		w.deb.Trigger(name, func() {
			// #nosec G304 -- path is a file the watcher was asked to watch.
			content, err := os.ReadFile(path)
			if err != nil {
				return
			}
			w.loop.Post(ChangeEvent{Kind: EventCreate, Name: name, Content: content})
		})
	case ev.Op&fsnotify.Write != 0:
		w.deb.Trigger(name, func() {
			// #nosec G304 -- path is a file the watcher was asked to watch.
			content, err := os.ReadFile(path)
			if err != nil {
				return
			}
			w.loop.Post(ChangeEvent{Kind: EventModify, Name: name, Content: content})
		})
	}
}

// IsModelSource reports whether path is a SysML/KerML source file.
func IsModelSource(path string) bool {
	return strings.HasSuffix(path, ".sysml") || strings.HasSuffix(path, ".kerml")
}
