package passes

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
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

// The constraint tier sees every end an n-ary clause declares, so its arity
// rules count the real arity rather than a truncated one.
func TestConstraintConnectionNaryEndCountReachesTheChecker(t *testing.T) {
	src := "part def C { part a; part b; part c; part d; connection conn connect (a, b, c, d); }"
	root := parser.New(source.New("<t>", []byte(src))).ParseFile()
	idx := symbols.NewIndex()
	idx.AddDocument("<t>", root)
	syms := idx.LookupQualified("C::conn")
	if len(syms) != 1 {
		t.Fatalf("expected one symbol for C::conn, got %d", len(syms))
	}
	u, ok := syms[0].Decl.(*ast.Usage)
	if !ok {
		t.Fatalf("C::conn declared by %T, want *ast.Usage", syms[0].Decl)
	}
	if len(u.ConnectorEnds) != 4 {
		t.Fatalf("connector ends at the constraint tier = %d, want 4", len(u.ConnectorEnds))
	}
	if hasCode(constraintDiags(t, src), "connector-ends") {
		t.Fatalf("a four-end connection should be allowed")
	}
}

// The anonymous inline form reaches the arity checker with its real end count.
func TestConstraintAnonymousNaryConnectionEndCountReachesTheChecker(t *testing.T) {
	src := "part def C { part a; part b; part c; connect (a, b, c); }"
	if hasCode(constraintDiags(t, src), "connector-ends") {
		t.Fatalf("a three-end anonymous connection should be allowed")
	}
	if !hasCode(constraintDiags(t, "part def C { part a; connect (a); }"), "connector-ends") {
		t.Fatalf("a one-end anonymous connection should be reported")
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

// A payload-only flow declares no ends. SysML v2 §8.2.2.16 makes the
// `'from' … 'to' …` part of a FlowDeclaration optional, and §8.4.12.2 requires a
// message to have no owned flowEnds, so this is well formed.
func TestConstraintFlowPayloadOnlyHasNoEndsOK(t *testing.T) {
	diags := constraintDiags(t,
		"part def C { item Fuel; flow f of Fuel; }")
	if hasCode(diags, "flow-ends") {
		t.Fatalf("unexpected flow-ends diagnostic for payload-only flow, got %v", diags)
	}
}

func TestConstraintMessageWithoutEndsOK(t *testing.T) {
	diags := constraintDiags(t,
		"item def SetSpeed; occurrence def I { message setSpeedMessage of SetSpeed; }")
	if hasCode(diags, "flow-ends") {
		t.Fatalf("unexpected flow-ends diagnostic for a message without ends, got %v", diags)
	}
}

// A flow naming a source but no target is a parse error, pinned by the parser's
// `flow_source_without_target` negative case, not by this tier.

// --- V-C4 Track 4 Task 13: typing conformance ---

// Subsetting does not require the subsetting feature's declared type to conform
// to the subsetted feature's type. Per KerML 8.3.3.3.4, the types of a Feature
// "are derived from its typings and the types of its subsettings", so the
// co-domain KerML 8.3.3.3.10 requires to specialize the subsetted co-domain is
// the intersection of both — it always does, whether or not the declared types
// are related (KerML 7.3.4.4: a subsetting feature can "add additional feature
// types").
func TestConstraintSubsettingConformingTypeOK(t *testing.T) {
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

func TestConstraintSubsettingUnrelatedTypeOK(t *testing.T) {
	src := `
		attribute def Vehicle;
		attribute def Animal;
		
		attribute vehicles : Vehicle[*];
		attribute myPet : Animal subsets vehicles;
	`
	diags := constraintDiags(t, src)
	if hasCode(diags, "typing-conformance") {
		t.Fatalf("unexpected typing-conformance diagnostic: subsetting intersects types, got %v", diags)
	}
}

// The occurrence shape from the OMG `Model Library Example`: `Cause` does not
// specialize `Situation`, yet `causes :> situations` is well formed.
func TestConstraintSubsettingOccurrenceUnrelatedTypeOK(t *testing.T) {
	src := `
		abstract occurrence def Situation;
		abstract occurrence situations : Situation[*] nonunique;
		abstract occurrence def Cause;
		abstract occurrence causes : Cause[*] nonunique :> situations;
	`
	diags := constraintDiags(t, src)
	if hasCode(diags, "typing-conformance") {
		t.Fatalf("unexpected typing-conformance diagnostic on the model library shape, got %v", diags)
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

// `[*]` is `0..*` in a redefinition: it keeps an inherited `0..*` but loosens an
// inherited `1..*`, dropping its lower bound to 0.
func TestConstraint_RedefinitionUnboundedMultiplicity(t *testing.T) {
	tests := []struct {
		name      string
		inherited string
		wantDiag  bool
	}{
		{"redefines optional collection", "0..*", false},
		{"redefines required collection", "1..*", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := `
				attribute def SpeedType;
				part def Vehicle {
					attribute speed : SpeedType[` + tt.inherited + `];
				}
				part def Car specializes Vehicle {
					attribute speed : SpeedType[*] :>> Vehicle::speed;
				}
			`
			diags := constraintDiags(t, src)
			if got := hasCode(diags, "redefinition-multiplicity"); got != tt.wantDiag {
				t.Fatalf("redefinition-multiplicity = %v, want %v (diags %v)", got, tt.wantDiag, diags)
			}
		})
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

// A member redefining two features derives no name, so its value reaches
// neither; the diagnostic is a warning because the declaration is well-formed.
func TestConstraintUnnamedRedefinitionValue(t *testing.T) {
	src := `
		attribute def N;
		part def B { attribute x : N; attribute y : N; }
		part def A :> B {
			attribute <sn> redefines x, y = 9;
		}
	`
	diags := constraintDiags(t, src)
	var got *Diagnostic
	for i, d := range diags {
		if d.Code == "redefinition-no-derived-name" {
			got = &diags[i]
		}
	}
	if got == nil {
		t.Fatalf("expected redefinition-no-derived-name diagnostic, got %v", diags)
	}
	if got.Severity != SeverityWarning {
		t.Errorf("severity = %v, want warning", got.Severity)
	}
	want := "a member redefining x and y derives no name, so this value is bound to the short name <sn> only; declare a name or redefine one feature"
	if got.Message != want {
		t.Errorf("message = %q, want %q", got.Message, want)
	}
}

// The symbol spelling reports identically, and a member with no short name says
// the value is unreachable instead.
func TestConstraintUnnamedRedefinitionValueSpellings(t *testing.T) {
	cases := []struct {
		name   string
		member string
		want   string
	}{
		{"keyword_short_name", "attribute <sn> redefines x, y = 9;", "bound to the short name <sn> only"},
		{"symbol_short_name", "attribute <sn> :>> x :>> y = 9;", "bound to the short name <sn> only"},
		{"keyword_anonymous", "attribute redefines x, y = 9;", "is not reachable by name"},
		{"symbol_anonymous", "attribute :>> x :>> y = 9;", "is not reachable by name"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			src := "attribute def N;\npart def B { attribute x : N; attribute y : N; }\n" +
				"part def A :> B {\n" + tt.member + "\n}"
			diags := constraintDiags(t, src)
			if !hasCode(diags, "redefinition-no-derived-name") {
				t.Fatalf("expected redefinition-no-derived-name, got %v", diags)
			}
			for _, d := range diags {
				if d.Code == "redefinition-no-derived-name" && !strings.Contains(d.Message, tt.want) {
					t.Errorf("message %q does not contain %q", d.Message, tt.want)
				}
			}
		})
	}
}

// A single redefinition derives its name from the target, so its value binds
// and nothing is reported.
func TestConstraintSingleRedefinitionValueOK(t *testing.T) {
	src := `
		attribute def N;
		part def B { attribute x : N; }
		part def A :> B {
			attribute <sn> redefines x = 5;
			attribute named redefines x = 6;
		}
	`
	if diags := constraintDiags(t, src); hasCode(diags, "redefinition-no-derived-name") {
		t.Fatalf("unexpected redefinition-no-derived-name, got %v", diags)
	}
}

// Two redefinitions without a value derive no name either, but nothing is lost.
func TestConstraintUnnamedRedefinitionNoValueOK(t *testing.T) {
	src := `
		attribute def N;
		part def B { attribute x : N; attribute y : N; }
		part def A :> B { attribute <sn> redefines x, y; }
	`
	if diags := constraintDiags(t, src); hasCode(diags, "redefinition-no-derived-name") {
		t.Fatalf("unexpected redefinition-no-derived-name, got %v", diags)
	}
}

// TestConstraintInterfaceEndConjugation covers SysML v2 §7.12.2: the ports at
// the two ends of an interface must have conjugate directed features, which one
// conjugated end (~P) supplies and two like-typed ends do not.
func TestConstraintInterfaceEndConjugation(t *testing.T) {
	const ports = `port def P { in item cmd; out item tlm; }
`
	conjugated := constraintDiags(t, ports+`interface def I {
		end a : P;
		end b : ~P;
	}`)
	if hasCode(conjugated, "port-conjugation") {
		t.Errorf("unexpected port-conjugation diagnostic for conjugate ends: %v", conjugated)
	}

	mismatched := constraintDiags(t, ports+`interface def I {
		end a : P;
		end b : P;
	}`)
	if !hasCode(mismatched, "port-conjugation") {
		t.Errorf("expected port-conjugation diagnostic for like-typed ends, got %v", mismatched)
	}

	// A port with no directed features imposes nothing.
	undirected := constraintDiags(t, `port def U { attribute x; }
	interface def I {
		end a : U;
		end b : U;
	}`)
	if hasCode(undirected, "port-conjugation") {
		t.Errorf("unexpected port-conjugation diagnostic for undirected ports: %v", undirected)
	}

	// Conjugation constrains directed features only, so ports holding different
	// undirected features still line up.
	extra := constraintDiags(t, `port def A { attribute pressure; out item flow; }
	port def B { in item flow; }
	interface def I {
		end a : A;
		end b : B;
	}`)
	if hasCode(extra, "port-conjugation") {
		t.Errorf("unexpected port-conjugation diagnostic for conjugate directed features: %v", extra)
	}
}

// A `variant` whose owner is not a variation offers no choice, so it is reported
// as a warning: the member is well-formed, only its `variant` keyword is idle.
func TestConstraintVariantOutsideVariation(t *testing.T) {
	src := `
		part def Widget {
			variant attribute misplaced = 1.0;
		}
	`
	diags := constraintDiags(t, src)
	var got *Diagnostic
	for i, d := range diags {
		if d.Code == "variant-outside-variation" {
			got = &diags[i]
		}
	}
	if got == nil {
		t.Fatalf("expected variant-outside-variation diagnostic, got %v", diags)
	}
	if got.Severity != SeverityWarning {
		t.Errorf("severity = %v, want warning", got.Severity)
	}
	if !strings.Contains(got.Message, "variant misplaced is declared in Widget") {
		t.Errorf("message %q does not name the variant and its owner", got.Message)
	}
}

// A variant of a variation is exactly what `variant` is for, so nothing is
// reported for it.
func TestConstraintVariantInsideVariationOK(t *testing.T) {
	src := `
		part def Widget {
			variation attribute pick {
				variant attribute cheap = 1.0;
				variant attribute rich = 2.0;
			}
		}
	`
	if diags := constraintDiags(t, src); hasCode(diags, "variant-outside-variation") {
		t.Fatalf("unexpected variant-outside-variation, got %v", diags)
	}
}

// A usage typed by a variation definition, and one redefining a variation usage,
// are variation points without restating the modifier.
func TestConstraintVariantUnderInheritedVariationOK(t *testing.T) {
	src := `
		part def Engine;
		variation part def EngineChoice :> Engine;
		part def Car {
			part engine : EngineChoice {
				variant part electric : Engine;
			}
		}
		abstract part refined : Car {
			part :>> engine {
				variant part petrol : Engine;
			}
		}
	`
	if diags := constraintDiags(t, src); hasCode(diags, "variant-outside-variation") {
		t.Fatalf("unexpected variant-outside-variation, got %v", diags)
	}
}
