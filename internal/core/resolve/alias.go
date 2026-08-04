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
		
		// Fast path: cached alias with target FQN (stdlib symbols loaded via AddRecords)
		if cur.AliasTargetFQN != "" {
			// Resolve target as qualified name from alias's scope
			// For cached symbols, we don't have OwnerScope, so resolve from root
			next := r.resolveCachedAliasTarget(cur.AliasTargetFQN, cur)
			if next == nil {
				return nil, false
			}
			cur = next
			continue
		}
		
		// Slow path: live-parsed alias with Decl (user code)
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

// resolveCachedAliasTarget resolves an alias target from its raw qualified name text.
// For cached stdlib symbols (no OwnerScope), attempts FQN lookup. If that fails,
// treats it as relative and searches parent package scope.
func (r *Resolver) resolveCachedAliasTarget(targetText string, aliasSym *symbols.Symbol) *symbols.Symbol {
	// Try direct FQN lookup first (absolute reference)
	if candidates := r.idx.LookupQualified(targetText); len(candidates) > 0 {
		return candidates[0]
	}
	
	// If alias symbol has OwnerScope (shouldn't for cached, but defensive),
	// resolve as qualified name from that scope
	if aliasSym.OwnerScope != nil {
		// Parse targetText into QualifiedName for resolution
		// For now, simple approach: try relative to parent package
	}
	
	// Fallback: construct likely FQN by going up from alias FQN
	// Example: alias at "ISQ::TimeValue" targeting "DurationValue"
	// → try "ISQ::DurationValue"
	if parentFQN := parentPackageFQN(aliasSym.Name); parentFQN != "" {
		candidateFQN := parentFQN + "::" + targetText
		if candidates := r.idx.LookupQualified(candidateFQN); len(candidates) > 0 {
			return candidates[0]
		}
	}
	
	return nil
}

// parentPackageFQN extracts parent package FQN from symbol FQN.
// "A::B::C" → "A::B", "A" → "", "" → ""
func parentPackageFQN(fqn string) string {
	lastIdx := -1
	for i := len(fqn) - 1; i >= 0; i-- {
		if i > 0 && fqn[i-1:i+1] == "::" {
			lastIdx = i - 1
			break
		}
	}
	if lastIdx < 0 {
		return ""
	}
	return fqn[:lastIdx]
}
