package identity_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/identity"
	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// buildTable analyzes src over the bundled libraries and computes its table.
func buildTable(t *testing.T, src string) (*identity.Table, *symbols.Index) {
	t.Helper()
	p := parser.New(source.New("<t>", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx := libs.NewModelIndex()
	idx.AddDocument("<t>", root)
	idx.ExpandWildcardImports()
	res := resolve.New(idx)
	model := semantics.NewModel(res)
	res.SetModel(model)
	return identity.Build(model, res, idx.DocumentRoot("<t>")), idx
}

func infoOf(t *testing.T, table *identity.Table, idx *symbols.Index, fqn string) *identity.Info {
	t.Helper()
	matches := idx.LookupQualified(fqn)
	if len(matches) == 0 {
		t.Fatalf("%s: no symbol", fqn)
	}
	info, ok := table.Info(matches[0])
	if !ok {
		t.Fatalf("%s: no identity info", fqn)
	}
	return info
}

func TestAnnotatedElementCarriesItsDeclaredID(t *testing.T) {
	table, idx := buildTable(t, `package Vehicles {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; branch = "main"; }
	part def Vehicle {
		@IdentityMetadata::ElementId { id = "8f3a41d0"; }
	}
}
`)
	info := infoOf(t, table, idx, "Vehicles::Vehicle")
	if !info.Annotated || info.DeclaredID != "8f3a41d0" || info.EffectiveID != "8f3a41d0" {
		t.Fatalf("annotated=%v declared=%q effective=%q, want annotated 8f3a41d0",
			info.Annotated, info.DeclaredID, info.EffectiveID)
	}
	if info.Scope == nil || info.Scope.ProjectID != "proj-1" || info.Scope.Branch != "main" || info.Scope.Org != "" {
		t.Fatalf("scope = %+v, want projectId proj-1, branch main, empty org", info.Scope)
	}
}

func TestUnannotatedElementDerivesItsID(t *testing.T) {
	table, idx := buildTable(t, `package Vehicles {
	part def Vehicle;
}
`)
	info := infoOf(t, table, idx, "Vehicles::Vehicle")
	want := rdf.EncodeElementID("Vehicles::Vehicle")
	if info.Annotated || info.EffectiveID != want {
		t.Fatalf("annotated=%v effective=%q, want derived %q", info.Annotated, info.EffectiveID, want)
	}
	if info.Scope != nil {
		t.Fatalf("scope = %+v, want unbound", info.Scope)
	}
}

func TestDeclaredIDEqualToDerivedIDIsStillAnnotated(t *testing.T) {
	derived := rdf.EncodeElementID("P::e")
	table, idx := buildTable(t, `package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; }
	part def e {
		@IdentityMetadata::ElementId { id = "`+derived+`"; }
	}
}
`)
	info := infoOf(t, table, idx, "P::e")
	if !info.Annotated || info.DeclaredID != derived || info.EffectiveID != derived {
		t.Fatalf("annotated=%v declared=%q effective=%q, want the derived id %q kept explicit",
			info.Annotated, info.DeclaredID, info.EffectiveID, derived)
	}
}

func TestNestedProjectRefScopesResolveToTheNearest(t *testing.T) {
	table, idx := buildTable(t, `package Outer {
	@IdentityMetadata::ProjectRef { projectId = "outer-proj"; org = "org-a"; }
	part def A;
	package Inner {
		@IdentityMetadata::ProjectRef { projectId = "inner-proj"; }
		part def B;
	}
}
`)
	a := infoOf(t, table, idx, "Outer::A")
	if a.Scope == nil || a.Scope.ProjectID != "outer-proj" || a.Scope.Org != "org-a" {
		t.Fatalf("A scope = %+v, want outer-proj/org-a", a.Scope)
	}
	b := infoOf(t, table, idx, "Outer::Inner::B")
	if b.Scope == nil || b.Scope.ProjectID != "inner-proj" || b.Scope.Org != "" {
		t.Fatalf("B scope = %+v, want inner-proj", b.Scope)
	}
	inner := infoOf(t, table, idx, "Outer::Inner")
	if inner.Scope == nil || inner.Scope.ProjectID != "inner-proj" {
		t.Fatalf("Inner scope = %+v, want its own inner-proj", inner.Scope)
	}
	if a.Scope.Key() == b.Scope.Key() {
		t.Fatal("distinct projects must have distinct scope keys")
	}
}
