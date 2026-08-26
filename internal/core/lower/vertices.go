package lower

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// VertexDecls returns every declaration a transition of this state machine may
// name as an endpoint, collected by the pass a lowered graph's vertices are
// collected by, so a checker reads the same ownership the executor does.
// scope is the scope the machine's body was declared in.
func VertexDecls(stateMachineDecl ast.Node, scope *symbols.Scope) (map[ast.Node]bool, error) {
	members, err := machineMembers(stateMachineDecl)
	if err != nil {
		return nil, err
	}
	// Collecting vertices resolves no endpoint, so the graph needs none.
	graph := newStateGraph(scope, nil)
	if err := collectVertices(graph, members, scope, stateMachineIsParallel(stateMachineDecl)); err != nil {
		return nil, err
	}
	vertices := make(map[ast.Node]bool, len(graph.vertexOf))
	for decl := range graph.vertexOf {
		vertices[decl] = true
	}
	return vertices, nil
}
