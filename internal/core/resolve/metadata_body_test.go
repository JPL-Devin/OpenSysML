package resolve

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

func TestMetadataBodyMissingFeatureDoesNotBecomeUnresolved(t *testing.T) {
	r := resolveDoc(t, "metadata.sysml", `metadata def A;
	item p {
		@A {
			missing = 1;
		}
	}
`)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("metadata body declaration produced diagnostics: %v", r.Diagnostics)
	}
}

func TestMetadataBodyDeclarationImplicitlyRedefinesMetadataFeature(t *testing.T) {
	r := resolveDocWithModel(t, "metadata.sysml", `metadata def A {
		attribute x;
	}
	item p {
		attribute outer;
		@A {
			x = outer;
		}
	}
	`)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("metadata body resolution diagnostics: %v", r.Diagnostics)
	}
	p, ok := r.idx.DocumentRoot("metadata.sysml").LookupLocal("p")
	if !ok {
		t.Fatal("item p not indexed")
	}
	usage, ok := p.Decl.(*ast.Usage)
	if !ok {
		t.Fatal("item p declaration not found")
	}
	var annotation *ast.PrefixMetadata
	for _, member := range usage.Members {
		if membership, ok := member.(*ast.Membership); ok {
			annotation, _ = membership.Member.(*ast.PrefixMetadata)
		}
	}
	if annotation == nil {
		t.Fatal("annotation prefix not found")
	}
	body := p.Scope.ChildFor(annotation)
	if body == nil {
		t.Fatal("annotation body scope not found")
	}
	x, ok := body.LookupLocal("x")
	if !ok {
		t.Fatal("body declaration x not indexed")
	}
	targets := r.redefined[x]
	if len(targets) != 1 || targets[0].Name != "x" {
		t.Fatalf("body declaration redefinition = %v, want metadata feature x", targets)
	}
	value, ok := annotation.Body[0].(*ast.Membership)
	if !ok {
		t.Fatal("body declaration wrapper not found")
	}
	valueUsage, ok := value.Member.(*ast.Usage)
	if !ok {
		t.Fatal("body declaration not a usage")
	}
	ref, ok := valueUsage.Value.(*ast.FeatureReference)
	if !ok || ref.Name == nil {
		t.Fatal("body value reference not found")
	}
	resolved, ok := r.ResolveQualified(body, ref.Name)
	if !ok || resolved.Name != "outer" {
		t.Fatalf("body value resolved to %v, %v; want enclosing feature outer", resolved, ok)
	}
}
