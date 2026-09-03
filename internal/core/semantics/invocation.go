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

// InvocationSelection is which declaration an invocation calls, out of every
// declaration its written name is visible as.
type InvocationSelection struct {
	Candidates []*symbols.Symbol // what the name denotes, in lookup order, aliases followed
	Applicable []*symbols.Symbol // the candidates the arguments bind to
	Selected   *symbols.Symbol   // the one called; nil when none applies or they tie
	Ambiguous  bool              // several applicable candidates are equally specific
}

// Resolved reports whether the name denotes at least one declaration.
func (s *InvocationSelection) Resolved() bool {
	return s != nil && len(s.Candidates) > 0
}

// invocationKey identifies an invocation expression in the scope it is read in.
type invocationKey struct {
	node  *ast.InvocationExpr
	scope *symbols.Scope
}

// SelectInvocation chooses the declaration e calls: the most specific visible
// candidate the arguments fit, or the first in lookup order when an argument's
// type is unknown. Memoized per invocation and scope.
func (m *Model) SelectInvocation(scope *symbols.Scope, e *ast.InvocationExpr, args []Argument) *InvocationSelection {
	if m == nil || e == nil || e.Type == nil {
		return &InvocationSelection{}
	}
	key := invocationKey{node: e, scope: scope}
	if sel, ok := m.invocations[key]; ok {
		return sel
	}
	sel := m.selectInvocation(scope, e, args)
	m.invocations[key] = sel
	return sel
}

func (m *Model) selectInvocation(scope *symbols.Scope, e *ast.InvocationExpr, args []Argument) *InvocationSelection {
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
	switch len(sel.Candidates) {
	case 0:
		return sel
	case 1:
		sel.Applicable = sel.Candidates
		sel.Selected = sel.Candidates[0]
		return sel
	}

	behaviors := make([]*symbols.Symbol, 0, len(sel.Candidates))
	for _, c := range sel.Candidates {
		if behaviorLike(c) {
			behaviors = append(behaviors, c)
		}
	}
	if len(behaviors) == 0 {
		// Nothing callable: the checker reports the first as it always has.
		sel.Selected = sel.Candidates[0]
		return sel
	}

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
	// Specificity decides only among candidates the arguments are known to
	// fit: those they match exactly, else those they widen to when no candidate
	// takes them through a parameter of undetermined type. Otherwise the first
	// in lookup order is called, as when nothing about the arguments is known.
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
	fitWiden                // a scalar argument widens to a wider scalar parameter
	fitOpen                 // a type the checker cannot compare, so anything fits
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
	optional bool // declares a default value
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
		sig.params = append(sig.params, signatureParameter{
			name:     name,
			typ:      m.featureType(p.Symbol),
			prim:     m.PrimTypeOf(p.Symbol),
			optional: isUsage && u.Value != nil,
		})
	}
	// A parameterless declaration with supertypes may inherit an unseen signature.
	sig.known = len(sig.params) > 0 || (sym.Decl != nil && len(m.DirectSupertypes(sym)) == 0)
	return sig
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

// argumentBinds reports whether arg may be passed to parameter p, and how
// surely. Strictly, two declared types decide by conformance alone; loosely, a
// non-literal's type only bounds its values, so either direction fits.
func (m *Model) argumentBinds(arg Argument, p signatureParameter, mode bindMode) (bindFit, bool) {
	if arg.Type != nil && p.typ != nil {
		if m.Conforms(arg.Type, p.typ) {
			return fitExact, true
		}
		if mode == bindStrict {
			return fitOpen, false
		}
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

// parameterConforms reports whether a's type conforms to b's: by declared type
// where both declare one, by scalar lattice otherwise; an untyped b accepts anything.
func (m *Model) parameterConforms(a, b signatureParameter) bool {
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
