package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

func TestMetadataBodyInheritedAndNestedDeclarations(t *testing.T) {
	src := `metadata def Base {
		attribute inherited;
	}
	metadata def A :> Base {
		attribute own;
		attribute nested {
			attribute value;
		}
	}
	item p {
		@A {
			inherited = 1;
			own = 2;
			nested {
				value = 3;
				missing = 4;
			}
		}
	}
	`
	m, root := buildModel(t, src)
	p := sym(t, root, "p")
	usage, ok := p.Decl.(*ast.Usage)
	if !ok {
		t.Fatal("item p not found")
	}
	var prefix *ast.PrefixMetadata
	for _, member := range usage.Members {
		if membership, ok := member.(*ast.Membership); ok {
			prefix, _ = membership.Member.(*ast.PrefixMetadata)
		}
	}
	if prefix == nil {
		t.Fatal("metadata annotation not found")
	}
	violations := m.MetadataBodyViolations(p.Scope, prefix)
	if len(violations) != 1 {
		t.Fatalf("metadata body violations = %d, want 1 (%v)", len(violations), violations)
	}
	missing, ok := violations[0].(*ast.Usage)
	if !ok || missing.Ident.Name != "missing" {
		t.Fatalf("metadata body violation = %T %v, want missing declaration", violations[0], violations[0])
	}
}

func TestMetadataBodyInheritedValueResolvesInBodyScope(t *testing.T) {
	src := `metadata def Base {
		attribute y;
	}
	metadata def A :> Base {
		attribute x;
	}
	item p {
		@A {
			x = y;
		}
	}
	`
	m, root := buildModel(t, src)
	p := sym(t, root, "p")
	usage, ok := p.Decl.(*ast.Usage)
	if !ok {
		t.Fatal("item p not found")
	}
	var prefix *ast.PrefixMetadata
	for _, member := range usage.Members {
		if membership, ok := member.(*ast.Membership); ok {
			prefix, _ = membership.Member.(*ast.PrefixMetadata)
		}
	}
	if prefix == nil {
		t.Fatal("metadata annotation not found")
	}
	if len(m.resolver.Diagnostics) != 0 {
		t.Fatalf("inherited metadata body resolution diagnostics: %v", m.resolver.Diagnostics)
	}
	value, ok := prefix.Body[0].(*ast.Membership)
	if !ok {
		t.Fatal("body declaration wrapper not found")
	}
	bound, ok := value.Member.(*ast.Usage)
	if !ok {
		t.Fatal("body declaration not a usage")
	}
	ref, ok := bound.Value.(*ast.FeatureReference)
	if !ok || ref.Name == nil {
		t.Fatal("body value reference not found")
	}
	body := p.Scope.ChildFor(prefix)
	if body == nil {
		t.Fatal("annotation body scope not found")
	}
	resolved, ok := m.resolver.ResolveQualified(body, ref.Name)
	if !ok || resolved.Name != "y" {
		t.Fatalf("body value resolved to %v, %v; want inherited feature y", resolved, ok)
	}
}
