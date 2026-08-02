package resolve

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/source"
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
	model       interface{} // Optional *semantics.Model for inheritance-aware member lookup
}

// New creates a resolver over the given index.
func New(idx *symbols.Index) *Resolver {
	return &Resolver{
		idx:  idx,
		memo: make(map[ast.Node]resolution),
	}
}

// SetModel attaches a semantic model for inheritance-aware member resolution.
// Must be called before resolving feature chains if inherited members are needed.
func (r *Resolver) SetModel(model interface{}) {
	r.model = model
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

// doResolveQualified is the uncached qualified-name resolution.
func (r *Resolver) doResolveQualified(scope *symbols.Scope, qn *ast.QualifiedName) resolution {
	return r.walkQualified(scope, qn)
}

// ResolveName resolves a single-segment (unqualified) reference from the given
// scope. The at node keys the memo table.
func (r *Resolver) ResolveName(scope *symbols.Scope, name string, at ast.Node) (*symbols.Symbol, bool) {
	if at != nil {
		if res, done := r.memo[at]; done {
			return res.sym, res.ok
		}
	}
	res := r.walkUnqualified(scope, name)
	if at != nil {
		r.memo[at] = res
	}
	if !res.ok {
		r.Diagnostics = append(r.Diagnostics, Diagnostic{
			Span:    spanOf(at),
			Message: "unresolved reference: " + name,
		})
	}
	return res.sym, res.ok
}

func spanOf(n ast.Node) source.Span {
	if n == nil {
		return source.Span{}
	}
	return n.Span()
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
