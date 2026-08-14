package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/repl"
)

func write(t *testing.T, path, content string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// A directory is a valid entry point: `sysml /tmp/proj` loads the model files
// under it rather than failing to read a directory.
func TestLoadFilesAcceptsADirectory(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "uses.sysml"), "package Uses { import Defs::*; part w : Wheel; }\n")
	write(t, filepath.Join(dir, "defs", "defs.sysml"), "package Defs { part def Wheel; }\n")

	sess := repl.NewSession()
	if err := loadFiles(sess, []string{dir}); err != nil {
		t.Fatalf("loadFiles(%s): %v", dir, err)
	}
	if got := sess.List(); len(got) != 2 {
		t.Fatalf("want both files in the session, got %v", got)
	}
}

func TestLoadFilesAcceptsAGlob(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "a.sysml"), "package A { }\n")
	write(t, filepath.Join(dir, "b.sysml"), "package B { }\n")

	sess := repl.NewSession()
	if err := loadFiles(sess, []string{filepath.Join(dir, "*.sysml")}); err != nil {
		t.Fatalf("loadFiles(glob): %v", err)
	}
	if got := sess.List(); len(got) != 2 {
		t.Fatalf("want 2 declarations, got %v", got)
	}
}

func TestLoadFilesReportsAMissingPath(t *testing.T) {
	sess := repl.NewSession()
	if err := loadFiles(sess, []string{filepath.Join(t.TempDir(), "nope.sysml")}); err == nil {
		t.Fatal("expected an error for a path that does not exist")
	}
}

func TestLoadFilesWithoutPathsDoesNothing(t *testing.T) {
	sess := repl.NewSession()
	if err := loadFiles(sess, nil); err != nil {
		t.Fatalf("loadFiles(nil): %v", err)
	}
	if got := sess.List(); len(got) != 0 {
		t.Fatalf("want an empty session, got %v", got)
	}
}
