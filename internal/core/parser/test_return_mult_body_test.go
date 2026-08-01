package parser

import (
	"testing"
	
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestReturnMultWithBody(t *testing.T) {
	input := `
action def TestCase {
	return ref result[0..*] {
		doc /* The result determined by the case */
	}
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
	if len(root.Members) == 0 {
		t.Fatal("Expected members in root")
	}
	
	mem0 := root.Members[0].(*ast.Membership)
	def := mem0.Member.(*ast.Definition)
	
	if len(def.Members) == 0 {
		t.Fatal("Expected def body with members")
	}
	
	returnParam := def.Members[0].(*ast.Usage)
	
	if returnParam.Kind != ast.UsageAttribute {
		t.Errorf("Expected UsageAttribute, got %v", returnParam.Kind)
	}
	
	if returnParam.Direction != ast.DirOut {
		t.Errorf("Expected DirOut, got %v", returnParam.Direction)
	}
	
	if !returnParam.IsReference {
		t.Error("Expected IsReference=true")
	}
	
	if returnParam.Multiplicity == nil {
		t.Error("Expected multiplicity")
	}
	
	if returnParam.Members == nil || len(returnParam.Members) == 0 {
		t.Error("Expected body with doc comment")
	}
}
