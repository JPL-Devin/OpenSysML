package lower

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

// A short name is not a name: KerML derives effectiveName from declaredName
// alone, so a usage stating only a short name still answers to the feature it
// references — and to its short name, which is a key of its own.
func TestShortNamedReferenceAnswersToBothNames(t *testing.T) {
	for _, target := range []string{"takePhoto", "s"} {
		for _, rel := range []string{"::>", "references"} {
			src := "action photo { action d; action <s> " + rel + " takePhoto; first d then " + target + "; }"
			root := parser.New(source.New("test.sysml", []byte(src))).ParseFile()
			usage := root.Members[0].(*ast.Membership).Member.(*ast.Usage)
			graph, err := ToActionGraph(usage)
			if err != nil {
				t.Fatalf("ToActionGraph(%s): %v", src, err)
			}
			var found bool
			for _, node := range graph.Nodes {
				u, ok := node.(*ast.Usage)
				if !ok || u.Ident.ShortName != "s" {
					continue
				}
				found = true
				if name := getNodeName(u); name != "takePhoto" {
					t.Errorf("%s: node name = %q, want takePhoto", src, name)
				}
				if !nodeAnswersTo(u, "s") {
					t.Errorf("%s: node does not answer to its short name", src)
				}
			}
			if !found {
				t.Fatalf("%s: short-named usage is not a node of the graph", src)
			}
			if len(graph.Edges) == 0 {
				t.Errorf("%s: succession naming %s produced no edge", src, target)
			}
		}
	}
}
