package runtime

import (
	"errors"
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Context carries runtime execution state. One per workspace session.
type Context struct {
	model    *semantics.Model
	resolver *resolve.Resolver
	// ids hands out instance identities. Contexts holding the same objects share
	// one sequence, so no two of them name different objects alike.
	ids       *idSequence
	steps     int64
	maxSteps  int64
	instances map[int64]*Instance
	created   []int64

	// maxActionSteps, maxStateEvents and maxDoSteps bound the executors this
	// context runs: token-flow steps, dispatched events, and do actions.
	// Unlike maxSteps they are counted by the executor, not here.
	maxActionSteps int64
	maxStateEvents int64
	maxDoSteps     int64

	// elements and maxElements bound the collection elements one run materializes,
	// which is what its memory grows with, unlike a step.
	elements    int64
	maxElements int64

	features map[*symbols.Symbol][]EffectiveFeature

	// writeTargets memoizes the declaration an assignment's target names, per
	// scope the statement was written in: what a value written must conform to.
	writeTargets map[writeTargetKey]*writeTarget

	// calcShapes memoizes resolved calc invocation interfaces (parameters,
	// defaults, result expression) per calc symbol.
	calcShapes map[*symbols.Symbol]*calcShape

	// invocationTargets memoizes what each invocation expression denotes in the
	// scope it is evaluated in; the model does not change under one context.
	invocationTargets map[invocationKey]*invocationTarget

	// integerLiterals and realLiterals memoize the value each numeric literal
	// node spells, so a literal in a recursion is parsed once per context.
	integerLiterals map[*ast.LiteralInteger]int64
	realLiterals    map[*ast.LiteralReal]float64

	// calcUsageRuns holds the evaluation of each calc usage read in an activation
	// under way, so reading several outputs of one usage answers from one
	// execution of its body. An activation's evaluations end with it.
	calcUsageRuns map[int64]map[calcUsageKey]*calcRun

	// activations numbers the body activations begun in this context: a calc
	// invocation, a block entry, a loop iteration, a body application.
	activations int64

	// occurrences holds the object each usage carrying no value of its own
	// denotes, so a feature chain through a part reads one occurrence of it.
	occurrences map[*symbols.Symbol]int64

	// variantObjects holds the object a variant stands for per owner that
	// selected it, so repeated reads of one selection read the same object.
	variantObjects map[variantObject]int64

	// selectedVariants records, per owner and variation name, the variant bound
	// to it in this run. Routing consults it: a connection a `variant interface`
	// declares joins its ends only where that variant is the one selected.
	selectedVariants map[variantSelection]string

	// materializingConnectors holds the connectors whose ends are being attached,
	// so a connector reached from its own end is reported as a cycle.
	materializingConnectors map[connectorRef]bool

	// objectConns memoizes the connections declared by each type an object is
	// of, which a behavior that object performs routes over.
	objectConns map[*symbols.Symbol][]lower.Connection

	// objectBindings memoizes binding connectors declared by each materialized
	// object type, including bindings inherited from its supertypes.
	bindingIR map[*symbols.Symbol][]lower.Binding

	// resolvingBindings guards binding endpoint resolution for one instance
	// feature, so a valueless binding cycle is reported rather than recursed.
	resolvingBindings map[featureValueRef]bool
	bindingOwners     map[featureValueRef]*ast.Usage
	bindingFeatures   map[*symbols.Symbol]map[string][]lower.Binding

	// classifierBehaviors memoizes the behaviors each type binds to its objects:
	// the machines it exhibits and the actions it performs.
	classifierBehaviors map[*symbols.Symbol][]classifierBehaviorDecl

	// pendingBehaviors are the object behaviors attached but not yet run, drained
	// by the outermost materialization so a start reached from inside a running
	// behavior does not run it recursively.
	pendingBehaviors []*ObjectBehavior

	// behaviorRunDepth is the number of classifier-behavior starts under way.
	behaviorRunDepth int

	// objectBehaviors are every behavior an object of this context runs, so a
	// drain to quiescence can re-run one a sibling's send woke.
	objectBehaviors []*ObjectBehavior

	// trace records evaluation, nil when not tracing.
	trace *TraceRecorder

	// actionDepth is the number of action invocations currently on the stack,
	// bounding recursion across nested action executors.
	actionDepth int

	// calcDepth is the number of calc invocations currently on the stack, which
	// maxCalcDepth bounds, so a recursion evaluates while it stays within it.
	calcDepth    int
	maxCalcDepth int64

	// freeInvocationFrames are the frames of returned calc invocations, kept so a
	// recursion reuses storage rather than allocating per call.
	freeInvocationFrames []*invocationFrame

	// argStack holds the positional arguments of the calc invocations under way,
	// innermost last, so an invocation borrows rather than allocates its storage.
	argStack []Value

	// scalarStack holds the frames of the compiled calc invocations under way —
	// parameters, then body locals — innermost last, the compiled tier's
	// counterpart of argStack.
	scalarStack []scalar

	// libraryArgBuf holds the boxed arguments of the library call a compiled body
	// is making, and libraryEval the context a collection built-in it calls takes.
	libraryArgBuf []Value
	libraryEval   EvalContext

	// compileCalcs enables the compiled tier for eligible calc bodies; the
	// OPENSYSML_CALC_COMPILE escape hatch clears it.
	compileCalcs bool

	// runDepth is the number of runs currently under way, so the step counter is
	// reset per run rather than accumulated over the context's whole life.
	runDepth int

	// messages are the signals in flight, oldest first. The bus is context-wide,
	// so a message one behavior sends can be accepted in another.
	messages []Message

	// derivingFeatureValues holds the feature values whose defaults are being evaluated, so a
	// default that refers back to its own feature value is reported as a cycle.
	derivingFeatureValues map[featureValueRef]bool

	// collectingSubsets holds the feature values whose subsetting features are being read,
	// so features that subset each other are reported as a cycle.
	collectingSubsets map[featureValueRef]bool

	// sources holds the text of the files the model was read from, by name, so an
	// error about a declaration can say where it was written. A file no caller
	// registered is reported by name and byte offset instead.
	sources map[string]*source.SourceFile
}

// featureValueRef identifies one feature value of one instance.
type featureValueRef struct {
	instance int64
	feature  string
}

// variantSelection identifies a variation point of one object: two objects of a
// type each select their own variant of the same variation.
type variantSelection struct {
	owner     int64
	variation string
}

// connectorRef identifies one connector being materialized in the context of
// the object whose features its ends name.
type connectorRef struct {
	owner     int64
	connector *symbols.Symbol
}

// NewContext creates a runtime context backed by the given semantic model.
// maxSteps sets the runaway guard (step counter limit); the executor bounds take
// their defaults, which SetBudgets replaces.
// It panics if maxSteps <= 0: the limit is a programmer-supplied invariant, not
// user input, so callers must pass a positive value.
func NewContext(model *semantics.Model, resolver *resolve.Resolver, maxSteps int64) *Context {
	if maxSteps <= 0 {
		panic(fmt.Sprintf("runtime: maxSteps must be > 0, got %d", maxSteps))
	}
	return &Context{
		model:      model,
		resolver:   resolver,
		ids:        &idSequence{next: 1}, // IDs start at 1 (0 = invalid)
		steps:      0,
		maxSteps:   maxSteps,
		instances:  make(map[int64]*Instance),
		features:   make(map[*symbols.Symbol][]EffectiveFeature),
		calcShapes: make(map[*symbols.Symbol]*calcShape),

		invocationTargets: make(map[invocationKey]*invocationTarget),
		integerLiterals:   make(map[*ast.LiteralInteger]int64),
		realLiterals:      make(map[*ast.LiteralReal]float64),
		compileCalcs:      CalcCompileFromEnv(),

		calcUsageRuns: make(map[int64]map[calcUsageKey]*calcRun),

		maxActionSteps: DefaultMaxActionSteps,
		maxStateEvents: DefaultMaxStateEvents,
		maxDoSteps:     DefaultMaxDoSteps,
		maxElements:    DefaultMaxElements,
		maxCalcDepth:   DefaultMaxCalcDepth,

		occurrences:      make(map[*symbols.Symbol]int64),
		variantObjects:   make(map[variantObject]int64),
		selectedVariants: make(map[variantSelection]string),

		materializingConnectors: make(map[connectorRef]bool),
		objectConns:             make(map[*symbols.Symbol][]lower.Connection),
		bindingIR:               make(map[*symbols.Symbol][]lower.Binding),
		classifierBehaviors:     make(map[*symbols.Symbol][]classifierBehaviorDecl),
		derivingFeatureValues:   make(map[featureValueRef]bool),
		resolvingBindings:       make(map[featureValueRef]bool),
		bindingOwners:           make(map[featureValueRef]*ast.Usage),
		bindingFeatures:         make(map[*symbols.Symbol]map[string][]lower.Binding),
		collectingSubsets:       make(map[featureValueRef]bool),
		sources:                 make(map[string]*source.SourceFile),
	}
}

// RegisterSource gives the context the text of a file the model was read from,
// so an error about a declaration in it reports a line and column.
func (ctx *Context) RegisterSource(sf *source.SourceFile) {
	if sf == nil {
		return
	}
	ctx.sources[sf.Name()] = sf
}

// sourceLocation renders where a span in a file was written, as
// `file:line:col`. It falls back to a byte offset for a file whose text was not
// registered, and to the file name alone when there is no span, so a diagnostic
// always says as much as the context knows.
func (ctx *Context) sourceLocation(file string, span source.Span) string {
	if file == "" {
		return ""
	}
	sf, ok := ctx.sources[file]
	if !ok || span.End() > sf.Len() {
		if span.Len == 0 && span.Offset == 0 {
			return file
		}
		return fmt.Sprintf("%s:#%d", file, span.Offset)
	}
	pos := sf.Lines().PosAt(span.Offset)
	return fmt.Sprintf("%s:%d:%d", file, pos.Line, pos.Col)
}

// symbolLocation renders where a symbol was declared, empty for none.
func (ctx *Context) symbolLocation(sym *symbols.Symbol) string {
	if sym == nil {
		return ""
	}
	return ctx.sourceLocation(sym.DocName, sym.DeclSpan)
}

// SetTrace attaches a trace recorder to this context, so that every expression
// and calc evaluated through it is recorded. Pass nil to stop tracing.
func (ctx *Context) SetTrace(tr *TraceRecorder) {
	ctx.trace = tr
}

// Model returns the semantic model this context operates over.
func (ctx *Context) Model() *semantics.Model {
	return ctx.model
}

// Resolver returns the name resolver this context resolves references with.
func (ctx *Context) Resolver() *resolve.Resolver {
	return ctx.resolver
}

// SourceLocation renders where a span in a file was written, as `file:line:col`,
// falling back to a byte offset for a file whose text was not registered.
func (ctx *Context) SourceLocation(file string, span source.Span) string {
	return ctx.sourceLocation(file, span)
}

// idSequence hands out instance identities, one per object over the contexts
// sharing it.
type idSequence struct {
	next int64
}

func (s *idSequence) take() int64 {
	id := s.next
	s.next++
	return id
}

// atLeast raises the sequence to hand out id next, never lowering it.
func (s *idSequence) atLeast(id int64) {
	if id > s.next {
		s.next = id
	}
}

// allocateID returns the next instance ID and increments the counter.
func (ctx *Context) allocateID() int64 {
	return ctx.ids.take()
}

// beginRun starts a run and returns the function that ends it, resetting the
// step counter so the budget bounds one run rather than a whole session. A run
// started inside another - an action invoked from an expression, say - shares the
// outer one's budget, so a runaway cannot escape the bound by starting runs.
func (ctx *Context) beginRun() func() {
	if ctx.runDepth == 0 {
		ctx.steps = 0
		ctx.elements = 0
		ctx.calcUsageRuns = make(map[int64]map[calcUsageKey]*calcRun)
	}
	ctx.runDepth++
	return func() { ctx.runDepth-- }
}

// beginExecutorRun brackets one call into an executor a caller drives itself, step
// by step - the REPL's %action and %state debuggers - whose run spans many calls
// and so has no single scope beginRun could bracket. started, held by the
// executor, marks its run as begun, so the counter is reset once, at its start,
// and every call of it counts as a run under way.
func (ctx *Context) beginExecutorRun(started *bool) func() {
	if ctx.runDepth == 0 && !*started {
		ctx.steps = 0
		ctx.elements = 0
		ctx.calcUsageRuns = make(map[int64]map[calcUsageKey]*calcRun)
	}
	*started = true
	ctx.runDepth++
	return func() { ctx.runDepth-- }
}

// newActivation begins one activation: the identity of a single execution of a
// body, which the values a calc usage answers within it belong to.
func (ctx *Context) newActivation() int64 {
	ctx.activations++
	return ctx.activations
}

// endActivation forgets what an activation computed, once it has ended, and the
// activations of the calc usage evaluations it held.
func (ctx *Context) endActivation(activation int64) {
	runs, ok := ctx.calcUsageRuns[activation]
	if !ok {
		return
	}
	delete(ctx.calcUsageRuns, activation)
	for _, run := range runs {
		ctx.endActivation(run.activation)
	}
}

// incrementStep increments the step counter and returns ErrStepLimitExceeded if limit reached.
// The error names the effective budget and the variable that raises it.
func (ctx *Context) incrementStep() error {
	ctx.steps++
	if ctx.steps > ctx.maxSteps {
		return ctx.stepLimitExceeded()
	}
	return nil
}

// stepLimitExceeded reports the step budget spent, naming the variable that raises
// it; kept out of line so the step charge on every evaluation inlines.
//
//go:noinline
func (ctx *Context) stepLimitExceeded() error {
	return fmt.Errorf("%w (%d steps; raise %s to allow more)", ErrStepLimitExceeded, ctx.maxSteps, MaxStepsEnvVar)
}

// elementScope brackets one evaluation and returns the function releasing what
// it materialized, so the bound counts elements held at once, not in total.
func (ctx *Context) elementScope() func() {
	held := ctx.elements
	return func() { ctx.elements = held }
}

// beginStep brackets one evaluation outside a body: it answers the activation the
// evaluation runs in and the function ending it, releasing what it materialized.
func (ctx *Context) beginStep() (int64, func()) {
	activation := ctx.newActivation()
	release := ctx.elementScope()
	return activation, func() {
		ctx.endActivation(activation)
		release()
	}
}

// chargeElements counts elements an evaluation materializes, which unlike a step
// is memory the collection holding it keeps, against the element budget.
func (ctx *Context) chargeElements(n int64) error {
	ctx.elements += n
	// A count that overflowed is past any budget, so it reads as one.
	if ctx.elements > ctx.maxElements || ctx.elements < 0 {
		return fmt.Errorf("%w (%d elements; raise %s to allow more)", ErrElementLimitExceeded, ctx.maxElements, MaxElementsEnvVar)
	}
	return nil
}

// Instance retrieves an instance by ID, so a caller holding a ValInstance can
// reach the object it names.
func (ctx *Context) Instance(id int64) (*Instance, bool) {
	return ctx.getInstance(id)
}

// getInstance retrieves an instance by ID.
func (ctx *Context) getInstance(id int64) (*Instance, bool) {
	inst, ok := ctx.instances[id]
	return inst, ok
}

// registerInstance stores an instance in the registry.
func (ctx *Context) registerInstance(inst *Instance) {
	if inst.ID <= 0 {
		panic(fmt.Sprintf("runtime: invalid instance ID %d (must be > 0)", inst.ID))
	}
	if _, exists := ctx.instances[inst.ID]; exists {
		panic(fmt.Sprintf("runtime: duplicate instance ID %d", inst.ID))
	}
	ctx.instances[inst.ID] = inst
	ctx.created = append(ctx.created, inst.ID)
}

// EvaluateConstraint evaluates a constraint definition/usage naming no object:
// against the single object of this runtime carrying it, the declared defaults
// when there is none, ErrAmbiguousSubject when there are several.
// Returns (satisfied, error). If IsAssert=true, violation is an error.
// If IsAssert=false (assume), always returns (true, nil) but logs assumptions.
func (ctx *Context) EvaluateConstraint(sym *symbols.Symbol, scope *symbols.Scope) (bool, error) {
	return ctx.EvaluateConstraintOn(sym, scope, nil)
}

// RequireConstraint returns an ErrNotAConstraint usage error unless sym
// declares a constraint, so a caller can settle the kind before evaluating.
func RequireConstraint(sym *symbols.Symbol) error {
	if _, ok := ast.OwnedConstraintOf(sym.Decl); ok {
		return nil
	}
	switch decl := sym.Decl.(type) {
	case *ast.Definition:
		if decl.Kind == ast.DefConstraint {
			return nil
		}
	case *ast.Usage:
		if decl.Kind == ast.UsageConstraint {
			return nil
		}
	}
	return notOfKind(ErrNotAConstraint, sym, "constraint")
}

// RequireRequirement returns an ErrNotARequirement usage error unless sym
// declares a requirement.
func RequireRequirement(sym *symbols.Symbol) error {
	switch decl := sym.Decl.(type) {
	case *ast.Definition:
		if decl.Kind == ast.DefRequirement {
			return nil
		}
	case *ast.Usage:
		if decl.Kind == ast.UsageRequirement {
			return nil
		}
	}
	return notOfKind(ErrNotARequirement, sym, "requirement")
}

// EvaluateConstraintOn evaluates a constraint against a concrete instance: a
// feature the constraint names resolves to that instance's feature value, so the same
// constraint can pass for one instance and fail for another. An instance that
// does not carry the constraint itself is searched for the nested object that
// does; a nil instance leaves the subject to EvaluateConstraint's rule.
func (ctx *Context) EvaluateConstraintOn(sym *symbols.Symbol, scope *symbols.Scope, self *Instance) (bool, error) {
	result, err := ctx.CheckConstraintOn(sym, scope, self)
	return result.Holds, err
}

// CheckConstraintOn evaluates a constraint as EvaluateConstraintOn does and also
// reports the object it turned out to be about, which a caller labelling the
// verdict needs: it is not always the instance supplied.
func (ctx *Context) CheckConstraintOn(sym *symbols.Symbol, scope *symbols.Scope, self *Instance) (CheckResult, error) {
	defer ctx.beginRun()()

	if err := RequireConstraint(sym); err != nil {
		return CheckResult{Subject: self}, err
	}
	subject, err := ctx.checkSubject("constraint", sym.Name, sym, self)
	if err != nil {
		return CheckResult{}, err
	}

	// Evaluate every condition the constraint states, inherited ones included.
	conds := ctx.conditionsOf(sym, ctx.chainMembers(sym, scope))
	holds, err := ctx.evaluateConditions(conditionCheck{
		sym:     sym,
		kind:    "constraint",
		what:    "assertion",
		self:    subject.instance,
		negated: NegatedDecl(sym),
	}, conds)
	return ctx.checkResultOf(holds, subject), err
}

// CheckResult is the outcome of one check: whether it holds, the object its
// conditions were evaluated against — nil when they were evaluated against the
// declaration because no object carries the checked element — and, for a nested
// subject, the object the search started from plus the features walked from it —
// ending in the declaration the object materializes, as an ambiguity names it —
// which are how a caller names an object holding no name of its own.
type CheckResult struct {
	Holds       bool
	Subject     *Instance
	SubjectRoot *Instance
	SubjectPath string
}

// checkResultOf reports a verdict about the object a check resolved to.
func (ctx *Context) checkResultOf(holds bool, subject carrier) CheckResult {
	return CheckResult{
		Holds:       holds,
		Subject:     subject.instance,
		SubjectRoot: subject.root,
		SubjectPath: ctx.carrierFeatures(subject),
	}
}

// memberBindings evaluates the values members bind by name — a subject or actor
// supplied by an expression (`actor operator = limit;`) — so a condition naming
// one reads it. element names the requirement in messages. A non-nil subject is
// the object supplied from outside (the `by` of a satisfaction assertion): it
// binds every subject the members declare, whose own binding is then neither
// evaluated nor used.
func (ctx *Context) memberBindings(sym *symbols.Symbol, element string, members []scopedMember, self *Instance, subject *Instance) (map[string]Value, error) {
	bindings := make(map[string]Value)
	features := ctx.conditionFeatures(sym)
	// The bindings are evaluated as one, so a calc usage two of them read answers
	// from one evaluation, and the next check reads it again.
	activation, endStep := ctx.beginStep()
	defer endStep()
	evalIn := func(memberScope *symbols.Scope) *EvalContext {
		ec := NewEvalContextIn(ctx, memberScope, self)
		ec.activation = activation
		ec.features = features
		ec.Push(bindings)
		return ec
	}

	for _, member := range members {
		var what, name string
		var expr ast.Node
		isSubject := false
		switch rm := member.node.(type) {
		case *ast.SubjectMember:
			what, name, expr, isSubject = "subject", rm.Name, rm.BindingExpr, true
		case *ast.Usage:
			switch rm.Kind {
			case ast.UsageSubject:
				name, isSubject = effectiveName(rm), true
			case ast.UsageActor:
				what, name, expr = "actor", effectiveName(rm), rm.Value
			}
		default:
			continue
		}
		if isSubject && subject != nil {
			if name != "" {
				bindings[name] = Value{Kind: ValInstance, Instance: subject.ID}
			}
			continue
		}
		if expr == nil {
			continue
		}
		value, err := evalIn(member.scope).Eval(expr)
		if err != nil {
			return nil, fmt.Errorf("requirement %s: %s binding evaluation failed: %w", element, what, err)
		}
		bindings[name] = value
	}
	return bindings, nil
}

// effectiveName is the name a usage answers to, which for a member written as a
// reference is its reference's rather than a declared one (ast.EffectiveName).
func effectiveName(u *ast.Usage) string {
	name, _ := ast.EffectiveName(u)
	return name
}

// NegatedDecl reports whether sym's declaration asserts that its conditions do
// not hold (`assert not constraint { … }`, `assert not satisfy … by …`).
func NegatedDecl(sym *symbols.Symbol) bool {
	usage, ok := sym.Decl.(*ast.Usage)
	return ok && usage.IsNegated
}

// scopedMember is a declaration member with the scope it was written in, since
// an inherited member's names resolve where its supertype was declared.
type scopedMember struct {
	node  ast.Node
	scope *symbols.Scope
}

// chainMembers returns the members declared by sym's supertypes, most general
// first, followed by sym's own. A usage that takes its conditions from a
// definition (constraint limit : MassLimit) carries no members itself. A library
// supertype states the metamodel frame every element specializes rather than the
// model's own objectives, conditions or parameters, so it contributes none.
func (ctx *Context) chainMembers(sym *symbols.Symbol, scope *symbols.Scope) []scopedMember {
	var out []scopedMember
	supers := ctx.model.AllSupertypes(sym)
	replaced := ctx.replacedConstraintBodies(sym, supers)
	for i := len(supers) - 1; i >= 0; i-- {
		link := supers[i]
		if link == nil || ctx.libraryDeclared(link) || replaced[link] {
			continue
		}
		for _, node := range declMembers(link.Decl) {
			out = append(out, scopedMember{node: node, scope: bodyScope(link, link.OwnerScope)})
		}
	}
	for _, node := range declMembers(sym.Decl) {
		out = append(out, scopedMember{node: node, scope: bodyScope(sym, scope)})
	}
	return out
}

// replacedConstraintBodies returns the supertypes a constraint's chain leaves out:
// those a redefinition stating its own body replaces, and what only they reach.
func (ctx *Context) replacedConstraintBodies(sym *symbols.Symbol, supers []*symbols.Symbol) map[*symbols.Symbol]bool {
	if !isConstraintSymbol(sym) {
		return nil
	}
	replaced := map[*symbols.Symbol]bool{}
	for _, link := range append([]*symbols.Symbol{sym}, supers...) {
		if link == nil || !isConstraintSymbol(link) || !statesCondition(declMembers(link.Decl)) {
			continue
		}
		for _, redefined := range ctx.model.AllRedefinedFeatures(link) {
			replaced[redefined] = true
		}
	}
	if len(replaced) == 0 {
		return nil
	}
	kept := map[*symbols.Symbol]bool{}
	var keep func(*symbols.Symbol)
	keep = func(s *symbols.Symbol) {
		for _, direct := range ctx.model.DirectSupertypes(s) {
			if direct == nil || direct == sym || replaced[direct] || kept[direct] {
				continue
			}
			kept[direct] = true
			keep(direct)
		}
	}
	keep(sym)
	skipped := map[*symbols.Symbol]bool{}
	for _, link := range supers {
		if link != nil && !kept[link] {
			skipped[link] = true
		}
	}
	return skipped
}

// statesCondition reports whether any of the members states a condition.
func statesCondition(members []ast.Node) bool {
	for _, member := range members {
		switch member.(type) {
		case *ast.ConstraintMember, *ast.RequireMember, *ast.AssumeMember:
			return true
		}
	}
	return false
}

// bodyScope is the scope a member of sym's body was written in: sym's own body,
// where its sibling declarations answer a name before the enclosing namespace
// does (KerML 8.2.3.5.4). fallback covers a declaration that owns no scope.
func bodyScope(sym *symbols.Symbol, fallback *symbols.Scope) *symbols.Scope {
	if sym != nil && sym.Scope != nil {
		return sym.Scope
	}
	return fallback
}

// EvaluateRequirement evaluates a requirement definition/usage naming no object,
// choosing its subject as EvaluateConstraint does.
// Returns (satisfied, error). Validates subject/actor types and evaluates assume/require expressions.
// Assume members always pass (trusted), require members must evaluate to true.
func (ctx *Context) EvaluateRequirement(sym *symbols.Symbol, scope *symbols.Scope) (bool, error) {
	return ctx.EvaluateRequirementOn(sym, scope, nil)
}

// EvaluateRequirementOn evaluates a requirement against a concrete instance,
// binding the features it names to that instance's feature values. The subject is chosen
// as EvaluateConstraintOn chooses it, and the subject/actor bindings are
// evaluated against that same object.
func (ctx *Context) EvaluateRequirementOn(sym *symbols.Symbol, scope *symbols.Scope, self *Instance) (bool, error) {
	result, err := ctx.CheckRequirementOn(sym, scope, self)
	return result.Holds, err
}

// CheckRequirementOn evaluates a requirement as EvaluateRequirementOn does and
// also reports the object it turned out to be about.
func (ctx *Context) CheckRequirementOn(sym *symbols.Symbol, scope *symbols.Scope, self *Instance) (CheckResult, error) {
	defer ctx.beginRun()()

	if err := RequireRequirement(sym); err != nil {
		return CheckResult{Subject: self}, err
	}
	subject, err := ctx.checkSubject("requirement", sym.Name, sym, self)
	if err != nil {
		return CheckResult{}, err
	}

	// Requirement-local bindings are shared by every member, whichever scope it
	// was declared in.
	members := ctx.chainMembers(sym, scope)

	// First pass: process subject/actor bindings
	reqBindings, err := ctx.memberBindings(sym, sym.Name, members, subject.instance, nil)

	if err != nil {
		return ctx.checkResultOf(false, subject), err
	}

	// Second pass: evaluate the assumed and required conditions.
	conds := ctx.conditionsOf(sym, members)
	holds, err := ctx.evaluateConditions(conditionCheck{
		sym:      sym,
		kind:     "requirement",
		what:     "require condition",
		self:     subject.instance,
		bindings: reqBindings,
		negated:  NegatedDecl(sym),
	}, conds)
	if err != nil {
		err = unboundSubjectError(err, "requirement", sym.Name, unboundSubjectNames(members, subject.instance))
	}
	return ctx.checkResultOf(holds, subject), err
}

// unboundSubjectNames are the subjects the members declare that nothing supplies
// a value for: no binding expression, no object supplied from outside.
func unboundSubjectNames(members []scopedMember, subject *Instance) map[string]bool {
	if subject != nil {
		return nil
	}
	names := make(map[string]bool)
	for _, member := range members {
		switch rm := member.node.(type) {
		case *ast.SubjectMember:
			if rm.BindingExpr == nil && rm.Name != "" {
				names[rm.Name] = true
			}
		case *ast.Usage:
			if rm.Kind == ast.UsageSubject {
				if name := effectiveName(rm); name != "" {
					names[name] = true
				}
			}
		}
	}
	return names
}

// unboundSubjectError reports a condition that read an unbound subject as such,
// rather than as a feature that happens to carry no value.
func unboundSubjectError(err error, kind, element string, unbound map[string]bool) error {
	var noValue *NoValueError
	if !errors.As(err, &noValue) || !unbound[noValue.Feature] {
		return err
	}
	return &UnboundSubjectError{Kind: kind, Element: element, Subject: noValue.Feature}
}

// ExecuteAction executes an action definition/usage to completion.
// Returns the values the action's features hold when it completed.
func (ctx *Context) ExecuteAction(action *symbols.Symbol) (map[string]Value, error) {
	return ctx.ExecuteActionWithInputs(action, nil)
}

// ExecuteActionWithInputs executes an action, seeding its feature space with the
// provided input parameter bindings (keyed by parameter name). Inputs override
// action attribute defaults of the same name. Returns the final feature values.
func (ctx *Context) ExecuteActionWithInputs(action *symbols.Symbol, inputs map[string]Value) (map[string]Value, error) {
	return ctx.ExecuteActionPerformedBy(action, nil, inputs)
}

// ExecuteActionPerformedBy executes an action performed by self, whose
// connections route what the action sends and whose variant selections decide
// which of them are realized. A nil self performs the action outside any object.
func (ctx *Context) ExecuteActionPerformedBy(action *symbols.Symbol, self *Instance, inputs map[string]Value) (map[string]Value, error) {
	defer ctx.beginRun()()

	// Create executor
	exec, err := newActionExecutor(ctx, action, self)
	if err != nil {
		return nil, fmt.Errorf("create action executor: %w", err)
	}

	// Bind inputs before initialization so they seed the initial token.
	if len(inputs) > 0 {
		exec.SetInputs(inputs)
	}

	// Initialize execution (spawns initial token)
	if err := exec.initialize(); err != nil {
		return nil, fmt.Errorf("initialize action: %w", err)
	}

	// Run to completion
	if err := exec.RunToCompletion(); err != nil {
		return nil, fmt.Errorf("execute action: %w", err)
	}

	// Return the values the action's features hold once it completed
	return exec.Results(), nil
}

// ExecuteState executes a state machine, processing events until completion or suspension.
// Returns final state data from the state machine's execution.
// Execution stops when:
// - A final state is reached (StateCompleted)
// - Event queue is empty (StateSuspended)
// - Max event processing steps exceeded (error)
func (ctx *Context) ExecuteState(stateMachine *symbols.Symbol) (map[string]Value, error) {
	data, _, err := ctx.ExecuteStateWithEvents(stateMachine, nil)
	return data, err
}

// ExecuteStateWithEvents executes a state machine, first injecting the provided
// signal events (by signal-type name) into the event queue, then processing all
// events until completion or suspension. Returns the final state data and the
// ordered list of visited state names.
func (ctx *Context) ExecuteStateWithEvents(stateMachine *symbols.Symbol, events []string) (map[string]Value, []string, error) {
	return ctx.ExecuteStatePerformedBy(stateMachine, nil, events)
}

// ExecuteStatePerformedBy executes a state machine performed by self, whose
// connections route what the machine sends and whose variant selections decide
// which of them are realized. A nil self performs it outside any object.
func (ctx *Context) ExecuteStatePerformedBy(stateMachine *symbols.Symbol, self *Instance, events []string) (map[string]Value, []string, error) {
	defer ctx.beginRun()()

	// Create executor
	exec, err := newStateExecutor(ctx, stateMachine, self)
	if err != nil {
		return nil, nil, fmt.Errorf("create state executor: %w", err)
	}

	// Initialize execution (enters initial state)
	if err := exec.initialize(); err != nil {
		return nil, nil, fmt.Errorf("initialize state machine: %w", err)
	}

	// Inject external signal events. Each event name is treated as a signal type
	// with no arguments; matching accept-triggers consume it in order.
	for _, event := range events {
		exec.SendSignal(event, nil)
	}

	if err := exec.RunToCompletion(); err != nil {
		return nil, nil, err
	}

	// Return state machine data and the real ordered visit trace
	return exec.StateData(), exec.GetStateVisits(), nil
}

// CreateActionExecutor creates an action executor without starting execution.
// For REPL debugging - allows step-by-step execution control.
func (ctx *Context) CreateActionExecutor(action *symbols.Symbol) (*ActionExecutor, error) {
	return ctx.CreateActionExecutorFor(action, nil)
}

// CreateActionExecutorFor creates an action executor for an action performed by
// self, without starting execution.
func (ctx *Context) CreateActionExecutorFor(action *symbols.Symbol, self *Instance) (*ActionExecutor, error) {
	exec, err := newActionExecutor(ctx, action, self)
	if err != nil {
		return nil, fmt.Errorf("create action executor: %w", err)
	}

	// Initialize (spawns initial token)
	if err := exec.initialize(); err != nil {
		return nil, fmt.Errorf("initialize action: %w", err)
	}

	return exec, nil
}

// CreateStateExecutor creates a state executor without starting execution.
// For REPL debugging - allows step-by-step execution control.
func (ctx *Context) CreateStateExecutor(stateMachine *symbols.Symbol) (*StateExecutor, error) {
	return ctx.CreateStateExecutorFor(stateMachine, nil)
}

// CreateStateExecutorFor creates a state executor for a machine performed by
// self, without starting execution.
func (ctx *Context) CreateStateExecutorFor(stateMachine *symbols.Symbol, self *Instance) (*StateExecutor, error) {
	exec, err := newStateExecutor(ctx, stateMachine, self)
	if err != nil {
		return nil, fmt.Errorf("create state executor: %w", err)
	}

	// Initialize (enters initial state, schedules initial events)
	if err := exec.initialize(); err != nil {
		return nil, fmt.Errorf("initialize state machine: %w", err)
	}

	return exec, nil
}
