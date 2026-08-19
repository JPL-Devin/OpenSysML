package parser

import (
	"strings"
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

// An unterminated metadata usage is reported rather than read as a prefix of the
// declaration after it; `#Type` is the prefix spelling.
func TestMetadataUsageRequiresTerminator(t *testing.T) {
	p := New(source.New("test.sysml", []byte("package P { metadata def M; @M part def Car; }")))
	root := p.ParseFile()
	if len(p.Diagnostics) != 1 {
		t.Fatalf("expected one diagnostic, got %v", p.Diagnostics)
	}
	if !strings.Contains(p.Diagnostics[0].Message, "after a metadata usage") {
		t.Errorf("unexpected message %q", p.Diagnostics[0].Message)
	}

	// Recovery leaves the declaration itself parsed.
	var parts int
	for _, member := range root.Members {
		if membership, ok := member.(*ast.Membership); ok {
			member = membership.Member
		}
		pkg, ok := member.(*ast.Package)
		if !ok {
			continue
		}
		for _, m := range pkg.Members {
			if membership, ok := m.(*ast.Membership); ok {
				m = membership.Member
			}
			if def, ok := m.(*ast.Definition); ok && def.Kind == ast.DefPart {
				parts++
			}
		}
	}
	if parts != 1 {
		t.Errorf("expected the part def to still be parsed, got %d", parts)
	}
}
