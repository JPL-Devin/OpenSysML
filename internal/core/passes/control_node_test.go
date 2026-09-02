package passes

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// controlNodeDiags runs the pass alone on a model that must parse clean.
func controlNodeDiags(t *testing.T, src string) []Diagnostic {
	t.Helper()
	sf := source.New("t.sysml", []byte(src))
	p := parser.New(sf)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("unexpected parse diagnostics: %+v", p.Diagnostics)
	}
	idx := newTestIndexFromDoc("t.sysml", root)
	return ControlNodeSuccessionPass{}.Run(NewContext("t.sysml", idx, nil), "t.sysml", root)
}

func wantControlNodesClean(t *testing.T, src string) {
	t.Helper()
	if got := controlNodeDiags(t, src); len(got) != 0 {
		t.Fatalf("got %+v, want no diagnostics", got)
	}
}

// wantControlNodeErrors fails unless the pass reports exactly the given codes, in
// order, each at the given 1-based line with a message containing the text.
func wantControlNodeErrors(t *testing.T, src string, want ...controlNodeWant) []Diagnostic {
	t.Helper()
	got := controlNodeDiags(t, src)
	if len(got) != len(want) {
		t.Fatalf("got %d diagnostics %+v, want %d", len(got), got, len(want))
	}
	sf := source.New("t.sysml", []byte(src))
	for i, d := range got {
		w := want[i]
		line := sf.Lines().PosAt(d.Span.Offset).Line
		if d.Severity != SeverityError || d.Source != "control-node" || d.Code != w.code {
			t.Fatalf("diagnostic %d: got %+v, want severity=error source=control-node code=%s", i, d, w.code)
		}
		if line != w.line {
			t.Fatalf("diagnostic %d: got line %d (%q), want line %d", i, line, d.Message, w.line)
		}
		if !strings.Contains(d.Message, w.text) {
			t.Fatalf("diagnostic %d: got message %q, want it to contain %q", i, d.Message, w.text)
		}
	}
	return got
}

type controlNodeWant struct {
	code string
	line int
	text string
}

// A well-formed graph in every succession notation is silent: a fork with one
// incoming, a join and merge with one outgoing, a decision with one incoming.
func TestControlNodeWellFormedGraphIsSilent(t *testing.T) {
	wantControlNodesClean(t, `package P {
	action def A {
		action a;
		then fork f;
		first f then b;
		first f then c;
		action b;
		action c;
		join j;
		succession s1 first b then j;
		succession s2 first c then j;
		then merge m;
		then decide d;
		if true then e;
		else g;
		action e;
		action g;
	}
}`)
}

// The specification's merge and decision examples (SysML v2 8.4.13.4) carry the
// required multiplicities on every succession end, written as the notation
// places them: before the end they constrain.
func TestControlNodeSpecificationExamplesAreSilent(t *testing.T) {
	wantControlNodesClean(t, `package P {
	action def A3 {
		action a1;
		action a2;
		succession s1 first [0..1] a1 then [1..1] m;
		succession s2 first [0..1] a2 then [1..1] m;
		merge m;
		succession s3 first [1..1] m then a3;
		action a3;
	}
	action def A4 {
		action a1;
		succession s1 first a1 then [1..1] d;
		decide d;
		succession s2 first [1..1] d then [0..1] a2;
		succession s3 first [1..1] d then [0..1] a3;
		action a2;
		action a3;
	}
	action def A2 {
		action a1;
		succession s1 first a1 then [1] f;
		fork f;
		succession s2 first [1] f then [1..1] a2;
		succession s3 first [1] f then [1] a3;
		action a2;
		action a3;
	}
}`)
}

// Only successions count: a connection or binding to a control node is not a
// succession into it.
func TestControlNodeOtherConnectorsDoNotCount(t *testing.T) {
	wantControlNodesClean(t, `package P {
	action def A {
		action a;
		action b;
		fork f;
		first a then f;
		connect b to f;
		bind b = f;
		first f then b;
	}
}`)
}

// Control nodes live in every action body shape: nested usages, loop and branch
// bodies, a control node's own body, and the action kinds that specialize action.
func TestControlNodeInEveryActionBodyIsSilent(t *testing.T) {
	wantControlNodesClean(t, `package P {
	action def A {
		action a { fork f; action x; action y; first f then x; first f then y; }
		while true { join j; action p; first p then j; }
		if true { merge m; }
		then fork g { decide d; }
	}
	part def Q {
		action c { fork r; }
		perform action p { join s; }
	}
	state def S { entry action { fork t; } }
	calc def K { fork kk; }
	case def C { fork cc; }
}`)
}

func TestForkWithTwoIncomingSuccessions(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"shorthand", `action a; then fork f; action b; then f; first f then c; action c;`},
		{"first then", `action a; action b; fork f; first a then f; first b then f; first f then a;`},
		{"succession usage", `action a; action b; fork f; succession s1 first a then f; succession s2 first b then f;`},
		{"mixed", `action a; action b; fork f; first a then f; succession s2 first b then f;`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wantControlNodeErrors(t, "package P {\n\taction def A {\n\t\t"+tc.body+"\n\t}\n}",
				controlNodeWant{CodeForkIncomingSuccessions, 3, "fork f has 2 incoming successions; a fork node may have at most one"})
		})
	}
}

func TestJoinWithTwoOutgoingSuccessions(t *testing.T) {
	wantControlNodeErrors(t, `package P {
	action def A {
		action a;
		action b;
		action c;
		join j;
		first a then j;
		first j then b;
		first j then c;
	}
}`, controlNodeWant{CodeJoinOutgoingSuccessions, 6, "join j has 2 outgoing successions; a join node may have at most one"})
}

func TestMergeWithTwoOutgoingSuccessions(t *testing.T) {
	wantControlNodeErrors(t, `package P {
	action def A {
		action a;
		action b;
		action c;
		merge m;
		first a then m;
		succession s1 first m then b;
		succession s2 first m then c;
	}
}`, controlNodeWant{CodeMergeOutgoingSuccessions, 6, "merge m has 2 outgoing successions; a merge node may have at most one"})
}

func TestDecisionWithTwoIncomingSuccessions(t *testing.T) {
	wantControlNodeErrors(t, `package P {
	action def A {
		action a;
		action b;
		decide d;
		first a then d;
		first b then d;
		if true then a;
		else b;
	}
}`, controlNodeWant{CodeDecisionIncomingSuccessions, 5, "decide d has 2 incoming successions; a decision node may have at most one"})
}

// The bounded side is the only one bounded: three successions out of a fork or
// decision, or into a join or merge, are legal.
func TestControlNodeUnboundedSideIsSilent(t *testing.T) {
	wantControlNodesClean(t, `package P {
	action def A {
		action a; action b; action c; action e;
		fork f;
		first a then f;
		first f then b; first f then c; first f then e;
		join j;
		first b then j; first c then j; first e then j;
		merge m;
		first j then m; first a then m; first b then m;
		decide d;
		succession first m then d;
		if true then a; if false then b; else c;
	}
}`)
}

func TestMergeIncomingSourceMultiplicity(t *testing.T) {
	wantControlNodeErrors(t, `package P {
	action def A {
		action a; action b; action c;
		succession s1 first [1..1] a then [1] m;
		succession s2 first [0..1] b then [1] m;
		merge m;
		succession s3 first [1] m then c;
	}
}`, controlNodeWant{CodeMergeIncomingMultiplicity, 4,
		"succession into merge m has source multiplicity [1]; successions into a merge node must have source multiplicity [0..1]"})
}

func TestDecisionOutgoingTargetMultiplicity(t *testing.T) {
	wantControlNodeErrors(t, `package P {
	action def A {
		action a; action b; action c;
		succession s1 first a then [1] d;
		decide d;
		succession s2 first [1] d then [1] b;
		succession s3 first [1] d then [0..*] c;
	}
}`, controlNodeWant{CodeDecisionOutgoingMultiplicity, 6,
		"succession out of decide d has target multiplicity [1]; successions out of a decision node must have target multiplicity [0..1]"},
		controlNodeWant{CodeDecisionOutgoingMultiplicity, 7,
			"succession out of decide d has target multiplicity [0..*]; successions out of a decision node must have target multiplicity [0..1]"})
}

// Every control node kind requires 1..1 on the target end of an incoming
// succession and on the source end of an outgoing one.
func TestControlNodeEndMultiplicities(t *testing.T) {
	wantControlNodeErrors(t, `package P {
	action def A {
		action a; action b; action c;
		succession s1 first a then [0..1] f;
		fork f;
		succession s2 first [0..1] f then b;
		succession s3 first [2..3] f then c;
		join j;
		succession s4 first b then [*] j;
		succession s5 first c then [1] j;
		succession s6 first [0..1] j then a;
	}
}`, controlNodeWant{CodeControlNodeIncomingMultiplicity, 4,
		"succession into fork f has target multiplicity [0..1]; successions into a fork node must have target multiplicity [1]"},
		controlNodeWant{CodeControlNodeOutgoingMultiplicity, 6,
			"succession out of fork f has source multiplicity [0..1]; successions out of a fork node must have source multiplicity [1]"},
		controlNodeWant{CodeControlNodeOutgoingMultiplicity, 7, "source multiplicity [2..3]"},
		controlNodeWant{CodeControlNodeIncomingMultiplicity, 9,
			"succession into join j has target multiplicity [0..*]; successions into a join node must have target multiplicity [1]"},
		controlNodeWant{CodeControlNodeOutgoingMultiplicity, 11,
			"succession out of join j has source multiplicity [0..1]; successions out of a join node must have source multiplicity [1]"})
}

// A specialized action inherits the successions of its general action. One it
// adds into an inherited fork is reported once, at the succession it adds; the
// general action, whose graph is well formed, stays silent.
func TestControlNodeInheritedSuccessionsCount(t *testing.T) {
	wantControlNodeErrors(t, `package P {
	action def Base {
		action a; action b; action c;
		fork f;
		succession s first a then f;
		first f then b;
		first f then c;
	}
	action def Derived :> Base {
		action d;
		first d then f;
	}
	action def Derived2 :> Base {
		action d;
		succession s2 first d then f;
	}
	action usage : Base {
		action d;
		then f;
	}
}`, controlNodeWant{CodeForkIncomingSuccessions, 11, "fork f has 2 incoming successions"},
		controlNodeWant{CodeForkIncomingSuccessions, 15, "fork f has 2 incoming successions"},
		controlNodeWant{CodeForkIncomingSuccessions, 19, "fork f has 2 incoming successions"})
}

// Redefining an inherited succession replaces it, so the count stays at one;
// redefining one of its ends does not drop it.
func TestControlNodeRedefinedSuccessionReplacesInherited(t *testing.T) {
	wantControlNodesClean(t, `package P {
	action def Base {
		action a; action b; action c;
		fork f;
		succession s first a then f;
		first f then b;
		first f then c;
	}
	action def Derived :> Base {
		succession :>> s first b then f;
	}
	action def Derived2 :> Base {
		action :>> a;
	}
}`)
	wantControlNodeErrors(t, `package P {
	action def Base {
		action a; action b; action c;
		fork f;
		succession s first a then f;
		first f then b;
	}
	action def Derived :> Base {
		action :>> a;
		first c then f;
	}
}`, controlNodeWant{CodeForkIncomingSuccessions, 10, "fork f has 2 incoming successions"})
}

// A violation the general action declares is reported there, once, not again in
// every action that specializes it.
func TestControlNodeInheritedViolationReportedOnce(t *testing.T) {
	wantControlNodeErrors(t, `package P {
	action def Base {
		action a; action b;
		join j;
		first a then j;
		first j then a;
		first j then b;
	}
	action def Derived :> Base {
		action c;
		first j then c;
	}
	action def Derived2 :> Base {
		action c;
	}
}`, controlNodeWant{CodeJoinOutgoingSuccessions, 4, "join j has 2 outgoing successions"},
		controlNodeWant{CodeJoinOutgoingSuccessions, 11, "join j has 3 outgoing successions"})
}

// A control node's owner must be an action definition or usage; the grammar
// admits one in a constraint body (a calculation body), and the parser also in
// an occurrence body and in a succession's body, where it is not.
func TestControlNodeOwningType(t *testing.T) {
	wantControlNodeErrors(t, `package P {
	occurrence def O {
		fork f;
	}
	action def A {
		action a; action b;
		first a then b { decide d; }
	}
	constraint def C {
		join j;
		true
	}
	part def Q {
		constraint c {
			merge m;
		}
	}
}`, controlNodeWant{CodeControlNodeOwner, 3,
		"fork f is declared in occurrence def O, which is not an action; declare it in the body of an action definition or usage"},
		controlNodeWant{CodeControlNodeOwner, 7,
			"decide d is declared in the body of a succession, which is not an action"},
		controlNodeWant{CodeControlNodeOwner, 10,
			"join j is declared in constraint def C, which is not an action"},
		controlNodeWant{CodeControlNodeOwner, 15,
			"merge m is declared in constraint c, which is not an action"})
}

// The runtime rejects a join or merge with several successors when it runs
// them; the static rule reports the same graphs before anything runs.
func TestControlNodeStaticRuleAgreesWithRuntime(t *testing.T) {
	wantControlNodeErrors(t, `package P {
	action def A {
		action a; action b; action c;
		join j;
		first a then j;
		first j then b;
		first j then c;
		merge m;
		first b then m;
		first m then a;
		first m then c;
	}
}`, controlNodeWant{CodeJoinOutgoingSuccessions, 4, "join j has 2 outgoing successions"},
		controlNodeWant{CodeMergeOutgoingSuccessions, 8, "merge m has 2 outgoing successions"})
}

// The rule reaches the CLI and LSP through the shared registry.
func TestControlNodeRuleIsRegistered(t *testing.T) {
	src := `package P {
	action def A {
		action a; action b;
		fork f;
		first a then f;
		first b then f;
	}
}`
	sf := source.New("t.sysml", []byte(src))
	p := parser.New(sf)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("unexpected parse diagnostics: %+v", p.Diagnostics)
	}
	for _, d := range Analyze("t.sysml", root, nil, newTestIndexFromDoc("t.sysml", root)) {
		if d.Code == CodeForkIncomingSuccessions {
			return
		}
	}
	t.Fatal("Analyze did not report the fork's incoming successions")
}
