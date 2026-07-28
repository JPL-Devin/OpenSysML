package resolve

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// ResolveAliasTarget follows an alias symbol to its ultimate non-alias target.
// Non-alias symbols resolve to themselves. Cycles yield (nil, false).
func (r *Resolver) ResolveAliasTarget(sym *symbols.Symbol) (*symbols.Symbol, bool) {
	seen := map[*symbols.Symbol]bool{}
	cur := sym
	for cur != nil {
		if cur.Kind != symbols.SymbolAlias {
			return cur, true
		}
		if seen[cur] {
			return nil, false
		}
		seen[cur] = true
		al, ok := cur.Decl.(*ast.Alias)
		if !ok || al.For == nil {
			return nil, false
		}
		// Resolve the alias target qualified name from the alias's own
		// enclosing scope.
		next, ok := r.ResolveQualified(aliasScope(cur), al.For)
		if !ok {
			return nil, false
		}
		cur = next
	}
	return nil, false
}

// aliasScope returns the scope in which an alias's target should be resolved:
// the alias symbol's enclosing scope (where it was declared). Leaf symbols
// (such as aliases) carry their enclosing scope in OwnerScope; Scope is the
// child scope a declaration owns and is nil for leaves.
func aliasScope(sym *symbols.Symbol) *symbols.Scope {
	return sym.OwnerScope
}
