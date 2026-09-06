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

// viewRenderingFQN is the library feature every `render` member redefines
// (SysML v2 8.3.26 RenderingUsage).
const viewRenderingFQN = "Views::View::viewRendering"

// ImplicitRoleRedefinitions returns the features sym redefines by its role: the
// same-role features of the owner's generals for a subject or objective, and the
// library `viewRendering` for a view's `render` member.
func (m *Model) ImplicitRoleRedefinitions(sym *symbols.Symbol) []*symbols.Symbol {
	if sym == nil || sym.OwnerScope == nil {
		return nil
	}
	if isViewRendering(sym.Decl) {
		return m.viewRenderingRedefinition(sym)
	}
	role := roleOf(sym)
	if role == noCaseRole {
		return nil
	}
	owner := sym.OwnerScope.Owner()
	if owner == nil || !behaviorLike(owner) {
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

// viewRenderingRedefinition is the library `viewRendering` a `render` member
// redefines, unless a clause of its own already names it.
func (m *Model) viewRenderingRedefinition(sym *symbols.Symbol) []*symbols.Symbol {
	if m.resolver == nil || m.resolver.Index() == nil {
		return nil
	}
	explicit := m.explicitRedefinitions(sym)
	for _, lib := range m.resolver.Index().LookupQualified(viewRenderingFQN) {
		if lib != nil && lib != sym && !explicit[lib] {
			return []*symbols.Symbol{lib}
		}
	}
	return nil
}

func isViewRendering(node ast.Node) bool {
	if wrapper, ok := node.(*ast.Membership); ok {
		node = wrapper.Member
	}
	usage, ok := node.(*ast.Usage)
	return ok && usage.Kind == ast.UsageViewRendering
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
		if target := m.relationshipTarget(sym, rel); target != nil {
			out[target] = true
		}
	}
	return out
}
