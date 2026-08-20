package resolve_test

import "testing"

// f67Clean resolves src and requires it to produce no diagnostics.
func f67Clean(t *testing.T, name, src string) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		r, _, _ := resolvedDoc(t, src)
		if len(r.Diagnostics) != 0 {
			for _, d := range r.Diagnostics {
				t.Logf("diag: %s", d.Message)
			}
			t.Error("unexpected diagnostics above")
		}
	})
}

// A name a wildcard import introduces into a namespace can itself be the
// target of a sibling import (F67: `private import RiskLevelEnum::*;` after
// the enum arrived through `private import RiskMetadata::*;`).
func TestF67ImportOfImportIntroducedName(t *testing.T) {
	f67Clean(t, "wildcard-then-wildcard", `package M {
		package RiskMeta {
			enum def E { low; medium; high; }
		}
		package Use {
			private import RiskMeta::*;
			private import E::*;
			attribute x = low;
		}
	}`)
	f67Clean(t, "membership-then-wildcard", `package M {
		package RiskMeta {
			enum def E { low; medium; high; }
		}
		package Use {
			private import RiskMeta::E;
			private import E::*;
			attribute x = low;
		}
	}`)
}

// A public wildcard import re-exports, so the name reaches a second importer
// through the chain (F67: `Diameter` in VehicleVariabilityModel's '150% Model',
// visible via `DesignModel::*` because DesignModel publicly imports
// `PartDefinitions::*`).
func TestF67WildcardReexportChain(t *testing.T) {
	f67Clean(t, "two-level-chain", `package M {
		package Inner { part def D; }
		package Mid { public import Inner::*; }
		package Use {
			private import Mid::*;
			part x : D;
		}
	}`)
}

// A private wildcard import does not re-export (KerML 8.2.3.3): the chained
// lookup must still miss when the middle import is private.
func TestF67PrivateImportDoesNotReexport(t *testing.T) {
	r, _, _ := resolvedDoc(t, `package M {
		package Inner { part def D; }
		package Mid { private import Inner::*; }
		package Use {
			private import Mid::*;
			part x : D;
		}
	}`)
	if len(r.Diagnostics) == 0 {
		t.Fatal("expected an unresolved-reference diagnostic through the private import chain")
	}
}

// Subsetting a feature reachable only by feature chain contributes the chain's
// final feature as a generalization, so its members resolve in the subsetter's
// body (F67: `part c subsets b.f { part aa subsets a; }`).
func TestF67FeatureChainSubsettingContributesMembers(t *testing.T) {
	f67Clean(t, "chain-subsetting", `package Q {
		part def F { part a; }
		part b { part f : F; }
		part c subsets b.f {
			part aa subsets a;
		}
	}`)
}

// An included use case contributes its members through the inclusion's
// reference subsetting (SysML.xtext IncludeUseCaseUsage), so its actors may be
// redefined in the include's body (F67: `actor :>> fueler = driver;`).
func TestF67IncludeUseCaseContributesActors(t *testing.T) {
	f67Clean(t, "include-actor-redef", `package U {
		part def Person;
		part def Vehicle;
		use case 'add fuel' {
			subject v : Vehicle;
			actor fueler : Person;
		}
		use case 'drive vehicle' {
			subject v : Vehicle;
			actor driver : Person;
			include 'add fuel' {
				subject;
				actor :>> fueler = driver;
			}
		}
	}`)
}

// A bare `variant X` is a VariantReference (SysML.xtext VariantReference): it
// reference-subsets the like-named feature visible outside the variation, so
// that feature's members resolve in the variant's body (F67:
// `variant '6cylEngine' { variation port :>> autoPort { … } }`).
func TestF67VariantReferenceContributesMembers(t *testing.T) {
	f67Clean(t, "variant-reference", `package V {
		port def AutoPort;
		part def Engine;
		package PartsTree {
			part engine : Engine {
				port autoPort : AutoPort;
			}
			part '6cylEngine' :> engine;
		}
		package Model150 {
			private import PartsTree::*;
			variation part def EngineChoices :> Engine {
				variant '4cylEngine';
				variant '6cylEngine' {
					variation port :>> autoPort {
						variant port autoPort1;
						variant port autoPort2;
					}
				}
			}
		}
	}`)
}

// A bare variant with no like-named outer feature stays a plain declaration:
// no reference is fabricated and nothing panics.
func TestF67VariantWithoutOuterNameNoPanic(t *testing.T) {
	r, _, _ := resolvedDoc(t, `package V {
		part def Engine;
		variation part def EngineChoices :> Engine {
			variant onlyHere;
		}
	}`)
	_ = r.Diagnostics
}

// An unresolvable feature chain target must degrade to diagnostics, not panic.
func TestF67FeatureChainSubsettingUnresolvedNoPanic(t *testing.T) {
	r, _, _ := resolvedDoc(t, `package Q {
		part c subsets nope.f {
			part aa subsets a;
		}
	}`)
	if len(r.Diagnostics) == 0 {
		t.Fatal("expected diagnostics for an unresolved feature chain target")
	}
}
