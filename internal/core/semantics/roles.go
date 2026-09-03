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

// ImplicitRoleRedefinitions returns the same-role features of the owner's generals
// that sym does not redefine by name: a role redefines every one it inherits.
func (m *Model) ImplicitRoleRedefinitions(sym *symbols.Symbol) []*symbols.Symbol {
	role := roleOf(sym)
	if role == noCaseRole || sym.OwnerScope == nil {
		return nil
	}
	owner := sym.OwnerScope.Owner()
	if !behaviorLike(owner) {
		return nil
	}
	var out []*symbols.Symbol
	seenCases := map[*symbols.Symbol]bool{}
	seenRoles := m.explicitRedefinitions(sym)
	for _, sup := range m.DirectSupertypes(owner) {
		if behaviorLike(sup) {
			for _, inherited := range m.effectiveRoles(sup, role, seenCases) {
				if !seenRoles[inherited] {
					seenRoles[inherited] = true
					out = append(out, inherited)
				}
			}
		}
	}
	return out
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
