package parser

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

// TestEndFeatureKeepsItsEndModifier covers the `end` features a connection or
// interface definition declares (SysML v2 7.13.2): every spelling of the
// modifier — with or without an early multiplicity, with or without a kind
// keyword — yields a usage marked as an end, since the semantic tier matches
// the ends of a connect clause against them by position.
func TestEndFeatureKeepsItsEndModifier(t *testing.T) {
	code := `
connection def PressureSeat {
	end [1] part bead : TireBead;
	end [1] rim : TireMountingRim;
	end supplierPort : FuelOutPort;
	end part consumerPort;
}
`
	p := New(source.New("test.sysml", []byte(code)))
	file := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("expected no parse diagnostics, got %v", p.Diagnostics)
	}

	def, ok := file.Members[0].(*ast.Membership).Member.(*ast.Definition)
	if !ok {
		t.Fatalf("expected a Definition, got %T", file.Members[0].(*ast.Membership).Member)
	}
	want := []string{"bead", "rim", "supplierPort", "consumerPort"}
	// The two ends written with an early multiplicity must keep it, whether or
	// not a kind keyword follows it.
	withMultiplicity := map[string]bool{"bead": true, "rim": true}
	if len(def.Members) != len(want) {
		t.Fatalf("expected %d members, got %d", len(want), len(def.Members))
	}
	for i, name := range want {
		u, ok := def.Members[i].(*ast.Membership).Member.(*ast.Usage)
		if !ok {
			t.Fatalf("member %d: expected a Usage, got %T", i, def.Members[i].(*ast.Membership).Member)
		}
		if u.Ident.Name != name {
			t.Fatalf("member %d: name = %q, want %q", i, u.Ident.Name, name)
		}
		if !u.IsEnd {
			t.Fatalf("%s: IsEnd = false, want true", name)
		}
		if got := u.Multiplicity != nil; got != withMultiplicity[name] {
			t.Fatalf("%s: has multiplicity = %v, want %v", name, got, withMultiplicity[name])
		}
	}
	if bead := def.Members[0].(*ast.Membership).Member.(*ast.Usage); bead.Kind != ast.UsagePart {
		t.Fatalf("bead kind = %v, want part", bead.Kind)
	}
}
