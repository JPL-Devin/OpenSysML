package lower

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/resolve"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// Endpoints resolves the vertex a transition endpoint names, implemented by the
// name-resolution tier (*resolve.Resolver) so lowering matches no names itself.
type Endpoints interface {
	// Endpoint returns the declaration qn names, written in scope, and whether it
	// names a vertex at all.
	Endpoint(scope *symbols.Scope, qn *ast.QualifiedName) (decl ast.Node, ok bool)
}

// machineEndpoints resolves every endpoint of one machine from that machine's
// own scope, indexed on its own: a machine lowered without the scope tree its
// endpoints were written in has no other scope to name them from.
type machineEndpoints struct {
	resolver *resolve.Resolver
	scope    *symbols.Scope
}

func (m machineEndpoints) Endpoint(_ *symbols.Scope, qn *ast.QualifiedName) (ast.Node, bool) {
	return m.resolver.Endpoint(m.scope, qn)
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

// localEndpoints indexes a machine no document declares — a hand-built one in a
// unit test — and resolves its endpoints against that, from the machine's body.
func localEndpoints(decl ast.Node) Endpoints {
	root := &ast.RootNamespace{Members: []ast.Node{decl}}
	idx := symbols.NewIndexFromDoc("<lowered>", root)
	scope := idx.DocumentRoot("<lowered>")
	if scope != nil {
		if body := scope.ChildFor(decl); body != nil {
			scope = body
		}
	}
	return machineEndpoints{resolver: resolve.New(idx), scope: scope}
}
