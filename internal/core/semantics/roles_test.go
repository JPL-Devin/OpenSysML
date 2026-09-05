package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

func TestImplicitRoleRedefinitionDeduplicatesDiamond(t *testing.T) {
	m, root := buildModel(t, `package P {
		verification def Base {
			objective baseObjective { attribute inherited; }
		}
		verification def Left :> Base;
		verification def Right :> Base;
		verification def Derived :> Left, Right {
			objective derivedObjective;
		}
	}`)
	p := sym(t, root, "P")
	baseObjective := nested(t, nested(t, p.Scope, "Base").Scope, "baseObjective")
	derivedObjective := nested(t, nested(t, p.Scope, "Derived").Scope, "derivedObjective")

	got := m.ImplicitRoleRedefinitions(derivedObjective)
	if len(got) != 1 || got[0] != baseObjective {
		t.Fatalf("ImplicitRoleRedefinitions(derivedObjective) = %v, want [baseObjective]", got)
	}
}

// A role redefines the same role of every general: naming one by `:>>` leaves
// the others implicit, and naming something else leaves them all implicit.
func TestImplicitRoleRedefinitionSkipsOnlyTheExplicitlyRedefinedRole(t *testing.T) {
	m, root := buildModel(t, `package P {
		requirement def Weight { subject w; attribute limit; }
		requirement def Volume { subject vol; }
		requirement def Both :> Weight, Volume { subject s :>> w; }
		requirement def Neither :> Weight, Volume { subject s :>> limit; }
	}`)
	p := sym(t, root, "P")
	w := nested(t, nested(t, p.Scope, "Weight").Scope, "w")
	vol := nested(t, nested(t, p.Scope, "Volume").Scope, "vol")

	both := nested(t, nested(t, p.Scope, "Both").Scope, "s")
	if got := m.ImplicitRoleRedefinitions(both); len(got) != 1 || got[0] != vol {
		t.Errorf("ImplicitRoleRedefinitions(Both::s) = %v, want [vol]", got)
	}
	neither := nested(t, nested(t, p.Scope, "Neither").Scope, "s")
	if got := m.ImplicitRoleRedefinitions(neither); len(got) != 2 || got[0] != w || got[1] != vol {
		t.Errorf("ImplicitRoleRedefinitions(Neither::s) = %v, want [w vol]", got)
	}
}

func TestBoundSubjectInheritsTheRedefinedSubjectsMembers(t *testing.T) {
	m, root := buildModel(t, `package P {
		part def Truck { attribute payload; }
		requirement def Req { subject truck : Truck; }
		part truck : Truck;
		requirement bound : Req { subject truck = truck; }
	}`)
	p := sym(t, root, "P")
	subject := nested(t, nested(t, p.Scope, "bound").Scope, "truck")
	payload := nested(t, nested(t, p.Scope, "Truck").Scope, "payload")

	got, ok := m.LookupMember(subject, "payload")
	if !ok || got != payload {
		t.Fatalf("LookupMember(bound's subject, %q) = %v, %v, want the truck's payload", "payload", got, ok)
	}
}

func TestImplicitRoleRedefinitionMatchesRole(t *testing.T) {
	m, root := buildModel(t, `package P {
		verification def Base {
			subject baseSubject;
			objective baseObjective;
		}
		verification def Derived :> Base {
			objective derivedObjective;
		}
	}`)
	p := sym(t, root, "P")
	base := nested(t, p.Scope, "Base")
	baseObjective := nested(t, base.Scope, "baseObjective")
	baseSubject := nested(t, base.Scope, "baseSubject")
	derivedObjective := nested(t, nested(t, p.Scope, "Derived").Scope, "derivedObjective")

	got := m.ImplicitRoleRedefinitions(derivedObjective)
	if len(got) != 1 || got[0] != baseObjective {
		t.Fatalf("ImplicitRoleRedefinitions(derivedObjective) = %v, want [baseObjective]", got)
	}
	if got[0] == baseSubject {
		t.Fatal("objective implicitly redefined the inherited subject")
	}
}

func TestImplicitRoleRedefinitionSurvivesExplicitRedefinition(t *testing.T) {
	m, root := buildModel(t, `package P {
		part def X;
		requirement def A { subject s : X; }
		requirement def B { subject s : X; }
		requirement def C :> A, B { subject s :>> A::s; }
	}`)
	p := sym(t, root, "P")
	aSubject := nested(t, nested(t, p.Scope, "A").Scope, "s")
	bSubject := nested(t, nested(t, p.Scope, "B").Scope, "s")
	cSubject := nested(t, nested(t, p.Scope, "C").Scope, "s")

	if got := RelationshipsOf(cSubject); len(got) != 1 || got[0].Kind != ast.RelRedefines {
		t.Fatalf("RelationshipsOf(C's subject) = %v, want its one redefinition", got)
	}
	if got := m.ImplicitRoleRedefinitions(cSubject); len(got) != 1 || got[0] != bSubject {
		t.Fatalf("ImplicitRoleRedefinitions(C's subject) = %v, want [B::s]", got)
	}
	got := m.AllRedefinedFeatures(cSubject)
	if len(got) != 2 || got[0] != aSubject || got[1] != bSubject {
		t.Fatalf("AllRedefinedFeatures(C's subject) = %v, want [A::s B::s]", got)
	}
}

// Only the first owned objective redefines, and it takes the first objective
// of each general; ObjectivesOf then reports what remains inherited.
func TestObjectivesOfMasksOnlyTheFirstOfEachGeneral(t *testing.T) {
	m, root := buildModel(t, `package P {
		case def C { objective o1; objective o2; }
		case def C1 :> C { objective o5; }
		case def B1 { objective b1; }
		case def B2 { objective b2; }
		case def D :> B1, B2;
		case def D2 :> B1, B2 { objective d; }
		case def E :> B1 { objective e1; objective e2; }
		case def C3 :> C { objective o6 :>> o2; }
	}`)
	p := sym(t, root, "P")
	c := nested(t, p.Scope, "C")
	o1 := nested(t, c.Scope, "o1")
	o2 := nested(t, c.Scope, "o2")
	b1 := nested(t, nested(t, p.Scope, "B1").Scope, "b1")
	b2 := nested(t, nested(t, p.Scope, "B2").Scope, "b2")

	o5 := nested(t, nested(t, p.Scope, "C1").Scope, "o5")
	if got := m.ImplicitRoleRedefinitions(o5); len(got) != 1 || got[0] != o1 {
		t.Errorf("ImplicitRoleRedefinitions(C1::o5) = %v, want [o1]", got)
	}
	if owned, inherited := m.ObjectivesOf(nested(t, p.Scope, "C1")); len(owned) != 1 || len(inherited) != 1 || inherited[0] != o2 {
		t.Errorf("ObjectivesOf(C1) = %v, %v; want [o5], [o2]", owned, inherited)
	}
	if owned, inherited := m.ObjectivesOf(nested(t, p.Scope, "D")); len(owned) != 0 || len(inherited) != 2 || inherited[0] != b1 || inherited[1] != b2 {
		t.Errorf("ObjectivesOf(D) = %v, %v; want [], [b1 b2]", owned, inherited)
	}
	if owned, inherited := m.ObjectivesOf(nested(t, p.Scope, "D2")); len(owned) != 1 || len(inherited) != 0 {
		t.Errorf("ObjectivesOf(D2) = %v, %v; want [d], []", owned, inherited)
	}
	e := nested(t, p.Scope, "E")
	if got := m.ImplicitRoleRedefinitions(nested(t, e.Scope, "e2")); len(got) != 0 {
		t.Errorf("ImplicitRoleRedefinitions(E::e2) = %v, want none: only the first owned objective redefines", got)
	}
	if owned, inherited := m.ObjectivesOf(e); len(owned) != 2 || len(inherited) != 0 {
		t.Errorf("ObjectivesOf(E) = %v, %v; want [e1 e2], []", owned, inherited)
	}
	if owned, inherited := m.ObjectivesOf(nested(t, p.Scope, "C3")); len(owned) != 1 || len(inherited) != 0 {
		t.Errorf("ObjectivesOf(C3) = %v, %v; want [o6], []: o6 redefines o2 by clause and o1 by role", owned, inherited)
	}
}
