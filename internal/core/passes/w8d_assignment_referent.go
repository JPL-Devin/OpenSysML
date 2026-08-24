package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

const msgAssignmentReferentTimeVarying = "Referent must be time varying."

// AssignmentReferentPass checks SysML AssignmentActionUsage referents.
type AssignmentReferentPass struct{}

func (AssignmentReferentPass) Level() PassLevel { return LevelConstraint }

func (AssignmentReferentPass) ElementScoped() {}

func (AssignmentReferentPass) Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil || !assignmentOccurrenceLibraryPresent(ctx) {
		return nil
	}
	c := &assignmentReferentChecker{
		ctx:      ctx,
		model:    ctx.Model(),
		resolver: ctx.Resolver(),
	}
	c.walk(rootScope, root.Members)
	return c.diags
}

func assignmentOccurrenceLibraryPresent(ctx *Context) bool {
	return len(ctx.Index.LookupQualified("Occurrences::Occurrence")) > 0
}

type assignmentReferentChecker struct {
	ctx      *Context
	model    *semantics.Model
	resolver *resolve.Resolver
	diags    []Diagnostic
}

func (c *assignmentReferentChecker) walk(scope *symbols.Scope, members []ast.Node) {
	for _, member := range members {
		c.walkNode(scope, unwrapType(member))
	}
}

func (c *assignmentReferentChecker) walkNode(scope *symbols.Scope, node ast.Node) {
	switch n := node.(type) {
	case *ast.Definition:
		c.walk(childScopeOr(scope, n), n.Members)
	case *ast.Usage:
		c.walk(childScopeOr(scope, n), n.Members)
	case *ast.Package:
		c.walk(childScopeOr(scope, n), n.Members)
	case *ast.Namespace:
		c.walk(childScopeOr(scope, n), n.Members)
	case *ast.SubjectMember:
		c.walk(childScopeOr(scope, n), n.Body)
	case *ast.ConstraintMember:
		c.walk(scope, n.Body)
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
		body := symbols.TriggerScope(scope, n)
		c.walk(body, n.Effect)
		c.walk(body, n.Members)
	case *ast.SendStatement:
		c.walk(childScopeOr(scope, n), n.Members)
	case *ast.WhileLoopActionNode:
		c.walk(childScopeOr(scope, n), n.Body)
	case *ast.IfActionNode:
		for _, branch := range n.Branches() {
			c.walkNode(scope, branch)
		}
	case *ast.IfBranchNode:
		c.walk(childScopeOr(scope, n), n.Body)
	case *ast.AssignmentActionNode:
		c.check(scope, n)
	}
}

func childScopeOr(scope *symbols.Scope, node ast.Node) *symbols.Scope {
	if child := childScopeOf(scope, node); child != nil {
		return child
	}
	return scope
}

func (c *assignmentReferentChecker) check(scope *symbols.Scope, assignment *ast.AssignmentActionNode) {
	if c.ctx.DownstreamOfFailure(assignment.Target) {
		return
	}
	referent, ok := c.resolver.ResolveTarget(scope, assignment.Target)
	if !ok || referent == nil {
		return
	}
	if _, ok := referent.Decl.(*ast.Usage); !ok || c.model.UsageMayTimeVary(referent) {
		return
	}
	span := assignment.Target.Span()
	if _, targetSpan := ast.TargetName(assignment.Target); targetSpan != (span) {
		span = targetSpan
	}
	c.diags = append(c.diags, Diagnostic{
		Severity: SeverityError,
		Span:     span,
		Message:  msgAssignmentReferentTimeVarying,
		Code:     "assignment-referent-time-varying",
		Source:   "constraint",
	})
}
