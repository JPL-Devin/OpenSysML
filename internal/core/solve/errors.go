package solve

import (
	"errors"
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// ErrNotTranslatable is returned when a condition uses a construct outside the
// translatable subset. A query that cannot encode one conjunct fails as a whole:
// a partial script would answer sat or unsat about conditions it does not hold.
var ErrNotTranslatable = errors.New("not translatable for solving")

// ErrNoConditions is returned for an element that states no condition to
// translate, as evaluating it reports the same.
var ErrNoConditions = errors.New("states no condition")

// ErrNoSolver is returned when no SMT solver could be found to run a query.
// Solving is optional, so its absence is reported rather than passed over.
var ErrNoSolver = errors.New("no SMT solver found")

// ErrSolverProcess is returned when a solver was found but did not answer: it
// crashed, failed, or replied unintelligibly. Never a verdict, not even `unknown`.
var ErrSolverProcess = errors.New("the SMT solver did not answer")

// ErrNoCore is returned when a solver answered unsat but would not report which
// assertions conflict, or named assertions the query never asserted. No core is
// invented in its place.
var ErrNoCore = errors.New("the SMT solver did not report an unsat core")

// CoreError says which solver would not explain its unsat verdict and how. It
// unwraps to both ErrNoCore and ErrSolverProcess, as the solver did not answer
// what it was asked.
type CoreError struct {
	// Solver is the executable that was run.
	Solver string

	// Detail says what it answered instead of a core.
	Detail string

	// Stderr is what the solver wrote on standard error, trimmed.
	Stderr string
}

// Error reports the failure, naming the solver and what it answered.
func (e *CoreError) Error() string {
	msg := fmt.Sprintf("%s: %s answered unsat but %s", ErrNoCore, e.Solver, e.Detail)
	if e.Stderr != "" {
		msg += ": " + e.Stderr
	}
	return msg
}

// Unwrap returns both kinds this failure is, so either is testable with
// errors.Is.
func (e *CoreError) Unwrap() []error { return []error{ErrNoCore, ErrSolverProcess} }

// NoSolverError names the candidates looked for and what to install. It unwraps
// to ErrNoSolver.
type NoSolverError struct {
	// Override is the value of the OPENSYSML_SMT override, empty when unset.
	Override string

	// Looked lists the executables looked for on PATH, in order.
	Looked []string
}

// Error reports that no solver was found, naming what would satisfy the search.
func (e *NoSolverError) Error() string {
	if e.Override != "" {
		return fmt.Sprintf("%s: %s names %q, which is not an executable file",
			ErrNoSolver, SolverEnv, e.Override)
	}
	return fmt.Sprintf("%s: install z3 (`apt install z3`, `brew install z3`) or cvc5, "+
		"or set %s to a solver executable; looked for %v on PATH",
		ErrNoSolver, SolverEnv, e.Looked)
}

// Unwrap returns ErrNoSolver.
func (e *NoSolverError) Unwrap() error { return ErrNoSolver }

// SolverProcessError says which solver failed, at which step, and what it wrote
// on standard error. It unwraps to ErrSolverProcess.
type SolverProcessError struct {
	// Solver is the executable that was run.
	Solver string

	// Stage names what was being done: "start", "write", "check-sat",
	// "get-value" or "exit".
	Stage string

	// Detail says what went wrong.
	Detail string

	// Stderr is what the solver wrote on standard error, trimmed.
	Stderr string

	// Err is the underlying error, nil when the failure was the reply itself.
	Err error
}

// Error reports the failure, naming the solver and the step it failed at.
func (e *SolverProcessError) Error() string {
	msg := fmt.Sprintf("%s: %s failed at %s: %s", ErrSolverProcess, e.Solver, e.Stage, e.Detail)
	if e.Stderr != "" {
		msg += ": " + e.Stderr
	}
	return msg
}

// Unwrap returns ErrSolverProcess, and the underlying error when there is one,
// so both are testable with errors.Is.
func (e *SolverProcessError) Unwrap() []error {
	if e.Err == nil {
		return []error{ErrSolverProcess}
	}
	return []error{ErrSolverProcess, e.Err}
}

// NotTranslatableError says which construct refused, why, and where it was
// written. It unwraps to ErrNotTranslatable.
type NotTranslatableError struct {
	// Construct names the construct that refused, as the notation writes it.
	Construct string

	// Reason says why it is outside the subset.
	Reason string

	// Element is the element whose condition was being translated.
	Element string

	// Condition is the condition as written, empty when the refusal is about a
	// declaration rather than a condition.
	Condition string

	// File is the document the construct was written in, empty when unknown.
	File string

	// Span is where in File it was written.
	Span source.Span

	// Location renders File and Span as `file:line:col`, empty when unknown.
	Location string
}

// Error reports the refusal, naming the construct, the condition it appeared in,
// and where it was written.
func (e *NotTranslatableError) Error() string {
	msg := fmt.Sprintf("%s: %s %s", e.Element, e.Construct, ErrNotTranslatable)
	if e.Reason != "" {
		msg += ": " + e.Reason
	}
	if e.Condition != "" {
		msg += fmt.Sprintf(" (in %s)", e.Condition)
	}
	if e.Location != "" {
		msg += " at " + e.Location
	}
	return msg
}

// Unwrap returns ErrNotTranslatable, so a caller tests the kind of failure
// rather than its text.
func (e *NotTranslatableError) Unwrap() error { return ErrNotTranslatable }
