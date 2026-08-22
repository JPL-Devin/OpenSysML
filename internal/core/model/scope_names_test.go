package model

import (
	"sort"
	"strings"
	"testing"
)

// namesAt enumerates the visible names at the first occurrence of anchor.
func namesAt(t *testing.T, src string, anchor string, opts VisibleNamesOptions) []string {
	t.Helper()
	ws := NewWorkspace()
	ws.Open("t.kerml", []byte(src), 1)
	offset := strings.Index(src, anchor)
	if offset < 0 {
		t.Fatalf("anchor %q not in source", anchor)
	}
	var out []string
	for _, n := range ws.VisibleNamesAt("t.kerml", offset, opts) {
		out = append(out, n.Name)
	}
	return out
}

// has reports whether names holds every wanted name and none of the unwanted.
func has(t *testing.T, names []string, want, unwanted []string) {
	t.Helper()
	set := map[string]bool{}
	for _, n := range names {
		set[n] = true
	}
	for _, w := range want {
		if !set[w] {
			t.Errorf("%q not visible; got %v", w, names)
		}
	}
	for _, u := range unwanted {
		if set[u] {
			t.Errorf("%q visible but should not be; got %v", u, names)
		}
	}
}

func TestVisibleNamesLocalsAndQualifiedSpellings(t *testing.T) {
	src := "package P {\n\tclassifier A;\n\tclassifier B specializes A;\n}\n"
	names := namesAt(t, src, "specializes A", VisibleNamesOptions{})
	has(t, names, []string{"A", "B", "P", "P.A", "P.B"}, nil)
}

func TestVisibleNamesPrivateMemberOnlyInside(t *testing.T) {
	src := "package P {\n\tpackage Q {\n\t\tprivate classifier Hidden;\n\t\tclassifier Seen;\n\t}\n\tclassifier X;\n}\n"
	inside := namesAt(t, src, "classifier Seen", VisibleNamesOptions{})
	has(t, inside, []string{"Hidden", "Seen"}, nil)
	outside := namesAt(t, src, "classifier X", VisibleNamesOptions{})
	has(t, outside, []string{"Q.Seen"}, []string{"Q.Hidden", "Hidden"})
}

func TestVisibleNamesProtectedMemberSeenThroughSpecialization(t *testing.T) {
	src := "package P {\n\tclassifier A {\n\t\tprotected feature p;\n\t\tprivate feature q;\n\t}\n\tclassifier B specializes A {\n\t\tfeature here;\n\t}\n}\n"
	// A specialization sees the protected member it inherits, never the private
	// one, and neither is offered to a reference outside B.
	inside := namesAt(t, src, "feature here", VisibleNamesOptions{})
	has(t, inside, []string{"p"}, []string{"q"})
	outside := namesAt(t, src, "classifier A", VisibleNamesOptions{})
	has(t, outside, []string{"B"}, []string{"B.p", "B.q", "A.p", "A.q"})
}

func TestVisibleNamesImportsAndAliasBindingName(t *testing.T) {
	src := "package Lib {\n\tclassifier A;\n\talias A_alias for A;\n}\n" +
		"package P {\n\tpublic import Lib::*;\n\tclassifier X;\n}\n"
	names := namesAt(t, src, "classifier X", VisibleNamesOptions{})
	has(t, names, []string{"A", "A_alias", "Lib.A_alias"}, nil)
}

func TestVisibleNamesMembershipImportSurfacesOnlyItsName(t *testing.T) {
	src := "package Lib {\n\tclassifier A;\n\tclassifier B;\n}\n" +
		"package P {\n\tpublic import Lib::A;\n\tclassifier X;\n}\n"
	names := namesAt(t, src, "classifier X", VisibleNamesOptions{})
	has(t, names, []string{"A"}, []string{"B"})
}

func TestVisibleNamesMembershipImportKeepsBothNamesOfItsElement(t *testing.T) {
	src := "package Lib {\n\tclassifier <'A_Id'> B;\n\tclassifier C;\n\talias C_alias for C;\n}\n" +
		"package P {\n\tpublic import Lib::A_Id;\n\tpublic import Lib::C_alias;\n\tclassifier X;\n}\n"
	names := namesAt(t, src, "classifier X", VisibleNamesOptions{})
	// An element imported by its short name keeps its regular name too; an
	// alias import surfaces the alias name only.
	has(t, names, []string{"A_Id", "B", "C_alias"}, []string{"C"})
}

func TestVisibleNamesInheritedThroughTyping(t *testing.T) {
	src := "package P {\n\tclassifier A {\n\t\tclassifier a1;\n\t}\n\tclassifier B {\n\t\tfeature b : A;\n\t}\n\tclassifier X;\n}\n"
	names := namesAt(t, src, "classifier X", VisibleNamesOptions{})
	has(t, names, []string{"B.b", "B.b.a1", "P.B.b.a1"}, nil)
}

func TestVisibleNamesLibraryRootsGateImplicitMembers(t *testing.T) {
	src := "package P {\n\tclassifier A;\n\tclassifier X;\n}\n"
	none := namesAt(t, src, "classifier X", VisibleNamesOptions{})
	has(t, none, []string{"A"}, []string{"A.self", "Base", "Base.Anything"})

	base := namesAt(t, src, "classifier X", VisibleNamesOptions{LibraryRoots: []string{"Base"}})
	has(t, base, []string{"A", "A.self"}, []string{"Base", "Base.Anything"})

}

// A `class`'s implicit supertype is Occurrences::Occurrence, so nothing it
// inherits is reachable while only Base is loaded (KerML §8.3.2).
func TestVisibleNamesClassImplicitMembersNeedOccurrences(t *testing.T) {
	src := "package P {\n\tclass A {\n\t\tclass a;\n\t}\n\tclassifier X;\n}\n"
	base := namesAt(t, src, "classifier X", VisibleNamesOptions{LibraryRoots: []string{"Base"}})
	has(t, base, []string{"A", "A.a", "P.A.a"}, []string{"A.self", "A.that", "A.a.self", "A.a.that"})

	occ := namesAt(t, src, "classifier X", VisibleNamesOptions{LibraryRoots: []string{"Base", "Occurrences"}})
	has(t, occ, []string{"A", "A.a", "A.self"}, nil)
}

// A path inherits through a type once: `self` comes from Base::Anything and
// `that` from Base::things, so `self.that` ends there rather than re-expanding
// through Anything again.
func TestVisibleNamesImplicitMembersDoNotRechain(t *testing.T) {
	src := "package P {\n\tclassifier A;\n\tclassifier X;\n}\n"
	names := namesAt(t, src, "classifier X", VisibleNamesOptions{LibraryRoots: []string{"Base"}})
	has(t, names, []string{"A", "A.self", "A.self.that"}, []string{
		"A.self.self", "A.self.that.self", "A.self.that.that", "A.that.self",
	})
}

func TestVisibleNamesCyclicImportTerminatesAndIsSorted(t *testing.T) {
	src := "package P1 {\n\tpublic import P2::*;\n\tclassifier A;\n}\n" +
		"package P2 {\n\tpublic import P1::*;\n\tclassifier B;\n}\n" +
		"package T {\n\tpublic import P1::*;\n\tclassifier X;\n}\n"
	names := namesAt(t, src, "classifier X", VisibleNamesOptions{})
	has(t, names, []string{"A", "B", "P1.A", "P1.B", "P2.A", "P2.B"}, nil)
	if !sort.StringsAreSorted(names) {
		t.Errorf("names not sorted: %v", names)
	}
}

func TestVisibleNamesRedefinitionSeesOnlyInherited(t *testing.T) {
	src := "package P {\n\tclassifier A {\n\t\tfeature a;\n\t}\n\tclassifier B specializes A {\n\t\tfeature b redefines a;\n\t}\n}\n"
	names := namesAt(t, src, "redefines a", VisibleNamesOptions{Redefinition: true})
	has(t, names, []string{"a"}, nil)
}

// The redefinition-anchor cases below follow the reference's `*_Rdef` fixtures:
// a redefinition removes the redefined feature's names from the type owning it.
func TestVisibleNamesRedefinitionMasksTheRedefinedName(t *testing.T) {
	src := "package test {\n\tfeature A {\n\t\tfeature a;\n\t}\n" +
		"\tfeature B subsets A {\n\t\tfeature b redefines a;\n\t}\n\tfeature X;\n}\n"
	names := namesAt(t, src, "feature X", VisibleNamesOptions{})
	has(t, names, []string{"A.a", "B.b", "test.B.b"}, []string{"B.a", "test.B.a"})
}

func TestVisibleNamesRedefinitionAnchorSeesTheChainedTarget(t *testing.T) {
	// MemberNameTests_MultipleInheritance_Rdef: at `c redefines b`, C offers the
	// inherited `b` and no `a` — B's redefinition masked it.
	src := "package test {\n\tfeature A {\n\t\tfeature a;\n\t}\n" +
		"\tfeature B subsets A {\n\t\tfeature b redefines a;\n\t}\n" +
		"\tfeature C subsets B {\n\t\tfeature c redefines b;\n\t}\n}\n"
	names := namesAt(t, src, "redefines b", VisibleNamesOptions{Redefinition: true})
	has(t, names, []string{"b", "C.b", "A.a"}, []string{"a", "C.a"})
}

func TestVisibleNamesRedefinitionAnchorOmitsTheRedefinitionBeingWritten(t *testing.T) {
	// Anchored in C's body, the walk cannot tell which of C's declarations is
	// being written, so each redefinition it declares is treated as the one:
	// absent as a member, masking nothing.
	src := "package test {\n\tfeature A {\n\t\tfeature a;\n\t}\n" +
		"\tfeature C subsets A {\n\t\tfeature c redefines a;\n\t}\n}\n"
	ws := NewWorkspace()
	ws.Open("t.kerml", []byte(src), 1)
	scope := ws.ScopeAt("t.kerml", strings.Index(src, "subsets A"))
	var names []string
	for _, n := range ws.VisibleNames(scope, VisibleNamesOptions{Redefinition: true}) {
		names = append(names, n.Name)
	}
	has(t, names, []string{"a", "C.a"}, []string{"c", "C.c"})
}

func TestVisibleNamesRedefinitionMasksTheShortName(t *testing.T) {
	src := "package test {\n\tfeature A {\n\t\tfeature <a_id> a;\n\t}\n" +
		"\tfeature B subsets A {\n\t\tfeature b redefines a;\n\t}\n\tfeature X;\n}\n"
	names := namesAt(t, src, "feature X", VisibleNamesOptions{})
	has(t, names, []string{"A.a", "A.a_id", "B.b"}, []string{"B.a", "B.a_id"})
}

func TestVisibleNamesRedefinitionMasksOnlyTheFeatureItNames(t *testing.T) {
	// Two inherited features named `a`, one redefined: the other keeps the name.
	src := "package test {\n\tfeature A1 {\n\t\tfeature a;\n\t}\n\tfeature A2 {\n\t\tfeature a;\n\t}\n" +
		"\tfeature B subsets A1, A2 {\n\t\tfeature b redefines A1::a;\n\t}\n\tfeature X;\n}\n"
	names := namesAt(t, src, "feature X", VisibleNamesOptions{})
	has(t, names, []string{"B.a", "B.b", "A1.a", "A2.a"}, nil)
}

func TestVisibleNamesSubsettingDoesNotMask(t *testing.T) {
	src := "package test {\n\tfeature A {\n\t\tfeature a;\n\t}\n" +
		"\tfeature B subsets A {\n\t\tfeature b subsets a;\n\t}\n\tfeature X;\n}\n"
	names := namesAt(t, src, "feature X", VisibleNamesOptions{})
	has(t, names, []string{"A.a", "B.a", "B.b"}, nil)
}

// The cases below pin the composition of the two filters on one enumeration:
// membership visibility (KerML 8.2.3.5) and redefinition masking (7.4.7).
func TestVisibleNamesRedefinitionMasksAProtectedInheritedMember(t *testing.T) {
	src := "package test {\n\tfeature A {\n\t\tprotected feature a;\n\t}\n" +
		"\tfeature B subsets A {\n\t\tfeature %s;\n\t\tfeature keep;\n\t}\n\tfeature X;\n}\n"
	// Inside B a protected inherited member is visible, so its absence is the
	// mask and nothing else: subsetting the same feature keeps it.
	masking := strings.Replace(src, "%s", "b redefines a", 1)
	has(t, namesAt(t, masking, "feature keep", VisibleNamesOptions{}), []string{"b", "keep"}, []string{"a", "B.a"})
	subsetting := strings.Replace(src, "%s", "b subsets a", 1)
	has(t, namesAt(t, subsetting, "feature keep", VisibleNamesOptions{}), []string{"a", "b"}, nil)
	// From outside, `B.a` is gone twice over — masked, and protected members
	// cannot be named through their namespace — and `A.a` for the second reason.
	has(t, namesAt(t, masking, "feature X", VisibleNamesOptions{}), []string{"B.b", "B.keep"}, []string{"B.a", "A.a"})
}

func TestVisibleNamesRedefinedNameWithNoVisiblePathIsGone(t *testing.T) {
	// The redefinition target is private, so no path reaches it: not as an
	// inherited member of B, and not as `A::a` either (8A). The name is absent
	// whether or not the target resolves well enough to mask anything.
	src := "package test {\n\tfeature A {\n\t\tprivate feature a;\n\t}\n" +
		"\tfeature B subsets A {\n\t\tfeature b redefines A::a;\n\t\tfeature keep;\n\t}\n}\n"
	names := namesAt(t, src, "feature keep", VisibleNamesOptions{})
	has(t, names, []string{"b", "keep", "B.b"}, []string{"a", "A.a", "B.a"})
}

func TestVisibleNamesMaskingLeavesAnotherRouteToTheElement(t *testing.T) {
	// Masking removes B's inherited member, not every path to the element: the
	// import route survives inside the namespace declaring it, where 8A admits
	// a private import, while `Q.B.a` is gone.
	src := "package P {\n\tfeature A {\n\t\tfeature a;\n\t}\n}\n" +
		"package Q {\n\tprivate import P::A::*;\n" +
		"\tfeature B subsets P::A {\n\t\tfeature b redefines a;\n\t\tfeature keep;\n\t}\n}\n"
	names := namesAt(t, src, "feature keep", VisibleNamesOptions{})
	has(t, names, []string{"a", "P.A.a", "b", "keep"}, []string{"Q.B.a", "B.a"})
}

func TestElementOnPathResolvesSegmentsAndAliases(t *testing.T) {
	src := "package P {\n\tclassifier A {\n\t\tclassifier a1;\n\t}\n\talias AA for A;\n}\n"
	ws := NewWorkspace()
	ws.Open("t.kerml", []byte(src), 1)
	scope := ws.Document("t.kerml").Scope
	cases := []struct{ path, fqn string }{
		{"P", "P"},
		{"P.A", "P::A"},
		{"P.A.a1", "P::A::a1"},
		{"P.AA", "P::A"},
		{"P.Absent", ""},
		{"Absent", ""},
	}
	for _, c := range cases {
		sym, ok := ws.ElementOnPath(scope, strings.Split(c.path, "."))
		if (c.fqn == "") == ok {
			t.Errorf("ElementOnPath(%q) resolved = %v, want %v", c.path, ok, c.fqn != "")
			continue
		}
		if ok && ws.FQNOf(sym) != c.fqn {
			t.Errorf("ElementOnPath(%q) = %q, want %q", c.path, ws.FQNOf(sym), c.fqn)
		}
	}
	if _, ok := ws.ElementOnPath(nil, []string{"P"}); ok {
		t.Error("a nil scope resolves nothing")
	}
}

func TestVisibleNamesNilScope(t *testing.T) {
	ws := NewWorkspace()
	if got := ws.VisibleNames(nil, VisibleNamesOptions{}); got != nil {
		t.Errorf("VisibleNames(nil) = %v, want nil", got)
	}
	if got := ws.VisibleNamesAt("absent.kerml", 0, VisibleNamesOptions{}); got != nil {
		t.Errorf("VisibleNamesAt on an absent document = %v, want nil", got)
	}
}

func TestVisibleNamesDeterministicAcrossRuns(t *testing.T) {
	src := "package P {\n\tclassifier A {\n\t\tfeature a;\n\t}\n\tclassifier B specializes A;\n\tclassifier X;\n}\n"
	first := namesAt(t, src, "classifier X", VisibleNamesOptions{LibraryRoots: []string{"Base"}})
	for i := 0; i < 3; i++ {
		again := namesAt(t, src, "classifier X", VisibleNamesOptions{LibraryRoots: []string{"Base"}})
		if strings.Join(first, ",") != strings.Join(again, ",") {
			t.Fatalf("run %d differs:\n%v\n%v", i, first, again)
		}
	}
}
