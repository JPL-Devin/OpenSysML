package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

type caseRole uint8

const (
	noCaseRole caseRole = iota
	subjectRole
	objectiveRole
)

// ImplicitRoleRedefinitions returns the same-role features of the owner's generals that sym
// does not redefine by name: every one for a subject, each general's first for a first objective.
func (m *Model) ImplicitRoleRedefinitions(sym *symbols.Symbol) []*symbols.Symbol {
	role := roleOf(sym)
	if role == noCaseRole || sym.OwnerScope == nil {
		return nil
	}
	owner := sym.OwnerScope.Owner()
	if !behaviorLike(owner) {
		return nil
	}
	if role == objectiveRole {
		if owned := ownedRoles(owner, role); len(owned) == 0 || owned[0] != sym {
			return nil
		}
	}
	var out []*symbols.Symbol
	seenCases := map[*symbols.Symbol]bool{}
	seenRoles := m.explicitRedefinitions(sym)
	for _, sup := range m.DirectSupertypes(owner) {
		if !behaviorLike(sup) {
			continue
		}
		inherited := m.effectiveRoles(sup, role, seenCases)
		if role == objectiveRole && len(inherited) > 1 {
			inherited = inherited[:1]
		}
		for _, f := range inherited {
			if !seenRoles[f] {
				seenRoles[f] = true
				out = append(out, f)
			}
		}
	}
	return out
}

// ObjectivesOf returns the objectives a case owns and the inherited ones that no
// owned or inherited objective redefines, by clause or by role.
func (m *Model) ObjectivesOf(sym *symbols.Symbol) (owned, inherited []*symbols.Symbol) {
	return m.visibleRoles(sym, objectiveRole)
}

// SubjectsOf is ObjectivesOf for the subjects of a requirement or case.
func (m *Model) SubjectsOf(sym *symbols.Symbol) (owned, inherited []*symbols.Symbol) {
	return m.visibleRoles(sym, subjectRole)
}

func (m *Model) visibleRoles(sym *symbols.Symbol, role caseRole) (owned, inherited []*symbols.Symbol) {
	if m == nil || sym == nil {
		return nil, nil
	}
	owned = ownedRoles(sym, role)
	var reachable []*symbols.Symbol
	seen := map[*symbols.Symbol]bool{sym: true}
	for _, sup := range m.DirectSupertypes(sym) {
		reachable = m.collectRoles(sup, role, seen, reachable)
	}
	masked := map[*symbols.Symbol]bool{}
	for _, o := range append(append([]*symbols.Symbol{}, owned...), reachable...) {
		for target := range m.explicitRedefinitions(o) {
			masked[target] = true
		}
		for _, target := range m.ImplicitRoleRedefinitions(o) {
			masked[target] = true
		}
	}
	for _, f := range reachable {
		if !masked[f] {
			inherited = append(inherited, f)
		}
	}
	return owned, inherited
}

// collectRoles appends the role features sym owns or inherits, each once.
func (m *Model) collectRoles(sym *symbols.Symbol, role caseRole, seen map[*symbols.Symbol]bool, out []*symbols.Symbol) []*symbols.Symbol {
	if sym == nil || seen[sym] || !behaviorLike(sym) {
		return out
	}
	seen[sym] = true
	out = append(out, ownedRoles(sym, role)...)
	for _, sup := range m.DirectSupertypes(sym) {
		out = m.collectRoles(sup, role, seen, out)
	}
	return out
}

// SubjectParameterOf returns the subject parameter of a requirement or case,
// owned or inherited along its generals, or nil when it has none.
func (m *Model) SubjectParameterOf(sym *symbols.Symbol) *symbols.Symbol {
	if m == nil || sym == nil {
		return nil
	}
	subjects := m.effectiveRoles(sym, subjectRole, map[*symbols.Symbol]bool{})
	if len(subjects) == 0 {
		return nil
	}
	return subjects[0]
}

func (m *Model) effectiveRoles(sym *symbols.Symbol, role caseRole, seen map[*symbols.Symbol]bool) []*symbols.Symbol {
	if sym == nil || seen[sym] {
		return nil
	}
	seen[sym] = true
	if owned := ownedRoles(sym, role); len(owned) > 0 {
		return owned
	}
	var out []*symbols.Symbol
	for _, sup := range m.DirectSupertypes(sym) {
		if behaviorLike(sup) {
			out = append(out, m.effectiveRoles(sup, role, seen)...)
		}
	}
	return out
}

func ownedRoles(sym *symbols.Symbol, role caseRole) []*symbols.Symbol {
	if sym == nil || sym.Scope == nil {
		return nil
	}
	var out []*symbols.Symbol
	for _, member := range declMembers(sym) {
		if roleOfNode(member) != role {
			continue
		}
		node := member
		if wrapper, ok := member.(*ast.Membership); ok {
			node = wrapper.Member
		}
		if found := memberSymbol(sym.Scope, node); found != nil {
			out = append(out, found)
		}
	}
	return out
}

func roleOf(sym *symbols.Symbol) caseRole {
	if sym == nil {
		return noCaseRole
	}
	return roleOfNode(sym.Decl)
}

func roleOfNode(node ast.Node) caseRole {
	if wrapper, ok := node.(*ast.Membership); ok {
		node = wrapper.Member
	}
	switch d := node.(type) {
	case *ast.SubjectMember:
		return subjectRole
	case *ast.Usage:
		switch d.Kind {
		case ast.UsageSubject:
			return subjectRole
		case ast.UsageObjective:
			return objectiveRole
		}
	}
	return noCaseRole
}

// explicitRedefinitions returns the features sym's own `:>>` clauses resolve to.
func (m *Model) explicitRedefinitions(sym *symbols.Symbol) map[*symbols.Symbol]bool {
	out := map[*symbols.Symbol]bool{}
	for _, rel := range RelationshipsOf(sym) {
		if rel == nil || rel.Kind != ast.RelRedefines || rel.Target == nil {
			continue
		}
		if target := m.resolveRelTarget(sym, rel); target != nil {
			out[target] = true
		}
	}
	return out
}
