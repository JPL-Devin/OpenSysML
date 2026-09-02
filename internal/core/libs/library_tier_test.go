package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// A bundled file is classified by the bundle directory it sits under, however
// the path is spelled; one outside the layout is library content of no tier.
func TestTierOf(t *testing.T) {
	for name, want := range map[string]symbols.LibraryTier{
		"Kernel Libraries/Kernel Semantic Library/Occurrences.kerml":   symbols.TierKernelSemantic,
		"Kernel Libraries/Kernel Data Type Library/ScalarValues.kerml": symbols.TierKernelDataType,
		"Kernel Libraries/Kernel Function Library/BaseFunctions.kerml": symbols.TierKernelFunction,
		"Systems Library/Items.sysml":                                  symbols.TierSystems,
		"Domain Libraries/Geometry/ShapeItems.sysml":                   symbols.TierDomain,
		"OpenSysML Libraries/Units.sysml":                              symbols.TierOpenSysML,
		"Kernel Libraries/Other.kerml":                                 symbols.TierLibrary,
		"Systems Library":                                              symbols.TierLibrary,
		"custom/Extra.sysml":                                           symbols.TierLibrary,
	} {
		if got := TierOf(name); got != want {
			t.Errorf("TierOf(%q) = %s, want %s", name, got, want)
		}
	}
}

// Every declaration of the bundled library carries its file's tier, whether
// the index was loaded from source or decoded from the embedded snapshot.
func TestLoadedLibraryCarriesTiers(t *testing.T) {
	fresh := symbols.NewIndex()
	if err := NewLoader(EmbeddedSource(), nil).LoadAll(fresh); err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	decoded, err := SnapshotIndex()
	if err != nil {
		t.Fatalf("SnapshotIndex: %v", err)
	}
	want := map[string]symbols.LibraryTier{
		"Occurrences::Occurrence::self":         symbols.TierKernelSemantic,
		"Occurrences::Occurrence::timeSlices":   symbols.TierKernelSemantic,
		"ScalarValues::Real":                    symbols.TierKernelDataType,
		"BaseFunctions::ToString":               symbols.TierKernelFunction,
		"Items::Item::isSolid":                  symbols.TierSystems,
		"Items::Item::voids":                    symbols.TierSystems,
		"Requirements::RequirementCheck::subj":  symbols.TierSystems,
		"ShapeItems::RectangularCuboid":         symbols.TierDomain,
		"ShapeItems::RectangularCuboid::length": symbols.TierDomain,
	}
	for name, idx := range map[string]*symbols.Index{"fresh": fresh, "decoded": decoded} {
		for fqn, tier := range want {
			sym := idx.Declaring(fqn)
			if sym == nil {
				t.Errorf("%s: %s declares nothing", name, fqn)
				continue
			}
			if got := idx.LibraryTier(sym); got != tier {
				t.Errorf("%s: LibraryTier(%s) = %s, want %s", name, fqn, got, tier)
			}
		}
	}
}
