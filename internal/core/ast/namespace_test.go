package ast

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestQualifiedNameParts(t *testing.T) {
	qn := &QualifiedName{
		Global: false,
		Parts:  []NameSegment{{Text: "A"}, {Text: "B"}, {Text: "C"}},
	}
	if len(qn.Parts) != 3 || qn.Parts[2].Text != "C" {
		t.Fatalf("parts = %+v", qn.Parts)
	}
}

func TestPackageIsNamespaceMember(t *testing.T) {
	var _ Node = &Package{}
	var _ Node = &Namespace{}
	var _ Node = &Import{}
	var _ Node = &Alias{}
	var _ Node = &Dependency{}
	var _ Node = &Comment{}
	var _ Node = &Documentation{}
	var _ Node = &TextualRepresentation{}
	var _ Node = &RootNamespace{}
	var _ Node = &Membership{}
	p := &Package{Ident: Identification{Name: "P"}}
	if p.Ident.Name != "P" {
		t.Fatalf("name = %q", p.Ident.Name)
	}
}

func TestMembershipVisibility(t *testing.T) {
	m := &Membership{Visibility: VisibilityPrivate}
	if m.Visibility != VisibilityPrivate {
		t.Fatalf("vis = %v", m.Visibility)
	}
	_ = source.Span{}
}
