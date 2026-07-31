package semantics

import "github.com/Open-MBEE/Systemica/internal/core/symbols"

// MembersOf returns the members visible on sym: those declared directly in its
// owned scope plus members inherited from its (transitive) supertypes, with
// name masking — a member declared closer to sym hides an inherited member of
// the same name (approximating redefinition/masking). Results are deterministic:
// local members first (declaration order), then supertype members in
// AllSupertypes order.
func (m *Model) MembersOf(sym *symbols.Symbol) []*symbols.Symbol {
	if sym == nil {
		return nil
	}
	var out []*symbols.Symbol
	seenName := make(map[string]bool)
	seenSym := make(map[*symbols.Symbol]bool)

	collect := func(scope *symbols.Scope) {
		if scope == nil {
			return
		}
		for _, key := range scope.MemberNames() {
			if seenName[key] {
				continue // masked by a closer declaration
			}
			for _, s := range scope.LookupLocalAll(key) {
				if !seenSym[s] {
					seenSym[s] = true
					out = append(out, s)
				}
			}
		}
		// Mark this scope's names only after processing it, so a short+primary
		// pair in the same scope does not mask its own second key.
		for _, key := range scope.MemberNames() {
			seenName[key] = true
		}
	}

	collect(sym.Scope)
	for _, sup := range m.AllSupertypes(sym) {
		collect(sup.Scope)
	}
	return out
}

// LookupMember returns the first visible member of sym (local or inherited)
// registered under name, honoring masking.
func (m *Model) LookupMember(sym *symbols.Symbol, name string) (*symbols.Symbol, bool) {
	if sym == nil || name == "" {
		return nil, false
	}
	// Local first.
	if sym.Scope != nil {
		if s, ok := sym.Scope.LookupLocal(name); ok {
			return s, true
		}
	}
	for _, sup := range m.AllSupertypes(sym) {
		if sup.Scope == nil {
			continue
		}
		if s, ok := sup.Scope.LookupLocal(name); ok {
			return s, true
		}
	}
	return nil, false
}
