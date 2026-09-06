package runtime

import (
	"errors"
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// subjectDecl is a case's subject as its body declares it: the object the case
// is about, which binds as the case's first input parameter (SysML v2 §7.21).
type subjectDecl struct {
	Name  string
	Value ast.Node
	Node  ast.Node
}

// subjectDeclaration reads a subject declaration, written as a subject member
// (`subject s : T;`, `subject = expr;`) or a subject-kind usage.
func subjectDeclaration(member ast.Node) (subjectDecl, bool) {
	switch m := member.(type) {
	case *ast.SubjectMember:
		name, _ := m.EffectiveName()
		return subjectDecl{Name: name, Value: m.BindingExpr, Node: m}, true
	case *ast.Usage:
		if m.Kind != ast.UsageSubject {
			return subjectDecl{}, false
		}
		name, _ := ast.EffectiveName(m)
		return subjectDecl{Name: name, Value: m.Value, Node: m}, true
	}
	return subjectDecl{}, false
}

// subjectParameter records link's subject among params: refining the subject an
// earlier link declared, else inserting it first, the position a subject binds by.
func (ctx *Context) subjectParameter(
	params []calcParameter, index map[string]int, aliases *map[string]string,
	link *symbols.Symbol, member ast.Node, subject subjectDecl,
) []calcParameter {
	sym := memberSymbol(declScope(link), member)
	param := calcParameter{
		Name: subject.Name, Default: subject.Value, Owner: link, IsSubject: true,
		Decl: ctx.calcMemberDeclOf(link, sym, subject.Name),
	}
	at := -1
	for i := range params {
		if params[i].IsSubject {
			at = i
			break
		}
	}
	if at < 0 {
		if seen, ok := ctx.redeclaredIndex(index, sym, subject.Name); ok {
			at = seen
		}
	}
	if at >= 0 {
		// A redeclaration binding no value keeps the inherited binding; one
		// declaring no name keeps the inherited name.
		if param.Default == nil {
			param.Default, param.Owner = params[at].Default, params[at].Owner
		}
		if param.Name == "" {
			param.Name = params[at].Name
		}
		param.Decl = param.Decl.redeclaring(params[at].Decl)
		if params[at].Name != param.Name {
			*aliases = aliasRedefined(*aliases, params[at].Name, param.Name)
			delete(index, params[at].Name)
		}
		params[at] = param
		index[param.Name] = at
		return params
	}
	if param.Name == "" {
		param.Name = "subject"
	}
	for name, i := range index {
		index[name] = i + 1
	}
	index[param.Name] = 0
	return append([]calcParameter{param}, params...)
}

// subjectParameter is the parameter the case's subject binds to, if it declares one.
func (shape *calcShape) subjectParameter() (*calcParameter, bool) {
	for i := range shape.Params {
		if shape.Params[i].IsSubject {
			return &shape.Params[i], true
		}
	}
	return nil, false
}

// enclosingSubject is the subject of the case whose body declares shape's usage,
// which a nested case binding no subject of its own takes (SysML v2 §7.21.2):
// read from the environment of the evaluation reading the usage.
func (ctx *Context) enclosingSubject(shape *calcShape, enclosing *EvalContext) (Value, bool) {
	if enclosing == nil {
		return Value{}, false
	}
	owner := enclosingBehavior(shape.Sym)
	if owner == nil || !isCalcSymbol(owner) {
		return Value{}, false
	}
	outer, err := ctx.calcShapeOf(owner)
	if err != nil {
		return Value{}, false
	}
	subject, ok := outer.subjectParameter()
	if !ok {
		return Value{}, false
	}
	return enclosing.Lookup(subject.Name)
}

// enclosingBehavior is the behavior whose body, directly or through body-local
// blocks, declares sym; nil for a member of a part or a package.
func enclosingBehavior(sym *symbols.Symbol) *symbols.Symbol {
	if sym == nil {
		return nil
	}
	for scope := sym.OwnerScope; scope != nil; scope = scope.Parent() {
		if owner := scope.Owner(); owner != nil {
			return owner
		}
		if !scope.BodyLocal() {
			return nil
		}
	}
	return nil
}

// unboundSubject reports a case run with no object as its subject.
func (shape *calcShape) unboundSubject(param *calcParameter) error {
	return &UnboundSubjectError{Kind: shape.Kind, Element: shape.Name, Subject: param.Name}
}

// AnalysisArgs are the values a run of an analysis case supplies: an object as
// its subject, and arguments for its input parameters by position or by name.
type AnalysisArgs struct {
	// Subject is the object the case is run on; nil leaves the case's own
	// subject binding, or the enclosing case's subject, to supply it.
	Subject *Instance

	// Positional bind the parameters the case does not bind itself, in
	// declaration order; the subject first when the case binds none and no
	// Subject is supplied.
	Positional []Value

	// Named bind parameters by name.
	Named map[string]Value
}

// AnalysisVerdict is what one check of an analysis case decided after its body
// ran: an objective's required conditions, or an assertion in its body.
type AnalysisVerdict struct {
	// Kind is "objective" or "assertion".
	Kind string

	// Name names the objective, or the asserted condition as written.
	Name string

	// Status is what the check decided.
	Status VerdictStatus

	// Detail is the violated condition of a failed check, or why an undecided
	// one could not be evaluated; empty for a satisfied one.
	Detail string
}

// VerdictStatus is what a check of a case decided.
type VerdictStatus int

const (
	// VerdictSatisfied is a check whose required conditions all held.
	VerdictSatisfied VerdictStatus = iota
	// VerdictNotSatisfied is a check with a required condition that did not hold.
	VerdictNotSatisfied
	// VerdictUndecided is a check that could not be evaluated.
	VerdictUndecided
)

// String names the status as a report words it.
func (s VerdictStatus) String() string {
	switch s {
	case VerdictSatisfied:
		return "satisfied"
	case VerdictNotSatisfied:
		return "not satisfied"
	default:
		return "undecided"
	}
}

// AnalysisResult is what one run of an analysis case produced: its output
// values in declaration order, and the verdict of each objective and assertion.
type AnalysisResult struct {
	// Case is the qualified name of the case that ran.
	Case string

	// Outputs are the case's out and return parameters, in declaration order;
	// a value the body returned into an unnamed result is named "result".
	Outputs []CalcOutputValue

	// Verdicts are the case's objectives, in order, then its assertions.
	Verdicts []AnalysisVerdict
}

// RunAnalysis runs an analysis case — a definition or a usage — binding its
// subject and input parameters from args and its own declarations, and reports
// every output it declares together with the verdict of each objective and
// assertion. self, when non-null, is the object a usage is a feature of.
// A usage run with no arguments answers from the same evaluation a read of
// its outputs does, so both report the same values.
func (ctx *Context) RunAnalysis(sym *symbols.Symbol, args AnalysisArgs, scope *symbols.Scope, self *Instance) (AnalysisResult, error) {
	defer ctx.beginRun()()

	if err := ctx.RequireAnalysisCase(sym); err != nil {
		return AnalysisResult{}, err
	}
	if err := ctx.checkCalcTyping(sym); err != nil {
		return AnalysisResult{}, err
	}
	shape, err := ctx.calcShapeOf(sym)
	if err != nil {
		return AnalysisResult{}, err
	}
	reader := NewEvalContextIn(ctx, scope, self)

	var run *calcRun
	if args.Subject == nil && len(args.Positional) == 0 && len(args.Named) == 0 && isCalcUsageSymbol(sym) {
		run, err = ctx.calcUsageRun(reader, sym)
	} else {
		run, err = ctx.analysisRun(shape, reader, args)
	}
	if err != nil {
		return AnalysisResult{}, err
	}

	result := AnalysisResult{Case: shape.Name}
	if result.Outputs, err = run.outputValues(ctx); err != nil {
		return AnalysisResult{}, err
	}
	result.Verdicts = ctx.analysisVerdicts(run, sym, scope)
	return result, nil
}

// analysisRun binds a case's parameters from args and runs its body once,
// unmemoized: arguments make it an invocation of its own, not the evaluation
// the case's outputs answer from when read as features.
func (ctx *Context) analysisRun(shape *calcShape, reader *EvalContext, args AnalysisArgs) (*calcRun, error) {
	if err := ctx.enterCalc(shape.Name); err != nil {
		return nil, err
	}
	defer ctx.leaveCalc()

	calcArgs, err := shape.analysisArgs(args)
	if err != nil {
		return nil, err
	}
	key := calcUsageKey{sym: shape.Sym}
	if reader.self != nil {
		key.instance = reader.self.ID
	}
	leave, err := ctx.enterCalcUsage(shape, key)
	if err != nil {
		return nil, err
	}
	defer leave()
	ec, nested, env, err := ctx.bindCalcUsage(shape, reader, calcArgs)
	if err != nil {
		return nil, err
	}
	return ctx.runCalcUsage(shape, ec, nested, env, reader)
}

// analysisArgs spells the run's arguments as bindings by parameter name: the
// supplied subject binds the subject parameter, and positional arguments bind
// the remaining parameters in declaration order — a subject the case binds
// itself excepted, a defaulted input included, as a calc invocation binds them.
func (shape *calcShape) analysisArgs(args AnalysisArgs) (calcArgs, error) {
	named := make(map[string]Value, len(args.Named)+len(args.Positional)+1)
	for name, value := range args.Named {
		if !shape.hasParameter(name) {
			return calcArgs{}, fmt.Errorf("%w: %s has no input parameter %q", ErrUnknownParameter, shape.Label, name)
		}
		named[name] = value
	}
	if args.Subject != nil {
		subject, ok := shape.subjectParameter()
		if !ok {
			return calcArgs{}, fmt.Errorf("%w: %s declares no subject to bind an object to",
				ErrUnknownParameter, shape.Label)
		}
		named[subject.Name] = Value{Kind: ValInstance, Instance: args.Subject.ID}
	}
	open := make([]*calcParameter, 0, len(shape.Params))
	for i := range shape.Params {
		param := &shape.Params[i]
		if _, bound := named[param.Name]; bound || (param.IsSubject && param.Default != nil) {
			continue
		}
		open = append(open, param)
	}
	if len(args.Positional) > len(open) {
		return calcArgs{}, fmt.Errorf("%w: %s takes %d argument(s), got %d",
			ErrCalcArity, shape.Label, len(open), len(args.Positional))
	}
	for i, value := range args.Positional {
		named[open[i].Name] = value
	}
	return calcArgs{named: named}, nil
}

// outputValues evaluates every output the case declares, in declaration order,
// naming a value the body returned into an unnamed result "result".
func (run *calcRun) outputValues(ctx *Context) ([]CalcOutputValue, error) {
	values := make([]CalcOutputValue, 0, len(run.shape.Outputs)+1)
	for _, out := range run.shape.Outputs {
		if out.Name == "" {
			if run.returned {
				values = append(values, CalcOutputValue{Name: resultOutputName, Value: run.result})
			}
			continue
		}
		value, err := run.output(ctx, out.Name)
		if err != nil {
			return nil, err
		}
		values = append(values, CalcOutputValue{Name: out.Name, Value: value})
	}
	if len(run.shape.Outputs) == 0 && run.returned {
		values = append(values, CalcOutputValue{Name: resultOutputName, Value: run.result})
	}
	return values, nil
}

// analysisVerdicts checks the case's objectives and assertions against the
// values its run bound: its parameters, its locals and its outputs.
func (ctx *Context) analysisVerdicts(run *calcRun, sym *symbols.Symbol, scope *symbols.Scope) []AnalysisVerdict {
	bindings := run.bindings(ctx)
	var verdicts []AnalysisVerdict
	for _, obj := range ctx.ObjectivesOf(sym, scope) {
		name := obj.Name
		if name == "" {
			name = "objective"
		}
		check := conditionCheck{
			sym: obj.Symbol, kind: "objective", what: "require condition",
			element: name, self: run.self, bindings: bindings,
		}
		verdicts = append(verdicts, ctx.analysisVerdict("objective", name, check, obj.Conditions))
	}
	for _, cond := range ctx.CaseConditionsOf(sym, scope) {
		if !cond.Required {
			continue
		}
		check := conditionCheck{
			sym: sym, kind: run.shape.Kind, what: "assertion",
			element: run.shape.Name, self: run.self, bindings: bindings,
		}
		verdicts = append(verdicts, ctx.analysisVerdict("assertion", cond.Label(), check, []Condition{cond}))
	}
	return verdicts
}

// analysisVerdict evaluates one check's conditions and words what it decided.
func (ctx *Context) analysisVerdict(kind, name string, check conditionCheck, conds []Condition) AnalysisVerdict {
	verdict := AnalysisVerdict{Kind: kind, Name: name}
	if len(conds) == 0 {
		verdict.Status, verdict.Detail = VerdictUndecided, "states no condition to check"
		return verdict
	}
	holds, err := ctx.evaluateConditions(check, conds)
	var violation *ViolationError
	switch {
	case err == nil && holds:
		verdict.Status = VerdictSatisfied
	case errors.As(err, &violation):
		verdict.Status, verdict.Detail = VerdictNotSatisfied, violation.Condition
	case err != nil:
		verdict.Status, verdict.Detail = VerdictUndecided, err.Error()
	default:
		verdict.Status = VerdictNotSatisfied
	}
	return verdict
}

// bindings are the values a run bound, by name: its parameters and locals, and
// the outputs it can evaluate, so a condition over the case reads them.
func (run *calcRun) bindings(ctx *Context) map[string]Value {
	bindings := make(map[string]Value, run.env.width()+len(run.shape.Outputs))
	run.env.each(func(name string, value Value) { bindings[name] = value })
	for _, out := range run.shape.Outputs {
		if out.Name == "" {
			continue
		}
		if value, err := run.output(ctx, out.Name); err == nil {
			bindings[out.Name] = value
		}
	}
	if run.returned {
		if out, err := run.shape.designatedOutput(); err != nil || out.Name == "" {
			bindings[resultOutputName] = run.result
		}
	}
	return bindings
}

// IsAnalysisSymbol reports whether sym declares an analysis case definition or usage.
func IsAnalysisSymbol(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	if sym.Decl != nil {
		return lower.PerformsSteps(sym.Decl)
	}
	return sym.Kind == symbols.SymbolAnalysisCaseDef || sym.Kind == symbols.SymbolAnalysisCaseUsage
}

// RequireAnalysisCase reports ErrNotAnAnalysis for a symbol that is not an analysis
// case definition or usage, describing what it is instead.
func (ctx *Context) RequireAnalysisCase(sym *symbols.Symbol) error {
	if sym == nil {
		return fmt.Errorf("%w: invalid symbol", ErrNotAnAnalysis)
	}
	if !IsAnalysisSymbol(sym) {
		return fmt.Errorf("%w: %s is %s, not an analysis case definition or usage",
			ErrNotAnAnalysis, ctx.qualifiedSymbolName(sym), describeDecl(sym.Decl))
	}
	return nil
}
