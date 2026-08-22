package passes

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// w8dDiags analyses src with the default registry, as the CLI does.
func w8dDiags(t *testing.T, src string) []Diagnostic {
	t.Helper()
	root := parser.New(source.New("<t>", []byte(src))).ParseFile()
	idx := symbols.NewIndex()
	idx.AddDocument("<t>", root)
	return Analyze("<t>", root, nil, idx)
}

// w8dLine returns the 1-based line of a span in src.
func w8dLine(src string, span source.Span) int {
	return strings.Count(src[:span.Offset], "\n") + 1
}

// w8dLines returns the 1-based lines carrying a diagnostic with code.
func w8dLines(t *testing.T, src, code string) []int {
	t.Helper()
	var lines []int
	for _, d := range only(w8dDiags(t, src), code) {
		lines = append(lines, w8dLine(src, d.Span))
	}
	return lines
}

func w8dWantLines(t *testing.T, src, code string, want ...int) {
	t.Helper()
	got := w8dLines(t, src, code)
	if len(got) != len(want) {
		t.Fatalf("%s: got lines %v, want %v", code, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s: got lines %v, want %v", code, got, want)
		}
	}
}

func TestW8DVariationMemberMustBeVariant(t *testing.T) {
	const src = `package P {
		part def D;
		variation attribute def AttributeChoices {
			variant attribute a1;
			attribute a2;
		}
		part c {
			variation part d : D {
				part d1;
				variant part d2;
			}
		}
	}`
	w8dWantLines(t, src, "variation-member-not-variant", 5, 9)
}

func TestW8DVariationMustNotSpecializeVariation(t *testing.T) {
	const src = `package P {
		variation attribute def AttributeChoices {
			variant attribute a1;
		}
		variation attribute def AttributeChoices1 :> AttributeChoices;
		variation attribute a3 : AttributeChoices1 {
			variant attribute a4;
		}
	}`
	w8dWantLines(t, src, "variation-specialization", 5, 6)
}

// A variation whose every usage is a variant and whose general is no variation
// stays silent, as does a variant nested in a variant.
func TestW8DLegalVariationStaysSilent(t *testing.T) {
	const src = `package P {
		part def Base;
		attribute def Choice;
		variation part def PartChoices :> Base {
			doc /* the choices */
			variant part p1;
			variant part p2 {
				part inner;
			}
			part def Nested;
		}
		part c {
			variation part d : Base {
				variant part d1;
			}
		}
	}`
	for _, code := range []string{"variation-member-not-variant", "variation-specialization"} {
		if diags := only(w8dDiags(t, src), code); len(diags) != 0 {
			t.Fatalf("legal variation model reported %s: %v", code, diags)
		}
	}
}
