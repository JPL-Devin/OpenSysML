package parser

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// A subject, actor, stakeholder, objective, state subaction or rendering written
// in a body whose grammar does not offer it is rejected with a diagnostic and an
// ErrorNode, and is not read as a generic declaration (SysML.xtext
// RequirementBodyItem, CaseBodyItem, StateBodyItem, ViewBodyItem; the pilot's
// *MembershipOwningType constraints).
func TestMemberOutsideOwningBodyIsRejected(t *testing.T) {
	tests := []struct {
		name          string
		src           string
		keyword       string
		forbiddenNode string
	}{
		{"actor in part def", "part def P { actor a : Person; }", "actor", `kind="actor"`},
		{"actor in part usage", "part p { actor a : Person; }", "actor", `kind="actor"`},
		{"actor in action def", "action def A { actor a : Person; }", "actor", `kind="actor"`},
		{"actor in perform body", "action def A { perform action x { actor a : Person; } }", "actor", `kind="actor"`},
		{"actor in package", "package P { actor a : Person; }", "actor", `kind="actor"`},
		{"actor with visibility", "part def P { private actor a : Person; }", "actor", `kind="actor"`},
		{"actor with prefix metadata", "part def P { actor #M a : Person; }", "actor", `kind="actor"`},
		{"actor in subject body", "requirement def R { subject v : V { actor a : Person; } }", "actor", `kind="actor"`},
		{"actor in required constraint body", "requirement def R { require constraint k : K { actor a : Person; } }", "actor", `kind="actor"`},
		{"actor in rendering body", "view def V { render asTable { actor a : Person; } }", "actor", `kind="actor"`},
		{"subject in part def", "part def P { subject s : S; }", "subject", "SubjectMember"},
		{"subject in constraint def", "constraint def C { subject s : S; }", "subject", "SubjectMember"},
		{"subject in view def", "view def V { subject s : S; }", "subject", "SubjectMember"},
		{"subject in subject body", "requirement def R { subject v : V { subject w : W; } }", "subject", `name="w"`},
		{"subject in required constraint body", "requirement def R { require constraint k : K { subject w : W; } }", "subject", "SubjectMember"},
		{"subject in nested constraint body", "requirement def R { require constraint k { constraint c { subject w : W; } } }", "subject", "SubjectMember"},
		{"subject in expr body", "attribute def A { expr e { subject s : S; } }", "subject", "SubjectMember"},
		{"anonymous subject", "part def P { subject : S; }", "subject", "SubjectMember"},
		{"stakeholder in part def", "part def P { stakeholder s : Person; }", "stakeholder", `kind="stakeholder"`},
		{"stakeholder in case def", "case def C { stakeholder s : Person; }", "stakeholder", `kind="stakeholder"`},
		{"stakeholder in use case", "use case u { stakeholder s : Person; }", "stakeholder", `kind="stakeholder"`},
		{"objective in part def", "part def P { objective o { require constraint { x > 0 } } }", "objective", `kind="objective"`},
		{"objective in requirement def", "requirement def R { objective o; }", "objective", `kind="objective"`},
		{"anonymous objective in action", "action def A { objective { require constraint { x > 0 } } }", "objective", `kind="objective"`},
		{"entry action in part def", "part def P { entry action init; }", "entry", "EntryMember"},
		{"entry reference in action def", "action def A { entry init; }", "entry", "EntryMember"},
		{"bare entry in part def", "part def P { entry; }", "entry", "EntryMember"},
		{"do action in part def", "part def P { do action run; }", "do", "DoMember"},
		{"exit action in requirement", "requirement def R { exit action stop; }", "exit", "ExitMember"},
		{"entry in case def", "case def C { entry action init; }", "entry", "EntryMember"},
		{"render in part def", "part def P { render asTable; }", "render", `kind="render"`},
		{"render rendering in part def", "part def P { render rendering r : R; }", "render", `kind="render"`},
		{"render in viewpoint", "viewpoint def V { render asTable; }", "render", `kind="render"`},
		{"render in requirement", "requirement def R { render asTable; }", "render", `kind="render"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("parser panicked: %v", r)
				}
			}()
			p := New(source.New("t.sysml", []byte(tt.src)))
			root := p.ParseFile()
			if root == nil {
				t.Fatal("ParseFile returned nil")
			}
			want := "'" + tt.keyword + "' declares "
			var found bool
			for _, d := range p.Diagnostics {
				if strings.Contains(d.Message, want) && strings.Contains(d.Message, "only allowed in a") {
					found = true
				}
			}
			if !found {
				t.Fatalf("no owning-body diagnostic for %q: %v", tt.keyword, p.Diagnostics)
			}
			dump := ast.Dump(root)
			if !strings.Contains(dump, "ErrorNode") {
				t.Fatalf("expected an ErrorNode:\n%s", dump)
			}
			if strings.Contains(dump, tt.forbiddenNode) {
				t.Fatalf("misplaced member kept as %s:\n%s", tt.forbiddenNode, dump)
			}
		})
	}
}

// The same members parse without a diagnostic in every body kind that offers
// them, in their definition and usage forms, and the state shorthands and
// transitions inside a state body are untouched.
func TestMemberInsideOwningBodyStaysClean(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"requirement def", "requirement def R { subject v : V; actor a : Person; stakeholder s : Person; }"},
		{"requirement usage", "requirement r { subject v : V; actor a : Person; stakeholder s : Person; }"},
		{"nested requirement", "requirement def R { requirement sub { actor a : Person; stakeholder s : Person; } }"},
		{"concern def", "concern def C { subject v : V; actor a : Person; stakeholder s : Person; }"},
		{"viewpoint usage", "viewpoint vp { subject v : V; actor a : Person; stakeholder s : Person; }"},
		{"satisfy body", "part p { satisfy r by p { subject v : V; actor a : Person; } }"},
		{"frame body", "requirement def R { frame c { stakeholder s : Person; actor a : Person; } }"},
		{"objective body", "case def C { objective o { subject v : V; actor a : Person; stakeholder s : Person; } }"},
		{"case def", "case def C { subject v : V; actor a : Person; objective o; }"},
		{"case usage", "case c { subject v : V; actor a : Person; objective { require constraint { x > 0 } } }"},
		{"analysis case def", "analysis def A { subject v : V; actor a : Person; objective o; }"},
		{"verification case usage", "verification v { subject v : V; actor a : Person; objective o; }"},
		{"use case def", "use case def U { subject v : V; actor a : Person; objective o; }"},
		{"include use case", "use case def U { include use case u { subject v : V; actor a : Person; } }"},
		{"include reference body", "use case def U { include u { subject v : V; actor a : Person; objective o; } }"},
		{"perform reference body", "part def P { perform a { first x; action x; } }"},
		{"view def", "view def V { render asTable; }"},
		{"view usage", "view v { render asTable; }"},
		{"view rendering declaration", "view v { render rendering r : R; }"},
		{"state def subactions", "state def S { entry action init; do action run; exit action stop; }"},
		{"state usage subactions", "state s { entry init; do run; exit stop; }"},
		{"state shorthands", "state def S { entry; do; exit; state a; state b; }"},
		{"entry then", "state def S { entry; then a; state a; }"},
		{"entry transition", "state def S { entry; then a; state a; transition a_to_b first a then b; state b; }"},
		{"inline state actions", "state def S { entry send x to y; do assign a := 1; exit send z to y; }"},
		{"braced state actions", "state def S { entry action { x := 1; } do { y := 2; } exit action { z := 3; } }"},
		{"nested state", "state def S { state inner { entry action init; state deeper { entry; exit action stop; } } }"},
		{"exhibit state body", "part def P { exhibit state s { entry action init; do action run; } }"},
		{"exhibit reference body", "part def P { exhibit s { entry; do action run; } }"},
		{"parallel state", "state def S parallel { state a { entry action init; } state b { exit; } }"},
		{"keywords as names", "package P { attribute actor; attribute subject; part render; }"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := New(source.New("t.sysml", []byte(c.src)))
			if p.ParseFile() == nil {
				t.Fatal("nil root")
			}
			for _, d := range p.Diagnostics {
				t.Errorf("unexpected diagnostic: %s", d.Message)
			}
		})
	}
}
