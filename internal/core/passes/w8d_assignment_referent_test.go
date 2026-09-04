package passes

import (
	"strings"
	"testing"
)

func assignmentReferentFindings(t *testing.T, src string, warm bool) []Diagnostic {
	t.Helper()
	return assignmentDiags(t, src, warm, "assignment-referent-time-varying")
}

func assignmentDiags(t *testing.T, src string, warm bool, code string) []Diagnostic {
	t.Helper()
	var out []Diagnostic
	for _, diag := range w9cLibraryDiags(t, src, warm) {
		if diag.Code == code {
			out = append(out, diag)
		}
	}
	return out
}

func TestAssignmentReferentMayTimeVary(t *testing.T) {
	src := `package Test {
	item i {
		attribute a;
		action d;
		portion attribute p;
	}
	action def A {
		assign i.a := null;
		assign i.d := null;
		assign i.p := null;
	}
}`
	for _, warm := range []bool{false, true} {
		got := assignmentReferentFindings(t, src, warm)
		if len(got) != 2 {
			t.Fatalf("warm=%v: got %v, want action and portion diagnostics", warm, got)
		}
		want := []int{
			strings.Index(src, "d := null"),
			strings.Index(src, "p := null"),
		}
		for i, diag := range got {
			if diag.Message != msgAssignmentReferentTimeVarying || diag.Span.Offset != want[i] {
				t.Errorf("warm=%v diagnostic %d: got %v, want offset %d", warm, i, diag, want[i])
			}
		}
	}
}

func TestAssignmentReferentIsElementScoped(t *testing.T) {
	src := `package Test {
	item i {
		private attribute hidden;
		action d;
	}
	action def A {
		assign i.hidden := null;
		assign i.d := null;
	}
}`
	for _, warm := range []bool{false, true} {
		got := assignmentReferentFindings(t, src, warm)
		if len(got) != 1 || got[0].Span.Offset != strings.Index(src, "d := null") {
			t.Fatalf("warm=%v: got %v, want only the independent d diagnostic", warm, got)
		}
	}
}

func TestAssignmentReferentUnresolvedOrNonFeatureDoesNotCascade(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{
			name: "unresolved",
			src: `package Test {
				action def A { assign missing := null; }
			}`,
		},
		{
			name: "definition",
			src: `package Test {
				part def P;
				action def A { assign P := null; }
			}`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, warm := range []bool{false, true} {
				if got := assignmentReferentFindings(t, tc.src, warm); len(got) != 0 {
					t.Errorf("warm=%v: got %v, want no cascading time-varying diagnostic", warm, got)
				}
			}
		})
	}
}

// An assignment's referent is the feature its target names (SysML v2 §8.3.16.2,
// validateAssignmentActionUsageReferent): a type or a namespace is none.
func TestAssignmentReferentNonFeatureRejected(t *testing.T) {
	tests := []struct {
		name, target, want string
	}{
		{"part definition", "P", "P is declared `part def`"},
		{"attribute definition", "AD", "AD is declared `attribute def`"},
		{"package", "Q", "Q is declared `package`"},
		{"library datatype", "ScalarValues::Integer", "ScalarValues::Integer is declared `datatype`"},
		{"nested definition", "Q::N", "Q::N is declared `part def`"},
	}
	prefix := "package Test { part def P; attribute def AD; package Q { part def N; } item i { attribute a; } action def A { assign "
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			src := prefix + tc.target + " := null; assign i.a := null; } }"
			for _, warm := range []bool{false, true} {
				all := w9cLibraryDiags(t, src, warm)
				var got []Diagnostic
				for _, d := range all {
					if d.Severity == SeverityError {
						got = append(got, d)
					}
				}
				if len(got) != 1 || got[0].Code != "assignment-referent" {
					t.Fatalf("warm=%v: got %v, want the one referent diagnostic", warm, got)
				}
				wantMsg := msgAssignmentReferent + " " + tc.want + ", not a feature."
				if got[0].Message != wantMsg {
					t.Errorf("warm=%v: got %q, want %q", warm, got[0].Message, wantMsg)
				}
				last := tc.target[strings.LastIndex(tc.target, ":")+1:]
				if got[0].Span.Offset != strings.Index(src, last+" := null") {
					t.Errorf("warm=%v: offset %d, want the target's last segment", warm, got[0].Span.Offset)
				}
			}
		})
	}
}

// A target that resolves to a feature is a referent, whatever else the rest of
// the pass says about it; an unresolved one is the name-resolution tier's alone.
func TestAssignmentReferentFeatureOrUnresolvedIsNotReported(t *testing.T) {
	src := `package Test {
	part def P { attribute x; }
	item i { attribute a; part p : P; action d; }
	action def A {
		assign i.a := null;
		assign i.p := null;
		assign i.p.x := null;
		assign i.d := null;
		assign missing := null;
		assign i.missing := null;
	}
}`
	for _, warm := range []bool{false, true} {
		if got := assignmentDiags(t, src, warm, "assignment-referent"); len(got) != 0 {
			t.Errorf("warm=%v: got %v, want no referent diagnostic", warm, got)
		}
	}
}

func TestAssignmentReferentWithoutOccurrenceOwnerFails(t *testing.T) {
	src := `package Test {
	attribute fixed;
	action def A {
		assign fixed := null;
	}
}`
	for _, warm := range []bool{false, true} {
		got := assignmentReferentFindings(t, src, warm)
		if len(got) != 1 || got[0].Span.Offset != strings.Index(src, "fixed := null") {
			t.Fatalf("warm=%v: got %v, want the package-owned referent diagnostic", warm, got)
		}
	}
}
