package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// w8dWalkSymbols visits every symbol of the scope subtree exactly once, so the
// wave 8D passes share one traversal rather than each rebuilding it.
func w8dWalkSymbols(root *symbols.Scope, visit func(*symbols.Symbol)) {
	seenSyms := make(map[*symbols.Symbol]bool)
	seenScopes := make(map[*symbols.Scope]bool)
	var walk func(*symbols.Scope)
	walk = func(scope *symbols.Scope) {
		if scope == nil || seenScopes[scope] {
			return
		}
		seenScopes[scope] = true
		for _, sym := range scope.AllMembers() {
			if sym == nil || seenSyms[sym] {
				continue
			}
			seenSyms[sym] = true
			visit(sym)
			walk(sym.Scope)
		}
		for _, child := range scope.Children() {
			walk(child)
		}
	}
	walk(root)
}

// w8dScopeOf returns the scope a symbol's own references resolve in.
func w8dScopeOf(sym *symbols.Symbol) *symbols.Scope {
	if sym == nil {
		return nil
	}
	if sym.Scope != nil {
		return sym.Scope
	}
	return sym.OwnerScope
}
