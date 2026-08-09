package symbols

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func addDoc(t *testing.T, idx *Index, name, src string) {
	t.Helper()
	sf := source.New(name, []byte(src))
	p := parser.New(sf)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("unexpected parse diagnostics for %s: %v", name, p.Diagnostics)
	}
	idx.AddDocument(name, root)
}

func TestIndexQualifiedLookup(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "a.sysml", "package P { package Q { namespace N; } }")

	syms := idx.LookupQualified("P::Q::N")
	if len(syms) != 1 {
		t.Fatalf("LookupQualified(P::Q::N) len = %d, want 1", len(syms))
	}
	if syms[0].Kind != SymbolNamespace {
		t.Fatalf("P::Q::N kind = %v, want namespace", syms[0].Kind)
	}
	if len(idx.LookupQualified("P::Missing")) != 0 {
		t.Fatalf("LookupQualified(P::Missing) should be empty")
	}
}

func TestIndexAmbiguousQualified(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "a.sysml", "package P { namespace D; }")
	addDoc(t, idx, "b.sysml", "package P { namespace D; }")

	if got := len(idx.LookupQualified("P::D")); got != 2 {
		t.Fatalf("LookupQualified(P::D) len = %d, want 2 (ambiguous)", got)
	}
}

// A member a namespace declares shadows one of the same name that a wildcard
// import re-exports through it, so the qualified name stays unambiguous — the
// pattern of SI::min, which is SI's own minute and not an imported function.
func TestIndexOwnedMemberShadowsWildcardReexport(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "a.sysml", "package Functions { calc def min; calc def clamp; }")
	addDoc(t, idx, "b.sysml", "package Units { public import Functions::*; attribute <min> minute; }")
	idx.ExpandWildcardImports()

	syms := idx.LookupQualified("Units::min")
	if len(syms) != 1 {
		t.Fatalf("LookupQualified(Units::min) len = %d, want 1", len(syms))
	}
	if syms[0].Name != "minute" {
		t.Errorf("Units::min = %q, want the declared minute", syms[0].Name)
	}
	if got := len(idx.LookupQualified("Units::clamp")); got != 1 {
		t.Errorf("LookupQualified(Units::clamp) len = %d, want 1: "+
			"a re-export stays visible when nothing shadows it", got)
	}
}

// Wildcard imports chain — KerML imports Kernel::*, Kernel imports Core::*,
// Core imports Root::* — and each link names a sibling of the importing
// package, so expansion has to follow the chain to its end and look the target
// up in the enclosing namespaces, not only in the importer and the root. The
// answer must not depend on the order the importers happen to be visited in.
func TestExpandWildcardImportsChainsAndIsOrderIndependent(t *testing.T) {
	const src = `package L {
		public import Mid::*;
		package Base { part def Element; attribute def <kg> Kilogram; }
		package Mid { public import Base::*; }
	}`
	want := []string{"L::Base::Element", "L::Mid::Element", "L::Element", "L::kg", "L::Mid::kg"}

	// Repeat: the pass walks a map, so a single run could pass by luck.
	for i := 0; i < 8; i++ {
		idx := NewIndex()
		addDoc(t, idx, "l.sysml", src)
		idx.ExpandWildcardImports()
		for _, fqn := range want {
			if got := len(idx.LookupQualified(fqn)); got != 1 {
				t.Fatalf("run %d: LookupQualified(%s) len = %d, want 1", i, fqn, got)
			}
		}
	}
}

// A relative wildcard target names the innermost enclosing declaration of that
// name, not a top-level package that happens to share it.
func TestExpandWildcardImportsPrefersTheEnclosingTarget(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "outer.sysml", "package Systems { part def Outer; }")
	addDoc(t, idx, "sysml.sysml", "package SysML { public import Systems::*; package Systems { part def Inner; } }")
	idx.ExpandWildcardImports()

	if got := len(idx.LookupQualified("SysML::Inner")); got != 1 {
		t.Errorf("LookupQualified(SysML::Inner) len = %d, want 1", got)
	}
	if got := len(idx.LookupQualified("SysML::Outer")); got != 0 {
		t.Errorf("LookupQualified(SysML::Outer) len = %d, want 0: "+
			"SysML::Systems shadows the top-level Systems", got)
	}
}

// A wildcard target names the package an earlier import brought into the
// importing namespace before a top-level one of that name, since KerML 8.2.3.5
// resolves a name against a namespace's imported memberships. Here P imports
// Outer::* (re-exporting Outer::Shared as P::Shared) and then Shared::*, which
// names that P::Shared; its members are read from where it was declared, since
// a re-export registers the symbol alone and copies none of its subtree.
func TestExpandWildcardImportsFollowsAReexportedTarget(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "shared.sysml", "package Shared { part def Widget; }")
	addDoc(t, idx, "outer.sysml", "package Outer { package Shared { part def Imported; } }")
	addDoc(t, idx, "p.sysml", "package P { public import Outer::*; public import Shared::*; }")
	idx.ExpandWildcardImports()

	if got := len(idx.LookupQualified("P::Imported")); got != 1 {
		t.Errorf("LookupQualified(P::Imported) len = %d, want 1: "+
			"`import Shared::*` names the P::Shared that `import Outer::*` "+
			"brought in, and its members live under Outer::Shared", got)
	}
	if got := len(idx.LookupQualified("P::Widget")); got != 0 {
		t.Errorf("LookupQualified(P::Widget) len = %d, want 0: "+
			"the imported Shared shadows the top-level one", got)
	}
}

// A wildcard import can name its target by the package's short name, whose
// index entry holds none of the members: they live under the declared FQN.
func TestExpandWildcardImportsResolvesAShortNameTarget(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "lib.sysml", "package <USCU> USCustomaryUnits { part def Inch; }")
	addDoc(t, idx, "p.sysml", "package P { public import USCU::*; }")
	idx.ExpandWildcardImports()

	if got := len(idx.LookupQualified("P::Inch")); got != 1 {
		t.Errorf("LookupQualified(P::Inch) len = %d, want 1: "+
			"`import USCU::*` names USCustomaryUnits, whose members live under its "+
			"declared name", got)
	}
}

// A name is exported when any import that surfaced it was public, so importing
// a namespace publicly still passes it on after a private import of the same
// name reached it first (KerML 8.2.3.3).
func TestExpandWildcardImportsExportsAPubliclyReimportedName(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "base.sysml", "package Base { part def Shared; }")
	addDoc(t, idx, "mid.sysml", "package Mid { public import Base::*; }")
	addDoc(t, idx, "top.sysml",
		"package Top { private import Base::*; public import Mid::*; }")
	addDoc(t, idx, "user.sysml", "package User { public import Top::*; }")
	idx.ExpandWildcardImports()

	if got := len(idx.LookupQualified("User::Shared")); got != 1 {
		t.Errorf("LookupQualified(User::Shared) len = %d, want 1: "+
			"Top imports Shared publicly through Mid as well", got)
	}
}

// Following a re-exported target only works while the name means one thing: two
// imports bringing in different packages of that name leave it ambiguous, and an
// ambiguous target imports nothing rather than picking one of them.
func TestExpandWildcardImportsIgnoresAnAmbiguousTarget(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "a.sysml", "package A { package Shared { part def FromA; } }")
	addDoc(t, idx, "b.sysml", "package B { package Shared { part def FromB; } }")
	addDoc(t, idx, "p.sysml",
		"package P { public import A::*; public import B::*; public import Shared::*; }")
	idx.ExpandWildcardImports()

	if got := len(idx.fqn["P::Shared"]); got != 2 {
		t.Fatalf("P::Shared names %d symbols, want the 2 re-exports this case needs", got)
	}
	for _, fqn := range []string{"P::FromA", "P::FromB"} {
		if got := len(idx.LookupQualified(fqn)); got != 0 {
			t.Errorf("LookupQualified(%s) len = %d, want 0: "+
				"`import Shared::*` cannot choose between A::Shared and B::Shared", fqn, got)
		}
	}
}

// A declared package stays a usable wildcard target when an import re-exported
// something of the same name alongside it — the declaration shadows the
// re-export, as it does for any lookup.
func TestExpandWildcardImportsTargetSurvivesAReexportOfItsName(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "base.sysml", "package Base { package Util { part def FromBase; } }")
	addDoc(t, idx, "lib.sysml", "package Lib { public import Base::*; "+
		"package Util { part def A; } package Sub { public import Util::*; } }")
	idx.ExpandWildcardImports()

	if got := len(idx.LookupQualified("Lib::Sub::A")); got != 1 {
		t.Errorf("LookupQualified(Lib::Sub::A) len = %d, want 1: "+
			"`import Util::*` names Lib's own Util", got)
	}
	if got := len(idx.LookupQualified("Lib::Sub::FromBase")); got != 0 {
		t.Errorf("LookupQualified(Lib::Sub::FromBase) len = %d, want 0: "+
			"Lib::Util shadows the re-exported Base::Util", got)
	}
}

// A private import is visible only inside the namespace that declares it, so a
// package importing that namespace must not see what it privately imported.
func TestExpandWildcardImportsDoesNotCarryOnAPrivateImport(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "base.sysml", "package Base { part def Hidden; part def Shown; }")
	addDoc(t, idx, "mid.sysml", "package Mid { private import Base::*; part def Own; }")
	addDoc(t, idx, "top.sysml", "package Top { public import Mid::*; }")
	idx.ExpandWildcardImports()

	if got := len(idx.LookupQualified("Mid::Hidden")); got != 1 {
		t.Errorf("LookupQualified(Mid::Hidden) len = %d, want 1: "+
			"a private import is visible inside Mid", got)
	}
	if got := len(idx.LookupQualified("Top::Own")); got != 1 {
		t.Errorf("LookupQualified(Top::Own) len = %d, want 1: Mid::Own is public", got)
	}
	if got := len(idx.LookupQualified("Top::Hidden")); got != 0 {
		t.Errorf("LookupQualified(Top::Hidden) len = %d, want 0: "+
			"Mid imported Base privately, so Top does not see Base's members", got)
	}
}

func TestIndexDocumentRoot(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "a.sysml", "package P;")
	rs := idx.DocumentRoot("a.sysml")
	if rs == nil {
		t.Fatalf("DocumentRoot(a.sysml) = nil")
	}
	if _, ok := rs.LookupLocal("P"); !ok {
		t.Fatalf("document root missing P")
	}
	if idx.DocumentRoot("missing.sysml") != nil {
		t.Fatalf("DocumentRoot(missing) should be nil")
	}
}

func TestIndexShortNameNotDuplicatedInFQN(t *testing.T) {
	// A package with both short and primary names registers one symbol; the
	// FQN uses the primary name. Both local keys still resolve via the scope.
	idx := NewIndex()
	addDoc(t, idx, "a.sysml", "package <p> Primary { namespace N; }")
	if len(idx.LookupQualified("Primary::N")) != 1 {
		t.Fatalf("Primary::N not indexed")
	}
}

func TestIndexRemoveDocument(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "a.sysml", "package P { namespace N; }")
	if got := idx.LookupQualified("P::N"); len(got) != 1 {
		t.Fatalf("before remove: P::N = %d symbols, want 1", len(got))
	}
	idx.RemoveDocument("a.sysml")
	if got := idx.LookupQualified("P::N"); len(got) != 0 {
		t.Fatalf("after remove: P::N = %d symbols, want 0", len(got))
	}
	if idx.DocumentRoot("a.sysml") != nil {
		t.Fatalf("after remove: DocumentRoot should be nil")
	}
}

func TestIndexReAddReplacesStaleEntries(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "a.sysml", "package P { namespace Old; }")
	addDoc(t, idx, "a.sysml", "package P { namespace New; }")
	if got := idx.LookupQualified("P::Old"); len(got) != 0 {
		t.Fatalf("P::Old = %d symbols after re-add, want 0 (stale)", len(got))
	}
	if got := idx.LookupQualified("P::New"); len(got) != 1 {
		t.Fatalf("P::New = %d symbols after re-add, want 1", len(got))
	}
	if got := idx.LookupQualified("P"); len(got) != 1 {
		t.Fatalf("P = %d symbols after re-add, want 1 (not doubled)", len(got))
	}
}

func TestIndexRemoveUnknownDocumentNoop(t *testing.T) {
	idx := NewIndex()
	idx.RemoveDocument("missing.sysml") // must not panic
	addDoc(t, idx, "a.sysml", "package P;")
	idx.RemoveDocument("b.sysml") // unrelated doc untouched
	if got := idx.LookupQualified("P"); len(got) != 1 {
		t.Fatalf("P = %d after removing unrelated doc, want 1", len(got))
	}
}
