package repl

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/passes"
)

// A source that analyses to exactly one warning and no error.
const warningSrc = `package W { attribute flag = 1 == "one"; }`

func TestWarningSourceIsAWarning(t *testing.T) {
	r := NewSession().Submit(warningSrc)
	if len(r.Diagnostics) != 1 || r.Diagnostics[0].Severity != passes.SeverityWarning {
		t.Fatalf("fixture is not a single warning: %v", r.Diagnostics)
	}
}

// A submission that analyses clean is confirmed even when an earlier
// submission left an error in the buffer, and that error is not re-echoed.
func TestEarlierErrorDoesNotSuppressThisSubmission(t *testing.T) {
	s := NewSession()
	s.Submit("namespace N { import Missing::X; }")
	got := strings.Join(renderResult(s.Submit("package P { }"), VerbosityNormal), "\n")

	wants(t, got, "package P")
	rejects(t, got, "Missing::X")
}

// The summary covers what this submission declared, not the whole buffer.
func TestSummaryCoversOnlyThisSubmission(t *testing.T) {
	s := NewSession()
	s.Submit("package Earlier { }")
	got := strings.Join(renderResult(s.Submit("package P { }"), VerbosityNormal), "\n")

	wants(t, got, "package P")
	rejects(t, got, "package Earlier")
}

// Positions are reported against what the user typed, not against the buffer
// the submission was appended to.
func TestDiagnosticLinesAreRelativeToTheSubmission(t *testing.T) {
	s := NewSession()
	s.Submit("package Earlier {\n}\n")
	r := s.Submit("namespace N {\n\timport Missing::X;\n}")
	got := strings.Join(renderResult(r, VerbosityNormal), "\n")

	wants(t, got, "2:", "Missing::X")
	rejects(t, got, "5:")
}

// Quiet drops warnings; normal keeps them. Neither drops an error.
func TestVerbosityFiltersBySeverity(t *testing.T) {
	quiet := strings.Join(renderResult(NewSession().Submit(warningSrc), VerbosityQuiet), "\n")
	rejects(t, quiet, "warning:")
	wants(t, quiet, "package W")

	normal := strings.Join(renderResult(NewSession().Submit(warningSrc), VerbosityNormal), "\n")
	wants(t, normal, "warning:", "package W")

	errored := strings.Join(renderResult(NewSession().Submit("namespace N { import Missing::X; }"), VerbosityQuiet), "\n")
	wants(t, errored, "error:")
}

// Debug reports the whole buffer, at buffer-absolute positions, naming the pass
// behind each diagnostic.
func TestDebugReportsTheWholeBuffer(t *testing.T) {
	s := NewSession()
	s.Submit("namespace N {\n\timport Missing::X;\n}")
	got := strings.Join(renderResult(s.Submit("package P { }"), VerbosityDebug), "\n")

	wants(t, got, "[debug]", "Missing::X", "2:")
	if !strings.Contains(got, "[nameres") && !strings.Contains(got, "[name") {
		t.Errorf("debug output does not name the pass:\n%s", got)
	}
}

func TestVerbosityMetaCommand(t *testing.T) {
	s := NewSession()
	wants(t, run(t, s, "%verbosity"), "verbosity: normal")
	wants(t, run(t, s, "%verbosity debug"), "verbosity: debug")
	if s.Verbosity() != VerbosityDebug {
		t.Errorf("verbosity = %v, want debug", s.Verbosity())
	}
	if _, _, err := s.RunMeta("%verbosity loud"); err == nil {
		t.Error("an unknown level was accepted")
	}
}
