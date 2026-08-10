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
		{"snapshot_occurrence", "snapshot occurrence takeoff : Flight;", ast.UsageOccurrence, false, true},
		{"snapshot_part", "snapshot part vehicleAtTakeoff : Vehicle;", ast.UsagePart, false, true},
		{"plain_usage", "occurrence takeoff : Flight;", ast.UsageOccurrence, false, false},
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
