package resolve

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// transitionTargetIn returns the target name of the first transition declared
// in root, the name node name resolution keyed its verdict by.
func transitionTargetIn(t *testing.T, root *ast.RootNamespace) *ast.QualifiedName {
	t.Helper()
	var walk func(members []ast.Node) *ast.QualifiedName
	walk = func(members []ast.Node) *ast.QualifiedName {
		for _, member := range members {
			if membership, ok := member.(*ast.Membership); ok {
				member = membership.Member
			}
			switch n := member.(type) {
			case *ast.TransitionMember:
				return n.Target
			case *ast.Definition:
				if qn := walk(n.Members); qn != nil {
					return qn
				}
			case *ast.Usage:
				if qn := walk(n.Members); qn != nil {
					return qn
				}
			}
		}
		return nil
	}
	qn := walk(root.Members)
	if qn == nil {
		t.Fatal("no transition member found")
	}
	return qn
}

// A transition endpoint naming a vertex of its machine resolves, whether the
// vertex is a sibling, a nested state, a state of a sibling orthogonal region,
// or an entry/exit point of a composite state.
func TestResolveEndpointsThatNameVertices(t *testing.T) {
	cases := map[string]string{
		"sibling": `
			state def M {
				entry; then idle;
				state idle;
				state busy;
				transition idle to busy;
			}
		`,
		"nested state": `
			state def M {
				entry; then outer;
				state outer {
					entry; then inner;
					state inner;
				}
				state done;
				transition outer::inner to done;
			}
		`,
		"unqualified nested state": `
			state def M {
				entry; then outer;
				state outer {
					entry; then inner;
					state inner;
				}
				state done;
				transition inner to done;
			}
		`,
		"sibling orthogonal region": `
			state def M {
				state running {
					region left {
						entry; then lidle;
						state lidle;
					}
					region right {
						entry; then ridle;
						state ridle;
					}
				}
				transition running::left::lidle to running::right::ridle;
			}
		`,
		"entry and exit point": `
			state def M {
				entry; then idle;
				state idle;
				state comp {
					entry point into;
					exit point outOf;
					entry; then working;
					state working;
				}
				state done;
				transition idle to comp::into;
				transition comp::outOf to done;
			}
		`,
		"sourceless accept then": `
			state def M {
				entry; then idle;
				state idle {
					accept Ping then done;
				}
				state done;
			}
		`,
	}

	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			r := resolveDoc(t, "d.sysml", src)
			if len(r.Diagnostics) != 0 {
				t.Fatalf("expected no diagnostics, got %v", r.Diagnostics)
			}
		})
	}
}

// A misspelled endpoint is a name-resolution diagnostic, reported where the name
// is written and suggesting the vertex it was meant to name.
func TestResolveEndpointMisspelledIsReportedWithASuggestion(t *testing.T) {
	r := resolveDoc(t, "d.sysml", `
		state def M {
			entry; then idle;
			state idle;
			state busy;
			transition idle to busyy;
		}
	`)
	if len(r.Diagnostics) != 1 {
		t.Fatalf("expected one diagnostic for the misspelled endpoint, got %v", r.Diagnostics)
	}
	diag := r.Diagnostics[0]
	if !strings.Contains(diag.Message, "busyy") || !strings.Contains(diag.Message, "did you mean busy") {
		t.Errorf("expected an unresolved message suggesting busy, got %q", diag.Message)
	}
	if diag.Code != "unresolved" {
		t.Errorf("expected the unresolved code, got %q", diag.Code)
	}
	if len(diag.Fixes) != 1 || !strings.Contains(diag.Fixes[0].Title, "busy") {
		t.Errorf("expected a fix replacing the endpoint with busy, got %v", diag.Fixes)
	}
}

// A qualified endpoint's fix corrects the spelling of the vertex it names and
// keeps the qualifiers saying which state the vertex lives in.
func TestResolveEndpointMisspelledQualifiedFixKeepsTheQualifier(t *testing.T) {
	src := `state def M {
	entry; then alpha;
	state alpha { entry; then work; state work; }
	state beta { entry; then work; state work; }
	transition beta::workk to alpha;
}`
	r := resolveDoc(t, "d.sysml", src)
	if len(r.Diagnostics) != 1 {
		t.Fatalf("expected one diagnostic, got %v", r.Diagnostics)
	}
	fixes := r.Diagnostics[0].Fixes
	if len(fixes) != 1 {
		t.Fatalf("expected one fix, got %v", fixes)
	}
	edit := fixes[0].Edits[0]
	if got := src[edit.Span.Offset : edit.Span.Offset+edit.Span.Len]; got != "workk" {
		t.Errorf("expected the fix to replace the last segment, it replaces %q", got)
	}
	if edit.NewText != "work" {
		t.Errorf("expected the fix to write work, got %q", edit.NewText)
	}
}

// An endpoint that resolves to something which is not a vertex names no state a
// transition can start or end at, which the name-resolution tier reports.
func TestResolveEndpointNotAVertexIsReported(t *testing.T) {
	r := resolveDoc(t, "d.sysml", `
		state def M {
			attribute count;
			entry; then idle;
			state idle;
			transition idle to count;
		}
	`)
	if len(r.Diagnostics) != 1 {
		t.Fatalf("expected one diagnostic for the non-vertex endpoint, got %v", r.Diagnostics)
	}
	diag := r.Diagnostics[0]
	if diag.Code != CodeNotAVertex {
		t.Errorf("expected the %s code, got %q", CodeNotAVertex, diag.Code)
	}
	if !strings.Contains(diag.Message, "count") {
		t.Errorf("expected the endpoint named in the message, got %q", diag.Message)
	}
}

// Diagnostics belong to the name-resolution tier: the lookup lowering makes
// reports nothing, however it turns out.
func TestEndpointLookupForLoweringReportsNothing(t *testing.T) {
	r := resolveDoc(t, "d.sysml", `
		state def M {
			entry; then idle;
			state idle;
			state busy;
			transition idle to busy;
		}
	`)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %v", r.Diagnostics)
	}
	qn := &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "nowhere"}}}
	_, ok, reported := r.Endpoint(nil, qn)
	if ok {
		t.Error("an endpoint naming nothing resolved")
	}
	if reported {
		t.Error("a failure nothing reported was called reported")
	}
	if len(r.Diagnostics) != 0 {
		t.Errorf("a lookup made for lowering reported: %v", r.Diagnostics)
	}
}

// An endpoint the document's own resolution reported is reported, so lowering
// leaves the edge out rather than reporting it a second time.
func TestEndpointLookupSaysWhatNameResolutionReported(t *testing.T) {
	src := `
		state def M {
			entry; then idle;
			state idle;
			state busy;
			transition idle to nowhere;
		}
	`
	p := parser.New(source.New("d.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	r := New(symbols.NewIndexFromDoc("d.sysml", root))
	r.ResolveDocument("d.sysml", root)

	qn := transitionTargetIn(t, root)
	_, ok, reported := r.Endpoint(nil, qn)
	if ok {
		t.Error("an endpoint naming nothing resolved")
	}
	if !reported {
		t.Errorf("the endpoint was reported by name resolution: %v", r.Diagnostics)
	}
}

// VertexInScope names an endpoint from the scope tree alone, for a machine
// lowering has the tree of but no resolution pass over: the innermost vertex of
// that spelling wins, and nothing outside the machine answers at all.
func TestVertexInScopeNamesTheInnermostVertexAndNothingOutside(t *testing.T) {
	r := resolveDoc(t, "d.sysml", `
		state def M {
			entry; then alpha;
			state alpha {
				entry; then work;
				state work;
			}
			state beta {
				entry; then work;
				state work;
				transition work to done;
			}
			state done;
		}
		state def Other {
			entry; then idle;
			state idle;
		}
	`)
	beta := scopeNamed(t, r, "d.sysml", "beta")
	work := &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "work"}}}
	decl, ok := VertexInScope(beta, work)
	if !ok {
		t.Fatal("work names no vertex from inside beta")
	}
	want := beta.LookupLocalAll("work")
	if len(want) == 0 || want[0].Decl != decl {
		t.Errorf("work resolved to %v, want beta's own work", decl)
	}

	outside := &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "Other"}, {Text: "idle"}}}
	if _, ok := VertexInScope(beta, outside); ok {
		t.Error("a vertex of another machine answered a scope-only lookup")
	}
}
