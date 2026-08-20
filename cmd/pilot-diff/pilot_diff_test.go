package main

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestCategorizePilot(t *testing.T) {
	cases := []struct {
		message string
		want    Category
	}{
		{"Couldn't resolve reference to Type 'Real'.", CategoryUnresolved},
		{"mismatched input 'import' expecting '}'", CategorySyntax},
		{"no viable alternative at input '::'", CategorySyntax},
		{"An attribute must be typed by attribute definitions.", CategoryKindMismatch},
		{"Subsetting/redefining feature should not have larger multiplicity upper bound", CategoryMultiplicity},
		{"Must have at least two related elements", CategoryMultiplicity},
		{"Duplicate of other owned member name", CategoryUnmapped},
	}
	for _, c := range cases {
		if got := categorizePilot(c.message); got != c.want {
			t.Errorf("categorizePilot(%q) = %q, want %q", c.message, got, c.want)
		}
	}
}

func TestCategorizeOpenSysML(t *testing.T) {
	cases := []struct {
		code, pass, message string
		want                Category
	}{
		{"unresolved", "nameres", "unresolved reference: length", CategoryUnresolved},
		{"syntax", "syntax", `"on" is a reserved keyword`, CategorySyntax},
		{"subsetting-multiplicity", "type", "upper bound exceeds subsetted cap", CategoryMultiplicity},
		{"", "type", "incommensurable units mm and s", CategoryUnits},
		{"specialization-cycle", "type", "A participates in a specialization cycle", CategoryUnmapped},
	}
	for _, c := range cases {
		if got := categorizeOpenSysML(c.code, c.pass, c.message); got != c.want {
			t.Errorf("categorizeOpenSysML(%q, %q, %q) = %q, want %q", c.code, c.pass, c.message, got, c.want)
		}
	}
}

// A tuple reported more often by one side stays a disagreement for the surplus.
func TestCompareFileBucketsMultisets(t *testing.T) {
	ours := []diagnostic{
		{Line: 3, Severity: "error", Category: CategoryUnresolved, Message: "a"},
		{Line: 3, Severity: "error", Category: CategoryUnresolved, Message: "b"},
		{Line: 9, Severity: "error", Category: CategoryMultiplicity, Message: "c"},
	}
	theirs := []diagnostic{
		{Line: 3, Severity: "error", Category: CategoryUnresolved, Message: "x"},
		{Line: 9, Severity: "warning", Category: CategoryMultiplicity, Message: "y"},
	}

	got := compareFile("m.sysml", ours, theirs)
	if len(got.Agreement) != 1 || got.Agreement[0].Count != 1 || got.Agreement[0].Line != 3 {
		t.Fatalf("agreement = %+v", got.Agreement)
	}
	if len(got.OpenSysMLOnly) != 1 || got.OpenSysMLOnly[0].Line != 3 {
		t.Fatalf("openSysMLOnly = %+v", got.OpenSysMLOnly)
	}
	// The line 9 pair matches on line and category but not severity.
	if len(got.PilotOnly) != 0 {
		t.Fatalf("pilotOnly = %+v", got.PilotOnly)
	}
	if len(got.SeverityMismatch) != 1 {
		t.Fatalf("severityMismatch = %+v", got.SeverityMismatch)
	}
	if sm := got.SeverityMismatch[0]; sm.Line != 9 || sm.OpenSysML != "error" || sm.Pilot != "warning" || sm.Count != 1 {
		t.Errorf("severityMismatch[0] = %+v", sm)
	}
}

// The downloaded OMG corpora live under examples/, which is a root of its own,
// so they must be compared once each rather than twice.
func TestDefaultRootsDoNotOverlap(t *testing.T) {
	dirs := map[string]string{}
	for _, root := range defaultRoots {
		if other, ok := dirs[root.Dir]; ok {
			t.Errorf("roots %s and %s share the directory %s", other, root.Name, root.Dir)
		}
		dirs[root.Dir] = root.Name
	}
	for _, root := range defaultRoots {
		for dir, name := range dirs {
			if name == root.Name || !strings.HasPrefix(dir, root.Dir+"/") {
				continue
			}
			nested := strings.TrimPrefix(dir, root.Dir+"/")
			if !slices.ContainsFunc(root.Skip, func(skip string) bool {
				return nested == skip || strings.HasPrefix(nested, skip+"/")
			}) {
				t.Errorf("root %s does not skip %s, which root %s compares", root.Name, nested, name)
			}
		}
	}
}

func TestCollectFilesSkipsNestedRoots(t *testing.T) {
	repo := t.TempDir()
	for _, rel := range []string{
		"examples/demo.sysml",
		"examples/pilot-corpora/sysml-examples/Model.sysml",
		"examples/pilot-corpora/kerml-examples/Model.kerml",
		"examples/sysml-v2-training/Training.sysml",
	} {
		path := filepath.Join(repo, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("package P;\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got, err := collectFiles(repo, corpusRoot{
		Name: "examples", Dir: "examples",
		Skip: []string{"sysml-v2-training", "pilot-corpora"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"demo.sysml"}; !reflect.DeepEqual(got, want) {
		t.Errorf("collectFiles() = %v, want %v", got, want)
	}
}

func TestBatchByBaseName(t *testing.T) {
	got := batchByBaseName([]string{"a/x.sysml", "b/x.sysml", "c/y.sysml", "d/x.sysml"})
	want := [][]string{
		{"a/x.sysml", "c/y.sysml"},
		{"b/x.sysml"},
		{"d/x.sysml"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("batchByBaseName() = %v, want %v", got, want)
	}
}

// The pilot processes its inputs sequentially, so an importing file must follow
// the file declaring the imported namespace even when it sorts first.
func TestOrderByImports(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a-user.sysml", "package User {\n\tprivate import 'Defs'::*;\n}\n")
	write("b-defs.sysml", "package 'Defs' {\n\tpart def Thing;\n}\n")
	write("c-standalone.sysml", "package Standalone;\n")

	files := []string{"a-user.sysml", "b-defs.sysml", "c-standalone.sysml"}
	want := []string{"b-defs.sysml", "c-standalone.sysml", "a-user.sysml"}
	if got := orderByImports(dir, ".", files); !reflect.DeepEqual(got, want) {
		t.Errorf("orderByImports() = %v, want %v", got, want)
	}
}

// A qualified import must match the longest declared namespace prefix.
func TestOrderByImportsQualifiedPrefix(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("z-provider.sysml", "package Wrapper {\n\tpackage SimpleVehicleModel {\n\t\tpart def Thing;\n\t}\n}\n")
	write("a-importer.sysml", "package Views {\n\tprivate import Wrapper::SimpleVehicleModel::*;\n}\n")

	files := []string{"a-importer.sysml", "z-provider.sysml"}
	want := []string{"z-provider.sysml", "a-importer.sysml"}
	if got := orderByImports(dir, ".", files); !reflect.DeepEqual(got, want) {
		t.Errorf("orderByImports() = %v, want %v", got, want)
	}
}

// Top-level definitions, usages, and aliases are importable declarations too.
func TestOrderByImportsDeclarations(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("z-definition.sysml", "part def DefinitionThing;\n")
	write("y-usage.sysml", "part usageThing;\n")
	write("x-alias.sysml", "alias AliasThing for DefinitionThing;\n")
	write("a-importer.sysml", "package Imports {\n\tprivate import DefinitionThing::*;\n\tprivate import usageThing::*;\n\tprivate import AliasThing::*;\n}\n")

	files := []string{"a-importer.sysml", "z-definition.sysml", "y-usage.sysml", "x-alias.sysml"}
	want := []string{"z-definition.sysml", "y-usage.sysml", "x-alias.sysml", "a-importer.sysml"}
	if got := orderByImports(dir, ".", files); !reflect.DeepEqual(got, want) {
		t.Errorf("orderByImports() = %v, want %v", got, want)
	}
}

// Repeated declarations must still propagate child prefixes from each occurrence.
func TestOrderByImportsRepeatedPackage(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("z-provider.sysml", "package Provider {\n\tpart first;\n}\npackage Provider {\n\tpackage Inner {\n\t\tpart second;\n\t}\n}\n")
	write("a-importer.sysml", "package Views {\n\tprivate import Provider::Inner::*;\n}\n")

	files := []string{"a-importer.sysml", "z-provider.sysml"}
	want := []string{"z-provider.sysml", "a-importer.sysml"}
	if got := orderByImports(dir, ".", files); !reflect.DeepEqual(got, want) {
		t.Errorf("orderByImports() = %v, want %v", got, want)
	}
}

// An import cycle must not hang or drop files.
func TestOrderByImportsCycle(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{
		"a.sysml": "package A {\n\tprivate import B::*;\n}\n",
		"b.sysml": "package B {\n\tprivate import A::*;\n}\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	files := []string{"a.sysml", "b.sysml"}
	if got := orderByImports(dir, ".", files); !reflect.DeepEqual(got, files) {
		t.Errorf("orderByImports() = %v, want %v", got, files)
	}
}

func TestPilotLineParsing(t *testing.T) {
	line := "Part Definition Example.sysml:12:3: warning: Bound features should have conforming types"
	match := pilotLine.FindStringSubmatch(line)
	if match == nil {
		t.Fatalf("no match for %q", line)
	}
	if match[1] != "Part Definition Example.sysml" || match[2] != "12" || match[4] != "warning" {
		t.Errorf("match = %q", match[1:])
	}
	if match[5] != "Bound features should have conforming types" {
		t.Errorf("message = %q", match[5])
	}
}
