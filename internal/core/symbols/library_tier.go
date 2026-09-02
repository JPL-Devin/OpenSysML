package symbols

// LibraryTier is the part of the bundled library a document belongs to. The
// Kernel tiers are the metamodel frame every element specializes; the Systems,
// Domain and OpenSysML tiers declare features with semantics of their own.
type LibraryTier uint8

const (
	// TierNone is the workspace's own content.
	TierNone LibraryTier = iota
	// TierLibrary is library content whose place in the bundle is not stated.
	TierLibrary
	// TierKernelSemantic is the Kernel Semantic Library (Base, Occurrences, Objects, …).
	TierKernelSemantic
	// TierKernelDataType is the Kernel Data Type Library (ScalarValues, Collections, …).
	TierKernelDataType
	// TierKernelFunction is the Kernel Function Library.
	TierKernelFunction
	// TierSystems is the Systems Library (Items, Parts, Requirements, …).
	TierSystems
	// TierDomain is the Domain Libraries (Geometry, Quantities and Units, Metadata, …).
	TierDomain
	// TierOpenSysML is this implementation's own library packages.
	TierOpenSysML
	numLibraryTiers
)

// tierNames spell the tiers for diagnostics and tests.
var tierNames = [numLibraryTiers]string{
	TierNone:           "model",
	TierLibrary:        "library",
	TierKernelSemantic: "Kernel Semantic Library",
	TierKernelDataType: "Kernel Data Type Library",
	TierKernelFunction: "Kernel Function Library",
	TierSystems:        "Systems Library",
	TierDomain:         "Domain Libraries",
	TierOpenSysML:      "OpenSysML Libraries",
}

func (t LibraryTier) String() string {
	if t < numLibraryTiers {
		return tierNames[t]
	}
	return "unknown library tier"
}

// Library reports whether the tier is bundled library content of any kind.
func (t LibraryTier) Library() bool {
	return t != TierNone
}

// Frame reports whether the tier frames every element rather than describing
// the objects a model asks for: the Kernel libraries, and library content of no
// stated tier, which is held to the same standard.
func (t LibraryTier) Frame() bool {
	switch t {
	case TierSystems, TierDomain, TierOpenSysML:
		return false
	default:
		return t.Library()
	}
}
