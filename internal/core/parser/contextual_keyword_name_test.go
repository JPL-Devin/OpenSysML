package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// parseClean parses src and fails when anything is reported: these shapes are
// well-formed, so neither an error nor a reserved-keyword warning belongs here.
func parseClean(t *testing.T, src string) ast.Node {
	t.Helper()
	p := New(source.New("test.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Errorf("%s\nerrors = %v, want none", src, p.Diagnostics)
	}
	if len(p.Warnings) != 0 {
		t.Errorf("%s\nwarnings = %v, want none", src, p.Warnings)
	}
	return root
}

// `on` is a name in every position: the word appears in none of the pilot
// implementation's grammars, while the OMG training corpus declares a state
// named `on` and names it as a transition target.
func TestParseOnIsAName(t *testing.T) {
	for _, src := range []string{
		"package P { state def S { state on; accept Sig then on; } }",
		"package P { state def S { state on { entry action light; } accept Sig then on; } }",
		// A source named `on` and the trigger after it.
		"package P { state def S { state on; state off; transition first on accept Sig then off; } }",
		"package P { part def D { attribute on : Boolean; attribute lit = on and not on; } }",
		"package P { part def D { port on : PortDef; } port def PortDef; }",
		"package P { action a { in on : Integer; } }",
		"package P { part def D { part on : On; } part def On; }",
	} {
		parseClean(t, src)
	}
}

// `var` marks a variable feature (KerML.xtext BasicFeaturePrefix,
// `isVariable ?= 'var'`) and is a name everywhere else, as SysML's own grammar
// — which has no `var` at all — implies.
func TestParseVarIsANameOutsideThePrefix(t *testing.T) {
	for _, src := range []string{
		"package P { attribute var : Integer; }",
		"package P { part def D { attribute var : Integer; attribute next = var + 1; } }",
		"package P { action a { attribute var : Integer; assign var := 1; } }",
		"package P { calc def C { in var : Integer; return x : Integer; } }",
		"package P { part def D { part var : Var; } part def Var; }",
	} {
		parseClean(t, src)
	}
}

// The prefix keeps its meaning: `var` before a kind keyword qualifies the
// declaration that follows rather than naming it.
func TestParseVarPrefixQualifiesTheKind(t *testing.T) {
	root := parseClean(t, "package P { part def D { var attribute total : Integer; } }")
	pkg := root.(*ast.RootNamespace).Members[0].(*ast.Membership).Member.(*ast.Package)
	def := pkg.Members[0].(*ast.Membership).Member.(*ast.Definition)
	u, ok := def.Members[0].(*ast.Membership).Member.(*ast.Usage)
	if !ok {
		t.Fatalf("member parsed to %T, want a usage", def.Members[0].(*ast.Membership).Member)
	}
	if u.Ident.Name != "total" {
		t.Errorf("declared %q, want the name after the prefix", u.Ident.Name)
	}
	if u.PrefixKeyword != "var" {
		t.Errorf("prefix = %q, want \"var\"", u.PrefixKeyword)
	}

	// `var feature` — the spelling the Kernel Semantic Library uses — reads the
	// same way, with the modifiers before it kept.
	parseClean(t, "package P { datatype D { derived var feature a : D[1]; } }")
}
