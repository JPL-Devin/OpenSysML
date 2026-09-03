package parser

import (
	"strings"
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

	t.Run("keyworded_anonymous_enum_value", func(t *testing.T) {
		pkg := parsePkg(t, "package B { enum def S { enum = 60.0; enum := 70.0; } }")
		def := pkg.Members[0].(*ast.Membership).Member.(*ast.Definition)
		for i := range def.Members {
			u := usageAt(t, def.Members, i)
			if u.Ident.Name != "" || u.Kind != ast.UsageEnumeration || u.Value == nil {
				t.Errorf("member %d = %#v, want anonymous enum with a value", i, u)
			}
		}
	})

	t.Run("named_enum_values", func(t *testing.T) {
		pkg := parsePkg(t, "package B { enum def S { enum red; enum x = 60.0; } }")
		def := pkg.Members[0].(*ast.Membership).Member.(*ast.Definition)
		for i, want := range []string{"red", "x"} {
			u := usageAt(t, def.Members, i)
			if u.Ident.Name != want {
				t.Errorf("member %d name = %q, want %q", i, u.Ident.Name, want)
			}
		}
	})

	t.Run("typed_enum_values", func(t *testing.T) {
		pkg := parsePkg(t, "package B { enum def L :> ScalarValues::Natural { uncl : L = 0; private conf :> uncl; x [1]; } }")
		def := pkg.Members[0].(*ast.Membership).Member.(*ast.Definition)
		if len(def.Members) != 3 {
			t.Fatalf("members = %d, want 3", len(def.Members))
		}
		for i, want := range []string{"uncl", "conf", "x"} {
			u := usageAt(t, def.Members, i)
			if u.Ident.Name != want || u.Kind != ast.UsageEnumeration {
				t.Errorf("member %d = %q kind %v, want enumerated value %q", i, u.Ident.Name, u.Kind, want)
			}
		}
		if u := usageAt(t, def.Members, 0); len(u.Relationships) != 1 || u.Relationships[0].Kind != ast.RelTyping || u.Value == nil {
			t.Errorf("uncl = %#v, want a typing and a value", u)
		}
		if u := usageAt(t, def.Members, 1); u.Visibility != ast.VisibilityPrivate || len(u.Relationships) != 1 || u.Relationships[0].Kind != ast.RelSubsets {
			t.Errorf("conf = %#v, want a private subsetting", u)
		}
	})

	t.Run("named_enum_values_with_default_or_initial_values", func(t *testing.T) {
		src := "package B { enum def S { a := 1; b default = 2; c default := 3; d default 4; enum e default = 5; } }"
		pkg := parsePkg(t, src)
		def := pkg.Members[0].(*ast.Membership).Member.(*ast.Definition)
		if len(def.Members) != 5 {
			t.Fatalf("members = %d, want 5", len(def.Members))
		}
		for i, want := range []struct{ name, op string }{
			{"a", ":="}, {"b", "default ="}, {"c", "default :="}, {"d", "default"}, {"e", "default ="},
		} {
			u := usageAt(t, def.Members, i)
			op := src[u.ValueOperatorSpan.Offset:u.ValueOperatorSpan.End()]
			if u.Kind != ast.UsageEnumeration || u.Ident.Name != want.name || u.Value == nil || op != want.op {
				t.Errorf("member %d = kind %v %q op %q value %v, want enumerated value %q with %q", i, u.Kind, u.Ident.Name, op, u.Value != nil, want.name, want.op)
			}
		}
	})

	t.Run("short_named_and_nameless_enum_values", func(t *testing.T) {
		pkg := parsePkg(t, "package B { attribute def D { attribute n; } enum def S :> D { <s1> a : S; <s2>; <s3> = 1; : S; :>> n = 3; [1]; public <s4> c; private : S; } }")
		def := pkg.Members[1].(*ast.Membership).Member.(*ast.Definition)
		if len(def.Members) != 8 {
			t.Fatalf("members = %d, want 8", len(def.Members))
		}
		for i, want := range []struct{ short, name string }{{"s1", "a"}, {"s2", ""}, {"s3", ""}, {"", ""}, {"", ""}, {"", ""}, {"s4", "c"}, {"", ""}} {
			u := usageAt(t, def.Members, i)
			if u.Kind != ast.UsageEnumeration || u.Ident.ShortName != want.short || u.Ident.Name != want.name {
				t.Errorf("member %d = kind %v <%q> %q, want enumerated value <%q> %q", i, u.Kind, u.Ident.ShortName, u.Ident.Name, want.short, want.name)
			}
		}
		if u := usageAt(t, def.Members, 2); u.Value == nil {
			t.Errorf("<s3> = 1 has no value")
		}
		if u := usageAt(t, def.Members, 3); len(u.Relationships) != 1 || u.Relationships[0].Kind != ast.RelTyping {
			t.Errorf(": S = %#v, want a typing", u)
		}
		if u := usageAt(t, def.Members, 4); len(u.Relationships) != 1 || u.Relationships[0].Kind != ast.RelRedefines || u.Value == nil {
			t.Errorf(":>> n = 3 = %#v, want a redefinition and a value", u)
		}
		if u := usageAt(t, def.Members, 5); u.Multiplicity == nil {
			t.Errorf("[1] has no multiplicity")
		}
		if u := usageAt(t, def.Members, 7); u.Visibility != ast.VisibilityPrivate {
			t.Errorf("private : S visibility = %v, want private", u.Visibility)
		}
	})

	t.Run("metadata_prefixed_enum_values", func(t *testing.T) {
		pkg := parsePkg(t, "package B { metadata def M; enum def S { #M a; #M b : S; #M enum c; private #M e; #M <f> ff; #M g [1]; #M h :> a; #M = 1; #M enum := 2; #M; #B::M { doc /* d */ } #M k default = 3; } }")
		def := pkg.Members[1].(*ast.Membership).Member.(*ast.Definition)
		if len(def.Members) != 12 {
			t.Fatalf("members = %d, want 12", len(def.Members))
		}
		for i, want := range []struct{ short, name string }{{"", "a"}, {"", "b"}, {"", "c"}, {"", "e"}, {"f", "ff"}, {"", "g"}, {"", "h"}, {"", ""}, {"", ""}, {"", ""}, {"", ""}, {"", "k"}} {
			u := usageAt(t, def.Members, i)
			if u.Kind != ast.UsageEnumeration || u.Ident.ShortName != want.short || u.Ident.Name != want.name {
				t.Errorf("member %d = kind %v <%q> %q, want enumerated value <%q> %q", i, u.Kind, u.Ident.ShortName, u.Ident.Name, want.short, want.name)
			}
			if len(u.Prefixes) != 1 || ast.SimpleName(u.Prefixes[0].Type) != "M" {
				t.Errorf("member %d prefixes = %#v, want one #M", i, u.Prefixes)
			}
		}
		if u := usageAt(t, def.Members, 1); len(u.Relationships) != 1 || u.Relationships[0].Kind != ast.RelTyping {
			t.Errorf("#M b : S = %#v, want a typing", u)
		}
		if u := usageAt(t, def.Members, 2); u.Keyword != "enum" {
			t.Errorf("#M enum c keyword = %q, want enum", u.Keyword)
		}
		if u := usageAt(t, def.Members, 3); u.Visibility != ast.VisibilityPrivate {
			t.Errorf("private #M e visibility = %v, want private", u.Visibility)
		}
		if u := usageAt(t, def.Members, 5); u.Multiplicity == nil {
			t.Errorf("#M g [1] has no multiplicity")
		}
		if u := usageAt(t, def.Members, 6); len(u.Relationships) != 1 || u.Relationships[0].Kind != ast.RelSubsets {
			t.Errorf("#M h :> a = %#v, want a subsetting", u)
		}
		for _, i := range []int{7, 8, 11} {
			if u := usageAt(t, def.Members, i); u.Value == nil {
				t.Errorf("member %d has no value", i)
			}
		}
		if u := usageAt(t, def.Members, 10); !u.HasBody || ast.QualifiedText(u.Prefixes[0].Type) != "B::M" {
			t.Errorf("#B::M { doc } = %#v, want a body and a qualified prefix", u)
		}
	})

	t.Run("globally_qualified_metadata_prefixed_enum_values", func(t *testing.T) {
		pkg := parsePkg(t, "package B { metadata def M; enum def S { #$::B::M a; #$::B::M = 1; #$::B::M #M b; #M #$::B::M c; #$::B::M d : S = 2; #$::B::M; private #$::B::M enum := 3; #$::B::M k default = 4; } }")
		def := pkg.Members[1].(*ast.Membership).Member.(*ast.Definition)
		if len(def.Members) != 8 {
			t.Fatalf("members = %d, want 8", len(def.Members))
		}
		for i, want := range []struct {
			name     string
			prefixes []string
		}{
			{"a", []string{"$::B::M"}}, {"", []string{"$::B::M"}}, {"b", []string{"$::B::M", "M"}}, {"c", []string{"M", "$::B::M"}},
			{"d", []string{"$::B::M"}}, {"", []string{"$::B::M"}}, {"", []string{"$::B::M"}}, {"k", []string{"$::B::M"}},
		} {
			u := usageAt(t, def.Members, i)
			if u.Kind != ast.UsageEnumeration || u.Ident.Name != want.name {
				t.Errorf("member %d = kind %v %q, want enumerated value %q", i, u.Kind, u.Ident.Name, want.name)
			}
			var got []string
			for _, pre := range u.Prefixes {
				got = append(got, ast.QualifiedText(pre.Type))
			}
			if strings.Join(got, " ") != strings.Join(want.prefixes, " ") {
				t.Errorf("member %d prefixes = %v, want %v", i, got, want.prefixes)
			}
			if u.Prefixes[0].Type.Global != (want.prefixes[0] == "$::B::M") {
				t.Errorf("member %d first prefix global = %v", i, u.Prefixes[0].Type.Global)
			}
		}
		if u := usageAt(t, def.Members, 4); len(u.Relationships) != 1 || u.Relationships[0].Kind != ast.RelTyping || u.Value == nil {
			t.Errorf("#$::B::M d : S = 2 = %#v, want a typing and a value", u)
		}
		if u := usageAt(t, def.Members, 6); u.Visibility != ast.VisibilityPrivate || u.Keyword != "enum" || u.Value == nil {
			t.Errorf("private #$::B::M enum := 3 = %#v, want private enum with a value", u)
		}
	})

	t.Run("enum_value_trivia_stays_with_the_next_member", func(t *testing.T) {
		pkg := parsePkg(t, "package B { enum def S { a; /* c */ b; /* d */ doc /* e */ } }")
		def := pkg.Members[0].(*ast.Membership).Member.(*ast.Definition)
		for i, want := range []int{0, 1, 1} {
			if got := len(def.Members[i].(*ast.Membership).LeadingTrivia()); got != want {
				t.Errorf("member %d has %d leading trivia, want %d", i, got, want)
			}
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

	t.Run("keyword_led_result_expression", func(t *testing.T) {
		pkg := parsePkg(t, "package B { attribute def V { attribute m; } analysis def A { subject v : V; if v.m > 1.0 ? v.m else 1.0 } }")
		def := pkg.Members[1].(*ast.Membership).Member.(*ast.Definition)
		last := def.Members[len(def.Members)-1]
		op, ok := last.(*ast.OperatorExpr)
		if !ok || op.Operator != ast.OpConditional {
			t.Errorf("last member = %T, want *ast.OperatorExpr with OpConditional", last)
		}
	})

	t.Run("word_operator_result_expression", func(t *testing.T) {
		pkg := parsePkg(t, "package B { attribute def V; analysis def A { subject v : V; in small; in large; small and large } }")
		def := pkg.Members[1].(*ast.Membership).Member.(*ast.Definition)
		last := def.Members[len(def.Members)-1]
		op, ok := last.(*ast.OperatorExpr)
		if !ok || op.Operator != ast.OpConditionalAnd {
			t.Errorf("last member = %T, want *ast.OperatorExpr with OpConditionalAnd", last)
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
