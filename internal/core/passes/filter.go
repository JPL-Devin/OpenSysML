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
		fc.check(f, "filter")
	}
	for _, f := range symbols.ImportFiltersIn(scope) {
		fc.check(f, "import filter")
	}
	for _, sym := range scope.AllMembers() {
		if sym != nil {
			fc.walk(sym.Scope)
		}
	}
}

// check reports the faults of one condition, described as what wrote it.
func (fc *filterChecker) check(f symbols.ElementFilter, what string) {
	if fc.model == nil {
		return
	}
	for _, p := range fc.model.CheckElementFilter(f) {
		fc.diags = append(fc.diags, filterDiagnostic(p, what))
	}
}

// filterDiagnostic renders one fault. A condition yielding a non-boolean is an
// error: it can never select an element. One the evaluator cannot decide is a
// warning, because the filter is then not applied and the model keeps every
// element it would have selected from — nothing is lost, but the filter the
// model asked for is not the one it got.
func filterDiagnostic(p semantics.FilterProblem, what string) Diagnostic {
	if p.NotBoolean {
		return Diagnostic{
			Severity: SeverityError,
			Span:     p.Span,
			Message:  "a " + what + " condition must be boolean-valued, but this one yields " + p.Reason + " (KerML 8.2.4)",
			Code:     "filter-not-boolean",
			Source:   "type",
		}
	}
	return Diagnostic{
		Severity: SeverityWarning,
		Span:     p.Span,
		Message:  "this " + what + " condition cannot be evaluated, so it selects nothing and is not applied: " + p.Reason,
		Code:     "filter-not-evaluable",
		Source:   "type",
	}
}
