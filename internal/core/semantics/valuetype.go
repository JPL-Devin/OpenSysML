package semantics

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Library types an expression's value is classified against, and the scalar
// types a literal is of.
const (
	FQNDurationValue    = "ISQBase::DurationValue"
	FQNTimeInstantValue = "Time::TimeInstantValue"
	FQNBoolean          = "ScalarValues::Boolean"

	fqnString                     = "ScalarValues::String"
	fqnNatural                    = "ScalarValues::Natural"
	fqnRational                   = "ScalarValues::Rational"
	fqnNumericalValue             = "ScalarValues::NumericalValue"
	fqnAnything                   = "Base::Anything"
	fqnTensorMeasurementReference = "MeasurementReferences::TensorMeasurementReference"
)

// Conformance is the outcome of judging an expression's value against a type.
// A value whose type only evaluation determines is Known false, and a rule that
// consumes the judgement must then stay silent rather than guess.
type Conformance struct {
	Known bool
	Holds bool
	// Found describes what the value is, for a diagnostic when Holds is false.
	Found string
	// Untyped marks a value whose static type says nothing — a feature declaring
	// no type, a result typed Anything — so a rule may leave it to evaluation.
	Untyped bool
}

func conformanceUnknown() Conformance { return Conformance{} }

// ExprConformsToLibrary is ExprConformsTo against the library type named by fqn,
// unknown when the library declaring it is not loaded.
func (m *Model) ExprConformsToLibrary(scope *symbols.Scope, node ast.Node, fqn string) Conformance {
	if m == nil {
		return conformanceUnknown()
	}
	return m.ExprConformsTo(scope, node, m.libSymbol(fqn))
}

// ExprConformsTo reports whether an expression's value conforms to want, as far
// as the declarations the expression names determine it statically: a literal by
// its scalar type, a feature by its declared type, a quantity literal by the unit
// it is written in, an invocation by its result, an arithmetic expression by the
// dimension of its value. A feature declared by nothing but a value is typed by
// that value (KerML checkFeatureValuationSpecialization); a declared type is
// never narrowed by the value.
func (m *Model) ExprConformsTo(scope *symbols.Scope, node ast.Node, want *symbols.Symbol) Conformance {
	return m.exprConformance(scope, node, want, true)
}

// exprConformance is ExprConformsTo, judging a quantity literal and quantity
// arithmetic by the unit written when byUnit is set and otherwise by the
// ScalarQuantityValue result the quantity functions declare.
func (m *Model) exprConformance(scope *symbols.Scope, node ast.Node, want *symbols.Symbol, byUnit bool) Conformance {
	if m == nil || node == nil || want == nil || m.resolver == nil {
		return conformanceUnknown()
	}
	if alias, ok := m.resolver.ResolveAliasTarget(want); ok {
		want = alias
	}
	switch n := node.(type) {
	case *ast.LiteralBool:
		return m.typeConformance(m.libSymbol(FQNBoolean), want)
	case *ast.LiteralString:
		return m.typeConformance(m.libSymbol(fqnString), want)
	case *ast.LiteralInteger:
		return m.typeConformance(m.libSymbol(fqnNatural), want)
	case *ast.LiteralReal:
		return m.typeConformance(m.libSymbol(fqnRational), want)
	case *ast.FeatureReference, *ast.QualifiedName, *ast.FeatureChainExpr:
		sym, ok := m.resolver.ResolveTarget(scope, n)
		if !ok || sym == nil {
			return conformanceUnknown()
		}
		return m.featureConformance(sym, want)
	case *ast.IndexExpr:
		if !n.Bracket {
			return conformanceUnknown()
		}
		if !byUnit {
			return m.typeConformance(m.libSymbol(fqnScalarQuantityValue), want)
		}
		return m.quantityConformance(scope, n, want)
	case *ast.OperatorExpr:
		return m.operatorConformance(scope, n, want, byUnit)
	case *ast.InvocationExpr:
		return m.invocationConformance(scope, n, want)
	case *ast.ConstructorExpr:
		if n.Type == nil {
			return conformanceUnknown()
		}
		def, ok := m.resolver.ResolveQualified(scope, n.Type)
		if !ok || def == nil {
			return conformanceUnknown()
		}
		if alias, ok := m.resolver.ResolveAliasTarget(def); ok {
			def = alias
		}
		return m.typeConformance(def, want)
	}
	return conformanceUnknown()
}

// typeConformance judges a value known to be of typ.
func (m *Model) typeConformance(typ, want *symbols.Symbol) Conformance {
	if typ == nil {
		return conformanceUnknown()
	}
	return Conformance{Known: true, Holds: m.Conforms(typ, want), Found: leafName(typ.Name)}
}

// featureConformance judges a value that is what a feature holds: the feature's
// declared type bounds it. A feature with a generalization that does not resolve
// has an undetermined type, reported elsewhere.
func (m *Model) featureConformance(sym *symbols.Symbol, want *symbols.Symbol) Conformance {
	if alias, ok := m.resolver.ResolveAliasTarget(sym); ok {
		sym = alias
	}
	if !isFeature(sym) {
		return conformanceUnknown()
	}
	if m.Conforms(sym, want) {
		return Conformance{Known: true, Holds: true}
	}
	// A step named as a value stands for its result: a constraint is its Boolean.
	if result := m.ResultParameterOf(sym); result != nil && result != sym {
		return m.featureConformance(result, want)
	}
	if value := m.typingValue(sym); value != nil {
		return m.valueConformance(sym, value, want)
	}
	if !m.generalizationsResolve(sym) {
		return conformanceUnknown()
	}
	if typ := m.nearestDeclaredType(sym); typ != nil {
		return Conformance{Known: true, Found: leafName(typ.Name)}
	}
	return Conformance{Known: true, Found: "an untyped feature", Untyped: true}
}

// typingValue returns the value a feature is implicitly typed by: a non-default
// value of a usage declaring no generalization and no direction (KerML §8.3.3.3
// checkFeatureValuationSpecialization), else nil.
func (m *Model) typingValue(sym *symbols.Symbol) ast.Node {
	u, ok := sym.Decl.(*ast.Usage)
	if !ok || u.Value == nil || u.IsDefault || u.Direction != ast.DirNone {
		return nil
	}
	for _, rel := range u.Relationships {
		if rel != nil && GeneralizationKind(rel.Kind) {
			return nil
		}
	}
	return u.Value
}

// valueConformance judges a feature by the result of the value it is typed by,
// resolved where the declaration was written; a quantity result is the
// ScalarQuantityValue its function declares, so a duration literal types a
// feature as a quantity, not a DurationValue. A value that leads back to its
// own feature has no static type.
func (m *Model) valueConformance(sym *symbols.Symbol, value ast.Node, want *symbols.Symbol) Conformance {
	if m.valuing[sym] {
		return conformanceUnknown()
	}
	m.valuing[sym] = true
	defer delete(m.valuing, sym)
	return m.exprConformance(sym.OwnerScope, value, want, false)
}

// generalizationsResolve reports whether every generalization a declaration
// states names an element.
func (m *Model) generalizationsResolve(sym *symbols.Symbol) bool {
	for _, rel := range RelationshipsOf(sym) {
		if !GeneralizationKind(rel.Kind) || rel.Target == nil {
			continue
		}
		if m.relationshipTarget(sym, rel) == nil {
			return false
		}
	}
	return true
}

// nearestDeclaredType returns the type a feature is declared with, or the one a
// feature it redefines or subsets is; nil for a feature typed by nothing but
// its kind's base.
func (m *Model) nearestDeclaredType(sym *symbols.Symbol) *symbols.Symbol {
	if defs := m.declaredTypes(sym, map[*symbols.Symbol]bool{}); len(defs) > 0 {
		return defs[0]
	}
	return nil
}

// declaredTypes returns the definitions a feature is directly typed by, or those
// of the features it redefines or subsets when it restates none itself. The
// base its kind implies (`Base::dataValues`) types every feature of the kind
// and so says nothing about this one.
func (m *Model) declaredTypes(sym *symbols.Symbol, seen map[*symbols.Symbol]bool) []*symbols.Symbol {
	if seen[sym] {
		return nil
	}
	seen[sym] = true
	var defs, features []*symbols.Symbol
	base := m.implicitBase(sym)
	for _, super := range m.DirectSupertypes(sym) {
		if alias, ok := m.resolver.ResolveAliasTarget(super); ok {
			super = alias
		}
		switch {
		case super == base:
		case isTypeDecl(super):
			defs = append(defs, super)
		case isFeature(super):
			features = append(features, super)
		}
	}
	if len(defs) > 0 {
		return defs
	}
	for _, f := range features {
		defs = append(defs, m.declaredTypes(f, seen)...)
	}
	return defs
}

// quantityConformance judges a magnitude paired with a measurement reference
// (`5 [s]`), a ScalarQuantityValue. Against a quantity value type it is one of
// that type when its reference is of the kind the type's `mRef` admits — a
// DurationValue is written in a DurationUnit — or, when the reference is not a
// single named one, when its dimension is the type's.
func (m *Model) quantityConformance(scope *symbols.Scope, n *ast.IndexExpr, want *symbols.Symbol) Conformance {
	quantity := m.libSymbol(fqnScalarQuantityValue)
	if quantity == nil {
		return conformanceUnknown()
	}
	ref := m.measurementReference(scope, n.Index)
	if !m.Conforms(want, quantity) {
		c := m.typeConformance(quantity, want)
		if ref != nil {
			c.Found = m.describeQuantity(ref)
		}
		return c
	}
	if ref == nil {
		return m.dimensionConformance(scope, n, want)
	}
	mRef, ok := m.LookupMember(want, memberMRef)
	if !ok {
		return conformanceUnknown()
	}
	admitted := m.declaredTypes(mRef, map[*symbols.Symbol]bool{})
	if len(admitted) == 0 {
		return conformanceUnknown()
	}
	holds := true
	for _, typ := range admitted {
		holds = holds && m.Conforms(ref, typ)
	}
	return Conformance{Known: true, Holds: holds, Found: m.describeQuantity(ref)}
}

// describeQuantity names a quantity by the measurement reference it is written in.
func (m *Model) describeQuantity(ref *symbols.Symbol) string {
	found := fmt.Sprintf("a quantity in %s", leafName(ref.Name))
	if typ := m.nearestDeclaredType(ref); typ != nil {
		found += fmt.Sprintf(" (a %s)", leafName(typ.Name))
	}
	return found
}

// measurementReference returns the measurement reference a quantity literal
// names when it is a single resolved one, or nil for a derived unit expression.
func (m *Model) measurementReference(scope *symbols.Scope, unit ast.Node) *symbols.Symbol {
	sym, ok := m.resolver.ResolveTarget(scope, unit)
	if !ok || sym == nil {
		return nil
	}
	if alias, ok := m.resolver.ResolveAliasTarget(sym); ok {
		sym = alias
	}
	reference := m.libSymbol(fqnTensorMeasurementReference)
	if reference == nil || !m.Conforms(sym, reference) || !m.generalizationsResolve(sym) {
		return nil
	}
	return sym
}

// operatorConformance judges an operator's value by the result of the library
// function its operands select: comparisons are Boolean, a conditional the
// Anything its function returns, `%` a number; arithmetic is a quantity when
// every operand is one (QuantityCalculations), else a number, and `+` over
// strings a String.
func (m *Model) operatorConformance(scope *symbols.Scope, e *ast.OperatorExpr, want *symbols.Symbol, byUnit bool) Conformance {
	switch e.Operator {
	case ast.OpConditional, ast.OpNullCoalesce:
		c := m.typeConformance(m.libSymbol(fqnAnything), want)
		if c.Known && !c.Holds {
			c.Found = fmt.Sprintf("the result of `%s`, typed Anything", e.Operator)
			c.Untyped = true
		}
		return c
	case ast.OpMod:
		if m.operandsConformTo(scope, e, m.libSymbol(fqnNumericalValue)) {
			return m.typeConformance(m.libSymbol(fqnNumericalValue), want)
		}
	case ast.OpNot, ast.OpAnd, ast.OpOr, ast.OpXor, ast.OpConditionalAnd, ast.OpConditionalOr,
		ast.OpImplies, ast.OpEq, ast.OpNeq, ast.OpEqEqEq, ast.OpNeqEqEq,
		ast.OpLt, ast.OpGt, ast.OpLe, ast.OpGe, ast.OpIsType, ast.OpHasType, ast.OpAt, ast.OpMetaAt:
		return m.typeConformance(m.libSymbol(FQNBoolean), want)
	case ast.OpNeg, ast.OpPos, ast.OpAdd, ast.OpSub, ast.OpMul, ast.OpDiv, ast.OpPow:
		if m.operandsConformTo(scope, e, m.libSymbol(fqnScalarQuantityValue)) {
			if byUnit {
				return m.dimensionConformance(scope, e, want)
			}
			return m.typeConformance(m.libSymbol(fqnScalarQuantityValue), want)
		}
		if m.operandsConformTo(scope, e, m.libSymbol(fqnNumericalValue)) {
			return m.typeConformance(m.libSymbol(fqnNumericalValue), want)
		}
		if e.Operator == ast.OpAdd && m.operandsConformTo(scope, e, m.libSymbol(fqnString)) {
			return m.typeConformance(m.libSymbol(fqnString), want)
		}
	}
	return conformanceUnknown()
}

// operandsConformTo reports whether every operand is known to be a value of
// typ, so the expression is the result the function over typ declares.
func (m *Model) operandsConformTo(scope *symbols.Scope, e *ast.OperatorExpr, typ *symbols.Symbol) bool {
	if typ == nil || len(e.Operands) == 0 {
		return false
	}
	for _, operand := range e.Operands {
		if c := m.exprConformance(scope, operand, typ, false); !c.Known || !c.Holds {
			return false
		}
	}
	return true
}

// dimensionConformance judges an expression whose value is a quantity of a
// determined dimension: against a quantity value type by that dimension, against
// any other type as the ScalarQuantityValue it is. A dimensionless value may be
// a plain number, so only a quantity value type judges it.
func (m *Model) dimensionConformance(scope *symbols.Scope, node ast.Node, want *symbols.Symbol) Conformance {
	got, ok := m.DimensionOfExpr(scope, node)
	if !ok {
		return conformanceUnknown()
	}
	found := "a value of dimension " + got.String()
	if got.Term.Dimensionless() {
		found = "a dimensionless value"
	}
	quantity := m.libSymbol(fqnScalarQuantityValue)
	if quantity != nil && !m.Conforms(want, quantity) {
		if got.Term.Dimensionless() {
			return conformanceUnknown()
		}
		c := m.typeConformance(quantity, want)
		c.Found = found
		return c
	}
	wantDim, ok := m.DimensionOfType(want)
	if !ok {
		return conformanceUnknown()
	}
	return Conformance{Known: true, Holds: got.Term.Commensurable(wantDim.Term), Found: found}
}

// invocationConformance judges an invocation's value by the declared type of the
// result parameter of what it invokes.
func (m *Model) invocationConformance(scope *symbols.Scope, e *ast.InvocationExpr, want *symbols.Symbol) Conformance {
	if e.Type == nil {
		return conformanceUnknown()
	}
	sym, ok := m.resolver.ResolveQualified(scope, e.Type)
	if !ok || sym == nil {
		return conformanceUnknown()
	}
	if alias, ok := m.resolver.ResolveAliasTarget(sym); ok {
		sym = alias
	}
	result := m.ResultParameterOf(sym)
	if result == nil {
		return conformanceUnknown()
	}
	return m.featureConformance(result, want)
}
