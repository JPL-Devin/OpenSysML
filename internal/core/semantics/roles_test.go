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
	got := m.ImplicitRoleRedefinitions(cSubject)
	if len(got) != 2 || got[0] != aSubject || got[1] != bSubject {
		t.Fatalf("ImplicitRoleRedefinitions(C's subject) = %v, want [A::s B::s]", got)
	}
}
