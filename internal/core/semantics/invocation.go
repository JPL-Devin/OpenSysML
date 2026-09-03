package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Argument is one invocation argument as the checker types it; Exact holds of
// a literal, whose type is written out rather than only bounded.
type Argument struct {
	Prim  PrimType
	Type  *symbols.Symbol // the declared type of the feature or result named, nil when none
	Exact bool
	Name  string // the parameter a named argument binds to; "" for positional
}

// known reports whether the checker knows anything about the argument's type.
func (a Argument) known() bool {
	return a.Prim != PrimUnknown || a.Type != nil
}

// ArgumentTyper types a call's arguments as the checker does, so a call read
// anywhere in the model selects the overload the checker selects.
type ArgumentTyper interface {
	InvocationArguments(scope *symbols.Scope, e *ast.InvocationExpr) []Argument
}

// SetArgumentTyper installs the checker's argument typing on the model.
func (m *Model) SetArgumentTyper(t ArgumentTyper) {
	m.arguments = t
}

// Performs is what a call site does with the declaration it names, which
// decides the kinds of declaration it may select.
type Performs int

const (
	// PerformsBehavior evaluates an expression: any behavior answers.
	PerformsBehavior Performs = iota
	// PerformsAction runs an action, as `action a = tag(x);` does: only actions answer.
	PerformsAction
)

// Performable reports whether a call site of kind p can run sym: a behavior, or for an
// evaluated call a feature typed by one, which performs that behavior.
func (m *Model) Performable(p Performs, sym *symbols.Symbol) bool {
	if p == PerformsAction {
		return sym.Kind == symbols.SymbolActionDef || sym.Kind == symbols.SymbolActionUsage
	}
	visited := map[*symbols.Symbol]bool{}
	for sym != nil && !visited[sym] {
		if behaviorLike(sym) {
			return true
		}
		visited[sym] = true
		sym = m.featureType(sym)
	}
	return false
}

// InvocationSelection is which declaration an invocation calls, out of every
// declaration its written name is visible as.
type InvocationSelection struct {
	Candidates []*symbols.Symbol // what the name denotes, in lookup order, aliases followed
	Applicable []*symbols.Symbol // the candidates the arguments bind to
	Selected   *symbols.Symbol   // the one called; nil when none applies or they tie
	Ambiguous  bool              // several applicable candidates are equally specific
	Tied       []*symbols.Symbol // the applicable candidates none is more specific than, when Ambiguous
	callable   *symbols.Symbol   // the first candidate the call site can run, when any is
}

// Resolved reports whether the name denotes at least one declaration.
func (s *InvocationSelection) Resolved() bool {
	return s != nil && len(s.Candidates) > 0
}

// Called is the declaration a call runs: the one selected, or when no candidate fits the
// first callable one (else the first at all), which then reports the mismatch; nil when none.
func (s *InvocationSelection) Called() *symbols.Symbol {
	switch {
	case s == nil:
		return nil
	case s.Selected != nil:
		return s.Selected
	case s.callable != nil:
		return s.callable
	case len(s.Candidates) > 0:
		return s.Candidates[0]
	}
	return nil
}

// invocationKey identifies an invocation expression in the scope and the kind of
// call site it is read in.
type invocationKey struct {
	node     *ast.InvocationExpr
	scope    *symbols.Scope
	performs Performs
}

// SelectInvocation chooses the declaration e calls: the most specific candidate the arguments
// fit, or the first in lookup order when an argument's type is unknown. Memoized per node/scope.
func (m *Model) SelectInvocation(scope *symbols.Scope, e *ast.InvocationExpr, args []Argument, performs Performs) *InvocationSelection {
	if m == nil || e == nil || e.Type == nil {
		return &InvocationSelection{}
	}
	key := invocationKey{node: e, scope: scope, performs: performs}
	if sel, ok := m.invocations[key]; ok {
		return sel
	}
	sel := m.selectInvocation(scope, e, args, performs)
	m.invocations[key] = sel
	return sel
}

func (m *Model) selectInvocation(scope *symbols.Scope, e *ast.InvocationExpr, args []Argument, performs Performs) *InvocationSelection {
	sel := &InvocationSelection{}
	for _, sym := range m.resolver.InvocationCandidates(scope, e.Type) {
		target, ok := m.resolver.ResolveAliasTarget(sym)
		if !ok || target == nil {
			continue
		}
		if !containsSymbol(sel.Candidates, target) {
			sel.Candidates = append(sel.Candidates, target)
		}
	}
	if len(sel.Candidates) == 0 {
		return sel
	}
	behaviors := make([]*symbols.Symbol, 0, len(sel.Candidates))
	for _, c := range sel.Candidates {
		if m.Performable(performs, c) {
			behaviors = append(behaviors, c)
		}
	}
	switch {
	case len(behaviors) == 0:
		// Nothing the call site can run: the checker reports the first as it always has.
		sel.Selected = sel.Candidates[0]
		return sel
	case len(sel.Candidates) == 1:
		sel.Applicable = sel.Candidates
		sel.Selected = sel.Candidates[0]
		return sel
	}
	sel.callable = behaviors[0]

	signatures := make([]invocationSignature, len(behaviors))
	for i, c := range behaviors {
		signatures[i] = m.signatureOf(c)
	}
	// Candidates the arguments conform to are preferred; failing any, the ones
	// they may fit at run time are kept, and no tie among those is reported.
	applicable, sigs, fits := m.filterApplicable(behaviors, signatures, args, bindStrict)
	strict := len(applicable) > 0
	if !strict {
		applicable, sigs, fits = m.filterApplicable(behaviors, signatures, args, bindLoose)
	}
	sel.Applicable = applicable
	switch len(applicable) {
	case 0:
		return sel
	case 1:
		sel.Selected = applicable[0]
		return sel
	}
	if !strict || !argumentsKnown(args) {
		sel.Selected = applicable[0]
		return sel
	}
	// Specificity decides only among candidates the arguments surely fit: exact matches, else
	// widenings when no candidate takes them through an undetermined type; else lookup order.
	decisive := indicesWithFit(fits, fitExact)
	if len(decisive) == 0 && !containsFit(fits, fitOpen) {
		decisive = indicesWithFit(fits, fitWiden)
	}
	if len(decisive) == 0 {
		sel.Selected = applicable[0]
		return sel
	}
	decisiveSigs := make([]invocationSignature, len(decisive))
	for i, k := range decisive {
		decisiveSigs[i] = sigs[k]
	}
	if best := m.mostSpecific(decisiveSigs, args); best >= 0 {
		sel.Selected = applicable[decisive[best]]
		return sel
	}
	sel.Ambiguous = true
	for _, k := range m.unbeaten(decisiveSigs, args) {
		sel.Tied = append(sel.Tied, applicable[decisive[k]])
	}
	return sel
}

// bindMode is how closely an argument's type must fit its parameter's.
type bindMode int

const (
	bindLoose  bindMode = iota // the argument's type may only bound its values
	bindStrict                 // the argument's type conforms to the parameter's
)

// bindFit is how surely a candidate's parameters take the arguments, weakest
// parameter deciding.
type bindFit int

const (
	fitExact bindFit = iota // declared types conform, or the same scalar type
	fitWiden                // the argument widens to a wider parameter type, Anything included
	fitOpen                 // a type the checker cannot determine, so anything may fit
)

// filterApplicable keeps the candidates whose signature args bind to, with how
// surely each takes them.
func (m *Model) filterApplicable(cands []*symbols.Symbol, sigs []invocationSignature, args []Argument, mode bindMode) ([]*symbols.Symbol, []invocationSignature, []bindFit) {
	var outCands []*symbols.Symbol
	var outSigs []invocationSignature
	var outFits []bindFit
	for i, sig := range sigs {
		if fit, ok := m.applicable(sig, args, mode); ok {
			outCands = append(outCands, cands[i])
			outSigs = append(outSigs, sig)
			outFits = append(outFits, fit)
		}
	}
	return outCands, outSigs, outFits
}

func indicesWithFit(fits []bindFit, want bindFit) []int {
	var out []int
	for i, fit := range fits {
		if fit == want {
			out = append(out, i)
		}
	}
	return out
}

func containsFit(fits []bindFit, want bindFit) bool {
	return len(indicesWithFit(fits, want)) > 0
}

// invocationSignature is the input parameters a candidate is invoked with.
type invocationSignature struct {
	params []signatureParameter
	known  bool // false when the parameters cannot be determined, so anything fits
}

type signatureParameter struct {
	name     string
	typ      *symbols.Symbol // the declared type, nil when untyped or unresolved
	prim     PrimType
	untyped  bool // typed Anything, whether written or by declaring no type
	optional bool // may go without an argument: a default, or a multiplicity admitting none
}

// signatureOf returns sym's effective input parameters, in signature order.
func (m *Model) signatureOf(sym *symbols.Symbol) invocationSignature {
	var sig invocationSignature
	for _, p := range m.BehaviorParametersOf(sym) {
		if p.IsResult || (p.Direction != ast.DirIn && p.Direction != ast.DirInOut) {
			continue
		}
		name := p.Symbol.Name
		u, isUsage := p.Symbol.Decl.(*ast.Usage)
		if isUsage {
			if effective, _ := ast.EffectiveName(u); effective != "" {
				name = effective
			}
		}
		param := signatureParameter{
			name:     name,
			typ:      m.featureType(p.Symbol),
			prim:     m.PrimTypeOf(p.Symbol),
			optional: m.optionalParameter(p.Symbol),
		}
		switch {
		case isAnything(param.typ):
			param.typ, param.untyped = nil, true
		case param.typ == nil && param.prim == PrimUnknown && !m.declaresType(p.Symbol):
			param.untyped = true
		}
		sig.params = append(sig.params, param)
	}
	// A parameterless declaration with supertypes may inherit an unseen signature.
	sig.known = len(sig.params) > 0 || (sym.Decl != nil && len(m.DirectSupertypes(sym)) == 0)
	return sig
}

// optionalParameter reports whether a call may omit the parameter: it or a parameter it
// redefines declares a default, or the nearest stated multiplicity admits no value.
func (m *Model) optionalParameter(sym *symbols.Symbol) bool {
	chain := m.parameterRedefinitionChain(sym)
	for _, u := range chain {
		if u.Value != nil {
			return true
		}
	}
	for _, u := range chain {
		if u.Multiplicity != nil {
			return m.IsOptionalParameter(u)
		}
	}
	return false
}

// parameterRedefinitionChain is sym's declaration followed by those of the parameters it
// redefines, explicitly or by position, nearest first.
func (m *Model) parameterRedefinitionChain(sym *symbols.Symbol) []*ast.Usage {
	var chain []*ast.Usage
	visited := map[*symbols.Symbol]bool{}
	for sym != nil && !visited[sym] {
		visited[sym] = true
		u, ok := sym.Decl.(*ast.Usage)
		if !ok {
			break
		}
		chain = append(chain, u)
		redefined := m.RedefinedFeatures(sym)
		if len(redefined) == 0 {
			redefined = m.implicitParameterRedefinitions(sym)
		}
		if len(redefined) != 1 {
			break
		}
		sym = redefined[0]
	}
	return chain
}

// declaresType reports whether sym writes a type or generalization, or implicitly
// redefines a parameter it may take one from.
func (m *Model) declaresType(sym *symbols.Symbol) bool {
	for _, rel := range RelationshipsOf(sym) {
		if rel != nil && GeneralizationKind(rel.Kind) {
			return true
		}
	}
	return len(m.implicitParameterRedefinitions(sym)) > 0
}

// applicable reports whether args bind to sig by count, name and type, and
// how surely.
func (m *Model) applicable(sig invocationSignature, args []Argument, mode bindMode) (bindFit, bool) {
	if !sig.known {
		return fitOpen, true
	}
	bound := make([]bool, len(sig.params))
	positional := 0
	fit := fitExact
	for _, arg := range args {
		var i int
		if arg.Name == "" {
			i = positional
			positional++
		} else {
			i = sig.index(arg.Name)
		}
		if i < 0 || i >= len(sig.params) || bound[i] {
			return fitOpen, false
		}
		bound[i] = true
		argFit, ok := m.argumentBinds(arg, sig.params[i], mode)
		if !ok {
			return fitOpen, false
		}
		if argFit > fit {
			fit = argFit
		}
	}
	for i, p := range sig.params {
		if !bound[i] && !p.optional {
			return fitOpen, false
		}
	}
	return fit, true
}

func (sig invocationSignature) index(name string) int {
	for i, p := range sig.params {
		if p.name == name {
			return i
		}
	}
	return -1
}

// argumentBinds reports whether and how surely arg may be passed to p: strictly by conformance
// alone; loosely a non-literal's type only bounds its values, so either direction fits.
func (m *Model) argumentBinds(arg Argument, p signatureParameter, mode bindMode) (bindFit, bool) {
	if arg.Type != nil && p.typ != nil {
		if m.Conforms(arg.Type, p.typ) {
			return fitExact, true
		}
		if mode == bindStrict {
			return fitOpen, false
		}
	}
	if p.untyped {
		return fitWiden, true
	}
	if PrimConforms(arg.Prim, p.prim) {
		switch {
		case arg.Prim == PrimUnknown || p.prim == PrimUnknown:
			return fitOpen, true
		case arg.Prim == p.prim:
			return fitExact, true
		}
		return fitWiden, true
	}
	if mode == bindLoose && !arg.Exact && PrimConforms(p.prim, arg.Prim) {
		return fitOpen, true
	}
	return fitOpen, false
}

func argumentsKnown(args []Argument) bool {
	for _, arg := range args {
		if !arg.known() {
			return false
		}
	}
	return true
}

// mostSpecific returns the index of the one signature at least as specific as
// every other at the bound parameters, or -1 when none or several are.
func (m *Model) mostSpecific(sigs []invocationSignature, args []Argument) int {
	best := -1
	for i := range sigs {
		leads := true
		for j := range sigs {
			if i != j && !m.atLeastAsSpecific(sigs[i], sigs[j], args) {
				leads = false
				break
			}
		}
		if !leads {
			continue
		}
		if best >= 0 {
			return -1
		}
		best = i
	}
	return best
}

// unbeaten returns the indices of the signatures no other is strictly more specific
// than at the bound parameters; every index when each is beaten by another.
func (m *Model) unbeaten(sigs []invocationSignature, args []Argument) []int {
	var out []int
	for i := range sigs {
		beaten := false
		for j := range sigs {
			if i != j && m.atLeastAsSpecific(sigs[j], sigs[i], args) && !m.atLeastAsSpecific(sigs[i], sigs[j], args) {
				beaten = true
				break
			}
		}
		if !beaten {
			out = append(out, i)
		}
	}
	if len(out) == 0 {
		for i := range sigs {
			out = append(out, i)
		}
	}
	return out
}

// atLeastAsSpecific reports whether a's parameter types conform to b's at every
// parameter the arguments bind; an unknown signature is the most general.
func (m *Model) atLeastAsSpecific(a, b invocationSignature, args []Argument) bool {
	if !b.known {
		return true
	}
	if !a.known {
		return false
	}
	positional := 0
	for _, arg := range args {
		var pa, pb int
		if arg.Name == "" {
			pa, pb = positional, positional
			positional++
		} else {
			pa, pb = a.index(arg.Name), b.index(arg.Name)
		}
		if pa < 0 || pb < 0 || pa >= len(a.params) || pb >= len(b.params) {
			return false
		}
		if !m.parameterConforms(a.params[pa], b.params[pb]) {
			return false
		}
	}
	return true
}

// parameterConforms reports whether a's type conforms to b's: by declared type where
// both declare one, by scalar lattice otherwise; the untyped parameter is the top type.
func (m *Model) parameterConforms(a, b signatureParameter) bool {
	if b.untyped {
		return true
	}
	if a.untyped {
		return false
	}
	if a.typ != nil && b.typ != nil {
		return m.Conforms(a.typ, b.typ)
	}
	if b.prim == PrimUnknown {
		return true
	}
	return a.prim != PrimUnknown && PrimConforms(a.prim, b.prim)
}

func containsSymbol(syms []*symbols.Symbol, sym *symbols.Symbol) bool {
	for _, s := range syms {
		if s == sym {
			return true
		}
	}
	return false
}
