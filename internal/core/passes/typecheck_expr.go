package passes

import (
	"fmt"
	"strconv"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/resolve"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// exprChecker types expressions against the stdlib scalar lattice and reports
// operand, binding, and invocation mismatches. Every rule is one-sided: a
// diagnostic is only produced when both the expected and the actual type are
// known, so partial type information never yields a false positive.
type exprChecker struct {
	resolver *resolve.Resolver
	model    *semantics.Model
	diags    []Diagnostic
	// chaining guards the type of a feature read through a chain against a
	// feature whose value names itself, directly or through another feature.
	chaining map[*symbols.Symbol]bool
}

func (ec *exprChecker) errorf(span source.Span, format string, args ...any) {
	ec.diags = append(ec.diags, Diagnostic{
		Severity: SeverityError,
		Span:     span,
		Message:  fmt.Sprintf(format, args...),
		Code:     "type.expr",
		Source:   "type",
	})
}

func (ec *exprChecker) warnf(span source.Span, format string, args ...any) {
	ec.diags = append(ec.diags, Diagnostic{
		Severity: SeverityWarning,
		Span:     span,
		Message:  fmt.Sprintf(format, args...),
		Code:     "type.expr",
		Source:   "type",
	})
}

// checkUsageValue checks a feature's bound value (`attribute x : T = expr`)
// against the type and multiplicity the feature declares.
func (ec *exprChecker) checkUsageValue(scope *symbols.Scope, u *ast.Usage) {
	if u.Value == nil {
		return
	}
	want := ec.declaredPrimType(scope, u.Relationships)
	// A collection literal binds elementwise, so each element is checked
	// against the feature's type rather than the sequence as a whole.
	for _, value := range valueElements(u.Value) {
		got := ec.infer(scope, value)
		if want == semantics.PrimUnknown || got == semantics.PrimUnknown {
			continue
		}
		if !semantics.PrimConforms(got, want) {
			ec.errorf(value.Span(), "cannot bind %s value to a feature typed by %s", got, want)
		}
	}
	ec.checkValueConformance(scope, u)
	ec.checkValueCount(u)
}

// declaredPrimType returns the scalar type a usage is typed by, or PrimUnknown.
func (ec *exprChecker) declaredPrimType(scope *symbols.Scope, rels []*ast.Relationship) semantics.PrimType {
	for _, rel := range rels {
		if rel == nil || rel.Kind != ast.RelTyping || rel.Target == nil {
			continue
		}
		if sym := ec.resolveTarget(scope, rel.Target); sym != nil {
			if prim := ec.model.PrimTypeOf(sym); prim != semantics.PrimUnknown {
				return prim
			}
		}
	}
	return semantics.PrimUnknown
}

// resolveTarget resolves a relationship target node to its symbol, following
// an alias to the type it names.
func (ec *exprChecker) resolveTarget(scope *symbols.Scope, target ast.Node) *symbols.Symbol {
	if fr, ok := target.(*ast.FeatureReference); ok {
		target = fr.Name
	}
	qn, ok := target.(*ast.QualifiedName)
	if !ok {
		return nil
	}
	sym, ok := ec.resolver.ResolveQualified(scope, qn)
	if !ok || sym == nil {
		return nil
	}
	if sym.Kind == symbols.SymbolAlias {
		if resolved, ok := ec.resolver.ResolveAliasTarget(sym); ok && resolved != nil {
			return resolved
		}
	}
	return sym
}

// checkBoolean checks an expression used where a condition is required.
func (ec *exprChecker) checkBoolean(scope *symbols.Scope, n ast.Node, context string) {
	if n == nil {
		return
	}
	got := ec.infer(scope, n)
	if got == semantics.PrimUnknown || got == semantics.PrimBoolean {
		return
	}
	ec.errorf(n.Span(), "%s must be Boolean, found %s", context, got)
}

// infer returns the scalar type of an expression, checking its operands on the
// way down. PrimUnknown means "not a scalar the checker models".
func (ec *exprChecker) infer(scope *symbols.Scope, n ast.Node) semantics.PrimType {
	switch e := n.(type) {
	case *ast.LiteralBool:
		return semantics.PrimBoolean
	case *ast.LiteralString:
		return semantics.PrimString
	case *ast.LiteralInteger:
		// The grammar has no negative literals (negation is a unary operator),
		// so an integer literal is always a Natural and conforms upward.
		return semantics.PrimNatural
	case *ast.LiteralReal:
		// A decimal literal denotes an exact ratio, so it conforms to Rational
		// as well as Real.
		return semantics.PrimRational
	case *ast.FeatureReference:
		return ec.inferQualified(scope, e.Name)
	case *ast.QualifiedName:
		return ec.inferQualified(scope, e)
	case *ast.FeatureChainExpr:
		return ec.inferFeatureChain(scope, e)
	case *ast.OperatorExpr:
		return ec.inferOperator(scope, e)
	case *ast.InvocationExpr:
		return ec.inferInvocation(scope, e)
	case *ast.SequenceExpr:
		// A sequence has no scalar type of its own; walking the elements keeps
		// errors inside them reported.
		for _, el := range e.Elements {
			ec.infer(scope, el)
		}
		return semantics.PrimUnknown
	case *ast.IndexExpr:
		return ec.inferIndex(scope, e)
	case *ast.CollectExpr:
		return ec.inferCollect(scope, e)
	case *ast.SelectExpr:
		return ec.inferSelect(scope, e)
	case *ast.BodyExpr:
		return ec.inferBody(scope, e)
	}
	return semantics.PrimUnknown
}

// inferIndex types `seq#(i)`, the sequence index, and `n [unit]`, the quantity
// expression the notation shares its node with — the bracket the expression was
// written with says which of the two it is.
func (ec *exprChecker) inferIndex(scope *symbols.Scope, e *ast.IndexExpr) semantics.PrimType {
	if e.Bracket {
		// A quantity is a magnitude in a unit, not a scalar of the lattice: the
		// unit is a name of the measurement reference library, checked as such by
		// the units pass rather than typed here. The magnitude is an expression in
		// its own right and is checked.
		ec.infer(scope, e.Operand)
		return semantics.PrimUnknown
	}

	// SequenceFunctions::'#' declares `in index: Positive[1]`: one whole number,
	// counting from 1. A Real, a Boolean or a String names no position and is
	// reported here; whether a whole number is a position the operand has is
	// known only from the value, so an Integer index — which is what a model
	// counting in a loop holds — is checked at evaluation, not rejected here for
	// not being declared `Natural`.
	index := ec.infer(scope, e.Index)
	if !semantics.PrimConforms(index, semantics.PrimInteger) && e.Index != nil {
		ec.errorf(e.Index.Span(), "sequence index must be an Integer, found %s", index)
	}
	elem := ec.infer(scope, e.Operand)

	// A literal index names a position at check time, so an index the operand
	// cannot have is an error before the model is ever run: index 0 for a
	// notation counting from 1, and any index beyond a sequence written out.
	if lit, ok := e.Index.(*ast.LiteralInteger); ok {
		if written, err := strconv.ParseInt(lit.Value, 10, 64); err == nil {
			seq, isSeq := e.Operand.(*ast.SequenceExpr)
			switch {
			case written == 0:
				ec.errorf(lit.Span(), "sequence index counts from 1, found 0")
			case isSeq && written > int64(len(seq.Elements)):
				ec.errorf(lit.Span(), "sequence index %d is outside 1..%d", written, len(seq.Elements))
			}
		}
	}

	// The index of a sequence of one scalar type is a value of that type, which
	// is what makes `(1, 2, 3)#(2) + 1` checkable. A sequence of mixed types has
	// no one scalar type and stays unknown.
	if seq, ok := e.Operand.(*ast.SequenceExpr); ok {
		return ec.commonElementType(scope, seq)
	}
	return elem
}

// commonElementType returns the scalar type every element of a sequence
// expression conforms to, or PrimUnknown where the elements have none in
// common.
func (ec *exprChecker) commonElementType(scope *symbols.Scope, seq *ast.SequenceExpr) semantics.PrimType {
	common := semantics.PrimUnknown
	for i, el := range seq.Elements {
		elem := ec.infer(scope, el)
		if i == 0 {
			common = elem
			continue
		}
		switch {
		case elem == common:
		case semantics.PrimConforms(elem, common):
		case semantics.PrimConforms(common, elem):
			common = elem
		default:
			return semantics.PrimUnknown
		}
	}
	return common
}

// inferCollect types `operand.{in x; ...}`, the collect notation: a sequence of
// the body's results, which is no scalar of its own. Its purpose here is to
// check the body, in the scope its parameters are declared in.
func (ec *exprChecker) inferCollect(scope *symbols.Scope, e *ast.CollectExpr) semantics.PrimType {
	ec.infer(scope, e.Operand)
	ec.infer(scope, e.Body)
	return semantics.PrimUnknown
}

// inferSelect types `operand.?{in x; ...}`, the select notation. The library
// declares the selector's result `Boolean[1]`, so a body that answers something
// else is reported here rather than only where the model is run.
func (ec *exprChecker) inferSelect(scope *symbols.Scope, e *ast.SelectExpr) semantics.PrimType {
	ec.infer(scope, e.Operand)
	if body, ok := e.Body.(*ast.BodyExpr); ok && body.Result != nil {
		ec.checkBoolean(ec.bodyScope(scope, body), body.Result, "select predicate")
		return semantics.PrimUnknown
	}
	ec.infer(scope, e.Body)
	return semantics.PrimUnknown
}

// inferBody types a body expression as the type of the result it states,
// checking that result in the scope the body's parameters are declared in.
func (ec *exprChecker) inferBody(scope *symbols.Scope, e *ast.BodyExpr) semantics.PrimType {
	inner := ec.bodyScope(scope, e)
	for i := range e.Params {
		ec.infer(scope, e.Params[i].Value)
	}
	if e.Result == nil {
		return semantics.PrimUnknown
	}
	return ec.infer(inner, e.Result)
}

// bodyScope returns the scope a body expression's result is written in: its
// parameters are declarations of that scope, the same one the resolver and the
// runtime read the body's names in.
func (ec *exprChecker) bodyScope(scope *symbols.Scope, body *ast.BodyExpr) *symbols.Scope {
	if scope == nil {
		return nil
	}
	return symbols.BodyExprScope(scope, body)
}

func (ec *exprChecker) inferQualified(scope *symbols.Scope, qn *ast.QualifiedName) semantics.PrimType {
	if qn == nil {
		return semantics.PrimUnknown
	}
	sym, ok := ec.resolver.ResolveQualified(scope, qn)
	if !ok || sym == nil {
		return semantics.PrimUnknown
	}
	return ec.model.PrimTypeOf(sym)
}

// inferFeatureChain types a feature chain (`c.a`) as the feature its last
// segment names. The segments of a chain are members of the preceding segment
// rather than of the enclosing scope (SysML 7.6.6), which is how a calc usage's
// `out` feature — inherited from the calc it is typed by — is reached.
func (ec *exprChecker) inferFeatureChain(scope *symbols.Scope, e *ast.FeatureChainExpr) semantics.PrimType {
	sym, ok := ec.resolver.ResolveTarget(scope, e)
	if !ok || sym == nil {
		// An unresolved chain is reported by the name-resolution tier; typing it
		// again here would double-report it.
		return semantics.PrimUnknown
	}
	return ec.featurePrimType(sym)
}

// featurePrimType returns the scalar type of the feature sym declares, falling
// back to the type of the value it is bound to when it declares none: an `out a
// = n + 1` of a calc carries its type in its default.
func (ec *exprChecker) featurePrimType(sym *symbols.Symbol) semantics.PrimType {
	if prim := ec.model.PrimTypeOf(sym); prim != semantics.PrimUnknown {
		return prim
	}
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok || usage.Value == nil || sym.OwnerScope == nil {
		return semantics.PrimUnknown
	}
	if ec.chaining[sym] {
		return semantics.PrimUnknown
	}
	if ec.chaining == nil {
		ec.chaining = make(map[*symbols.Symbol]bool)
	}
	ec.chaining[sym] = true
	defer delete(ec.chaining, sym)
	// The value belongs to the declaring scope, and is checked there in its own
	// right, so this only reads its type: diagnostics raised here would be
	// reported once per reader.
	silent := exprChecker{resolver: ec.resolver, model: ec.model, chaining: ec.chaining}
	return silent.infer(sym.OwnerScope, usage.Value)
}

func (ec *exprChecker) inferOperator(scope *symbols.Scope, e *ast.OperatorExpr) semantics.PrimType {
	switch e.Operator {
	case ast.OpNot:
		return ec.checkUnaryBoolean(scope, e)
	case ast.OpNeg, ast.OpPos:
		return ec.checkUnaryNumeric(scope, e)
	case ast.OpAnd, ast.OpConditionalAnd, ast.OpOr, ast.OpConditionalOr, ast.OpXor, ast.OpImplies:
		return ec.checkBinaryBoolean(scope, e)
	case ast.OpAdd:
		return ec.checkAddition(scope, e)
	case ast.OpSub, ast.OpMul, ast.OpMod, ast.OpPow:
		return ec.checkArithmetic(scope, e, semantics.PrimWiden)
	case ast.OpDiv:
		return ec.checkArithmetic(scope, e, divisionResult)
	case ast.OpLt, ast.OpGt, ast.OpLe, ast.OpGe:
		return ec.checkComparison(scope, e)
	case ast.OpEq, ast.OpNeq:
		return ec.checkEquality(scope, e)
	case ast.OpConditional:
		return ec.checkConditional(scope, e)
	}
	// Operators outside the scalar lattice (casts, classification, ranges,
	// indexing): still walk operands so nested errors surface.
	for _, operand := range e.Operands {
		ec.infer(scope, operand)
	}
	return semantics.PrimUnknown
}

func (ec *exprChecker) checkUnaryBoolean(scope *symbols.Scope, e *ast.OperatorExpr) semantics.PrimType {
	if len(e.Operands) != 1 {
		return semantics.PrimUnknown
	}
	got := ec.infer(scope, e.Operands[0])
	if got != semantics.PrimUnknown && got != semantics.PrimBoolean {
		ec.errorf(e.Span(), "operator 'not' requires a Boolean operand, found %s", got)
	}
	return semantics.PrimBoolean
}

func (ec *exprChecker) checkUnaryNumeric(scope *symbols.Scope, e *ast.OperatorExpr) semantics.PrimType {
	if len(e.Operands) != 1 {
		return semantics.PrimUnknown
	}
	got := ec.infer(scope, e.Operands[0])
	if got == semantics.PrimUnknown {
		return semantics.PrimUnknown
	}
	if !got.IsNumeric() {
		ec.errorf(e.Span(), "operator '%s' requires a numeric operand, found %s", e.Operator, got)
		return semantics.PrimUnknown
	}
	if e.Operator == ast.OpNeg {
		// Negation leaves the naturals.
		return semantics.PrimWiden(got, semantics.PrimInteger)
	}
	return got
}

func (ec *exprChecker) checkBinaryBoolean(scope *symbols.Scope, e *ast.OperatorExpr) semantics.PrimType {
	lhs, rhs, ok := ec.operands(scope, e)
	if !ok {
		return semantics.PrimBoolean
	}
	for _, got := range []semantics.PrimType{lhs, rhs} {
		if got != semantics.PrimUnknown && got != semantics.PrimBoolean {
			ec.errorf(e.Span(), "operator '%s' requires Boolean operands, found %s and %s", e.Operator, lhs, rhs)
			break
		}
	}
	return semantics.PrimBoolean
}

// checkAddition allows the numeric tower plus String concatenation, which the
// stdlib defines as StringFunctions::'+'.
func (ec *exprChecker) checkAddition(scope *symbols.Scope, e *ast.OperatorExpr) semantics.PrimType {
	lhs, rhs, ok := ec.operands(scope, e)
	if !ok {
		return semantics.PrimUnknown
	}
	if lhs == semantics.PrimUnknown || rhs == semantics.PrimUnknown {
		return semantics.PrimUnknown
	}
	if lhs == semantics.PrimString && rhs == semantics.PrimString {
		return semantics.PrimString
	}
	if lhs.IsNumeric() && rhs.IsNumeric() {
		return semantics.PrimWiden(lhs, rhs)
	}
	ec.errorf(e.Span(), "operator '+' is not defined for %s and %s", lhs, rhs)
	return semantics.PrimUnknown
}

// divisionResult follows the kernel function library: Natural/Natural stays
// Natural, Integer/Integer is Rational, wider types divide within themselves.
func divisionResult(lhs, rhs semantics.PrimType) semantics.PrimType {
	switch widened := semantics.PrimWiden(lhs, rhs); widened {
	case semantics.PrimNatural:
		return semantics.PrimNatural
	case semantics.PrimInteger:
		return semantics.PrimRational
	default:
		return widened
	}
}

// checkArithmetic requires numeric operands and types the result with the
// operator's own result rule.
func (ec *exprChecker) checkArithmetic(
	scope *symbols.Scope,
	e *ast.OperatorExpr,
	result func(lhs, rhs semantics.PrimType) semantics.PrimType,
) semantics.PrimType {
	lhs, rhs, ok := ec.operands(scope, e)
	if !ok {
		return semantics.PrimUnknown
	}
	if lhs == semantics.PrimUnknown || rhs == semantics.PrimUnknown {
		return semantics.PrimUnknown
	}
	if !lhs.IsNumeric() || !rhs.IsNumeric() {
		ec.errorf(e.Span(), "operator '%s' requires numeric operands, found %s and %s", e.Operator, lhs, rhs)
		return semantics.PrimUnknown
	}
	return result(lhs, rhs)
}

func (ec *exprChecker) checkComparison(scope *symbols.Scope, e *ast.OperatorExpr) semantics.PrimType {
	lhs, rhs, ok := ec.operands(scope, e)
	if !ok {
		return semantics.PrimBoolean
	}
	if lhs == semantics.PrimUnknown || rhs == semantics.PrimUnknown {
		return semantics.PrimBoolean
	}
	bothNumeric := lhs.IsNumeric() && rhs.IsNumeric()
	bothString := lhs == semantics.PrimString && rhs == semantics.PrimString
	if !bothNumeric && !bothString {
		ec.errorf(e.Span(), "operator '%s' is not defined for %s and %s", e.Operator, lhs, rhs)
	}
	return semantics.PrimBoolean
}

// checkEquality warns rather than errors: the stdlib declares '==' over
// Anything, so comparing disjoint scalars is legal but almost always a mistake.
func (ec *exprChecker) checkEquality(scope *symbols.Scope, e *ast.OperatorExpr) semantics.PrimType {
	lhs, rhs, ok := ec.operands(scope, e)
	if !ok {
		return semantics.PrimBoolean
	}
	if lhs == semantics.PrimUnknown || rhs == semantics.PrimUnknown {
		return semantics.PrimBoolean
	}
	if !semantics.PrimConforms(lhs, rhs) && !semantics.PrimConforms(rhs, lhs) {
		result := "false"
		if e.Operator == ast.OpNeq {
			result = "true"
		}
		ec.warnf(e.Span(), "comparing %s with %s is always %s", lhs, rhs, result)
	}
	return semantics.PrimBoolean
}

func (ec *exprChecker) checkConditional(scope *symbols.Scope, e *ast.OperatorExpr) semantics.PrimType {
	if len(e.Operands) != 3 {
		return semantics.PrimUnknown
	}
	ec.checkBoolean(scope, e.Operands[0], "condition of 'if'")
	thenType := ec.infer(scope, e.Operands[1])
	elseType := ec.infer(scope, e.Operands[2])
	if thenType == elseType {
		return thenType
	}
	return semantics.PrimWiden(thenType, elseType)
}

func (ec *exprChecker) operands(scope *symbols.Scope, e *ast.OperatorExpr) (semantics.PrimType, semantics.PrimType, bool) {
	if len(e.Operands) != 2 {
		return semantics.PrimUnknown, semantics.PrimUnknown, false
	}
	return ec.infer(scope, e.Operands[0]), ec.infer(scope, e.Operands[1]), true
}

// inferInvocation checks the argument list of a calc/action invocation against
// the invoked behavior's `in` parameters. In the arrow form `x->f(a)` the
// receiver binds to the first parameter, so it is prepended to the arguments.
func (ec *exprChecker) inferInvocation(scope *symbols.Scope, e *ast.InvocationExpr) semantics.PrimType {
	args := e.Args
	if e.Operand != nil {
		args = append([]ast.Node{e.Operand}, args...)
	}
	// Typed once and reused by checkArguments, so nested errors report once.
	argTypes := make([]semantics.PrimType, len(args))
	for i, arg := range args {
		argTypes[i] = ec.infer(scope, arg)
	}
	for _, arg := range e.NamedArgs {
		ec.infer(scope, arg.Value)
	}
	if e.Type == nil {
		return semantics.PrimUnknown
	}
	sym, ok := ec.resolver.ResolveQualified(scope, e.Type)
	if !ok || sym == nil || !isBehaviorKind(sym.Kind) {
		return semantics.PrimUnknown
	}
	params, ok := ec.effectiveInParameters(sym)
	if !ok {
		return semantics.PrimUnknown
	}
	ec.checkArguments(e, sym, args, argTypes, params)
	return semantics.PrimUnknown
}

func (ec *exprChecker) checkArguments(
	e *ast.InvocationExpr,
	sym *symbols.Symbol,
	args []ast.Node,
	argTypes []semantics.PrimType,
	params []parameter,
) {
	if len(e.NamedArgs) > 0 {
		names := make(map[string]bool, len(params))
		for _, p := range params {
			names[p.name()] = true
		}
		for _, arg := range e.NamedArgs {
			if arg.Name == nil || len(arg.Name.Parts) != 1 {
				continue
			}
			if name := arg.Name.Parts[0].Text; !names[name] {
				ec.errorf(e.Span(), "%s has no parameter named %q", sym.Name, name)
			}
		}
		return
	}
	required := 0
	for _, p := range params {
		if p.usage.Value == nil {
			required++
		}
	}
	switch {
	case len(args) > len(params):
		ec.errorf(e.Span(), "%s takes %d argument(s), found %d", sym.Name, len(params), len(args))
		return
	case len(args) < required:
		ec.errorf(e.Span(), "%s requires %d argument(s), found %d", sym.Name, required, len(args))
		return
	}
	for i, arg := range args {
		want := ec.declaredPrimType(params[i].scope(), params[i].usage.Relationships)
		got := argTypes[i]
		if want == semantics.PrimUnknown || got == semantics.PrimUnknown {
			continue
		}
		if !semantics.PrimConforms(got, want) {
			ec.errorf(arg.Span(), "argument %d of %s expects %s, found %s", i+1, sym.Name, want, got)
		}
	}
}

// isBehaviorKind reports whether a symbol is a calc or action, the invocable
// kinds whose parameter lists the checker understands.
func isBehaviorKind(k symbols.SymbolKind) bool {
	switch k {
	case symbols.SymbolCalcDef, symbols.SymbolCalcUsage, symbols.SymbolActionDef, symbols.SymbolActionUsage:
		return true
	}
	return false
}

// parameter is one `in` parameter of an invoked behavior together with the
// symbol declaring it, whose scope its type names resolve in.
type parameter struct {
	usage *ast.Usage
	owner *symbols.Symbol
}

// name returns the name the parameter answers to, which a declaration written
// as a redefinition takes from what it redefines (`in redefines ifTest;`).
func (p parameter) name() string {
	name, _ := ast.EffectiveName(p.usage)
	return name
}

// scope returns the scope the parameter's type names resolve in, which is the
// declaring behavior's, not the invoking one's.
func (p parameter) scope() *symbols.Scope {
	if p.owner.Scope != nil {
		return p.owner.Scope
	}
	return p.owner.OwnerScope
}

// effectiveInParameters returns the `in` parameters a calc/action is invoked
// with, in inherited declaration order. A behavior inherits the signature of the
// types it specializes or is typed by; a parameter it declares itself replaces
// the inherited one it redefines and is appended otherwise, so a specialization
// refining only a subset of them keeps the full signature. ok is false when no
// signature can be determined, in which case the invocation is left unchecked.
func (ec *exprChecker) effectiveInParameters(sym *symbols.Symbol) ([]parameter, bool) {
	// Merging runs over every parameter, because redefinition is positional
	// over the whole parameter list (KerML 7.4.7.2), as
	// semantics.Model.parametersOf computes it; only the invocation signature
	// is restricted to the inputs.
	all := ec.mergedParameters(sym, map[*symbols.Symbol]bool{})
	var params []parameter
	for _, p := range all {
		if p.usage.Direction == ast.DirIn || p.usage.Direction == ast.DirInOut {
			params = append(params, p)
		}
	}
	if len(params) > 0 {
		return params, true
	}
	// A parameterless declaration with no supertypes really takes no
	// arguments; with supertypes the signature may live somewhere the checker
	// cannot see, so stay silent.
	return nil, len(ec.model.DirectSupertypes(sym)) == 0 && sym.Decl != nil
}

// mergedParameters returns sym's parameter list: the parameter lists of the
// types it specializes or is typed by, in declaration order, with sym's own
// parameters folded in. Recursing keeps each type's parameters positioned
// against the ones it actually inherits, which is what implicit redefinition is
// relative to; visiting is the set of symbols on the current path, which breaks
// cycles.
func (ec *exprChecker) mergedParameters(sym *symbols.Symbol, visiting map[*symbols.Symbol]bool) []parameter {
	if visiting[sym] {
		return nil
	}
	visiting[sym] = true
	defer delete(visiting, sym)

	var inherited []parameter
	for _, super := range ec.model.DirectSupertypes(sym) {
		for _, p := range ec.mergedParameters(super, visiting) {
			// A parameter reached through more than one supertype (a
			// diamond) contributes one signature entry.
			if i := indexOfName(inherited, p.name()); i >= 0 {
				inherited[i] = p
				continue
			}
			inherited = append(inherited, p)
		}
	}
	return mergeParameters(inherited, sym)
}

// mergeParameters returns a symbol's parameter list: the ones it declares, in
// declaration order, followed by the inherited ones none of them redefines. A
// declaration redefines the inherited parameter its `:>>` names, or, failing
// that, the one at its own position with the same direction; only once the inherited parameters are used
// up does a declaration purely add to the list. This is the order and the
// matching semantics.Model.parametersOf derives (KerML 7.4.7.2), so both tiers
// see one parameter list.
func mergeParameters(inherited []parameter, sym *symbols.Symbol) []parameter {
	declared := declaredParameters(sym)
	if len(declared) == 0 {
		return inherited
	}
	merged := make([]parameter, 0, len(declared)+len(inherited))
	claimed := make([]bool, len(inherited))
	for position, u := range declared {
		merged = append(merged, parameter{usage: u, owner: sym})
		i := indexOfRedefined(inherited, u)
		if i < 0 {
			i = position
		}
		// A position whose directions disagree is not a redefinition, so the
		// inherited parameter there stays in the list.
		if i < len(inherited) && inherited[i].usage.Direction == u.Direction {
			claimed[i] = true
		}
	}
	for i, p := range inherited {
		if !claimed[i] {
			merged = append(merged, p)
		}
	}
	return merged
}

// indexOfRedefined finds the inherited parameter a declaration redefines, named
// by its `:>>` target. A declaration with no explicit target redefines by
// position (KerML 7.4.7.2), which the caller applies, so its own name does not
// select the inherited parameter.
func indexOfRedefined(params []parameter, u *ast.Usage) int {
	for _, name := range redefinedNames(u) {
		if i := indexOfName(params, name); i >= 0 {
			return i
		}
	}
	return -1
}

// indexOfName finds the parameter with the given name, or -1.
func indexOfName(params []parameter, name string) int {
	if name == "" {
		return -1
	}
	for i, p := range params {
		if p.name() == name {
			return i
		}
	}
	return -1
}

// redefinedNames returns the unqualified names a usage redefines (`:>>`).
func redefinedNames(u *ast.Usage) []string {
	var names []string
	for _, rel := range u.Relationships {
		if rel == nil || rel.Kind != ast.RelRedefines {
			continue
		}
		qn, ok := rel.Target.(*ast.QualifiedName)
		if !ok || len(qn.Parts) == 0 {
			continue
		}
		names = append(names, qn.Parts[len(qn.Parts)-1].Text)
	}
	return names
}

// declaredParameters returns the parameters declared directly by a symbol's
// def/usage declaration: its directed features, minus the result parameter,
// which is redefined as the result rather than by position.
func declaredParameters(sym *symbols.Symbol) []*ast.Usage {
	var members []ast.Node
	switch d := sym.Decl.(type) {
	case *ast.Definition:
		members = d.Members
	case *ast.Usage:
		members = d.Members
	default:
		return nil
	}
	var params []*ast.Usage
	for _, m := range members {
		u, ok := unwrapType(m).(*ast.Usage)
		if !ok {
			continue
		}
		if u.Direction != ast.DirNone && !u.IsResult {
			params = append(params, u)
		}
	}
	return params
}
