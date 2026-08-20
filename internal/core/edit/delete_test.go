package edit

import (
	"strings"
	"testing"
)

func TestDeleteReferencedRefusesAndCascadeRemovesReferrers(t *testing.T) {
	src := "package P {\n    part def Base;\n    part x : Base;\n}\n"
	m := loadContent(t, "delete.sysml", src)
	e := addFailure(t, m, Delete("P::Base", false), FailureDeleteReferenced)
	if !strings.Contains(strings.Join(e.Referring, ","), "P") {
		t.Fatalf("referrers = %v, want P", e.Referring)
	}
	res, err := Apply(m, []Operation{Delete("P::Base", true)})
	if err != nil {
		t.Fatalf("cascade delete: %v", err)
	}
	if strings.Contains(string(res.Content), "Base") || strings.Contains(string(res.Content), "part x") {
		t.Fatalf("cascade left target or referrer:\n%s", res.Content)
	}
}

func TestDeleteOnlyMemberRootAndNeighborTrivia(t *testing.T) {
	tests := []struct {
		name, src, target, want string
	}{
		{
			name:   "only member",
			src:    "package P {\n    part def Only;\n}\n",
			target: "P::Only",
			want:   "package P {\n}\n",
		},
		{
			name:   "root declaration",
			src:    "// keep\npart def Keep;\n\n// remove\npart def Gone;\n",
			target: "Gone",
			want:   "// keep\npart def Keep;\n",
		},
		{
			name:   "neighbor comment and blank line",
			src:    "package P {\n    // keep\n    part def Keep;\n\n    // remove\n    part def Gone;\n\n    // neighbor\n    part def Next;\n}\n",
			target: "P::Gone",
			want:   "package P {\n    // keep\n    part def Keep;\n\n    // neighbor\n    part def Next;\n}\n",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := loadContent(t, "delete.sysml", tc.src)
			res, err := Apply(m, []Operation{Delete(tc.target, false)})
			if err != nil {
				t.Fatalf("Apply: %v", err)
			}
			if got := string(res.Content); got != tc.want {
				t.Fatalf("content = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestMixedBatchAppliesInRequestOrder(t *testing.T) {
	src := "package P {\n    attribute a = 1;\n    part def Gone;\n}\n"
	m := loadContent(t, "mixed.sysml", src)
	res, err := Apply(m, []Operation{
		{
			Kind:       OpAddMember,
			Owner:      "P",
			MemberKind: "attribute",
			MemberName: "b",
			Value:      "2",
		},
		SetValue("P::a", "3"),
		Delete("P::Gone", false),
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	got := string(res.Content)
	if !strings.Contains(got, "attribute a = 3;") ||
		!strings.Contains(got, "attribute b = 2;") ||
		strings.Contains(got, "Gone") {
		t.Fatalf("mixed batch result:\n%s", got)
	}
}
