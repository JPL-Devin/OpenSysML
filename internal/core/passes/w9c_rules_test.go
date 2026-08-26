package passes

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// w9cLibraryDiags analyzes src as SysML against the standard library, which the
// inherited-name rule needs to see the Action/Part diamond.
func w9cLibraryDiags(t *testing.T, src string, warm bool) []Diagnostic {
	t.Helper()
	// The loader below is what populates the library here, so this starts empty:
	// loading it twice would re-add every library document.
	idx := symbols.NewIndex()
	libSrc := libs.DefaultSource()
	var cache *libs.Cache
	if warm {
		// Populate a cache of this test's own, so the library really is restored
		// from records rather than parsed.
		t.Setenv("XDG_CACHE_HOME", t.TempDir())
		c, err := libs.NewCache()
		if err != nil {
			t.Fatalf("cache: %v", err)
		}
		cache = c
		warmIdx := symbols.NewIndex()
		if err := libs.NewLoader(libSrc, cache).LoadAll(warmIdx); err != nil {
			t.Fatalf("warm library: %v", err)
		}
	}
	ld := libs.NewLoader(libSrc, cache)
	if err := ld.LoadAll(idx); err != nil {
		t.Fatalf("load the library: %v", err)
	}
	if warm && ld.Hits() == 0 {
		// Facts derived again would make the warm run a second cold one, which
		// proves nothing about what a cache hit restores.
		t.Fatal("warm run derived the library facts instead of restoring them")
	}
	// Either path parses the library, so the diamond is read from declarations.
	for _, sym := range idx.LookupQualified("Actions::Action") {
		if sym.Decl == nil {
			t.Fatalf("warm=%v: Actions::Action carries no declaration", warm)
		}
	}
	root := parser.New(source.New("<t>.sysml", []byte(src))).ParseFile()
	idx.AddDocument("<t>.sysml", root)
	idx.ExpandWildcardImports()
	return Analyze("<t>.sysml", root, nil, idx)
}

func w9cMessages(diags []Diagnostic, prefix string) []string {
	var out []string
	for _, d := range diags {
		if strings.HasPrefix(d.Message, prefix) {
			out = append(out, d.Message)
		}
	}
	return out
}

// An action typed by a part definition inherits self/start/done from both
// Action and Part (ActionUsage_invalid.sysml.xt:40), as a warning.
func TestW9CActionPartDiamondWarns(t *testing.T) {
	src := `package Test {
	part def ABlock;
	action def AnAction {
		action a : ABlock;
	}
}`
	for _, warm := range []bool{false, true} {
		got := w9cMessages(w9cLibraryDiags(t, src, warm), msgW9CDuplicateInherited)
		want := []string{
			msgW9CDuplicateInherited + " 'done' from Action, Part",
			msgW9CDuplicateInherited + " 'self' from Action, Part",
			msgW9CDuplicateInherited + " 'start' from Action, Part",
		}
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("warm=%v: got %v, want %v", warm, got, want)
		}
	}
}

// A usage typed by a definition of its own kind inherits each name once.
func TestW9CConformingTypingStaysSilent(t *testing.T) {
	src := `package Test {
	action def AnAction;
	part def P {
		action a : AnAction;
	}
}`
	for _, warm := range []bool{false, true} {
		if got := w9cMessages(w9cLibraryDiags(t, src, warm), msgW9CDuplicateInherited); len(got) != 0 {
			t.Errorf("warm=%v: got %v, want none", warm, got)
		}
	}
}

func TestW9CBinaryInterfaceEndDiamondWarns(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{
			name: "reference subsetting",
			src: `package Test {
	part def C {
		part p;
		interface i {
			end part ::> p;
			end port q1;
		}
	}
}`,
		},
		{
			name: "declared end",
			src: `package Test {
	part def C {
		interface i {
			end part p1;
			end port q1;
		}
	}
}`,
		},
		{
			name: "interface definition",
			src: `package Test {
	interface def I {
		end part p1;
		end port q1;
	}
}`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, warm := range []bool{false, true} {
				got := w9cMessages(w9cLibraryDiags(t, tc.src, warm), msgW9CDuplicateInherited)
				want := []string{msgW9CDuplicateInherited + " 'self' from Part, Port"}
				if strings.Join(got, "\n") != strings.Join(want, "\n") {
					t.Errorf("warm=%v: got %v, want %v", warm, got, want)
				}
			}
		})
	}
}

func TestW9CNonBinaryConnectorEndsStaySilent(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "three-ended interface",
			body: `interface i {
				end part p1;
				end port q1;
				end port q2;
			}`,
		},
		{
			name: "binary connection",
			body: `connection c {
				end part p1;
				end part p2;
			}`,
		},
		{
			name: "binary connection definition",
			body: `connection def C {
				end part p1;
				end part p2;
			}`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			src := `package Test {
	part def C {
		` + tc.body + `
	}
}`
			for _, warm := range []bool{false, true} {
				if got := w9cMessages(w9cLibraryDiags(t, src, warm), msgW9CDuplicateInherited); len(got) != 0 {
					t.Errorf("warm=%v: got %v, want none", warm, got)
				}
			}
		})
	}
}

// Short names are names for distinguishability: a repeated short name, and a
// short name repeating another member's name, are both reported
// (ShortNameTests_Distinguishibility1/2.kerml.xt:20,22).
func TestW9CShortNameDistinguishability(t *testing.T) {
	tests := []struct {
		name  string
		src   string
		lines []int
	}{
		{
			name: "repeated short name",
			src: `package Test {
	classifier <one> two;
	classifier <one> three;
}`,
			lines: []int{2, 3},
		},
		{
			name: "short name repeats a name",
			src: `package Test {
	classifier <one> two;
	classifier <two> three;
}`,
			lines: []int{2, 3},
		},
		{
			name: "distinct names and short names",
			src: `package Test {
	classifier <one> two;
	classifier <three> four;
}`,
			lines: nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w9cWantLines(t, tc.src, "name-conflict", tc.lines...)
		})
	}
}

// A member reusing a name its one library base supplies is indistinguishable
// from it: `state start;` against StatePerformances::StateAction::start
// (examples/orthogonal-regions-demo.sysml:12).
func TestW9COwnedNameAgainstOneLibraryBaseWarns(t *testing.T) {
	src := `package Test {
	state S {
		state start;
		state idle;
	}
}`
	for _, warm := range []bool{false, true} {
		got := w9cMessages(w9cLibraryDiags(t, src, warm), msgW9CDuplicateInherited)
		want := []string{msgW9CDuplicateInherited + " 'start' from StateAction"}
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("warm=%v: got %v, want %v", warm, got, want)
		}
	}
}

// A library base reached through user supertypes supplies its names just the
// same; the pinned validate-sysml reports both warnings at the same columns.
func TestW9CInheritedNameThroughUserSupertypesWarns(t *testing.T) {
	src := `package Test {
	private import Occurrences::Occurrence;
	private import Actions::Action;
	part def Base :> Occurrence;
	part def Mid :> Base;
	part def Leaf :> Mid { part portions; }
	part def Ok :> Mid { part :>> portions; }
	action def MyAct :> Action;
	action def Sub :> MyAct { action done; }
}`
	for _, warm := range []bool{false, true} {
		got := w9cMessages(w9cLibraryDiags(t, src, warm), msgW9CDuplicateInherited)
		want := []string{
			msgW9CDuplicateInherited + " 'portions' from Occurrence",
			msgW9CDuplicateInherited + " 'done' from Action",
		}
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("warm=%v: got %v, want %v", warm, got, want)
		}
	}
}

// Redefining or subsetting the inherited feature makes the reused name its own
// name, so nothing is indistinguishable; an unreused name is silent too.
func TestW9COwnedNameSpecializingItsLibraryBaseStaysSilent(t *testing.T) {
	srcs := []string{
		`package Test {
	state S {
		state :>> start;
	}
}`,
		`package Test {
	state S {
		state redefines start;
	}
}`,
		`package Test {
	state S {
		state begin;
	}
}`,
	}
	for _, src := range srcs {
		for _, warm := range []bool{false, true} {
			if got := w9cMessages(w9cLibraryDiags(t, src, warm), msgW9CDuplicateInherited); len(got) != 0 {
				t.Errorf("warm=%v: %s got %v, want none", warm, src, got)
			}
		}
	}
}

// The fixtures the pinned validate-sysml is compared against: it reports the
// same two warnings on the first and is silent on the second.
func TestW9CInheritedNameLibraryBaseFixtures(t *testing.T) {
	got := w9cMessages(libraryFixtureDiags(t, "inherited_name_library_base.sysml"), msgW9CDuplicateInherited)
	want := []string{
		msgW9CDuplicateInherited + " 'start' from StateAction",
		msgW9CDuplicateInherited + " 'done' from Action",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got %v, want %v", got, want)
	}
	if diags := libraryFixtureDiags(t, "inherited_name_library_base_clean.sysml"); len(diags) != 0 {
		t.Errorf("clean fixture: got %v, want none", diags)
	}
}

// A metadata usage body names owned redefinitions of the metadata definition's
// features (SysML.xtext MetadataBodyUsage), so reusing those names is silent
// (examples/pilot-corpora RationaleMetadataExample.sysml:11).
func TestW9CMetadataBodyNamesStaySilent(t *testing.T) {
	src := `package Test {
	private import ModelingMetadata::Rationale;
	part engine;
	metadata why : Rationale about engine {
		text = "because";
	}
}`
	for _, warm := range []bool{false, true} {
		if got := w9cMessages(w9cLibraryDiags(t, src, warm), msgW9CDuplicateInherited); len(got) != 0 {
			t.Errorf("warm=%v: got %v, want none", warm, got)
		}
	}
}

// A standard library package outside the standard library is a warning
// (LibraryPackage_invalid_notStandard.kerml.xt:15); a plain library package is not.
func TestW9CUserStandardLibraryPackage(t *testing.T) {
	w9cWantLines(t, "standard library package Outer {\n\tlibrary package Inner;\n}", "library-package", 1)
	w9cWantLines(t, "library package Outer {\n\tlibrary package Inner;\n}", "library-package")
}

// Binding features of unrelated types is a warning; a specializing type conforms
// (BindingConnector_Invalid3.sysml.xt:56).
func TestW9CBoundFeatureTypes(t *testing.T) {
	src := `package Test {
	part def A;
	part def B;
	part def C specializes B;
	part P {
		part a : A;
		part b : B;
		part c : C;
		bind a = b;
		bind c = b;
	}
}`
	w9cWantLines(t, src, "bound-feature-types", 9)
}

// Redefining an untyped library feature replaces its implicit value typing, so
// no diamond is drawn through it (examples/pilot-corpora TradeStudyTest.sysml:18).
func TestW9CRedefinedUntypedFeatureStaysSilent(t *testing.T) {
	srcs := []string{
		`package Test {
	part def Engine;
	analysis def A {
		subject : Engine;
		return part : Engine;
	}
}`,
		`package Test {
	private import Flows::*;
	part def Fuel;
	flow def FuelFlow {
		ref :>> payload : Fuel;
	}
}`,
	}
	for _, src := range srcs {
		for _, warm := range []bool{false, true} {
			if got := w9cMessages(w9cLibraryDiags(t, src, warm), msgW9CDuplicateInherited); len(got) != 0 {
				t.Errorf("warm=%v: got %v, want none", warm, got)
			}
		}
	}
}

// A variant only references a member of its variation, so it draws no diamond
// (examples/pilot-corpora VehicleVariabilityModel.sysml:128).
func TestW9CVariantStaysSilent(t *testing.T) {
	src := `package Test {
	part def ABlock;
	action def AnActivity;
	part P {
		action a4 : ABlock;
		action a6 : ABlock;
		variation action a : AnActivity {
			variant a4;
			variant a6;
		}
	}
}`
	got := w9cMessages(w9cLibraryDiags(t, src, false), msgW9CDuplicateInherited)
	for _, msg := range got {
		if !strings.Contains(msg, "from Action, Part") {
			t.Errorf("unexpected %q", msg)
		}
	}
	if len(got) != 6 { // a4 and a6 themselves, not the variants referencing them
		t.Errorf("got %v, want the two typed actions only", got)
	}
}

// The wave-9c rules are registered once each and level-scoped (AGENTS.md §4).
func TestW9CPassesAreRegistered(t *testing.T) {
	want := map[string]PassLevel{
		"W9CShortNameDistinguishabilityPass": LevelNameResolution,
		"W9CUserStandardLibraryPass":         LevelNameResolution,
		"W9CInheritedNameConflictPass":       LevelType,
		"W9CBoundFeatureTypesPass":           LevelType,
	}
	seen := map[string]int{}
	for _, p := range DefaultRegistry().passes {
		name := fmt.Sprintf("%T", p)
		if i := strings.LastIndex(name, "."); i >= 0 {
			name = name[i+1:]
		}
		if level, ok := want[name]; ok {
			seen[name]++
			if p.Level() != level {
				t.Errorf("%s at level %v, want %v", name, p.Level(), level)
			}
		}
	}
	for name := range want {
		if seen[name] != 1 {
			t.Errorf("%s registered %d time(s), want 1", name, seen[name])
		}
	}
}

// w9cWantLines asserts the 1-based lines carrying a diagnostic with code.
func w9cWantLines(t *testing.T, src, code string, want ...int) {
	t.Helper()
	got := w8dLines(t, src, code)
	if len(got) != len(want) {
		t.Fatalf("%s: got lines %v, want %v", code, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s: got lines %v, want %v", code, got, want)
		}
	}
}
