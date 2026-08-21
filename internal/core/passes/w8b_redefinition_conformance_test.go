package passes

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// conformanceDiags returns the feature-conformance findings of a KerML source.
func conformanceDiags(t *testing.T, src string) []Diagnostic {
	t.Helper()
	root := parser.New(source.New("<t>.kerml", []byte(src))).ParseFile()
	idx := symbols.NewIndex()
	idx.AddDocument("<t>.kerml", root)
	var out []Diagnostic
	for _, d := range Analyze("<t>.kerml", root, nil, idx) {
		switch d.Code {
		case "redefinition-direction-conformance",
			"subsetting-uniqueness-conformance",
			"subsetting-constancy-conformance":
			out = append(out, d)
		}
	}
	return out
}

// codes returns the diagnostic codes, so a case can assert what did and did not
// fire without depending on order.
func codes(diags []Diagnostic) []string {
	out := make([]string, 0, len(diags))
	for _, d := range diags {
		out = append(out, d.Code)
	}
	return out
}

// Redefinition_DirectionConformance_invalid.kerml.xt: `out :>> z` redefines an
// `in` feature.
func TestW8BDirectionConformanceOpposingDirection(t *testing.T) {
	diags := conformanceDiags(t, `
		package P {
			class C { in z; }
			class D specializes C { out :>> z; }
		}
	`)
	if !hasCode(diags, "redefinition-direction-conformance") {
		t.Fatalf("expected a direction violation, got %v", codes(diags))
	}
}

func TestW8BDirectionConformanceThroughConjugation(t *testing.T) {
	// D1 reaches C by a conjugation, which reverses z to `out`, so `in` breaks.
	diags := conformanceDiags(t, `
		package P {
			class C { in z; }
			class C1 conjugates C;
			class D1 specializes C1 { in :>> z; }
		}
	`)
	if !hasCode(diags, "redefinition-direction-conformance") {
		t.Fatalf("expected a direction violation through conjugation, got %v", codes(diags))
	}
}

func TestW8BDirectionConformanceAdmitsInoutAndUndeclared(t *testing.T) {
	// A redefined `inout` admits any direction, and a redefinition declaring no
	// direction takes the redefined one's.
	diags := conformanceDiags(t, `
		package P {
			class C { inout y; in x; }
			class D specializes C { out :>> y; feature :>> x; }
		}
	`)
	if len(diags) != 0 {
		t.Fatalf("expected no violation, got %v", codes(diags))
	}
}

// Feature_nonunique_invalid.kerml.xt: a nonunique feature may not subset or
// redefine a unique one.
func TestW8BUniquenessConformance(t *testing.T) {
	diags := conformanceDiags(t, `
		package P {
			classifier A { feature x; }
			classifier B specializes A { feature x1 nonunique subsets x; }
			classifier C specializes A { feature x2 nonunique redefines x; }
		}
	`)
	if got := len(diags); got != 2 {
		t.Fatalf("expected a violation on each of the two features, got %v", codes(diags))
	}
	for _, d := range diags {
		if d.Code != "subsetting-uniqueness-conformance" {
			t.Fatalf("unexpected diagnostic %q", d.Code)
		}
	}
}

func TestW8BUniquenessConformanceAllowsNonuniqueTarget(t *testing.T) {
	diags := conformanceDiags(t, `
		package P {
			classifier A { feature x nonunique; }
			classifier B specializes A { feature x1 nonunique subsets x; }
		}
	`)
	if len(diags) != 0 {
		t.Fatalf("a nonunique target imposes nothing, got %v", codes(diags))
	}
}

// Subsetting_constant_invalid.kerml.xt: a variable feature may not subset or
// redefine a constant one, transitively.
func TestW8BConstancyConformance(t *testing.T) {
	diags := conformanceDiags(t, `
		package P {
			class A {
				const feature f;
				var feature g :> f;
			}
			class B :> A {
				var feature h :>> f;
				var feature i :> g;
			}
		}
	`)
	if got := len(diags); got != 2 {
		t.Fatalf("expected the two declared violations, got %d: %v", got, codes(diags))
	}
	for _, d := range diags {
		if d.Code != "subsetting-constancy-conformance" {
			t.Fatalf("unexpected diagnostic %q", d.Code)
		}
	}
}

func TestW8BConstancyConformanceAllowsConstantRestriction(t *testing.T) {
	diags := conformanceDiags(t, `
		package P {
			class A { const feature f; }
			class B :> A { const feature h :>> f; feature k :> f; }
		}
	`)
	if len(diags) != 0 {
		t.Fatalf("a constant or unmodified restriction conforms, got %v", codes(diags))
	}
}

// Constancy is inherited along the subsetting chain, which parseable input may
// close into a cycle: the walk must terminate rather than recurse forever.
func TestW8BConstancyConformanceTerminatesOnCyclicSubsetting(t *testing.T) {
	diags := conformanceDiags(t, `
		package P {
			class A {
				feature a :> b;
				feature b :> a;
				var feature v :> a;
			}
		}
	`)
	if len(diags) != 0 {
		t.Fatalf("a cycle declares no constant feature, got %v", codes(diags))
	}
}

// MetadataTests_MetadataFeature_invalid.kerml.xt: a metadata annotation body
// restates features of the annotated type, so a name it does not offer — even
// one that exists in the surrounding namespace — redefines nothing.
func TestW8BMetadataBodyMustRedefineOwningTypeFeature(t *testing.T) {
	src := `package P {
		metadata def A { feature x; feature u { feature v; } }
		feature bad;
		feature a {
			@A {
				x = 1;
				u { v = 1; bad; }
			}
		}
	}`
	root := parser.New(source.New("<t>.kerml", []byte(src))).ParseFile()
	idx := symbols.NewIndex()
	idx.AddDocument("<t>.kerml", root)
	var got []Diagnostic
	for _, d := range Analyze("<t>.kerml", root, nil, idx) {
		if d.Code == "metadata-owning-type-feature" {
			got = append(got, d)
		}
	}
	if len(got) != 1 {
		t.Fatalf("expected one violation, on `bad`, got %d: %v", len(got), got)
	}
	if got[0].Span.Offset == 0 {
		t.Fatalf("diagnostic must be located at the offending declaration: %+v", got[0])
	}
}
