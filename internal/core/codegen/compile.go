package codegen

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// ErrUnsupported reports notation outside the compiled subset. The message
// names the construct so the model can be adjusted or run interpreted.
var ErrUnsupported = errors.New("not compilable")

// UnsupportedError is an ErrUnsupported with the construct and calc it was met in.
type UnsupportedError struct {
	Calc string
	What string
}

func (e *UnsupportedError) Error() string {
	return fmt.Sprintf("%s: in calc %s: %s", ErrUnsupported, e.Calc, e.What)
}

func (e *UnsupportedError) Unwrap() error { return ErrUnsupported }

// Compiler lowers calc definitions to the codegen IR. One Compiler compiles one
// Program; the functions it produces are shared across calls within it.
type Compiler struct {
	model    *semantics.Model
	resolver *resolve.Resolver
	funcs    map[*symbols.Symbol]*Func
	order    []*Func
}

// New returns a Compiler resolving names through resolver and typing through model.
func New(model *semantics.Model, resolver *resolve.Resolver) *Compiler {
	return &Compiler{model: model, resolver: resolver, funcs: map[*symbols.Symbol]*Func{}}
}

// Compile compiles entry and every calc it invokes, transitively.
func (c *Compiler) Compile(entry *symbols.Symbol) (*Program, error) {
	fn, err := c.compileCalc(entry)
	if err != nil {
		return nil, err
	}
	return &Program{Funcs: c.order, Entry: fn}, nil
}

// env is the lexical environment of a body: parameters and body-local variables,
// innermost block last.
type env struct {
	frames []map[string]binding
}

// binding is the declared type of a variable and the range it is narrowed to.
type binding struct {
	t Type
	r Range
}

func (e *env) push()                    { e.frames = append(e.frames, map[string]binding{}) }
func (e *env) pop()                     { e.frames = e.frames[:len(e.frames)-1] }
func (e *env) bind(n string, b binding) { e.frames[len(e.frames)-1][n] = b }
func (e *env) lookup(n string) (binding, bool) {
	for i := len(e.frames) - 1; i >= 0; i-- {
		if b, ok := e.frames[i][n]; ok {
			return b, true
		}
	}
	return binding{}, false
}

// funcCompiler compiles one calc body.
type funcCompiler struct {
	c     *Compiler
	fn    *Func
	scope *symbols.Scope
	env   env
	// result is the return type once a return has fixed it.
	result Type
}

func (c *Compiler) compileCalc(sym *symbols.Symbol) (*Func, error) {
	if fn, ok := c.funcs[sym]; ok {
		return fn, nil
	}
	if sym == nil || sym.Decl == nil {
		return nil, &UnsupportedError{Calc: "?", What: "no declaration"}
	}
	body, rels, err := calcDecl(sym.Decl)
	if err != nil {
		return nil, &UnsupportedError{Calc: c.name(sym), What: err.Error()}
	}
	if len(rels) > 0 {
		return c.compileInheriting(sym, body, rels)
	}

	fn := &Func{Name: c.name(sym), Ident: identOf(sym)}
	// Registered before the body is compiled so recursion finds it; the result
	// type of a recursive call is fixed by an earlier return (see compileCall).
	c.funcs[sym] = fn
	c.order = append(c.order, fn)

	fc := &funcCompiler{c: c, fn: fn, scope: sym.Scope}
	fc.env.push()
	for _, member := range unwrapped(body) {
		u, ok := member.(*ast.Usage)
		if !ok {
			continue
		}
		if u.Direction != ast.DirIn && u.Direction != ast.DirInOut {
			continue
		}
		name, _ := ast.EffectiveName(u)
		if name == "" {
			return nil, fc.unsupported("an unnamed parameter")
		}
		if u.Value != nil {
			return nil, fc.unsupported(fmt.Sprintf("parameter %s has a default value", name))
		}
		if err := fc.exactlyOne(u, "parameter "+name); err != nil {
			return nil, err
		}
		t, r, err := fc.declaredType(sym.Scope, u, name)
		if err != nil {
			return nil, err
		}
		fn.Params = append(fn.Params, Param{Name: name, Type: t, Range: r})
		fc.env.bind(name, binding{t, r})
	}

	stmts := lower.CalcBody(body, sym.Scope)
	if !lower.Returns(stmts) {
		return nil, fc.unsupported("a calc that binds `out` features rather than returning a value")
	}
	if declared, r, err := fc.declaredResult(sym, body); err != nil {
		return nil, err
	} else if declared != TypeInvalid {
		fc.result = declared
		fn.Result = declared
		fn.ResultRange = r
	}
	compiled, err := fc.compileBlock(stmts)
	if err != nil {
		return nil, err
	}
	if fc.result == TypeInvalid {
		return nil, fc.unsupported("the result type could not be inferred")
	}
	fn.Result = fc.result
	fn.Body = compiled
	return fn, nil
}

// calcDecl is the body and the declared relationships of a calc def or a calc
// usage; any other declaration is not a calc.
func calcDecl(decl ast.Node) ([]ast.Node, []*ast.Relationship, error) {
	switch d := decl.(type) {
	case *ast.Definition:
		if d.Kind == ast.DefCalc {
			return d.Members, d.Relationships, nil
		}
	case *ast.Usage:
		if d.Kind == ast.UsageCalc {
			return d.Members, d.Relationships, nil
		}
	}
	return nil, nil, errors.New("not a calc def or calc usage")
}

// compileInheriting compiles a calc that specializes or is typed by another.
// The interpreter flattens parameters and body along the chain of calc
// supertypes; a calc adding no member of its own is that chain's one other
// calc, so it compiles to that calc's function. Anything richer is refused.
func (c *Compiler) compileInheriting(sym *symbols.Symbol, body []ast.Node, rels []*ast.Relationship) (*Func, error) {
	var parent *symbols.Symbol
	for _, super := range c.model.DirectSupertypes(sym) {
		if super == nil || !isCalc(super.Decl) || c.resolver.Index().Library(super) {
			continue
		}
		if parent != nil && parent != super {
			return nil, &UnsupportedError{Calc: c.name(sym), What: "a calc inheriting from several calcs"}
		}
		parent = super
	}
	if parent == nil {
		return nil, &UnsupportedError{Calc: c.name(sym), What: "a calc declaring `" + rels[0].Kind.String() + "` of something that is not a compiled calc"}
	}
	if len(unwrapped(body)) > 0 {
		return nil, &UnsupportedError{Calc: c.name(sym), What: "a calc declaring `" + rels[0].Kind.String() + "` and members of its own; redefinition of inherited parameters and body is not compiled"}
	}
	fn, err := c.compileCalc(parent)
	if err != nil {
		return nil, err
	}
	c.funcs[sym] = fn
	return fn, nil
}

func isCalc(decl ast.Node) bool {
	_, _, err := calcDecl(decl)
	return err == nil
}

// declaredResult is the type and range the `return` parameter declares,
// TypeInvalid when it declares none.
func (fc *funcCompiler) declaredResult(sym *symbols.Symbol, body []ast.Node) (Type, Range, error) {
	for _, member := range unwrapped(body) {
		u, ok := member.(*ast.Usage)
		if !ok || !(u.IsResult || u.Direction == ast.DirOut) {
			continue
		}
		if u.Direction == ast.DirOut && !u.IsResult {
			return TypeInvalid, RangeAny, fc.unsupported("an `out` parameter")
		}
		if err := fc.exactlyOne(u, "the result"); err != nil {
			return TypeInvalid, RangeAny, err
		}
		name, _ := ast.EffectiveName(u)
		if !hasTyping(u) {
			return TypeInvalid, RangeAny, nil
		}
		return fc.declaredType(sym.Scope, u, name)
	}
	return TypeInvalid, RangeAny, nil
}

// exactlyOne accepts a usage declaring no multiplicity or one equivalent to
// `[1]`; a scalar is one value.
func (fc *funcCompiler) exactlyOne(u *ast.Usage, what string) error {
	rng, ok := fc.c.model.RangeOf(u.Multiplicity)
	if !ok {
		return nil
	}
	one := func(b semantics.Bound) bool { return b.Known && !b.Infinite && b.Value == 1 }
	if one(rng.Lower) && one(rng.Upper) {
		return nil
	}
	return fc.unsupported(what + " declares a multiplicity other than [1]")
}

func hasTyping(u *ast.Usage) bool {
	for _, r := range u.Relationships {
		if r != nil && r.Kind == ast.RelTyping {
			return true
		}
	}
	return false
}

// declaredType resolves the scalar type a usage is typed by, and its range.
func (fc *funcCompiler) declaredType(scope *symbols.Scope, u *ast.Usage, name string) (Type, Range, error) {
	var typ *symbols.Symbol
	for _, r := range u.Relationships {
		if r == nil || r.Kind != ast.RelTyping || r.Target == nil {
			continue
		}
		target := r.Target
		if fr, ok := target.(*ast.FeatureReference); ok {
			target = fr.Name
		}
		qn, ok := target.(*ast.QualifiedName)
		if !ok {
			return TypeInvalid, RangeAny, fc.unsupported(fmt.Sprintf("%s: typing by an expression", name))
		}
		resolved, ok := fc.c.resolver.ResolveQualified(scope, qn)
		if !ok {
			return TypeInvalid, RangeAny, fc.unsupported(fmt.Sprintf("%s: type %s does not resolve", name, qnText(qn)))
		}
		if typ != nil {
			return TypeInvalid, RangeAny, fc.unsupported(fmt.Sprintf("%s is typed more than once", name))
		}
		typ = resolved
	}
	if typ == nil {
		return TypeInvalid, RangeAny, fc.unsupported(fmt.Sprintf("%s declares no type", name))
	}
	t, r, ok := scalarType(fc.c.name(typ))
	if !ok {
		return TypeInvalid, RangeAny, fc.unsupported(fmt.Sprintf("%s: type %s is not Integer, Real or Boolean", name, fc.c.name(typ)))
	}
	return t, r, nil
}

// scalarType maps a library data type to the compiled representation and the
// range a write to it is checked against.
func scalarType(fqn string) (Type, Range, bool) {
	switch fqn {
	case "ScalarValues::Integer":
		return TypeInt, RangeAny, true
	case "ScalarValues::Natural":
		return TypeInt, RangeNatural, true
	case "ScalarValues::Positive":
		return TypeInt, RangePositive, true
	case "ScalarValues::Real", "ScalarValues::Rational":
		return TypeReal, RangeAny, true
	case "ScalarValues::Boolean":
		return TypeBool, RangeAny, true
	}
	return TypeInvalid, RangeAny, false
}

func (fc *funcCompiler) compileBlock(stmts []lower.Statement) ([]Stmt, error) {
	var out []Stmt
	for _, s := range stmts {
		compiled, err := fc.compileStmt(s)
		if err != nil {
			return nil, err
		}
		if compiled != nil {
			out = append(out, compiled)
		}
	}
	return out, nil
}

func (fc *funcCompiler) compileStmt(s lower.Statement) (Stmt, error) {
	switch s := s.(type) {
	case lower.Declare:
		return fc.compileDeclare(s)
	case lower.Assign:
		if s.Chain != nil {
			return nil, fc.unsupported("assignment through a feature chain")
		}
		b, ok := fc.env.lookup(s.Target)
		if !ok {
			return nil, fc.unsupported(fmt.Sprintf("assignment to %s, which the body does not declare", s.Target))
		}
		v, err := fc.compileExpr(s.Value)
		if err != nil {
			return nil, err
		}
		v, err = fc.coerce(v, b.t, "assigned to "+s.Target)
		if err != nil {
			return nil, err
		}
		return Assign{Name: s.Target, Range: b.r, Value: v}, nil
	case lower.If:
		cond, err := fc.compileBool(s.Condition, "condition of if")
		if err != nil {
			return nil, err
		}
		fc.env.push()
		then, err := fc.compileBlock(s.Then.Steps())
		fc.env.pop()
		if err != nil {
			return nil, err
		}
		var els []Stmt
		if s.Else != nil {
			fc.env.push()
			els, err = fc.compileBlock(s.Else.Steps())
			fc.env.pop()
			if err != nil {
				return nil, err
			}
		}
		return If{Cond: cond, Then: then, Else: els}, nil
	case lower.Loop:
		return fc.compileLoop(s)
	case lower.Return:
		v, err := fc.compileExpr(s.Value)
		if err != nil {
			return nil, err
		}
		if fc.result == TypeInvalid {
			fc.result = v.Type()
		}
		v, err = fc.coerce(v, fc.result, "returned")
		if err != nil {
			return nil, err
		}
		return Return{Value: v}, nil
	case lower.Block:
		fc.env.push()
		body, err := fc.compileBlock(s.Steps())
		fc.env.pop()
		if err != nil {
			return nil, err
		}
		return If{Cond: BoolLit{Value: true}, Then: body}, nil
	case lower.Unsupported:
		return nil, fc.unsupported(s.Description)
	case lower.DeclareUsage:
		return nil, fc.unsupported("a body-local calc usage")
	case lower.Effect:
		return nil, fc.unsupported("a statement acting outside the calc")
	default:
		return nil, fc.unsupported(fmt.Sprintf("statement %T", s))
	}
}

func (fc *funcCompiler) compileDeclare(s lower.Declare) (Stmt, error) {
	u, _ := s.Node.(*ast.Usage)
	var declared Type
	var rng Range
	if u != nil && hasTyping(u) {
		t, r, err := fc.declaredType(s.Scope, u, s.Name)
		if err != nil {
			return nil, err
		}
		declared, rng = t, r
	}
	if u != nil {
		if err := fc.exactlyOne(u, "attribute "+s.Name); err != nil {
			return nil, err
		}
	}
	if s.Value == nil {
		return nil, fc.unsupported(fmt.Sprintf("attribute %s has no value; an unbound attribute is null, which the compiled types cannot hold", s.Name))
	}
	v, err := fc.compileExpr(s.Value)
	if err != nil {
		return nil, err
	}
	if declared == TypeInvalid {
		declared = v.Type()
	}
	init, err := fc.coerce(v, declared, "bound to "+s.Name)
	if err != nil {
		return nil, err
	}
	fc.env.bind(s.Name, binding{declared, rng})
	return Declare{Name: s.Name, T: declared, Init: init}, nil
}

func (fc *funcCompiler) compileLoop(s lower.Loop) (Stmt, error) {
	if s.Kind == ast.LoopFor {
		return nil, fc.unsupported("a `for` loop over a collection")
	}
	loop := While{Cond: BoolLit{Value: true}}
	if s.Condition != nil {
		cond, err := fc.compileBool(s.Condition, "condition of while")
		if err != nil {
			return nil, err
		}
		loop.Cond = cond
	}
	if s.Until != nil {
		until, err := fc.compileBool(s.Until, "condition of until")
		if err != nil {
			return nil, err
		}
		loop.Until = until
	}
	if s.Condition == nil && s.Until == nil {
		return nil, fc.unsupported("a loop with neither a while nor an until condition")
	}
	fc.env.push()
	body, err := fc.compileBlock(s.Body.Steps())
	fc.env.pop()
	if err != nil {
		return nil, err
	}
	loop.Body = body
	return loop, nil
}

func (fc *funcCompiler) compileBool(n ast.Node, what string) (Expr, error) {
	v, err := fc.compileExpr(n)
	if err != nil {
		return nil, err
	}
	if v.Type() != TypeBool {
		return nil, fc.unsupported(fmt.Sprintf("%s is %s, not Boolean", what, v.Type()))
	}
	return v, nil
}

// coerce widens an Integer to a Real where a Real is expected; any other
// mismatch is outside the subset.
func (fc *funcCompiler) coerce(v Expr, t Type, what string) (Expr, error) {
	switch {
	case v.Type() == t:
		return v, nil
	case v.Type() == TypeInt && t == TypeReal:
		return ToReal{X: v}, nil
	}
	return nil, fc.unsupported(fmt.Sprintf("a %s %s where %s is expected", v.Type(), what, t))
}

func (fc *funcCompiler) compileExpr(n ast.Node) (Expr, error) {
	switch n := n.(type) {
	case *ast.LiteralInteger:
		v, err := strconv.ParseInt(n.Value, 10, 64)
		if err != nil {
			return nil, fc.unsupported(fmt.Sprintf("literal %s is outside the Integer range", n.Value))
		}
		return IntLit{Value: v}, nil
	case *ast.LiteralReal:
		v, err := strconv.ParseFloat(n.Value, 64)
		if err != nil {
			return nil, fc.unsupported(fmt.Sprintf("literal %s is outside the Real range", n.Value))
		}
		return RealLit{Value: v}, nil
	case *ast.LiteralBool:
		return BoolLit{Value: n.Value}, nil
	case *ast.FeatureReference:
		return fc.compileName(n.Name)
	case *ast.OperatorExpr:
		return fc.compileOperator(n)
	case *ast.InvocationExpr:
		return fc.compileCall(n)
	case *ast.BodyExpr:
		// A parenthesized expression parses as a body holding one expression.
		if len(n.Members) == 1 {
			if inner := unwrap(n.Members[0]); inner != nil {
				if _, decl := inner.(*ast.Usage); !decl {
					return fc.compileExpr(inner)
				}
			}
		}
		return nil, fc.unsupported("an expression body")
	}
	return nil, fc.unsupported(fmt.Sprintf("expression %T", n))
}

func unwrap(n ast.Node) ast.Node {
	if m, ok := n.(*ast.Membership); ok {
		return m.Member
	}
	return n
}

func (fc *funcCompiler) compileName(qn *ast.QualifiedName) (Expr, error) {
	if qn == nil || len(qn.Parts) != 1 {
		return nil, fc.unsupported(fmt.Sprintf("reference to %s, which is not a parameter or body-local attribute", qnText(qn)))
	}
	name := qn.Parts[0].Text
	if b, ok := fc.env.lookup(name); ok {
		return Var{Name: name, T: b.t}, nil
	}
	return nil, fc.unsupported(fmt.Sprintf("reference to %s, which is not a parameter or body-local attribute", name))
}

func (fc *funcCompiler) compileOperator(n *ast.OperatorExpr) (Expr, error) {
	switch n.Operator {
	case ast.OpConditional:
		if len(n.Operands) != 3 {
			return nil, fc.unsupported("an `if` with no else")
		}
		cond, err := fc.compileBool(n.Operands[0], "condition of if")
		if err != nil {
			return nil, err
		}
		then, err := fc.compileExpr(n.Operands[1])
		if err != nil {
			return nil, err
		}
		els, err := fc.compileExpr(n.Operands[2])
		if err != nil {
			return nil, err
		}
		t, err := fc.unify(then, els, "branches of if")
		if err != nil {
			return nil, err
		}
		then, _ = fc.coerce(then, t, "")
		els, _ = fc.coerce(els, t, "")
		return Cond{C: cond, Then: then, Else: els, T: t}, nil
	case ast.OpAdd, ast.OpSub, ast.OpMul, ast.OpDiv, ast.OpMod, ast.OpPow:
		l, r, t, err := fc.numericOperands(n)
		if err != nil {
			return nil, err
		}
		if n.Operator == ast.OpDiv {
			// A quotient of Integers is a Rational.
			t = TypeReal
		}
		if n.Operator == ast.OpPow && t == TypeInt {
			// Integer ** Integer is an Integer only for a non-negative exponent,
			// a distinction a static type cannot make of a run-time exponent.
			lit, isLit := r.(IntLit)
			switch {
			case isLit && lit.Value >= 0:
			case isLit:
				t = TypeReal
				l, r = ToReal{X: l}, ToReal{X: r}
			default:
				return nil, fc.unsupported("`**` of an Integer by a non-literal Integer exponent (write the exponent as a literal, or make the base Real)")
			}
		}
		return Binary{Op: n.Operator, L: l, R: r, T: t}, nil
	case ast.OpLt, ast.OpLe, ast.OpGt, ast.OpGe:
		l, r, _, err := fc.numericOperands(n)
		if err != nil {
			return nil, err
		}
		return Binary{Op: n.Operator, L: l, R: r, T: TypeBool}, nil
	case ast.OpEq, ast.OpNeq:
		l, r, err := fc.binaryOperands(n)
		if err != nil {
			return nil, err
		}
		if l.Type() == TypeBool || r.Type() == TypeBool {
			if l.Type() != r.Type() {
				return nil, fc.unsupported("equality between a Boolean and a number")
			}
			return Binary{Op: n.Operator, L: l, R: r, T: TypeBool}, nil
		}
		t, _ := fc.unify(l, r, "")
		l, _ = fc.coerce(l, t, "")
		r, _ = fc.coerce(r, t, "")
		return Binary{Op: n.Operator, L: l, R: r, T: TypeBool}, nil
	case ast.OpAnd, ast.OpConditionalAnd, ast.OpOr, ast.OpConditionalOr, ast.OpXor, ast.OpImplies:
		l, r, err := fc.binaryOperands(n)
		if err != nil {
			return nil, err
		}
		if l.Type() != TypeBool || r.Type() != TypeBool {
			return nil, fc.unsupported(fmt.Sprintf("'%s' over non-Boolean operands", n.Operator))
		}
		return Binary{Op: n.Operator, L: l, R: r, T: TypeBool}, nil
	case ast.OpNeg, ast.OpPos:
		if len(n.Operands) != 1 {
			return nil, fc.unsupported("a unary operator with two operands")
		}
		// The least Integer is only writable as a negated literal.
		if lit, ok := n.Operands[0].(*ast.LiteralInteger); ok && n.Operator == ast.OpNeg {
			if v, err := strconv.ParseInt("-"+lit.Value, 10, 64); err == nil {
				return IntLit{Value: v}, nil
			}
		}
		x, err := fc.compileExpr(n.Operands[0])
		if err != nil {
			return nil, err
		}
		if x.Type() == TypeBool {
			return nil, fc.unsupported(fmt.Sprintf("'%s' over a Boolean", n.Operator))
		}
		return Unary{Op: n.Operator, X: x, T: x.Type()}, nil
	case ast.OpNot:
		if len(n.Operands) != 1 {
			return nil, fc.unsupported("a unary operator with two operands")
		}
		x, err := fc.compileBool(n.Operands[0], "operand of not")
		if err != nil {
			return nil, err
		}
		return Unary{Op: n.Operator, X: x, T: TypeBool}, nil
	}
	return nil, fc.unsupported(fmt.Sprintf("operator '%s'", n.Operator))
}

func (fc *funcCompiler) binaryOperands(n *ast.OperatorExpr) (Expr, Expr, error) {
	if len(n.Operands) != 2 {
		return nil, nil, fc.unsupported(fmt.Sprintf("'%s' with %d operands", n.Operator, len(n.Operands)))
	}
	l, err := fc.compileExpr(n.Operands[0])
	if err != nil {
		return nil, nil, err
	}
	r, err := fc.compileExpr(n.Operands[1])
	if err != nil {
		return nil, nil, err
	}
	return l, r, nil
}

// numericOperands compiles both operands and widens them to a common numeric type.
func (fc *funcCompiler) numericOperands(n *ast.OperatorExpr) (Expr, Expr, Type, error) {
	l, r, err := fc.binaryOperands(n)
	if err != nil {
		return nil, nil, TypeInvalid, err
	}
	if l.Type() == TypeBool || r.Type() == TypeBool {
		return nil, nil, TypeInvalid, fc.unsupported(fmt.Sprintf("'%s' over a Boolean", n.Operator))
	}
	t, _ := fc.unify(l, r, "")
	l, _ = fc.coerce(l, t, "")
	r, _ = fc.coerce(r, t, "")
	return l, r, t, nil
}

// unify is the common type of two numeric expressions: Real if either is.
func (fc *funcCompiler) unify(a, b Expr, what string) (Type, error) {
	switch {
	case a.Type() == b.Type():
		return a.Type(), nil
	case a.Type() == TypeBool || b.Type() == TypeBool:
		return TypeInvalid, fc.unsupported(fmt.Sprintf("%s mix a Boolean and a number", what))
	}
	return TypeReal, nil
}

func (fc *funcCompiler) compileCall(n *ast.InvocationExpr) (Expr, error) {
	if n.Operand != nil {
		return nil, fc.unsupported("an invocation with a receiver (`x->F()`)")
	}
	if n.Type == nil {
		return nil, fc.unsupported("an invocation naming no calc")
	}
	sym, ok := fc.c.resolver.ResolveQualified(fc.scope, n.Type)
	if !ok {
		return nil, fc.unsupported(fmt.Sprintf("invocation of %s, which does not resolve", qnText(n.Type)))
	}
	if !isCalc(sym.Decl) || fc.c.resolver.Index().Library(sym) {
		return nil, fc.unsupported(fmt.Sprintf("invocation of %s, which is not a calc of the model (library functions are not compiled)", fc.c.name(sym)))
	}
	callee, err := fc.c.compileCalc(sym)
	if err != nil {
		return nil, err
	}
	var args []Arg
	bound := make([]bool, len(callee.Params))
	if len(n.NamedArgs) > 0 {
		for _, na := range n.NamedArgs {
			i := paramIndex(callee, na.Name)
			if i < 0 {
				return nil, fc.unsupported(fmt.Sprintf("%s has no parameter %s", callee.Name, qnText(na.Name)))
			}
			v, err := fc.compileArg(na.Value, callee.Params[i], callee)
			if err != nil {
				return nil, err
			}
			args = append(args, Arg{Param: i, Value: v})
			bound[i] = true
		}
	} else {
		if len(n.Args) != len(callee.Params) {
			return nil, fc.unsupported(fmt.Sprintf("%s takes %d arguments, %d given", callee.Name, len(callee.Params), len(n.Args)))
		}
		for i, a := range n.Args {
			v, err := fc.compileArg(a, callee.Params[i], callee)
			if err != nil {
				return nil, err
			}
			args = append(args, Arg{Param: i, Value: v})
			bound[i] = true
		}
	}
	for i, b := range bound {
		if !b {
			return nil, fc.unsupported(fmt.Sprintf("%s: parameter %s is not bound", callee.Name, callee.Params[i].Name))
		}
	}
	if callee.Result == TypeInvalid {
		// Reached only from inside callee's own body, before a return fixed it.
		if fc.fn != callee || fc.result == TypeInvalid {
			return nil, fc.unsupported(fmt.Sprintf("call of %s before a return fixes its result type; declare its return type", callee.Name))
		}
		callee.Result = fc.result
	}
	return Call{Fn: callee, Args: args}, nil
}

func (fc *funcCompiler) compileArg(n ast.Node, p Param, callee *Func) (Expr, error) {
	v, err := fc.compileExpr(n)
	if err != nil {
		return nil, err
	}
	return fc.coerce(v, p.Type, fmt.Sprintf("argument for %s of %s", p.Name, callee.Name))
}

func paramIndex(fn *Func, name *ast.QualifiedName) int {
	if name == nil || len(name.Parts) != 1 {
		return -1
	}
	for i, p := range fn.Params {
		if p.Name == name.Parts[0].Text {
			return i
		}
	}
	return -1
}

func (fc *funcCompiler) unsupported(what string) error {
	return &UnsupportedError{Calc: fc.fn.Name, What: what}
}

func (c *Compiler) name(sym *symbols.Symbol) string {
	if idx := c.resolver.Index(); idx != nil {
		if fqn := idx.GetFQN(sym); fqn != "" {
			return fqn
		}
	}
	return sym.Name
}

func qnText(qn *ast.QualifiedName) string {
	if qn == nil {
		return ""
	}
	parts := make([]string, len(qn.Parts))
	for i, p := range qn.Parts {
		parts[i] = p.Text
	}
	return strings.Join(parts, "::")
}

// unwrapped is members with each membership wrapper removed.
func unwrapped(members []ast.Node) []ast.Node {
	out := make([]ast.Node, 0, len(members))
	for _, m := range members {
		if inner := unwrap(m); inner != nil {
			out = append(out, inner)
		}
	}
	return out
}

// identOf gives a symbol a C/Go identifier, injectively: names of the owner
// chain are joined by `_s_`, non-alphanumeric runes (`_` too) become `_hex_`.
func identOf(sym *symbols.Symbol) string {
	var names []string
	for s := sym; s != nil; {
		names = append(names, s.Name)
		if s.OwnerScope == nil {
			break
		}
		s = s.OwnerScope.Owner()
	}
	var b strings.Builder
	b.WriteString("calc")
	for i := len(names) - 1; i >= 0; i-- {
		b.WriteString("_s_")
		encodeName(&b, names[i])
	}
	return b.String()
}

func encodeName(b *strings.Builder, name string) {
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			fmt.Fprintf(b, "_%x_", r)
		}
	}
}
