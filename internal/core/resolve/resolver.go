package resolve

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// resolution is a memoized lookup outcome.
type resolution struct {
	sym *symbols.Symbol
	ok  bool
}

// Resolver performs lazy name resolution over a symbol index, memoizing results
// keyed by the reference AST node and collecting diagnostics.
type Resolver struct {
	idx         *symbols.Index
	memo        map[ast.Node]resolution
	Diagnostics []Diagnostic
}

// New creates a resolver over the given index.
func New(idx *symbols.Index) *Resolver {
	return &Resolver{
		idx:  idx,
		memo: make(map[ast.Node]resolution),
	}
}

// ResolveQualified resolves a qualified-name reference against the given scope.
// scope may be nil to resolve purely from the global index / document root.
// Later tasks implement the walk; this skeleton reports unresolved.
func (r *Resolver) ResolveQualified(scope *symbols.Scope, qn *ast.QualifiedName) (*symbols.Symbol, bool) {
	if qn == nil {
		return nil, false
	}
	if res, done := r.memo[qn]; done {
		return res.sym, res.ok
	}
	res := r.doResolveQualified(scope, qn)
	r.memo[qn] = res
	return res.sym, res.ok
}

// doResolveQualified is the uncached qualified-name resolution. Task 6 replaces
// the body; the skeleton always fails with an unresolved diagnostic.
func (r *Resolver) doResolveQualified(scope *symbols.Scope, qn *ast.QualifiedName) resolution {
	r.Diagnostics = append(r.Diagnostics, Diagnostic{
		Span:    qn.Span(),
		Message: "unresolved reference: " + qnText(qn),
	})
	return resolution{nil, false}
}

// qnText renders a qualified name for diagnostics (segments joined by "::",
// "$::" prefix when global).
func qnText(qn *ast.QualifiedName) string {
	s := ""
	for i, part := range qn.Parts {
		if i > 0 {
			s += "::"
		}
		s += part.Text
	}
	if qn.Global {
		s = "$::" + s
	}
	return s
}
