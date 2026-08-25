package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// w8dWalkSymbols visits every symbol of the scope subtree exactly once.
func w8dWalkSymbols(ctx *Context, root *symbols.Scope, visit func(*symbols.Symbol)) {
	for _, sym := range w8dSymbols(ctx, root) {
		visit(sym)
	}
}

func w8dSymbols(ctx *Context, root *symbols.Scope) []*symbols.Symbol {
	if ctx != nil {
		if cached, ok := ctx.w8dCache[root]; ok {
			return cached
		}
	}
	out := w8dCollectSymbols(root)
	if ctx != nil {
		if ctx.w8dCache == nil {
			ctx.w8dCache = make(map[*symbols.Scope][]*symbols.Symbol)
		}
		ctx.w8dCache[root] = out
	}
	return out
}

func w8dCollectSymbols(root *symbols.Scope) []*symbols.Symbol {
	seenSyms := make(map[*symbols.Symbol]bool)
	seenScopes := make(map[*symbols.Scope]bool)
	var out []*symbols.Symbol
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
			out = append(out, sym)
			walk(sym.Scope)
		}
		for _, child := range scope.Children() {
			walk(child)
		}
	}
	walk(root)
	return out
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
