package runtime

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Kernel-library members frame every object (`Occurrence::self`, `portions`) and stay
// out of its shape; Systems, Domain and OpenSysML members describe it and are kept.

// frameRoots names the Kernel features whose restatements stay in the frame: the
// object's identity, its portions over time, and its part in transfers.
var frameRoots = map[string]bool{
	"Base::Anything::self":                               true,
	"Occurrences::Occurrence::timeSlices":                true,
	"Occurrences::Occurrence::incomingTransfersToSelf":   true,
	"Occurrences::Occurrence::outgoingTransfersFromSelf": true,
}

// libraryTier reports the tier of the library that declares sym, TierNone for a
// declaration of the model under evaluation.
func (ctx *Context) libraryTier(sym *symbols.Symbol) symbols.LibraryTier {
	if ctx == nil || ctx.resolver == nil {
		return symbols.TierNone
	}
	idx := ctx.resolver.Index()
	if idx == nil {
		return symbols.TierNone
	}
	return idx.LibraryTier(sym)
}

// frameFeature reports whether a library-declared member frames the object
// rather than describing it, so that the shape leaves it out.
func (ctx *Context) frameFeature(sym *symbols.Symbol) bool {
	tier := ctx.libraryTier(sym)
	switch {
	case !tier.Library():
		return false
	case tier.Frame():
		return true
	case sym.OwnerScope != nil && isValueTypeSymbol(sym.OwnerScope.Owner()):
		return true
	case isParameter(sym):
		return true
	default:
		return ctx.restatesFrameRoot(sym)
	}
}

// isParameter reports whether sym is a directed or result parameter of a behavior.
func isParameter(sym *symbols.Symbol) bool {
	usage, ok := sym.Decl.(*ast.Usage)
	return ok && (usage.Direction != ast.DirNone || usage.IsResult)
}

// restatesFrameRoot reports whether sym redefines or subsets a frame root,
// directly or through the features those name.
func (ctx *Context) restatesFrameRoot(sym *symbols.Symbol) bool {
	seen := map[*symbols.Symbol]bool{sym: true}
	for queue := []*symbols.Symbol{sym}; len(queue) > 0; {
		cur := queue[0]
		queue = queue[1:]
		if frameRoots[symbols.FQNOf(cur)] {
			return true
		}
		for _, kind := range []ast.RelationshipKind{ast.RelRedefines, ast.RelSubsets} {
			for _, qn := range relationshipTargets(cur, kind) {
				named, ok := ctx.resolver.ResolveQualified(cur.OwnerScope, qn)
				if ok && named != nil && !seen[named] {
					seen[named] = true
					queue = append(queue, named)
				}
			}
		}
	}
	return false
}
