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
			"%w: calc %s computes %d output features (%s) and designates no result; declare a calc usage of it and read the outputs as features",
			ErrAmbiguousResult, shape.Name, len(valued), strings.Join(names, ", "),
		)
	}
}

// calcUsageKey identifies one evaluation of one calc usage: the usage and the
// object whose state it reads, so two objects carrying the same usage do not
// share the values computed for either.
type calcUsageKey struct {
	sym      *symbols.Symbol
	instance int64
}

// calcRun is one evaluation of a calc: the environment its computation left
// behind and the outputs read from it so far. The computation runs once, when
// the run is created, so reading several outputs of a calc usage does not run
// its body again.
type calcRun struct {
	shape  *calcShape
	scope  *symbols.Scope // scope of whoever reads the usage, for a calc that owns none
	self   *Instance      // object the usage is a feature of, nil when unbound
	env    map[string]Value
	result Value
	// returned reports whether the computation returned a value, which a calc
	// whose outputs are all features does not.
	returned bool
	outputs  map[string]Value
	// computing holds the outputs whose bindings are being evaluated, so an
	// output written in terms of itself is reported as a cycle rather than
	// evaluated until the step budget runs out.
	computing map[string]bool
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

	run, err := ctx.calcUsageRun(sym, scope, self)
	if err != nil {
		return Value{}, err
	}
	return run.output(ctx, name)
}

// CalcUsageOutputs evaluates a calc usage and returns the value of every output
// feature it declares, in declaration order, from one run of its body.
func (ctx *Context) CalcUsageOutputs(sym *symbols.Symbol, scope *symbols.Scope, self *Instance) ([]CalcOutputValue, error) {
	defer ctx.beginRun()()

	run, err := ctx.calcUsageRun(sym, scope, self)
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
// it come from, running its body the first time it is needed. The run is
// memoized per usage and object for the current run of the context, so two
// outputs read from one usage answer from one execution while a later run
// recomputes them against whatever state it finds.
func (ctx *Context) calcUsageRun(sym *symbols.Symbol, scope *symbols.Scope, self *Instance) (*calcRun, error) {
	if !isCalcUsageSymbol(sym) {
		return nil, fmt.Errorf(
			"%w: %s does not evaluate output features", ErrNotACalcUsage, ctx.qualifiedSymbolName(sym),
		)
	}

	key := calcUsageKey{sym: sym}
	if self != nil {
		key.instance = self.ID
	}
	if run, ok := ctx.calcUsageRuns[key]; ok {
		return run, nil
	}

	if err := ctx.checkCalcTyping(sym); err != nil {
		return nil, err
	}
	shape, err := ctx.calcShapeOf(sym)
	if err != nil {
		return nil, err
	}
	run, err := ctx.runCalcUsage(shape, scope, self)
	if err != nil {
		return nil, err
	}
	ctx.calcUsageRuns[key] = run
	return run, nil
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

// runCalcUsage binds a calc usage's inputs and runs its computation once,
// keeping the environment the computation ends with so the usage's outputs can
// be evaluated against it.
func (ctx *Context) runCalcUsage(shape *calcShape, scope *symbols.Scope, self *Instance) (*calcRun, error) {
	if ctx.calcDepth >= maxCalcNestingDepth {
		return nil, fmt.Errorf(
			"%w: calc %s nested more than %d deep (recursive calc?)",
			ErrCalcRecursionLimit, shape.Name, maxCalcNestingDepth,
		)
	}
	ctx.calcDepth++
	defer func() { ctx.calcDepth-- }()

	ec := NewEvalContextIn(ctx, ctx.calcScope(shape.BodyOwner, shape.Sym, scope), self)
	if ec.trace != nil {
		ec.trace.RecordCalcEnter(shape.Name)
	}

	env := make(map[string]Value, len(shape.Params))
	ec.Push(env)

	// A usage passes no arguments: every input binds from the value the usage or
	// its definition declares for it.
	if err := ctx.bindCalcParameters(shape, ec, calcArgs{}, scope, env); err != nil {
		if ec.trace != nil {
			ec.trace.RecordCalcExitError(shape.Name, err)
		}
		return nil, err
	}

	result, returned, err := ctx.runCalcSteps(shape, env)
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

	run := newCalcRun(shape, scope, self, env)
	run.result, run.returned = result, returned
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

	ec := NewEvalContextIn(ctx, ctx.calcScope(out.Owner, run.shape.Sym, run.scope), run.self)
	ec.Push(run.env)
	ec.calcRun = run

	value, err := ec.Eval(out.Value)
	if err != nil {
		return Value{}, fmt.Errorf("calc %s: output %s: %w", run.shape.Name, run.outputDescription(out), err)
	}
	if out.Name != "" {
		run.outputs[out.Name] = value
	}
	return value, nil
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
	run, err := ec.ctx.calcUsageRun(sym, ec.scope, ec.self)
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
