package passes

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
)

// checkExposeOwners reports every `expose` whose owning namespace is not a view
// (validateExposeOwningNamespace, SysML v2 8.3.26.2). The spec constrains the
// owner to a ViewUsage, so a `view def` body is a warning rather than an error:
// Systemica resolves an `expose` there, and the corpus never writes one.
func checkExposeOwners(owner ast.Node, members []ast.Node) []Diagnostic {
	var diags []Diagnostic
	for _, m := range members {
		switch n := unwrapType(m).(type) {
		case *ast.Import:
			if d, ok := exposeOwnerDiagnostic(owner, n); ok {
				diags = append(diags, d)
			}
		case *ast.Package:
			diags = append(diags, checkExposeOwners(n, n.Members)...)
		case *ast.Namespace:
			diags = append(diags, checkExposeOwners(n, n.Members)...)
		case *ast.Definition:
			diags = append(diags, checkExposeOwners(n, n.Members)...)
		case *ast.Usage:
			diags = append(diags, checkExposeOwners(n, n.Members)...)
		}
	}
	return diags
}

// exposeOwnerDiagnostic reports imp when it is an `expose` its owner may not own.
func exposeOwnerDiagnostic(owner ast.Node, imp *ast.Import) (Diagnostic, bool) {
	if !imp.IsExpose {
		return Diagnostic{}, false
	}
	switch n := owner.(type) {
	case *ast.Usage:
		if n.Kind == ast.UsageView {
			return Diagnostic{}, false
		}
	case *ast.Definition:
		if n.Kind == ast.DefView {
			return Diagnostic{
				Severity: SeverityWarning,
				Span:     imp.Span(),
				Message:  "expose in a view def body: SysML v2 8.3.26.2 constrains an expose to a view usage",
				Code:     "expose-owning-namespace",
				Source:   "constraint",
			}, true
		}
	}
	return Diagnostic{
		Severity: SeverityError,
		Span:     imp.Span(),
		Message:  "expose is only allowed in a view usage body (SysML v2 8.3.26.2)",
		Code:     "expose-owning-namespace",
		Source:   "constraint",
	}, true
}
