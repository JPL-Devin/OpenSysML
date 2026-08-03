package model

import (
	"os"
	"testing"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/ast"
)

func TestRequirementAST(t *testing.T) {
	path := "../../../examples/sysml-v2-training/32. Requirements/Requirement Definitions.sysml"
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	
	p := parser.New(source.New("test.sysml", content))
	root := p.ParseFile()
	
	unwrap := func(n ast.Node) ast.Node {
		if m, ok := n.(*ast.Membership); ok {
			return m.Member
		}
		return n
	}
	
	pkg := unwrap(root.Members[0]).(*ast.Package)
	t.Logf("Package has %d members", len(pkg.Members))
	
	// Find VehicleMassLimitationRequirement (requirement def with <'1'>)
	reqDef := unwrap(pkg.Members[4]).(*ast.Definition)
	t.Logf("Requirement def: %s, kind=%v", reqDef.Ident.Name, reqDef.Kind)
	t.Logf("Requirement has %d members", len(reqDef.Members))
	
	for i, m := range reqDef.Members {
		inner := unwrap(m)
		switch n := inner.(type) {
		case *ast.Documentation:
			t.Logf("  [%d] Documentation", i)
		case *ast.Usage:
			t.Logf("  [%d] Usage: name=%s, kind=%v, identification=%+v", i, n.Ident.Name, n.Kind, n.Ident)
		// case *ast.FeatureMember:
			// t.Logf("  [%d] FeatureMember: %+v", i, n)
		default:
			t.Logf("  [%d] %T", i, inner)
		}
	}
}
