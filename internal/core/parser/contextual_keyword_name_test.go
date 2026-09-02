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

	// `derived var feature` — the spelling the Kernel Semantic Library uses —
	// reads the same way: the modifier is kept and the prefix is still recorded,
	// so both spellings describe the feature as variable.
	root = parseClean(t, "package P { datatype D { derived var feature a : D[1]; } }")
	pkg = root.(*ast.RootNamespace).Members[0].(*ast.Membership).Member.(*ast.Package)
	dt := pkg.Members[0].(*ast.Membership).Member.(*ast.Usage)
	u = dt.Members[0].(*ast.Membership).Member.(*ast.Usage)
	if !u.IsDerived || u.PrefixKeyword != "var" || u.Ident.Name != "a" {
		t.Errorf("derived = %v, prefix = %q, name = %q; want true, \"var\", \"a\"", u.IsDerived, u.PrefixKeyword, u.Ident.Name)
	}
}

// `chain` is the feature chain modifier only when a name follows it; before
// `=`, `:`, `;`, `[` or a keyword that continues the declaration it names the
// feature, on either side of the kind keyword.
func TestParseChainIsANameBeforeAnythingButAName(t *testing.T) {
	for _, src := range []string{
		"package P { attribute chain = 1; attribute pt = chain + 1; }",
		"package P { attribute chain : Integer; }",
		"package P { part def D { attribute chain; attribute chain[*]; } }",
		"package P { part def D { ref chain :> other; attribute other; } }",
		"package P { action a { in chain : Integer; assign chain := 1; } }",
		"package P { attribute chain default 1; }",
		"package P { attribute chain defined by Integer; }",
		"package P { attribute chain subsets other; attribute other; }",
		"package P { attribute chain redefines other; attribute other; }",
		"package P { metadata chain about other; attribute other; }",
	} {
		root := parseClean(t, src)
		pkg := root.(*ast.RootNamespace).Members[0].(*ast.Membership).Member.(*ast.Package)
		u := firstUsage(t, pkg.Members[0].(*ast.Membership).Member)
		if u.Ident.Name != "chain" || u.IsChain {
			t.Errorf("%s\ndeclared %q chain=%t, want the feature named chain", src, u.Ident.Name, u.IsChain)
		}
	}

	for _, src := range []string{
		"package P { attribute chain x : Integer; }",
		"package P { ref chain x : Integer; }",
		"package P { ref chain 'x y' : Integer; }",
		"package P { ref chain <s> x : Integer; }",
	} {
		root := parseClean(t, src)
		pkg := root.(*ast.RootNamespace).Members[0].(*ast.Membership).Member.(*ast.Package)
		u := firstUsage(t, pkg.Members[0].(*ast.Membership).Member)
		if u.Ident.Name == "chain" || !u.IsChain {
			t.Errorf("%s\ndeclared %q chain=%t, want the modifier and the name after it", src, u.Ident.Name, u.IsChain)
		}
	}
}

// Whether a keyword after `chain` names the declaration is the declaration
// kind's call, the same one parseUsageIdentification makes: `do` names a step
// and nothing else, `about` names nothing in a metadata usage.
func TestKeywordNamesUsageFollowsTheKind(t *testing.T) {
	for _, tt := range []struct {
		kind ast.UsageKind
		kw   string
		want bool
	}{
		{ast.UsageStep, "do", true},
		{ast.UsageAttribute, "do", false},
		{ast.UsageAction, "do", false},
		{ast.UsageMetadata, "about", false},
		{ast.UsageAttribute, "about", true},
		{ast.UsageAttribute, "default", false},
		{ast.UsageStep, "default", false},
		{ast.UsageAttribute, "item", true},
	} {
		if got := keywordNamesUsage(tt.kind, tt.kw); got != tt.want {
			t.Errorf("keywordNamesUsage(%v, %q) = %t, want %t", tt.kind, tt.kw, got, tt.want)
		}
	}
}

// firstUsage returns n when it is a usage, else the first usage n declares.
func firstUsage(t *testing.T, n ast.Node) *ast.Usage {
	t.Helper()
	switch d := n.(type) {
	case *ast.Usage:
		if len(d.Members) > 0 {
			if u, ok := d.Members[0].(*ast.Membership).Member.(*ast.Usage); ok {
				return u
			}
		}
		return d
	case *ast.Definition:
		if u, ok := d.Members[0].(*ast.Membership).Member.(*ast.Usage); ok {
			return u
		}
	}
	t.Fatalf("member parsed to %T, want a usage", n)
	return nil
}
