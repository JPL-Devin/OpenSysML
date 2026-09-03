package resolve

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// resolveSuccessionEdge resolves the ends a succession names, the members
// lowering sequences the token flow over: a misspelled one is reported here
// rather than only when the model runs.
func (r *Resolver) resolveSuccessionEdge(scope *symbols.Scope, edge *ast.SuccessionEdge) {
	r.resolveEdgeEnd(scope, edge.Source, edge.SourceMember, edge.SourceImplied)
	r.resolveEdgeEnd(scope, edge.Target, edge.TargetMember, edge.TargetImplied)
}

// resolveControlFlowEdge resolves the ends of a guarded branch of a decision.
func (r *Resolver) resolveControlFlowEdge(scope *symbols.Scope, edge *ast.ControlFlowEdge) {
	r.resolveEdgeEnd(scope, edge.Source, edge.SourceMember, edge.SourceImplied)
	r.resolveEdgeEnd(scope, edge.Target, edge.TargetMember, edge.TargetImplied)
}

// resolveEdgeEnd resolves an end the author named. An end bound to a member by
// position, or one the notation supplied from the member beside the keyword,
// names nothing an author could misspell: lowering reads that member itself.
func (r *Resolver) resolveEdgeEnd(scope *symbols.Scope, qn *ast.QualifiedName, member ast.Node, implied bool) {
	if qn == nil || len(qn.Parts) == 0 || member != nil {
		return
	}
	if implied {
		// The name is the body's own member's, the one a `first x` label also
		// starts at; record what it binds, undiagnosed.
		if len(qn.Parts) != 1 {
			return
		}
		sym, ok := memberPastLabels(scope, qn.Parts[0].Text)
		if !ok {
			sym, ok = scope.LookupLocal(qn.Parts[0].Text)
		}
		if ok {
			r.resolvedPart(qn, 0, sym)
		}
		return
	}
	if inStateMachine(scope) {
		// In a machine, judge the end as a transition endpoint, including its kind.
		r.ResolveEndpoint(scope, qn)
		return
	}
	r.ResolveQualified(scope, qn)
}

// inStateMachine reports whether an edge written in scope belongs to a state
// machine, the body a vertex lookup applies to; an action body or anything else
// is not one.
func inStateMachine(scope *symbols.Scope) bool {
	for s := scope; s != nil; s = s.Parent() {
		switch n := s.Node().(type) {
		case *ast.Definition:
			if n.Kind == ast.DefState {
				return true
			}
			if n.Kind == ast.DefAction {
				return false
			}
		case *ast.Usage:
			if n.Kind == ast.UsageState {
				return true
			}
			if n.Kind == ast.UsageAction {
				return false
			}
		}
	}
	return false
}
