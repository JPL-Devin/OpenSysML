package resolve

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
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

// resolveCachedAliasTarget resolves an alias target from its raw qualified name
// text. A cached symbol carries no scope, so the lookup goes through the index:
// first as an absolute FQN, then relative to the namespace that declares the
// alias.
//
// Both lookups are made *from* that namespace (LookupQualifiedFrom), which is
// what lets an alias reach a target its namespace only sees through a private
// wildcard import — `ISQThermodynamics::TemperatureValue` aliases
// `ThermodynamicTemperatureValue`, which ISQThermodynamics holds only by way of
// its `private import ISQBase::*`. That name is a member of the namespace but
// not a visible one (KerML 8.2.3.3), so it is unreachable by a qualified
// reference from elsewhere while remaining reachable from within, which is where
// an alias of it is declared.
func (r *Resolver) resolveCachedAliasTarget(targetText string, aliasSym *symbols.Symbol) *symbols.Symbol {
	namespaceFQN := parentPackageFQN(aliasSym.Name)

	// Absolute reference.
	if candidates := r.idx.LookupQualifiedFrom(targetText, namespaceFQN); len(candidates) > 0 {
		return candidates[0]
	}

	// Relative to the declaring namespace: an alias at "ISQ::TimeValue" naming
	// "DurationValue" means "ISQ::DurationValue".
	if namespaceFQN != "" {
		candidateFQN := namespaceFQN + "::" + targetText
		if candidates := r.idx.LookupQualifiedFrom(candidateFQN, namespaceFQN); len(candidates) > 0 {
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
