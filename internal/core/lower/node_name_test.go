package lower

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

// A short name is not a name: KerML derives effectiveName from declaredName
// alone, so a usage stating only a short name still answers to the feature it
// references, and a succession may name it.
func TestShortNamedReferenceKeepsTheNameItReferences(t *testing.T) {
	for _, src := range []string{
		`action photo { action d; action <s> ::> takePhoto; first d then takePhoto; }`,
		`action photo { action d; action <s> references takePhoto; first d then takePhoto; }`,
	} {
		root := parser.New(source.New("test.sysml", []byte(src))).ParseFile()
		usage := root.Members[0].(*ast.Membership).Member.(*ast.Usage)
		graph, err := ToActionGraph(usage)
		if err != nil {
			t.Fatalf("ToActionGraph(%s): %v", src, err)
		}
		var found bool
		for _, node := range graph.Nodes {
			if u, ok := node.(*ast.Usage); ok && u.Ident.ShortName == "s" {
				if name := getNodeName(u); name != "takePhoto" {
					t.Errorf("%s: node name = %q, want takePhoto", src, name)
				}
				found = true
			}
		}
		if !found {
			t.Fatalf("%s: short-named usage is not a node of the graph", src)
		}
	}
}
