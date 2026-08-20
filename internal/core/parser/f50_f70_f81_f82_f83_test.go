package parser

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

func parseFeatureFix(t *testing.T, name, input string) (*ast.RootNamespace, []Diagnostic) {
	t.Helper()
	p := New(source.New(name, []byte(input)))
	root := p.ParseFile()
	return root, p.Diagnostics
}

func TestF50F70F81F82F83AndF62F63Parse(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{
			"variable_and_member",
			"package P { classifier C; classifier D { abstract var feature x [0..*]; member abstract feature y [0..*] featured by C; } }",
			"variable=true",
		},
		{
			"variable_before_modifier",
			"package P { classifier D { var abstract feature x; } }",
			"variable=true",
		},
		{
			"representation_keywords_as_names",
			"package P { classifier D { feature rep; feature language; } }",
			`name="language"`,
		},
		{
			"textual_representation",
			"package P { classifier C { inv check { rep inOCL language \"ocl\" /* body */ } } }",
			`TextualRepresentation language="\"ocl\"" name="inOCL"`,
		},
		{
			"differences",
			"package P { classifier A; classifier B; classifier D differences A, B; }",
			`kind="differences"`,
		},
		{
			"namespace_disjoint",
			"package P { classifier A; classifier B; disjoint B from A; }",
			`kind="disjoint"`,
		},
		{
			"classifier_multiplicity",
			"package P { classifier A; classifier B [1] specializes A; }",
			"(Multiplicity",
		},
		{
			"metaclass_multiplicity",
			"package P { classifier A; metaclass S [1] specializes A; }",
			`(Definition kind="metaclass"`,
		},
		{
			"exhibit_chain",
			"package P { state def S { state on; } part vehicle : S { exhibit vehicleStates.on; } }",
			"FeatureChainExpr",
		},
		{
			"reference_body",
			"package P { part medicalDevice { ref patient { event occurrence therapyDelayed; } } }",
			`name="patient"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, diags := parseFeatureFix(t, tt.name+".sysml", tt.src)
			if root == nil {
				t.Fatal("ParseFile returned nil")
			}
			if len(diags) != 0 {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}
			if dump := ast.Dump(root); !strings.Contains(dump, tt.want) {
				t.Fatalf("AST dump does not contain %q:\n%s", tt.want, dump)
			}
		})
	}
}

func TestNegativeF50F70F81F82F83AndF62F63(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{"abstract_var", "package P { abstract var; }"},
		{"member", "package P { classifier D { member ; } }"},
		{"representation", "package P { classifier D { inv c { rep language; } } }"},
		{"differences", "package P { classifier D differences ; }"},
		{"disjoint", "package P { disjoint from A; }"},
		{"multiplicity", "package P { classifier B [ specializes A; }"},
		{"sysml_definition_multiplicity", "package P { part def P [1] :> A; }"},
		{"exhibit", "package P { part v { exhibit .on { } } }"},
		{"reference", "package P { part v { ref { } } }"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("parser panicked: %v", r)
				}
			}()
			root, diags := parseFeatureFix(t, tt.name+".sysml", tt.src)
			if root == nil {
				t.Fatal("ParseFile returned nil")
			}
			if len(diags) == 0 {
				t.Fatal("expected a diagnostic")
			}
		})
	}
}

func TestNoPanicF50F70F81F82F83AndF62F63Truncated(t *testing.T) {
	sources := []string{
		"package P { classifier D { abstract var feature x [0..*]; } }",
		"package P { classifier D { inv c { rep inOCL language \"ocl\" /* body */ } } }",
		"package P { classifier D differences A, B; }",
		"package P { disjoint B from A; }",
		"package P { classifier B [1] specializes A; }",
		"package P { part v { exhibit vehicleStates.on { doc /* body */ } } }",
		"package P { part v { ref patient { event occurrence therapyDelayed; } } }",
	}
	for _, src := range sources {
		for i := 0; i <= len(src); i++ {
			prefix := src[:i]
			t.Run("", func(t *testing.T) {
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("panic at prefix %d: %v", i, r)
					}
				}()
				root, _ := parseFeatureFix(t, "truncated.sysml", prefix)
				if root == nil {
					t.Fatal("ParseFile returned nil")
				}
			})
		}
	}
}
