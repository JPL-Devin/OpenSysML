package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// ElementFilterPass checks the element-filter conditions a document writes: the
// `filter` members of a namespace (KerML 8.2.4) and the `[...]` clause of an
// import or expose (SysML v2 7.4.4). A condition is a predicate over one
// candidate element, so it has to be boolean-valued and it has to be a form the
// filter evaluator can decide; a condition that is neither selects nothing, and
// name resolution keeps every candidate rather than hiding model content on a
// verdict it could not reach.
//
// It runs at LevelType: deciding what a condition yields needs the elements it
// names to have resolved, and adds nothing over the unresolved-reference
// diagnostics when they have not.
type ElementFilterPass struct{}

func (ElementFilterPass) Level() PassLevel { return LevelType }

func (ElementFilterPass) Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	fc := &filterChecker{model: ctx.Model(), seen: make(map[*symbols.Scope]bool)}
	fc.walk(rootScope)
	return fc.diags
}

type filterChecker struct {
	model *semantics.Model
	seen  map[*symbols.Scope]bool
	diags []Diagnostic
}

// walk checks the conditions every namespace in the scope subtree writes. A
// definition or usage body is a namespace too, and the corpus writes a filter in
// one, so every scope is visited rather than only the packages.
func (fc *filterChecker) walk(scope *symbols.Scope) {
	if scope == nil || fc.seen[scope] {
		return
	}
	fc.seen[scope] = true
	for _, f := range symbols.NamespaceFiltersIn(scope) {
		fc.check(f)
	}
	for _, f := range symbols.ImportFiltersIn(scope) {
		fc.check(f)
	}
	for _, sym := range scope.AllMembers() {
		if sym != nil {
			fc.walk(sym.Scope)
		}
	}
}

// check reports the faults of one condition.
func (fc *filterChecker) check(f symbols.ElementFilter) {
	if fc.model == nil {
		return
	}
	for _, p := range fc.model.CheckElementFilter(f) {
		fc.diags = append(fc.diags, filterDiagnostic(p))
	}
}

// Pilot KerMLValidator (2026-05) validateElementFilterMembershipIsBoolean and
// the model-level evaluability check behind it: a condition must yield a truth
// value, and must do so without running the model.
const (
	msgFilterNotBoolean   = "Must have a Boolean result"
	msgFilterNotEvaluable = "Must be model-level evaluable"
)

// filterDiagnostic renders one fault. Both are errors: a condition that cannot
// yield a truth value at model level never selects an element.
func filterDiagnostic(p semantics.FilterProblem) Diagnostic {
	if p.NotBoolean {
		return Diagnostic{
			Severity: SeverityError,
			Span:     p.Span,
			Message:  msgFilterNotBoolean,
			Code:     "filter-not-boolean",
			Source:   "type",
		}
	}
	return Diagnostic{
		Severity: SeverityError,
		Span:     p.Span,
		Message:  msgFilterNotEvaluable,
		Code:     "filter-not-evaluable",
		Source:   "type",
	}
}
