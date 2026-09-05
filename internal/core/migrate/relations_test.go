package migrate_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/migrate"
)

// document wraps model members and stereotype applications in an XMI document.
func document(members, applications string) []byte {
	return []byte(`<?xml version="1.0" encoding="UTF-8"?>
<xmi:XMI xmi:version="2.5.1" xmlns:xmi="http://www.omg.org/spec/XMI/20131001"
         xmlns:uml="http://www.omg.org/spec/UML/20161101"
         xmlns:sysml="http://www.omg.org/spec/SysML/20181001/SysML">
  <uml:Model xmi:type="uml:Model" xmi:id="_m" name="Model">
` + members + `
  </uml:Model>
` + applications + `
</xmi:XMI>`)
}

func migrateDocument(t *testing.T, members, applications string) *migrate.Result {
	t.Helper()
	r, err := migrate.Migrate("t.xmi", document(members, applications))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func entriesFor(r *migrate.Result, id string) []migrate.Entry {
	var out []migrate.Entry
	for _, e := range r.Report.Entries {
		if e.ID == id {
			out = append(out, e)
		}
	}
	return out
}

func wantLine(t *testing.T, notation []byte, line string) {
	t.Helper()
	if !strings.Contains(string(notation), line) {
		t.Errorf("notation lacks %q:\n%s", line, notation)
	}
}

// flowModel is a system whose connector lists the flow's target end first,
// and whose two ports carry differently named flow properties for each item.
const flowModel = `
    <packagedElement xmi:type="uml:Class" xmi:id="_fuel" name="Fuel"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_heat" name="Heat"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_out_if" name="Outlet">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_out_fuel" name="fuelOut" type="_fuel"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_out_heat" name="heatOut" type="_heat"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_in_if" name="Inlet">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_in_fuel" name="fuelIn" type="_fuel"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_sys" name="System">
      <ownedAttribute xmi:type="uml:Port" xmi:id="_src" name="source" type="_out_if" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Port" xmi:id="_dst" name="sink" type="_in_if" aggregation="composite"/>
      <ownedConnector xmi:type="uml:Connector" xmi:id="_conn">
        <end xmi:type="uml:ConnectorEnd" xmi:id="_e1" role="_dst"/>
        <end xmi:type="uml:ConnectorEnd" xmi:id="_e2" role="_src"/>
      </ownedConnector>
    </packagedElement>
    <packagedElement xmi:type="uml:InformationFlow" xmi:id="_if" informationSource="_src" informationTarget="_dst" conveyed="_fuel" realizingConnector="_conn"/>
    <packagedElement xmi:type="uml:InformationFlow" xmi:id="_if2" informationSource="_src" informationTarget="_dst" conveyed="_fuel _heat" realizingConnector="_conn"/>`

const flowApplications = `
  <sysml:Block xmi:id="_s1" base_Class="_sys"/>
  <sysml:Block xmi:id="_s2" base_Class="_fuel"/>
  <sysml:Block xmi:id="_s3" base_Class="_heat"/>
  <sysml:InterfaceBlock xmi:id="_s4" base_Class="_out_if"/>
  <sysml:InterfaceBlock xmi:id="_s5" base_Class="_in_if"/>
  <sysml:FlowProperty xmi:id="_s6" base_Property="_out_fuel" direction="out"/>
  <sysml:FlowProperty xmi:id="_s7" base_Property="_out_heat" direction="out"/>
  <sysml:FlowProperty xmi:id="_s8" base_Property="_in_fuel" direction="in"/>
  <sysml:ProxyPort xmi:id="_s9" base_Port="_src"/>
  <sysml:ProxyPort xmi:id="_s10" base_Port="_dst"/>
  <sysml:ItemFlow xmi:id="_s11" base_InformationFlow="_if"/>
  <sysml:ItemFlow xmi:id="_s12" base_InformationFlow="_if2"/>`

func TestItemFlowFollowsSourceAndTargetNotEndOrder(t *testing.T) {
	r := migrateDocument(t, flowModel, flowApplications)
	wantLine(t, r.Notation, "connect sink to source;")
	wantLine(t, r.Notation, "flow source.fuelOut to sink.fuelIn;")
	if strings.Contains(string(r.Notation), "flow sink.") {
		t.Errorf("a flow runs from the sink:\n%s", r.Notation)
	}
	if es := entriesFor(r, "_if"); len(es) != 1 || es[0].Verdict != migrate.Mapped {
		t.Errorf("entries for _if = %+v", es)
	}
	if diags := errors(t, "t.sysml", r.Notation); len(diags) > 0 {
		t.Errorf("migrated notation has errors: %v", diags)
	}
}

func TestMultiItemFlowReportsOnce(t *testing.T) {
	r := migrateDocument(t, flowModel, flowApplications)
	if n := strings.Count(string(r.Notation), "flow source.fuelOut to sink.fuelIn;"); n != 2 {
		t.Errorf("fuel flow written %d times, want 2 (one per item flow)", n)
	}
	es := entriesFor(r, "_if2")
	if len(es) != 1 {
		t.Fatalf("entries for _if2 = %+v, want one", es)
	}
	if es[0].Verdict != migrate.Approximated || !strings.Contains(es[0].Note, "Heat") {
		t.Errorf("entry for _if2 = %+v, want approximated over the unwritten Heat flow", es[0])
	}
}

func TestMultiEndedDependenciesWriteEveryPair(t *testing.T) {
	members := `
    <packagedElement xmi:type="uml:Class" xmi:id="_a" name="A"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_b" name="B"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_c" name="C"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_d" name="D"/>
    <packagedElement xmi:type="uml:Abstraction" xmi:id="_alloc" client="_a _b" supplier="_c _d"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_req1" name="R1"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_req2" name="R2"/>
    <packagedElement xmi:type="uml:Abstraction" xmi:id="_sat" client="_a" supplier="_req1 _req2"/>
    <packagedElement xmi:type="uml:Abstraction" xmi:id="_sat_half" client="_a" supplier="_req1 _b"/>`
	applications := `
  <sysml:Block xmi:id="_s1" base_Class="_a"/>
  <sysml:Block xmi:id="_s2" base_Class="_b"/>
  <sysml:Block xmi:id="_s3" base_Class="_c"/>
  <sysml:Block xmi:id="_s4" base_Class="_d"/>
  <sysml:Requirement xmi:id="_s5" base_Class="_req1"/>
  <sysml:Requirement xmi:id="_s6" base_Class="_req2"/>
  <sysml:Allocate xmi:id="_s7" base_Abstraction="_alloc"/>
  <sysml:Satisfy xmi:id="_s8" base_Abstraction="_sat"/>
  <sysml:Satisfy xmi:id="_s9" base_Abstraction="_sat_half"/>`
	r := migrateDocument(t, members, applications)
	for _, line := range []string{
		"allocate A to C;", "allocate A to D;", "allocate B to C;", "allocate B to D;",
		"satisfy requirement : R1;", "satisfy requirement : R2;",
	} {
		wantLine(t, r.Notation, line)
	}
	if n := strings.Count(string(r.Notation), "satisfy requirement : R1;"); n != 2 {
		t.Errorf("R1 satisfied %d times, want 2", n)
	}
	for id, want := range map[string]migrate.Verdict{"_alloc": migrate.Approximated, "_sat": migrate.Approximated, "_sat_half": migrate.Approximated} {
		es := entriesFor(r, id)
		if len(es) != 1 || es[0].Verdict != want {
			t.Errorf("entries for %s = %+v, want one %v", id, es, want)
		}
	}
	if es := entriesFor(r, "_sat_half"); len(es) == 1 && !strings.Contains(es[0].Note, "1 of 2") {
		t.Errorf("_sat_half note = %q, want the unwritten pair counted", es[0].Note)
	}
	if diags := errors(t, "t.sysml", r.Notation); len(diags) > 0 {
		t.Errorf("migrated notation has errors: %v", diags)
	}
}

func TestRequirementKeepsCommentWithBodyElement(t *testing.T) {
	members := `
    <packagedElement xmi:type="uml:Class" xmi:id="_req" name="R">
      <ownedComment xmi:type="uml:Comment" xmi:id="_cmt">
        <body>Rationale in a body element.</body>
        <annotatedElement xmi:idref="_req"/>
      </ownedComment>
    </packagedElement>`
	applications := `
  <sysml:Requirement xmi:id="_s1" base_Class="_req" Id="R-1" Text="The system shall."/>`
	r := migrateDocument(t, members, applications)
	wantLine(t, r.Notation, "doc /* The system shall. */")
	wantLine(t, r.Notation, "comment /* Rationale in a body element. */")
	if es := entriesFor(r, "_cmt"); len(es) != 1 || es[0].Verdict != migrate.Mapped {
		t.Errorf("entries for _cmt = %+v", es)
	}
}
