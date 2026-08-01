package parser

import (
	"testing"
	
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestParseReturnModsAfterType(t *testing.T) {
	input := `
	function def test {
		in x: Anything[0..*] nonunique;
		return : Anything[0..*] nonunique;
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
	if len(def.Members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(def.Members))
	}
	
	ret := def.Members[1].(*ast.Usage)
	if !ret.IsNonunique {
		t.Errorf("expected IsNonunique=true")
	}
	t.Logf("Passed: return : Type[mult] nonunique;")
}
