package repl

import (
	"errors"
	"fmt"

	"github.com/Open-MBEE/Systemica/internal/core/runtime"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// VerdictStatus is what a checked condition answered.
type VerdictStatus int

const (
	// VerdictHolds is a condition the model satisfies.
	VerdictHolds VerdictStatus = iota
	// VerdictFails is a condition the model answered false.
	VerdictFails
	// VerdictUnresolved is a check that was never decided — an unknown name, a
	// session with no declarations, an element stating no assertion, an
	// evaluation that could not be carried out — which is a question about the
	// model, not an answer from it.
	VerdictUnresolved
)

// Verdict is the outcome of one checked constraint, requirement or satisfaction
// assertion. It carries both the status a caller decides on (an exit code, a
// report) and the lines the REPL prints, so the prompt and a non-interactive
// caller always report the same verdict from the same evaluation.
type Verdict struct {
	// Subject is what was checked, spelled as the caller named it.
	Subject string
	Status  VerdictStatus
	Lines   []string
}

// Holds reports whether the checked condition is satisfied.
func (v Verdict) Holds() bool { return v.Status == VerdictHolds }

// WorstStatus is the status a set of verdicts should be judged by: unresolved
// outranks a failure, since a check that was never made says nothing about the
// model. An empty set holds.
func WorstStatus(verdicts []Verdict) VerdictStatus {
	worst := VerdictHolds
	for _, v := range verdicts {
		if v.Status > worst {
			worst = v.Status
		}
	}
	return worst
}

// failedStatus is the status of a condition that did not hold: the model
// answering false, or, for any other evaluation error, a check that was never
// decided — an unbound subject or feature is not evidence against the model.
func failedStatus(err error) VerdictStatus {
	var violation *runtime.ViolationError
	if err == nil || errors.As(err, &violation) || errors.Is(err, runtime.ErrViolated) {
		return VerdictFails
	}
	return VerdictUnresolved
}

// unresolvedVerdict reports a check that could not be made, phrased as the
// prompt reports any command it cannot carry out.
func unresolvedVerdict(subject, msg string) Verdict {
	return Verdict{Subject: subject, Status: VerdictUnresolved, Lines: []string{"error: " + msg}}
}

// withTrace prefixes a verdict with the execution trace its evaluation produced,
// so a caller outside the prompt reports what `%trace on` reports there.
func (s *Session) withTrace(v Verdict) Verdict {
	if trace := s.drainTrace(); len(trace) > 0 {
		v.Lines = append(trace, v.Lines...)
	}
	return v
}

// CheckConstraint evaluates a constraint definition, against the object that
// carries it when one has been created, so the verdict is about concrete values.
func (s *Session) CheckConstraint(name string) Verdict {
	return s.withTrace(s.checkConstraint(name))
}

func (s *Session) checkConstraint(name string) Verdict {
	target, bad := s.resolveCheckTarget(name)
	if bad != nil {
		return *bad
	}

	inst, owner := s.owningInstance(target.fqn)
	passed, err := target.ctx.EvaluateConstraintOn(target.sym, target.scope, inst)
	if err != nil || !passed {
		return Verdict{Subject: name, Status: failedStatus(err), Lines: []string{
			fmt.Sprintf("✗ Constraint %s failed%s", name, onInstance(inst, owner)),
			"  " + verdictDetail("Assertion", err),
		}}
	}
	return Verdict{Subject: name, Status: VerdictHolds, Lines: []string{
		fmt.Sprintf("✓ Constraint %s passed%s", name, onInstance(inst, owner)),
	}}
}

// CheckRequirement evaluates a requirement definition, against the object that
// carries it when one has been created.
func (s *Session) CheckRequirement(name string) Verdict {
	return s.withTrace(s.checkRequirement(name))
}

func (s *Session) checkRequirement(name string) Verdict {
	target, bad := s.resolveCheckTarget(name)
	if bad != nil {
		return *bad
	}

	inst, owner := s.owningInstance(target.fqn)
	passed, err := target.ctx.EvaluateRequirementOn(target.sym, target.scope, inst)
	if err != nil || !passed {
		return Verdict{Subject: name, Status: failedStatus(err), Lines: []string{
			fmt.Sprintf("✗ Requirement %s failed%s", name, onInstance(inst, owner)),
			"  " + verdictDetail("Required condition", err),
		}}
	}
	return Verdict{Subject: name, Status: VerdictHolds, Lines: []string{
		fmt.Sprintf("✓ Requirement %s satisfied%s", name, onInstance(inst, owner)),
	}}
}

// CheckSatisfy evaluates satisfaction assertions: every one the model states
// when name is empty, or, given a name, the ones the named element states — or
// that element itself, when it is a named satisfaction assertion. The usual
// `assert satisfy r by p;` is anonymous, so the element stating it is how a
// caller reaches it.
func (s *Session) CheckSatisfy(name string) []Verdict {
	verdicts := s.checkSatisfy(name)
	if len(verdicts) > 0 {
		// The trace of every assertion belongs to the report as a whole, which is
		// how the prompt reports it too.
		verdicts[0] = s.withTrace(verdicts[0])
	}
	return verdicts
}

func (s *Session) checkSatisfy(name string) []Verdict {
	doc := s.ws.Document(docName)
	if doc == nil || doc.Scope == nil {
		return []Verdict{unresolvedVerdict(name, "no declarations loaded")}
	}
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return []Verdict{unresolvedVerdict(name, err.Error())}
	}

	scope := doc.Scope
	where := "the session"
	if name != "" {
		sym, fqn, lerr := s.lookupSymbol(name)
		if lerr != nil {
			return []Verdict{unresolvedVerdict(name, lerr.Error())}
		}
		if a, aerr := ctx.SatisfyAssertionOf(sym); aerr == nil {
			return []Verdict{s.satisfyVerdict(ctx, a)}
		}
		if sym.Scope == nil {
			return []Verdict{unresolvedVerdict(name, fmt.Sprintf("%s states no satisfaction assertion", name))}
		}
		scope, where = sym.Scope, fqn
	}

	assertions := ctx.SatisfyAssertionsIn(scope)
	if len(assertions) == 0 {
		// Nothing was checked, so nothing is claimed about the model.
		return []Verdict{{
			Subject: name,
			Status:  VerdictUnresolved,
			Lines:   []string{fmt.Sprintf("no satisfaction assertion in %s", where)},
		}}
	}
	verdicts := make([]Verdict, 0, len(assertions))
	for _, a := range assertions {
		verdicts = append(verdicts, s.satisfyVerdict(ctx, a))
	}
	return verdicts
}

// checkTarget is a resolved element to evaluate the conditions of: the symbol,
// the name instances of it are keyed under, the scope its conditions were
// written in, and the runtime to evaluate them.
type checkTarget struct {
	sym   *symbols.Symbol
	fqn   string
	scope *symbols.Scope
	ctx   *runtime.Context
}

// resolveCheckTarget resolves the element a constraint/requirement check names.
// The second result is non-nil for a check that cannot be made at all.
func (s *Session) resolveCheckTarget(name string) (checkTarget, *Verdict) {
	doc := s.ws.Document(docName)
	if doc == nil || doc.Scope == nil {
		bad := unresolvedVerdict(name, "no declarations loaded")
		return checkTarget{}, &bad
	}
	sym, fqn, lerr := s.lookupSymbol(name)
	if lerr != nil {
		bad := unresolvedVerdict(name, lerr.Error())
		return checkTarget{}, &bad
	}
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		bad := unresolvedVerdict(name, err.Error())
		return checkTarget{}, &bad
	}
	return checkTarget{sym: sym, fqn: fqn, scope: declaringScope(sym, doc.Scope), ctx: ctx}, nil
}

// InstantiateNamed creates an object of the named definition and keeps it, so a
// later check about that name is a check about this object. It returns the lines
// `%instantiate` prints, and an error for a name the session cannot resolve or
// an instantiation the runtime rejected.
func (s *Session) InstantiateNamed(name string) ([]string, error) {
	lines, err := s.instantiateNamed(name)
	if err != nil {
		return nil, err
	}
	return append(s.drainTrace(), lines...), nil
}

func (s *Session) instantiateNamed(name string) ([]string, error) {
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return nil, fmt.Errorf("runtime init: %w", err)
	}

	sym, fqn, lerr := s.lookupSymbol(name)
	if lerr != nil {
		return nil, lerr
	}

	inst, err := ctx.Instantiate(sym)
	if err != nil {
		return nil, fmt.Errorf("instantiation failed: %w", err)
	}

	// Keyed by the resolved name, so %slots finds the instance whichever
	// spelling of the name created it.
	s.instances[fqn] = inst
	return []string{
		fmt.Sprintf("✓ Created instance of %s", fqn),
		fmt.Sprintf("  ID: %d", inst.ID),
		fmt.Sprintf("  Use %%slots %s to inspect", name),
	}, nil
}
