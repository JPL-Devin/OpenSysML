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
			bind x = y;
			binding named bind x = y;
			binding namedOf of x = y;
			binding namedOnly = x;
			binding [1] config.host = serverAddress;
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
	if len(bindings) != 5 {
		t.Fatalf("lowered %d bindings, want 5", len(bindings))
	}
	wants := map[[2]string]int{
		{"x", "y"}:                       4,
		{"config.host", "serverAddress"}: 1,
	}
	for _, binding := range bindings {
		paths := [2]string{binding.Ends[0].Path, binding.Ends[1].Path}
		if wants[paths] == 0 {
			t.Errorf("binding paths = %q, want one of [x, y] or [config.host, serverAddress]", paths)
			continue
		}
		wants[paths]--
	}
	for paths, count := range wants {
		if count != 0 {
			t.Errorf("binding paths %q occurred %d extra times", paths, count)
		}
	}
}
