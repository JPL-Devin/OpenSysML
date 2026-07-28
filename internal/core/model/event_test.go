package model

import "testing"

func TestEventLoopAppliesEvents(t *testing.T) {
	ws := NewWorkspace()
	loop := NewEventLoop(ws)
	go loop.Run()
	defer loop.Close()

	done := make(chan struct{})
	loop.Post(ChangeEvent{Kind: EventOpen, Name: "a.sysml", Content: []byte("package P { namespace N; }"), Version: 1, ack: done})
	<-done

	if syms := ws.LookupQualified("P::N"); len(syms) != 1 {
		t.Fatalf("P::N = %d, want 1 after event applied", len(syms))
	}
}

func TestEventKindString(t *testing.T) {
	cases := map[EventKind]string{
		EventOpen: "open", EventChange: "change", EventClose: "close",
		EventCreate: "create", EventModify: "modify", EventDelete: "delete",
		EventKind(999): "unknown",
	}
	for k, want := range cases {
		if got := k.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", k, got, want)
		}
	}
}
