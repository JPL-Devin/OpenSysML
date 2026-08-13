package runtime

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/lower"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// A calc computes as many values as it declares output features, but an
// invocation of it yields exactly one result (KerML 7.4.9). Several outputs are
// therefore read from a calc usage rather than from an invocation: the usage's
// body runs once and each `out` feature is a value readable by name.
//
//	calc def Two { in n; out a = n + 1; out b = n * 2; }
//	calc c : Two { in n = 5; }
//	attribute z = c.b;   // 10
//
// This file evaluates such a usage: its inputs bind from its own member values
// and the defaults along its specialization chain, its computation runs through
// the same lowered statement engine an invocation uses, and its outputs are
// evaluated on demand against the environment that computation left behind.

// calcOutput is one output feature of a calc: an `out` parameter, or the result
// parameter a `return` declares. Its value is a feature of the calc, read by
// name, rather than a statement of the calc's body.
type calcOutput struct {
	Name     string          // feature name the output answers to, empty for an anonymous result parameter
	Value    ast.Node        // value-binding expression, nil when the calc gives the output no value
	Owner    *symbols.Symbol // the calc declaring the binding, whose scope it is written in
	IsResult bool            // declared with `return`: the result an invocation of the calc yields
}

// calcOutputs flattens the output features declared along chain (most general
// first). An output redeclared closer to the invoked calc keeps its inherited
// position and its inherited binding unless it binds a new one, as an input
// parameter does.
func calcOutputs(chain []*symbols.Symbol) []calcOutput {
	var outs []calcOutput
	index := make(map[string]int)

	for _, link := range chain {
		for _, member := range declMembers(link.Decl) {
			usage, ok := member.(*ast.Usage)
			if !ok {
				continue
			}
			if usage.Direction != ast.DirOut && !usage.IsResult {
				continue
			}
			// An output written as a redefinition names the one it overrides.
			name, _ := ast.EffectiveName(usage)
			out := calcOutput{Name: name, Value: usage.Value, Owner: link, IsResult: usage.IsResult}

			var at int
			var seen bool
			if name != "" {
				at, seen = index[name]
			} else {
				// An anonymous result parameter has no name to redeclare by, so
				// it refines the anonymous result it inherits.
				at, seen = indexOfAnonymousResult(outs)
			}
			if seen {
				if out.Value == nil {
					out.Value = outs[at].Value
					out.Owner = outs[at].Owner
				}
				outs[at] = out
				continue
			}
			if name != "" {
				index[name] = len(outs)
			}
			outs = append(outs, out)
		}
	}
	return outs
}

// indexOfAnonymousResult finds the unnamed result parameter among outs, which is
// the one an unnamed redeclaration refines.
func indexOfAnonymousResult(outs []calcOutput) (int, bool) {
	for i, out := range outs {
		if out.Name == "" {
			return i, true
		}
	}
	return 0, false
}

// calcSteps is the computation an invoked calc runs: its lowered body without
// the value bindings of its `out` features. An `out` binding states what that
// feature is, not a step of the body, so a calc declaring several of them
// returns none of them by falling off the end of its body.
func calcSteps(body []lower.Statement) []lower.Statement {
	steps := make([]lower.Statement, 0, len(body))
	for _, stmt := range body {
		if ret, ok := stmt.(lower.Return); ok && isOutputBinding(ret.Node) {
			continue
		}
		steps = append(steps, stmt)
	}
	return steps
}

// isOutputBinding reports whether a lowered return states an `out` feature's
// value rather than a `return`. A result parameter stays a return: it is the
// one value the calc designates.
func isOutputBinding(node ast.Node) bool {
	usage, ok := node.(*ast.Usage)
	return ok && usage.Direction == ast.DirOut && !usage.IsResult
}

// output finds the output feature of that name.
func (shape *calcShape) output(name string) (calcOutput, bool) {
	if name == "" {
		return calcOutput{}, false
	}
	for _, out := range shape.Outputs {
		if out.Name == name {
			return out, true
		}
	}
	return calcOutput{}, false
}

// outputNames renders the output features the calc declares, for a diagnostic
// about one it does not.
func (shape *calcShape) outputNames() string {
	names := make([]string, 0, len(shape.Outputs))
	for _, out := range shape.Outputs {
		if out.Name != "" {
			names = append(names, out.Name)
		}
	}
	if len(names) == 0 {
		return "none"
	}
	return strings.Join(names, ", ")
}

// designatedOutput returns the output feature an invocation of the calc yields
// as its result: the one declared with `return`, or, failing that, the only
// output the calc binds a value to. A calc binding several of them designates
// none, so its values are read from a calc usage's output features instead of
// being narrowed to whichever output comes first.
func (shape *calcShape) designatedOutput() (calcOutput, error) {
	var valued []calcOutput
	for _, out := range shape.Outputs {
		if out.IsResult {
			if out.Value == nil {
				break
			}
			return out, nil
		}
		if out.Value != nil {
			valued = append(valued, out)
		}
	}

	switch len(valued) {
	case 0:
		// The calc computed nothing to hand back: a body that fell off its end,
		// or outputs none of which states a value.
		return calcOutput{}, fmt.Errorf("%w: calc %s ended without a return", ErrCalcNoReturn, shape.Name)
	case 1:
		return valued[0], nil
	default:
		names := make([]string, 0, len(valued))
		for _, out := range valued {
			names = append(names, out.Name)
		}
		return calcOutput{}, fmt.Errorf(
			"%w: calc %s computes %d output features (%s) and designates no result; read them from a calc usage instead: %s",
			ErrAmbiguousResult, shape.Name, len(valued), strings.Join(names, ", "), shape.usageSpelling(valued[0].Name),
		)
	}
}

// usageSpelling writes the calc usage a modeler declares to read the outputs of
// a calc that designates no result, with the inputs it has to bind.
func (shape *calcShape) usageSpelling(output string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "calc c : %s {", shape.Name)
	for _, param := range shape.Params {
		fmt.Fprintf(&b, " in %s = ...;", param.Name)
	}
	fmt.Fprintf(&b, " } then read c.%s", output)
	return b.String()
}

// calcUsageKey identifies one evaluation of one calc usage within an activation:
// the usage and the object whose state it reads. The bound input values are not
// part of it, so a read after an assignment to a feature an input names cannot
// key a second evaluation and observe a different binding.
type calcUsageKey struct {
	sym      *symbols.Symbol
	instance int64
}

// calcRun is one evaluation of a calc: the environment its computation left
// behind and the outputs read from it so far. The computation runs once, when
// the run is created, so reading several outputs of a calc usage does not run
// its body again.
type calcRun struct {
	shape *calcShape
	scope *symbols.Scope // scope of whoever reads the usage, for a calc that owns none
	self  *Instance      // object the usage is a feature of, nil when unbound
	env   map[string]Value
	// outer is the environment of the evaluation reading the usage, for a usage
	// nested in a calc: the bindings the usage itself declares are written there.
	outer  *EvalContext
	result Value
	// returned reports whether the computation returned a value, which a calc
	// whose outputs are all features does not.
	returned bool
	outputs  map[string]Value
	// computing holds the outputs whose bindings are being evaluated, so an
	// output written in terms of itself is reported as a cycle rather than
	// evaluated until the step budget runs out.
	computing map[string]bool
	// onStack reports whether this evaluation already holds a nesting slot, so
	// the outputs of it that name each other do not count a level apiece.
	onStack bool
	// activation is the execution this run is, so a usage its outputs read is
	// evaluated once for the whole run rather than once per output.
	activation int64
}

// newCalcRun holds the environment one evaluation of a calc computed.
func newCalcRun(shape *calcShape, scope *symbols.Scope, self *Instance, env map[string]Value) *calcRun {
	return &calcRun{
		shape:     shape,
		scope:     scope,
		self:      self,
		env:       env,
		outputs:   make(map[string]Value),
		computing: make(map[string]bool),
	}
}

// CalcOutputValue is the value one output feature of a calc usage took, in the
// order the calc declares its outputs.
type CalcOutputValue struct {
	Name  string
	Value Value
}

// CalcUsageOutput evaluates a calc usage and returns the value of one of its
// output features. The usage's inputs bind from its own member values, falling
// back to the defaults declared along its specialization chain; self, when
// non-null, is the object the usage is a feature of, whose slots the inputs may
// name. Reading several outputs of the same usage runs its body once.
func (ctx *Context) CalcUsageOutput(sym *symbols.Symbol, name string, scope *symbols.Scope, self *Instance) (Value, error) {
	defer ctx.beginRun()()

	run, err := ctx.calcUsageRun(NewEvalContextIn(ctx, scope, self), sym)
	if err != nil {
		return Value{}, err
	}
	return run.output(ctx, name)
}

// CalcUsageOutputs evaluates a calc usage and returns the value of every output
// feature it declares, in declaration order, from one run of its body.
func (ctx *Context) CalcUsageOutputs(sym *symbols.Symbol, scope *symbols.Scope, self *Instance) ([]CalcOutputValue, error) {
	defer ctx.beginRun()()

	run, err := ctx.calcUsageRun(NewEvalContextIn(ctx, scope, self), sym)
	if err != nil {
		return nil, err
	}
	values := make([]CalcOutputValue, 0, len(run.shape.Outputs))
	for _, out := range run.shape.Outputs {
		if out.Name == "" {
			continue
		}
		value, err := run.output(ctx, out.Name)
		if err != nil {
			return nil, err
		}
		values = append(values, CalcOutputValue{Name: out.Name, Value: value})
	}
	return values, nil
}

// calcUsageRun returns the evaluation of a calc usage that the values read from
// it come from, running its body the first time it is needed. reader is the
// evaluation reading the usage, whose environment the usage's own input
// bindings are evaluated in. The run is memoized per usage, object and reading
// activation, so every output read of one usage within one activation answers
// from one execution of its body, while another activation gets its own.
func (ctx *Context) calcUsageRun(reader *EvalContext, sym *symbols.Symbol) (*calcRun, error) {
	if !isCalcUsageSymbol(sym) {
		return nil, fmt.Errorf(
			"%w: %s does not evaluate output features", ErrNotACalcUsage, ctx.qualifiedSymbolName(sym),
		)
	}
	if err := ctx.checkCalcTyping(sym); err != nil {
		return nil, err
	}
	shape, err := ctx.calcShapeOf(sym)
	if err != nil {
		return nil, err
	}

	leave, err := ctx.enterCalc(shape.Name)
	if err != nil {
		return nil, err
	}
	defer leave()

	// The evaluation is looked up before the inputs are bound, so a usage already
	// evaluated in this activation is not re-bound: its outputs all answer from
	// the binding its first read established.
	key := calcUsageKey{sym: sym}
	if reader.self != nil {
		key.instance = reader.self.ID
	}
	if run, ok := ctx.calcUsageRuns[reader.activation][key]; ok {
		if ctx.trace != nil {
			ctx.trace.RecordCalcUsageReuse(shape.Name)
		}
		return run, nil
	}

	ec, nested, env, err := ctx.bindCalcUsage(shape, reader)
	if err != nil {
		return nil, err
	}
	run, err := ctx.runCalcUsage(shape, ec, nested, env, reader)
	if err != nil {
		return nil, err
	}
	runs, ok := ctx.calcUsageRuns[reader.activation]
	if !ok {
		runs = make(map[calcUsageKey]*calcRun)
		ctx.calcUsageRuns[reader.activation] = runs
	}
	runs[key] = run
	return run, nil
}

// bodyUsageSymbol resolves the usage a body-local declaration declares, in the
// scope the declaration was written in.
func (ctx *Context) bodyUsageSymbol(stmt lower.DeclareUsage) (*symbols.Symbol, error) {
	if ctx.resolver == nil {
		return nil, fmt.Errorf("%w: calc usage %s needs a resolved model", ErrNotACalcUsage, stmt.Name)
	}
	sym, ok := ctx.resolver.LookupName(stmt.Scope, stmt.Name)
	if !ok || !isCalcUsageSymbol(sym) {
		return nil, fmt.Errorf(
			"%w: calc usage %s is declared in this body but is not resolved to one",
			ErrNotACalcUsage, stmt.Name,
		)
	}
	if err := ctx.checkCalcTyping(sym); err != nil {
		return nil, err
	}
	return sym, nil
}

// forgetCalcUsage drops the evaluation of one calc usage an activation holds, so
// the next read of it in that activation evaluates its body again.
func (ctx *Context) forgetCalcUsage(activation int64, sym *symbols.Symbol) {
	runs, ok := ctx.calcUsageRuns[activation]
	if !ok {
		return
	}
	for key, run := range runs {
		if key.sym == sym {
			ctx.endActivation(run.activation)
			delete(runs, key)
		}
	}
}

// bindCalcUsage binds a calc usage's inputs, answering with the environment the
// usage's body runs in, the environment reading it (null unless it is nested in
// a calc), and the bindings themselves.
func (ctx *Context) bindCalcUsage(shape *calcShape, reader *EvalContext) (*EvalContext, *EvalContext, map[string]Value, error) {
	ec := NewEvalContextIn(ctx, ctx.calcScope(shape.BodyOwner, shape.Sym, reader.scope), reader.self)
	if ec.trace != nil {
		ec.trace.RecordCalcEnter(shape.Name)
	}

	env := make(map[string]Value, len(shape.Params))
	ec.Push(env)

	// A usage declared among a calc's members is written in that calc's body, so
	// its own bindings see the parameters and locals of the evaluation reading it,
	// and none of the inputs being bound here, so every input resolves names in
	// the enclosing environment alike.
	var nested *EvalContext
	if enclosedByCalc(shape.Sym) {
		nested = reader.nestedEnv(ctx.calcScope(shape.Sym, shape.Sym, reader.scope))
	}

	// A usage passes no arguments: every input binds from the value the usage or
	// its definition declares for it.
	if err := ctx.bindCalcParameters(shape, ec, calcArgs{}, reader.scope, env, nested); err != nil {
		if ec.trace != nil {
			ec.trace.RecordCalcExitError(shape.Name, err)
		}
		return nil, nil, nil, err
	}
	return ec, nested, env, nil
}

// enclosedByCalc reports whether sym is declared in a calc's body — among its
// members, or in a body-local block of it, which declares no owner of its own —
// rather than in a part or a package.
func enclosedByCalc(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	for scope := sym.OwnerScope; scope != nil; scope = scope.Parent() {
		if owner := scope.Owner(); owner != nil {
			return isCalcSymbol(owner)
		}
		if !scope.BodyLocal() {
			return false
		}
	}
	return false
}

// checkCalcTyping rejects a calc usage typed by something that is not a calc: it
// inherits no parameters, no outputs and no body from it, so reading an output
// of it would report a missing feature rather than the specialization error.
func (ctx *Context) checkCalcTyping(sym *symbols.Symbol) error {
	for _, super := range ctx.model.DirectSupertypes(sym) {
		if super == nil || isCalcSymbol(super) {
			continue
		}
		return fmt.Errorf(
			"%w: calc usage %s specializes %s, which is not a calc",
			ErrNotACalc, ctx.qualifiedSymbolName(sym), ctx.qualifiedSymbolName(super),
		)
	}
	return nil
}

// runCalcUsage runs a calc usage's computation once over its bound inputs,
// keeping the environment the computation ends with so the usage's outputs can
// be evaluated against it.
func (ctx *Context) runCalcUsage(
	shape *calcShape, ec, nested *EvalContext, env map[string]Value, reader *EvalContext,
) (*calcRun, error) {
	result, returned, activation, err := ctx.runCalcSteps(shape, env)
	if err != nil {
		if ec.trace != nil {
			ec.trace.RecordCalcExitError(shape.Name, err)
		}
		return nil, fmt.Errorf("calc %s: %w", shape.Name, err)
	}
	if ec.trace != nil {
		if returned {
			ec.trace.RecordCalcExit(shape.Name, result)
		} else {
			ec.trace.RecordCalcUsageExit(shape.Name)
		}
	}

	run := newCalcRun(shape, reader.scope, reader.self, env)
	run.outer, run.result, run.returned = nested, result, returned
	run.activation = activation
	// A computation that returned has produced the calc's designated output, so
	// reading that output by name answers from the run rather than evaluating
	// the same expression again.
	if returned {
		if out, err := shape.designatedOutput(); err == nil && out.Name != "" {
			run.outputs[out.Name] = result
		}
	}
	return run, nil
}

// output returns the value of one output feature of this evaluation, evaluating
// its binding the first time it is read.
func (run *calcRun) output(ctx *Context, name string) (Value, error) {
	if value, ok := run.outputs[name]; ok {
		return value, nil
	}
	out, ok := run.shape.output(name)
	if !ok {
		return Value{}, fmt.Errorf(
			"%w: calc %s declares no output %s (it declares: %s)",
			ErrUnknownOutput, run.shape.Name, name, run.shape.outputNames(),
		)
	}
	value, err := run.value(ctx, out)
	if err != nil {
		return Value{}, err
	}
	if ctx.trace != nil {
		ctx.trace.RecordCalcOutput(run.shape.Name, name, value)
	}
	return value, nil
}

// value evaluates one output feature's binding in the scope of the calc that
// declares it, with the calc's parameters and locals in scope, and memoizes it
// for this evaluation. An output that names another output has it evaluated on
// demand; one that names itself, directly or through others, is a cycle.
func (run *calcRun) value(ctx *Context, out calcOutput) (Value, error) {
	if value, ok := run.outputs[out.Name]; ok && out.Name != "" {
		return value, nil
	}
	if out.Value == nil {
		return Value{}, fmt.Errorf(
			"%w for output %s of calc %s", ErrNoValue, run.outputDescription(out), run.shape.Name,
		)
	}
	if run.computing[out.Name] {
		return Value{}, fmt.Errorf(
			"%w: output %s of calc %s depends on itself",
			ErrCyclicOutput, run.outputDescription(out), run.shape.Name,
		)
	}
	run.computing[out.Name] = true
	defer delete(run.computing, out.Name)

	// An output is evaluated after the run that produced it has left the stack,
	// so the chain of usages a binding reads through is counted here.
	leave, err := run.enter(ctx)
	if err != nil {
		return Value{}, err
	}
	defer leave()

	value, err := run.bindingEnv(ctx, out.Owner).Eval(out.Value)
	if err != nil {
		return Value{}, fmt.Errorf("calc %s: output %s: %w", run.shape.Name, run.outputDescription(out), err)
	}
	if out.Name != "" {
		run.outputs[out.Name] = value
	}
	return value, nil
}

// enter counts this evaluation onto the calc stack while one of its outputs is
// worked out, unless it is counted already — by the invocation running it, or by
// an output of the same run whose binding named this one.
func (run *calcRun) enter(ctx *Context) (func(), error) {
	if run.onStack {
		return func() {}, nil
	}
	leave, err := ctx.enterCalc(run.shape.Name)
	if err != nil {
		return nil, err
	}
	run.onStack = true
	return func() { run.onStack = false; leave() }, nil
}

// bindingEnv is the environment one of this run's bindings is evaluated in: the
// calc's own, over the environment reading the usage for a binding the usage
// itself declares, which is written in the enclosing calc's body.
func (run *calcRun) bindingEnv(ctx *Context, owner *symbols.Symbol) *EvalContext {
	scope := ctx.calcScope(owner, run.shape.Sym, run.scope)
	ec := NewEvalContextIn(ctx, scope, run.self)
	// A binding of the calc's own belongs to this run; one the usage declares is
	// written in the enclosing body and belongs to the activation reading it.
	ec.activation = run.activation
	if run.outer != nil && owner == run.shape.Sym {
		ec = run.outer.nestedEnv(scope)
	}
	ec.Push(run.env)
	ec.calcRun = run
	return ec
}

// outputDescription names an output for a diagnostic, saying what an anonymous
// result parameter is rather than naming it with an empty string.
func (run *calcRun) outputDescription(out calcOutput) string {
	if out.Name != "" {
		return out.Name
	}
	return "(result)"
}

// lookupOutput answers a name an output binding refers to when it names another
// output of the same calc, evaluating that output on demand. It reports false
// for every other name, which the enclosing environment answers.
func (run *calcRun) lookupOutput(ctx *Context, name string) (Value, bool, error) {
	if run == nil {
		return Value{}, false, nil
	}
	out, ok := run.shape.output(name)
	if !ok {
		return Value{}, false, nil
	}
	value, err := run.value(ctx, out)
	return value, true, err
}

// isCalcUsageSymbol reports whether sym declares a calc usage, the form that
// carries an evaluation whose outputs are features. A calc definition is a type:
// it is invoked, not read.
func isCalcUsageSymbol(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	if sym.Decl != nil {
		usage, ok := sym.Decl.(*ast.Usage)
		return ok && usage.Kind == ast.UsageCalc
	}
	return sym.Kind == symbols.SymbolCalcUsage
}

// evalCalcUsageMembers reads a name written after a calc usage: its first part
// is an output feature of the usage, evaluated from one run of the usage's body,
// and any further part is read through the object that output names.
func (ec *EvalContext) evalCalcUsageMembers(sym *symbols.Symbol, parts []ast.NameSegment) (Value, error) {
	if len(parts) == 0 {
		return Value{}, fmt.Errorf("empty member chain")
	}
	run, err := ec.ctx.calcUsageRun(ec, sym)
	if err != nil {
		return Value{}, err
	}
	value, err := run.output(ec.ctx, parts[0].Text)
	if err != nil {
		return Value{}, err
	}
	return ec.chainMemberValue(value, parts[1:], parts[0].Text)
}

// calcUsageOperand reports whether the operand of a feature chain names a calc
// usage, whose members are computed rather than declared values. A local binding
// of the same name is the value the expression names, so a frame masks the
// declaration.
func (ec *EvalContext) calcUsageOperand(operand ast.Node) (*symbols.Symbol, bool) {
	ref, ok := operand.(*ast.FeatureReference)
	if !ok || ref.Name == nil || len(ref.Name.Parts) == 0 || ec.ctx.resolver == nil {
		return nil, false
	}
	if len(ref.Name.Parts) == 1 {
		if _, bound := ec.Lookup(ref.Name.Parts[0].Text); bound {
			return nil, false
		}
	}
	sym, ok := ec.ctx.resolver.ResolveQualified(ec.scope, ref.Name)
	if !ok || !isCalcUsageSymbol(sym) {
		return nil, false
	}
	return sym, true
}

// occurrenceOperand reports whether the operand of a feature chain names one
// occurrence — a part or item — that no local binding, and no slot of the object
// being evaluated, already answers with.
func (ec *EvalContext) occurrenceOperand(operand ast.Node) (*symbols.Symbol, bool) {
	ref, ok := operand.(*ast.FeatureReference)
	if !ok || ref.Name == nil || len(ref.Name.Parts) == 0 || ec.ctx.resolver == nil {
		return nil, false
	}
	if len(ref.Name.Parts) == 1 {
		name := ref.Name.Parts[0].Text
		if _, bound := ec.Lookup(name); bound {
			return nil, false
		}
		if ec.self != nil {
			if _, carried := ec.self.Slots[name]; carried {
				return nil, false
			}
		}
	}
	sym, ok := ec.ctx.resolver.ResolveQualified(ec.scope, ref.Name)
	if !ok || !isOccurrenceUsage(sym) || !ec.ctx.occursOnce(sym) {
		return nil, false
	}
	return sym, true
}

// calcUsageMemberValue reads parts from a calc usage a part declares, running it
// against that object so its inputs read the object's slots. Naming the usage
// itself names no value: its outputs are what it computes.
func (ec *EvalContext) calcUsageMemberValue(sym *symbols.Symbol, self *Instance, parts []ast.NameSegment) (Value, error) {
	if len(parts) == 0 {
		return Value{}, fmt.Errorf(
			"%w: calc usage %s computes output features (%s); read one of them",
			ErrNoValue, sym.Name, ec.ctx.calcUsageOutputSummary(sym),
		)
	}
	scope := sym.OwnerScope
	if scope == nil {
		scope = ec.scope
	}
	return NewEvalContextIn(ec.ctx, scope, self).evalCalcUsageMembers(sym, parts)
}

// calcUsageOutputSummary describes what a calc usage computes, for a diagnostic
// about reading the usage itself rather than one of its outputs.
func (ctx *Context) calcUsageOutputSummary(sym *symbols.Symbol) string {
	names := make([]string, 0)
	for _, out := range calcOutputs(ctx.calcChain(sym)) {
		if out.Name != "" {
			names = append(names, out.Name)
		}
	}
	if len(names) == 0 {
		return "no output feature"
	}
	return strings.Join(names, ", ")
}
