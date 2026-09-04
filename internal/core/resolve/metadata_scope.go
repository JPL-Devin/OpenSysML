package resolve

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// MetadataBodyOwner returns the metadata definition whose features a metadata
// body's declarations implicitly redefine (KerML 7.4.7), for an annotation body
// or a `metadata m : M { … }` usage body; nil otherwise or when M does not resolve.
func (r *Resolver) MetadataBodyOwner(scope *symbols.Scope) *symbols.Symbol {
	if scope == nil {
		return nil
	}
	if scope.BodyLocal() {
		if _, ok := scope.Node().(*ast.PrefixMetadata); !ok {
			return nil
		}
		return r.scopeOwner(scope)
	}
	owner := scope.Owner()
	if owner == nil || owner.Kind != symbols.SymbolMetadataUsage || owner.OwnerScope == nil {
		return nil
	}
	usage, ok := owner.Decl.(*ast.Usage)
	if !ok {
		return nil
	}
	if def, done := r.bodyOwners[scope]; done {
		return def
	}
	r.bodyOwners[scope] = nil
	var resolved *symbols.Symbol
	r.aside(func() {
		if types := r.findTypingTargets(owner.OwnerScope, usage.Relationships); len(types) == 1 {
			resolved = types[0]
		}
	})
	r.bodyOwners[scope] = resolved
	return resolved
}

// scopeOwner returns the symbol whose members scope contributes to unqualified
// lookup. A metadata annotation body not yet stamped by document resolution has
// its owner — the metadata definition the annotation names — resolved on
// demand, quietly, and memoized per resolver so shared scopes stay unmutated.
func (r *Resolver) scopeOwner(scope *symbols.Scope) *symbols.Symbol {
	if scope == nil {
		return nil
	}
	if owner := scope.Owner(); owner != nil {
		return owner
	}
	prefix, ok := scope.Node().(*ast.PrefixMetadata)
	if !ok || !scope.BodyLocal() {
		return nil
	}
	if owner, done := r.bodyOwners[scope]; done {
		return owner
	}
	// Break cycles while the metaclass itself resolves.
	r.bodyOwners[scope] = nil
	var resolved *symbols.Symbol
	r.aside(func() {
		owner, ok := r.ResolveQualified(scope.Parent(), prefix.Type)
		if !ok || owner == nil {
			return
		}
		if target, aliasOK := r.ResolveAliasTarget(owner); aliasOK {
			owner = target
		}
		resolved = owner
	})
	r.bodyOwners[scope] = resolved
	return resolved
}
