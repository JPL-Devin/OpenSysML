package semantics

import "github.com/Open-MBEE/OpenSysML/internal/core/symbols"

// MembersOf returns the members visible on sym: those declared directly in its
// owned scope plus members inherited from what it specializes and what it
// reference-subsets, with name masking — a member declared closer to sym hides
// an inherited member of the same name (approximating redefinition/masking).
// Results are deterministic: local members first (declaration order), then
// contributed members in MemberSources order.
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
	for _, src := range m.MemberSources(sym) {
		collect(src.Scope)
	}
	return out
}

// LookupMember returns the first visible member of sym — declared by it, or
// contributed by what it specializes or reference-subsets — registered under
// name, honoring masking.
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
	// If no scope (cached stdlib symbol), query index for direct children
	if sym.Scope == nil {
		idx := m.resolver.Index()
		children := idx.LookupDirectChildren(sym.Name)
		for _, child := range children {
			// Extract leaf name from child FQN
			leafName := child.Name
			if lastIdx := lastDoubleColon(child.Name); lastIdx >= 0 {
				leafName = child.Name[lastIdx+2:]
			}
			if leafName == name {
				return child, true
			}
		}
	}
	return m.LookupContributedMember(sym, name)
}

// LookupContributedMember is LookupMember without sym's own declarations: only
// the members contributed by what sym specializes, is typed by or
// reference-subsets. Callers that must not see a local binding — resolving a
// reference subsetting's target past the borrowed name it binds itself — ask
// for the contributed member instead.
func (m *Model) LookupContributedMember(sym *symbols.Symbol, name string) (*symbols.Symbol, bool) {
	if sym == nil || name == "" {
		return nil, false
	}
	for _, sup := range m.MemberSources(sym) {
		if sup.Scope != nil {
			if s, ok := sup.Scope.LookupLocal(name); ok {
				return s, true
			}
		}
		// Also check index for cached sources with nil Scope
		if sup.Scope == nil {
			idx := m.resolver.Index()
			children := idx.LookupDirectChildren(sup.Name)
			for _, child := range children {
				leafName := child.Name
				if lastIdx := lastDoubleColon(child.Name); lastIdx >= 0 {
					leafName = child.Name[lastIdx+2:]
				}
				if leafName == name {
					return child, true
				}
			}
		}
	}
	return nil, false
}

// lastDoubleColon returns the index of the last "::" in s, or -1 if not found.
func lastDoubleColon(s string) int {
	for i := len(s) - 1; i > 0; i-- {
		if s[i-1:i+1] == "::" {
			return i - 1
		}
	}
	return -1
}
