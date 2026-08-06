package parser

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestParseKeywordAsName(t *testing.T) {
	src := source.New("test.sysml", []byte(`
		part def Item {
			part done: Item;
		}
	`))
	p := New(src)
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		for _, d := range p.Diagnostics {
			t.Errorf("%s", d.Message)
		}
		t.FailNow()
	}

	// root -> membership -> definition -> membership -> usage
	mem1 := root.Members[0].(*ast.Membership)
	def := mem1.Member.(*ast.Definition)
	mem2 := def.Members[0].(*ast.Membership)
	usage := mem2.Member.(*ast.Usage)

	if usage.Ident.Name != "done" {
		t.Errorf("Expected usage name 'done', got %q", usage.Ident.Name)
	}
}

// A keyword that names a declaration whose kind is also a keyword keeps its
// name: `action flow { ... }` is an action named flow, and dropping the name
// would leave the declaration anonymous and unreferenceable.
func TestParseKeywordAsNameAfterKindKeyword(t *testing.T) {
	tests := []struct {
		src  string
		kind ast.UsageKind
		name string
	}{
		{"package P { action flow { assign x := 1; } }", ast.UsageAction, "flow"},
		{"package P { action flow; }", ast.UsageAction, "flow"},
		{"package P { attribute item : Integer = 1; }", ast.UsageAttribute, "item"},
		{"package P { part state; }", ast.UsagePart, "state"},
	}
	for _, tt := range tests {
		t.Run(tt.name+"/"+tt.kind.String(), func(t *testing.T) {
			p := New(source.New("test.sysml", []byte(tt.src)))
			root := p.ParseFile()
			if len(p.Diagnostics) > 0 {
				t.Fatalf("parse errors: %v", p.Diagnostics)
			}
			pkg := root.Members[0].(*ast.Membership).Member.(*ast.Package)
			usage, ok := pkg.Members[0].(*ast.Membership).Member.(*ast.Usage)
			if !ok {
				t.Fatalf("%s parsed to %T, want a usage", tt.src, pkg.Members[0].(*ast.Membership).Member)
			}
			if usage.Ident.Name != tt.name {
				t.Errorf("%s declared %q, want %q", tt.src, usage.Ident.Name, tt.name)
			}
			if usage.Kind != tt.kind {
				t.Errorf("%s has kind %v, want %v", tt.src, usage.Kind, tt.kind)
			}
		})
	}
}

// The converse: a keyword before a kind keyword qualifies the kind, so the name
// that follows the kind is the declaration's, and a declaration with no name
// stays anonymous rather than being named after its own kind.
func TestParseKeywordBeforeKindKeywordIsNotAName(t *testing.T) {
	tests := []struct {
		src  string
		name string
	}{
		{"package P { individual item def Alice; }", "Alice"},
		{"package P { part def S { var feature x : Integer; } }", "x"},
		{"package P { part def S { assert constraint { 1 > 0 } } }", ""},
	}
	for _, tt := range tests {
		t.Run(tt.src, func(t *testing.T) {
			p := New(source.New("test.sysml", []byte(tt.src)))
			root := p.ParseFile()
			if len(p.Diagnostics) > 0 {
				t.Fatalf("parse errors: %v", p.Diagnostics)
			}
			if got := lastDeclaredName(root); got != tt.name {
				t.Errorf("%s declared %q, want %q", tt.src, got, tt.name)
			}
		})
	}
}

// lastDeclaredName returns the name of the innermost declaration in the tree.
func lastDeclaredName(node ast.Node) string {
	name := ""
	var members []ast.Node
	switch n := node.(type) {
	case *ast.RootNamespace:
		members = n.Members
	case *ast.Membership:
		return lastDeclaredName(n.Member)
	case *ast.Package:
		members = n.Members
	case *ast.Definition:
		name, members = n.Ident.Name, n.Members
	case *ast.Usage:
		name, members = n.Ident.Name, n.Members
	}
	for _, member := range members {
		switch member.(type) {
		case *ast.Membership, *ast.Definition, *ast.Usage:
			return lastDeclaredName(member)
		}
	}
	return name
}
