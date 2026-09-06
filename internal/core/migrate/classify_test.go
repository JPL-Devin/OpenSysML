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

func TestInstanceOfUnwritableClassifierIsUnmapped(t *testing.T) {
	cases := map[string]string{
		"proxy": `<packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_i" name="car">
      <classifier href="other.xmi#_ext"/>
    </packagedElement>`,
		"library": `<packagedElement xmi:type="uml:Package" xmi:id="_lib" name="Lib">
      <packagedElement xmi:type="uml:Class" xmi:id="_lc" name="LibThing"/>
    </packagedElement>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_i" name="car" classifier="_lc"/>`,
		"unmapped": `<packagedElement xmi:type="uml:Activity" xmi:id="_act" name="Drive"/>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_i" name="car" classifier="_act"/>`,
	}
	for name, members := range cases {
		t.Run(name, func(t *testing.T) {
			r := migrateDocument(t, members, `<sysml:ModelLibrary xmi:id="_ml" base_Package="_lib"/>`)
			wantNoLine(t, r.Notation, "individual def")
			if es := entriesFor(r, "_i"); len(es) != 1 || es[0].Verdict != migrate.Unmapped {
				t.Errorf("entries = %+v", es)
			}
			for _, d := range errors(t, name+".sysml", r.Notation) {
				t.Errorf("%v", d)
			}
		})
	}
}

func TestDanglingDependencyEndIsReported(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_a" name="A"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_b" name="B"/>
    <packagedElement xmi:type="uml:Abstraction" xmi:id="_d" client="_a _gone" supplier="_b"/>`,
		`<sysml:Block xmi:id="_s1" base_Class="_a"/><sysml:Block xmi:id="_s2" base_Class="_b"/><sysml:Trace xmi:id="_s3" base_Abstraction="_d"/>`)
	wantLine(t, r.Notation, "dependency A to B;")
	es := entriesFor(r, "_d")
	if len(es) != 1 || es[0].Verdict != migrate.Approximated || !strings.Contains(es[0].Note, "_gone") {
		t.Errorf("entries = %+v", es)
	}
}

func TestBlockEncapsulationIsKeptAndReported(t *testing.T) {
	r := migrateFixture(t)
	wantLine(t, r.Notation, "/* «Block» tags with no v2 form: isEncapsulated = true */")
	es := entriesFor(r, "_blk_vehicle")
	if len(es) != 1 || es[0].Verdict != migrate.Approximated || !strings.Contains(es[0].Note, "isEncapsulated") {
		t.Errorf("entries = %+v", es)
	}
}

func TestBooleanLiteralForms(t *testing.T) {
	for val, want := range map[string]string{"true": "true", "1": "true", "false": "false", "0": "false"} {
		r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_a" name="A">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p" name="on">
        <type href="http://www.omg.org/spec/UML/20161101/PrimitiveTypes.xmi#Boolean"/>
        <defaultValue xmi:type="uml:LiteralBoolean" xmi:id="_v" value="`+val+`"/>
      </ownedAttribute>
    </packagedElement>`, `<sysml:Block xmi:id="_s1" base_Class="_a"/>`)
		wantLine(t, r.Notation, "attribute on : ScalarValues::Boolean default = "+want)
	}
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_a" name="A">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p" name="on">
        <type href="http://www.omg.org/spec/UML/20161101/PrimitiveTypes.xmi#Boolean"/>
        <defaultValue xmi:type="uml:LiteralBoolean" xmi:id="_v" value="yes"/>
      </ownedAttribute>
    </packagedElement>`, `<sysml:Block xmi:id="_s1" base_Class="_a"/>`)
	wantNoLine(t, r.Notation, "default =")
	if es := entriesFor(r, "_p"); len(es) != 1 || es[0].Verdict != migrate.Approximated || !strings.Contains(es[0].Note, `"yes"`) {
		t.Errorf("entries = %+v", es)
	}
}

func TestInstanceWithSeveralClassifiers(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_a" name="A"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_b" name="B"/>
    <packagedElement xmi:type="uml:Activity" xmi:id="_act" name="Drive"/>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_both" name="both" classifier="_a _b"/>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_mixed" name="mixed" classifier="_a _act"/>`,
		`<sysml:Block xmi:id="_s1" base_Class="_a"/><sysml:Block xmi:id="_s2" base_Class="_b"/>`)
	wantLine(t, r.Notation, "individual def both :> A, B;")
	wantLine(t, r.Notation, "individual def mixed :> A;")
	if es := entriesFor(r, "_both"); len(es) != 1 || es[0].Verdict != migrate.Mapped {
		t.Errorf("both entries = %+v", es)
	}
	if es := entriesFor(r, "_mixed"); len(es) != 1 || es[0].Verdict != migrate.Approximated || !strings.Contains(es[0].Note, "Drive is not migrated") {
		t.Errorf("mixed entries = %+v", es)
	}
	for _, d := range errors(t, "multi.sysml", r.Notation) {
		t.Errorf("%v", d)
	}
}

// A comment that also annotates an element with no v2 declaration keeps its
// writable subjects and is approximated, naming what it left out.
func TestCommentAboutUnwrittenElementIsApproximated(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_b" name="Thing"/>
    <packagedElement xmi:type="uml:Activity" xmi:id="_act" name="Run"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_r" name="Req">
      <ownedComment xmi:type="uml:Comment" xmi:id="_c1" body="see both" annotatedElement="_r _b _act"/>
      <ownedComment xmi:type="uml:Comment" xmi:id="_c2" body="see the run" annotatedElement="_r _act"/>
    </packagedElement>`, `<sysml:Block xmi:id="_s1" base_Class="_b"/><sysml:Block xmi:id="_s2" base_Class="_r"/>`)
	wantLine(t, r.Notation, "comment about Req, Thing /* see both */")
	wantLine(t, r.Notation, "comment about Req /* see the run */")
	for _, id := range []string{"_c1", "_c2"} {
		es := entriesFor(r, id)
		if len(es) != 1 || es[0].Verdict != migrate.Approximated || !strings.Contains(es[0].Note, "Run") {
			t.Errorf("%s entries = %+v", id, es)
		}
	}
	for _, d := range errors(t, "self.sysml", r.Notation) {
		t.Errorf("%v", d)
	}
}

// packageRootDocument is an XMI document whose only UML root is a package.
func packageRootDocument(name, members, applications string) []byte {
	return []byte(`<?xml version="1.0" encoding="UTF-8"?>
<xmi:XMI xmi:version="2.5.1" xmlns:xmi="http://www.omg.org/spec/XMI/20131001"
         xmlns:uml="http://www.omg.org/spec/UML/20161101"
         xmlns:sysml="http://www.omg.org/spec/SysML/20181001/SysML">
  <uml:Package xmi:type="uml:Package" xmi:id="_p" name="` + name + `">
` + members + `
  </uml:Package>
` + applications + `
</xmi:XMI>`)
}

func TestSolePackageRootWithLibraryNameMigrates(t *testing.T) {
	for _, name := range []string{"SysML", "Libraries", "QUDV"} {
		r, err := migrate.Migrate("t.xmi", packageRootDocument(name, `
    <packagedElement xmi:type="uml:Class" xmi:id="_b" name="Thing"/>`, `<sysml:Block xmi:id="_st" base_Class="_b"/>`))
		if err != nil {
			t.Fatal(err)
		}
		wantLine(t, r.Notation, "part def Thing;")
		if es := entriesFor(r, "_b"); len(es) != 1 || es[0].Verdict != migrate.Mapped {
			t.Errorf("%s::Thing: entries = %+v", name, es)
		}
	}
	t.Run("beside the user's model", func(t *testing.T) {
		r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_own" name="Own"/>`,
			`<uml:Package xmi:type="uml:Package" xmi:id="_lib" name="SysML">
    <packagedElement xmi:type="uml:Class" xmi:id="_b" name="Thing"/>
  </uml:Package>
  <sysml:Block xmi:id="_s1" base_Class="_own"/><sysml:Block xmi:id="_st" base_Class="_b"/>`)
		wantLine(t, r.Notation, "part def Own;")
		wantNoLine(t, r.Notation, "part def Thing")
		if es := entriesFor(r, "_lib"); len(es) != 1 || es[0].Verdict != migrate.Skipped {
			t.Errorf("entries = %+v", es)
		}
	})
}

func TestEmptyUpperBoundIsZero(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_a" name="A"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_b" name="B">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p1" name="none" type="_a" aggregation="composite">
        <lowerValue xmi:type="uml:LiteralInteger" xmi:id="_l1" value="0"/>
        <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="_u1"/>
      </ownedAttribute>
    </packagedElement>`, `<sysml:Block xmi:id="_s1" base_Class="_a"/><sysml:Block xmi:id="_s2" base_Class="_b"/>`)
	wantLine(t, r.Notation, "part none : A[0];")
}

func TestCustomStereotypesWithStandardNamesAreNotConsumed(t *testing.T) {
	custom := func(name, base, id string) string {
		return `<custom:` + name + ` xmlns:custom="http://example.com/custom" xmi:id="_c` + id + `" base_` + base + `="` + id + `"/>`
	}
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_r" name="Req"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_b" name="Thing">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_fp" name="x" type="_r"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Package" xmi:id="_lib" name="Lib">
      <packagedElement xmi:type="uml:Class" xmi:id="_lb" name="InLib"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Abstraction" xmi:id="_d" name="link" client="_b" supplier="_r"/>`,
		custom("Requirement", "Class", "_r")+custom("Block", "Class", "_b")+custom("ModelLibrary", "Package", "_lib")+
			custom("FlowProperty", "Property", "_fp")+custom("Satisfy", "Abstraction", "_d")+`<sysml:Block xmi:id="_s" base_Class="_lb"/>`)
	wantNoLine(t, r.Notation, "requirement def")
	wantLine(t, r.Notation, "part def Req {")
	wantLine(t, r.Notation, "applied stereotype «Requirement»")
	wantLine(t, r.Notation, "applied stereotype «Block»")
	wantLine(t, r.Notation, "applied stereotype «ModelLibrary»")
	wantLine(t, r.Notation, "applied stereotype «FlowProperty»")
	wantLine(t, r.Notation, "part def InLib;")
	wantLine(t, r.Notation, "ref part x : Req {")
	wantLine(t, r.Notation, "dependency link from Thing to Req;")
	wantNoLine(t, r.Notation, "satisfy")
	if es := entriesFor(r, "_r"); len(es) != 1 || es[0].Verdict != migrate.Approximated || !strings.Contains(es[0].Note, "without «Block»") {
		t.Errorf("_r entries = %+v", es)
	}
	if es := entriesFor(r, "_lib"); len(es) != 1 || es[0].Verdict != migrate.Mapped {
		t.Errorf("_lib entries = %+v", es)
	}
	if es := entriesFor(r, "_d"); len(es) != 1 || es[0].Verdict != migrate.Approximated || !strings.Contains(es[0].Note, "«Satisfy» is written as a plain dependency") {
		t.Errorf("_d entries = %+v", es)
	}
}
