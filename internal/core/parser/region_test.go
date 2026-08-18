package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

func TestRegionParsing(t *testing.T) {
	src := `
package Test {
	state def TrafficLight {
		region pedestrian {
			state Walk;
			state DontWalk;
		}
		
		region vehicle {
			state Green;
			state Yellow;
			state Red;
		}
	}
}
`

	file := source.New("test.sysml", []byte(src))
	p := New(file)
	root := p.ParseFile()

	if len(p.Diagnostics) > 0 {
		for _, d := range p.Diagnostics {
			t.Logf("Diagnostic: %s", d.Message)
		}
		t.Fatalf("Expected 0 diagnostics, got %d", len(p.Diagnostics))
	}

	// Navigate to state definition
	pkg, ok := root.Members[0].(*ast.Membership)
	if !ok {
		t.Fatalf("Expected membership, got %T", root.Members[0])
	}

	pkgNode, ok := pkg.Member.(*ast.Package)
	if !ok {
		t.Fatalf("Expected Package, got %T", pkg.Member)
	}

	stateMembership, ok := pkgNode.Members[0].(*ast.Membership)
	if !ok {
		t.Fatalf("Expected state membership, got %T", pkgNode.Members[0])
	}

	stateDef, ok := stateMembership.Member.(*ast.Definition)
	if !ok {
		t.Fatalf("Expected state Definition, got %T", stateMembership.Member)
	}

	t.Logf("State def has %d members", len(stateDef.Members))

	// Count regions
	regionCount := 0
	for _, member := range stateDef.Members {
		if membership, ok := member.(*ast.Membership); ok {
			member = membership.Member
		}
		if region, ok := member.(*ast.StateRegion); ok {
			regionCount++
			t.Logf("Found region: %s with %d states", region.Name, len(region.States))
		}
	}

	if regionCount != 2 {
		t.Errorf("Expected 2 regions, got %d", regionCount)
	}
}
