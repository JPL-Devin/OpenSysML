package model

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// MetadataBodyRedefines returns the metadata-definition feature a metadata
// annotation body declaration implicitly redefines (KerML 7.4.7), with the
// feature's qualified name. ok is false when sym is not such a declaration or
// the annotation's metaclass does not resolve.
func (w *Workspace) MetadataBodyRedefines(sym *symbols.Symbol) (*symbols.Symbol, string, bool) {
	if sym == nil || sym.OwnerScope == nil {
		return nil, "", false
	}
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok {
		return nil, "", false
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	resolver, sem := w.newResolver()
	owner := resolver.MetadataBodyOwner(sym.OwnerScope)
	if owner == nil {
		return nil, "", false
	}
	target := symbols.MetadataBodyTarget(sem, owner, usage.Ident)
	if target == nil {
		return nil, "", false
	}
	return target, symbols.FQNOf(target), true
}

// MetadataBodyMembers returns the members of the metadata definition an
// annotation body's declarations redefine, as seen from the body scope, or nil
// when scope is not a metadata annotation body or its metaclass does not
// resolve.
func (w *Workspace) MetadataBodyMembers(scope *symbols.Scope) []*symbols.Symbol {
	w.mu.RLock()
	defer w.mu.RUnlock()
	resolver, sem := w.newResolver()
	owner := resolver.MetadataBodyOwner(scope)
	if owner == nil {
		return nil
	}
	return w.memberSymbolsLocked(resolver, sem, scope, owner)
}
