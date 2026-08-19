package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// `@` introduces a metadata usage (SysML v2 grammar: MetadataUsageKeyword), so
// `@Security;` is a member of its own wherever a member may be written.
func TestMetadataUsageMember(t *testing.T) {
	src := `package P {
	metadata def Security;
	@Security;
	part x {
		@Security;
	}
	part y;
	@Security;
}`
	p := New(source.New("test.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("expected no diagnostics, got %v", p.Diagnostics)
	}

	var annotations, parts int
	var walk func(nodes []ast.Node)
	walk = func(nodes []ast.Node) {
		for _, node := range nodes {
			if membership, ok := node.(*ast.Membership); ok {
				node = membership.Member
			}
			switch n := node.(type) {
			case *ast.Package:
				walk(n.Members)
			case *ast.PrefixMetadata:
				annotations++
				if got := ast.SimpleName(n.Type); got != "Security" {
					t.Errorf("expected the metadata type Security, got %q", got)
				}
			case *ast.Usage:
				if n.Kind == ast.UsagePart {
					parts++
					if len(n.Prefixes) != 0 {
						t.Errorf("a metadata member is not a prefix of the member after it: %v", n.Prefixes)
					}
					walk(n.Members)
				}
			}
		}
	}
	walk(root.Members)

	if annotations != 3 || parts != 2 {
		t.Errorf("expected 3 metadata members and 2 parts, got %d and %d", annotations, parts)
	}
}
