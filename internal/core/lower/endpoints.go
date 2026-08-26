package lower

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Endpoints resolves the vertex a transition endpoint names, implemented by the
// name-resolution tier (*resolve.Resolver) so lowering matches no names itself.
type Endpoints interface {
	// Endpoint returns the declaration qn names, written in scope, and whether it
	// names a vertex at all.
	Endpoint(scope *symbols.Scope, qn *ast.QualifiedName) (decl ast.Node, ok bool)
}

// successionEnds returns the names a succession usage states its two ends with
// (`succession first a then b;`), and whether the usage is one at all.
func successionEnds(n *ast.Usage) (source, target *ast.QualifiedName, ok bool) {
	if n.Kind != ast.UsageSuccession || len(n.ConnectorEnds) != 2 {
		return nil, nil, false
	}
	end := func(e *ast.ConnectorEnd) *ast.QualifiedName {
		if e == nil {
			return nil
		}
		named := e.Target
		if named == nil {
			named = e.Reference
		}
		qn, _ := named.(*ast.QualifiedName)
		return qn
	}
	source, target = end(n.ConnectorEnds[0]), end(n.ConnectorEnds[1])
	return source, target, source != nil && target != nil
}

// machineEndpoints resolves the endpoints of one machine through a resolver over
// an index of that machine, from the scope each endpoint was written in.
type machineEndpoints struct {
	resolver *resolve.Resolver
	scope    *symbols.Scope
}

func (m machineEndpoints) Endpoint(scope *symbols.Scope, qn *ast.QualifiedName) (ast.Node, bool) {
	if scope == nil {
		scope = m.scope
	}
	return m.resolver.Endpoint(scope, qn)
}

// scopeEndpoints resolves an endpoint from the caller's own scope tree, for a
// machine lowered with that tree but without the resolver over its document.
type scopeEndpoints struct{ machine *symbols.Scope }

func (s scopeEndpoints) Endpoint(scope *symbols.Scope, qn *ast.QualifiedName) (ast.Node, bool) {
	if scope == nil {
		scope = s.machine
	}
	return resolve.VertexInScope(scope, qn)
}

// localEndpoints indexes a machine no scope tree holds — a hand-built one in a
// unit test — and returns its endpoints plus the machine body scope of that
// index, which lowering descends so a region names its own vertices.
func localEndpoints(decl ast.Node) (Endpoints, *symbols.Scope) {
	root := &ast.RootNamespace{Members: []ast.Node{decl}}
	idx := symbols.NewIndexFromDoc("<lowered>", root)
	scope := idx.DocumentRoot("<lowered>")
	if scope != nil {
		if body := scope.ChildFor(decl); body != nil {
			scope = body
		}
	}
	return machineEndpoints{resolver: resolve.New(idx), scope: scope}, scope
}
