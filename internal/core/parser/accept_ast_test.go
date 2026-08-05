package parser

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"testing"
)

func TestAcceptActionAST(t *testing.T) {
	src := `package test {
		item def Scene;
		
		action takePicture {
			action trigger accept scene : Scene;
		}
	}`

	file := New(source.New("test", []byte(src))).ParseFile()

	t.Logf("Root members: %d", len(file.Members))
	for i, m := range file.Members {
		if memb, ok := m.(*ast.Membership); ok {
			t.Logf("  Member %d: %T", i, memb.Member)
		}
	}

	// Navigate to action takePicture
	pkg := file.Members[0].(*ast.Membership).Member.(*ast.Package)
	t.Logf("Package members: %d", len(pkg.Members))
	for i, m := range pkg.Members {
		if memb, ok := m.(*ast.Membership); ok {
			t.Logf("  Member %d: %T", i, memb.Member)
		}
	}

	if len(pkg.Members) < 2 {
		t.Fatalf("Expected at least 2 members in package, got %d", len(pkg.Members))
	}

	takePictureMemb := pkg.Members[1].(*ast.Membership)
	takePicture := takePictureMemb.Member.(*ast.Usage)

	// Check nested action trigger
	triggerMemb := takePicture.Members[0].(*ast.Membership)
	trigger := triggerMemb.Member

	t.Logf("Nested action type: %T", trigger)
	if usage, ok := trigger.(*ast.Usage); ok {
		t.Logf("  Name: %s", usage.Ident.Name)
		t.Logf("  Kind: %v", usage.Kind)
		t.Logf("  Members count: %d", len(usage.Members))
		for i, m := range usage.Members {
			if memb, ok := m.(*ast.Membership); ok {
				t.Logf("    Member %d: %T", i, memb.Member)
				if innerUsage, ok := memb.Member.(*ast.Usage); ok {
					t.Logf("      Name: %s", innerUsage.Ident.Name)
					t.Logf("      Kind: %v", innerUsage.Kind)
					t.Logf("      Direction: %v", innerUsage.Direction)
					t.Logf("      IsAccept: %v", innerUsage.IsAccept)
				}
			}
		}
	}
}
