package repl

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/conformance"
	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
)

// stateExtension is notation of ours: a warning at the prompt by default, an
// error when the prompt is asked strictly.
const stateExtension = "state def S { initial a; state a; }\n"

func TestStrictMetaCommandReportsAndSetsTheMode(t *testing.T) {
	s := NewSession()
	if got := meta(t, s, "%strict"); len(got) == 0 || !strings.Contains(got[0], "off") {
		t.Fatalf("%%strict = %v, want it to report off", got)
	}
	if got := meta(t, s, "%strict on"); len(got) == 0 || !strings.Contains(got[0], "on") {
		t.Fatalf("%%strict on = %v, want it to report on", got)
	}
	if s.ConformanceMode() != conformance.ModeStrict {
		t.Fatalf("mode = %v, want strict", s.ConformanceMode())
	}
	if got := meta(t, s, "%strict off"); len(got) == 0 || !strings.Contains(got[0], "off") {
		t.Fatalf("%%strict off = %v, want it to report off", got)
	}
	if s.ConformanceMode() != conformance.ModeDefault {
		t.Fatalf("mode = %v, want default", s.ConformanceMode())
	}
}

func TestStrictMetaCommandRejectsAnUnknownSetting(t *testing.T) {
	s := NewSession()
	got := meta(t, s, "%strict maybe")
	if len(got) != 1 || !strings.Contains(got[0], "error") {
		t.Fatalf("%%strict maybe = %v, want one error line", got)
	}
	if s.ConformanceMode() != conformance.ModeDefault {
		t.Fatal("a rejected setting must leave the mode alone")
	}
}

// The mode decides what the same submission weighs.
func TestStrictModeEscalatesNotationAtThePrompt(t *testing.T) {
	s := NewSession()
	s.Submit(stateExtension)
	if notationErrors(s.Diagnostics()) != 0 {
		t.Fatalf("default mode errored: %v", s.Diagnostics())
	}
	strict := NewSession()
	meta(t, strict, "%strict on")
	strict.Submit(stateExtension)
	if notationErrors(strict.Diagnostics()) == 0 {
		t.Fatalf("strict mode reported no notation error: %v", strict.Diagnostics())
	}
}

// Switching the mode re-reports the buffer instead of serving the other mode's
// verdict from the cache.
func TestStrictMetaCommandRepeatsTheDiagnostics(t *testing.T) {
	s := NewSession()
	s.Submit(stateExtension)
	out := strings.Join(meta(t, s, "%strict on"), "\n")
	if !strings.Contains(out, passes.CodeNonstandardNotation) && !strings.Contains(out, "initial") {
		t.Fatalf("%%strict on = %q, want the buffer's findings restated", out)
	}
}

func meta(t *testing.T, s *Session, line string) []string {
	t.Helper()
	out, quit, err := s.runMeta(line)
	if err != nil || quit {
		t.Fatalf("%s: err=%v quit=%v", line, err, quit)
	}
	return out
}

func notationErrors(diags []passes.Diagnostic) int {
	var n int
	for _, d := range diags {
		if d.Severity == passes.SeverityError && d.Code == passes.CodeNonstandardNotation {
			n++
		}
	}
	return n
}
