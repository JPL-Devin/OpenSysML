package repl

import (
	"errors"
	"fmt"
	"os"
	"slices"

	"github.com/Open-MBEE/Systemica/internal/core/passes"
	"github.com/Open-MBEE/Systemica/internal/core/runtime"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

// errRuntimeInit marks a runtime the session could not create at all, which the
// prompt reports as a command error rather than as a line of output.
var errRuntimeInit = errors.New("runtime init")

// NamedValue is one value a check or a run produced, formatted as the prompt
// prints it: a calculation's result, an action's output, a machine's state.
type NamedValue struct {
	Name  string
	Value string
}

// namedValues lists an executor's results in name order, so two reports of the
// same run read the same way.
func namedValues(results map[string]runtime.Value) []NamedValue {
	if len(results) == 0 {
		return nil
	}
	names := make([]string, 0, len(results))
	for name := range results {
		names = append(names, name)
	}
	slices.Sort(names)
	values := make([]NamedValue, 0, len(names))
	for _, name := range names {
		values = append(values, NamedValue{Name: name, Value: formatValue(results[name])})
	}
	return values
}

// errorLines adapts a command that reports its failures as errors to the
// prompt, which prints them as output.
func errorLines(lines []string, _ []NamedValue, err error) ([]string, bool, error) {
	if err != nil {
		return []string{"error: " + err.Error()}, false, nil
	}
	return lines, false, nil
}

// LoadFile submits the contents of path to the session and returns the lines
// `%load` prints. The error is the file it could not read; a model that read
// but did not analyse cleanly is reported by Diagnostics.
func (s *Session) LoadFile(path string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(expandHome(path))
	if err != nil {
		return nil, readError(path, err)
	}
	return renderResult(s.submit(path, string(data)), s.verbosity), nil
}

// LoadFileSummary submits the contents of path and returns only what it
// declared, without what analysis has found so far. A caller loading a model
// that spans several files reports the analysis once every file is in, a
// reference from one file to another resolving only then.
func (s *Session) LoadFileSummary(path string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(expandHome(path))
	if err != nil {
		return nil, readError(path, err)
	}
	res := s.submit(path, string(data))
	return append(append([]string(nil), res.Notices...), renderSummary(res.ownMembers())...), nil
}

// DiagnosticLines reports the analysis of everything submitted so far as the
// prompt prints it: the source line each finding is on, under a position naming
// the file the finding is in, at the verbosity the session was asked for.
func (s *Session) DiagnosticLines() []string {
	diags := s.Diagnostics()
	if len(diags) == 0 {
		return nil
	}
	var out []string
	start := 0
	for i, sn := range s.snippets {
		end := start + len(sn.src)
		var own []passes.Diagnostic
		for _, d := range diags {
			if d.Span.Offset < start || (d.Span.Offset > end && i != len(s.snippets)-1) {
				continue
			}
			if d.Severity != passes.SeverityError && s.verbosity <= VerbosityQuiet {
				continue
			}
			d.Span.Offset -= start
			own = append(own, d)
		}
		out = append(out, renderDiagnostics(own, sn.src, inFile(sn.origin), s.verbosity >= VerbosityDebug)...)
		start = end + 1 // the newline joined() writes between snippets
	}
	return out
}

// Diagnostics reports the analysis of everything submitted so far.
func (s *Session) Diagnostics() []passes.Diagnostic {
	return s.ws.Diagnostics(docName)
}

// HasErrors reports whether analysis found something that stops the model from
// being run, as against something worth saying about a model that does run.
func (s *Session) HasErrors() bool {
	for _, d := range s.Diagnostics() {
		if d.Severity == passes.SeverityError {
			return true
		}
	}
	return false
}

// Diagnostic is one finding about the session's model, located in it, for a
// caller reporting analysis as data rather than as the prompt's text.
type Diagnostic struct {
	Severity string
	Message  string
	// File is the loaded file the finding is in, empty for typed input, and Line
	// and Column place it within that file rather than within the session buffer.
	File   string
	Line   int
	Column int
	// Pass names what produced the finding, and Code what it found.
	Pass string
	Code string
}

// LocatedDiagnostics reports the analysis of everything submitted so far, each
// finding placed in the submission it is about: the file it was loaded from, at
// the line and column that file has it on.
func (s *Session) LocatedDiagnostics() []Diagnostic {
	diags := s.Diagnostics()
	if len(diags) == 0 {
		return nil
	}
	out := make([]Diagnostic, 0, len(diags))
	for _, d := range diags {
		sn, start := s.snippetAt(d.Span.Offset)
		p := source.New(docName, []byte(sn.src)).Lines().PosAt(d.Span.Offset - start)
		out = append(out, Diagnostic{
			Severity: d.Severity.String(),
			Message:  d.Message,
			File:     sn.origin,
			Line:     p.Line,
			Column:   p.Col,
			Pass:     d.Source,
			Code:     d.Code,
		})
	}
	return out
}

// snippetAt returns the submission a session-buffer offset falls in and the
// offset that submission starts at, joined() being the snippets with a newline
// between them.
func (s *Session) snippetAt(offset int) (snippet, int) {
	start := 0
	for i, sn := range s.snippets {
		end := start + len(sn.src)
		if offset <= end || i == len(s.snippets)-1 {
			return sn, start
		}
		start = end + 1 // the newline joined() writes between snippets
	}
	return snippet{}, 0
}

// EvalExpr evaluates an expression and returns the lines `%eval` prints, with an
// error for one that could not be evaluated.
func (s *Session) EvalExpr(expr string) ([]string, error) {
	lines, err := s.evalExpr(expr)
	if err != nil {
		return nil, err
	}
	return append(s.drainTrace(), lines...), nil
}

// RunCalc invokes a calculation and returns what it computed. invocation is
// what `%calc` takes: a name, optionally followed by its arguments or carrying
// them as `Fall(3, 4)`.
func (s *Session) RunCalc(invocation string) Verdict {
	name, argText := splitCalcArgs(invocation)
	lines, values, err := s.evalCalc(name, argText)
	if err != nil {
		return s.withTrace(unresolvedVerdict(name, err.Error()))
	}
	return s.withTrace(Verdict{Subject: name, Status: VerdictHolds, Lines: lines, Values: values})
}

// RunAction runs an action to completion outside the prompt, on the object
// performer names when it names one. An action that could not be run, or that
// stopped short of completing, is unresolved: it produced no outputs to judge.
func (s *Session) RunAction(name string, performer ...string) Verdict {
	started, err := s.startAction(name, performer)
	if err != nil {
		return s.withTrace(unresolvedVerdict(name, err.Error()))
	}
	lines, values, err := s.continueAction()
	if err != nil {
		return s.withTrace(unresolvedVerdict(name, err.Error()))
	}
	lines = append(started, lines...)
	if state := s.actionExec.executor.State(); state != runtime.StateCompleted {
		lines = append(lines, fmt.Sprintf("error: action %s stopped at %s without completing", name, state))
		return s.withTrace(Verdict{Subject: name, Status: VerdictUnresolved, Lines: lines, Values: values})
	}
	return s.withTrace(Verdict{Subject: name, Status: VerdictHolds, Lines: lines, Values: values})
}

// RunStateMachine starts a state machine outside the prompt, taking only its
// initial transition, which is `%state` alone. The values are the configuration
// the machine settled in.
func (s *Session) RunStateMachine(name string, performer ...string) Verdict {
	return s.runStateMachine(name, nil, performer)
}

// RunStateMachineFor starts a state machine and advances it by duration time
// units, which is `%state` followed by `%advance`. A duration of 0 is a run to
// the current time, dispatching the events already due, as `%advance 0` is.
func (s *Session) RunStateMachineFor(name string, duration float64, performer ...string) Verdict {
	return s.runStateMachine(name, &duration, performer)
}

func (s *Session) runStateMachine(name string, duration *float64, performer []string) Verdict {
	started, err := s.startStateMachine(name, performer)
	if err != nil {
		return s.withTrace(unresolvedVerdict(name, err.Error()))
	}
	lines := started
	if duration != nil {
		advanced, err := s.advanceBy(*duration)
		if err != nil {
			return s.withTrace(unresolvedVerdict(name, err.Error()))
		}
		lines = append(lines, advanced...)
	}
	exec := s.stateExec.executor
	values := []NamedValue{
		{Name: "state", Value: currentStateName(exec)},
		{Name: "time", Value: fmt.Sprintf("%.2f", s.stateExec.now)},
	}
	values = append(values, namedValues(exec.StateData())...)
	return s.withTrace(Verdict{Subject: name, Status: VerdictHolds, Lines: lines, Values: values})
}
