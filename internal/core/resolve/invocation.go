package resolve

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/libnames"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// ResolveInvocationName resolves the name an invocation calls as ResolveQualified
// does, except that a bare name the model declares nothing for denotes the
// Kernel Function Library declaration of that name, which is in force unimported.
func (r *Resolver) ResolveInvocationName(scope *symbols.Scope, qn *ast.QualifiedName) (*symbols.Symbol, bool) {
	if qn == nil {
		return nil, false
	}
	if len(qn.Parts) == 1 && !qn.Global {
		r.invocationNames[qn] = true
	}
	return r.resolveQualified(scope, qn, nil)
}

// libraryFunctionFallback is the most general library declaration of name, the
// one a bare invocation reaches when the model declares nothing for it.
func (r *Resolver) libraryFunctionFallback(name string) (*symbols.Symbol, bool) {
	fns := r.LibraryFunctions(name)
	if len(fns) == 0 {
		return nil, false
	}
	return fns[0], true
}

// LibraryFunctions returns the library declarations a bare call to name may
// denote, most general first; only those the index holds exactly once. Memoized.
func (r *Resolver) LibraryFunctions(name string) []*symbols.Symbol {
	if cached, ok := r.libraryFunctions[name]; ok {
		return cached
	}
	var out []*symbols.Symbol
	if r.idx != nil {
		for _, fqn := range libnames.Declarations(name) {
			if matches := r.idx.LookupQualified(fqn); len(matches) == 1 {
				out = append(out, matches[0])
			}
		}
	}
	r.libraryFunctions[name] = out
	return out
}

// InvocationCandidates returns every declaration an invocation's name may denote
// from scope, in lookup order: the first is what ResolveInvocationName reaches.
// A bare name is bound by the first scope step that finds it, every import of
// that scope contributing, or by the library when the model declares nothing.
func (r *Resolver) InvocationCandidates(scope *symbols.Scope, qn *ast.QualifiedName) []*symbols.Symbol {
	if qn == nil || len(qn.Parts) == 0 {
		return nil
	}
	if len(qn.Parts) != 1 || qn.Global || scope == nil {
		var sym *symbols.Symbol
		var ok bool
		r.aside(func() { sym, ok = r.resolveQualified(scope, qn, nil) })
		if !ok || sym == nil {
			return nil
		}
		return []*symbols.Symbol{sym}
	}
	name := qn.Parts[0].Text
	var out []*symbols.Symbol
	r.aside(func() { out = r.unqualifiedCandidates(scope, name) })
	if len(out) == 0 {
		return r.LibraryFunctions(name)
	}
	return out
}

// unqualifiedCandidates walks the scopes as walkUnqualifiedHiding does, stopping
// at the first step that binds name; the imports of a scope yield every match.
func (r *Resolver) unqualifiedCandidates(scope *symbols.Scope, name string) []*symbols.Symbol {
	one := func(sym *symbols.Symbol, ok bool) ([]*symbols.Symbol, bool) {
		if !ok || sym == nil {
			return nil, false
		}
		if r.AliasNamesNothing(sym) {
			return nil, true
		}
		return []*symbols.Symbol{sym}, true
	}
	for s := scope; s != nil; s = s.Parent() {
		if out, ok := one(r.localBinding(s, name, nil)); ok {
			return out
		}
		if out, ok := one(r.acceptPayload(s, name)); ok {
			return out
		}
		if out, ok := one(r.implicitlyNamedMember(s, name, nil)); ok {
			return out
		}
		if out, ok := r.visibleMemberCandidates(r.scopeOwner(s), name); ok {
			return out
		}
		if out, ok := one(r.lookupInheritedImports(s, name)); ok {
			return out
		}
		if out, ok := one(r.enclosingLocal(s.Parent(), name, nil)); ok {
			return out
		}
		if out := r.importMatches(s, name); len(out) > 0 {
			return out
		}
	}
	if root := rootOf(scope); root != nil {
		if out, ok := one(r.localBinding(root, name, nil)); ok {
			return out
		}
	}
	if out, ok := one(r.nestedInRedefined(scope, name, nil)); ok {
		return out
	}
	if sym := r.lookupGlobalTop(scope, name); sym != nil {
		if out, ok := one(sym, true); ok {
			return out
		}
	}
	return nil
}

// visibleMemberCandidates is visibleMember with every import of sym's scope
// contributing, and reports whether that step binds name at all.
func (r *Resolver) visibleMemberCandidates(sym *symbols.Symbol, name string) ([]*symbols.Symbol, bool) {
	if r.model == nil || sym == nil {
		return nil, false
	}
	admits := func(found *symbols.Symbol) bool {
		return visibleAsInheritedMember(sym, found) && !r.inheritanceMaskedDeclaring(sym, found, "")
	}
	if found, ok := r.lookupMemberOf(sym, name); ok {
		if !admits(found) {
			return nil, false
		}
		if r.AliasNamesNothing(found) {
			return nil, true
		}
		return []*symbols.Symbol{found}, true
	}
	if sym.Scope == nil {
		return nil, false
	}
	var out []*symbols.Symbol
	for _, imp := range r.importsOf(sym.Scope.Node()) {
		if r.resolvingImports[imp] || !r.importPrefixAvailable(sym.Scope, imp, name) {
			continue
		}
		for _, found := range r.importMatchesAll(sym.Scope, imp, name) {
			if admits(found) && !r.AliasNamesNothing(found) {
				out = appendSymbol(out, found)
			}
		}
	}
	return out, len(out) > 0
}

// importMatches returns the declarations named name surfaced by the imports
// declared directly in scope, in import order.
func (r *Resolver) importMatches(scope *symbols.Scope, name string) []*symbols.Symbol {
	var out []*symbols.Symbol
	for _, imp := range r.importsOf(scope.Node()) {
		if r.resolvingImports[imp] || !r.importPrefixAvailable(scope, imp, name) {
			continue
		}
		for _, sym := range r.importMatchesAll(scope, imp, name) {
			if !r.AliasNamesNothing(sym) {
				out = appendSymbol(out, sym)
			}
		}
	}
	return out
}

// appendSymbol appends sym to out unless it is already there.
func appendSymbol(out []*symbols.Symbol, sym *symbols.Symbol) []*symbols.Symbol {
	for _, seen := range out {
		if seen == sym {
			return out
		}
	}
	return append(out, sym)
}
