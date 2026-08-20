package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

// TestImplicitBaseNeedsTheLibrary covers a model resolved without the standard
// library: no base is in the index, so an untyped usage stays untyped rather
// than reporting a phantom supertype.
func TestImplicitBaseWithoutLibrary(t *testing.T) {
	m, root := buildModel(t, "part p;")
	if supers := m.DirectSupertypes(sym(t, root, "p")); len(supers) != 0 {
		t.Fatalf("DirectSupertypes(p) = %v, want none without the library", supers)
	}
}

// TestImplicitBaseResolvesThroughIndex covers the lookup itself against a
// stand-in library declared in the same document.
func TestImplicitBaseResolvesThroughIndex(t *testing.T) {
	m, root := buildModel(t, "package Parts { part def Part; } part p; part q : Parts::Part;")
	parts := sym(t, root, "Parts")
	part, _ := parts.Scope.LookupLocal("Part")

	supers := m.DirectSupertypes(sym(t, root, "p"))
	if len(supers) != 1 || supers[0] != part {
		t.Fatalf("DirectSupertypes(p) = %v, want [Parts::Part]", supers)
	}
	if supers := m.DirectSupertypes(sym(t, root, "q")); len(supers) != 1 || supers[0] != part {
		t.Fatalf("DirectSupertypes(q) = %v, want the declared [Parts::Part] only", supers)
	}
}

// TestUsageKindsWithoutImplicitBase pins the kinds deliberately left out of the
// mapping: connector, succession, flow, binding, satisfy, subject and objective
// take their type from the element they relate to, and the KerML structural
// kinds are definitions in usage position.
func TestUsageKindsWithoutImplicitBase(t *testing.T) {
	for _, k := range []ast.UsageKind{
		ast.UsageConnector, ast.UsageSuccession, ast.UsageFlow, ast.UsageBinding,
		ast.UsageSatisfy, ast.UsageSubject, ast.UsageObjective, ast.UsageInteraction,
		ast.UsageBehavior, ast.UsageAssoc, ast.UsageStruct, ast.UsageClass,
		ast.UsagePredicate, ast.UsageBool,
	} {
		if fqn, ok := implicitUsageBases[k]; ok {
			t.Errorf("usage kind %v unexpectedly has implicit base %q", k, fqn)
		}
	}
}

// TestImplicitBaseUsageContributesThat covers SysML v2 §7.6: every usage element
// subsets the most general base usage, which is what makes `that` visible in a
// usage body. The base usage contributes members only — it is not a supertype.
func TestImplicitBaseUsageContributesThat(t *testing.T) {
	m, root := buildModel(t, `package Base { abstract feature things { feature that; } }
		package Parts { part def Part; }
		part p : Parts::Part;`)
	p := sym(t, root, "p")

	if _, ok := m.LookupMember(p, "that"); !ok {
		t.Errorf("`that` is not a member of a usage")
	}
	parts := sym(t, root, "Parts")
	part, _ := parts.Scope.LookupLocal("Part")
	if supers := m.DirectSupertypes(p); len(supers) != 1 || supers[0] != part {
		t.Errorf("DirectSupertypes(p) = %v, want the declared [Parts::Part] only", supers)
	}
	base := sym(t, root, "Base")
	things, _ := base.Scope.LookupLocal("things")
	if m.Conforms(p, things) {
		t.Errorf("a usage reported conforming to the base usage")
	}
	// A definition is not a usage element and takes nothing from the base usage.
	if _, ok := m.LookupMember(part, "that"); ok {
		t.Errorf("`that` reported as a member of a definition")
	}
}

func TestKerMLImplicitDefinitionBases(t *testing.T) {
	m, root := buildModelNamed(t, "t.kerml", `package P {
		class C;
		behavior B;
		datatype D;
		feature F;
	}
	package Base { classifier Anything; datatype DataValue; }
	package Occurrences { classifier Occurrence; }
	package Objects { classifier Object; }
	package Links { classifier Link; }
	package Performances {
		classifier Performance;
		classifier Evaluation;
		classifier BooleanEvaluation;
	}
	package Transfers { classifier Transfer; }
	package Metaobjects { classifier Metaobject; }`)
	p := sym(t, root, "P")
	for _, tc := range []struct {
		name string
		want string
	}{
		{"C", "Occurrences::Occurrence"},
		{"B", "Performances::Performance"},
		{"D", "Base::DataValue"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target := sym(t, p.Scope, tc.name)
			got := m.DirectSupertypes(target)
			if len(got) != 1 || m.resolver.Index().GetFQN(got[0]) != tc.want {
				t.Fatalf("supertypes of %s = %v, want [%s]", tc.name, got, tc.want)
			}
		})
	}
	if got := m.DirectSupertypes(sym(t, p.Scope, "F")); len(got) != 0 {
		t.Fatalf("KerML feature received an implicit datatype base: %v", got)
	}
}

func TestSysMLImplicitDefinitionBasesRemainKindBased(t *testing.T) {
	m, root := buildModelNamed(t, "t.sysml", `package P {
		part def C;
		action def A;
		attribute def D;
	}
	package Parts { part def Part; }
	package Actions { action def Action; }
	package Base { attribute def DataValue; }`)
	p := sym(t, root, "P")
	for _, tc := range []struct {
		name string
		want string
	}{
		{"C", "Parts::Part"},
		{"A", "Actions::Action"},
		{"D", "Base::DataValue"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target := sym(t, p.Scope, tc.name)
			got := m.DirectSupertypes(target)
			if len(got) != 1 || m.resolver.Index().GetFQN(got[0]) != tc.want {
				t.Fatalf("supertypes of %s = %v, want [%s]", tc.name, got, tc.want)
			}
		})
	}
}

func TestImplicitBaseOfExplicitSupertypeIsTransitive(t *testing.T) {
	m, root := buildModelNamed(t, "t.kerml", `package Occurrences {
		class Occurrence { feature endShot; }
	}
	package Objects {
		struct Object specializes Occurrences::Occurrence;
	}
	struct Wheel;
	struct MyWheel specializes Wheel {
		feature redefines endShot : EndShot;
	}
	struct EndShot;`)
	myWheel := sym(t, root, "MyWheel")
	supers := m.AllSupertypes(myWheel)
	if len(supers) < 3 {
		var fqns []string
		for _, super := range supers {
			fqns = append(fqns, m.resolver.Index().GetFQN(super))
		}
		t.Fatalf("AllSupertypes(MyWheel) = %v, want Wheel, Objects::Object, and Occurrences::Occurrence", fqns)
	}
	if inherited, ok := m.LookupContributedMember(myWheel, "endShot"); !ok {
		t.Fatal("MyWheel does not inherit endShot through Wheel's implicit base")
	} else if got := m.resolver.Index().GetFQN(inherited); got != "Occurrences::Occurrence::endShot" {
		t.Fatalf("inherited endShot = %q, want Occurrences::Occurrence::endShot", got)
	}
}

func TestSysMLImplicitBaseOfExplicitSupertypeIsTransitive(t *testing.T) {
	m, root := buildModelNamed(t, "t.sysml", `package Parts {
		part def Part { feature endShot; }
	}
	part def Wheel;
	part def MyWheel specializes Wheel {
		feature redefines endShot;
	}`)
	myWheel := sym(t, root, "MyWheel")
	if inherited, ok := m.LookupContributedMember(myWheel, "endShot"); !ok {
		t.Fatal("MyWheel does not inherit endShot through Wheel's implicit base")
	} else if got := m.resolver.Index().GetFQN(inherited); got != "Parts::Part::endShot" {
		t.Fatalf("inherited endShot = %q, want Parts::Part::endShot", got)
	}
}
