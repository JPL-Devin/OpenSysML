package lower

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/resolve"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// Endpoints resolves the vertex a transition endpoint names, implemented by the
// name-resolution tier (*resolve.Resolver) so lowering matches no names itself.
type Endpoints interface {
	// Endpoint returns the declaration qn names, written in scope, whether it names
	// a vertex at all, and whether a failure was already reported to the user.
	Endpoint(scope *symbols.Scope, qn *ast.QualifiedName) (decl ast.Node, ok, reported bool)
}

// machineEndpoints resolves the endpoints of one machine, from where each was
// written when the caller knows that scope, and from the machine's own body when
// it does not — a machine lowered without the scope tree it was declared in.
type machineEndpoints struct {
	resolver *resolve.Resolver
	scope    *symbols.Scope
}

func (m machineEndpoints) Endpoint(scope *symbols.Scope, qn *ast.QualifiedName) (ast.Node, bool, bool) {
	if scope == nil {
		scope = m.scope
	}
	return m.resolver.Endpoint(scope, qn)
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
