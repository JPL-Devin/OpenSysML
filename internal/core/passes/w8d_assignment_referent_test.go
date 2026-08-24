package passes

import (
	"strings"
	"testing"
)

func assignmentReferentFindings(t *testing.T, src string, warm bool) []Diagnostic {
	t.Helper()
	var out []Diagnostic
	for _, diag := range w9cLibraryDiags(t, src, warm) {
		if diag.Code == "assignment-referent-time-varying" {
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
