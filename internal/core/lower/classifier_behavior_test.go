package lower

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// A note beside a behavior a type binds describes the binding, so the binding
// still names the element holding the body rather than stating one itself.
func TestClassifierBehaviorAnnotationIsNotABody(t *testing.T) {
	behaviors := classifierBehaviorsIn(t, `
		package test {
			state def Modes;
			action def Report;

			part def Monitor {
				exhibit state modes : Modes {
					doc /* the operating modes */
					comment /* watched closely */
				}
				perform action report : Report {
					doc /* the report */
				}
			}
		}
	`)

	if len(behaviors) != 2 {
		t.Fatalf("expected the type to bind 2 behaviors, got %d", len(behaviors))
	}
	for _, behavior := range behaviors {
		if behavior.StatesBody {
			t.Errorf("%s %q states a body, but its members only annotate the binding",
				behavior.Kind, behavior.Name)
		}
	}
}

// A binding that both annotates itself and states a body still states one.
func TestClassifierBehaviorAnnotatedBodyIsABody(t *testing.T) {
	behaviors := classifierBehaviorsIn(t, `
		package test {
			part def Monitor {
				exhibit state modes {
					doc /* the operating modes */
					entry; then start;
					state start;
					state idle;
					succession first start then idle;
				}
			}
		}
	`)

	if len(behaviors) != 1 {
		t.Fatalf("expected the type to bind 1 behavior, got %d", len(behaviors))
	}
	if !behaviors[0].StatesBody {
		t.Error("an annotated machine body was read as a binding that states none")
	}
}

// classifierBehaviorsIn parses src and reports the behaviors the first part
// definition in it binds to its objects.
func classifierBehaviorsIn(t *testing.T, src string) []ClassifierBehavior {
	t.Helper()
	p := parser.New(source.New("test.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse errors: %v", p.Diagnostics)
	}

	var found *ast.Definition
	var walk func(members []ast.Node)
	walk = func(members []ast.Node) {
		for _, member := range members {
			if found != nil {
				return
			}
			switch node := unwrapMembership(member).(type) {
			case *ast.Package:
				walk(node.Members)
			case *ast.Definition:
				if node.Kind == ast.DefPart {
					found = node
				}
			}
		}
	}
	walk(root.Members)
	if found == nil {
		t.Fatal("no part definition in the source")
	}
	return ClassifierBehaviorsOf(found.Members)
}
