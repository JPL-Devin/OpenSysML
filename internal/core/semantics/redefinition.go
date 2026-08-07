package semantics

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// This file implements the implicit redefinition of parameters described by
// KerML 7.4.7.2 (Behavior Declaration) and 7.4.7.3 (Step Declaration), and by
// SysML v2 7.17.2 (Action Definitions and Usages) and 7.19.2 (Calculation
// Definitions and Usages):
//
//   - The directed features declared in the body of a behavior or step are its
//     parameters, ordered by the lexical order of their declarations.
//   - For each general behavior or step of the owning behavior or step, an
//     owned parameter that declares no redefinition of its own implicitly
//     redefines the parameter at the same position of that general type. The
//     redefining parameter has the same direction as the redefined one.
//   - A result parameter (declared with `return`) instead redefines the result
//     parameter of each general type, regardless of position.
//
// Redefinition is a kind of subsetting, hence a generalization: the redefining
// parameter takes the redefined parameter's type (and everything else it
// inherits) unless it declares its own. That is what makes `out image` in
// `action focus : Focus` a use of `Focus::image`, typed by `Image`.

// behaviorLike reports whether sym declares a behavior or step — the only
// owning types whose directed features are parameters, and the only general
// types whose parameters are implicitly redefined.
func behaviorLike(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	switch d := sym.Decl.(type) {
	case *ast.Definition:
		switch d.Kind {
		case ast.DefAction, ast.DefState, ast.DefCalc, ast.DefConstraint,
			ast.DefRequirement, ast.DefCase, ast.DefAnalysisCase,
			ast.DefVerificationCase, ast.DefUseCase, ast.DefBehavior,
			ast.DefPredicate, ast.DefBool:
			return true
		}
	case *ast.Usage:
		switch d.Kind {
		case ast.UsageAction, ast.UsageState, ast.UsageCalc, ast.UsageExpr,
			ast.UsageConstraint, ast.UsageRequirement, ast.UsageCase,
			ast.UsageAnalysisCase, ast.UsageVerificationCase, ast.UsageUseCase,
			ast.UsageStep, ast.UsageBehavior, ast.UsagePredicate, ast.UsageBool:
			return true
		}
	}
	return false
}

// declMembers returns the body member nodes of a def/usage declaration in
// lexical order, or nil for anything else.
func declMembers(sym *symbols.Symbol) []ast.Node {
	switch d := sym.Decl.(type) {
	case *ast.Definition:
		return d.Members
	case *ast.Usage:
		return d.Members
	default:
		return nil
	}
}

// parameter is one owned parameter of a behavior or step: the symbol declared
// for a directed feature in its body, plus whether it is the result parameter.
type parameter struct {
	sym      *symbols.Symbol
	usage    *ast.Usage
	isResult bool
}

// behaviorParameters is the parameter list of a behavior or step: the
// parameters that are matched by position, in order, and the result parameter,
// which is matched as such rather than by position. Both include parameters
// inherited from a general behavior or step and not redefined.
type behaviorParameters struct {
	positional []parameter
	result     parameter
}

// parametersOf returns the effective parameters of the behavior or step sym:
// its owned parameters, in declaration order, followed by the parameters it
// inherits and does not redefine (KerML 7.4.7.2: parameters of a single general
// behavior beyond those redefined are inherited, ordered after the owned ones).
// The result is memoized.
func (m *Model) parametersOf(sym *symbols.Symbol) behaviorParameters {
	if cached, ok := m.params[sym]; ok {
		return cached
	}
	// Guard against re-entrancy on cyclic specialization graphs.
	m.params[sym] = behaviorParameters{}

	owned := ownedParameters(sym)
	out := behaviorParameters{positional: positionalParameters(owned), result: resultParameter(owned)}

	// Only a single general behavior or step may leave parameters inherited;
	// with more than one, every parameter has to be redefined by an owned one.
	var generals []*symbols.Symbol
	for _, sup := range m.DirectSupertypes(sym) {
		if behaviorLike(sup) {
			generals = append(generals, sup)
		}
	}
	if len(generals) == 1 {
		general := m.parametersOf(generals[0])
		claimed := claimedParameters(out, general)
		for _, p := range general.positional {
			if !claimed[p.sym] {
				out.positional = append(out.positional, p)
			}
		}
		if out.result.sym == nil && !claimed[general.result.sym] {
			out.result = general.result
		}
	}

	m.params[sym] = out
	return out
}

// claimedParameters returns the parameters of general that owned redefines,
// each by the target its declaration names explicitly or, failing that, the one
// at its own position with the same direction. Only what no owned parameter
// claims is inherited.
func claimedParameters(owned, general behaviorParameters) map[*symbols.Symbol]bool {
	claimed := make(map[*symbols.Symbol]bool)
	for i, p := range owned.positional {
		if explicit := namedParameters(p.usage, general); len(explicit) > 0 {
			for _, target := range explicit {
				claimed[target.sym] = true
			}
			continue
		}
		// A position whose directions disagree is not a redefinition, so the
		// general parameter there is neither redefined nor dropped.
		if i < len(general.positional) && general.positional[i].usage.Direction == p.usage.Direction {
			claimed[general.positional[i].sym] = true
		}
	}
	if owned.result.sym != nil {
		for _, target := range namedParameters(owned.result.usage, general) {
			claimed[target.sym] = true
		}
		claimed[general.result.sym] = true
	}
	delete(claimed, nil)
	return claimed
}

// namedParameters returns the parameters of general that a declaration's `:>>`
// clauses name, matching on the last segment of each qualified name.
func namedParameters(u *ast.Usage, general behaviorParameters) []parameter {
	var out []parameter
	for _, rel := range u.Relationships {
		if rel == nil || rel.Kind != ast.RelRedefines {
			continue
		}
		qn, ok := rel.Target.(*ast.QualifiedName)
		if !ok || len(qn.Parts) == 0 {
			continue
		}
		name := qn.Parts[len(qn.Parts)-1].Text
		for _, p := range general.positional {
			if p.sym != nil && p.sym.Name == name {
				out = append(out, p)
			}
		}
		if general.result.sym != nil && general.result.sym.Name == name {
			out = append(out, general.result)
		}
	}
	return out
}

// ownedParameters returns the parameters owned by sym in declaration order.
// Only symbols the scope builder created are reported, so a parameter of a
// library symbol with no parsed declaration yields nothing.
func ownedParameters(sym *symbols.Symbol) []parameter {
	if sym == nil || sym.Scope == nil {
		return nil
	}
	var out []parameter
	for _, member := range declMembers(sym) {
		usage, ok := unwrapUsage(member)
		if !ok || usage.Direction == ast.DirNone {
			continue
		}
		found := memberSymbol(sym.Scope, usage)
		if found == nil {
			continue
		}
		out = append(out, parameter{sym: found, usage: usage, isResult: usage.IsResult})
	}
	return out
}

// unwrapUsage returns the usage a body member node declares, unwrapping the
// membership wrapper the parser puts around visibility-carrying members.
func unwrapUsage(member ast.Node) (*ast.Usage, bool) {
	if wrapper, ok := member.(*ast.Membership); ok {
		member = wrapper.Member
	}
	usage, ok := member.(*ast.Usage)
	return usage, ok
}

// memberSymbol returns the symbol scope declares for node, including anonymous
// ones (`in item;` declares a parameter with no name), or nil if there is none.
func memberSymbol(scope *symbols.Scope, node ast.Node) *symbols.Symbol {
	for _, member := range scope.AllMembers() {
		if member.Decl == node {
			return member
		}
	}
	return nil
}

// implicitParameterRedefinitions returns the features sym implicitly redefines
// as a parameter of its owning behavior or step: the parameter at the same
// position of each general behavior or step, or, for a result parameter, their
// result parameters. It returns nothing for a feature that is not a parameter,
// or whose declaration redefines something explicitly.
func (m *Model) implicitParameterRedefinitions(sym *symbols.Symbol) []*symbols.Symbol {
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok || usage.Direction == ast.DirNone {
		return nil
	}
	for _, rel := range usage.Relationships {
		if rel != nil && rel.Kind == ast.RelRedefines {
			return nil // explicit redefinition governs
		}
	}
	if sym.OwnerScope == nil {
		return nil
	}
	owner := sym.OwnerScope.Owner()
	if !behaviorLike(owner) {
		return nil
	}
	position := -1
	for i, p := range positionalParameters(ownedParameters(owner)) {
		if p.sym == sym {
			position = i
			break
		}
	}
	if position < 0 && !usage.IsResult {
		return nil
	}

	var out []*symbols.Symbol
	for _, sup := range m.DirectSupertypes(owner) {
		if !behaviorLike(sup) {
			continue
		}
		supParams := m.parametersOf(sup)
		var target parameter
		if usage.IsResult {
			target = supParams.result
		} else {
			if position >= len(supParams.positional) {
				continue
			}
			target = supParams.positional[position]
		}
		if target.sym == nil || target.sym == sym {
			continue
		}
		// A redefining parameter has the same direction as the parameter it
		// redefines; a position whose directions disagree is not a redefinition.
		if target.usage.Direction != usage.Direction {
			continue
		}
		out = append(out, target.sym)
	}
	return out
}

// positionalParameters returns the parameters that participate in redefinition
// by position: every owned parameter except the result parameter, which is
// redefined by name of its role rather than by position.
func positionalParameters(params []parameter) []parameter {
	var out []parameter
	for _, p := range params {
		if p.isResult {
			continue
		}
		out = append(out, p)
	}
	return out
}

// resultParameter returns the result parameter among params, or the zero value
// when there is none.
func resultParameter(params []parameter) parameter {
	for _, p := range params {
		if p.isResult {
			return p
		}
	}
	return parameter{}
}
