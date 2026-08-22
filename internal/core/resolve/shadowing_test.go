package resolve

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

func TestGeneralizationHeaderResolvesOutsideTheDeclarationBody(t *testing.T) {
	r := resolveVisibilityDoc(t, `package P {
		class A { class a1; }
		class B specializes A {
			class A { class a2; }
			class b specializes a1;
		}
		feature C { feature c1; }
		feature D subsets C {
			feature C { feature c2; }
			feature d redefines C::c1;
		}
	}`)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("generalization headers resolved in their bodies: %v", r.Diagnostics)
	}
}

func TestQualifiedRedefinitionReportsOneUnresolvedDiagnostic(t *testing.T) {
	r := resolveVisibilityDoc(t, `package P {
		feature C { feature c1; }
		feature D subsets C {
			feature d redefines C::nope;
		}
	}`)
	if len(r.Diagnostics) != 1 {
		t.Fatalf("diagnostics = %v, want one unresolved target", r.Diagnostics)
	}
}

func TestQualifiedRedefinitionFallbackDoesNotReportSpeculativeFailure(t *testing.T) {
	r := resolveVisibilityDoc(t, `package P {
		feature C { feature c1; }
		feature D subsets C {
			feature C { feature nope; }
			feature d redefines C::nope;
		}
	}`)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("fallback resolution reported speculative diagnostics: %v", r.Diagnostics)
	}
}

func TestEnclosingDeclarationShadowsANestedImport(t *testing.T) {
	idx := symbols.NewIndex()
	idx.AddDocument("lib.kerml", parsedRoot(t, "lib.kerml", `package Lib {
		feature A { feature a1; }
	}`))
	root := parsedRoot(t, "app.kerml", `package P {
		feature A { feature a2; }
		feature direct {
			public import Lib::*;
			feature B redefines A { feature b redefines a2; }
		}
		feature inheritedSource { public import Lib::*; }
		feature inherited subsets inheritedSource {
			feature B redefines A { feature b redefines a1; }
		}
	}`)
	idx.AddDocument("app.kerml", root)
	idx.ExpandWildcardImports()
	r := resolverWithSpecializations(idx)
	r.ResolveDocument("app.kerml", root)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("nested import shadowed the enclosing declaration: %v", r.Diagnostics)
	}
}
