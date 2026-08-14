package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestExpandDirectoryIsWalkedInSortedOrder(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "b.sysml"), "package B { }\n")
	write(t, filepath.Join(dir, "a.sysml"), "package A { }\n")
	write(t, filepath.Join(dir, "sub", "c.kerml"), "package C { }\n")
	write(t, filepath.Join(dir, "notes.md"), "ignored\n")
	write(t, filepath.Join(dir, ".hidden", "d.sysml"), "package D { }\n")

	got, err := Expand([]string{dir})
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	want := []string{
		filepath.Join(dir, "a.sysml"),
		filepath.Join(dir, "b.sysml"),
		filepath.Join(dir, "sub", "c.kerml"),
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("Expand(%s) = %v, want %v", dir, got, want)
	}
}

func TestExpandGlob(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "a.sysml"), "package A { }\n")
	write(t, filepath.Join(dir, "b.sysml"), "package B { }\n")
	write(t, filepath.Join(dir, "c.kerml"), "package C { }\n")

	got, err := Expand([]string{filepath.Join(dir, "*.sysml")})
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	want := []string{filepath.Join(dir, "a.sysml"), filepath.Join(dir, "b.sysml")}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("glob = %v, want %v", got, want)
	}
}

func TestExpandGlobMatchingNothingIsReported(t *testing.T) {
	dir := t.TempDir()
	_, err := Expand([]string{filepath.Join(dir, "*.sysml")})
	if err == nil {
		t.Fatal("expected an error for a pattern matching nothing")
	}
	if !strings.Contains(err.Error(), "no model files match") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExpandEmptyDirectoryIsReported(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "README.md"), "no models here\n")
	_, err := Expand([]string{dir})
	if err == nil {
		t.Fatal("expected an error for a directory holding no model files")
	}
	if !strings.Contains(err.Error(), "no .sysml or .kerml files") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExpandFileNamedTwiceLoadsOnce(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "a.sysml")
	write(t, file, "package A { }\n")

	got, err := Expand([]string{file, dir, file})
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if len(got) != 1 || got[0] != file {
		t.Fatalf("Expand = %v, want [%s]", got, file)
	}
}

func TestExpandMissingFileIsLeftToTheLoader(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope.sysml")
	got, err := Expand([]string{missing})
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if len(got) != 1 || got[0] != missing {
		t.Fatalf("Expand = %v, want [%s]", got, missing)
	}
}

func TestExpandMultipleInputsKeepTheOrderGiven(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "z.sysml"), "package Z { }\n")
	write(t, filepath.Join(dir, "sub", "a.sysml"), "package A { }\n")
	write(t, filepath.Join(dir, "sub", "b.sysml"), "package B { }\n")

	got, err := Expand([]string{filepath.Join(dir, "z.sysml"), filepath.Join(dir, "sub")})
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	want := []string{
		filepath.Join(dir, "z.sysml"),
		filepath.Join(dir, "sub", "a.sysml"),
		filepath.Join(dir, "sub", "b.sysml"),
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("Expand = %v, want %v", got, want)
	}
}

func TestIsModelFile(t *testing.T) {
	for path, want := range map[string]bool{
		"a.sysml": true, "a.SysML": true, "a.kerml": true,
		"a.md": false, "sysml": false, "a.sysml.bak": false,
	} {
		if got := IsModelFile(path); got != want {
			t.Errorf("IsModelFile(%q) = %v, want %v", path, got, want)
		}
	}
}
