package resolve

import (
	"strings"
	"testing"
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
