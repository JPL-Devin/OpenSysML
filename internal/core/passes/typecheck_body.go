package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// bodyWalker visits every statement a document states in a behavior body — an
// `assign`, a transition or accept trigger — and checks it in the body it
// stands in. It walks separately from the type checker's own walk because
// state, transition and node bodies carry their statements outside the member
// lists that walk follows.
type bodyWalker struct {
	expr *exprChecker
}

func (w *bodyWalker) walk(scope *symbols.Scope, members []ast.Node) {
	for _, member := range members {
		w.walkNode(scope, unwrapType(member))
	}
}

func (w *bodyWalker) walkNode(scope *symbols.Scope, node ast.Node) {
	switch n := node.(type) {
	case *ast.Definition:
		w.walk(childScopeOr(scope, n), n.Members)
	case *ast.Usage:
		if n.IsAccept {
			w.expr.checkTrigger(scope, n)
		}
		w.walk(childScopeOr(scope, n), n.Members)
	case *ast.Package:
		w.walk(childScopeOr(scope, n), n.Members)
	case *ast.Namespace:
		w.walk(childScopeOr(scope, n), n.Members)
	case *ast.SubjectMember:
		w.walk(childScopeOr(scope, n), n.Body)
	case *ast.ConstraintMember:
		w.walk(scope, n.Body)
	case *ast.AssumeMember:
		w.walk(symbols.ConstraintBodyScope(scope, n), n.Body)
	case *ast.RequireMember:
		w.walk(symbols.ConstraintBodyScope(scope, n), n.Body)
	case *ast.EntryMember:
		w.walk(scope, n.Actions)
	case *ast.DoMember:
		w.walk(scope, n.Actions)
	case *ast.ExitMember:
		w.walk(scope, n.Actions)
	case *ast.InitialNode:
		w.walk(childScopeOr(scope, n), n.Members)
	case *ast.ForkNode, *ast.JoinNode, *ast.MergeNode, *ast.DecisionNode:
		w.walk(childScopeOr(scope, n), ast.NodeBodyMembers(n))
	case *ast.StateNode:
		body := childScopeOr(scope, n)
		w.walk(body, n.Entry)
		w.walk(body, n.Do)
		w.walk(body, n.Exit)
		w.walk(body, n.Substates)
		for _, region := range n.Regions {
			w.walkNode(body, region)
		}
	case *ast.StateRegion:
		w.walk(childScopeOr(scope, n), n.States)
	case *ast.TransitionMember:
		w.expr.checkTrigger(scope, n.Trigger)
		body := symbols.TriggerScope(scope, n)
		w.walk(body, n.Effect)
		w.walk(body, n.Members)
	case *ast.SendStatement:
		w.walk(childScopeOr(scope, n), n.Members)
	case *ast.SuccessionEdge:
		w.walk(childScopeOr(scope, n), n.Members)
	case *ast.WhileLoopActionNode:
		w.walk(childScopeOr(scope, n), n.Body)
	case *ast.IfActionNode:
		for _, branch := range n.Branches() {
			w.walkNode(scope, branch)
		}
	case *ast.IfBranchNode:
		w.walk(childScopeOr(scope, n), n.Body)
	case *ast.AssignmentActionNode:
		w.expr.checkAssignmentValue(scope, n)
	}
}
