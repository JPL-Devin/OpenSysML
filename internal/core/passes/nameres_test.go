package passes

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

func nameresCtx(t *testing.T, name, src string) (*Context, *ast.RootNamespace) {
	t.Helper()
	sf := source.New(name, []byte(src))
	p := parser.New(sf)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("unexpected parse diagnostics: %+v", p.Diagnostics)
	}
	idx := symbols.NewIndexFromDoc(name, root)
	return NewContext(name, idx, nil), root
}

func TestNameResolutionPassReportsUnresolved(t *testing.T) {
	ctx, root := nameresCtx(t, "a.sysml", "package P { alias A for P::Missing; }")
	got := NameResolutionPass{}.Run(ctx, "a.sysml", root)
	if len(got) == 0 {
		t.Fatalf("expected an unresolved diagnostic, got none")
	}
	d := got[0]
	if d.Source != "name-resolution" || d.Code != "unresolved" || d.Severity != SeverityError {
		t.Fatalf("got %+v, want source=name-resolution code=unresolved severity=error", d)
	}
}

func TestNameResolutionPassCleanWhenAllResolve(t *testing.T) {
	ctx, root := nameresCtx(t, "a.sysml", "package P { namespace N; alias A for P::N; }")
	got := NameResolutionPass{}.Run(ctx, "a.sysml", root)
	if len(got) != 0 {
		t.Fatalf("got %+v, want no diagnostics", got)
	}
}

func TestNameResolutionPassLevel(t *testing.T) {
	if (NameResolutionPass{}).Level() != LevelNameResolution {
		t.Fatalf("Level() = %v, want LevelNameResolution", (NameResolutionPass{}).Level())
	}
}

// parseDoc parses src into a RootNamespace, failing on any parse diagnostic.
func parseDoc(t *testing.T, name, src string) *ast.RootNamespace {
	t.Helper()
	p := parser.New(source.New(name, []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("unexpected parse diagnostics in %s: %+v", name, p.Diagnostics)
	}
	return root
}

func TestNameResolutionPassReportsAmbiguous(t *testing.T) {
	// Two documents each declare a top-level package P, so a qualified
	// reference whose first segment is P has two global candidates and the
	// resolver reports an ambiguous reference.
	rootA := parseDoc(t, "a.sysml", "package P { namespace X; }")
	rootB := parseDoc(t, "b.sysml", "package P { namespace Y; }")
	rootC := parseDoc(t, "c.sysml", "package Q { alias A for P::X; }")

	idx := symbols.NewIndex()
	idx.AddDocument("a.sysml", rootA)
	idx.AddDocument("b.sysml", rootB)
	idx.AddDocument("c.sysml", rootC)

	ctx := NewContext("c.sysml", idx, nil)
	got := NameResolutionPass{}.Run(ctx, "c.sysml", rootC)
	if len(got) == 0 {
		t.Fatalf("expected an ambiguous diagnostic, got none")
	}
	d := got[0]
	if d.Source != "name-resolution" || d.Code != "ambiguous" || d.Severity != SeverityError {
		t.Fatalf("got %+v, want source=name-resolution code=ambiguous severity=error", d)
	}
}

// nameresDiags runs the pass over one document and returns its diagnostics.
func nameresDiags(t *testing.T, src string) []Diagnostic {
	t.Helper()
	ctx, root := nameresCtx(t, "a.sysml", src)
	return NameResolutionPass{}.Run(ctx, "a.sysml", root)
}

// An unnamed parameter takes the effective name of the parameter it implicitly
// redefines, and that name resolves in the owning scope (KerML 7.3.4.5,
// SysML 7.6.5).
func TestImplicitlyRedefiningParameterBindsRedefinedName(t *testing.T) {
	got := nameresDiags(t, `package P {
		item def Image;
		action def Shoot { in image : Image; }
		action shoot : Shoot {
			in item;
			attribute x = image;
		}
	}`)
	if len(got) != 0 {
		t.Fatalf("got %+v, want no diagnostics: image names the anonymous parameter", got)
	}
}

// resolvedInBody resolves name in the body scope of the member called fqn,
// after the pass has built the semantic model.
func resolvedInBody(t *testing.T, src, fqn, name string) (*symbols.Symbol, *symbols.Scope) {
	t.Helper()
	ctx, root := nameresCtx(t, "a.sysml", src)
	NameResolutionPass{}.Run(ctx, "a.sysml", root)
	owners := ctx.Index.LookupQualified(fqn)
	if len(owners) != 1 || owners[0].Scope == nil {
		t.Fatalf("looking up %s: got %d symbols with a body scope", fqn, len(owners))
	}
	sym, ok := ctx.Resolver().ResolveName(owners[0].Scope, name, nil)
	if !ok {
		t.Fatalf("%s does not resolve in %s", name, fqn)
	}
	return sym, owners[0].Scope
}

// The name binds the anonymous parameter itself, not the inherited one it
// takes the name from.
func TestImplicitlyRedefiningParameterIsWhatTheNameBinds(t *testing.T) {
	sym, scope := resolvedInBody(t, `package P {
		item def Image;
		action def Shoot { in image : Image; }
		action shoot : Shoot {
			in item;
			attribute x = image;
		}
	}`, "P::shoot", "image")
	if sym.OwnerScope != scope {
		t.Fatalf("image resolved to %s outside the body, want the anonymous parameter", sym.Name)
	}
}

// Two implicit redefinitions need not agree on a name, so neither names the
// parameter and the inherited feature keeps the name (KerML 7.3.4.5).
func TestParameterRedefiningTwoInheritedOnesStaysAnonymous(t *testing.T) {
	sym, scope := resolvedInBody(t, `package P {
		action def B1 { in p1; }
		action def B2 { in p2; }
		action a : B1, B2 { in item; }
	}`, "P::a", "p1")
	if sym.OwnerScope == scope {
		t.Fatalf("p1 resolved to the anonymous parameter of a, want B1::p1")
	}
}

// The name is the redefined parameter's, not the keyword the parameter was
// declared with.
func TestImplicitlyRedefiningParameterDoesNotBindItsKeyword(t *testing.T) {
	got := nameresDiags(t, `package P {
		item def Image;
		action def Shoot { in image : Image; }
		action shoot : Shoot {
			in item;
			attribute x = item;
		}
	}`)
	if len(got) != 1 || got[0].Code != "unresolved" {
		t.Fatalf("got %+v, want one unresolved diagnostic for item", got)
	}
}

// A nested usage sharing a name with an inherited feature is a name conflict:
// it is a distinct feature, not a redefinition of the inherited one
// (SysML 7.6.1, KerML 7.3.2.1).
func TestNameResolutionPassReportsInheritedNameConflict(t *testing.T) {
	got := nameresDiags(t, `package P {
		part def Engine;
		part def Vehicle { part engine : Engine; }
		part v : Vehicle { part engine; }
	}`)
	if len(got) != 1 {
		t.Fatalf("got %+v, want one diagnostic", got)
	}
	d := got[0]
	if d.Code != "name-conflict" || d.Source != "name-resolution" || d.Severity != SeverityError {
		t.Fatalf("got %+v, want code=name-conflict source=name-resolution severity=error", d)
	}
	if want := "name conflict: engine is already the name of the inherited feature Vehicle::engine"; d.Message != want {
		t.Fatalf("message = %q, want %q", d.Message, want)
	}
	if d.Span.Len != len("engine") {
		t.Fatalf("span = %+v, want the declared name's span", d.Span)
	}
}

// Redefining the inherited feature is how the name is legitimately reused.
func TestRedeclaredInheritedNameIsNoConflictWhenRedefined(t *testing.T) {
	got := nameresDiags(t, `package P {
		part def Engine;
		part def Vehicle { part engine : Engine; }
		part v : Vehicle { part engine :>> engine; }
		part w : Vehicle { part :>> engine; }
	}`)
	if len(got) != 0 {
		t.Fatalf("got %+v, want no diagnostics", got)
	}
}

// A parameter redefines its inherited counterpart by position, and a case's
// features redefine theirs by name, so neither conflicts.
func TestInheritedNameConflictExemptsRedefiningFeatures(t *testing.T) {
	got := nameresDiags(t, `package P {
		part def Vehicle;
		action def Shoot { in image; }
		action shoot : Shoot { in image; }
		use case def Drive { subject vehicle : Vehicle; }
		use case drive : Drive { subject vehicle; }
	}`)
	if len(got) != 0 {
		t.Fatalf("got %+v, want no diagnostics", got)
	}
}

// `render r;` names the rendering the view uses and declares no member of its
// own (SysML.xtext ViewRenderingUsage), so it does not conflict with the
// inherited rendering it names, and the name resolves to that rendering.
func TestRenderReferenceToInheritedRenderingIsNoConflict(t *testing.T) {
	got := nameresDiags(t, `package P {
		rendering def AsTree;
		view def Base { rendering r : AsTree; }
		view def Derived :> Base { render r; }
	}`)
	if len(got) != 0 {
		t.Fatalf("got %+v, want no diagnostics", got)
	}
}

// A rendering the view declares for itself does conflict with an inherited one
// of the same name, exactly as any other declaration does: only the reference
// form declares nothing.
func TestRenderDeclarationOfInheritedNameConflicts(t *testing.T) {
	got := nameresDiags(t, `package P {
		rendering def AsTree;
		view def Base { rendering r : AsTree; }
		view def Derived :> Base { render rendering r : AsTree; }
	}`)
	if len(got) != 1 || got[0].Code != "name-conflict" {
		t.Fatalf("got %+v, want one name-conflict diagnostic", got)
	}
}

// A concern and a viewpoint are requirements, so their bodies are exempt too.
func TestInheritedNameConflictExemptsConcernsAndViewpoints(t *testing.T) {
	got := nameresDiags(t, `package P {
		concern def C { stakeholder s; }
		concern c : C { stakeholder s; }
		viewpoint def V { stakeholder t; }
		viewpoint v : V { stakeholder t; }
	}`)
	if len(got) != 0 {
		t.Fatalf("got %+v, want no diagnostics", got)
	}
}

// A name that is not inherited is not a conflict.
func TestDistinctNestedNameIsNoConflict(t *testing.T) {
	got := nameresDiags(t, `package P {
		part def Engine;
		part def Vehicle { part engine : Engine; }
		part v : Vehicle { part motor; }
	}`)
	if len(got) != 0 {
		t.Fatalf("got %+v, want no diagnostics", got)
	}
}
