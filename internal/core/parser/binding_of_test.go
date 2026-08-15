package parser

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/source"
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
