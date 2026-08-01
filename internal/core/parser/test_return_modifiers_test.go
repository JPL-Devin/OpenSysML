package parser

import (
	"testing"
	
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestParseReturnModifierNoType(t *testing.T) {
	input := `
	calc def Test {
		return attribute result;
	}
	`
	
	sf := source.New("test.sysml", []byte(input))
	p := New(sf)
	file := p.ParseFile()
	
	if len(p.Diagnostics) > 0 {
		for _, d := range p.Diagnostics {
			t.Logf("diagnostic: %v", d)
		}
		t.Fatalf("parse errors: %d diagnostics", len(p.Diagnostics))
	}
	
	memb := file.Members[0].(*ast.Membership)
	def := memb.Member.(*ast.Definition)
	if len(def.Members) != 1 {
		t.Fatalf("expected 1 member, got %d", len(def.Members))
	}
	
	u := def.Members[0].(*ast.Usage)
	if u.Kind != ast.UsageAttribute {
		t.Errorf("expected UsageAttribute, got %v", u.Kind)
	}
	if u.Direction != ast.DirOut {
		t.Errorf("expected DirOut, got %v", u.Direction)
	}
	if u.Ident.Name != "result" {
		t.Errorf("expected name 'result', got %v", u.Ident.Name)
	}
	t.Logf("Pattern 5 passed: return attribute result;")
}

func TestParseReturnModifierWithBody(t *testing.T) {
	input := `
	calc def Test {
		return attribute result : ScalarValue[1] {
			doc /* comment */
		}
	}
	`
	
	sf := source.New("test.sysml", []byte(input))
	p := New(sf)
	file := p.ParseFile()
	
	if len(p.Diagnostics) > 0 {
		for _, d := range p.Diagnostics {
			t.Logf("diagnostic: %v", d)
		}
		t.Fatalf("parse errors: %d diagnostics", len(p.Diagnostics))
	}
	
	memb := file.Members[0].(*ast.Membership)
	def := memb.Member.(*ast.Definition)
	if len(def.Members) != 1 {
		t.Fatalf("expected 1 member, got %d", len(def.Members))
	}
	
	u := def.Members[0].(*ast.Usage)
	if u.Kind != ast.UsageAttribute {
		t.Errorf("expected UsageAttribute, got %v", u.Kind)
	}
	if !u.HasBody {
		t.Errorf("expected HasBody=true")
	}
	if len(u.Members) != 1 {
		t.Errorf("expected 1 body member (doc), got %d", len(u.Members))
	}
	t.Logf("Pattern 6 passed: return attribute result : Type { body }")
}
