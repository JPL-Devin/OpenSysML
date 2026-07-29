package libs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEmbedSourceListsAndReads(t *testing.T) {
	src := DefaultSource()
	names := src.List()
	found := false
	for _, n := range names {
		if n == "ScalarValues.kerml" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected ScalarValues.kerml in embedded list, got %v", names)
	}
	data, err := src.Read("ScalarValues.kerml")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty embedded library content")
	}
}

func TestDirSourceOverride(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Custom.kerml"), []byte("package Custom;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SYSML_LIBRARY_PATH", dir)
	src := DefaultSource()
	names := src.List()
	if len(names) != 1 || names[0] != "Custom.kerml" {
		t.Fatalf("expected [Custom.kerml] from override dir, got %v", names)
	}
	data, err := src.Read("Custom.kerml")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if string(data) != "package Custom;\n" {
		t.Fatalf("unexpected override content: %q", data)
	}
}

func TestReadUnknownFileErrors(t *testing.T) {
	src := DefaultSource()
	if _, err := src.Read("Nope.kerml"); err == nil {
		t.Fatal("expected error reading unknown library file")
	}
}
