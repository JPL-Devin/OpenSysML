package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// `binding b of f = v` names the feature the binding binds, so the `of` target
// is a reference and not a typing: typing it would type-check the bound feature
// as the binding's type.
func TestBindingOfTargetIsAReference(t *testing.T) {
	const code = `package P { binding linkAToB of [1] featureA = [1] featureB; }`

	p := New(source.New("test.sysml", []byte(code)))
	file := p.ParseFile()
	for _, d := range p.Diagnostics {
		t.Errorf("offset %d: %s", d.Span.Offset, d.Message)
	}

	binding := findBindingUsage(t, file)
	var kinds []ast.RelationshipKind
	for _, rel := range binding.Relationships {
		kinds = append(kinds, rel.Kind)
	}
	for _, kind := range kinds {
		if kind == ast.RelTyping {
			t.Errorf("`of` target became a typing relationship: %v", kinds)
		}
	}
	if !hasKind(kinds, ast.RelReferences) {
		t.Errorf("`of` target is not a reference relationship: %v", kinds)
	}
}

func TestAnonymousBindingSimpleEndsAreReferences(t *testing.T) {
	const code = `package P {
		binding bind b = a;
		bind b = a;
	}`

	sf := source.New("test.sysml", []byte(code))
	file := New(sf).ParseFile()
	var bindings []*ast.Usage
	var collect func([]ast.Node)
	collect = func(nodes []ast.Node) {
		for _, node := range nodes {
			switch v := node.(type) {
			case *ast.Membership:
				collect([]ast.Node{v.Member})
			case *ast.Package:
				collect(v.Members)
			case *ast.Usage:
				if v.Kind == ast.UsageBinding {
					bindings = append(bindings, v)
				}
				collect(v.Members)
			}
		}
	}
	collect(file.Members)
	if len(bindings) != 2 {
		t.Fatalf("parsed %d bindings, want 2", len(bindings))
	}
	for _, binding := range bindings {
		if binding.Ident.Name != "" {
			t.Errorf("binding ident = %q, want empty", binding.Ident.Name)
		}
		if len(binding.Relationships) != 1 {
			t.Fatalf("binding has %d relationships, want 1", len(binding.Relationships))
		}
		rel := binding.Relationships[0]
		if rel.Kind != ast.RelReferences {
			t.Errorf("binding relationship kind = %v, want references", rel.Kind)
		}
		target, ok := rel.Target.(*ast.QualifiedName)
		if !ok || len(target.Parts) != 1 || target.Parts[0].Text != "b" {
			t.Errorf("binding target = %#v, want qualified name b", rel.Target)
			continue
		}
		if sf.Text(target.Parts[0].Span) != "b" {
			t.Errorf("binding target span = %#v, want source span for b", target)
		}
	}
}

func findBindingUsage(t *testing.T, root *ast.RootNamespace) *ast.Usage {
	t.Helper()

	var find func(nodes []ast.Node) *ast.Usage
	find = func(nodes []ast.Node) *ast.Usage {
		for _, n := range nodes {
			switch v := n.(type) {
			case *ast.Membership:
				if found := find([]ast.Node{v.Member}); found != nil {
					return found
				}
			case *ast.Package:
				if found := find(v.Members); found != nil {
					return found
				}
			case *ast.Usage:
				if v.Kind == ast.UsageBinding {
					return v
				}
				if found := find(v.Members); found != nil {
					return found
				}
			}
		}
		return nil
	}

	found := find(root.Members)
	if found == nil {
		t.Fatalf("no binding usage parsed")
	}
	return found
}

func hasKind(kinds []ast.RelationshipKind, want ast.RelationshipKind) bool {
	for _, kind := range kinds {
		if kind == want {
			return true
		}
	}
	return false
}
