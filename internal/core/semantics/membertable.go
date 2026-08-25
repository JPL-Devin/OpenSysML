package semantics

import "github.com/Open-MBEE/OpenSysML/internal/core/symbols"

type memberEntry struct {
	sym   *symbols.Symbol
	depth int
}

type memberTable struct {
	entries map[string]memberEntry
	sources map[*symbols.Symbol]bool
}

type memberPlan struct {
	tables  []*memberTable
	indices []int
}

func (m *Model) libraryMemberTable(sym *symbols.Symbol) *memberTable {
	if sym == nil || m.resolver == nil || m.resolver.Index() == nil {
		return nil
	}
	idx := m.resolver.Index()
	generation := idx.Generation()
	if m.memberTableGeneration != generation {
		m.memberTables = make(map[*symbols.Symbol]*memberTable)
		m.memberTableGeneration = generation
		m.memberPlans = make(map[*symbols.Symbol]*memberPlan)
		m.memberPlanGeneration = generation
	}
	table, permanent := m.buildLibraryMemberTable(sym, make(map[*symbols.Symbol]bool), generation)
	if idx.Generation() != generation {
		m.memberTables = make(map[*symbols.Symbol]*memberTable)
		m.memberTableGeneration = idx.Generation()
		m.memberPlans = make(map[*symbols.Symbol]*memberPlan)
		m.memberPlanGeneration = idx.Generation()
		return nil
	}
	if table == nil && permanent {
		m.memberTables[sym] = nil
	}
	return table
}

func (m *Model) memberPlan(sym *symbols.Symbol) (*memberPlan, bool) {
	if sym == nil || m.resolver == nil || m.resolver.Index() == nil {
		return nil, false
	}
	idx := m.resolver.Index()
	generation := idx.Generation()
	if m.memberPlanGeneration != generation {
		m.memberPlans = make(map[*symbols.Symbol]*memberPlan)
		m.memberPlanGeneration = generation
	}
	plan, ok := m.memberPlans[sym]
	return plan, ok
}

func (m *Model) buildLibraryMemberTable(sym *symbols.Symbol, visiting map[*symbols.Symbol]bool, generation uint64) (*memberTable, bool) {
	idx := m.resolver.Index()
	if idx.Generation() != generation {
		return nil, false
	}
	if !idx.Library(sym) {
		return nil, true
	}
	if libraryReferenceInFlight(idx, m.resolvingRef) {
		return nil, false
	}
	if m.supersUnstable(sym) {
		return nil, false
	}
	if visiting[sym] {
		return nil, true
	}
	if table, ok := m.memberTables[sym]; ok {
		return table, table == nil
	}
	visiting[sym] = true
	defer delete(visiting, sym)

	entries := make(map[string]memberEntry)
	firstSteps := make(map[string]int)
	if sym.Scope != nil {
		for _, name := range sym.Scope.MemberNames() {
			member, ok := sym.Scope.LookupLocal(name)
			if !ok {
				continue
			}
			if !idx.Library(member) {
				return nil, true
			}
			if m.supersUnstable(member) {
				return nil, false
			}
			entries[name] = memberEntry{sym: member}
			firstSteps[name] = -1
		}
	} else {
		for _, member := range idx.LookupDirectChildren(sym.Name) {
			name := member.Name
			if lastIdx := lastDoubleColon(name); lastIdx >= 0 {
				name = name[lastIdx+2:]
			}
			if _, exists := entries[name]; exists {
				continue
			}
			if !idx.Library(member) {
				return nil, true
			}
			if m.supersUnstable(member) {
				return nil, false
			}
			entries[name] = memberEntry{sym: member}
			firstSteps[name] = -1
		}
	}

	contributors := m.contributors(sym)
	sources := map[*symbols.Symbol]bool{sym: true}
	for i, contributor := range contributors {
		if contributor == nil || contributor == sym {
			continue
		}
		if !idx.Library(contributor) {
			return nil, true
		}
		if m.supersUnstable(contributor) {
			return nil, false
		}
		contributorTable, permanent := m.buildLibraryMemberTable(contributor, visiting, generation)
		if contributorTable == nil {
			if permanent && idx.Generation() == generation {
				m.memberTables[contributor] = nil
			}
			return nil, permanent
		}
		if contributorTable.sources[sym] {
			return nil, true
		}
		for source := range contributorTable.sources {
			sources[source] = true
		}
		for name, entry := range contributorTable.entries {
			candidate := memberEntry{sym: entry.sym, depth: entry.depth + 1}
			current, exists := entries[name]
			if !exists || candidate.depth < current.depth ||
				(candidate.depth == current.depth && i < firstSteps[name]) {
				entries[name] = candidate
				firstSteps[name] = i
			}
		}
	}
	for _, entry := range entries {
		if entry.sym == nil {
			return nil, true
		}
		if m.supersUnstable(entry.sym) {
			return nil, false
		}
	}
	if idx.Generation() != generation {
		return nil, false
	}
	if libraryReferenceInFlight(idx, m.resolvingRef) {
		return nil, false
	}
	table := &memberTable{entries: entries, sources: sources}
	m.memberTables[sym] = table
	return table, false
}

// A user reference cannot change library-only tables, but a library reference can.
func libraryReferenceInFlight(idx *symbols.Index, refs map[*symbols.Symbol]bool) bool {
	for sym := range refs {
		if idx.Library(sym) {
			return true
		}
	}
	return false
}
