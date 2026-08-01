// Simple parser test for Phase C demo
package main

import (
	"fmt"
	"os"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: test-parse <file.sysml>")
		os.Exit(1)
	}

	content, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		os.Exit(1)
	}

	src := source.New(os.Args[1], content)
	p := parser.New(src)
	file := p.ParseFile()

	if len(p.Diagnostics) > 0 {
		fmt.Println("Parse errors:")
		for _, diag := range p.Diagnostics {
			fmt.Printf("  %s\n", diag)
		}
		os.Exit(1)
	}

	fmt.Printf("✓ Parsed successfully: %s\n", os.Args[1])
	fmt.Printf("  Root members: %d\n", len(file.Members))
	
	// Count behavioral elements
	calcCount := 0
	constraintCount := 0
	requirementCount := 0
	actionCount := 0
	stateCount := 0
	
	for _, member := range file.Members {
		// Unwrap Membership
		var node ast.Node
		if membership, ok := member.(*ast.Membership); ok {
			node = membership.Member
		} else {
			node = member
		}
		
		// Check if it's a Package
		if pkg, ok := node.(*ast.Package); ok {
			for _, pkgMember := range pkg.Members {
				// Unwrap Membership in package
				var pkgNode ast.Node
				if membership, ok := pkgMember.(*ast.Membership); ok {
					pkgNode = membership.Member
				} else {
					pkgNode = pkgMember
				}
				
				switch n := pkgNode.(type) {
				case *ast.Definition:
					switch n.Kind {
					case ast.DefCalc:
						calcCount++
					case ast.DefConstraint:
						constraintCount++
					case ast.DefRequirement:
						requirementCount++
					case ast.DefAction:
						actionCount++
					case ast.DefState:
						stateCount++
					}
				case *ast.Usage:
					switch n.Kind {
					case ast.UsageCalc:
						calcCount++
					case ast.UsageConstraint:
						constraintCount++
					case ast.UsageRequirement:
						requirementCount++
					case ast.UsageAction:
						actionCount++
					case ast.UsageState:
						stateCount++
					}
				}
			}
		}
	}
	
	fmt.Println("\nBehavioral elements found:")
	fmt.Printf("  Calculations:  %d\n", calcCount)
	fmt.Printf("  Constraints:   %d\n", constraintCount)
	fmt.Printf("  Requirements:  %d\n", requirementCount)
	fmt.Printf("  Actions:       %d\n", actionCount)
	fmt.Printf("  States:        %d\n", stateCount)
}
