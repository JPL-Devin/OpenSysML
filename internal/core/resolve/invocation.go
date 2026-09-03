package resolve

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/libnames"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// ResolveInvocationName resolves the name an invocation calls as ResolveQualified
// does, except that a bare name the model declares nothing for denotes the
// Kernel Function Library declaration of that name, which is in force unimported,
// and several declarations under a qualified name denote the first of them.
func (r *Resolver) ResolveInvocationName(scope *symbols.Scope, qn *ast.QualifiedName) (*symbols.Symbol, bool) {
	if qn == nil {
		return nil, false
	}
	r.invocationNames[qn] = true
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
// Imported library declarations do not hide the rest of the in-force library set
// of that name; a declaration of the model's own does. A qualified name denotes
// every member its last segment names in the namespace the rest resolves to.
func (r *Resolver) InvocationCandidates(scope *symbols.Scope, qn *ast.QualifiedName) []*symbols.Symbol {
	if qn == nil || len(qn.Parts) == 0 {
		return nil
	}
	if len(qn.Parts) != 1 || qn.Global || scope == nil {
		var out []*symbols.Symbol
		r.aside(func() { out = r.qualifiedCandidates(scope, qn) })
		return out
	}
	name := qn.Parts[0].Text
	var out []*symbols.Symbol
	r.aside(func() { out = r.unqualifiedCandidates(scope, name) })
	if len(out) == 0 {
		return r.LibraryFunctions(name)
	}
	if r.allLibrary(out) {
		for _, sym := range r.LibraryFunctions(name) {
			out = appendSymbol(out, sym)
		}
	}
	return out
}

// qualifiedCandidates resolves qn as an invocation name and, when it has a
// qualifier, widens the last segment to every member it names there.
func (r *Resolver) qualifiedCandidates(scope *symbols.Scope, qn *ast.QualifiedName) []*symbols.Symbol {
	r.invocationNames[qn] = true
	sym, ok := r.resolveQualified(scope, qn, nil)
	if !ok || sym == nil {
		return nil
	}
	last := len(qn.Parts) - 1
	parts := r.parts[qn]
	if last == 0 || len(parts) <= last || parts[last-1] == nil {
		return []*symbols.Symbol{sym}
	}
	all, ok := r.qualifiedSegment(scope, qn, parts[last-1], last)
	if !ok {
		return []*symbols.Symbol{sym}
	}
	out := []*symbols.Symbol{sym}
	for _, found := range all {
		if !r.AliasNamesNothing(found) {
			out = appendSymbol(out, r.AliasedElement(found))
		}
	}
	return out
}

// allLibrary reports whether every symbol is declared by bundled library content.
func (r *Resolver) allLibrary(syms []*symbols.Symbol) bool {
	if r.idx == nil {
		return false
	}
	for _, sym := range syms {
		if !r.idx.Library(sym) {
			return false
		}
	}
	return true
}

// unqualifiedCandidates walks the scopes as walkUnqualifiedHiding does, stopping
// at the first step that binds name; the owned members and the imports of a
// scope yield every match.
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
		if out, ok := r.localBindingCandidates(s, name); ok {
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
		for enclosing := s.Parent(); enclosing != nil; enclosing = enclosing.Parent() {
			if out, ok := r.localBindingCandidates(enclosing, name); ok {
				return out
			}
		}
		if out := r.importMatches(s, name); len(out) > 0 {
			return out
		}
	}
	if root := rootOf(scope); root != nil {
		if out, ok := r.localBindingCandidates(root, name); ok {
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

// localBindingCandidates is localBinding with every owned member of scope that
// binds name contributing, the one localBinding returns first, and reports
// whether that step binds name at all.
func (r *Resolver) localBindingCandidates(scope *symbols.Scope, name string) ([]*symbols.Symbol, bool) {
	first, ok := r.localBinding(scope, name, nil)
	if !ok {
		return nil, false
	}
	if r.AliasNamesNothing(first) {
		return nil, true
	}
	out := []*symbols.Symbol{first}
	for _, sym := range symbols.PreferDeclared(scope.LookupLocalAll(name)) {
		if r.bindsEffectiveName(sym) && !r.AliasNamesNothing(sym) {
			out = appendSymbol(out, sym)
		}
	}
	return out, true
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
		out := []*symbols.Symbol{found}
		// Each general type contributes its own declaration of a name sym does not declare.
		if all, ok := r.model.(contributedMembersLookup); ok && !declaresLocally(sym, found, name) {
			for _, other := range all.LookupContributedMembers(sym, name) {
				if admits(other) && !r.AliasNamesNothing(other) {
					out = appendSymbol(out, other)
				}
			}
		}
		return out, true
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

// contributedMembersLookup is the member lookup that can list the declaration
// each of a type's generals contributes under one name; *semantics.Model implements it.
type contributedMembersLookup interface {
	LookupContributedMembers(sym *symbols.Symbol, name string) []*symbols.Symbol
}

// declaresLocally reports whether found is the member sym itself declares under name.
func declaresLocally(sym, found *symbols.Symbol, name string) bool {
	if sym.Scope == nil {
		return false
	}
	local, ok := sym.Scope.LookupLocal(name)
	return ok && local == found
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
