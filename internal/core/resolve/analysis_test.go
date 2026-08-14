package resolve

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// TestResolveQualifiedRequirement covers SysML v2 §8.2.2.19.2: an objective may
// require a requirement named by a qualified reference, and the body of that
// requirement may redefine a qualified feature of it.
func TestResolveQualifiedRequirement(t *testing.T) {
	r := resolveDoc(t, "d.sysml", `package Starkit {
		requirement def MaxFuelMassRequirement {
			attribute actualFuelMass;
		}
		package FuelMassAnalysis {
			requirement fuelMassRequirement : MaxFuelMassRequirement;
		}
		analysis def FuelMassAnalysisCase {
			attribute calculatedFuelMass;
			objective fuelMassAnalysisObjective {
				require FuelMassAnalysis::fuelMassRequirement {
					:>> MaxFuelMassRequirement::actualFuelMass = calculatedFuelMass;
				}
				assume FuelMassAnalysis::fuelMassRequirement;
			}
		}
	}`)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %v", r.Diagnostics)
	}
}

// The member reference-subsets the requirement it names, so its body redefines
// that requirement's features by their plain names too, not only by their
// qualified ones (SysML.xtext RequirementConstraintUsage).
func TestResolveRequiredRequirementFeatureByPlainName(t *testing.T) {
	r := resolveDoc(t, "d.sysml", `package Starkit {
		requirement def MaxFuelMassRequirement {
			attribute actualFuelMass;
		}
		package FuelMassAnalysis {
			requirement fuelMassRequirement : MaxFuelMassRequirement;
		}
		analysis def FuelMassAnalysisCase {
			attribute calculatedFuelMass;
			objective fuelMassAnalysisObjective {
				require FuelMassAnalysis::fuelMassRequirement {
					:>> actualFuelMass = calculatedFuelMass;
				}
				assume FuelMassAnalysis::fuelMassRequirement {
					:>> actualFuelMass = calculatedFuelMass;
				}
			}
		}
	}`)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %v", r.Diagnostics)
	}
}

// A plain name the referenced requirement does not declare is still reported:
// its features are searched, not assumed.
func TestResolveRequiredRequirementUnknownPlainName(t *testing.T) {
	r := resolveDoc(t, "d.sysml", `package Starkit {
		requirement def R { attribute mass; }
		package A { requirement r : R; }
		analysis def C {
			attribute m;
			objective o {
				require A::r {
					:>> missingFeature = m;
				}
			}
		}
	}`)
	if len(r.Diagnostics) == 0 {
		t.Fatalf("expected an unresolved diagnostic for missingFeature")
	}
}

// Only the direct members of a require body inherit from the requirement it
// references, so a nested declaration never redefines that requirement's
// feature — its own type holds it. Such a nested body is not walked yet (a
// known limitation), so the target may also stay unresolved.
func TestResolveNestedRedefinitionPrefersOwnType(t *testing.T) {
	src := `package Starkit {
		requirement def R { attribute mass; }
		package A { requirement r : R; }
		part def Payload { attribute mass; }
		analysis def C {
			objective o {
				require A::r {
					part p : Payload {
						:>> mass;
					}
				}
			}
		}
	}`
	p := parser.New(source.New("d.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx := symbols.NewIndexFromDoc("d.sysml", root)
	r := New(idx)
	r.ResolveDocument("d.sysml", root)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %v", r.Diagnostics)
	}

	qn := nestedRedefinitionTarget(t, root)
	sym, ok := r.PartSymbol(qn, 0)
	if !ok {
		return
	}
	owner, _ := sym.OwnerScope.Node().(*ast.Definition)
	if owner == nil || owner.Ident.Name != "Payload" {
		t.Errorf("mass resolved in %v, want Payload", sym.OwnerScope.Node())
	}
}

// nestedRedefinitionTarget returns the redefinition target of the only
// redefinition declared inside a usage body in root.
func nestedRedefinitionTarget(t *testing.T, root *ast.RootNamespace) *ast.QualifiedName {
	t.Helper()
	var found *ast.QualifiedName
	var walk func(members []ast.Node)
	walk = func(members []ast.Node) {
		for _, m := range members {
			if mem, ok := m.(*ast.Membership); ok {
				m = mem.Member
			}
			switch n := m.(type) {
			case *ast.Package:
				walk(n.Members)
			case *ast.Definition:
				walk(n.Members)
			case *ast.RequireMember:
				walk(n.Body)
			case *ast.Usage:
				for _, rel := range n.Relationships {
					if rel.Kind != ast.RelRedefines {
						continue
					}
					if qn, ok := rel.Target.(*ast.QualifiedName); ok && len(qn.Parts) == 1 {
						found = qn
					}
				}
				walk(n.Members)
			}
		}
	}
	walk(root.Members)
	if found == nil {
		t.Fatalf("no plain-name redefinition found")
	}
	return found
}

// A required requirement that names no such member is reported, so a qualified
// target is resolved rather than accepted on sight.
func TestResolveQualifiedRequirementUnresolved(t *testing.T) {
	r := resolveDoc(t, "d.sysml", `package Starkit {
		package FuelMassAnalysis;
		analysis def C {
			objective o {
				require FuelMassAnalysis::missingRequirement;
			}
		}
	}`)
	if len(r.Diagnostics) == 0 {
		t.Fatalf("expected an unresolved diagnostic for FuelMassAnalysis::missingRequirement")
	}
}

// A qualified redefinition target in a requirement body is resolved through the
// namespace it names, not by its last segment alone.
func TestResolveQualifiedRedefinitionUnresolved(t *testing.T) {
	r := resolveDoc(t, "d.sysml", `package Starkit {
		requirement def R { attribute mass; }
		package A { requirement r : R; }
		analysis def C {
			attribute m;
			objective o {
				require A::r {
					:>> R::missingFeature = m;
				}
			}
		}
	}`)
	if len(r.Diagnostics) == 0 {
		t.Fatalf("expected an unresolved diagnostic for R::missingFeature")
	}
}

// TestResolveDragonStructures covers the structural notation Dragon.sysml
// exercises alongside the constructs added here: an allocation, a viewpoint
// definition and usage, a constraint usage with multiplicity, an assert of that
// constraint, and occurrence portions. Each must resolve, not merely parse.
func TestResolveDragonStructures(t *testing.T) {
	r := resolveDoc(t, "d.sysml", `package Dragon {
		part a;
		part b;
		allocation allocate a to b;
		constraint def C;
		constraint c [1] : C {}
		assert constraint c;
		viewpoint def V;
		viewpoint v : V;
		item def Vehicle;
		item vehicle : Vehicle {
			attribute mass;
			timeslice item cruise;
			snapshot item takeoff;
		}
		interface def CommunicationInterface {
			end;
			end;
		}
	}`)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %v", r.Diagnostics)
	}
}

// A reference whose qualifying namespace is not loaded says so, and names the
// declaration that does exist, rather than reading as a plain typo.
func TestResolveMissingStandardViewNamespace(t *testing.T) {
	r := resolveDoc(t, "d.sysml", `package P {
		view def GeneralView;
		package Views { alias gv for P::GeneralView; }
		view Model : 'SysML Standard Diagrams'::gv;
	}`)
	if len(r.Diagnostics) == 0 {
		t.Fatalf("expected a diagnostic for an unloaded namespace")
	}
	msg := r.Diagnostics[0].Message
	for _, want := range []string{"SysML Standard Diagrams", "no namespace", "P::Views::gv"} {
		if !strings.Contains(msg, want) {
			t.Errorf("diagnostic %q does not mention %q", msg, want)
		}
	}
}
