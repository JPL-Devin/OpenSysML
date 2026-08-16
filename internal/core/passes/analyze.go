package passes

import (
	"sort"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// DefaultRegistry returns the default pass registry: syntax, name resolution,
// state transitions, type checking, and semantic constraints.
func DefaultRegistry() *Registry {
	reg := NewRegistry()
	reg.Register(SyntaxPass{})
	reg.Register(NameResolutionPass{})
	reg.Register(StateTransitionPass{})
	reg.Register(TypeCheckPass{})
	reg.Register(ElementFilterPass{})
	reg.Register(ConstraintPass{})
	return reg
}

// Analyze runs the default validation passes over a single parsed document and
// returns diagnostics sorted by span offset, then source, then message.
// parseDiags are the parser's diagnostics, already adapted to passes.Diagnostic
// by the caller.
func Analyze(name string, root *ast.RootNamespace, parseDiags []Diagnostic, idx *symbols.Index) []Diagnostic {
	ctx := NewContext(name, idx, parseDiags)
	diags := DefaultRegistry().Run(ctx, name, root)
	sort.SliceStable(diags, func(i, j int) bool {
		a, b := diags[i], diags[j]
		if a.Span.Offset != b.Span.Offset {
			return a.Span.Offset < b.Span.Offset
		}
		if a.Source != b.Source {
			return a.Source < b.Source
		}
		return a.Message < b.Message
	})
	return diags
}
