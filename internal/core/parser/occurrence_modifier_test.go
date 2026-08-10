package parser

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

// parseUsages parses src and returns the usages declared directly in the given
// namespace member, flattening memberships.
func parseSingleUsage(t *testing.T, input string) *ast.Usage {
	t.Helper()
	sf := source.New("occurrence_modifier.sysml", []byte(input))
	p := New(sf)
	file := p.ParseFile()
	for _, d := range p.Diagnostics {
		t.Errorf("parse error: %s", d.Message)
	}
	if len(file.Members) != 1 {
		t.Fatalf("expected one namespace member, got %d", len(file.Members))
	}
	m, ok := file.Members[0].(*ast.Membership)
	if !ok {
		t.Fatalf("expected a membership, got %T", file.Members[0])
	}
	u, ok := m.Member.(*ast.Usage)
	if !ok {
		t.Fatalf("expected a usage, got %T", m.Member)
	}
	return u
}

// parseSingleBodyUsage parses src, which must declare a single definition whose
// body declares a single usage, and returns that usage.
func parseSingleBodyUsage(t *testing.T, input string) *ast.Usage {
	t.Helper()
	sf := source.New("occurrence_modifier_body.sysml", []byte(input))
	p := New(sf)
	file := p.ParseFile()
	for _, d := range p.Diagnostics {
		t.Errorf("parse error: %s", d.Message)
	}
	def, ok := file.Members[0].(*ast.Membership).Member.(*ast.Definition)
	if !ok {
		t.Fatalf("expected a definition, got %T", file.Members[0].(*ast.Membership).Member)
	}
	if len(def.Members) != 1 {
		t.Fatalf("expected one body member, got %d", len(def.Members))
	}
	u, ok := def.Members[0].(*ast.Membership).Member.(*ast.Usage)
	if !ok {
		t.Fatalf("expected a usage, got %T", def.Members[0].(*ast.Membership).Member)
	}
	return u
}

// The `individual` and `snapshot` usage modifiers are syntax, so the parser
// stores them on the usage node: SysML v2 §8.3.9.11 gives OccurrenceUsage an
// `isIndividual` attribute and a `portionKind`, of which `snapshot` is one.
func TestParseUsageOccurrenceModifiers(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantKind       ast.UsageKind
		wantIndividual bool
		wantSnapshot   bool
	}{
		{"individual", "individual testSystem : TestSystem;", ast.UsageAttribute, true, false},
		// The modifier is orthogonal to the kind keyword, which still declares the kind.
		{"individual_part", "individual part testVehicle : TestSystem;", ast.UsagePart, true, false},
		{"individual_occurrence", "individual occurrence testFlight : Flight;", ast.UsageOccurrence, true, false},
		{"individual_item", "individual item testCargo : Cargo;", ast.UsageItem, true, false},
		{"individual_anonymous", "individual : TestSystem;", ast.UsageIndividual, true, false},
		{"snapshot", "snapshot atLiftoff : Flight;", ast.UsageOccurrence, false, true},
		{"snapshot_occurrence", "snapshot occurrence takeoff : Flight;", ast.UsageOccurrence, false, true},
		{"snapshot_part", "snapshot part vehicleAtTakeoff : Vehicle;", ast.UsagePart, false, true},
		{"individual_snapshot", "individual snapshot testAtLiftoff : Flight;", ast.UsageOccurrence, true, true},
		{"plain_usage", "occurrence takeoff : Flight;", ast.UsageOccurrence, false, false},
		{"plain_part", "part vehicle : Vehicle;", ast.UsagePart, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := parseSingleUsage(t, tt.input)
			if u.Kind != tt.wantKind {
				t.Errorf("kind = %v, want %v", u.Kind, tt.wantKind)
			}
			if u.IsIndividual != tt.wantIndividual {
				t.Errorf("IsIndividual = %t, want %t", u.IsIndividual, tt.wantIndividual)
			}
			if u.IsSnapshot != tt.wantSnapshot {
				t.Errorf("IsSnapshot = %t, want %t", u.IsSnapshot, tt.wantSnapshot)
			}
		})
	}
}

// A modified usage in a definition body keeps its modifier: the body is the
// form these declarations actually take in a model, and the enclosing
// declaration parser must not consume `individual` as a bare kind prefix.
func TestParseUsageOccurrenceModifiersInBody(t *testing.T) {
	tests := []struct {
		name           string
		member         string
		wantKind       ast.UsageKind
		wantIndividual bool
		wantSnapshot   bool
	}{
		{"individual", "individual testSystem : TestSystem;", ast.UsageAttribute, true, false},
		{"individual_part", "individual part p;", ast.UsagePart, true, false},
		{"individual_occurrence", "individual occurrence o;", ast.UsageOccurrence, true, false},
		{"individual_snapshot", "individual snapshot s;", ast.UsageOccurrence, true, true},
		{"snapshot_part", "snapshot part sp;", ast.UsagePart, false, true},
		{"plain_part", "part p;", ast.UsagePart, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := parseSingleBodyUsage(t, "part def C { "+tt.member+" }")
			if u.Kind != tt.wantKind {
				t.Errorf("kind = %v, want %v", u.Kind, tt.wantKind)
			}
			if u.IsIndividual != tt.wantIndividual {
				t.Errorf("IsIndividual = %t, want %t", u.IsIndividual, tt.wantIndividual)
			}
			if u.IsSnapshot != tt.wantSnapshot {
				t.Errorf("IsSnapshot = %t, want %t", u.IsSnapshot, tt.wantSnapshot)
			}
		})
	}
}

// A directed parameter takes the same modifiers through the behavior parser's
// own parameter path, which must store them too.
func TestParseDirectionParameterOccurrenceModifiers(t *testing.T) {
	input := `action def Collect {
		in individual subjectVehicle : TestVehicle;
		in snapshot vehicleState : Flight;
		out item payload : Cargo;
	}`
	sf := source.New("occurrence_modifier_params.sysml", []byte(input))
	p := New(sf)
	file := p.ParseFile()
	for _, d := range p.Diagnostics {
		t.Errorf("parse error: %s", d.Message)
	}
	def := file.Members[0].(*ast.Membership).Member.(*ast.Definition)
	want := []struct {
		name           string
		wantIndividual bool
		wantSnapshot   bool
	}{
		{"subjectVehicle", true, false},
		{"vehicleState", false, true},
		{"payload", false, false},
	}
	if len(def.Members) != len(want) {
		t.Fatalf("expected %d parameters, got %d", len(want), len(def.Members))
	}
	for i, w := range want {
		u := def.Members[i].(*ast.Membership).Member.(*ast.Usage)
		if u.Ident.Name != w.name {
			t.Fatalf("parameter %d is %q, want %q", i, u.Ident.Name, w.name)
		}
		if u.IsIndividual != w.wantIndividual {
			t.Errorf("%s: IsIndividual = %t, want %t", w.name, u.IsIndividual, w.wantIndividual)
		}
		if u.IsSnapshot != w.wantSnapshot {
			t.Errorf("%s: IsSnapshot = %t, want %t", w.name, u.IsSnapshot, w.wantSnapshot)
		}
	}
}

// A declaration may be named with a kind keyword (`individual item : Integer`
// names the declaration `item`); the modifier path must not consume that
// keyword as the kind and leave the declaration anonymous.
func TestParseOccurrenceModifierKeywordName(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantName       string
		wantIndividual bool
		wantSnapshot   bool
	}{
		{"individual_typed", "individual item : Integer;", "item", true, false},
		{"individual_terminated", "individual part;", "part", true, false},
		{"individual_body", "individual state { }", "state", true, false},
		{"snapshot_typed", "snapshot item : Integer;", "item", false, true},
		{"snapshot_terminated", "snapshot part;", "part", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := parseSingleUsage(t, tt.input)
			if u.Ident.Name != tt.wantName {
				t.Errorf("name = %q, want %q", u.Ident.Name, tt.wantName)
			}
			if u.IsIndividual != tt.wantIndividual {
				t.Errorf("IsIndividual = %t, want %t", u.IsIndividual, tt.wantIndividual)
			}
			if u.IsSnapshot != tt.wantSnapshot {
				t.Errorf("IsSnapshot = %t, want %t", u.IsSnapshot, tt.wantSnapshot)
			}
		})
	}
}
