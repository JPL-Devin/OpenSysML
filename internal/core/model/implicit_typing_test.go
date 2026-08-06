package model

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/resolve"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// implicitBaseOf opens src in a stdlib-loaded workspace and returns the names of
// the direct supertypes the semantic model reports for the symbol reached by
// walking path from the document root.
func implicitBaseOf(t *testing.T, src string, path ...string) []string {
	t.Helper()
	const uri = "file:///implicit.sysml"
	ws := NewWorkspace()
	ws.Open(uri, []byte(src), 1)
	defer ws.Close(uri)

	scope := ws.index.DocumentRoot(uri)
	var sym *symbols.Symbol
	for _, part := range path {
		if scope == nil {
			t.Fatalf("no scope while looking up %q", part)
		}
		s, ok := scope.LookupLocal(part)
		if !ok {
			t.Fatalf("symbol %q not found", part)
		}
		sym, scope = s, s.Scope
	}

	r := resolve.New(ws.index)
	m := semantics.NewModel(r)
	r.SetModel(m)
	var names []string
	for _, sup := range m.DirectSupertypes(sym) {
		names = append(names, sup.Name)
	}
	return names
}

// TestImplicitUsageBaseTypes covers the standard library definition each kind of
// untyped usage is implicitly typed by, so members inherited from it resolve.
func TestImplicitUsageBaseTypes(t *testing.T) {
	cases := []struct {
		decl string
		want string
	}{
		{"part x;", "Parts::Part"},
		{"attribute x;", "Base::DataValue"},
		{"item x;", "Items::Item"},
		{"occurrence x;", "Occurrences::Occurrence"},
		{"individual occurrence x;", "Occurrences::Life"},
		{"port x;", "Ports::Port"},
		{"connection x;", "Connections::Connection"},
		{"interface x;", "Interfaces::Interface"},
		{"allocation x;", "Allocations::Allocation"},
		{"action x;", "Actions::Action"},
		{"state x;", "States::StateAction"},
		{"calc x;", "Calculations::Calculation"},
		{"constraint x;", "Constraints::ConstraintCheck"},
		{"requirement x;", "Requirements::RequirementCheck"},
		{"concern x;", "Requirements::ConcernCheck"},
		{"case x;", "Cases::Case"},
		{"analysis x;", "AnalysisCases::AnalysisCase"},
		{"verification x;", "VerificationCases::VerificationCase"},
		{"use case x;", "UseCases::UseCase"},
		{"view x;", "Views::View"},
		{"viewpoint x;", "Views::ViewpointCheck"},
		{"rendering x;", "Views::Rendering"},
		{"metadata x;", "Metadata::MetadataItem"},
	}

	for _, tc := range cases {
		t.Run(tc.decl, func(t *testing.T) {
			got := implicitBaseOf(t, "package P { "+tc.decl+" }", "P", "x")
			if len(got) != 1 || got[0] != tc.want {
				t.Fatalf("supertypes of %q = %v, want [%s]", tc.decl, got, tc.want)
			}
		})
	}
}

// TestImplicitBaseNotAppliedToTypedUsage covers the negative cases: a usage that
// declares its own type or specialization keeps exactly that supertype, and a
// definition never gets an implicit usage base.
func TestImplicitBaseNotAppliedToTypedUsage(t *testing.T) {
	cases := []struct {
		name string
		src  string
		path []string
		want string
	}{
		{"typed usage", "package P { part def Engine; part x : Engine; }", []string{"P", "x"}, "Engine"},
		{"subsetting usage", "package P { part y; part x subsets y; }", []string{"P", "x"}, "y"},
		{"specializing usage", "package P { part y; part x :> y; }", []string{"P", "x"}, "y"},
		{"definition", "package P { part def Engine :> Parts::Part; }", []string{"P", "Engine"}, "Parts::Part"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := implicitBaseOf(t, tc.src, tc.path...)
			if len(got) != 1 || got[0] != tc.want {
				t.Fatalf("supertypes = %v, want [%s]", got, tc.want)
			}
		})
	}
}

// TestImplicitBaseYieldsToImplicitRedefinition covers a usage whose name matches
// a feature its owner inherits: that feature supplies the type, so the usage
// must not be given the generic standard library base instead.
func TestImplicitBaseYieldsToImplicitRedefinition(t *testing.T) {
	src := `package P {
		part def Engine;
		part def Vehicle { part engine : Engine; }
		part v : Vehicle { part engine; }
	}`
	if got := implicitBaseOf(t, src, "P", "v", "engine"); len(got) != 0 {
		t.Fatalf("supertypes = %v, want none (implicit redefinition governs)", got)
	}
}

// TestInheritedMembersResolveThroughUntypedUsage covers the user-visible effect:
// members inherited from the implicit base resolve through an untyped usage,
// while a name no base declares still reports.
func TestInheritedMembersResolveThroughUntypedUsage(t *testing.T) {
	good := []struct {
		name string
		src  string
	}{
		{"state done", `package P {
			state machine {
				state normal;
				constraint { Time::TimeOf(normal.done) > 0 }
			}
		}`},
		{"state with body", `package P {
			state machine {
				state normal { entry; }
				constraint { Time::TimeOf(normal.start) > 0 }
			}
		}`},
		{"action done", `package P {
			action a;
			action b { constraint { Time::TimeOf(a.done) > 0 } }
		}`},
	}
	for _, tc := range good {
		t.Run(tc.name, func(t *testing.T) {
			if found := diagnose(t, "implicit_ok", tc.src); len(found) != 0 {
				t.Fatalf("expected no findings, got %v", found)
			}
		})
	}

	bad := `package P {
		state machine {
			state normal;
			constraint { Time::TimeOf(normal.notAMember) > 0 }
		}
	}`
	if found := diagnose(t, "implicit_bad", bad); len(found) != 1 {
		t.Fatalf("expected one finding for the undeclared member, got %d: %v", len(found), found)
	}
}
