package passes

import (
	"fmt"
	"strings"
	"testing"
)

// wantNotation asserts one warning per want, matched by code and message
// substring, and nothing else.
func wantNotation(t *testing.T, name, src, code string, wants ...string) {
	t.Helper()
	root, pd, idx := analyzeInputs(t, name, src)
	if len(pd) != 0 {
		t.Fatalf("%s: extension notation must stay parsed, got errors %+v", src, pd)
	}
	got := NonstandardNotationPass{}.Run(NewContext(name, idx, pd), name, root)
	if len(got) != len(wants) {
		t.Fatalf("%s: got %d diagnostics %+v, want %d", src, len(got), got, len(wants))
	}
	for i, want := range wants {
		if got[i].Severity != SeverityWarning {
			t.Errorf("%s: severity = %v, want warning", src, got[i].Severity)
		}
		if got[i].Code != code {
			t.Errorf("%s: code = %q, want %q", src, got[i].Code, code)
		}
		if !strings.Contains(got[i].Message, want) {
			t.Errorf("%s: message = %q, want it to contain %q", src, got[i].Message, want)
		}
	}
}

// wantSilent asserts a document parses without error and carries no notation
// diagnostic: a source that failed to parse would be silent for the wrong
// reason.
func wantSilent(t *testing.T, name, src string) {
	t.Helper()
	root, pd, idx := analyzeInputs(t, name, src)
	if len(pd) != 0 {
		t.Fatalf("%s: parse errors %+v", src, pd)
	}
	if got := (NonstandardNotationPass{}).Run(NewContext(name, idx, pd), name, root); len(got) != 0 {
		t.Fatalf("%s: got %+v, want no diagnostics", src, got)
	}
}

// `namespace` is KerML-only notation: both its body forms are reported in a
// SysML file, and neither is in a KerML one.
func TestNamespaceInSysMLIsKerMLNotation(t *testing.T) {
	for _, src := range []string{
		"namespace N { }",
		"namespace N;",
		"package P { namespace N; }",
	} {
		wantNotation(t, "a.sysml", src, CodeKerMLNotation, "`namespace` is KerML notation")
	}
}

// A document of no file kind — the REPL and CLI buffer — takes the SysML reading,
// so its `namespace` is reported rather than silently accepted.
func TestNamespaceInAnUnnamedDocumentIsKerMLNotation(t *testing.T) {
	wantNotation(t, "<repl>", "namespace N;", CodeKerMLNotation, "`namespace` is KerML notation")
}

func TestNamespaceInKerMLIsSilent(t *testing.T) {
	for _, src := range []string{
		"namespace N { }",
		"namespace N;",
		"package P { namespace N; }",
	} {
		wantSilent(t, "a.kerml", src)
	}
}

// `featured by` is KerML-only notation: the SysML grammar has no featuring
// clause, so a SysML file is warned and a KerML one is silent.
func TestFeaturedByInSysMLIsKerMLNotation(t *testing.T) {
	const want = "`featured by` is KerML notation"
	wantNotation(t, "a.sysml", "package P { part def A; part x featured by A; }", CodeKerMLNotation, want)
	// A list states one featuring per target, so each is reported.
	wantNotation(t, "a.sysml", "package P { part def A; part def B; part x featured by A, B; }",
		CodeKerMLNotation, want, want)
}

func TestFeaturedByInKerMLIsSilent(t *testing.T) {
	wantSilent(t, "a.kerml", "package P { class A; class B; feature x featured by A, B; }")
}

// The state notation with no production of its own is reported, one warning per
// construct.
func TestStateExtensionsAreReported(t *testing.T) {
	for _, tc := range []struct{ src, want string }{
		{"state def S { initial a; }", "`initial <state>;`"},
		{"state def S { final b; }", "`final <state>;`"},
		{"state def S { choice c; }", "`choice <name>;`"},
		{"state def S { junction j; }", "`junction <name>;`"},
		{"state def S { history h; }", "history"},
		{"state def S { shallow history h; }", "history"},
		{"state def S { deep history h; }", "history"},
		{"state def S { region r { state a; } }", "`region <name> { … }`"},
		{"state def S { entry point p; }", "point"},
		{"state def S { exit point p; }", "point"},
		{"state def S { state a { defer e; } }", "`defer <event>;`"},
	} {
		wantNotation(t, "a.sysml", tc.src, CodeNonstandardNotation, tc.want)
	}
}

// The action node spellings that stand in for a standard one are reported.
func TestActionNodeExtensionSpellingsAreReported(t *testing.T) {
	for _, tc := range []struct{ src, want string }{
		{"action def A { initial a; }", "`initial` as an action node"},
		{"action def A { final b; }", "`final` as an action node"},
		{"action def A { decision d; }", "`decision` as an action node"},
	} {
		wantNotation(t, "a.sysml", tc.src, CodeNonstandardNotation, tc.want)
	}
}

// `transition <source> to <target>;` is ours; the standard form states its ends
// with `first` and `then`.
func TestTransitionToSpellingIsReported(t *testing.T) {
	wantNotation(t, "a.sysml",
		"state def S { state a; state b; transition a to b; }",
		CodeNonstandardNotation, "`transition <source> to <target>;`")
}

func TestComputedReturnExpressionsAreReported(t *testing.T) {
	for _, tc := range []struct {
		name, src string
	}{
		{"literal", "calc c { in a : Real; return 42; }"},
		{"arithmetic", "calc c { in basePrice : Real; return basePrice * 2.0; }"},
		{"invocation", "calc c { in dx : Real; return sqrt(dx*dx); }"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wantNotation(t, "a.sysml", tc.src, CodeNonstandardNotation,
				"`return <expression>;`")
		})
	}
}

func TestReferenceReturnsStaySilent(t *testing.T) {
	for _, tc := range []struct {
		name, src string
	}{
		{"bare_name", "calc c { in a : Real; return a; }"},
		{"qualified_name", "calc c { in a : Real; return P::a; }"},
		{"dotted_feature_chain", "calc c { in a : Real; return a.b; }"},
		{"named_result_parameter", "calc c { return result : Real = 1.0; }"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wantSilent(t, "a.sysml", tc.src)
		})
	}
}

func TestKeywordedConstraintConditionsAreReported(t *testing.T) {
	for _, tc := range []struct {
		name, keyword string
	}{
		{"assert", "assert"},
		{"assume", "assume"},
		{"negated_assert", "assert"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			expr := "x >= 0"
			if tc.name == "negated_assert" {
				expr = "not x >= 0"
			}
			src := fmt.Sprintf("constraint validRange { in x : Real; %s %s; }", tc.keyword, expr)
			wantNotation(t, "a.sysml", src, CodeNonstandardNotation,
				fmt.Sprintf("`%s <expression>;`", tc.keyword))
		})
	}
}

func TestNotationWarningsPointAtTheirKeywords(t *testing.T) {
	src := `calc c {
		in x : Real;
		return x * 2;
	}
	constraint validRange {
		in x : Real;
		assert x >= 0;
	}`
	root, pd, idx := analyzeInputs(t, "a.sysml", src)
	if len(pd) != 0 {
		t.Fatalf("%s: parse errors %+v", src, pd)
	}
	got := (NonstandardNotationPass{}).Run(NewContext("a.sysml", idx, pd), "a.sysml", root)
	if len(got) != 2 {
		t.Fatalf("%s: got %d diagnostics %+v, want 2", src, len(got), got)
	}
	for i, want := range []struct {
		line, length int
	}{
		{3, len("return")},
		{7, len("assert")},
	} {
		if line := strings.Count(src[:got[i].Span.Offset], "\n") + 1; line != want.line {
			t.Errorf("diagnostic %d is on line %d, want %d", i, line, want.line)
		}
		if got[i].Span.Len != want.length {
			t.Errorf("diagnostic %d spans %d bytes, want %d", i, got[i].Span.Len, want.length)
		}
	}
}

func TestStandardConstraintConditionsStaySilent(t *testing.T) {
	for _, tc := range []struct {
		name, src string
	}{
		{"bare_condition", "constraint validRange { in x : Real; x >= 0 }"},
		{"named_constraint_assertion", "part def P { assert constraint c1 : C; }"},
		{"satisfy_assertion", "requirement def R { assert satisfy r by q; }"},
		{"metadata_constraint_assumption", "requirement def R { assume #goal constraint payloadMassLimit; }"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wantSilent(t, "a.sysml", tc.src)
		})
	}
}

// Notation the pinned grammars do define stays silent: a false extension warning
// on standard notation is the worse defect.
func TestStandardNotationIsSilent(t *testing.T) {
	for _, src := range []string{
		"package P { part def Widget; }",
		"state def S { entry; then a; state a; }",
		"state def S { state a { entry action x; do action y; exit action z; } }",
		"state s parallel { state a; state b; }",
		"state def S { state a; state b; transition first a then b; }",
		"state def S { state a; state b; first a then b; }",
		"action def A { first start; then done; }",
		"action def A { fork f; join j; merge m; decide d; }",
		"action def A { action a; action b; succession a then b; }",
		"action def A { action t; action e; if true then t; else e; }",
		"action def A { while true { action a; } }",
		// The words we no longer reserve are ordinary names.
		"package P { attribute done : Boolean; attribute region : Boolean; }",
		"package P { part def final; part def history; part def defer; }",
		"package P { attribute initial = 1; attribute choice = 2; attribute junction = 3; }",
		"package P { attribute deep = 1; attribute shallow = 2; attribute decision = 3; }",
	} {
		wantSilent(t, "a.sysml", src)
	}
}
