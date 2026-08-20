package passes

import (
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

func TestNamespaceInKerMLIsSilent(t *testing.T) {
	for _, src := range []string{
		"namespace N { }",
		"namespace N;",
		"package P { namespace N; }",
	} {
		wantSilent(t, "a.kerml", src)
	}
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
