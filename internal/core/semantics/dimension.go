package semantics

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// Library elements the quantity-dimension model is written in terms of, and the
// members it is read from.
const (
	fqnScalarQuantityValue = "Quantities::ScalarQuantityValue"

	memberQuantityDimension    = "quantityDimension"
	memberQuantityPowerFactors = "quantityPowerFactors"
	memberQuantity             = "quantity"
	memberExponent             = "exponent"
	memberMRef                 = "mRef"
)

// Dimension is the quantity dimension of a measurement: a product of powers of
// base quantities (ISQ's L, M, T, ...), held as a UnitTerm over the base-quantity
// features so that UnitTerm.Commensurable — the same commensurability the runtime
// applies to units — decides whether two dimensions are comparable.
type Dimension struct {
	Term UnitTerm
	// Unit is the unit as written or the operand's quantity type, for the
	// diagnostic; empty for a computed dimension, described by Term alone.
	Unit string
}

// String renders the dimension over its base quantities ("M", "L·T^-1"), or "1"
// for the dimension of a count. Base quantities are named as declared (ISQ's L,
// M, T, ...) rather than by qualified name, which no dimension is written with.
func (d Dimension) String() string {
	out := ""
	for _, factor := range d.Term.Factors {
		if out != "" {
			out += "·"
		}
		out += leafName(factor.Unit.Name)
		if factor.Exponent != 1 {
			out += fmt.Sprintf("^%g", factor.Exponent)
		}
	}
	if out == "" {
		return "1"
	}
	return out
}

// leafName is a symbol's declared name: a cached symbol carries it as the leaf of
// its qualified name, so a diagnostic reads the same whether the library was
// parsed or restored from the index cache.
func leafName(name string) string {
	if i := strings.LastIndex(name, "::"); i >= 0 {
		return name[i+2:]
	}
	return name
}

// dimensionResult is a memoized dimension lookup, the negative outcome included.
type dimensionResult struct {
	term UnitTerm
	ok   bool
}

// DimensionOfExpr reports the dimension of an expression's value when the
// declarations it names determine it statically. A dimension that only
// evaluation determines — an untyped feature or parameter, a calculation result,
// an unresolved reference, an unbound redefinition — is reported as unknown
// rather than guessed.
func (m *Model) DimensionOfExpr(scope *symbols.Scope, node ast.Node) (Dimension, bool) {
	if m == nil || node == nil {
		return Dimension{}, false
	}
	switch n := node.(type) {
	case *ast.IndexExpr:
		return m.dimensionOfQuantity(scope, n)
	case *ast.LiteralInteger, *ast.LiteralReal:
		// A bare number is a count: dimensionless, and known to be.
		return Dimension{Term: UnitTerm{Scale: UnitScale(1)}}, true
	case *ast.FeatureReference:
		return m.dimensionOfName(scope, n.Name)
	case *ast.QualifiedName:
		return m.dimensionOfName(scope, n)
	case *ast.FeatureChainExpr:
		if m.resolver == nil {
			return Dimension{}, false
		}
		sym, ok := m.resolver.ResolveTarget(scope, n)
		if !ok {
			return Dimension{}, false
		}
		return m.dimensionOfFeature(sym)
	case *ast.OperatorExpr:
		return m.dimensionOfOperator(scope, n)
	}
	return Dimension{}, false
}

// dimensionOfQuantity reports the dimension of a magnitude with a unit
// (`1000.0[m]`): the dimension of the unit its reduction is over.
func (m *Model) dimensionOfQuantity(scope *symbols.Scope, n *ast.IndexExpr) (Dimension, bool) {
	if !n.Bracket || n.Index == nil {
		return Dimension{}, false
	}
	term, err := m.UnitTermOfExpr(scope, n.Index)
	if err != nil {
		return Dimension{}, false
	}
	dim, ok := m.dimensionOfUnitTerm(term)
	if !ok {
		return Dimension{}, false
	}
	return Dimension{Term: dim, Unit: UnitExprText(n.Index)}, true
}

// dimensionOfName reports the dimension of the feature a name refers to.
func (m *Model) dimensionOfName(scope *symbols.Scope, qn *ast.QualifiedName) (Dimension, bool) {
	if qn == nil || m.resolver == nil {
		return Dimension{}, false
	}
	sym, ok := m.resolver.ResolveQualified(scope, qn)
	if !ok || sym == nil {
		return Dimension{}, false
	}
	if alias, ok := m.resolver.ResolveAliasTarget(sym); ok {
		sym = alias
	}
	return m.dimensionOfFeature(sym)
}

// dimensionOfOperator reports the dimension of an arithmetic expression, following
// the unit arithmetic the runtime performs. An operand of unknown dimension, or a
// sum of incommensurable ones, leaves the result unknown.
func (m *Model) dimensionOfOperator(scope *symbols.Scope, e *ast.OperatorExpr) (Dimension, bool) {
	switch e.Operator {
	case ast.OpNeg, ast.OpPos:
		if len(e.Operands) == 1 {
			return m.DimensionOfExpr(scope, e.Operands[0])
		}
	case ast.OpAdd, ast.OpSub:
		lhs, rhs, ok := m.operandDimensions(scope, e)
		if ok && lhs.Term.Commensurable(rhs.Term) {
			return lhs, true
		}
	case ast.OpMul, ast.OpDiv:
		lhs, rhs, ok := m.operandDimensions(scope, e)
		if !ok {
			return Dimension{}, false
		}
		if e.Operator == ast.OpMul {
			return Dimension{Term: lhs.Term.Times(rhs.Term)}, true
		}
		return Dimension{Term: lhs.Term.DividedBy(rhs.Term)}, true
	case ast.OpPow:
		if len(e.Operands) != 2 {
			return Dimension{}, false
		}
		base, ok := m.DimensionOfExpr(scope, e.Operands[0])
		if !ok {
			return Dimension{}, false
		}
		exp, ok := m.Eval(e.Operands[1])
		if !ok || !exp.IsNumeric() {
			return Dimension{}, false
		}
		return Dimension{Term: base.Term.Pow(exp.asReal())}, true
	}
	return Dimension{}, false
}

// operandDimensions reports the dimensions of a binary expression's two
// operands, and whether both are statically known.
func (m *Model) operandDimensions(scope *symbols.Scope, e *ast.OperatorExpr) (Dimension, Dimension, bool) {
	if len(e.Operands) != 2 {
		return Dimension{}, Dimension{}, false
	}
	lhs, lhsOK := m.DimensionOfExpr(scope, e.Operands[0])
	if !lhsOK {
		return Dimension{}, Dimension{}, false
	}
	rhs, rhsOK := m.DimensionOfExpr(scope, e.Operands[1])
	if !rhsOK {
		return Dimension{}, Dimension{}, false
	}
	return lhs, rhs, true
}

// dimensionOfFeature reports the dimension of a feature's value: that of the unit
// its declared quantity type measures in. A feature that declares no quantity
// type has no statically determined dimension even when the value it is bound to
// carries a unit, since an assignment may replace that value with another.
func (m *Model) dimensionOfFeature(sym *symbols.Symbol) (Dimension, bool) {
	if sym == nil {
		return Dimension{}, false
	}
	if typ := m.quantityValueTypeOf(sym); typ != nil {
		mRef, ok := m.LookupMember(typ, memberMRef)
		if !ok {
			return Dimension{}, false
		}
		term, ok := m.dimensionOf(mRef)
		if !ok {
			return Dimension{}, false
		}
		return Dimension{Term: term, Unit: leafName(typ.Name)}, true
	}
	return Dimension{}, false
}

// quantityValueTypeOf returns the nearest supertype of sym that is a
// ScalarQuantityValue definition, or nil. ScalarQuantityValue itself does not
// count: its measurement reference is any unit, so it fixes no dimension.
func (m *Model) quantityValueTypeOf(sym *symbols.Symbol) *symbols.Symbol {
	quantity := m.libSymbol(fqnScalarQuantityValue)
	if quantity == nil {
		return nil
	}
	for _, super := range m.AllSupertypes(sym) {
		if m.resolver != nil {
			// A type named through an alias (ISQ::TemperatureValue) is reached as the
			// alias, which declares no dimension of its own.
			if alias, ok := m.resolver.ResolveAliasTarget(super); ok {
				super = alias
			}
		}
		if super == quantity || super.Kind != symbols.SymbolAttributeDef {
			continue
		}
		if m.Conforms(super, quantity) {
			return super
		}
	}
	return nil
}

// dimensionOfUnitTerm reports the dimension a reduced unit measures in: its base
// units' dimensions raised to their exponents. Scale is not part of a dimension,
// so `km` and `m` share one.
func (m *Model) dimensionOfUnitTerm(term UnitTerm) (UnitTerm, bool) {
	out := UnitTerm{Scale: UnitScale(1)}
	for _, factor := range term.Factors {
		dim, ok := m.dimensionOf(factor.Unit)
		if !ok {
			return UnitTerm{}, false
		}
		out = out.Times(dim.Pow(factor.Exponent))
	}
	return out, true
}

// dimensionOf reports the dimension a measurement unit or unit definition
// measures in, and whether any declaration determines it. A definition states it
// as the power factors of its `quantityDimension` member; a unit takes it from the
// definition that types it; a unit of dimension one has the empty dimension.
// A unit specializing MeasurementUnit directly has none — reported as unknown
// rather than assumed dimensionless. Memoized per symbol, negatives included.
func (m *Model) dimensionOf(sym *symbols.Symbol) (UnitTerm, bool) {
	if m == nil || sym == nil {
		return UnitTerm{}, false
	}
	if cached, ok := m.dimensions[sym]; ok {
		return cached.term, cached.ok
	}
	if m.dimensioning[sym] {
		return UnitTerm{}, false
	}
	m.dimensioning[sym] = true
	term, ok := m.deriveDimension(sym)
	delete(m.dimensioning, sym)
	m.dimensions[sym] = dimensionResult{term: term, ok: ok}
	return term, ok
}

// deriveDimension computes the dimension dimensionOf memoizes.
func (m *Model) deriveDimension(sym *symbols.Symbol) (UnitTerm, bool) {
	if sym.Dimension != nil {
		return m.persistedDimension(sym)
	}
	if dimOne := m.libSymbol(fqnDimensionOneUnit); dimOne != nil && m.Conforms(sym, dimOne) {
		return UnitTerm{Scale: UnitScale(1)}, true
	}
	if term, ok := m.declaredDimension(sym); ok {
		return term, true
	}
	return m.inheritedDimension(sym)
}

// persistedDimension rebuilds the dimension persisted with a cached library
// symbol, resolving each base quantity by qualified name.
func (m *Model) persistedDimension(sym *symbols.Symbol) (UnitTerm, bool) {
	term := UnitTerm{Scale: UnitScale(1)}
	for _, factor := range sym.Dimension.Factors {
		quantity := m.libSymbol(factor.FQN)
		if quantity == nil {
			return UnitTerm{}, false
		}
		term.Factors = append(term.Factors, UnitFactor{Unit: quantity, Exponent: factor.Exponent})
	}
	return normalizeTerm(term), true
}

// declaredDimension reads the quantity and exponent of each power factor a unit
// definition's `quantityDimension` member lists. A definition that restates no
// factors states no dimension of its own and takes the one it inherits.
func (m *Model) declaredDimension(sym *symbols.Symbol) (UnitTerm, bool) {
	dimension, ok := m.LookupMember(sym, memberQuantityDimension)
	if !ok {
		return UnitTerm{}, false
	}
	factors, ok := m.LookupMember(dimension, memberQuantityPowerFactors)
	if !ok {
		return UnitTerm{}, false
	}
	listed := usageValue(factors)
	if listed == nil {
		return UnitTerm{}, false
	}
	term := UnitTerm{Scale: UnitScale(1)}
	scope := scopeOf(factors)
	for _, ref := range sequenceElements(listed) {
		factor, ok := m.powerFactor(scope, ref)
		if !ok {
			return UnitTerm{}, false
		}
		term.Factors = append(term.Factors, factor)
	}
	if len(term.Factors) == 0 {
		return UnitTerm{}, false
	}
	return normalizeTerm(term), true
}

// powerFactor reads one listed QuantityPowerFactor: the base quantity it names and
// the exponent it raises it to.
func (m *Model) powerFactor(scope *symbols.Scope, ref ast.Node) (UnitFactor, bool) {
	if m.resolver == nil {
		return UnitFactor{}, false
	}
	sym, ok := m.resolver.ResolveTarget(scope, ref)
	if !ok || sym == nil {
		return UnitFactor{}, false
	}
	quantity, ok := m.LookupMember(sym, memberQuantity)
	if !ok {
		return UnitFactor{}, false
	}
	named := usageValue(quantity)
	if named == nil {
		return UnitFactor{}, false
	}
	base, ok := m.resolver.ResolveTarget(scopeOf(quantity), named)
	if !ok || base == nil {
		return UnitFactor{}, false
	}
	exponent, ok := m.LookupMember(sym, memberExponent)
	if !ok {
		return UnitFactor{}, false
	}
	value := usageValue(exponent)
	if value == nil {
		return UnitFactor{}, false
	}
	evaluated, ok := m.Eval(value)
	if !ok || !evaluated.IsNumeric() {
		return UnitFactor{}, false
	}
	return UnitFactor{Unit: base, Exponent: evaluated.asReal()}, true
}

// inheritedDimension reports the dimension a unit takes from what it specializes.
// Supertypes determining none (MeasurementUnit, SimpleUnit) contribute nothing;
// supertypes determining different dimensions leave it undetermined.
func (m *Model) inheritedDimension(sym *symbols.Symbol) (UnitTerm, bool) {
	var found UnitTerm
	known := false
	for _, super := range m.DirectSupertypes(sym) {
		term, ok := m.dimensionOf(super)
		if !ok {
			continue
		}
		if known && !found.Commensurable(term) {
			return UnitTerm{}, false
		}
		found, known = term, true
	}
	return found, known
}

// sequenceElements returns the expressions a value lists: a sequence's elements,
// or the expression itself.
func sequenceElements(node ast.Node) []ast.Node {
	if seq, ok := node.(*ast.SequenceExpr); ok {
		return seq.Elements
	}
	return []ast.Node{node}
}

// DimensionFactsOf returns the dimension to persist with a symbol whose
// declaration a cache record drops, or nil when it is undetermined or its base
// quantities have no qualified name to restore them by.
func (m *Model) DimensionFactsOf(sym *symbols.Symbol, idx *symbols.Index) *symbols.DimensionFacts {
	term, ok := m.dimensionOf(sym)
	if !ok {
		return nil
	}
	facts := &symbols.DimensionFacts{}
	for _, factor := range term.Factors {
		fqn := idx.GetFQN(factor.Unit)
		if fqn == "" {
			return nil
		}
		facts.Factors = append(facts.Factors, symbols.DimensionFactorFacts{FQN: fqn, Exponent: factor.Exponent})
	}
	return facts
}
