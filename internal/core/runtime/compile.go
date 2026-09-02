package runtime

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/envvar"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// CalcCompileEnvVar switches the compiled tier for pure calc bodies off when set
// to 0 (or false/off/no), leaving every calc to the reference evaluator.
const CalcCompileEnvVar = "OPENSYSML_CALC_COMPILE"

// CalcCompileFromEnv reports whether the environment leaves the compiled calc
// tier on, which it does unless CalcCompileEnvVar (or its legacy SYSML_ name)
// switches it off.
func CalcCompileFromEnv() bool {
	return calcCompileFromValue(envvar.Lookup(CalcCompileEnvVar))
}

// calcCompileFromValue reads the switch's value: unset or empty leaves it on.
func calcCompileFromValue(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "0", "false", "off", "no":
		return false
	}
	return true
}

// SetCalcCompile turns the compiled tier on or off for this context.
func (ctx *Context) SetCalcCompile(on bool) { ctx.compileCalcs = on }

// CalcCompile reports whether this context runs eligible calc bodies compiled.
func (ctx *Context) CalcCompile() bool { return ctx.compileCalcs }

// compileState is where a shape stands with the compiled tier.
type compileState uint8

const (
	compileUndecided compileState = iota
	compileInProgress
	compileEligible
	compileIneligible
)

// compiledCalc is a calc body in the compiled tier: parameters bound by slot,
// the body a tree of closures over unboxed scalars, and the callees it invokes
// resolved to their own compiled bodies.
type compiledCalc struct {
	name   string
	params []compiledParam
	// minArgs is the fewest positional arguments that bind every parameter
	// without a default, so an invocation short of it keeps the evaluator.
	minArgs int
	result  *scalarCheck
	body    compiledExpr
	// bodyErr prefixes an error the body raises and resultWhat names the result
	// in a refusal, each worded as the evaluator does for this body form.
	bodyErr    string
	resultWhat string
}

// compiledParam is one parameter: its compiled default, if any, and the check
// on the value bound to it.
type compiledParam struct {
	name  string
	dflt  compiledExpr
	check scalarCheck
}

// scalarCheck is a parameter's or result's declaration, decided for a scalar
// without boxing where the scalar lattice decides it.
type scalarCheck struct {
	decl    *calcMemberDecl
	countOK bool
	// Whether the declared type holds a value of each lattice type a scalar has.
	boolOK, naturalOK, integerOK, rationalOK, realOK bool
}

// accepts reports whether the declaration surely holds v, deciding on the
// scalar lattice alone; a value it declines is left to refuse.
func (c *scalarCheck) accepts(v scalar) bool {
	if !c.countOK {
		return false
	}
	switch v.kind {
	case scalarInt:
		return c.integerOK || (c.naturalOK && v.int() >= 0)
	case scalarBool:
		return c.boolOK
	}
	return c.acceptsReal(v)
}

// acceptsReal places a Real on the lattice by its value, as the evaluator does.
func (c *scalarCheck) acceptsReal(v scalar) bool {
	switch semantics.PrimTypeOfValue(v.semantic()) {
	case semantics.PrimNatural:
		return c.naturalOK
	case semantics.PrimInteger:
		return c.integerOK
	case semantics.PrimRational:
		return c.rationalOK
	case semantics.PrimReal:
		return c.realOK
	}
	return false
}

// refuse is the evaluator's verdict on a value accepts declined, so a refusal
// carries the reference's message and a value it would take is taken.
func (c *scalarCheck) refuse(ctx *Context, v scalar, what func() string) error {
	boxed := v.boxed()
	return c.decl.check(ctx, &boxed, what)
}

// compiledCalcOf answers the compiled body of shape, compiling it and every
// calc it invokes on first ask, or nil when the shape is ineligible.
func (ctx *Context) compiledCalcOf(shape *calcShape) *compiledCalc {
	switch shape.compileState {
	case compileEligible:
		return shape.compiled
	case compileIneligible:
		return nil
	}
	batch := &compileBatch{ctx: ctx}
	batch.compile(shape)
	batch.settle()
	if shape.compileState == compileEligible {
		return shape.compiled
	}
	return nil
}

// compileBatch is one compilation and the calc call graph it pulls in. A shape
// calling one still being compiled (a recursion or a cycle) takes its cell on
// trust, so the batch settles every call once each shape has an answer.
type compileBatch struct {
	ctx *Context
	// callees records the shapes each member compiled a call to.
	callees map[*calcShape][]*calcShape
}

// compile decides shape, compiling its callees first.
func (b *compileBatch) compile(shape *calcShape) {
	shape.compileState = compileInProgress
	shape.compiled = &compiledCalc{name: shape.Name}
	c := &calcCompiler{batch: b, ctx: b.ctx, shape: shape}
	if err := c.compile(shape.compiled); err != nil {
		shape.withdraw(err.Error())
		return
	}
	shape.compileState = compileEligible
}

// withdraw keeps shape on the evaluator for good, for the reason given.
func (shape *calcShape) withdraw(why string) {
	shape.compileState = compileIneligible
	shape.compiled = nil
	shape.ineligibleWhy = why
}

// call records that member compiled a call to callee.
func (b *compileBatch) call(member, callee *calcShape) {
	if b.callees == nil {
		b.callees = map[*calcShape][]*calcShape{}
	}
	b.callees[member] = append(b.callees[member], callee)
}

// settle withdraws eligibility from every member calling a shape that turned
// out ineligible, until no eligible member calls one.
func (b *compileBatch) settle() {
	for changed := true; changed; {
		changed = false
		for member, callees := range b.callees {
			if member.compileState != compileEligible {
				continue
			}
			for _, callee := range callees {
				if callee.compileState != compileEligible {
					member.withdraw("callee " + callee.Name + " is ineligible")
					changed = true
					break
				}
			}
		}
	}
}

// ineligible says why a body stays with the evaluator; the reason is recorded
// on the shape and never reaches a user.
func ineligible(why string) error { return errors.New(why) }

// calcCompiler compiles one shape's parameters and body.
type calcCompiler struct {
	batch *compileBatch
	ctx   *Context
	shape *calcShape
}

// compile fills cell with the shape's compiled form, or says why it cannot.
func (c *calcCompiler) compile(cell *compiledCalc) error {
	shape := c.shape
	value, scope, err := c.resultExpression(cell)
	if err != nil {
		return err
	}
	if scope == nil || scope.BodyLocal() {
		return ineligible("result without a plain expression scope")
	}
	for i := range shape.Outputs {
		if !shape.Outputs[i].IsResult {
			return ineligible(fmt.Sprintf("output feature %q beside the result", shape.Outputs[i].Name))
		}
	}
	if name, redeclared := c.redeclaredParameter(); redeclared {
		return ineligible(fmt.Sprintf("parameter %q is redeclared along the specialization chain", name))
	}
	cell.params = make([]compiledParam, len(shape.Params))
	for i := range shape.Params {
		param := &shape.Params[i]
		check, ok := c.scalarCheckFor(&param.Decl)
		if !ok {
			return ineligible(fmt.Sprintf("parameter %q declares a type outside the scalar lattice", param.Name))
		}
		cell.params[i] = compiledParam{name: param.Name, check: check}
		if param.Default == nil {
			cell.minArgs = i + 1
			continue
		}
		dflt, err := c.compileNode(param.Default, c.ctx.calcScope(param.Owner, shape.Sym, nil), i)
		if err != nil {
			return err
		}
		if !dflt.isConst {
			return ineligible(fmt.Sprintf("default of %q is not a literal", param.Name))
		}
		cell.params[i].dflt = dflt.expr()
	}
	if out := shape.resultOutput(); out != nil {
		check, ok := c.scalarCheckFor(&out.Decl)
		if !ok {
			return ineligible("result declares a type outside the scalar lattice")
		}
		cell.result = &check
	}
	body, err := c.compileNode(value, scope, len(shape.Params))
	if err != nil {
		return err
	}
	cell.body = body.expr()
	return nil
}

// resultExpression finds the one expression the body yields: a lone `return`
// statement's, or a bound result expression of a body without statements. It
// sets the wording the evaluator gives each form's errors on cell.
func (c *calcCompiler) resultExpression(cell *compiledCalc) (ast.Node, *symbols.Scope, error) {
	shape := c.shape
	switch len(shape.Steps) {
	case 0:
		if shape.ResultExpr == nil {
			return nil, nil, ineligible("body without a result expression")
		}
		cell.bodyErr = fmt.Sprintf("calc %s: output result: ", shape.Name)
		cell.resultWhat = fmt.Sprintf("calc %s: output result", shape.Name)
		return shape.ResultExpr, c.ctx.calcScope(shape.Sym, shape.Sym, nil), nil
	case 1:
		ret, ok := shape.Steps[0].(lower.Return)
		if !ok {
			return nil, nil, ineligible("body is not a single return")
		}
		if ret.Value == nil {
			return nil, nil, ineligible("return without a value")
		}
		cell.bodyErr = "evaluating the returned expression: "
		cell.resultWhat = "result"
		return ret.Value, ret.Scope, nil
	}
	return nil, nil, ineligible(fmt.Sprintf("body of %d statements", len(shape.Steps)))
}

// redeclaredParameter finds an input parameter more than one calc of the
// specialization chain declares, whose declaration is a merge of them.
func (c *calcCompiler) redeclaredParameter() (string, bool) {
	declared := map[string]bool{}
	for _, link := range c.ctx.calcChain(c.shape.Sym) {
		for _, member := range declMembers(link.Decl) {
			usage, ok := member.(*ast.Usage)
			if !ok || (usage.Direction != ast.DirIn && usage.Direction != ast.DirInOut) {
				continue
			}
			name, _ := ast.EffectiveName(usage)
			if name == "" {
				continue
			}
			if declared[name] {
				return name, true
			}
			declared[name] = true
		}
	}
	return "", false
}

// scalarCheckFor decides a declaration for scalars, declining a declared type
// the scalar lattice does not place. A declaration stating no type, or one that
// does not resolve, holds any scalar, as it does on the evaluator; a non-scalar
// argument never reaches the compiled tier.
func (c *calcCompiler) scalarCheckFor(decl *calcMemberDecl) (scalarCheck, bool) {
	check := scalarCheck{decl: decl, countOK: true,
		boolOK: true, naturalOK: true, integerOK: true, rationalOK: true, realOK: true}
	if decl.Target == nil {
		return check, true
	}
	one := Value{Kind: ValConst}
	check.countOK = c.ctx.writeCountRefusal(decl.Target, &one) == ""
	if decl.Target.typ == nil {
		return check, true
	}
	prim := c.ctx.model.PrimTypeOf(decl.Target.typ)
	if prim == semantics.PrimUnknown {
		return scalarCheck{}, false
	}
	holds := func(from semantics.PrimType) bool { return semantics.PrimConforms(from, prim) }
	check.boolOK = holds(semantics.PrimBoolean)
	check.naturalOK = holds(semantics.PrimNatural)
	check.integerOK = holds(semantics.PrimInteger)
	check.rationalOK = holds(semantics.PrimRational)
	check.realOK = holds(semantics.PrimReal)
	return check, true
}

// cnode is one compiled expression node. The evaluator charges a step per node
// as it reaches it; the compiled form charges the steps up to a subtree's first
// fallible operation at once, which leaves the counter identical at every point
// an error can be observed.
type cnode struct {
	// prefix is the steps charged before the subtree's first operation that can
	// fail; infallible says no operation in it can, so prefix is the whole.
	prefix     int64
	infallible bool
	// emit builds the closure; precharged says the node above charged prefix.
	emit func(precharged bool) compiledExpr
	// A leaf reads a parameter slot or yields a constant.
	leaf     bool
	isConst  bool
	slot     int
	constant scalar
}

// expr is the node as a standalone expression charging its own steps.
func (n *cnode) expr() compiledExpr { return n.emit(false) }

// compileNode compiles n written in scope, where the parameters below nParams
// are bound; a name that is not one of them makes the body ineligible.
func (c *calcCompiler) compileNode(n ast.Node, scope *symbols.Scope, nParams int) (*cnode, error) {
	switch e := n.(type) {
	case *ast.LiteralInteger:
		v, err := strconv.ParseInt(e.Value, 10, 64)
		if err != nil {
			return nil, ineligible(fmt.Sprintf("integer literal %s outside the range", e.Value))
		}
		return constNode(intScalar(v)), nil
	case *ast.LiteralReal:
		v, err := strconv.ParseFloat(e.Value, 64)
		if err != nil {
			return nil, ineligible(fmt.Sprintf("real literal %s outside the range", e.Value))
		}
		return constNode(realScalar(v)), nil
	case *ast.LiteralBool:
		return constNode(boolScalar(e.Value)), nil
	case *ast.FeatureReference:
		return c.compileName(e.Name, nParams)
	case *ast.QualifiedName:
		return c.compileName(e, nParams)
	case *ast.OperatorExpr:
		return c.compileOperator(e, scope, nParams)
	case *ast.InvocationExpr:
		return c.compileInvocation(e, scope, nParams)
	default:
		return nil, ineligible(fmt.Sprintf("%T is outside the pure subset", n))
	}
}

// compileName resolves a name to a bound parameter's slot.
func (c *calcCompiler) compileName(qn *ast.QualifiedName, nParams int) (*cnode, error) {
	if qn == nil || len(qn.Parts) != 1 {
		return nil, ineligible(fmt.Sprintf("qualified name %s", qualifiedNameToString(qn)))
	}
	name := qn.Parts[0].Text
	for i := 0; i < nParams && i < len(c.shape.ParamNames); i++ {
		if c.shape.ParamNames[i] == name {
			return paramNode(i), nil
		}
	}
	return nil, ineligible(fmt.Sprintf("name %q is not a bound parameter", name))
}

// compileOperator compiles an operator application, folding it as the
// evaluator does before it looks at the operands.
func (c *calcCompiler) compileOperator(n *ast.OperatorExpr, scope *symbols.Scope, nParams int) (*cnode, error) {
	if folded, ok := c.ctx.model.Eval(n); ok {
		v, ok := scalarOfConst(folded)
		if !ok {
			return nil, ineligible("folds to a non-scalar constant")
		}
		return constNode(v), nil
	}
	switch n.Operator {
	case ast.OpConditional:
		if len(n.Operands) != 3 {
			return nil, ineligible(fmt.Sprintf("conditional with %d operands", len(n.Operands)))
		}
		return c.compileOperands(n, scope, nParams, conditionalNode)
	case ast.OpAdd, ast.OpSub, ast.OpMul, ast.OpDiv, ast.OpMod, ast.OpPow:
		if len(n.Operands) != 2 {
			return nil, ineligible(fmt.Sprintf("arithmetic with %d operands", len(n.Operands)))
		}
		return c.compileOperands(n, scope, nParams, arithmeticNode)
	case ast.OpEq, ast.OpNeq:
		if len(n.Operands) != 2 {
			return nil, ineligible(fmt.Sprintf("equality with %d operands", len(n.Operands)))
		}
		return c.compileOperands(n, scope, nParams, equalityNode)
	case ast.OpEqEqEq, ast.OpNeqEqEq:
		if len(n.Operands) != 2 {
			return nil, ineligible(fmt.Sprintf("identity with %d operands", len(n.Operands)))
		}
		return c.compileOperands(n, scope, nParams, identityNode)
	case ast.OpLt, ast.OpLe, ast.OpGt, ast.OpGe:
		if len(n.Operands) != 2 {
			return nil, ineligible(fmt.Sprintf("comparison with %d operands", len(n.Operands)))
		}
		return c.compileOperands(n, scope, nParams, comparisonNode)
	case ast.OpAnd, ast.OpConditionalAnd, ast.OpOr, ast.OpConditionalOr, ast.OpXor, ast.OpImplies:
		if len(n.Operands) != 2 {
			return nil, ineligible(fmt.Sprintf("logical operator with %d operands", len(n.Operands)))
		}
		return c.compileOperands(n, scope, nParams, logicalNode)
	case ast.OpNeg, ast.OpPos, ast.OpNot:
		if len(n.Operands) != 1 {
			return nil, ineligible(fmt.Sprintf("unary operator with %d operands", len(n.Operands)))
		}
		// The least Integer is read with its sign, as the evaluator reads it.
		if lit, ok := n.Operands[0].(*ast.LiteralInteger); ok && n.Operator == ast.OpNeg {
			if _, err := strconv.ParseInt(lit.Value, 10, 64); err != nil {
				if v, err := strconv.ParseInt("-"+lit.Value, 10, 64); err == nil {
					return constNode(intScalar(v)), nil
				}
			}
		}
		return c.compileOperands(n, scope, nParams, unaryNode)
	default:
		return nil, ineligible(fmt.Sprintf("operator '%s' is outside the pure subset", n.Operator))
	}
}

// compileOperands compiles n's operands in order and builds the node over them.
func (c *calcCompiler) compileOperands(n *ast.OperatorExpr, scope *symbols.Scope, nParams int,
	build func(op ast.OperatorKind, kids []*cnode) *cnode) (*cnode, error) {
	kids := make([]*cnode, len(n.Operands))
	for i, operand := range n.Operands {
		kid, err := c.compileNode(operand, scope, nParams)
		if err != nil {
			return nil, err
		}
		kids[i] = kid
	}
	return build(n.Operator, kids), nil
}

// compileInvocation compiles a call of another eligible calc, arguments by
// position, the callee's arity checked here rather than per call.
func (c *calcCompiler) compileInvocation(n *ast.InvocationExpr, scope *symbols.Scope, nParams int) (*cnode, error) {
	if n.Operand != nil || len(n.NamedArgs) > 0 || scope == nil {
		return nil, ineligible("invocation with a receiver or named arguments, or without a scope")
	}
	target := (&EvalContext{ctx: c.ctx, scope: scope}).invocationTarget(n)
	if target.shape == nil {
		return nil, ineligible(fmt.Sprintf("%s is not a calc with a shape", target.qualName))
	}
	callee := target.shape
	if callee.compileState == compileUndecided {
		c.batch.compile(callee)
	}
	if callee.compileState == compileIneligible {
		return nil, ineligible(fmt.Sprintf("callee %s is ineligible", callee.Name))
	}
	c.batch.call(c.shape, callee)
	if len(n.Args) > len(callee.Params) {
		return nil, ineligible(fmt.Sprintf("%s called with %d arguments for %d parameters", callee.Name, len(n.Args), len(callee.Params)))
	}
	for i := len(n.Args); i < len(callee.Params); i++ {
		if callee.Params[i].Default == nil {
			return nil, ineligible(fmt.Sprintf("%s called without an argument for %q", callee.Name, callee.Params[i].Name))
		}
	}
	args := make([]*cnode, len(n.Args))
	for i, arg := range n.Args {
		kid, err := c.compileNode(arg, scope, nParams)
		if err != nil {
			return nil, err
		}
		args[i] = kid
	}
	return invocationNode(callee.compiled, args), nil
}

// invoke runs the compiled body over the arguments pushed on the scalar stack
// above base, binding defaults for the parameters they leave unbound.
func (c *compiledCalc) invoke(ctx *Context, base int) (scalar, error) {
	if err := ctx.enterCalc(c.name); err != nil {
		return scalar{}, err
	}
	nargs := len(ctx.scalarStack) - base
	for i := range c.params {
		p := &c.params[i]
		var v scalar
		source := "argument"
		if i < nargs {
			v = ctx.scalarStack[base+i]
		} else {
			var err error
			if v, err = p.dflt(ctx, ctx.scalarStack[base:]); err != nil {
				ctx.leaveCalc()
				return scalar{}, fmt.Errorf("calc %s: default for parameter %q: %w", c.name, p.name, err)
			}
			source = "default"
			ctx.scalarStack = append(ctx.scalarStack, v)
		}
		if !p.check.accepts(v) {
			err := p.check.refuse(ctx, v, func() string {
				return fmt.Sprintf("calc %s: %s for parameter %q", c.name, source, p.name)
			})
			if err != nil {
				ctx.leaveCalc()
				return scalar{}, err
			}
		}
	}
	result, err := c.body(ctx, ctx.scalarStack[base:])
	ctx.leaveCalc()
	if err != nil {
		return scalar{}, calcFrame(c.name, fmt.Errorf("%s%w", c.bodyErr, err))
	}
	if c.result != nil && !c.result.accepts(result) {
		if err := c.result.refuse(ctx, result, func() string { return c.resultWhat }); err != nil {
			return scalar{}, calcFrame(c.name, err)
		}
	}
	return result, nil
}

// invokeBoxed is the tier's entry from the evaluator: it unboxes the positional
// arguments, runs the body and boxes the result. It declines, leaving the
// invocation to the evaluator, when an argument is not a scalar.
func (c *compiledCalc) invokeBoxed(ctx *Context, args []Value) (Value, bool, error) {
	if len(args) < c.minArgs || len(args) > len(c.params) {
		return Value{}, false, nil
	}
	base := len(ctx.scalarStack)
	for _, arg := range args {
		s, ok := scalarOf(arg)
		if !ok {
			ctx.scalarStack = ctx.scalarStack[:base]
			return Value{}, false, nil
		}
		ctx.scalarStack = append(ctx.scalarStack, s)
	}
	result, err := c.invoke(ctx, base)
	ctx.scalarStack = ctx.scalarStack[:base]
	if err != nil {
		return Value{}, true, err
	}
	return result.boxed(), true, nil
}
