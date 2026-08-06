package passes

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// checkValueConformance checks a bound value against the declaring feature's
// type and multiplicity for the cases the scalar lattice does not cover: values
// typed by a user-declared type, enumeration literals, and collections.
//
// Like the scalar rules, every check is one-sided — it reports only when both
// the expected and the actual property are known — so a partially typed model
// never produces a false positive.
func (ec *exprChecker) checkValueConformance(scope *symbols.Scope, u *ast.Usage) {
	want := ec.declaredTypeSymbol(scope, u.Relationships)
	if want == nil || ec.model.PrimTypeOf(want) != semantics.PrimUnknown {
		// An untyped feature has nothing to conform to, and a scalar-typed one
		// is the lattice rules' to report — in lattice terms (Natural vs
		// Integer), which is the more precise message.
		return
	}
	// A collection literal binds elementwise, so each element is checked
	// against the feature's type rather than the sequence as a whole.
	for _, value := range valueElements(u.Value) {
		if got := ec.valueTypeSymbol(scope, value); got != nil {
			if !ec.model.Conforms(got, want) {
				ec.errorf(value.Span(), "cannot bind a value of type %s to a feature typed by %s", got.Name, want.Name)
			}
			continue
		}
		// The feature's type has no scalar ancestor, so no literal value can
		// conform to it. Only literals are judged here: any other expression
		// may produce an instance of the type.
		if prim := literalPrimType(value); prim != semantics.PrimUnknown {
			ec.errorf(value.Span(), "cannot bind %s value to a feature typed by %s", prim, want.Name)
		}
	}
}

// literalPrimType returns the scalar type of a literal value, or PrimUnknown
// for anything that is not one.
func literalPrimType(value ast.Node) semantics.PrimType {
	switch value.(type) {
	case *ast.LiteralBool:
		return semantics.PrimBoolean
	case *ast.LiteralString:
		return semantics.PrimString
	case *ast.LiteralInteger:
		return semantics.PrimNatural
	case *ast.LiteralReal:
		return semantics.PrimRational
	}
	return semantics.PrimUnknown
}

// checkValueCount checks a bound value's element count against the feature's
// declared multiplicity.
func (ec *exprChecker) checkValueCount(u *ast.Usage) {
	count, known := exactCount(u.Value)
	if !known {
		return
	}
	r, ok := ec.model.RangeOf(u.Multiplicity)
	if !ok {
		return
	}
	if r.Upper.Known && !r.Upper.Infinite && count > r.Upper.Value {
		ec.errorf(u.Value.Span(), "%d value(s) bound to a feature with multiplicity upper bound %d", count, r.Upper.Value)
		return
	}
	if r.Lower.Known && !r.Lower.Infinite && count < r.Lower.Value {
		ec.errorf(u.Value.Span(), "%d value(s) bound to a feature with multiplicity lower bound %d", count, r.Lower.Value)
	}
}

// exactCount returns how many values a bound expression produces, and whether
// that is statically known. A collection literal contributes its elements and a
// literal contributes one; anything else (a feature reference, an invocation)
// may itself be multi-valued, so its count is unknown.
func exactCount(value ast.Node) (int64, bool) {
	if value == nil {
		return 0, false
	}
	if seq, ok := value.(*ast.SequenceExpr); ok {
		return int64(len(seq.Elements)), true
	}
	if literalPrimType(value) != semantics.PrimUnknown {
		return 1, true
	}
	return 0, false
}

// valueElements returns the values a bound expression contributes: the elements
// of a collection literal, or the expression itself.
func valueElements(value ast.Node) []ast.Node {
	if value == nil {
		return nil
	}
	if seq, ok := value.(*ast.SequenceExpr); ok {
		return seq.Elements
	}
	return []ast.Node{value}
}

// declaredTypeSymbol returns the symbol a usage is typed by, or nil.
func (ec *exprChecker) declaredTypeSymbol(scope *symbols.Scope, rels []*ast.Relationship) *symbols.Symbol {
	for _, rel := range rels {
		if rel == nil || rel.Kind != ast.RelTyping || rel.Target == nil {
			continue
		}
		if sym := ec.resolveTarget(scope, rel.Target); sym != nil {
			return sym
		}
	}
	return nil
}

// valueTypeSymbol returns the type of a bound value as a symbol, or nil when it
// is not a name the checker can type: a literal (the scalar rules cover those),
// an expression, or a name that does not resolve.
func (ec *exprChecker) valueTypeSymbol(scope *symbols.Scope, value ast.Node) *symbols.Symbol {
	qn, ok := qualifiedNameOf(value)
	if !ok {
		return nil
	}
	sym, resolved := ec.resolver.ResolveQualified(scope, qn)
	if !resolved || sym == nil {
		return nil
	}
	if sym.Kind == symbols.SymbolAlias {
		if target, ok := ec.resolver.ResolveAliasTarget(sym); ok && target != nil {
			sym = target
		}
	}
	u, isUsage := sym.Decl.(*ast.Usage)
	if !isUsage {
		// A definition used as a value is a type name, not an instance of one;
		// the checker has no rule for that.
		return nil
	}
	if typed := ec.declaredTypeSymbol(symbolScope(sym), u.Relationships); typed != nil {
		return typed
	}
	// An enumeration literal declares no type: it is a member of the
	// enumeration that owns it, which is what it conforms to.
	if u.Kind == ast.UsageEnumeration {
		return ec.owningEnumeration(sym)
	}
	return nil
}

// owningEnumeration returns the enumeration definition a literal belongs to, or
// nil when the literal is not owned by one.
func (ec *exprChecker) owningEnumeration(sym *symbols.Symbol) *symbols.Symbol {
	if sym.OwnerScope == nil {
		return nil
	}
	owner := sym.OwnerScope.Owner()
	if owner == nil || owner.Kind != symbols.SymbolEnumerationDef {
		return nil
	}
	return owner
}

// symbolScope returns the scope a symbol's own type names resolve in.
func symbolScope(sym *symbols.Symbol) *symbols.Scope {
	if sym.Scope != nil {
		return sym.Scope
	}
	return sym.OwnerScope
}

// qualifiedNameOf unwraps the name forms a value expression can take.
func qualifiedNameOf(value ast.Node) (*ast.QualifiedName, bool) {
	switch v := value.(type) {
	case *ast.FeatureReference:
		return v.Name, v.Name != nil
	case *ast.QualifiedName:
		return v, true
	}
	return nil, false
}
