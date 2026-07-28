package parser

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
)

var _ = ast.Node(nil)

func TestParseQualifiedNameSimple(t *testing.T) {
	p := newParser("A::B::C")
	qn := p.parseQualifiedName()
	if qn == nil {
		t.Fatal("got nil qualified name")
	}
	if qn.Global {
		t.Error("expected not global")
	}
	if len(qn.Parts) != 3 {
		t.Fatalf("got %d parts, want 3", len(qn.Parts))
	}
	if qn.Parts[0].Text != "A" || qn.Parts[1].Text != "B" || qn.Parts[2].Text != "C" {
		t.Errorf("parts = %q/%q/%q", qn.Parts[0].Text, qn.Parts[1].Text, qn.Parts[2].Text)
	}
	if len(p.Diagnostics) != 0 {
		t.Errorf("got %d diagnostics, want 0", len(p.Diagnostics))
	}
}

func TestParseQualifiedNameGlobal(t *testing.T) {
	p := newParser("$::Root::X")
	qn := p.parseQualifiedName()
	if qn == nil {
		t.Fatal("got nil qualified name")
	}
	if !qn.Global {
		t.Error("expected global")
	}
	if len(qn.Parts) != 2 {
		t.Fatalf("got %d parts, want 2", len(qn.Parts))
	}
	if qn.Parts[0].Text != "Root" || qn.Parts[1].Text != "X" {
		t.Errorf("parts = %q/%q", qn.Parts[0].Text, qn.Parts[1].Text)
	}
}

func TestParseQualifiedNameUnrestricted(t *testing.T) {
	p := newParser("'my name'::B")
	qn := p.parseQualifiedName()
	if qn == nil {
		t.Fatal("got nil qualified name")
	}
	if len(qn.Parts) != 2 {
		t.Fatalf("got %d parts, want 2", len(qn.Parts))
	}
	if qn.Parts[0].Text != "'my name'" {
		t.Errorf("part0 = %q, want quotes kept", qn.Parts[0].Text)
	}
}

func TestParseQualifiedNameNoName(t *testing.T) {
	p := newParser(";")
	qn := p.parseQualifiedName()
	if qn != nil {
		t.Errorf("expected nil, got %+v", qn)
	}
	if len(p.Diagnostics) != 1 {
		t.Errorf("got %d diagnostics, want 1", len(p.Diagnostics))
	}
}

func TestParseIdentificationShortAndName(t *testing.T) {
	p := newParser("<v1> Vehicle")
	id := p.parseIdentification()
	if id.ShortName != "v1" {
		t.Errorf("ShortName = %q, want v1", id.ShortName)
	}
	if id.Name != "Vehicle" {
		t.Errorf("Name = %q, want Vehicle", id.Name)
	}
}

func TestParseIdentificationNameOnly(t *testing.T) {
	p := newParser("Vehicle")
	id := p.parseIdentification()
	if id.ShortName != "" {
		t.Errorf("ShortName = %q, want empty", id.ShortName)
	}
	if id.Name != "Vehicle" {
		t.Errorf("Name = %q, want Vehicle", id.Name)
	}
}

func TestParseIdentificationEmpty(t *testing.T) {
	p := newParser("{")
	id := p.parseIdentification()
	if id.Name != "" || id.ShortName != "" {
		t.Errorf("expected empty id, got %+v", id)
	}
}

func TestParseFileEmpty(t *testing.T) {
	p := newParser("")
	root := p.ParseFile()
	if root == nil || len(root.Members) != 0 {
		t.Fatalf("root = %+v", root)
	}
	if len(p.Diagnostics) != 0 {
		t.Fatalf("diags = %+v", p.Diagnostics)
	}
}

func TestParseFileVisibilityPrefix(t *testing.T) {
	p := newParser("private package P;")
	root := p.ParseFile()
	if len(root.Members) != 1 {
		t.Fatalf("members = %+v", root.Members)
	}
	m, ok := root.Members[0].(*ast.Membership)
	if !ok {
		t.Fatalf("member type = %T", root.Members[0])
	}
	if m.Visibility != ast.VisibilityPrivate {
		t.Fatalf("vis = %v", m.Visibility)
	}
	if _, ok := m.Member.(*ast.Package); !ok {
		t.Fatalf("inner type = %T", m.Member)
	}
}

func TestParseFileUnknownKeywordErrorNode(t *testing.T) {
	p := newParser("part def Vehicle;")
	root := p.ParseFile()
	if len(root.Members) != 1 {
		t.Fatalf("members = %+v", root.Members)
	}
	if _, ok := root.Members[0].(*ast.ErrorNode); !ok {
		t.Fatalf("expected ErrorNode, got %T", root.Members[0])
	}
	if len(p.Diagnostics) == 0 {
		t.Fatal("expected a diagnostic")
	}
}

func TestParseNamespaceBodyMembers(t *testing.T) {
	p := newParser("package Outer { package Inner; }")
	root := p.ParseFile()
	m := root.Members[0].(*ast.Membership)
	outer := m.Member.(*ast.Package)
	if !outer.HasBody || len(outer.Members) != 1 {
		t.Fatalf("outer = %+v", outer)
	}
	inner := outer.Members[0].(*ast.Membership).Member.(*ast.Package)
	if inner.Ident.Name != "Inner" {
		t.Fatalf("inner = %+v", inner)
	}
}

func TestParsePackageEmptyBody(t *testing.T) {
	p := newParser("package P { }")
	root := p.ParseFile()
	pkg := root.Members[0].(*ast.Membership).Member.(*ast.Package)
	if pkg.Ident.Name != "P" || !pkg.HasBody || len(pkg.Members) != 0 {
		t.Fatalf("pkg = %+v", pkg)
	}
}

func TestParsePackageSemicolon(t *testing.T) {
	p := newParser("package P;")
	root := p.ParseFile()
	pkg := root.Members[0].(*ast.Membership).Member.(*ast.Package)
	if pkg.HasBody {
		t.Fatalf("expected no body: %+v", pkg)
	}
}

func TestParseLibraryPackage(t *testing.T) {
	p := newParser("standard library package Base;")
	root := p.ParseFile()
	pkg := root.Members[0].(*ast.Membership).Member.(*ast.Package)
	if !pkg.IsLibrary || !pkg.IsStandard {
		t.Fatalf("flags = %+v", pkg)
	}
}

func TestParseNamespaceDecl(t *testing.T) {
	p := newParser("namespace N { }")
	root := p.ParseFile()
	ns := root.Members[0].(*ast.Membership).Member.(*ast.Namespace)
	if !ns.HasBody {
		t.Fatalf("ns = %+v", ns)
	}
}

func TestParsePrefixMetadata(t *testing.T) {
	p := newParser("#Meta package P;")
	root := p.ParseFile()
	pkg := root.Members[0].(*ast.Membership).Member.(*ast.Package)
	if len(pkg.Prefixes) != 1 || pkg.Prefixes[0].Type == nil {
		t.Fatalf("prefixes = %+v", pkg.Prefixes)
	}
	if pkg.Prefixes[0].Type.Parts[0].Text != "Meta" {
		t.Fatalf("prefix type = %+v", pkg.Prefixes[0].Type)
	}
}

func importOf(t *testing.T, src string) *ast.Import {
	t.Helper()
	p := newParser(src)
	root := p.ParseFile()
	if len(root.Members) != 1 {
		t.Fatalf("members = %+v (diags %+v)", root.Members, p.Diagnostics)
	}
	imp, ok := root.Members[0].(*ast.Import)
	if !ok {
		t.Fatalf("member type = %T", root.Members[0])
	}
	return imp
}

func TestParseMembershipImport(t *testing.T) {
	imp := importOf(t, "import A::B;")
	if imp.Kind != ast.ImportMembership || imp.IsRecursive || imp.IsAll {
		t.Fatalf("imp = %+v", imp)
	}
	if imp.Imported == nil || len(imp.Imported.Parts) != 2 {
		t.Fatalf("imported = %+v", imp.Imported)
	}
}

func TestParseNamespaceImportStar(t *testing.T) {
	imp := importOf(t, "import A::B::*;")
	if imp.Kind != ast.ImportNamespace || imp.IsRecursive {
		t.Fatalf("imp = %+v", imp)
	}
}

func TestParseRecursiveImport(t *testing.T) {
	imp := importOf(t, "import A::B::**;")
	if imp.Kind != ast.ImportNamespace || !imp.IsRecursive {
		t.Fatalf("imp = %+v", imp)
	}
}

func TestParseStarThenRecursiveImport(t *testing.T) {
	imp := importOf(t, "import A::*::**;")
	if imp.Kind != ast.ImportNamespace || !imp.IsRecursive {
		t.Fatalf("imp = %+v", imp)
	}
}

func TestParseImportAll(t *testing.T) {
	imp := importOf(t, "import all A::B::*;")
	if !imp.IsAll || imp.Kind != ast.ImportNamespace {
		t.Fatalf("imp = %+v", imp)
	}
}

func TestParseImportPublicPrefix(t *testing.T) {
	p := newParser("public import A::B;")
	root := p.ParseFile()
	imp := root.Members[0].(*ast.Import)
	if imp.Visibility != ast.VisibilityPublic {
		t.Fatalf("vis = %v", imp.Visibility)
	}
}
