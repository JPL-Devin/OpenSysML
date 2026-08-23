package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// newTestIndex returns an index holding the bundled standard library, which is
// the resource set a workspace analyzes a document in.
func newTestIndex() *symbols.Index {
	idx := symbols.NewIndex()
	libs.LoadInto(idx)
	return idx
}

// newTestIndexFromDoc is newTestIndex with root indexed under name.
func newTestIndexFromDoc(name string, root *ast.RootNamespace) *symbols.Index {
	idx := newTestIndex()
	idx.AddDocument(name, root)
	return idx
}
