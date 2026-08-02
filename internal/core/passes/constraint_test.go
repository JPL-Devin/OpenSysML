package passes

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// constraintDiags parses src, indexes it, runs the full default registry, and
// returns only diagnostics whose Source is "constraint".
func constraintDiags(t *testing.T, src string) []Diagnostic {
	t.Helper()
	root := parser.New(source.New("<t>", []byte(src))).ParseFile()
	idx := symbols.NewIndex()
	idx.AddDocument("<t>", root)
	all := Analyze("<t>", root, nil, idx)
	var out []Diagnostic
	for _, d := range all {
		if d.Source == "constraint" {
			out = append(out, d)
		}
	}
	return out
}

func hasCode(diags []Diagnostic, code string) bool {
	for _, d := range diags {
		if d.Code == code {
			return true
		}
	}
	return false
}

func TestConstraintDirectSpecializationCycle(t *testing.T) {
	diags := constraintDiags(t, "part def A specializes A;")
	if !hasCode(diags, "specialization-cycle") {
		t.Fatalf("expected specialization-cycle diagnostic, got %v", diags)
	}
}

func TestConstraintTransitiveSpecializationCycle(t *testing.T) {
	diags := constraintDiags(t, "part def A specializes B; part def B specializes A;")
	// Both A and B are in the cycle; expect a diagnostic for each.
	n := 0
	for _, d := range diags {
		if d.Code == "specialization-cycle" {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("expected 2 specialization-cycle diagnostics, got %d: %v", n, diags)
	}
}

func TestConstraintNoCycleAcyclicOK(t *testing.T) {
	diags := constraintDiags(t, "part def Vehicle; part def Car specializes Vehicle;")
	if hasCode(diags, "specialization-cycle") {
		t.Fatalf("unexpected specialization-cycle diagnostic, got %v", diags)
	}
}

func TestConstraintMultiplicityRangeInverted(t *testing.T) {
	diags := constraintDiags(t, "part def C { part b [5..2]; }")
	if !hasCode(diags, "multiplicity-range") {
		t.Fatalf("expected multiplicity-range diagnostic, got %v", diags)
	}
}

func TestConstraintMultiplicityRangeValidOK(t *testing.T) {
	diags := constraintDiags(t, "part def C { part a [2..5]; part c [1..*]; }")
	if hasCode(diags, "multiplicity-range") {
		t.Fatalf("unexpected multiplicity-range diagnostic, got %v", diags)
	}
}

func TestConstraintSubsettingMultiplicityExceeds(t *testing.T) {
	diags := constraintDiags(t,
		"part def C { part cap [0..3]; part many subsets cap [0..10]; }")
	if !hasCode(diags, "subsetting-multiplicity") {
		t.Fatalf("expected subsetting-multiplicity diagnostic, got %v", diags)
	}
}

func TestConstraintSubsettingMultiplicityConformsOK(t *testing.T) {
	diags := constraintDiags(t,
		"part def C { part cap [0..10]; part few subsets cap [0..3]; }")
	if hasCode(diags, "subsetting-multiplicity") {
		t.Fatalf("unexpected subsetting-multiplicity diagnostic, got %v", diags)
	}
}

func TestConstraintSubsettingUnboundedSupersetOK(t *testing.T) {
	diags := constraintDiags(t,
		"part def C { part cap [0..*]; part many subsets cap [0..100]; }")
	if hasCode(diags, "subsetting-multiplicity") {
		t.Fatalf("unexpected subsetting-multiplicity diagnostic, got %v", diags)
	}
}

func TestConstraintSubsettingInfiniteSubsetOfFiniteExceeds(t *testing.T) {
	diags := constraintDiags(t,
		"part def C { part cap [0..5]; part many subsets cap [0..*]; }")
	if !hasCode(diags, "subsetting-multiplicity") {
		t.Fatalf("expected subsetting-multiplicity diagnostic, got %v", diags)
	}
}

// --- V-C3 §4.3/§4.6: connector and flow ends ---

func TestConstraintConnectionBinaryOK(t *testing.T) {
	diags := constraintDiags(t,
		"part def C { part a; part b; connection conn connect a to b; }")
	if hasCode(diags, "connector-ends") {
		t.Fatalf("unexpected connector-ends diagnostic, got %v", diags)
	}
}

func TestConstraintConnectionNaryOK(t *testing.T) {
	diags := constraintDiags(t,
		"part def C { part a; part b; part d; connection conn connect (a, b, d); }")
	if hasCode(diags, "connector-ends") {
		t.Fatalf("n-ary connection should be allowed, got %v", diags)
	}
}

func TestConstraintConnectionSingleEndFails(t *testing.T) {
	diags := constraintDiags(t,
		"part def C { part a; connection conn connect (a); }")
	if !hasCode(diags, "connector-ends") {
		t.Fatalf("expected connector-ends diagnostic for single-end connection, got %v", diags)
	}
}

func TestConstraintInterfaceBinaryOK(t *testing.T) {
	diags := constraintDiags(t,
		"part def C { port p; port q; interface i connect p to q; }")
	if hasCode(diags, "connector-ends") {
		t.Fatalf("unexpected connector-ends diagnostic, got %v", diags)
	}
}

func TestConstraintInterfaceNaryFails(t *testing.T) {
	diags := constraintDiags(t,
		"part def C { port p; port q; port r; interface i connect (p, q, r); }")
	if !hasCode(diags, "connector-ends") {
		t.Fatalf("expected connector-ends diagnostic for n-ary interface, got %v", diags)
	}
}

func TestConstraintAllocationBinaryOK(t *testing.T) {
	diags := constraintDiags(t,
		"part def C { part f; part g; allocation al allocate f to g; }")
	if hasCode(diags, "connector-ends") {
		t.Fatalf("unexpected connector-ends diagnostic, got %v", diags)
	}
}

func TestConstraintAllocationNaryFails(t *testing.T) {
	diags := constraintDiags(t,
		"part def C { part f; part g; part h; allocation al allocate (f, g, h); }")
	if !hasCode(diags, "connector-ends") {
		t.Fatalf("expected connector-ends diagnostic for n-ary allocation, got %v", diags)
	}
}

func TestConstraintFlowCompleteOK(t *testing.T) {
	diags := constraintDiags(t,
		"part def C { item Fuel; part a; part b; flow f of Fuel from a to b; }")
	if hasCode(diags, "flow-ends") {
		t.Fatalf("unexpected flow-ends diagnostic, got %v", diags)
	}
}

func TestConstraintFlowMissingEndsFails(t *testing.T) {
	diags := constraintDiags(t,
		"part def C { item Fuel; flow f of Fuel; }")
	if !hasCode(diags, "flow-ends") {
		t.Fatalf("expected flow-ends diagnostic for payload-only flow, got %v", diags)
	}
}

// --- V-C4 Track 4 Task 13: typing conformance ---

func TestConstraint_TypingConformanceValid(t *testing.T) {
	src := `
		attribute def Vehicle;
		attribute def Car specializes Vehicle;
		
		attribute vehicles : Vehicle[*];
		attribute myCar : Car subsets vehicles;
	`
	diags := constraintDiags(t, src)
	if hasCode(diags, "typing-conformance") {
		t.Fatalf("expected no typing-conformance diagnostic for valid conformance, got %v", diags)
	}
}

func TestConstraint_TypingConformanceInvalid(t *testing.T) {
	src := `
		attribute def Vehicle;
		attribute def Animal;
		
		attribute vehicles : Vehicle[*];
		attribute myPet : Animal subsets vehicles;
	`
	diags := constraintDiags(t, src)
	if !hasCode(diags, "typing-conformance") {
		t.Fatalf("expected typing-conformance diagnostic, got %v", diags)
	}
}

// --- V-C4 Track 4 Task 14: redefinition validation ---

func TestConstraint_RedefinitionValid(t *testing.T) {
	src := `
		attribute def SpeedType;
		attribute def Vehicle {
			attribute speed : SpeedType;
		}
		attribute def Car specializes Vehicle {
			attribute speed : SpeedType :>> Vehicle::speed;
		}
	`
	diags := constraintDiags(t, src)
	if hasCode(diags, "redefinition-no-inherited") || hasCode(diags, "redefinition-type-mismatch") {
		t.Fatalf("expected no redefinition diagnostic for valid redefinition, got %v", diags)
	}
}

func TestConstraint_RedefinitionNoInheritedMember(t *testing.T) {
	src := `
		attribute def SpeedType;
		attribute def Vehicle {
			attribute speed : SpeedType;
		}
		attribute def Car {
			attribute speed : SpeedType :>> Vehicle::speed;
		}
	`
	diags := constraintDiags(t, src)
	if !hasCode(diags, "redefinition-no-inherited") {
		t.Fatalf("expected redefinition-no-inherited diagnostic, got %v", diags)
	}
}

func TestConstraint_RedefinitionTypeMismatch(t *testing.T) {
	src := `
		attribute def SpeedType;
		attribute def NameType;
		attribute def Vehicle {
			attribute speed : SpeedType;
		}
		attribute def Car specializes Vehicle {
			attribute speed : NameType :>> Vehicle::speed;
		}
	`
	diags := constraintDiags(t, src)
	if !hasCode(diags, "redefinition-type-mismatch") {
		t.Fatalf("expected redefinition-type-mismatch diagnostic, got %v", diags)
	}
}

func TestConstraint_RedefinitionMultiplicityInvalid(t *testing.T) {
	src := `
		attribute def SpeedType;
		part def Vehicle {
			attribute speed : SpeedType[1..2];
		}
		part def Car specializes Vehicle {
			attribute speed : SpeedType[0..5] :>> Vehicle::speed;
		}
	`
	diags := constraintDiags(t, src)
	if !hasCode(diags, "redefinition-multiplicity") {
		t.Fatalf("expected redefinition-multiplicity diagnostic, got %v", diags)
	}
}

// --- V-C4 Track 4 Integration: typing conformance + redefinition ---

func TestConstraint_Track4Integration(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		wantErr bool
		codes   []string
	}{
		{
			name: "valid redefinition with multiplicity narrowing",
			src: `
				attribute def SpeedType;
				attribute def Vehicle {
					attribute speed : SpeedType[1..2];
				}
				attribute def Car specializes Vehicle {
					attribute speed : SpeedType[1..1] :>> Vehicle::speed;
				}
				attribute myCar : Car;
			`,
			wantErr: false,
		},
		{
			name: "redefinition without inheritance",
			src: `
				attribute def SpeedType;
				attribute def Animal {
					attribute speed : SpeedType;
				}
				attribute def Vehicle {
					attribute speed : SpeedType;
				}
				attribute def Car specializes Vehicle {
					attribute speed : SpeedType redefines Animal::speed;
				}
				attribute myCar : Car;
			`,
			wantErr: true,
			codes:   []string{"redefinition-no-inherited"},
		},
		{
			name: "redefinition multiplicity violation",
			src: `
				attribute def SpeedType;
				attribute def Vehicle {
					attribute speed : SpeedType[1..2];
				}
				attribute def Car specializes Vehicle {
					attribute speed : SpeedType[0..5] :>> Vehicle::speed;
				}
				attribute myCar : Car;
			`,
			wantErr: true,
			codes:   []string{"redefinition-multiplicity"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diags := constraintDiags(t, tt.src)
			hasError := len(diags) > 0
			if hasError != tt.wantErr {
				t.Fatalf("wantErr=%v, got diagnostics: %v", tt.wantErr, diags)
			}
			if tt.wantErr {
				for _, code := range tt.codes {
					if !hasCode(diags, code) {
						t.Errorf("expected code %q, got: %v", code, diags)
					}
				}
			}
		})
	}
}
