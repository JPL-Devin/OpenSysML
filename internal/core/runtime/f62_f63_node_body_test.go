package runtime

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

// TestF62F63NodeBodyRobustness covers the failure modes the newly accepted node
// bodies reach: a body statement that cannot run must error, never panic or hang.
func TestF62F63NodeBodyRobustness(t *testing.T) {
	t.Run("control_node_body_never_terminates", testControlNodeBodyNeverTerminates)
	t.Run("control_node_body_sends_without_a_message", testControlNodeBodySendsWithoutAMessage)
	t.Run("send_body_declares_no_payload", testSendBodyDeclaresNoPayload)
	t.Run("send_via_a_port_to_a_receiver", testSendViaAPortToAReceiver)
}

func runActionForError(t *testing.T, src, name string) error {
	t.Helper()
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), name, ast.DefAction)
	if sym == nil {
		t.Fatalf("action %s not found", name)
	}
	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		return err
	}
	return exec.RunToCompletion()
}

// A control node body runs as control reaches the node, so a loop of it that
// never ends is bounded by the step budget instead of hanging.
func testControlNodeBodyNeverTerminates(t *testing.T) {
	src := `
		package test {
			action gate {
				attribute n = 0;
				first start;
				fork split {
					while n >= 0 { assign n := n + 1; }
				}
				done end;
				then start split;
				then split end;
			}
		}
	`
	err := runActionForError(t, src, "gate")
	if err == nil {
		t.Fatal("expected the step budget to bound the fork body, it completed")
	}
	t.Logf("error: %v", err)
}

// A send in a control node body is the same statement as anywhere else, so one
// with no message is reported there too.
func testControlNodeBodySendsWithoutAMessage(t *testing.T) {
	src := `
		package test {
			action gate {
				first start;
				fork split { send to receiver { } }
				action receiver accept n : Integer;
				done end;
				then start split;
				then split receiver;
				then receiver end;
			}
		}
	`
	err := runActionForError(t, src, "gate")
	if err == nil {
		t.Fatal("expected an error, the send in the fork body declared no message")
	}
	if !strings.Contains(err.Error(), "send") {
		t.Errorf("expected the error to name the send, got: %v", err)
	}
}

// A send with neither an argument nor a payload in its body has no message to
// send, which is reported rather than sending nothing.
func testSendBodyDeclaresNoPayload(t *testing.T) {
	src := `
		package test {
			action talk {
				first start;
				action sender {
					send to receiver { }
				}
				action receiver accept n : Integer;
				done end;
				then start sender;
				then sender receiver;
				then receiver end;
			}
		}
	`
	err := runActionForError(t, src, "talk")
	if err == nil {
		t.Fatal("expected an error, the send declared no message")
	}
	if !strings.Contains(err.Error(), "send") {
		t.Errorf("expected the error to name the send, got: %v", err)
	}
}

// `send x via p to r` states both a port and a receiver (SysML.xtext
// SenderReceiverPart). It parses, and the runtime reports it as unsupported
// rather than dropping either end.
func testSendViaAPortToAReceiver(t *testing.T) {
	src := `
		package test {
			port def Pt;
			action talk {
				port p : Pt;
				attribute x = 1;
				first start;
				action sender {
					send x via p to receiver;
				}
				action receiver accept n : Integer;
				done end;
				then start sender;
				then sender receiver;
				then receiver end;
			}
		}
	`
	err := runActionForError(t, src, "talk")
	if err == nil {
		t.Fatal("expected an unsupported error for a send stating both a port and a receiver")
	}
	t.Logf("error: %v", err)
}
