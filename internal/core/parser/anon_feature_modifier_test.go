package parser

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestAnonymousFeatureWithModifier(t *testing.T) {
	input := `action def Test {
		ref stateSpace: StateSpace;
	}`
	sf := source.New("test.sysml", []byte(input))
	p := New(sf)
	file := p.ParseFile()

	if len(p.Diagnostics) > 0 {
		for _, d := range p.Diagnostics {
			t.Errorf("Parse error: %s", d.Message)
		}
		t.FailNow()
	}

	// File has one namespace member (action def)
	def := file.Members[0].(*ast.Membership).Member.(*ast.Definition)
	bodyMem := def.Members[0].(*ast.Membership)
	usage := bodyMem.Member.(*ast.Usage)

	if usage.Kind != ast.UsageAttribute {
		t.Errorf("Expected UsageAttribute, got %v", usage.Kind)
	}
	if usage.Ident.Name != "stateSpace" {
		t.Errorf("Expected name 'stateSpace', got %s", usage.Ident.Name)
	}
	if !usage.IsReference {
		t.Error("Expected IsReference=true")
	}
	if len(usage.Relationships) < 1 {
		t.Fatal("Expected at least 1 relationship")
	}
	if usage.Relationships[0].Kind != ast.RelTyping {
		t.Errorf("Expected RelTyping, got %v", usage.Relationships[0].Kind)
	}
}
