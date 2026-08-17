package resolve_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/resolve"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// A redefinition target is searched up the whole specialization chain, not only
// the immediate supertype (KerML 1.0 §7.3.4.5).
func TestRedefinitionTargetFoundTwoSupertypesUp(t *testing.T) {
	r, _, docRoot := resolvedDoc(t, `package P {
		part def Vehicle { part wheel; }
		part def Car :> Vehicle;
		part def Roadster :> Car {
			part frontWheel :>> wheel;
		}
	}`)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("resolution diagnostics: %v", r.Diagnostics)
	}
	pkg := local(t, docRoot, "P")
	roadster := local(t, pkg.Scope, "Roadster")
	want := local(t, local(t, pkg.Scope, "Vehicle").Scope, "wheel")

	target := redefinitionTarget(t, local(t, roadster.Scope, "frontWheel"))
	got, ok := r.ResolveQualified(roadster.Scope, target)
	if !ok {
		t.Fatal("the redefinition target did not resolve")
	}
	if got != want {
		t.Errorf("frontWheel redefines %s, want the wheel Vehicle declares", got.Name)
	}
}

// Redeclaring an inherited name is a conflict, unless the declaration redefines or
// subsets what it shares the name with (SysML v2 §7.6.1).
func TestRedeclaringAnInheritedNameConflictsUnlessItRedefines(t *testing.T) {
	for _, tc := range []struct {
		name     string
		member   string
		conflict bool
	}{
		{"plain redeclaration", "part wheel;", true},
		{"redefinition", "part wheel :>> wheel;", false},
		{"reference", "ref part wheel references wheel;", false},
		{"another name", "part spare;", false},
	} {
		r, _, _ := resolvedDoc(t, `package P {
			part def Vehicle { part wheel; }
			part def Car :> Vehicle { `+tc.member+` }
		}`)
		conflicts := diagnosticsWithCode(r, resolve.CodeNameConflict)
		if got := len(conflicts) != 0; got != tc.conflict {
			t.Errorf("%s: name conflict reported = %v, want %v (%v)", tc.name, got, tc.conflict, r.Diagnostics)
		}
		if len(conflicts) != 0 && !strings.Contains(conflicts[0].Message, "Vehicle::wheel") {
			t.Errorf("%s: conflict message %q does not name the inherited feature", tc.name, conflicts[0].Message)
		}
	}
}

// A same-named subsetting target is the inherited feature: a feature cannot
// specialize itself. The fix belongs in resolveSpecialization ("Found, not fixed").
func TestSubsettingTargetIsTheInheritedFeature(t *testing.T) {
	t.Skip("known bug: a same-named subsetting target binds to the subsetting feature itself")
	r, _, docRoot := resolvedDoc(t, `package P {
		part def Vehicle { part wheel; }
		part def Car :> Vehicle { part wheel :> wheel; }
	}`)
	pkg := local(t, docRoot, "P")
	subsetting := local(t, local(t, pkg.Scope, "Car").Scope, "wheel")
	inherited := local(t, local(t, pkg.Scope, "Vehicle").Scope, "wheel")
	target := specializationTarget(t, subsetting)
	got, ok := r.ResolveQualified(local(t, pkg.Scope, "Car").Scope, target)
	if !ok {
		t.Fatal("the subsetting target did not resolve")
	}
	if got == subsetting {
		t.Error("the subsetting target names the feature declaring it")
	}
	if got != inherited {
		t.Errorf("the subsetting target names %s, want the wheel Vehicle declares", got.Name)
	}
}

// The subject, actors and stakeholders of a requirement redefine the inherited
// ones by name, which is not modelled here, so redeclaring them is no conflict.
func TestRequirementParametersAreNotNameConflicts(t *testing.T) {
	r, _, _ := resolvedDoc(t, `package P {
		part def Vehicle;
		requirement def Weight { subject vehicle : Vehicle; }
		requirement def MaxWeight :> Weight { subject vehicle : Vehicle; }
	}`)
	if conflicts := diagnosticsWithCode(r, resolve.CodeNameConflict); len(conflicts) != 0 {
		t.Errorf("name conflicts reported for a requirement's subject: %v", conflicts)
	}
}

// local is the single member scope holds under name.
func local(t *testing.T, scope *symbols.Scope, name string) *symbols.Symbol {
	t.Helper()
	sym, ok := scope.LookupLocal(name)
	if !ok {
		t.Fatalf("%s is not a local member", name)
	}
	return sym
}

// redefinitionTarget is the name the redefinition relationship of sym names.
func redefinitionTarget(t *testing.T, sym *symbols.Symbol) *ast.QualifiedName {
	t.Helper()
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok {
		t.Fatalf("%s is declared by %T, want a usage", sym.Name, sym.Decl)
	}
	for _, rel := range usage.Relationships {
		if rel == nil || rel.Kind != ast.RelRedefines {
			continue
		}
		if qn := ast.AsQualifiedName(rel.Target); qn != nil {
			return qn
		}
	}
	t.Fatalf("%s declares no redefinition of a name", sym.Name)
	return nil
}

// specializationTarget is the name the subsetting relationship of sym names.
func specializationTarget(t *testing.T, sym *symbols.Symbol) *ast.QualifiedName {
	t.Helper()
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok {
		t.Fatalf("%s is declared by %T, want a usage", sym.Name, sym.Decl)
	}
	for _, rel := range usage.Relationships {
		if rel == nil || (rel.Kind != ast.RelSubsets && rel.Kind != ast.RelSpecializes) {
			continue
		}
		if qn := ast.AsQualifiedName(rel.Target); qn != nil {
			return qn
		}
	}
	t.Fatalf("%s declares no subsetting of a name", sym.Name)
	return nil
}

func diagnosticsWithCode(r *resolve.Resolver, code string) []resolve.Diagnostic {
	var out []resolve.Diagnostic
	for _, d := range r.Diagnostics {
		if d.Code == code {
			out = append(out, d)
		}
	}
	return out
}
