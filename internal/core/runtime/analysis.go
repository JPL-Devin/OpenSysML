package runtime

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Qualified names of the trade-study objective definitions whose specialization
// states which way an objective's value is to be improved (SysML v2 §8.3.22.4).
const (
	minimizeObjectiveFQN = "TradeStudies::MinimizeObjective"
	maximizeObjectiveFQN = "TradeStudies::MaximizeObjective"
)

// An objective states the value it improves by restating the trade-study
// library's `best` feature (`attribute :>> best = expression;`), since an
// objective is a requirement usage and carries no scalar value itself.
const (
	tradeStudyObjectiveFQN = "TradeStudies::TradeStudyObjective"
	objectiveBestName      = "best"
)

// ObjectiveDirection is the way an objective's value is to be improved, taken
// from the trade-study objective definition it is typed by.
type ObjectiveDirection int

const (
	// NoDirection is an objective specializing neither MinimizeObjective nor
	// MaximizeObjective, whose direction the model therefore does not state.
	NoDirection ObjectiveDirection = iota
	// Minimize is an objective specializing TradeStudies::MinimizeObjective.
	Minimize
	// Maximize is an objective specializing TradeStudies::MaximizeObjective.
	Maximize
)

// String names the direction as the model states it.
func (d ObjectiveDirection) String() string {
	switch d {
	case Minimize:
		return "minimize"
	case Maximize:
		return "maximize"
	default:
		return "unstated"
	}
}

// Objective is one objective an analysis case states: which way its value is to
// be improved, the expression stating that value, and the conditions it states
// itself.
type Objective struct {
	// Name is the objective's name, empty for an anonymous one.
	Name string

	// Symbol is the objective usage's symbol.
	Symbol *symbols.Symbol

	// Type is the objective definition it is typed by, nil when untyped.
	Type *symbols.Symbol

	// Direction is the way its value is to be improved.
	Direction ObjectiveDirection

	// Value is the expression stating the value to improve, nil when the
	// objective states none.
	Value ast.Node

	// Scope is where Value's names resolve.
	Scope *symbols.Scope

	// Conditions are the conditions the objective states in its own body,
	// inherited ones left out: an objective inherits the trade-study library's
	// conditions, which are about choosing among alternatives rather than about
	// which values are feasible.
	Conditions []Condition
}

// Text renders the expression stating the objective's value as written, empty
// when it states none.
func (o Objective) Text() string {
	if o.Value == nil {
		return ""
	}
	return conditionText(o.Value)
}

// RequireAnalysis returns an ErrNotAnAnalysis usage error unless sym declares an
// analysis case, so a caller can settle the kind before asking for objectives.
func RequireAnalysis(sym *symbols.Symbol) error {
	switch decl := sym.Decl.(type) {
	case *ast.Definition:
		if decl.Kind == ast.DefAnalysisCase {
			return nil
		}
	case *ast.Usage:
		if decl.Kind == ast.UsageAnalysisCase {
			return nil
		}
	}
	return notOfKind(ErrNotAnAnalysis, sym, "analysis case")
}

// ObjectivesOf returns the objectives sym states, its inherited ones first and
// in declaration order. An objective restating an inherited one by name stands
// where it is restated.
func (ctx *Context) ObjectivesOf(sym *symbols.Symbol, scope *symbols.Scope) []Objective {
	if sym == nil {
		return nil
	}
	if scope == nil {
		scope = sym.OwnerScope
	}
	var out []Objective
	for _, member := range ctx.chainMembers(sym, scope) {
		usage, ok := member.node.(*ast.Usage)
		if !ok || usage.Kind != ast.UsageObjective {
			continue
		}
		objSym := memberSymbol(member.scope, member.node)
		if objSym == nil {
			continue
		}
		out = replaceObjective(out, ctx.objectiveOf(objSym, sym))
	}
	return out
}

// replaceObjective appends obj, dropping the objective it restates by name: a
// redeclared objective is the same objective declared again.
func replaceObjective(out []Objective, obj Objective) []Objective {
	if obj.Name != "" {
		for i, prev := range out {
			if prev.Name == obj.Name {
				return append(append(out[:i:i], out[i+1:]...), obj)
			}
		}
	}
	return append(out, obj)
}

// objectiveOf reads one objective usage: its direction from the definition it is
// typed by, its value from the expression it states for the trade-study `best`
// feature, and its conditions from its own body. owner is the case stating it,
// which is where a value it takes from a redeclared objective is looked for.
func (ctx *Context) objectiveOf(objSym, owner *symbols.Symbol) Objective {
	typ := ctx.extractType(objSym)
	value := ctx.extractDefaultValue(objSym)
	valueDecl := objSym
	if value == nil {
		value, valueDecl = ctx.bestValueOf(objSym)
	}
	if value == nil {
		value, valueDecl = ctx.redefinedDefault(objSym, owner)
	}
	if value == nil {
		// An objective restating another takes the value the restated one
		// states for `best`.
		for _, restated := range ctx.relatedFeatures(objSym, owner, ast.RelRedefines) {
			if value, valueDecl = ctx.bestValueOf(restated); value != nil {
				break
			}
		}
	}
	scope := objSym.OwnerScope
	if valueDecl != nil {
		scope = valueDecl.OwnerScope
	}
	return Objective{
		Name:       objSym.Name,
		Symbol:     objSym,
		Type:       typ,
		Direction:  ctx.objectiveDirection(typ),
		Value:      value,
		Scope:      scope,
		Conditions: ctx.ownConditionsOf(objSym),
	}
}

// bestValueOf returns the expression an objective states for the trade-study
// `best` feature in its own body, and the usage that states it.
func (ctx *Context) bestValueOf(objSym *symbols.Symbol) (ast.Node, *symbols.Symbol) {
	if objSym == nil {
		return nil, nil
	}
	body := bodyScope(objSym, objSym.OwnerScope)
	if body == nil {
		return nil, nil
	}
	for _, node := range declMembers(objSym.Decl) {
		member := memberSymbol(body, node)
		if member == nil {
			continue
		}
		value := ctx.extractDefaultValue(member)
		if value == nil || !ctx.statesObjectiveBest(member, ctx.extractType(objSym)) {
			continue
		}
		return value, member
	}
	return nil, nil
}

// statesObjectiveBest reports whether sym restates the `best` feature of an
// objective typed by objType, which must be a trade-study objective for that
// feature to be the library's one.
func (ctx *Context) statesObjectiveBest(sym, objType *symbols.Symbol) bool {
	if sym == nil || !restatesFeatureNamed(sym, objectiveBestName) {
		return false
	}
	return ctx.specializesLibraryType(objType, tradeStudyObjectiveFQN)
}

// restatesFeatureNamed reports whether sym redefines a feature of the given
// name, whichever way the redefinition names it.
func restatesFeatureNamed(sym *symbols.Symbol, name string) bool {
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel == nil || rel.Kind != ast.RelRedefines || rel.Target == nil {
			continue
		}
		target := rel.Target
		if ref, ok := target.(*ast.FeatureReference); ok {
			target = ref.Name
		}
		qn, ok := target.(*ast.QualifiedName)
		if !ok || len(qn.Parts) == 0 {
			continue
		}
		if qn.Parts[len(qn.Parts)-1].Text == name {
			return true
		}
	}
	return false
}

// specializesLibraryType reports whether typ is the library type the qualified
// name states, or specializes it, matched by identity so a type merely named
// alike is not one.
func (ctx *Context) specializesLibraryType(typ *symbols.Symbol, fqn string) bool {
	if typ == nil || ctx.resolver == nil || ctx.resolver.Index() == nil {
		return false
	}
	stated := make(map[*symbols.Symbol]bool, 1)
	for _, sym := range ctx.resolver.Index().LookupQualified(fqn) {
		if sym != nil {
			stated[sym] = true
		}
	}
	if stated[typ] {
		return true
	}
	for _, super := range ctx.model.AllSupertypes(typ) {
		if stated[super] {
			return true
		}
	}
	return false
}

// objectiveDirection classifies an objective definition against the trade-study
// library: the nearest of MinimizeObjective and MaximizeObjective it
// specializes, matched by identity so a type merely named alike is not one.
func (ctx *Context) objectiveDirection(typ *symbols.Symbol) ObjectiveDirection {
	if typ == nil || ctx.resolver == nil || ctx.resolver.Index() == nil {
		return NoDirection
	}
	idx := ctx.resolver.Index()
	directions := make(map[*symbols.Symbol]ObjectiveDirection, 2)
	for fqn, direction := range map[string]ObjectiveDirection{
		minimizeObjectiveFQN: Minimize,
		maximizeObjectiveFQN: Maximize,
	} {
		for _, sym := range idx.LookupQualified(fqn) {
			if sym != nil {
				directions[sym] = direction
			}
		}
	}
	if direction, ok := directions[typ]; ok {
		return direction
	}
	// AllSupertypes is breadth-first over declaration order, so the nearest
	// trade-study ancestor is found first.
	for _, super := range ctx.model.AllSupertypes(typ) {
		if direction, ok := directions[super]; ok {
			return direction
		}
	}
	return NoDirection
}

// ownConditionsOf returns the conditions sym states in its own body, leaving out
// the ones it inherits from the definitions it is typed by.
func (ctx *Context) ownConditionsOf(sym *symbols.Symbol) []Condition {
	if sym == nil {
		return nil
	}
	body := bodyScope(sym, sym.OwnerScope)
	var members []scopedMember
	for _, node := range declMembers(sym.Decl) {
		members = append(members, scopedMember{node: node, scope: body})
	}
	return conditionsOf(members)
}

// CaseConditionsOf returns the conditions a case states as what it holds true of
// its parameters: the conditions its own members state, and the ones stated by
// the constraints it requires, assumes or asserts in its body. A constraint
// declared without one of those keywords states nothing the case checks, so it
// is left out.
func (ctx *Context) CaseConditionsOf(sym *symbols.Symbol, scope *symbols.Scope) []Condition {
	if sym == nil {
		return nil
	}
	if scope == nil {
		scope = sym.OwnerScope
	}
	var out []Condition
	for _, member := range ctx.chainMembers(sym, scope) {
		out = appendConditions(out, member.node, member.scope, true, false)
		out = ctx.appendCheckedConstraint(out, member)
	}
	return out
}

// appendCheckedConstraint appends the conditions a nested constraint usage
// states when the case requires, assumes, asserts or states it as an invariant.
// A negation it wrote negates what its conditions state together (De Morgan),
// as a negated constraint body does.
func (ctx *Context) appendCheckedConstraint(out []Condition, member scopedMember) []Condition {
	usage, ok := member.node.(*ast.Usage)
	if !ok || usage.Kind != ast.UsageConstraint {
		return out
	}
	// The keyword sits ahead of a kind keyword (`require constraint { … }`) or
	// stands for the kind itself (`require c : C`, `inv { … }`).
	keyword := usage.PrefixKeyword
	if keyword == "" {
		keyword = usage.Keyword
	}
	var required bool
	switch keyword {
	case "require", "assert", "inv":
		required = true
	case "assume":
		required = false
	default:
		return out
	}
	sym := memberSymbol(member.scope, member.node)
	if sym == nil {
		return out
	}
	conds := ctx.ConditionsOf(sym, nil)
	for i := range conds {
		conds[i].Required = required
	}
	if !usage.IsNegated {
		return append(out, conds...)
	}
	if len(conds) == 1 {
		only := conds[0]
		only.Negated = !only.Negated
		return append(out, only)
	}
	return append(out, Condition{Group: conds, Negated: true, Required: required})
}

// memberSymbol returns the symbol scope declares for node, anonymous ones
// included, or nil when there is none.
func memberSymbol(scope *symbols.Scope, node ast.Node) *symbols.Symbol {
	if scope == nil || node == nil {
		return nil
	}
	for _, member := range scope.AllMembers() {
		if member.Decl == node {
			return member
		}
	}
	return nil
}
