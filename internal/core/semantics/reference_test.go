package semantics

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

func TestReferencedFeature(t *testing.T) {
	m, root := buildModel(t, `package P {
		action takePicture { action focus; }
		part camera { perform action takePhoto references takePicture; }
	}`)

	pkg := sym(t, root, "P")
	camera := sym(t, pkg.Scope, "camera")
	takePhoto := sym(t, camera.Scope, "takePhoto")
	takePicture := sym(t, pkg.Scope, "takePicture")

	if got := m.ReferencedFeature(takePhoto); got != takePicture {
		t.Fatalf("ReferencedFeature(takePhoto) = %v, want takePicture", got)
	}
	if got := m.ReferencedFeature(takePicture); got != nil {
		t.Fatalf("ReferencedFeature(takePicture) = %v, want nil", got)
	}
}

// Reference subsetting contributes members without making the referenced
// feature a supertype: it is not a conformance edge (KerML 8.3.3.3.9).
func TestReferenceContributesMembersNotSupertypes(t *testing.T) {
	m, root := buildModel(t, `package P {
		action takePicture { action focus; action shoot; }
		part camera { perform action takePhoto references takePicture; }
	}`)

	pkg := sym(t, root, "P")
	camera := sym(t, pkg.Scope, "camera")
	takePhoto := sym(t, camera.Scope, "takePhoto")
	takePicture := sym(t, pkg.Scope, "takePicture")

	for _, name := range []string{"focus", "shoot"} {
		if _, ok := m.LookupMember(takePhoto, name); !ok {
			t.Errorf("LookupMember(takePhoto, %q) not found", name)
		}
	}
	if names := memberNames(m.MembersOf(takePhoto)); !names["focus"] || !names["shoot"] {
		t.Errorf("MembersOf(takePhoto) = %v, want focus and shoot", names)
	}

	for _, sup := range m.AllSupertypes(takePhoto) {
		if sup == takePicture {
			t.Fatalf("takePicture must not be a supertype of takePhoto")
		}
	}
	if !contains(m.MemberSources(takePhoto), takePicture) {
		t.Fatalf("MemberSources(takePhoto) = %v, want takePicture", m.MemberSources(takePhoto))
	}
}

// The performed action is named by a feature chain, and members inherited by
// the referenced feature are reachable through it.
func TestReferenceThroughFeatureChain(t *testing.T) {
	m, root := buildModel(t, `package P {
		action def ProvidePower { action generateTorque { action rev; } }
		action providePower : ProvidePower;
		part torqueGenerator { perform providePower.generateTorque; }
	}`)

	pkg := sym(t, root, "P")
	generator := sym(t, pkg.Scope, "torqueGenerator")
	perform := sym(t, generator.Scope, "generateTorque")

	if _, ok := m.LookupMember(perform, "rev"); !ok {
		t.Fatalf("LookupMember(generateTorque, rev) not found")
	}
}

// A perform statement binds the referenced feature's name in the scope the
// reference itself resolves in; the reference names the outer feature.
func TestReferenceResolvesOutsideItsOwnName(t *testing.T) {
	m, root := buildModel(t, `package P {
		action providePower { action generateTorque; }
		part vehicle { perform providePower; }
	}`)

	pkg := sym(t, root, "P")
	vehicle := sym(t, pkg.Scope, "vehicle")
	perform := sym(t, vehicle.Scope, "providePower")
	action := sym(t, pkg.Scope, "providePower")

	if perform == action {
		t.Fatalf("the perform statement should declare its own feature")
	}
	if got := m.ReferencedFeature(perform); got != action {
		t.Fatalf("ReferencedFeature(perform providePower) = %v, want the action", got)
	}
	if _, ok := m.LookupMember(perform, "generateTorque"); !ok {
		t.Fatalf("LookupMember(perform providePower, generateTorque) not found")
	}
}

// A reference cycle must not hang member lookup.
func TestReferenceCycleTerminates(t *testing.T) {
	m, root := buildModel(t, `package P {
		action a references b;
		action b references a;
	}`)

	pkg := sym(t, root, "P")
	if _, ok := m.LookupMember(sym(t, pkg.Scope, "a"), "nothing"); ok {
		t.Fatalf("unexpected member found")
	}
}

func TestGeneralizationKindExcludesReferences(t *testing.T) {
	if GeneralizationKind(ast.RelReferences) {
		t.Fatalf("reference subsetting must not be a generalization edge")
	}
}

func contains(syms []*symbols.Symbol, want *symbols.Symbol) bool {
	for _, s := range syms {
		if s == want {
			return true
		}
	}
	return false
}
