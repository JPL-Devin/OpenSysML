package model

// EventKind classifies a change event feeding the reindex pipeline.
type EventKind int

const (
	EventOpen   EventKind = iota // LSP didOpen: register authoritative buffer
	EventChange                  // LSP didChange: replace buffer content
	EventClose                   // LSP didClose: drop buffer, revert to on-disk
	EventCreate                  // fsnotify: file created on disk
	EventModify                  // fsnotify: file modified on disk
	EventDelete                  // fsnotify: file deleted on disk
)

var eventKindNames = map[EventKind]string{
	EventOpen:   "open",
	EventChange: "change",
	EventClose:  "close",
	EventCreate: "create",
	EventModify: "modify",
	EventDelete: "delete",
}

func (k EventKind) String() string {
	if s, ok := eventKindNames[k]; ok {
		return s
	}
	return "unknown"
}

// ChangeEvent is one mutation posted to the workspace owner goroutine.
// ack, when non-nil, is closed after the event has been applied (test/sync aid).
type ChangeEvent struct {
	Kind    EventKind
	Name    string
	Content []byte
	Version int
	ack     chan struct{}
}

// EventLoop is the single owner goroutine that serializes workspace mutations.
type EventLoop struct {
	ws     *Workspace
	events chan ChangeEvent
	done   chan struct{}
}

// NewEventLoop returns a loop bound to ws. Call Run in a goroutine, then Post.
func NewEventLoop(ws *Workspace) *EventLoop {
	return &EventLoop{
		ws:     ws,
		events: make(chan ChangeEvent, 64),
		done:   make(chan struct{}),
	}
}

// Post enqueues an event for the owner goroutine. If the loop has been closed,
// Post returns without blocking (the event is dropped) so late producers such as
// the fsnotify watcher never hang on a stopped loop.
func (l *EventLoop) Post(ev ChangeEvent) {
	select {
	case l.events <- ev:
	case <-l.done:
	}
}

// Close stops the owner goroutine. Events still buffered when Close is observed
// are not guaranteed to be applied; callers that need delivery should drain
// before closing.
func (l *EventLoop) Close() { close(l.done) }

// Run is the owner loop; run it in its own goroutine. It is the sole writer to
// the workspace, so mutations are serialized even with many event producers.
func (l *EventLoop) Run() {
	for {
		select {
		case <-l.done:
			return
		case ev := <-l.events:
			l.apply(ev)
			if ev.ack != nil {
				close(ev.ack)
			}
		}
	}
}

func (l *EventLoop) apply(ev ChangeEvent) {
	switch ev.Kind {
	case EventOpen:
		l.ws.Open(ev.Name, ev.Content, ev.Version)
	case EventChange:
		l.ws.Update(ev.Name, ev.Content, ev.Version)
	case EventClose:
		l.ws.Close(ev.Name)
	case EventCreate, EventModify:
		l.ws.SetOnDisk(ev.Name, ev.Content)
	case EventDelete:
		l.ws.Remove(ev.Name)
	}
}
