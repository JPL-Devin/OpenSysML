package lsp

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// isMetadataBodyScope reports whether scope is a metadata annotation body —
// the anonymous scope 11G builds for the members of `@A { ... }`.
func isMetadataBodyScope(scope *symbols.Scope) bool {
	if scope == nil || !scope.BodyLocal() {
		return false
	}
	_, ok := scope.Node().(*ast.PrefixMetadata)
	return ok
}

// enclosingMetadataBody returns the nearest enclosing metadata annotation body
// scope, or nil — a cursor on a body declaration sits in that declaration's own
// child scope, one level below the body.
func enclosingMetadataBody(scope *symbols.Scope) *symbols.Scope {
	for ; scope != nil; scope = scope.Parent() {
		if isMetadataBodyScope(scope) {
			return scope
		}
	}
	return nil
}

// metadataBodyDeclAt returns the metadata body declaration whose own name
// contains offset, or nil when the cursor is elsewhere.
func metadataBodyDeclAt(scope *symbols.Scope, offset int) *symbols.Symbol {
	sym := symbolAtOffset(scope, offset)
	if sym == nil || !isMetadataBodyScope(sym.OwnerScope) {
		return nil
	}
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok {
		return nil
	}
	for _, sp := range []source.Span{usage.Ident.NameSpan, usage.Ident.ShortNameSpan} {
		if sp.Len > 0 && offset >= sp.Offset && offset < sp.End() {
			return sym
		}
	}
	return nil
}
