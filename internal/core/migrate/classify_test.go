package migrate_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/migrate"
)

func wantNoLine(t *testing.T, notation []byte, line string) {
	t.Helper()
	if strings.Contains(string(notation), line) {
		t.Errorf("notation has %q:\n%s", line, notation)
	}
}

func TestUserPackagesWithLibraryNamesMigrate(t *testing.T) {
	for _, name := range []string{"SysML", "Libraries", "QUDV", "SI Definitions"} {
		r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Package" xmi:id="_p" name="`+name+`">
      <packagedElement xmi:type="uml:Class" xmi:id="_b" name="Thing"/>
    </packagedElement>`, `<sysml:Block xmi:id="_st" base_Class="_b"/>`)
		wantLine(t, r.Notation, "part def Thing;")
		if es := entriesFor(r, "_b"); len(es) != 1 || es[0].Verdict != migrate.Mapped {
			t.Errorf("%s::Thing: entries = %+v", name, es)
		}
	}
	t.Run("marked library", func(t *testing.T) {
		r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Package" xmi:id="_p" name="QUDV">
      <packagedElement xmi:type="uml:Class" xmi:id="_b" name="Thing"/>
    </packagedElement>`, `<StandardProfile:ModelLibrary xmlns:StandardProfile="http://www.omg.org/spec/UML/20161101/StandardProfile" xmi:id="_ml" base_Package="_p"/>`)
		wantNoLine(t, r.Notation, "part def Thing")
		if es := entriesFor(r, "_p"); len(es) != 1 || es[0].Verdict != migrate.Skipped {
			t.Errorf("entries = %+v", es)
		}
	})
}

func TestExternalScalarNamesOnlyFromPrimitiveLibraries(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_b" name="Thing">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_a" name="count">
        <type href="http://www.omg.org/spec/UML/20161101/PrimitiveTypes.xmi#Integer"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_c" name="score">
        <type href="Shared%20Types.mdzip#Integer"/>
      </ownedAttribute>
    </packagedElement>`, `<sysml:Block xmi:id="_st" base_Class="_b"/>`)
	wantLine(t, r.Notation, "attribute count : ScalarValues::Integer;")
	wantNoLine(t, r.Notation, "score : ScalarValues")
	es := entriesFor(r, "_c")
	if len(es) != 1 || es[0].Verdict == migrate.Mapped || !strings.Contains(es[0].Note, "outside the document") {
		t.Errorf("entries = %+v", es)
	}
}

func TestAnonymousAssociationOwningEveryEndIsWritten(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_a" name="A"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_b" name="B">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_ba" name="a" type="_a" association="_as2"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Association" xmi:id="_as1" memberEnd="_e1 _e2">
      <ownedEnd xmi:type="uml:Property" xmi:id="_e1" type="_a" association="_as1">
        <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="_u1" value="*"/>
      </ownedEnd>
      <ownedEnd xmi:type="uml:Property" xmi:id="_e2" type="_b" association="_as1"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Association" xmi:id="_as2" memberEnd="_ba _e3">
      <ownedEnd xmi:type="uml:Property" xmi:id="_e3" type="_b" association="_as2"/>
    </packagedElement>`, `<sysml:Block xmi:id="_s1" base_Class="_a"/><sysml:Block xmi:id="_s2" base_Class="_b"/>`)
	wantLine(t, r.Notation, "connection def unnamed {")
	wantLine(t, r.Notation, "end a : A[1..*];")
	wantLine(t, r.Notation, "end b : B;")
	if es := entriesFor(r, "_as1"); len(es) != 1 || es[0].Verdict != migrate.Approximated {
		t.Errorf("_as1 entries = %+v", es)
	}
	if es := entriesFor(r, "_as2"); len(es) != 1 || es[0].Verdict != migrate.Mapped || es[0].Target != "" {
		t.Errorf("_as2 entries = %+v", es)
	}
	if strings.Count(string(r.Notation), "connection def") != 1 {
		t.Errorf("expected one connection def:\n%s", r.Notation)
	}
}

func TestOmittedMultiplicityBoundDefaultsToOne(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_a" name="A"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_b" name="B">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p1" name="upperOnly" type="_a" aggregation="composite">
        <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="_u1" value="4"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p2" name="lowerOnly" type="_a" aggregation="composite">
        <lowerValue xmi:type="uml:LiteralInteger" xmi:id="_l2" value="0"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p3" name="one" type="_a" aggregation="composite">
        <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="_u3" value="1"/>
      </ownedAttribute>
    </packagedElement>`, `<sysml:Block xmi:id="_s1" base_Class="_a"/><sysml:Block xmi:id="_s2" base_Class="_b"/>`)
	wantLine(t, r.Notation, "part upperOnly : A[1..4];")
	wantLine(t, r.Notation, "part lowerOnly : A[0..1];")
	wantLine(t, r.Notation, "part one : A;")
}

func TestEnumerationLiteralsAreReported(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Enumeration" xmi:id="_e" name="Color">
      <ownedLiteral xmi:type="uml:EnumerationLiteral" xmi:id="_red" name="red"/>
    </packagedElement>`, "")
	if es := entriesFor(r, "_red"); len(es) != 1 || es[0].Verdict != migrate.Mapped || es[0].Target != "Color::red" {
		t.Errorf("entries = %+v", es)
	}
}

func TestUserPrimitiveTypeWithBuiltinNameStaysOwn(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:PrimitiveType" xmi:id="_real" name="Real"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_b" name="Thing">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_a" name="x" type="_real"/>
    </packagedElement>`, `<sysml:Block xmi:id="_st" base_Class="_b"/>`)
	wantLine(t, r.Notation, "attribute def Real;")
	wantLine(t, r.Notation, "attribute x : Real;")
	wantNoLine(t, r.Notation, "ScalarValues")
}

func TestSelfAssociationEndsAreDistinct(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_a" name="Über"/>
    <packagedElement xmi:type="uml:Association" xmi:id="_as" memberEnd="_e1 _e2">
      <ownedEnd xmi:type="uml:Property" xmi:id="_e1" type="_a" association="_as"/>
      <ownedEnd xmi:type="uml:Property" xmi:id="_e2" type="_a" association="_as"/>
    </packagedElement>`, `<sysml:Block xmi:id="_st" base_Class="_a"/>`)
	wantLine(t, r.Notation, "end 'über' : 'Über';")
	wantLine(t, r.Notation, "end 'über2' : 'Über';")
	for _, d := range errors(t, "self.sysml", r.Notation) {
		t.Errorf("%v", d)
	}
}

func TestRequirementTextKeepsCommentsAboutOthers(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_b" name="Thing"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_r" name="Req">
      <ownedComment xmi:type="uml:Comment" xmi:id="_c1" body="see the block" annotatedElement="_r _b"/>
      <ownedComment xmi:type="uml:Comment" xmi:id="_c2" body="" annotatedElement="_r"/>
      <ownedComment xmi:type="uml:Comment" xmi:id="_c3" body="own note" annotatedElement="_r"/>
    </packagedElement>`, `<sysml:Block xmi:id="_s1" base_Class="_b"/><sysml:Requirement xmi:id="_s2" base_Class="_r" Text="Shall work."/>`)
	wantLine(t, r.Notation, "doc /* Shall work. */")
	wantLine(t, r.Notation, "comment about Req, Thing /* see the block */")
	wantLine(t, r.Notation, "comment /* own note */")
	for id, v := range map[string]migrate.Verdict{"_c1": migrate.Mapped, "_c2": migrate.Skipped, "_c3": migrate.Mapped} {
		if es := entriesFor(r, id); len(es) != 1 || es[0].Verdict != v {
			t.Errorf("%s entries = %+v", id, es)
		}
	}
}
