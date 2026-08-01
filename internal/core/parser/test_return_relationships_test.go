package parser

import (
	"testing"
	
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestReturnWithRelationships(t *testing.T) {
	input := `
action def TestCase {
	return verdict : VerdictKind :>> result;
}
`
	sf := source.New("test.sysml", []byte(input))
	p := New(sf)
	node := p.ParseFile()
	
	if len(p.Diagnostics) > 0 {
		for _, d := range p.Diagnostics {
			t.Errorf("Parse error: %s at offset %d", d.Message, d.Span.Offset)
		}
	}
	
	root := node
	mem0 := root.Members[0].(*ast.Membership)
	def := mem0.Member.(*ast.Definition)
	returnParam := def.Members[0].(*ast.Usage)
	
	if len(returnParam.Relationships) != 2 {
		t.Fatalf("Expected 2 relationships, got %d", len(returnParam.Relationships))
	}
	
	if returnParam.Relationships[0].Kind != ast.RelTyping {
		t.Errorf("Expected first relationship to be Typing, got %v", returnParam.Relationships[0].Kind)
	}
	
	if returnParam.Relationships[1].Kind != ast.RelRedefines {
		t.Errorf("Expected second relationship to be Redefines, got %v", returnParam.Relationships[1].Kind)
	}
}
