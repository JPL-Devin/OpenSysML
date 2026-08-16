package resolve

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
)

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
	if _, ok := r.Endpoint(nil, qn); ok {
		t.Error("an endpoint naming nothing resolved")
	}
	if len(r.Diagnostics) != 0 {
		t.Errorf("a lookup made for lowering reported: %v", r.Diagnostics)
	}
}
