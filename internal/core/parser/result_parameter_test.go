package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// TestResultParameterIsMarked covers the `return` forms that declare the result
// parameter of a calculation. All of them are out parameters, and only they are
// result parameters: the distinction decides whether the parameter is redefined
// by position or as the result (SysML v2 7.19.2).
func TestResultParameterIsMarked(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		param    string
		isResult bool
	}{
		{"typed result", "return average : Speed;", "average", true},
		{"untyped result", "return average;", "average", true},
		{"result with value", "return average = 1;", "average", true},
		{"out parameter", "out average : Speed;", "average", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := "package P { attribute def Speed; calc def C { in samples : Speed; " + tc.body + " } }"
			p := New(source.New("t.sysml", []byte(src)))
			root := p.ParseFile()
			if len(p.Diagnostics) != 0 {
				t.Fatalf("parse diagnostics: %v", p.Diagnostics)
			}
			usage := findUsageNamed(root, tc.param)
			if usage == nil {
				t.Fatalf("parameter %q not found", tc.param)
			}
			if usage.Direction != ast.DirOut {
				t.Fatalf("direction of %q = %v, want out", tc.param, usage.Direction)
			}
			if usage.IsResult != tc.isResult {
				t.Fatalf("IsResult of %q = %v, want %v", tc.param, usage.IsResult, tc.isResult)
			}
			if samples := findUsageNamed(root, "samples"); samples == nil || samples.IsResult {
				t.Fatalf("an in parameter must not be marked as the result parameter")
			}
		})
	}
}

// findUsageNamed returns the first usage named name in the namespace members of
// the tree rooted at node.
func findUsageNamed(node ast.Node, name string) *ast.Usage {
	var members []ast.Node
	switch v := node.(type) {
	case *ast.Membership:
		return findUsageNamed(v.Member, name)
	case *ast.RootNamespace:
		members = v.Members
	case *ast.Package:
		members = v.Members
	case *ast.Definition:
		members = v.Members
	case *ast.Usage:
		if v.Ident.Name == name {
			return v
		}
		members = v.Members
	default:
		return nil
	}
	for _, member := range members {
		if found := findUsageNamed(member, name); found != nil {
			return found
		}
	}
	return nil
}
