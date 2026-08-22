package passes

import "testing"

// A reference subsetting carries the referenced feature's type along, so a
// referencing usage inherits from it as well as from its own kind's library
// definition.
func TestW10BReferenceSubsettingContributesABase(t *testing.T) {
	src := `package Test {
	part def ABlock { part x; }
	part def B {
		action a: ABlock;
	}
	ref b : B;
	perform b.a;
}`
	var at []source0
	for _, d := range w8cLibraryDiagnostics(t, "t.sysml", src) {
		if d.Message == "Duplicate of inherited member name 'self' from Action, Part" {
			at = append(at, source0{d.Span.Offset, d.Severity})
		}
	}
	if len(at) != 2 {
		t.Fatalf("want the warning on both `action a` and `perform b.a`, got %v", at)
	}
	for _, a := range at {
		if a.severity != SeverityWarning {
			t.Errorf("want a warning, got %v", a.severity)
		}
	}
	perform := offsetOfW10B(src, "perform b.a;")
	if at[1].offset != perform {
		t.Errorf("want the second warning at offset %d, got %d", perform, at[1].offset)
	}
}

type source0 struct {
	offset   int
	severity Severity
}

func offsetOfW10B(src, want string) int {
	for i := 0; i+len(want) <= len(src); i++ {
		if src[i:i+len(want)] == want {
			return i
		}
	}
	return -1
}
