package resolve_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
)

// The document walk reads `featured by startShot` in the feature's own scope,
// where its type contributes Occurrence::startShot, ahead of the enclosing
// portion that redefines startShot; a collected reference must do the same.
const headRelationshipModel = `package P {
	class Occurrence {
		feature startShot : Occurrence;
		feature snapshots : Occurrence;
	}
	class CC1 :> Occurrence {
		portion :>> startShot {
			feature snaps :>> snapshots featured by startShot;
		}
	}
}`

func TestHeadRelationshipReferenceResolvesWhereTheDocumentDid(t *testing.T) {
	walk, root, rootScope := resolvedDocNamed(t, "app.kerml", headRelationshipModel)
	if len(walk.Diagnostics) != 0 {
		t.Fatalf("the document walk must resolve every name: %v", walk.Diagnostics)
	}

	var featuredBy *resolve.Reference
	for _, ref := range resolve.References(root, rootScope) {
		ref := ref
		if nameText(ref.QN) == "startShot" && !ref.Redefines && ref.Head != nil {
			featuredBy = &ref
		}
	}
	if featuredBy == nil {
		t.Fatal("`featured by startShot` was not collected as a head relationship")
	}

	walked, ok := walk.PartSymbol(featuredBy.QN, 0)
	if !ok {
		t.Fatal("the document walk did not bind `featured by startShot`")
	}
	if !isFeature(walked.Decl) || isPortion(walked.Decl) {
		t.Fatalf("the document walk bound `featured by startShot` to %T, want the inherited feature", walked.Decl)
	}

	sym, ok := walk.ResolveReference(*featuredBy)
	if !ok {
		t.Fatal("`featured by startShot` does not resolve on its own")
	}
	if sym != walked {
		t.Errorf("`featured by startShot` resolves to a different declaration on its own than in the document walk")
	}

	// Spelled through the class, the same occurrence names the redefining
	// portion, which the header does not hold: the enclosing scope decides.
	qualified := &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "CC1"}, {Text: "startShot"}}}
	portion, ok := walk.ProbeReference(featuredBy.Spelled(qualified))
	if !ok {
		t.Fatal("`featured by CC1::startShot` does not resolve")
	}
	if !isPortion(portion.Decl) {
		t.Errorf("`featured by CC1::startShot` resolves to %T, want the portion redefining startShot", portion.Decl)
	}
}

func isFeature(n ast.Node) bool {
	_, ok := n.(*ast.Usage)
	return ok
}

func isPortion(n ast.Node) bool {
	u, ok := n.(*ast.Usage)
	return ok && u.IsPortion
}
