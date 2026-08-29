package passes

import (
	"errors"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/queryplan"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// DocumentQueryPass validates native document-query definitions.
type DocumentQueryPass struct{}

func (DocumentQueryPass) Level() PassLevel { return LevelConstraint }

func (DocumentQueryPass) Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	scope := ctx.Index.DocumentRoot(name)
	if scope == nil {
		return nil
	}
	var diagnostics []Diagnostic
	w8dWalkSymbols(ctx, scope, func(sym *symbols.Symbol) {
		if !queryplan.IsQueryDefinition(ctx.Index, ctx.Model(), sym) {
			return
		}
		if _, err := queryplan.Compile(ctx.Index, ctx.Model(), ctx.Resolver(), sym); err != nil {
			diagnostics = append(diagnostics, documentQueryDiagnostic(err))
		}
	})
	return diagnostics
}

func documentQueryDiagnostic(err error) Diagnostic {
	var planning *queryplan.Error
	if !errors.As(err, &planning) {
		return Diagnostic{
			Severity: SeverityError,
			Message:  err.Error(),
			Code:     "document-query",
			Source:   "document-query",
		}
	}
	return Diagnostic{
		Severity: SeverityError,
		Span:     planning.Span,
		Message:  planning.Error(),
		Code:     "document-query-" + string(planning.Kind),
		Source:   "document-query",
	}
}
