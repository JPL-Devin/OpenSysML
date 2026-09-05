package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Diagnostic codes of the trigger argument rules (SysML v2 §8.3.17,
// TriggerInvocationExpression): the argument of `after` is a DurationValue,
// of `at` a TimeInstantValue, of `when` a Boolean.
const (
	codeTriggerAfterDuration = "trigger-after-duration"
	codeTriggerAtTimeInstant = "trigger-at-time-instant"
	codeTriggerWhenBoolean   = "trigger-when-boolean"

	msgTriggerAfterDuration = "an 'after' trigger's delay must be a DurationValue, found %s: write it with a duration unit (`after 5 [s]`) or name a feature typed by DurationValue"
	msgTriggerAtTimeInstant = "an 'at' trigger's time must be a TimeInstantValue, found %s: name a feature typed by TimeInstantValue or a clock reading such as `TimeOf(this)`"
	msgTriggerWhenBoolean   = "a 'when' trigger's condition must be Boolean, found %s: compare the value (`when x > 3`) or name a feature typed by Boolean"
)

// TriggerArgumentPass types the argument of every `after`, `at` or `when`
// trigger, each gated on its own argument rather than the whole document.
type TriggerArgumentPass struct{}

func (TriggerArgumentPass) Level() PassLevel { return LevelType }

func (TriggerArgumentPass) ElementScoped() { /* marker: per-element gating */ }

func (TriggerArgumentPass) Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	c := &triggerArgumentChecker{
		ctx:  ctx,
		expr: &exprChecker{resolver: ctx.Resolver(), model: ctx.Model()},
	}
	c.walk(rootScope, root.Members)
	return c.expr.diags
}

type triggerArgumentChecker struct {
	ctx  *Context
	expr *exprChecker
}

func (c *triggerArgumentChecker) walk(scope *symbols.Scope, members []ast.Node) {
	for _, member := range members {
		c.walkNode(scope, unwrapType(member))
	}
}

// walkNode descends through every body shape a trigger may be written in.
func (c *triggerArgumentChecker) walkNode(scope *symbols.Scope, node ast.Node) {
	switch n := node.(type) {
	case *ast.Definition:
		c.walk(childScopeOr(scope, n), n.Members)
	case *ast.Usage:
		if n.IsAccept {
			c.check(scope, n)
		}
		c.walk(childScopeOr(scope, n), n.Members)
	case *ast.Package:
		c.walk(childScopeOr(scope, n), n.Members)
	case *ast.Namespace:
		c.walk(childScopeOr(scope, n), n.Members)
	case *ast.SubjectMember:
		c.walk(childScopeOr(scope, n), n.Body)
	case *ast.ConstraintMember:
		c.walk(symbols.ConstraintBodyScope(scope, n), n.Body)
	case *ast.AssumeMember:
		c.walk(symbols.ConstraintBodyScope(scope, n), n.Body)
	case *ast.RequireMember:
		c.walk(symbols.ConstraintBodyScope(scope, n), n.Body)
	case *ast.EntryMember:
		c.walk(scope, n.Actions)
	case *ast.DoMember:
		c.walk(scope, n.Actions)
	case *ast.ExitMember:
		c.walk(scope, n.Actions)
	case *ast.InitialNode:
		c.walk(childScopeOr(scope, n), n.Members)
	case *ast.ForkNode, *ast.JoinNode, *ast.MergeNode, *ast.DecisionNode:
		c.walk(childScopeOr(scope, n), ast.NodeBodyMembers(n))
	case *ast.StateNode:
		body := childScopeOr(scope, n)
		c.walk(body, n.Entry)
		c.walk(body, n.Do)
		c.walk(body, n.Exit)
		c.walk(body, n.Substates)
		for _, region := range n.Regions {
			c.walkNode(body, region)
		}
	case *ast.StateRegion:
		c.walk(childScopeOr(scope, n), n.States)
	case *ast.TransitionMember:
		c.check(scope, n.Trigger)
		body := symbols.TriggerScope(scope, n)
		c.walk(body, n.Effect)
		c.walk(body, n.Members)
	case *ast.SendStatement:
		c.walk(childScopeOr(scope, n), n.Members)
	case *ast.SuccessionEdge:
		c.walk(childScopeOr(scope, n), n.Members)
	case *ast.WhileLoopActionNode:
		c.walk(childScopeOr(scope, n), n.Body)
	case *ast.IfActionNode:
		for _, branch := range n.Branches() {
			c.walkNode(scope, branch)
		}
	case *ast.IfBranchNode:
		c.walk(childScopeOr(scope, n), n.Body)
	}
}

// check types one trigger unless its argument rests on a lower-tier fault.
func (c *triggerArgumentChecker) check(scope *symbols.Scope, trigger ast.Node) {
	trigger, arg := triggerArgument(trigger)
	if arg == nil || c.ctx.DownstreamOfFailure(arg) {
		return
	}
	if t, ok := trigger.(*ast.TimeEvent); ok {
		c.expr.checkTimeEvent(scope, t)
		return
	}
	c.expr.checkCondition(scope, arg, codeTriggerWhenBoolean, msgTriggerWhenBoolean, true)
}

// triggerArgument returns a trigger and the expression its rule judges; a
// signal or call trigger (a bare name included) names an event and has none.
func triggerArgument(trigger ast.Node) (ast.Node, ast.Node) {
	switch t := trigger.(type) {
	case nil:
		return nil, nil
	case *ast.ChangeEvent:
		return t, t.Condition
	case *ast.TimeEvent:
		return t, t.Duration
	case *ast.Usage:
		if t.IsAccept && isTriggerValue(t.Value) {
			return triggerArgument(t.Value)
		}
		return nil, nil
	case *ast.FeatureReference, *ast.QualifiedName, *ast.AcceptEvent, *ast.CallEvent:
		return nil, nil
	}
	return trigger, trigger
}

// isTriggerValue reports whether a usage's value is the trigger of an accept
// (`accept after 5 [s]`), checked by the body walk, rather than a bound value.
func isTriggerValue(value ast.Node) bool {
	switch value.(type) {
	case *ast.TimeEvent, *ast.ChangeEvent:
		return true
	}
	return false
}

// checkTimeEvent checks the argument of a time trigger against the library type
// it must be a value of. An argument whose type only evaluation determines is
// not reported.
func (ec *exprChecker) checkTimeEvent(scope *symbols.Scope, t *ast.TimeEvent) {
	if t.Duration == nil {
		return
	}
	ec.infer(scope, t.Duration)
	fqn, code, format := semantics.FQNDurationValue, codeTriggerAfterDuration, msgTriggerAfterDuration
	if t.Absolute {
		fqn, code, format = semantics.FQNTimeInstantValue, codeTriggerAtTimeInstant, msgTriggerAtTimeInstant
	}
	c := ec.model.ExprConformsToLibrary(scope, t.Duration, fqn)
	if !c.Known || c.Holds {
		return
	}
	ec.errorCode(code, t.Duration.Span(), format, c.Found)
}
