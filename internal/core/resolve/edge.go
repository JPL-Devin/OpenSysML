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
	if qn == nil || len(qn.Parts) == 0 || member != nil || implied {
		return
	}
	r.ResolveQualified(scope, qn)
}
