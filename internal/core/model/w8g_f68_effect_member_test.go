package model

import (
	"strings"
	"testing"
)

// F68's remaining residue: `Actions::TransitionAction::effect` comes back from
// the library cache without a scope, so a chain through it cannot be looked up.
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

	var noScope []string
	for _, d := range ws.Diagnostics(uri) {
		if strings.Contains(d.Message, "no scope for member lookup") {
			noScope = append(noScope, d.Message)
		}
	}
	if len(noScope) != 1 || !strings.Contains(noScope[0], "Actions::TransitionAction::effect") {
		t.Fatalf("expected the cached effect member to be the one gap, got %v", noScope)
	}
}
