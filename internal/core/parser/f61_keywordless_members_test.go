package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// F61: keyword-less members. DefaultReferenceUsage (SysML.xtext:632) needs no
// kind keyword — a declaration may be only a value, a specialization, a
// redefinition or a typing; EnumeratedValue (:786) makes even the declaration
// optional; ResultExpressionMember (:1967) allows a trailing expression; and
// Comment (:86) makes its `comment` keyword optional before `locale`.
func TestF61KeywordlessMembers(t *testing.T) {
	parsePkg := func(t *testing.T, src string) *ast.Package {
		t.Helper()
		sf := source.New("f61.sysml", []byte(src))
		p := New(sf)
		root := p.ParseFile()
		if len(p.Diagnostics) > 0 {
			t.Fatalf("unexpected diagnostics: %v", p.Diagnostics)
		}
		return root.Members[0].(*ast.Membership).Member.(*ast.Package)
	}
	usageAt := func(t *testing.T, members []ast.Node, i int) *ast.Usage {
		t.Helper()
		u, ok := members[i].(*ast.Membership).Member.(*ast.Usage)
		if !ok {
			t.Fatalf("member %d = %T, want *ast.Usage", i, members[i])
		}
		return u
	}

	t.Run("value_only", func(t *testing.T) {
		pkg := parsePkg(t, "package B { T1 = 10.0; }")
		u := usageAt(t, pkg.Members, 0)
		if u.Ident.Name != "T1" || u.Value == nil {
			t.Errorf("got name=%q value=%v, want T1 with a value", u.Ident.Name, u.Value)
		}
	})

	t.Run("specialization_and_value", func(t *testing.T) {
		pkg := parsePkg(t, "package B { attribute s; attribute d; attribute v; dpv :> s = d / v; }")
		u := usageAt(t, pkg.Members, 3)
		if len(u.Relationships) != 1 || u.Relationships[0].Kind != ast.RelSubsets {
			t.Fatalf("relationships = %v, want one subsets", u.Relationships)
		}
		if u.Value == nil {
			t.Error("value = nil, want an expression")
		}
	})

	t.Run("typing_and_value", func(t *testing.T) {
		pkg := parsePkg(t, "package B { attribute def D; attribute km; kpl : D = km; }")
		u := usageAt(t, pkg.Members, 2)
		if len(u.Relationships) != 1 || u.Relationships[0].Kind != ast.RelTyping {
			t.Fatalf("relationships = %v, want one typing", u.Relationships)
		}
		if u.Value == nil {
			t.Error("value = nil, want an expression")
		}
	})

	t.Run("redefinition_only", func(t *testing.T) {
		pkg := parsePkg(t, "package B { attribute def D; attribute e; attribute def C { value :>> e : D; } }")
		def := pkg.Members[2].(*ast.Membership).Member.(*ast.Definition)
		u := usageAt(t, def.Members, 0)
		if u.Ident.Name != "value" {
			t.Errorf("name = %q, want value", u.Ident.Name)
		}
		if len(u.Relationships) != 2 || u.Relationships[0].Kind != ast.RelRedefines {
			t.Fatalf("relationships = %v, want redefines then typing", u.Relationships)
		}
	})

	t.Run("anonymous_enum_value", func(t *testing.T) {
		pkg := parsePkg(t, "package B { enum def S { = 60.0; = 70.0; } }")
		def := pkg.Members[0].(*ast.Membership).Member.(*ast.Definition)
		if len(def.Members) != 2 {
			t.Fatalf("members = %d, want 2", len(def.Members))
		}
		u := usageAt(t, def.Members, 0)
		if u.Kind != ast.UsageEnumeration || u.Value == nil {
			t.Errorf("got kind=%v value=%v, want enum with a value", u.Kind, u.Value)
		}
	})

	t.Run("result_expression_in_case_body", func(t *testing.T) {
		pkg := parsePkg(t, "package B { attribute def V { attribute m; } analysis def A { subject v : V; v.m } }")
		def := pkg.Members[1].(*ast.Membership).Member.(*ast.Definition)
		last := def.Members[len(def.Members)-1]
		if _, ok := last.(*ast.FeatureChainExpr); !ok {
			t.Errorf("last member = %T, want *ast.FeatureChainExpr", last)
		}
	})

	t.Run("anonymous_locale_comment", func(t *testing.T) {
		pkg := parsePkg(t, "package B { locale \"en_US\" /* body */ }")
		c, ok := pkg.Members[0].(*ast.Membership).Member.(*ast.Comment)
		if !ok {
			t.Fatalf("member = %T, want *ast.Comment", pkg.Members[0].(*ast.Membership).Member)
		}
		if c.Locale != "\"en_US\"" {
			t.Errorf("locale = %q, want \"en_US\"", c.Locale)
		}
	})
}

// Assignment statements keep their meaning: `a = e;` in a behavioral body is
// an assignment shorthand, not a keyword-less usage.
func TestF61AssignmentStaysAssignment(t *testing.T) {
	src := "package B { calc def C { in n : ScalarValues::Integer; out a : ScalarValues::Integer; a = n + 1; } }"
	sf := source.New("f61.sysml", []byte(src))
	p := New(sf)
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("unexpected diagnostics: %v", p.Diagnostics)
	}
	pkg := root.Members[0].(*ast.Membership).Member.(*ast.Package)
	def := pkg.Members[0].(*ast.Membership).Member.(*ast.Definition)
	found := false
	for _, m := range def.Members {
		if _, ok := m.(*ast.AssignmentActionNode); ok {
			found = true
		}
	}
	if !found {
		t.Error("no AssignmentActionNode in calc body; assignment was reparsed as a usage")
	}
}

// Malformed keyword-less members must produce diagnostics, never a panic.
func TestF61Negative(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"value_no_expr", "package B { T1 = ; }"},
		{"spec_no_target", "package B { x :> = 1.0; }"},
		{"enum_value_no_expr", "package B { enum def S { = ; } }"},
		{"locale_no_string", "package B { locale }"},
		{"locale_no_body", "package B { locale \"en_US\" }"},
		{"dangling_redef", "package B { attribute def C { value :>> ; } }"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sf := source.New("f61_neg.sysml", []byte(tt.input))
			p := New(sf)
			p.ParseFile()
			if len(p.Diagnostics) == 0 {
				t.Errorf("expected diagnostics for %q, got none", tt.input)
			}
		})
	}
}
