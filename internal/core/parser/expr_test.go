package parser

import (
	"testing"
	
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestParseExprParam(t *testing.T) {
	input := `calc def 'if' {
		in expr thenValue[0..1] { return : Anything[0..*] ordered nonunique; }
	}`
	
	sf := source.New("test.sysml", []byte(input))
	p := New(sf)
	ns := p.ParseFile()
	
	if len(p.Diagnostics) > 0 {
		for _, d := range p.Diagnostics {
			t.Logf("Diagnostic: %s (offset=%d)", d.Message, d.Span.Offset)
		}
	}
	
	if len(ns.Members) != 1 {
		t.Fatalf("Expected 1 member, got %d", len(ns.Members))
	}
	
	m1, ok := ns.Members[0].(*ast.Membership)
	if !ok {
		t.Fatalf("Expected Membership, got %T", ns.Members[0])
	}
	
	def, ok := m1.Member.(*ast.Definition)
	if !ok {
		t.Fatalf("Expected Definition, got %T", m1.Member)
	}
	
	dump := ast.Dump(def)
	t.Logf("AST dump:\n%s", dump)
	
	// Should have 1 member: thenValue expr param
	if len(def.Members) < 1 {
		t.Fatalf("Expected at least 1 member in calc body, got %d", len(def.Members))
	}
	
	// Check first member
	firstMem, ok := def.Members[0].(*ast.Membership)
	if !ok {
		t.Fatalf("Expected Membership, got %T", def.Members[0])
	}
	
	firstUsage, ok := firstMem.Member.(*ast.Usage)
	if !ok {
		t.Fatalf("Expected first to be Usage, got %T", firstMem.Member)
	}
	
	if firstUsage.Kind != ast.UsageExpr {
		t.Errorf("Expected UsageExpr, got %v", firstUsage.Kind)
	}
	
	if firstUsage.Ident.Name != "thenValue" {
		t.Errorf("Expected name 'thenValue', got %q", firstUsage.Ident.Name)
	}
	
	if firstUsage.Direction != ast.DirIn {
		t.Errorf("Expected direction In, got %v", firstUsage.Direction)
	}
}
