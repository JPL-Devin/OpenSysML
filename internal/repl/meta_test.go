package repl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMetaHelpAndList(t *testing.T) {
	s := NewSession()
	s.Submit("package P { }")
	out, quit, err := s.runMeta("%help")
	if err != nil || quit {
		t.Fatalf("%%help: err=%v quit=%v", err, quit)
	}
	if !strings.Contains(strings.Join(out, "\n"), "%load") {
		t.Errorf("%%help should list commands: %v", out)
	}
	out, _, _ = s.runMeta("%list")
	if !strings.Contains(strings.Join(out, "\n"), "package P") {
		t.Errorf("%%list should show declarations: %v", out)
	}
}

func TestMetaClear(t *testing.T) {
	s := NewSession()
	s.Submit("package P { }")
	if _, _, err := s.runMeta("%clear"); err != nil {
		t.Fatal(err)
	}
	if len(s.List()) != 0 {
		t.Errorf("%%clear should empty session, got %v", s.List())
	}
}

func TestMetaLoad(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "m.sysml")
	if err := os.WriteFile(f, []byte("package Loaded { }"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := NewSession()
	if _, _, err := s.runMeta("%load " + f); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(s.List(), "\n"), "Loaded") {
		t.Errorf("%%load should submit file contents: %v", s.List())
	}
}

func TestIsMeta(t *testing.T) {
	if !isMeta("%help") || isMeta("package P") || isMeta("") {
		t.Fatal("isMeta classification wrong")
	}
}
