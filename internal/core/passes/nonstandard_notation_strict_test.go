package passes

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/conformance"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// extensionInventory is every construct the audit classifies as OpenSysML
// notation: the pass must report each one in either mode.
var extensionInventory = []string{
	"state def S { initial a; }",
	"state def S { final b; }",
	"state def S { choice c; }",
	"state def S { junction j; }",
	"state def S { history h; }",
	"state def S { shallow history h; }",
	"state def S { deep history h; }",
	"state def S { region r { state a; } }",
	"state def S { entry point p; }",
	"state def S { exit point p; }",
	"state def S { state a { defer e; } }",
	"state def S { state a; state b; transition a to b; }",
	"action def A { initial a; }",
	"action def A { final b; }",
	"action def A { decision d; }",
}

// notationDiags runs the pass over a document in the named mode.
func notationDiags(t *testing.T, name, src string, mode conformance.Mode) []Diagnostic {
	t.Helper()
	root, pd, idx := analyzeInputs(t, name, src)
	if len(pd) != 0 {
		t.Fatalf("%s: the notation must stay parsed in either mode, got %+v", src, pd)
	}
	ctx := NewContextWithOptions(name, source.KindOf(name), idx, pd, Options{Conformance: mode})
	return NonstandardNotationPass{}.Run(ctx, name, root)
}

// Strict mode changes what a finding weighs, never whether there is one: the
// same constructs are reported, as errors.
func TestExtensionInventoryIsAnErrorUnderStrictMode(t *testing.T) {
	for _, src := range extensionInventory {
		strict := notationDiags(t, "a.sysml", src, conformance.ModeStrict)
		def := notationDiags(t, "a.sysml", src, conformance.ModeDefault)
		if len(strict) == 0 || len(strict) != len(def) {
			t.Fatalf("%s: strict gave %d finding(s), default %d; want the same, non-zero count", src, len(strict), len(def))
		}
		for i, d := range strict {
			if d.Severity != SeverityError {
				t.Errorf("%s: strict severity = %v, want error", src, d.Severity)
			}
			if def[i].Severity != SeverityWarning {
				t.Errorf("%s: default severity = %v, want warning", src, def[i].Severity)
			}
			if d.Code != CodeNonstandardNotation || d.Code != def[i].Code {
				t.Errorf("%s: strict code = %q, default %q, want %q", src, d.Code, def[i].Code, CodeNonstandardNotation)
			}
			if d.Message != def[i].Message || d.Span != def[i].Span {
				t.Errorf("%s: strict mode must change only the severity, got %+v vs %+v", src, d, def[i])
			}
		}
	}
}

// KerML notation in a SysML file is non-conforming SysML too, so strict mode
// errors on it and a KerML file stays silent in either mode.
func TestKerMLNotationFollowsTheMode(t *testing.T) {
	const src = "package P { namespace N; }"
	for _, d := range notationDiags(t, "a.sysml", src, conformance.ModeStrict) {
		if d.Severity != SeverityError || d.Code != CodeKerMLNotation {
			t.Errorf("strict: got %+v, want a kerml-notation error", d)
		}
	}
	if got := notationDiags(t, "a.kerml", src, conformance.ModeStrict); len(got) != 0 {
		t.Errorf("a KerML file uses KerML notation: got %+v, want silence", got)
	}
}

// Standard notation must stay silent under strict mode: a false rejection of
// conforming notation would be the worse defect.
func TestStandardNotationIsSilentUnderStrictMode(t *testing.T) {
	for _, src := range []string{
		"package P { part def Widget; }",
		"state def S { entry; then a; state a; }",
		"state s parallel { state a; state b; }",
		"state def S { state a; state b; transition first a then b; }",
		"action def A { first start; then done; }",
		"action def A { fork f; join j; merge m; decide d; }",
		"package P { attribute done : Boolean; attribute region : Boolean; }",
	} {
		if got := notationDiags(t, "a.sysml", src, conformance.ModeStrict); len(got) != 0 {
			t.Errorf("%s: got %+v, want silence under strict mode", src, got)
		}
	}
}

// The strict severity is chosen by the mode alone, so an unnamed mode is the
// default one.
func TestNotationSeverity(t *testing.T) {
	if got := notationSeverity(conformance.ModeStrict); got != SeverityError {
		t.Errorf("strict severity = %v, want error", got)
	}
	if got := notationSeverity(conformance.ModeDefault); got != SeverityWarning {
		t.Errorf("default severity = %v, want warning", got)
	}
}
