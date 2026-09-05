package xmi

import (
	"archive/zip"
	"bytes"
	"os"
	"strings"
	"testing"
)

const fixture = "../migrate/testdata/cameo/vehicle.xmi"

func readFixture(t *testing.T) *Model {
	t.Helper()
	data, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	m, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestParseTree(t *testing.T) {
	m := readFixture(t)
	if m.Exporter != "MagicDraw UML" {
		t.Errorf("exporter = %q", m.Exporter)
	}
	if len(m.Roots) != 2 || m.Roots[0].Type != "Model" || m.Roots[0].Name != "Model" {
		t.Fatalf("roots = %+v", m.Roots)
	}
	vehicle := m.Lookup("_blk_vehicle")
	if vehicle == nil || vehicle.Type != "Class" || vehicle.Role != "packagedElement" {
		t.Fatalf("vehicle = %+v", vehicle)
	}
	if got := strings.Join(vehicle.Path(), "::"); got != "Model::Vehicle Design::Vehicle" {
		t.Errorf("path = %q", got)
	}
	if attrs := vehicle.Owned("ownedAttribute"); len(attrs) != 13 {
		t.Errorf("owned attributes = %d", len(attrs))
	}
	// The diagram inside xmi:Extension is tool-private and not read.
	if m.Lookup("_diag_bdd") != nil {
		t.Error("xmi:Extension content was read")
	}
}

func TestReferences(t *testing.T) {
	m := readFixture(t)
	engine := m.Lookup("_prop_engine")
	if got := m.Ref(engine, "type"); got == nil || got.ID != "_blk_engine" {
		t.Errorf("type ref = %+v", got)
	}
	// Multi-valued attribute references are space-separated.
	assoc := m.Lookup("_assoc_vehicle_engine")
	if ends := m.Refs(assoc, "memberEnd"); len(ends) != 2 || ends[1].ID != "_ae_vehicle_1" {
		t.Errorf("memberEnd = %+v", ends)
	}
	// Child idref elements resolve too.
	cmt := m.Lookup("_cmt_vehicle")
	if got := m.Ref(cmt, "annotatedElement"); got == nil || got.ID != "_blk_vehicle" {
		t.Errorf("annotatedElement = %+v", got)
	}
	// An href yields a named proxy when its fragment reads as a name.
	name := m.Lookup("_prop_name")
	typ := m.Ref(name, "type")
	if typ == nil || !typ.IsProxy() || typ.Name != "String" {
		t.Errorf("href type = %+v", typ)
	}
	// An opaque body is the element's text.
	body := m.Lookup("_dv_total").Owned("body")
	if len(body) != 1 || strings.TrimSpace(body[0].Text) != "mass + engine.mass" {
		t.Errorf("body = %+v", body)
	}
}

func TestStereotypes(t *testing.T) {
	m := readFixture(t)
	vehicle := m.Lookup("_blk_vehicle")
	block := vehicle.Stereotype("Block")
	if block == nil || block.Tag("isEncapsulated") != "true" {
		t.Fatalf("Block = %+v", block)
	}
	if !strings.Contains(block.Namespace, "SysML") {
		t.Errorf("namespace = %q", block.Namespace)
	}
	req := m.Lookup("_req_mass_1").Stereotype("Requirement")
	if req == nil || req.Tag("Id") != "R1.1" || req.Tag("Text") != "The chassis shall have a mass of less than 400 kg." {
		t.Errorf("Requirement tags = %+v", req)
	}
	nested := m.Lookup("_ce_nested_2").Stereotype("NestedConnectorEnd")
	if nested == nil || len(nested.Tags["propertyPath"]) != 2 || nested.Tags["propertyPath"][1] != "_prop_piston" {
		t.Errorf("propertyPath = %+v", nested)
	}
	if !m.Lookup("_req_speed").HasStereotype("performanceRequirement") {
		t.Error("MagicDraw customization stereotype not applied")
	}
}

func TestParseArchive(t *testing.T) {
	data, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range []string{"PROJECT_MANIFEST", "com.nomagic.magicdraw.core.project.options"} {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte("<options/>"))
	}
	w, err := zw.Create("com.nomagic.magicdraw.uml_model.model")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write(data)
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	m, err := Parse(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if m.Lookup("_blk_vehicle") == nil {
		t.Error("archive model entry was not read")
	}
}

func TestParseArchiveWithoutModel(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("readme.txt")
	_, _ = w.Write([]byte("nothing"))
	_ = zw.Close()
	_, err := Parse(buf.Bytes())
	if err == nil || !strings.Contains(err.Error(), "readme.txt") {
		t.Errorf("err = %v", err)
	}
}

func TestParseRejectsNonXMI(t *testing.T) {
	for _, in := range []string{"part def V;", "<html><body/></html>", ""} {
		if _, err := Parse([]byte(in)); err == nil {
			t.Errorf("%q: accepted", in)
		}
	}
}

func TestParseBareModelRoot(t *testing.T) {
	src := `<?xml version="1.0"?>
<uml:Model xmi:version="2.1" xmlns:xmi="http://schema.omg.org/spec/XMI/2.1" xmlns:uml="http://www.eclipse.org/uml2/5.0.0/UML" xmi:id="m" name="M">
  <packagedElement xmi:type="uml:Class" xmi:id="c" name="C"/>
</uml:Model>`
	m, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Roots) != 1 || m.Roots[0].Type != "Model" || m.Lookup("c") == nil {
		t.Errorf("roots = %+v", m.Roots)
	}
}
