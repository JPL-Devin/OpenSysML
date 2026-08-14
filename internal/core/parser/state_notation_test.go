package parser

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

// stateDefMembers parses src and returns the members of its first state
// definition, with memberships unwrapped.
func stateDefMembers(t *testing.T, src string) []ast.Node {
	t.Helper()
	p := New(source.New("test.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("expected no diagnostics, got %v", p.Diagnostics)
	}

	var members []ast.Node
	var walk func(nodes []ast.Node)
	walk = func(nodes []ast.Node) {
		for _, node := range nodes {
			if membership, ok := node.(*ast.Membership); ok {
				node = membership.Member
			}
			switch n := node.(type) {
			case *ast.Package:
				walk(n.Members)
			case *ast.Definition:
				if n.Kind == ast.DefState && members == nil {
					members = unwrapAll(n.Members)
				}
			case *ast.Usage:
				if n.Kind == ast.UsageState && members == nil {
					members = unwrapAll(n.Members)
				}
			}
		}
	}
	walk(root.Members)
	if members == nil {
		t.Fatal("no state declaration found")
	}
	return members
}

func unwrapAll(nodes []ast.Node) []ast.Node {
	out := make([]ast.Node, 0, len(nodes))
	for _, node := range nodes {
		if membership, ok := node.(*ast.Membership); ok {
			node = membership.Member
		}
		out = append(out, node)
	}
	return out
}

// The history and entry/exit point keywords each produce the pseudostate kind
// they name, with `history` on its own meaning shallow history.
func TestHistoryAndPointPseudostateParsing(t *testing.T) {
	members := stateDefMembers(t, `
package Test {
	state def Controller {
		state Idle;
		history resume;
		shallow history resumeShallow;
		deep history resumeDeep;
		entry point into;
		exit point outOf;
	}
}`)

	got := make(map[string]ast.PseudostateKind)
	for _, member := range members {
		if ps, ok := member.(*ast.PseudostateNode); ok {
			got[ps.Name] = ps.Kind
		}
	}

	want := map[string]ast.PseudostateKind{
		"resume":        ast.PseudostateShallowHistory,
		"resumeShallow": ast.PseudostateShallowHistory,
		"resumeDeep":    ast.PseudostateDeepHistory,
		"into":          ast.PseudostateEntry,
		"outOf":         ast.PseudostateExit,
	}
	for name, kind := range want {
		if got[name] != kind {
			t.Errorf("pseudostate %q: expected kind %v, got %v", name, kind, got[name])
		}
	}
	if len(got) != len(want) {
		t.Errorf("expected %d pseudostates, got %d: %v", len(want), len(got), got)
	}
}

// `defer` collects the events of one declaration in order, parsing each of them
// exactly like a transition trigger.
func TestDeferMemberParsing(t *testing.T) {
	members := stateDefMembers(t, `
package Test {
	state def Controller {
		defer Ping, setSpeed(value);
		defer Pong;
	}
}`)

	var defers []*ast.DeferMember
	for _, member := range members {
		if d, ok := member.(*ast.DeferMember); ok {
			defers = append(defers, d)
		}
	}
	if len(defers) != 2 {
		t.Fatalf("expected 2 defer members, got %d", len(defers))
	}
	if len(defers[0].Triggers) != 2 {
		t.Fatalf("expected the first defer to carry 2 events, got %d", len(defers[0].Triggers))
	}
	if name, ok := defers[0].Triggers[0].(*ast.QualifiedName); !ok || ast.SimpleName(name) != "Ping" {
		t.Errorf("expected the first deferred event to be the signal Ping, got %T", defers[0].Triggers[0])
	}
	call, ok := defers[0].Triggers[1].(*ast.CallEvent)
	if !ok {
		t.Fatalf("expected the second deferred event to be a call event, got %T", defers[0].Triggers[1])
	}
	if ast.SimpleName(call.Operation) != "setSpeed" {
		t.Errorf("expected the deferred call to be setSpeed, got %q", ast.SimpleName(call.Operation))
	}
	if len(defers[1].Triggers) != 1 {
		t.Errorf("expected the second defer to carry 1 event, got %d", len(defers[1].Triggers))
	}
}

// `point` is matched contextually rather than reserved, so a feature may still
// be named `point` in the same state that declares an entry point.
func TestPointIsNotReserved(t *testing.T) {
	members := stateDefMembers(t, `
package Test {
	state def Controller {
		attribute point : Integer;
		entry point into;
	}
}`)

	var attributes int
	var points int
	for _, member := range members {
		switch n := member.(type) {
		case *ast.Usage:
			if n.Kind == ast.UsageAttribute && n.Ident.Name == "point" {
				attributes++
			}
		case *ast.PseudostateNode:
			if n.Kind == ast.PseudostateEntry && n.Name == "into" {
				points++
			}
		}
	}
	if attributes != 1 {
		t.Errorf("expected the attribute named point to survive, got %d", attributes)
	}
	if points != 1 {
		t.Errorf("expected 1 entry point pseudostate, got %d", points)
	}
}

// A statement written as a transition's `do` effect is terminated by the
// transition's own `then` clause, so no semicolon is expected before it.
func TestTransitionEffectStatementNeedsNoSemicolonBeforeThen(t *testing.T) {
	members := stateDefMembers(t, `
package Test {
	attribute def Warning;
	state def Controller {
		attribute level : Integer;
		state idle;
		state alerting;
		transition first idle accept w : Warning do assign level := 1 then alerting;
		transition first alerting accept w : Warning do perform notify then idle;
	}
}`)

	var transitions []*ast.TransitionMember
	for _, member := range members {
		if trans, ok := member.(*ast.TransitionMember); ok {
			transitions = append(transitions, trans)
		}
	}
	if len(transitions) != 2 {
		t.Fatalf("expected 2 transitions, got %d", len(transitions))
	}
	for i, want := range []string{"alerting", "idle"} {
		if len(transitions[i].Effect) != 1 {
			t.Errorf("transition %d: expected 1 effect, got %d", i, len(transitions[i].Effect))
		}
		if got := ast.SimpleName(transitions[i].Target); got != want {
			t.Errorf("transition %d: expected target %q, got %q", i, want, got)
		}
	}
	if _, ok := transitions[0].Effect[0].(*ast.AssignmentActionNode); !ok {
		t.Errorf("expected an assignment effect, got %T", transitions[0].Effect[0])
	}
	effect := transitions[1].Effect[0]
	if membership, ok := effect.(*ast.Membership); ok {
		effect = membership.Member
	}
	if usage, ok := effect.(*ast.Usage); !ok || usage.Keyword != "perform" {
		t.Errorf("expected a performed action effect, got %T", effect)
	}
}
