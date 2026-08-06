package passes

import (
	"fmt"

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
	}
	return semantics.PrimUnknown
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
			names[p.usage.Ident.Name] = true
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
	params := ec.mergedInParameters(sym, map[*symbols.Symbol]bool{})
	if len(params) > 0 {
		return params, true
	}
	// A parameterless declaration with no supertypes really takes no
	// arguments; with supertypes the signature may live somewhere the checker
	// cannot see, so stay silent.
	return nil, len(ec.model.DirectSupertypes(sym)) == 0 && sym.Decl != nil
}

// mergedInParameters returns sym's signature: the signatures of the types it
// specializes or is typed by, in declaration order, with sym's own parameters
// folded in. Recursing keeps each type's parameters positioned against the ones
// it actually inherits, which is what implicit redefinition is relative to;
// visiting is the set of symbols on the current path, which breaks cycles.
func (ec *exprChecker) mergedInParameters(sym *symbols.Symbol, visiting map[*symbols.Symbol]bool) []parameter {
	if visiting[sym] {
		return nil
	}
	visiting[sym] = true
	defer delete(visiting, sym)

	var inherited []parameter
	for _, super := range ec.model.DirectSupertypes(sym) {
		for _, p := range ec.mergedInParameters(super, visiting) {
			// A parameter reached through more than one supertype (a
			// diamond) contributes one signature entry.
			if i := indexOfName(inherited, p.usage.Ident.Name); i >= 0 {
				inherited[i] = p
				continue
			}
			inherited = append(inherited, p)
		}
	}
	return mergeParameters(inherited, sym)
}

// mergeParameters folds a symbol's own `in` parameters into an inherited list,
// replacing the entry each one redefines and appending the rest. A declaration
// that names neither a `:>>` target nor an inherited parameter redefines the
// next inherited one by position, which is how a specializing behavior refines a
// parameter under a new name; only once the inherited parameters are used up
// does a declaration add to the signature.
func mergeParameters(inherited []parameter, sym *symbols.Symbol) []parameter {
	declared := declaredInParameters(sym)
	if len(declared) == 0 {
		return inherited
	}
	merged := append([]parameter(nil), inherited...)
	next := 0 // first inherited parameter not yet redefined
	for _, u := range declared {
		p := parameter{usage: u, owner: sym}
		i := indexOfRedefined(merged, u)
		if i < 0 && next < len(inherited) {
			i = next
		}
		if i < 0 {
			merged = append(merged, p)
			continue
		}
		merged[i] = p
		if i >= next {
			next = i + 1
		}
	}
	return merged
}

// indexOfRedefined finds the inherited parameter a declaration redefines, named
// by its `:>>` target when it has one and by its own name otherwise.
func indexOfRedefined(params []parameter, u *ast.Usage) int {
	names := redefinedNames(u)
	if len(names) == 0 && u.Ident.Name != "" {
		names = []string{u.Ident.Name}
	}
	for _, name := range names {
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
		if p.usage.Ident.Name == name {
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

// declaredInParameters returns the `in`/`inout` features declared directly by a
// symbol's def/usage declaration.
func declaredInParameters(sym *symbols.Symbol) []*ast.Usage {
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
		if u.Direction == ast.DirIn || u.Direction == ast.DirInOut {
			params = append(params, u)
		}
	}
	return params
}
