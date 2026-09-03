package semantics

import "github.com/Open-MBEE/OpenSysML/internal/core/symbols"

// MembersOf returns the members visible on sym: those declared directly in its
// owned scope plus members inherited from what it specializes and what it
// reference-subsets. Two maskings apply: a member declared closer to sym hides
// an inherited member of the same name, and a feature redefined by one of sym's
// features is not inherited at all (see masking.go).
// Results are deterministic: local members first (declaration order), then
// contributed members in MemberSources order.
func (m *Model) MembersOf(sym *symbols.Symbol) []*symbols.Symbol {
	return m.membersOf(sym, memberViewEffective, nil)
}

// MembersOfIncludingRedefined is MembersOf without redefinition masking: the
// members a type would have if none of its features redefined anything. A
// redefinition shares its target's feature value, so the runtime shape needs
// both.
func (m *Model) MembersOfIncludingRedefined(sym *symbols.Symbol) []*symbols.Symbol {
	return m.membersOf(sym, memberViewUnmasked, nil)
}

// MembersOfDeclaring returns the members of sym as a declaration being written
// in it sees them: that declaration is not yet a member of its own owner and
// the redefinition it carries masks nothing, so its target stays resolvable
// (KerML 8.3.3.3.6). Sym's other declarations, and the masks they cause, are
// present. A nil declaring — the caller cannot tell which declaration is being
// written — stands for every redefinition sym declares.
func (m *Model) MembersOfDeclaring(sym, declaring *symbols.Symbol) []*symbols.Symbol {
	return m.membersOf(sym, memberViewDeclaring, declaring)
}

// memberView selects which of a type's members MembersOf reports.
type memberView int

const (
	// memberViewEffective is what the type actually has: declarations plus
	// unmasked inherited members.
	memberViewEffective memberView = iota
	// memberViewUnmasked applies no redefinition mask.
	memberViewUnmasked
	// memberViewDeclaring is the view a declaration being written in the type
	// has: itself absent, and the mask it causes suspended.
	memberViewDeclaring
)

func (m *Model) membersOf(sym *symbols.Symbol, view memberView, declaring *symbols.Symbol) []*symbols.Symbol {
	if sym == nil {
		return nil
	}
	if target, ok := m.resolver.ResolveAliasTarget(sym); ok {
		sym = target
	}
	var out []*symbols.Symbol
	seenName := make(map[string]bool)
	seenSym := make(map[*symbols.Symbol]bool)
	// One mask per enumeration: it depends only on sym and declaring.
	mask := m.viewMask(sym, view, declaring)

	collect := func(scope *symbols.Scope, inherited bool) {
		if scope == nil {
			return
		}
		for _, key := range scope.MemberNames() {
			if seenName[key] {
				continue // masked by a closer declaration
			}
			for _, s := range scope.LookupLocalAll(key) {
				if !inherited && view == memberViewDeclaring && NotYetMember(s, declaring) {
					continue // a feature being declared is not yet a member
				}
				if inherited && m.maskedBy(mask, s) {
					continue // redefined by a feature of sym
				}
				if !seenSym[s] {
					seenSym[s] = true
					out = append(out, s)
				}
			}
		}
		// Mark this scope's names only after processing it, so a short+primary
		// pair in the same scope does not mask its own second key. A name no
		// kept member binds masks nothing.
		for _, key := range scope.MemberNames() {
			for _, s := range scope.LookupLocalAll(key) {
				if seenSym[s] {
					seenName[key] = true
					break
				}
			}
		}
	}

	collect(sym.Scope, false)
	for _, src := range m.MemberSources(sym) {
		collect(src.Scope, true)
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
	if target, ok := m.resolver.ResolveAliasTarget(sym); ok {
		sym = target
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
	var found *symbols.Symbol
	m.eachContributedMember(sym, name, func(s *symbols.Symbol) bool {
		found = s
		return false
	})
	return found, found != nil
}

// LookupContributedMembers is LookupContributedMember collecting the member
// each source contributes under name, in source order, without duplicates.
func (m *Model) LookupContributedMembers(sym *symbols.Symbol, name string) []*symbols.Symbol {
	var out []*symbols.Symbol
	m.eachContributedMember(sym, name, func(s *symbols.Symbol) bool {
		if !containsSymbol(out, s) {
			out = append(out, s)
		}
		return true
	})
	return out
}

// eachContributedMember calls yield with the member each of sym's member
// sources holds under name, until yield returns false.
func (m *Model) eachContributedMember(sym *symbols.Symbol, name string, yield func(*symbols.Symbol) bool) {
	if sym == nil || name == "" {
		return
	}
	if target, ok := m.resolver.ResolveAliasTarget(sym); ok {
		sym = target
	}
	for _, sup := range m.MemberSources(sym) {
		if sup.Scope != nil {
			for _, s := range symbols.PreferDeclared(sup.Scope.LookupLocalAll(name)) {
				if !yield(s) {
					return
				}
			}
			continue
		}
		// A cached source with no scope is read from the index.
		for _, child := range m.resolver.Index().LookupDirectChildren(sup.Name) {
			leafName := child.Name
			if lastIdx := lastDoubleColon(child.Name); lastIdx >= 0 {
				leafName = child.Name[lastIdx+2:]
			}
			if leafName == name && !yield(child) {
				return
			}
		}
	}
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
