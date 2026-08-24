package model

import (
	"strings"
	"testing"
)

// F101: a transition's effect action is the `effect` its own scope names, so a
// chain through it reads that action rather than the scope-less library feature.
func TestW8GEffectMemberOfACachedLibrarySymbol(t *testing.T) {
	ws := NewWorkspace()
	uri := "file:///w8g_f68.sysml"
	ws.Open(uri, []byte(`package F68 {
	item def Msg;
	action def Send { in item m : Msg; }
	state def Behavior {
		state idle;
		state busy;
		transition delivering first idle do send Send() to busy then busy;
	}
	part server {
		exhibit state serverBehavior : Behavior;
		attribute a = serverBehavior.delivering.effect.sentMessage;
	}
}`), 1)
	defer ws.Close(uri)

	var unresolved []string
	for _, d := range ws.Diagnostics(uri) {
		if strings.Contains(d.Message, "no scope for member lookup") || strings.Contains(d.Message, "unresolved member") {
			unresolved = append(unresolved, d.Message)
		}
	}
	if len(unresolved) != 0 {
		t.Fatalf("expected the effect member chain to resolve, got %v", unresolved)
	}
}
