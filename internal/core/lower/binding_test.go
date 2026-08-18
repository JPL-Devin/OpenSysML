package lower

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

func TestToBindingsNormalizesBindingSpellings(t *testing.T) {
	p := parser.New(source.New("binding.sysml", []byte(`package P {
		part def Owner {
			binding bind x = y;
			binding named bind x = y;
			binding namedOf of x = y;
		}
	}`)))
	file := p.ParseFile()
	idx := symbols.NewIndex()
	idx.AddDocument("binding.sysml", file)
	scope := idx.DocumentRoot("binding.sysml")
	var bindings []Binding
	for _, member := range file.Members {
		membership, ok := member.(*ast.Membership)
		if !ok {
			continue
		}
		pkg, ok := membership.Member.(*ast.Package)
		if !ok {
			continue
		}
		for _, nested := range pkg.Members {
			ownerMembership, ok := nested.(*ast.Membership)
			if !ok {
				continue
			}
			owner, ok := ownerMembership.Member.(*ast.Definition)
			if ok {
				bindings = append(bindings, ToBindings(owner, scope)...)
			}
		}
	}
	if len(bindings) != 3 {
		t.Fatalf("lowered %d bindings, want 3", len(bindings))
	}
	for _, binding := range bindings {
		if binding.Ends[0].Path != "x" || binding.Ends[1].Path != "y" {
			t.Errorf("binding paths = [%q, %q], want [x, y]", binding.Ends[0].Path, binding.Ends[1].Path)
		}
	}
}
