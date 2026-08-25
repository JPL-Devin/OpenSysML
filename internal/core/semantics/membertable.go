package semantics

import "github.com/Open-MBEE/OpenSysML/internal/core/symbols"

type memberEntry struct {
	sym   *symbols.Symbol
	depth int
}

func (m *Model) libraryMemberTable(sym *symbols.Symbol) map[string]memberEntry {
	if sym == nil || m.resolver == nil || m.resolver.Index() == nil {
		return nil
	}
	idx := m.resolver.Index()
	generation := idx.Generation()
	if m.memberTableGeneration != generation {
		m.memberTables = make(map[*symbols.Symbol]map[string]memberEntry)
		m.memberTableGeneration = generation
	}
	table := m.buildLibraryMemberTable(sym, make(map[*symbols.Symbol]bool), generation)
	if idx.Generation() != generation {
		m.memberTables = make(map[*symbols.Symbol]map[string]memberEntry)
		m.memberTableGeneration = idx.Generation()
		return nil
	}
	return table
}

func (m *Model) buildLibraryMemberTable(sym *symbols.Symbol, visiting map[*symbols.Symbol]bool, generation uint64) map[string]memberEntry {
	idx := m.resolver.Index()
	if idx.Generation() != generation || !idx.Library(sym) || len(m.resolvingRef) != 0 || m.supersUnstable(sym) {
		return nil
	}
	if visiting[sym] {
		return nil
	}
	if table, ok := m.memberTables[sym]; ok {
		return table
	}
	visiting[sym] = true
	defer delete(visiting, sym)

	table := make(map[string]memberEntry)
	firstSteps := make(map[string]int)
	if sym.Scope != nil {
		for _, name := range sym.Scope.MemberNames() {
			member, ok := sym.Scope.LookupLocal(name)
			if !ok {
				continue
			}
			if !idx.Library(member) || m.supersUnstable(member) {
				return nil
			}
			table[name] = memberEntry{sym: member}
			firstSteps[name] = -1
		}
	} else {
		for _, member := range idx.LookupDirectChildren(sym.Name) {
			name := member.Name
			if lastIdx := lastDoubleColon(name); lastIdx >= 0 {
				name = name[lastIdx+2:]
			}
			if _, exists := table[name]; exists {
				continue
			}
			if !idx.Library(member) || m.supersUnstable(member) {
				return nil
			}
			table[name] = memberEntry{sym: member}
			firstSteps[name] = -1
		}
	}

	for i, contributor := range m.contributors(sym) {
		if contributor == nil || contributor == sym {
			continue
		}
		if !idx.Library(contributor) || m.supersUnstable(contributor) {
			return nil
		}
		contributorTable := m.buildLibraryMemberTable(contributor, visiting, generation)
		if contributorTable == nil {
			return nil
		}
		for name, entry := range contributorTable {
			candidate := memberEntry{sym: entry.sym, depth: entry.depth + 1}
			current, exists := table[name]
			if !exists || candidate.depth < current.depth ||
				(candidate.depth == current.depth && i < firstSteps[name]) {
				table[name] = candidate
				firstSteps[name] = i
			}
		}
	}
	for _, entry := range table {
		if entry.sym == nil || m.supersUnstable(entry.sym) {
			return nil
		}
	}
	if idx.Generation() != generation || len(m.resolvingRef) != 0 {
		return nil
	}
	m.memberTables[sym] = table
	return table
}
