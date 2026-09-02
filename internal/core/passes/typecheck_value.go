package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// checkValueConformance checks a bound value against the declaring feature's
// type and multiplicity for the cases the scalar lattice does not cover: values
// typed by a user-declared type, enumeration literals, and collections.
//
// Like the scalar rules, every check is one-sided — it reports only when both
// the expected and the actual property are known — so a partially typed model
// never produces a false positive.
func (ec *exprChecker) checkValueConformance(valueScope, declScope *symbols.Scope, u *ast.Usage, value ast.Node) {
	want := ec.declaredTypeSymbol(declScope, u.Relationships)
	if want == nil || ec.model.PrimTypeOf(want) != semantics.PrimUnknown {
		// An untyped feature has nothing to conform to, and a scalar-typed one
		// is the lattice rules' to report — in lattice terms (Natural vs
		// Integer), which is the more precise message.
		return
	}
	// A collection literal binds elementwise, so each element is checked
	// against the feature's type rather than the sequence as a whole.
	for _, value := range valueElements(value) {
		if got := ec.valueTypeSymbol(valueScope, value); got != nil {
			// A binding equates the two features, so conformance in either
			// direction suffices; only unrelated types are rejected.
			if !ec.model.Conforms(got, want) && !ec.model.Conforms(want, got) {
				ec.errorf(value.Span(), "cannot bind a value of type %s to a feature typed by %s", got.Name, want.Name)
			}
			continue
		}
		if got := ec.invocationResultTypeSymbol(valueScope, value); got != nil {
			if !ec.model.Conforms(got, want) && !ec.model.Conforms(want, got) {
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

// checkValueCount checks a bound value's element count against the multiplicity
// governing the feature.
func (ec *exprChecker) checkValueCount(declScope *symbols.Scope, u *ast.Usage, value ast.Node) {
	count, known := exactCount(value)
	if !known {
		return
	}
	r, ok := ec.effectiveRange(declScope, u, 0)
	if !ok {
		return
	}
	if msg := r.CountViolation(count); msg != "" {
		ec.errorf(value.Span(), "%s", msg)
	}
}

// maxRedefinitionDepth bounds the redefinition chain the effective multiplicity
// is looked up along, so a cyclic chain terminates.
const maxRedefinitionDepth = 32

// effectiveRange returns the multiplicity governing a usage: the one it
// declares, or the one it inherits from the feature it redefines
// (KerML 1.0 §7.3.4.5).
func (ec *exprChecker) effectiveRange(scope *symbols.Scope, u *ast.Usage, depth int) (semantics.Range, bool) {
	if r, ok := ec.model.RangeOf(u.Multiplicity); ok {
		return r, true
	}
	if depth >= maxRedefinitionDepth {
		return semantics.Range{}, false
	}
	for _, rel := range u.Relationships {
		if rel == nil || rel.Kind != ast.RelRedefines || rel.Target == nil {
			continue
		}
		// The redefining declaration may carry the redefined feature's name, so
		// the target is resolved with the declaration's own bindings hidden.
		target, ok := ec.resolver.ResolveReferenceTarget(scope, u, rel.Target)
		if !ok || target == nil {
			continue
		}
		tu, isUsage := target.Decl.(*ast.Usage)
		if !isUsage || tu == u {
			continue
		}
		if r, ok := ec.effectiveRange(target.OwnerScope, tu, depth+1); ok {
			return r, true
		}
	}
	return semantics.Range{}, false
}

// exactCount returns how many values a bound expression produces, and whether
// that is statically known. A literal contributes one value and a collection
// literal the values of its elements; anything else (a feature reference, an
// invocation) may itself be multi-valued, so its count is unknown — as is that
// of a collection holding one.
func exactCount(value ast.Node) (int64, bool) {
	if value == nil {
		return 0, false
	}
	// Binding flattens a collection into the values its elements produce, so a
	// nested literal contributes its own elements rather than one value.
	if seq, ok := value.(*ast.SequenceExpr); ok {
		var total int64
		for _, element := range seq.Elements {
			n, ok := exactCount(element)
			if !ok {
				return 0, false
			}
			total += n
		}
		return total, true
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
	// The type name resolves from the scope the usage was declared in, as
	// generalization targets do elsewhere (semantics.DirectSupertypes, the
	// subsetting conformance checks). Resolving from the scope the usage owns
	// would let its own members shadow the type it names.
	if typed := ec.declaredTypeSymbol(sym.OwnerScope, u.Relationships); typed != nil {
		return typed
	}
	// An enumeration literal declares no type: it is a member of the
	// enumeration that owns it, which is what it conforms to.
	if u.Kind == ast.UsageEnumeration {
		return ec.owningEnumeration(sym)
	}
	return nil
}

// invocationResultTypeSymbol returns the type of the result parameter of the
// behavior an invocation names, which is the type of the value it produces.
func (ec *exprChecker) invocationResultTypeSymbol(scope *symbols.Scope, value ast.Node) *symbols.Symbol {
	inv, ok := value.(*ast.InvocationExpr)
	if !ok || inv.Type == nil {
		return nil
	}
	sym := SelectInvocation(ec.resolver, ec.model, scope, inv).Selected
	if sym == nil || !ec.isInvocationBehavior(sym, map[*symbols.Symbol]bool{}) {
		return nil
	}
	result := ec.model.ResultParameterOf(sym)
	if result == nil {
		return nil
	}
	u, isUsage := result.Decl.(*ast.Usage)
	if !isUsage {
		return nil
	}
	return ec.declaredTypeSymbol(result.OwnerScope, u.Relationships)
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
