package passes

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// filterDiags returns the element-filter findings of src.
func filterDiags(t *testing.T, src string) []Diagnostic {
	t.Helper()
	root := parser.New(source.New("<t>", []byte(src))).ParseFile()
	idx := newTestIndex()
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

// except returns the findings without one code, for tests whose subject is a
// different rule.
func except(diags []Diagnostic, code string) []Diagnostic {
	var out []Diagnostic
	for _, d := range diags {
		if d.Code != code {
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
			if diags[0].Message != msgFilterNotBoolean {
				t.Errorf("message = %q, want %q", diags[0].Message, msgFilterNotBoolean)
			}
		})
	}
}

// A condition outside the subset the evaluator decides is an error, as it is in
// the reference: it selects nothing, so the filter the model asked for is not
// the one it got.
func TestFilterNotEvaluableIsReported(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
	}{
		{"an indexed condition", filterMetadata + "package P { filter @Safety[1]; }"},
		{"an invocation", filterMetadata + "package P { filter coll->select(x); }"},
		{"a Boolean-result operator", filterMetadata + "package P { filter ~(1 as Integer); }"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			all := filterDiags(t, tc.src)
			diags := only(all, "filter-not-evaluable")
			if len(diags) == 0 {
				t.Fatalf("the condition is not evaluable but nothing was reported")
			}
			if got := len(only(all, "filter-not-boolean")); got != 0 {
				t.Fatalf("got %d filter-not-boolean diagnostics, want none: %v", got, all)
			}
			if diags[0].Severity != SeverityError {
				t.Errorf("severity = %v, want an error", diags[0].Severity)
			}
			if diags[0].Span.Len == 0 {
				t.Errorf("the diagnostic has no span: %v", diags[0])
			}
			if diags[0].Message != msgFilterNotEvaluable {
				t.Errorf("message = %q, want %q", diags[0].Message, msgFilterNotEvaluable)
			}
		})
	}
}

func TestUnsupportedFilterResultReportsBooleanRule(t *testing.T) {
	for _, tc := range []struct {
		name string
		cond string
	}{
		{"an unsupported operator", "@Safety + 1"},
		{"a feature chain", "a.b.c"},
		{"a conditional", "if c ? a else b"},
		{"a cast", "x as Integer"},
		{"an unresolved reference", "Undefined"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			diags := filterDiags(t, filterMetadata+"package P { filter "+tc.cond+"; }")
			if got := len(only(diags, "filter-not-boolean")); got != 1 {
				t.Fatalf("got %d filter-not-boolean diagnostics, want one: %v", got, diags)
			}
			if got := len(only(diags, "filter-not-evaluable")); got != 0 {
				t.Fatalf("got %d filter-not-evaluable diagnostics, want none: %v", got, diags)
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

// A constructor is model-level evaluable, so a filter built from one is reported
// for yielding an instance rather than a truth value (KerML 7.4.9, 8.2.4).
func TestFilterConstructorIsNotBooleanRatherThanInevaluable(t *testing.T) {
	src := filterMetadata + `package P { part def A { attribute n; }
		filter new A(null, 1, "", false); }`
	diags := filterDiags(t, src)
	if len(only(diags, "filter-not-boolean")) != 1 || len(only(diags, "filter-not-evaluable")) != 0 {
		t.Fatalf("want one filter-not-boolean and no filter-not-evaluable, got %v", diags)
	}
}

// The two faults are stated on the membership carrying the condition, once each,
// however many operands share the fault.
func TestFilterReportsEachFaultOnceOnTheMembership(t *testing.T) {
	src := filterMetadata + "package P { filter (3 + @Safety) and (4 + @Safety); }"
	diags := filterDiags(t, src)
	if len(only(diags, "filter-not-evaluable")) != 1 {
		t.Fatalf("want one filter-not-evaluable, got %v", diags)
	}
	for _, d := range diags {
		if d.Span.Len == 0 {
			t.Errorf("the diagnostic has no span: %v", d)
		}
	}
}
