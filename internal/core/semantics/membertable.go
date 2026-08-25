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
	table := m.buildLibraryMemberTable(sym, make(map[*symbols.Symbol]bool), generation)
	if idx.Generation() != generation {
		m.memberTables = make(map[*symbols.Symbol]*memberTable)
		m.memberTableGeneration = idx.Generation()
		m.memberPlans = make(map[*symbols.Symbol]*memberPlan)
		m.memberPlanGeneration = idx.Generation()
		return nil
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

func (m *Model) buildLibraryMemberTable(sym *symbols.Symbol, visiting map[*symbols.Symbol]bool, generation uint64) *memberTable {
	idx := m.resolver.Index()
	if idx.Generation() != generation {
		return nil
	}
	if !idx.Library(sym) {
		return nil
	}
	if libraryReferenceInFlight(idx, m.resolvingRef) {
		return nil
	}
	if m.supersUnstable(sym) {
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

	entries := make(map[string]memberEntry)
	firstSteps := make(map[string]int)
	if sym.Scope != nil {
		for _, name := range sym.Scope.MemberNames() {
			member, ok := sym.Scope.LookupLocal(name)
			if !ok {
				continue
			}
			if !idx.Library(member) {
				return nil
			}
			if m.supersUnstable(member) {
				return nil
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
				return nil
			}
			if m.supersUnstable(member) {
				return nil
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
			return nil
		}
		if m.supersUnstable(contributor) {
			return nil
		}
		contributorTable := m.buildLibraryMemberTable(contributor, visiting, generation)
		if contributorTable == nil {
			return nil
		}
		if contributorTable.sources[sym] {
			return nil
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
			return nil
		}
		if m.supersUnstable(entry.sym) {
			return nil
		}
	}
	if idx.Generation() != generation {
		return nil
	}
	if libraryReferenceInFlight(idx, m.resolvingRef) {
		return nil
	}
	table := &memberTable{entries: entries, sources: sources}
	m.memberTables[sym] = table
	return table
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
