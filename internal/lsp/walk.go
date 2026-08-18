package lsp

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// collectRefs gathers every qualified-name reference in a document with the
// scope it resolves in (see resolve.References).
func collectRefs(root *ast.RootNamespace, rootScope *symbols.Scope) []resolve.Reference {
	return resolve.References(root, rootScope)
}

// refAtOffset returns the reference whose qualified-name span contains offset.
func refAtOffset(refs []resolve.Reference, offset int) *resolve.Reference {
	for i := range refs {
		sp := refs[i].QN.Span()
		if offset >= sp.Offset && offset < sp.End() {
			return &refs[i]
		}
	}
	return nil
}
