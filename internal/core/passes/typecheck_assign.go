package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// assignWalker visits every `assign` a document states, in every body that
// admits one, and checks the value written against the feature it is written
// to. It walks separately from the type checker's own walk because state,
// transition and node bodies carry their statements outside the member lists
// that walk follows.
type assignWalker struct {
	expr *exprChecker
}

func (w *assignWalker) walk(scope *symbols.Scope, members []ast.Node) {
	for _, member := range members {
		w.walkNode(scope, unwrapType(member))
	}
}

func (w *assignWalker) walkNode(scope *symbols.Scope, node ast.Node) {
	switch n := node.(type) {
	case *ast.Definition:
		w.walk(childScopeOr(scope, n), n.Members)
	case *ast.Usage:
		w.walk(childScopeOr(scope, n), n.Members)
	case *ast.Package:
		w.walk(childScopeOr(scope, n), n.Members)
	case *ast.Namespace:
		w.walk(childScopeOr(scope, n), n.Members)
	case *ast.SubjectMember:
		w.walk(childScopeOr(scope, n), n.Body)
	case *ast.ConstraintMember:
		w.walk(symbols.ConstraintBodyScope(scope, n), n.Body)
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
		body := symbols.TriggerScope(scope, n)
		w.walk(body, n.Effect)
		w.walk(body, n.Members)
	case *ast.SendStatement:
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

// checkAssignmentValue checks the value an assignment writes against the type
// and multiplicity of the feature it writes it to. An assignment assigns the
// values of the feature it accesses (KerML FeatureWritePerformance), so the
// rule is the one that governs binding an initial value; a value whose type is
// not statically known stays for the run time to judge.
func (ec *exprChecker) checkAssignmentValue(scope *symbols.Scope, n *ast.AssignmentActionNode) {
	if n == nil || n.Value == nil {
		return
	}
	target, ok := ec.resolveAssignmentTarget(scope, n.Target)
	if !ok {
		// No feature the declarations identify, so only the written expression
		// itself can be checked.
		ec.infer(scope, n.Value)
		return
	}
	u, isUsage := target.Decl.(*ast.Usage)
	if !isUsage {
		ec.infer(scope, n.Value)
		return
	}
	// The written value's names are the statement's; the declaration read is
	// the target feature's own, which a chained target reaches elsewhere.
	ec.checkBoundValue(scope, target.OwnerScope, u, n.Value)
}

// resolveAssignmentTarget resolves the feature an assignment writes, following
// a feature chain to its final segment: that segment is the feature written.
func (ec *exprChecker) resolveAssignmentTarget(scope *symbols.Scope, target ast.Node) (*symbols.Symbol, bool) {
	if target == nil {
		return nil, false
	}
	sym, ok := ec.resolver.ResolveTarget(scope, target)
	if !ok || sym == nil {
		return nil, false
	}
	if sym.Kind == symbols.SymbolAlias {
		if resolved, ok := ec.resolver.ResolveAliasTarget(sym); ok && resolved != nil {
			sym = resolved
		}
	}
	return sym, true
}
