package parser

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

// parseOneMember parses src and returns the single unwrapped top-level member.
func parseOneMember(t *testing.T, src string) ast.Node {
	t.Helper()
	p := New(source.New("<t>", []byte(src)))
	root := p.ParseFile()
	if len(root.Members) != 1 {
		t.Fatalf("%q: expected 1 member, got %d", src, len(root.Members))
	}
	m := root.Members[0]
	if mem, ok := m.(*ast.Membership); ok {
		return mem.Member
	}
	return m
}

func TestParseDefinitionDispatch(t *testing.T) {
	def, ok := parseOneMember(t, "part def Vehicle;").(*ast.Definition)
	if !ok {
		t.Fatalf("expected *ast.Definition")
	}
	if def.Kind != ast.DefPart || def.Ident.Name != "Vehicle" {
		t.Fatalf("got kind=%v name=%q", def.Kind, def.Ident.Name)
	}
	if def.HasBody {
		t.Fatalf("expected no body")
	}
}

func TestParseAttributeDefAndModifiers(t *testing.T) {
	def := parseOneMember(t, "abstract variation attribute def Mass;").(*ast.Definition)
	if def.Kind != ast.DefAttribute || !def.IsAbstract || !def.IsVariation {
		t.Fatalf("got kind=%v abstract=%v variation=%v", def.Kind, def.IsAbstract, def.IsVariation)
	}
}

func TestParseUsageDispatch(t *testing.T) {
	u, ok := parseOneMember(t, "part engine;").(*ast.Usage)
	if !ok {
		t.Fatalf("expected *ast.Usage")
	}
	if u.Kind != ast.UsagePart || u.Ident.Name != "engine" {
		t.Fatalf("got kind=%v name=%q", u.Kind, u.Ident.Name)
	}
}

func TestParseUsageModifiers(t *testing.T) {
	u := parseOneMember(t, "ref in composite derived ordered nonunique part p;").(*ast.Usage)
	if !u.IsReference || u.Direction != ast.DirIn || !u.IsComposite || !u.IsDerived || !u.IsOrdered || !u.IsNonunique {
		t.Fatalf("modifier flags wrong: %+v", u)
	}
	if u.Kind != ast.UsagePart {
		t.Fatalf("got kind=%v", u.Kind)
	}
}

func TestParseAnonymousUsage(t *testing.T) {
	u := parseOneMember(t, "attribute;").(*ast.Usage)
	if u.Kind != ast.UsageAttribute || u.Ident.Name != "" {
		t.Fatalf("got kind=%v name=%q", u.Kind, u.Ident.Name)
	}
}
