package passes

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// filterDiags returns the element-filter findings of src.
func filterDiags(t *testing.T, src string) []Diagnostic {
	t.Helper()
	root := parser.New(source.New("<t>", []byte(src))).ParseFile()
	idx := symbols.NewIndex()
	idx.AddDocument("<t>", root)
	idx.ExpandWildcardImports()

	var out []Diagnostic
	for _, d := range Analyze("<t>", root, nil, idx) {
		if strings.HasPrefix(d.Code, "filter-") {
			out = append(out, d)
		}
	}
	return out
}

// only returns the findings with one code.
func only(diags []Diagnostic, code string) []Diagnostic {
	var out []Diagnostic
	for _, d := range diags {
		if d.Code == code {
			out = append(out, d)
		}
	}
	return out
}

const filterMetadata = `metadata def Safety { attribute level; }
part def Belt;
`

// A filter condition is a predicate, so a condition yielding anything else can
// never select an element and is an error (KerML 8.2.4).
func TestFilterNotBooleanIsReported(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
	}{
		{"a namespace filter", filterMetadata + "package P { public import Belt; filter 3; }"},
		{"an import filter", filterMetadata + "package P { public import Belt[42]; }"},
		{"an operand of a boolean operator", filterMetadata + "package P { filter @Safety and 7; }"},
		{"an element reference where a truth value is needed", filterMetadata + "package P { filter Safety implies @Safety; }"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			diags := only(filterDiags(t, tc.src), "filter-not-boolean")
			if len(diags) != 1 {
				t.Fatalf("got %d filter-not-boolean diagnostics, want 1: %v", len(diags), diags)
			}
			if diags[0].Severity != SeverityError {
				t.Errorf("severity = %v, want an error", diags[0].Severity)
			}
			if diags[0].Span.Len == 0 {
				t.Errorf("the diagnostic has no span: %v", diags[0])
			}
			if !strings.Contains(diags[0].Message, "boolean-valued") {
				t.Errorf("message = %q, want it to say the condition must be boolean-valued", diags[0].Message)
			}
		})
	}
}

// A condition outside the subset the evaluator decides is reported and not
// applied, which keeps every candidate: hiding model content on a verdict that
// was never reached would be worse than surfacing an element a filter meant to
// leave out. So it is a warning, not an error.
func TestFilterNotEvaluableIsReported(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
	}{
		{"an unsupported operator", filterMetadata + "package P { filter @Safety + 1; }"},
		{"an indexed condition", filterMetadata + "package P { filter @Safety[1]; }"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			diags := only(filterDiags(t, tc.src), "filter-not-evaluable")
			if len(diags) == 0 {
				t.Fatalf("the condition is not evaluable but nothing was reported")
			}
			if diags[0].Severity != SeverityWarning {
				t.Errorf("severity = %v, want a warning", diags[0].Severity)
			}
			if diags[0].Span.Len == 0 {
				t.Errorf("the diagnostic has no span: %v", diags[0])
			}
			if diags[0].Message == "" || !strings.Contains(diags[0].Message, "cannot be evaluated") {
				t.Errorf("message = %q, want it to say the condition cannot be evaluated", diags[0].Message)
			}
		})
	}
}

// The conditions filters are actually written with are reported by neither: a
// classification, its boolean composition, and a comparison of an annotation
// feature the evaluator reads once a candidate is at hand.
func TestFilterConditionsInTheSupportedSubsetAreClean(t *testing.T) {
	src := filterMetadata + `package P {
		public import Belt;
		filter @Safety;
		filter not @Safety or (@Safety and Safety::level >= 3);
		filter @@Safety implies @Safety;
	}
	package Q { public import P::*[@Safety and Safety::level == 3]; }`

	if diags := filterDiags(t, src); len(diags) != 0 {
		t.Fatalf("supported filter conditions were reported: %v", diags)
	}
}
