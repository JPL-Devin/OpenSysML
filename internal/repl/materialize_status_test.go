package repl

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/runtime"
)

// unmaterializableModel binds two values to a feature declaring no multiplicity,
// so reading the slot finds a default that does not conform to 1..1.
const unmaterializableModel = `package Demo { attribute def X { attribute bad : ScalarValues::Real = (1.0, 2.0); }
               part def R { attribute b : X; } }
`

// conformingModel reads clean, so nothing about it is left unanswered.
const conformingModel = `package Demo { part def R { attribute b : ScalarValues::Real = 1.0; } }
`

func submitted(t *testing.T, src string) *Session {
	t.Helper()
	s := NewSession()
	if res := s.Submit(src); len(res.Diagnostics) > 0 {
		t.Fatalf("model has diagnostics: %v", res.Diagnostics)
	}
	return s
}

// A slot a command could not materialize is a finding about the model, so it
// reaches the session status a non-interactive run exits on — as the typed error
// the runtime reported, not as the rendering the command printed.
func TestSlotsCarriesMaterializationFailureIntoStatus(t *testing.T) {
	s := submitted(t, unmaterializableModel)
	if s.HasErrors() {
		t.Fatalf("a model that analyses clean has errors before any command: %v", s.DiagnosticLines())
	}

	run(t, s, "%instantiate Demo::R")
	wants(t, run(t, s, "%slots Demo::R"), "bad: <error:", "multiplicity violation")

	if !s.HasErrors() {
		t.Error("a rendered materialization failure did not reach the session status")
	}
	failures := s.MaterializationFailures()
	if len(failures) != 1 {
		t.Fatalf("materialization failures = %v, want one", failures)
	}
	if !errors.Is(failures[0], runtime.ErrMultiplicityViolation) {
		t.Errorf("failure %v is not the multiplicity violation the runtime reported", failures[0])
	}
}

// %eval reads a slot too, so a value it could not materialize is the same finding.
func TestEvalCarriesMaterializationFailureIntoStatus(t *testing.T) {
	s := submitted(t, unmaterializableModel)
	wants(t, run(t, s, "%instantiate Demo::R"), "Created instance")

	wants(t, run(t, s, "%eval bad"), "error: evaluation failed", "multiplicity violation")
	if !s.HasErrors() {
		t.Error("a failed evaluation of a slot did not reach the session status")
	}
	if got := s.MaterializationFailures(); len(got) == 0 ||
		!errors.Is(got[0], runtime.ErrMultiplicityViolation) {
		t.Errorf("materialization failures = %v, want the multiplicity violation", got)
	}
}

// A model whose slots all materialize leaves the session with nothing unanswered.
func TestSlotsOfAConformingModelLeaveNoFailure(t *testing.T) {
	s := submitted(t, conformingModel)
	run(t, s, "%instantiate Demo::R")
	wants(t, run(t, s, "%slots Demo::R"), "b = 1")

	if s.HasErrors() {
		t.Errorf("a conforming model was reported wrong: %v", s.MaterializationFailures())
	}
	if got := s.MaterializationFailures(); len(got) != 0 {
		t.Errorf("materialization failures = %v, want none", got)
	}
}

// The record is of what the session answered, so a later command about a
// different object does not clear it.
func TestMaterializationFailureStandsAfterALaterCommand(t *testing.T) {
	s := submitted(t, unmaterializableModel)
	run(t, s, "%instantiate Demo::R")
	run(t, s, "%slots Demo::R")
	run(t, s, "%list")

	if !s.HasErrors() {
		t.Error("a later command cleared what the session could not answer")
	}
}

// The prompt is unaffected: the rendering is printed and the loop goes on taking
// lines, so a user sees the error and keeps working.
func TestPromptContinuesAfterAMaterializationFailure(t *testing.T) {
	s := submitted(t, unmaterializableModel)
	var out strings.Builder
	if err := Loop(&scriptReader{lines: []string{"%instantiate Demo::R", "%slots Demo::R", "%eval 1 + 1"}}, &out, s); err != nil {
		t.Fatalf("Loop error: %v", err)
	}

	got := out.String()
	wants(t, got, "bad: <error:", "multiplicity violation", "= 2")
	if !s.HasErrors() {
		t.Error("the failure the prompt rendered did not reach the session status")
	}
}

// A load reports what analysis found, so a command's materialization failure does
// not turn an earlier load into a failed one.
func TestLoadReportsAnalysisAlone(t *testing.T) {
	s := submitted(t, unmaterializableModel)
	run(t, s, "%instantiate Demo::R")
	run(t, s, "%slots Demo::R")

	path := filepath.Join(t.TempDir(), "model.sysml")
	if err := os.WriteFile(path, []byte(conformingModel), 0o600); err != nil {
		t.Fatal(err)
	}
	report, err := s.LoadPathsReport([]string{path})
	if err != nil {
		t.Fatalf("LoadPathsReport: %v", err)
	}
	if report.Errors {
		t.Error("a load of a model that analyses clean was reported as failed")
	}
}
