package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Pilot SysMLValidator (2026-05) checkAtMostOneFeature and checkSubjectParameter:
// a requirement or case owns at most one subject, and it is the first parameter.
// Cases also own at most one objective.
const (
	msgOnlyOneSubject   = "Only one subject is allowed."
	msgOnlyOneObjective = "Only one objective is allowed."

	msgSubjectParameterPosition = "Subject must be first parameter."
)

func (cc *constraintChecker) checkAtMostOneMember(sym *symbols.Symbol) {
	if sym == nil {
		return
	}
	members := declMembers(sym.Decl)
	if subjectOwnerDecl(sym.Decl) {
		var subjects []ast.Node
		for _, m := range members {
			if isSubjectMemberNode(m) {
				subjects = append(subjects, m)
			}
		}
		// An owned subject redefines the inherited one, so only owned subjects
		// accumulate.
		cc.reportExtraMembers(subjects, msgOnlyOneSubject, "only-one-subject")
		cc.checkSubjectParameterPosition(sym, members)
		cc.checkAtMostOneObjective(sym, members)
	}
}

// checkAtMostOneObjective applies the case objective cardinality rule across
// owned and inherited members. Local objectives compete with an inherited
// objective, while multiple inherited objectives are reported on the owner.
func (cc *constraintChecker) checkAtMostOneObjective(sym *symbols.Symbol, members []ast.Node) {
	var local []ast.Node
	for _, member := range members {
		if isObjectiveMemberNode(member) {
			local = append(local, member)
		}
	}
	inherited := 0
	for _, member := range cc.model.MembersOf(sym) {
		if member == nil || member.OwnerScope == sym.Scope {
			continue
		}
		if isObjectiveDecl(member.Decl) {
			inherited++
		}
	}
	if inherited == 0 {
		cc.reportExtraMembers(local, msgOnlyOneObjective, "only-one-objective")
		return
	}
	if len(local) > 0 {
		cc.reportEachMember(local, msgOnlyOneObjective, "only-one-objective")
		return
	}
	if inherited > 1 {
		cc.diags = append(cc.diags, Diagnostic{
			Severity: SeverityError,
			Span:     sym.Decl.Span(),
			Message:  msgOnlyOneObjective,
			Code:     "only-one-objective",
			Source:   "constraint",
		})
	}
}

// checkSubjectParameterPosition requires the first parameter of a requirement or
// case to be its subject (pilot checkSubjectParameter). It is reported on the
// owned subject when there is one, otherwise on the declaration; a declaration
// with no parameter at all is left to the reference's implicit subject.
func (cc *constraintChecker) checkSubjectParameterPosition(sym *symbols.Symbol, members []ast.Node) {
	var firstInput *symbols.Symbol
	for _, member := range cc.model.MembersOf(sym) {
		if member == nil {
			continue
		}
		if isSubjectDecl(member.Decl) || isInputParameterDecl(member.Decl) {
			firstInput = member
			break
		}
	}
	if firstInput == nil || isSubjectDecl(firstInput.Decl) {
		return
	}
	span := sym.Decl.Span()
	for _, member := range members {
		if isSubjectMemberNode(member) {
			span = member.Span()
			break
		}
	}
	cc.diags = append(cc.diags, Diagnostic{
		Severity: SeverityError,
		Span:     span,
		Message:  msgSubjectParameterPosition,
		Code:     "subject-parameter-position",
		Source:   "constraint",
	})
}

func isSubjectDecl(decl ast.Node) bool {
	switch d := decl.(type) {
	case *ast.SubjectMember:
		return true
	case *ast.Usage:
		return d.Kind == ast.UsageSubject
	}
	return false
}

func isObjectiveDecl(decl ast.Node) bool {
	u, ok := decl.(*ast.Usage)
	return ok && u.Kind == ast.UsageObjective
}

func isInputParameterDecl(decl ast.Node) bool {
	u, ok := decl.(*ast.Usage)
	return ok && (u.Direction == ast.DirIn || u.Direction == ast.DirInOut)
}

// reportExtraMembers errors on every member after the first, the reference's
// behavior when all the competing memberships are owned.
func (cc *constraintChecker) reportExtraMembers(members []ast.Node, msg, code string) {
	if len(members) > 1 {
		cc.reportEachMember(members[1:], msg, code)
	}
}

func (cc *constraintChecker) reportEachMember(members []ast.Node, msg, code string) {
	for _, m := range members {
		cc.diags = append(cc.diags, Diagnostic{
			Severity: SeverityError,
			Span:     m.Span(),
			Message:  msg,
			Code:     code,
			Source:   "constraint",
		})
	}
}

// unwrapUsageMember returns the usage a body member declares, through the
// membership wrapper the parser keeps.
func unwrapUsageMember(n ast.Node) (*ast.Usage, bool) {
	if mem, ok := n.(*ast.Membership); ok && mem != nil {
		n = mem.Member
	}
	u, ok := n.(*ast.Usage)
	return u, ok && u != nil
}

func isSubjectMemberNode(n ast.Node) bool {
	if _, ok := n.(*ast.SubjectMember); ok {
		return true
	}
	u, ok := unwrapUsageMember(n)
	return ok && u.Kind == ast.UsageSubject
}

func isObjectiveMemberNode(n ast.Node) bool {
	u, ok := unwrapUsageMember(n)
	return ok && u.Kind == ast.UsageObjective
}

func stateLikeDecl(decl ast.Node) bool {
	switch d := decl.(type) {
	case *ast.Definition:
		return d.Kind == ast.DefState
	case *ast.Usage:
		return d.Kind == ast.UsageState
	}
	return false
}

func subjectOwnerDecl(decl ast.Node) bool {
	switch d := decl.(type) {
	case *ast.Definition:
		return d.Kind == ast.DefRequirement || isCaseDefKind(d.Kind)
	case *ast.Usage:
		return d.Kind == ast.UsageRequirement || isCaseUsageKind(d.Kind)
	}
	return false
}

func isCaseDefKind(k ast.DefinitionKind) bool {
	switch k {
	case ast.DefCase, ast.DefAnalysisCase, ast.DefVerificationCase, ast.DefUseCase:
		return true
	}
	return false
}

func isCaseUsageKind(k ast.UsageKind) bool {
	switch k {
	case ast.UsageCase, ast.UsageAnalysisCase, ast.UsageVerificationCase, ast.UsageUseCase:
		return true
	}
	return false
}
