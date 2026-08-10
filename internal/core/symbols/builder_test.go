package symbols

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func build(t *testing.T, src string) *Scope {
	t.Helper()
	sf := source.New("test", []byte(src))
	p := parser.New(sf)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("unexpected parse diagnostics: %v", p.Diagnostics)
	}
	return Build(root)
}

func TestBuildTopLevelPackage(t *testing.T) {
	root := build(t, "package P;")
	sym, ok := root.LookupLocal("P")
	if !ok {
		t.Fatalf("P not found in root scope")
	}
	if sym.Kind != SymbolPackage {
		t.Fatalf("P kind = %v, want package", sym.Kind)
	}
	if _, isPkg := sym.Decl.(*ast.Package); !isPkg {
		t.Fatalf("P Decl type = %T, want *ast.Package", sym.Decl)
	}
}

func TestBuildNestedMembers(t *testing.T) {
	root := build(t, "package Outer { package Inner; namespace N; }")
	outer, ok := root.LookupLocal("Outer")
	if !ok {
		t.Fatalf("Outer not found")
	}
	outerScope := outer.Scope
	if outerScope == nil {
		t.Fatalf("Outer has no child scope")
	}
	if _, ok := outerScope.LookupLocal("Inner"); !ok {
		t.Fatalf("Inner not found in Outer scope")
	}
	nsym, ok := outerScope.LookupLocal("N")
	if !ok || nsym.Kind != SymbolNamespace {
		t.Fatalf("N not found as namespace in Outer scope")
	}
}

func TestBuildShortAndPrimaryNameKeys(t *testing.T) {
	root := build(t, "package <p> Primary;")
	for _, key := range []string{"p", "Primary"} {
		sym, ok := root.LookupLocal(key)
		if !ok {
			t.Fatalf("key %q not found", key)
		}
		if sym.Kind != SymbolPackage {
			t.Fatalf("key %q kind = %v, want package", key, sym.Kind)
		}
	}
	// Both keys must map to the same symbol.
	a, _ := root.LookupLocal("p")
	b, _ := root.LookupLocal("Primary")
	if a != b {
		t.Fatalf("short and primary keys map to different symbols")
	}
}

func TestBuildVisibilityCarried(t *testing.T) {
	root := build(t, "private package Secret;")
	sym, ok := root.LookupLocal("Secret")
	if !ok {
		t.Fatalf("Secret not found")
	}
	if sym.Visibility != ast.VisibilityPrivate {
		t.Fatalf("Secret visibility = %v, want private", sym.Visibility)
	}
}

func TestBuildAliasSymbol(t *testing.T) {
	root := build(t, "package P; alias A for P;")
	sym, ok := root.LookupLocal("A")
	if !ok || sym.Kind != SymbolAlias {
		t.Fatalf("alias A not found as alias symbol")
	}
}

func TestBuildErrorNodeSkipped(t *testing.T) {
	// Unknown declaration keyword yields an ErrorNode; builder must not panic
	// and must still register the good package.
	root := build(t, "package Good;")
	if _, ok := root.LookupLocal("Good"); !ok {
		t.Fatalf("Good not registered")
	}
}

func TestBuildDefinitionAndNestedUsages(t *testing.T) {
	src := "part def Car { part engine; attribute mass; }"
	root := parser.New(source.New("<t>", []byte(src))).ParseFile()
	scope := Build(root)

	syms := scope.LookupLocalAll("Car")
	if len(syms) != 1 {
		t.Fatalf("expected 1 Car symbol, got %d", len(syms))
	}
	car := syms[0]
	if car.Kind != SymbolPartDef {
		t.Fatalf("Car kind = %v, want SymbolPartDef", car.Kind)
	}
	if car.Scope == nil {
		t.Fatalf("Car should own a child scope")
	}
	if len(car.Scope.LookupLocalAll("engine")) != 1 {
		t.Fatalf("engine not registered in Car scope")
	}
	eng := car.Scope.LookupLocalAll("engine")[0]
	if eng.Kind != SymbolPartUsage {
		t.Fatalf("engine kind = %v, want SymbolPartUsage", eng.Kind)
	}
	if len(car.Scope.LookupLocalAll("mass")) != 1 {
		t.Fatalf("mass not registered in Car scope")
	}
	if car.Scope.LookupLocalAll("mass")[0].Kind != SymbolAttributeUsage {
		t.Fatalf("mass kind wrong")
	}
}

func TestBuildAttributeDefKind(t *testing.T) {
	root := parser.New(source.New("<t>", []byte("attribute def Mass;"))).ParseFile()
	scope := Build(root)
	syms := scope.LookupLocalAll("Mass")
	if len(syms) != 1 || syms[0].Kind != SymbolAttributeDef {
		t.Fatalf("Mass symbol wrong: %+v", syms)
	}
}

func TestBuildAnonymousUsageNotNamed(t *testing.T) {
	root := parser.New(source.New("<t>", []byte("part def Car { part; }"))).ParseFile()
	scope := Build(root)
	car := scope.LookupLocalAll("Car")[0]
	if len(car.Scope.LookupLocalAll("")) != 0 {
		t.Fatalf("anonymous usage should not be registered under empty name")
	}
	if len(car.Scope.Children()) != 1 {
		t.Fatalf("expected 1 child scope for the anonymous usage, got %d", len(car.Scope.Children()))
	}
}

// A feature declared without a name takes the name of the feature its single
// owned redefinition names (KerML 7.3.4.5): `part :>> engine;` declares
// `engine`, and a qualified target contributes its last segment.
func TestUnnamedRedefinitionTakesRedefinedName(t *testing.T) {
	root := build(t, `package P {
		part def Vehicle { part engine; attribute mass; }
		part v : Vehicle { part :>> engine; attribute :>> Vehicle::mass; }
	}`)

	pkg, _ := root.LookupLocal("P")
	v, _ := pkg.Scope.LookupLocal("v")
	for _, name := range []string{"engine", "mass"} {
		sym, ok := v.Scope.LookupLocal(name)
		if !ok {
			t.Fatalf("v members = %v, want %s", v.Scope.MemberNames(), name)
		}
		if !sym.EffectiveName {
			t.Fatalf("%s should be marked as an effective name, not a declared one", name)
		}
	}
}

// A declared name is the feature's name whatever it redefines.
func TestRedefinitionDoesNotOverrideDeclaredName(t *testing.T) {
	root := build(t, `package P {
		part def Vehicle { part engine; }
		part v : Vehicle { part motor :>> engine; }
	}`)

	pkg, _ := root.LookupLocal("P")
	v, _ := pkg.Scope.LookupLocal("v")
	if _, ok := v.Scope.LookupLocal("motor"); !ok {
		t.Fatalf("v members = %v, want motor", v.Scope.MemberNames())
	}
	if _, ok := v.Scope.LookupLocal("engine"); ok {
		t.Fatalf("v should not bind the redefined name when motor is declared")
	}
}

// A reference subsetting is the naming feature when a declaration has both
// (KerML 7.3.4.5 prefers the referenced feature over the redefined one).
func TestReferenceSubsettingOutranksRedefinitionAsNamingFeature(t *testing.T) {
	root := build(t, `package P {
		action takePicture;
		action def Camera { action shoot; }
		part camera : Camera { perform action :>> shoot references takePicture; }
	}`)

	pkg, _ := root.LookupLocal("P")
	camera, _ := pkg.Scope.LookupLocal("camera")
	if _, ok := camera.Scope.LookupLocal("takePicture"); !ok {
		t.Fatalf("camera members = %v, want takePicture", camera.Scope.MemberNames())
	}
	if _, ok := camera.Scope.LookupLocal("shoot"); ok {
		t.Fatalf("camera should be named by the referenced feature, not the redefined one")
	}
}

// Two redefinitions have no single naming feature, so the declaration stays
// anonymous rather than picking one of the redefined names.
func TestTwoRedefinitionsLeaveFeatureAnonymous(t *testing.T) {
	root := build(t, `package P {
		part def Vehicle { part engine; part motor; }
		part v : Vehicle { part :>> engine :>> motor; }
	}`)

	pkg, _ := root.LookupLocal("P")
	v, _ := pkg.Scope.LookupLocal("v")
	if names := v.Scope.MemberNames(); len(names) != 0 {
		t.Fatalf("v members = %v, want none", names)
	}
}

// The reference that named a feature is recorded, so resolving it can skip the
// name it gave away.
func TestNamingFeatureIsRecordedOnTheSymbol(t *testing.T) {
	root := build(t, `package P {
		part def Vehicle { part engine; }
		part v : Vehicle { part :>> engine; }
	}`)

	pkg, _ := root.LookupLocal("P")
	v, _ := pkg.Scope.LookupLocal("v")
	engine, _ := v.Scope.LookupLocal("engine")
	usage := engine.Decl.(*ast.Usage)
	if engine.NamingTarget != ast.AsQualifiedName(usage.Relationships[0].Target) {
		t.Fatalf("NamingTarget = %v, want the redefinition's target", engine.NamingTarget)
	}
}
