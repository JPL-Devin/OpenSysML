package passes

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// transitionDiags runs the pass alone, so what it reports is not mixed with what
// another tier reports about the same model.
func transitionDiags(t *testing.T, src string) []Diagnostic {
	t.Helper()
	sf := source.New("t.sysml", []byte(src))
	p := parser.New(sf)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("unexpected parse diagnostics: %+v", p.Diagnostics)
	}
	idx := symbols.NewIndexFromDoc("t.sysml", root)
	return StateTransitionPass{}.Run(NewContext("t.sysml", idx, nil), "t.sysml", root)
}

// analyzeTransitions runs every tier, for a verdict another tier reports.
func analyzeTransitions(t *testing.T, src string) []Diagnostic {
	t.Helper()
	sf := source.New("t.sysml", []byte(src))
	p := parser.New(sf)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("unexpected parse diagnostics: %+v", p.Diagnostics)
	}
	return Analyze("t.sysml", root, nil, symbols.NewIndexFromDoc("t.sysml", root))
}

// wantClean fails when the pass reports anything about a legal model, which is
// the failure mode that breaks models a modeller wrote correctly.
func wantClean(t *testing.T, src string) {
	t.Helper()
	if got := transitionDiags(t, src); len(got) != 0 {
		t.Fatalf("got %+v, want no diagnostics", got)
	}
}

// wantOneError fails unless the pass reports exactly the expected diagnostic.
func wantOneError(t *testing.T, src, code, messagePart string) Diagnostic {
	t.Helper()
	got := transitionDiags(t, src)
	if len(got) != 1 {
		t.Fatalf("got %+v, want exactly one diagnostic", got)
	}
	d := got[0]
	if d.Severity != SeverityError || d.Source != "state-transition" || d.Code != code {
		t.Fatalf("got %+v, want severity=error source=state-transition code=%s", d, code)
	}
	if !strings.Contains(d.Message, messagePart) {
		t.Fatalf("got message %q, want it to contain %q", d.Message, messagePart)
	}
	return d
}

// A vertex of a sibling orthogonal region belongs to the same state machine, so
// a transition crossing regions is legal (UML 2.5.1 §14.2.3.9).
func TestTransitionTargetInSiblingRegionIsLegal(t *testing.T) {
	wantClean(t, `package test {
	state def M {
		state a parallel {
			state r1 {
				initial i1;
				state x;
				i1 then x;
				transition x to y;
			}
			state r2 {
				initial i2;
				state y;
				i2 then y;
			}
		}
	}
}`)
}

// The same crossing written with the `region` keyword, the spelling the runtime
// conformance cases use, is legal too.
func TestTransitionTargetInSiblingRegionKeywordIsLegal(t *testing.T) {
	wantClean(t, `package test {
	state def M {
		attribute crossed;
		initial start;
		state running {
			region left {
				initial lstart;
				state lidle;

				then lstart lidle;
				transition lidle to rtarget;
			}
			region right {
				initial rstart;
				state ridle;
				state rtarget;

				then rstart ridle;
			}
		}

		start then running;
	}
}`)
}

// A vertex of another state machine is not a vertex of this one, so naming it is
// illegal however well the name resolves.
func TestTransitionTargetInUnrelatedMachineIsIllegal(t *testing.T) {
	wantOneError(t, `package test {
	state def Other { initial s; state running; s then running; }
	state def M {
		initial i;
		state busy;
		i then busy;
		transition busy to Other::running;
	}
}`, CodeEndpointNotOfMachine, "Other::running")
}

// The same endpoint written as a succession is the same violation.
func TestSuccessionTargetInUnrelatedMachineIsIllegal(t *testing.T) {
	wantOneError(t, `package test {
	state def Other { initial s; state running; s then running; }
	state def M {
		initial i;
		state busy;
		i then busy;
		busy then Other::running;
	}
}`, CodeEndpointNotOfMachine, "Other::running")
}

// An entry point is a pseudostate the composite state owns, so a transition to
// it is legal (UML 2.5.1 §14.2.3.8).
func TestTransitionToEntryPointIsLegal(t *testing.T) {
	wantClean(t, `package test {
	state def M {
		initial i;
		state comp {
			entry point ep;
			initial ci;
			state cs;
			ci then cs;
		}
		i then comp;
		transition comp to comp::ep;
	}
}`)
}

// An attribute is no vertex, so a transition reaching one has no target vertex.
// Endpoint resolution reports this one, so the pass must not report it twice.
func TestTransitionTargetResolvingToNonVertexIsIllegal(t *testing.T) {
	src := `package test {
	state def M {
		attribute count;
		initial i;
		state busy;
		i then busy;
		transition busy to count;
	}
}`
	got := analyzeTransitions(t, src)
	if len(got) != 1 {
		t.Fatalf("got %+v, want exactly one diagnostic", got)
	}
	if got[0].Severity != SeverityError || got[0].Code != "not-a-vertex" {
		t.Fatalf("got %+v, want an error coded not-a-vertex", got[0])
	}
	if len(transitionDiags(t, src)) != 0 {
		t.Fatalf("the pass reported an endpoint name resolution already reported")
	}
}

// A sourceless `accept … then` takes the state it is written in as its source
// (SysML v2 §7.19.3), so it has one, and reporting it would break a legal model.
func TestSourcelessAcceptTransitionIsLegal(t *testing.T) {
	wantClean(t, `package test {
	state def M {
		initial i;
		state busy;
		state done;
		i then busy;
		state idle {
			accept go then done;
		}
	}
}`)
}

// A junction no transition leaves routes a transition reaching it nowhere, which
// no cycle check finds, since the chain reaching it is acyclic.
func TestJunctionChainTerminatingNowhereIsIllegal(t *testing.T) {
	wantOneError(t, `package test {
	state def M {
		initial i;
		state busy;
		junction j;
		i then busy;
		transition busy to j;
	}
}`, CodeNoOutgoingTransition, "junction j has no outgoing transition")
}

// A junction a transition does leave routes onward, so it is legal — the check
// above must rest on the missing transition, not on the junction itself.
func TestJunctionWithOutgoingTransitionIsLegal(t *testing.T) {
	wantClean(t, `package test {
	state def M {
		initial i;
		state busy;
		state done;
		junction j;
		i then busy;
		transition busy to j;
		transition j to done;
	}
}`)
}

// Sibling regions may declare same-named junctions, so a dead end is reported by
// the declaration it is: one region's `pick then …` says nothing about the other's.
func TestDeadEndJunctionIsReportedBesideASameNamedOneThatRoutes(t *testing.T) {
	wantOneError(t, `package test {
	state def M {
		state both {
			region left {
				state lidle;
				state ldone;
				junction pick;
				lidle then pick;
				pick then ldone;
			}
			region right {
				state ridle;
				junction pick;
				ridle then pick;
			}
		}
	}
}`, CodeNoOutgoingTransition, "junction pick has no outgoing transition")
}

// A succession is an outgoing transition too, so a junction one leaves is legal.
func TestJunctionLeftBySuccessionIsLegal(t *testing.T) {
	wantClean(t, `package test {
	state def M {
		initial i;
		state busy;
		state done;
		junction j;
		i then busy;
		transition busy to j;
		j then done;
	}
}`)
}

// Handed over from #205: a named `first`/`then` marker is not a vertex, and UML
// 2.5.1 §15.7.18 gives the initial pseudostate it stands for no incoming
// transition — so this reports at check time rather than at executor construction.
func TestTransitionToFirstMarkerIsIllegal(t *testing.T) {
	wantOneError(t, `package test {
	state def M {
		initial i;
		state busy;
		state other;
		first marker then other;
		i then busy;
		transition busy to marker;
	}
}`, CodeEndpointNotOfMachine, "marker")
}

// A final state is a vertex, so a transition to one is legal.
func TestTransitionToFinalStateIsLegal(t *testing.T) {
	wantClean(t, `package test {
	state def M {
		initial i;
		state busy;
		final done;
		i then busy;
		transition busy to done;
	}
}`)
}

// A machine written as a usage is checked like one written as a definition.
func TestStateUsageMachineIsChecked(t *testing.T) {
	wantOneError(t, `package test {
	state def Other { initial s; state running; s then running; }
	state machine {
		initial i;
		state busy;
		i then busy;
		transition busy to Other::running;
	}
}`, CodeEndpointNotOfMachine, "Other::running")
}
