package repl

import (
	"errors"
	"fmt"

	"github.com/Open-MBEE/Systemica/internal/core/runtime"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// doView reports what a view exposes, the way every other name-taking command
// reports: a name the session cannot find, or an element that is no view, is a
// line rather than a failure of the command.
func (s *Session) doView(name string) ([]string, bool, error) {
	lines, err := s.View(name)
	if err != nil {
		if errors.Is(err, errRuntimeInit) {
			return nil, false, err
		}
		return []string{"error: " + err.Error()}, false, nil
	}
	return lines, false, nil
}

// View reports what a view exposes, the views nested in it, and whether it
// conforms to the viewpoints it satisfies. A view exposing nothing says so; an
// element that is no view is semantics.ErrNotAView.
func (s *Session) View(name string) ([]string, error) {
	sym, fqn, err := s.lookupSymbol(name)
	if err != nil {
		return nil, err
	}
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errRuntimeInit, err)
	}
	model := ctx.Model()
	exposed, err := model.ExposedElements(sym)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", notationName(fqn), err)
	}
	out := []string{fmt.Sprintf("view %s", notationName(fqn))}
	if len(exposed) == 0 {
		out = append(out, "  exposes nothing")
	} else {
		out = append(out, "  exposes")
		for _, elem := range exposed {
			out = append(out, "    "+s.viewElementLine(elem))
		}
	}
	nested, err := model.NestedViews(sym)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", notationName(fqn), err)
	}
	if len(nested) > 0 {
		out = append(out, "  nested views")
		for _, view := range nested {
			out = append(out, "    "+s.viewElementLine(view))
		}
	}
	report, err := model.ViewConformance(sym, concernEvaluator{session: s, ctx: ctx})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", notationName(fqn), err)
	}
	return append(out, s.conformanceLines(report)...), nil
}

// conformanceLines renders a conformance report in declaration order: each
// satisfy, each concern the viewpoint frames, each element checked.
func (s *Session) conformanceLines(report *semantics.ViewConformance) []string {
	if report == nil || len(report.Viewpoints) == 0 {
		return nil
	}
	out := []string{"  viewpoint conformance"}
	for _, vp := range report.Viewpoints {
		satisfy := fmt.Sprintf("satisfy %s", quoted(vp.Ref))
		if vp.SatisfiedIn != nil && vp.SatisfiedIn != report.View {
			satisfy += fmt.Sprintf(" (from %s)", s.viewElementName(vp.SatisfiedIn))
		}
		out = append(out, "    "+withReason(fmt.Sprintf("%s: %v", satisfy, vp.Verdict), vp.Reason))
		for _, concern := range vp.Concerns {
			out = append(out, "      "+withReason(fmt.Sprintf("concern %s: %v", quoted(concern.Name), concern.Verdict), concernReason(concern)))
			for _, check := range concern.Checks {
				if check.Holds {
					continue
				}
				out = append(out, fmt.Sprintf("        %s: %v", s.viewElementName(check.Element), check.Err))
			}
		}
		for _, party := range vp.Parties {
			if party.Reason != "" {
				out = append(out, "      ? "+party.Reason)
			}
		}
	}
	return out
}

// concernReason is the reason a concern's verdict carries, suppressed for one
// whose per-element lines already say it.
func concernReason(concern semantics.ConcernConformance) string {
	if len(concern.Checks) > 0 {
		return ""
	}
	return concern.Reason
}

// withReason appends a verdict's reason in parentheses.
func withReason(line, reason string) string {
	if reason == "" {
		return line
	}
	return line + " (" + reason + ")"
}

// quoted renders a name the notation reads, so a name needing quotes keeps them.
func quoted(name string) string {
	if name == "" {
		return "<none>"
	}
	return notationName(name)
}

// concernEvaluator answers whether a framed concern holds of one exposed element
// through the runtime's requirement engine, as `satisfy <concern> by <element>`.
type concernEvaluator struct {
	session *Session
	ctx     *runtime.Context
}

// EvaluateConcern evaluates concern's conditions against an object of element.
func (e concernEvaluator) EvaluateConcern(concern, element *symbols.Symbol) (bool, error) {
	inst, err := e.session.viewSubject(element)
	if err != nil {
		return false, err
	}
	assertion := &runtime.SatisfyAssertion{
		Symbol:     concern,
		Subject:    element,
		SubjectRef: e.session.viewElementName(element),
	}
	if requirement := e.ctx.Model().FramedConcernTarget(concern); requirement != nil {
		// Named only when it resolved, so a reference naming nothing is reported
		// as unresolved rather than as this concern's own conditions.
		assertion.Requirement = requirement
		assertion.RequirementRef = requirement.Name
	}
	result, err := e.ctx.CheckSatisfactionOn(assertion, inst)
	// A concern stating no condition lacks a condition, not a requirement.
	if errors.Is(err, runtime.ErrNoRequirement) || errors.Is(err, runtime.ErrNoConditions) {
		return false, errNoConcernCondition
	}
	return result.Holds, err
}

// viewSubject returns the object a concern is evaluated against for one exposed
// element: the one the session already created for it, else one created and kept
// as %satisfy keeps its subject, so a repeated %view is about the same object
// rather than another copy of it.
func (s *Session) viewSubject(element *symbols.Symbol) (*runtime.Instance, error) {
	name := s.viewElementName(element)
	if inst, ok := s.instances[name]; ok {
		return inst, nil
	}
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return nil, err
	}
	inst, err := ctx.Instantiate(element)
	if err != nil {
		return nil, err
	}
	s.instances[name] = inst
	return inst, nil
}

// errNoConcernCondition is a framed concern that states nothing to evaluate.
var errNoConcernCondition = errors.New("states no condition to evaluate")

// IsViolation reports whether an evaluation error is the model answering false,
// which the REPL already distinguishes from a check it could not make.
func (e concernEvaluator) IsViolation(err error) bool {
	return err != nil && !unevaluable(err)
}

// viewElementLine names an element by qualified name and kind, as %search does.
func (s *Session) viewElementLine(sym *symbols.Symbol) string {
	return fmt.Sprintf("%s (%s)", s.viewElementName(sym), sym.Kind.String())
}

// viewElementName names an element by qualified name where the index knows one.
func (s *Session) viewElementName(sym *symbols.Symbol) string {
	name := sym.Name
	if idx := s.browseIndex(); idx != nil {
		if fqn := idx.GetFQN(sym); fqn != "" {
			name = fqn
		}
	}
	return notationName(name)
}
