package resolve

import (
	"fmt"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// walkQualified resolves a qualified name segment-by-segment.
func (r *Resolver) walkQualified(scope *symbols.Scope, qn *ast.QualifiedName) resolution {
	if len(qn.Parts) == 0 {
		return resolution{nil, false}
	}

	// Resolve the first segment. A non-global name first searches the enclosing
	// scope chain; a global ($::) name starts at the document root. When the
	// local scope tree has no match, fall back to the global qualified-name
	// index so top-level names declared in other documents resolve.
	first := qn.Parts[0].Text
	var cur *symbols.Symbol
	if qn.Global {
		cur = r.lookupInRoot(scope, first)
	} else {
		cur = r.lookupOutward(scope, first)
	}
	if cur == nil {
		sym, n := r.lookupGlobalTop(first)
		if n > 1 {
			r.ambiguous(qn, n)
			return resolution{nil, false}
		}
		cur = sym
	}
	if cur == nil {
		r.unresolved(qn)
		return resolution{nil, false}
	}

	// Walk remaining segments as local members of the current symbol's scope.
	for _, seg := range qn.Parts[1:] {
		if cur.Scope == nil {
			r.unresolved(qn)
			return resolution{nil, false}
		}
		all := cur.Scope.LookupLocalAll(seg.Text)
		if len(all) == 0 {
			r.unresolved(qn)
			return resolution{nil, false}
		}
		if len(all) > 1 {
			r.ambiguous(qn, len(all))
			return resolution{nil, false}
		}
		cur = all[0]
	}
	return resolution{cur, true}
}

// lookupInRoot finds a name in the document root scope reachable from scope.
func (r *Resolver) lookupInRoot(scope *symbols.Scope, name string) *symbols.Symbol {
	root := rootOf(scope)
	if root == nil {
		return nil
	}
	sym, _ := root.LookupLocal(name)
	return sym
}

// lookupOutward searches scope and its ancestors for a locally-defined name.
// Import-aware search is added in Task 7.
func (r *Resolver) lookupOutward(scope *symbols.Scope, name string) *symbols.Symbol {
	for s := scope; s != nil; s = s.Parent() {
		if sym, ok := s.LookupLocal(name); ok {
			return sym
		}
	}
	return nil
}

// lookupGlobalTop finds a top-level (single-segment FQN) symbol in the global
// index. Returns the unique match and the total number of matches, so the
// caller can report ambiguity (n > 1) rather than silently degrading to
// "unresolved". A unique symbol is returned only when n == 1.
func (r *Resolver) lookupGlobalTop(name string) (*symbols.Symbol, int) {
	if r.idx == nil {
		return nil, 0
	}
	syms := r.idx.LookupQualified(name)
	if len(syms) == 1 {
		return syms[0], 1
	}
	return nil, len(syms)
}

// rootOf returns the topmost ancestor of scope (the document root), or nil.
func rootOf(scope *symbols.Scope) *symbols.Scope {
	if scope == nil {
		return nil
	}
	for scope.Parent() != nil {
		scope = scope.Parent()
	}
	return scope
}

// unresolved records an unresolved-reference diagnostic.
func (r *Resolver) unresolved(qn *ast.QualifiedName) {
	r.Diagnostics = append(r.Diagnostics, Diagnostic{
		Span:    qn.Span(),
		Message: "unresolved reference: " + qnText(qn),
	})
}

// ambiguous records an ambiguity diagnostic reporting the number of matches.
func (r *Resolver) ambiguous(qn *ast.QualifiedName, n int) {
	r.Diagnostics = append(r.Diagnostics, Diagnostic{
		Span:    qn.Span(),
		Message: fmt.Sprintf("ambiguous reference: %s (%d candidates)", qnText(qn), n),
	})
}
