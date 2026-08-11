package repl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMetaSaveSysML(t *testing.T) {
	s := NewSession()
	s.Submit("package Demo {\npart def Engine;\n}")
	path := filepath.Join(t.TempDir(), "out.sysml")

	out, quit, err := s.runMeta("%save " + path)
	if err != nil || quit {
		t.Fatalf("%%save: err=%v quit=%v", err, quit)
	}
	if !strings.Contains(strings.Join(out, "\n"), "saved") {
		t.Errorf("expected a confirmation, got %v", out)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"package Demo", "part def Engine;"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("saved file is missing %q:\n%s", want, data)
		}
	}
}

func TestMetaSaveTurtle(t *testing.T) {
	s := NewSession()
	s.Submit("package Demo { part def Engine; }")
	path := filepath.Join(t.TempDir(), "out.ttl")

	if _, _, err := s.runMeta("%save " + path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"@prefix sysml:", "elmt:Demo::Engine", "a sysml:PartDefinition"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("saved Turtle is missing %q:\n%s", want, data)
		}
	}
}

// TestMetaSaveKeepsEveryDeclaration checks that a save covers the whole session,
// not just the last thing typed.
func TestMetaSaveKeepsEveryDeclaration(t *testing.T) {
	s := NewSession()
	s.Submit("package First { part def A; }")
	s.Submit("package Second { part def B; }")
	path := filepath.Join(t.TempDir(), "out.sysml")

	if _, _, err := s.runMeta("%save " + path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"package First", "package Second"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("saved file is missing %q:\n%s", want, data)
		}
	}
}

func TestMetaSaveUsage(t *testing.T) {
	s := NewSession()
	s.Submit("package P { }")
	out, _, err := s.runMeta("%save")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(out, "\n"), "usage:") {
		t.Errorf("expected usage guidance, got %v", out)
	}
}

func TestMetaSaveEmptySession(t *testing.T) {
	s := NewSession()
	path := filepath.Join(t.TempDir(), "out.sysml")
	out, _, err := s.runMeta("%save " + path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(out, "\n"), "empty") {
		t.Errorf("expected an empty-session message, got %v", out)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("an empty session should not write a file")
	}
}

func TestMetaSaveUnknownExtension(t *testing.T) {
	s := NewSession()
	s.Submit("package P { }")
	path := filepath.Join(t.TempDir(), "out.json")
	out, _, err := s.runMeta("%save " + path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(out, "\n"), "cannot tell the format") {
		t.Errorf("expected a format error, got %v", out)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("an unsupported extension should not write a file")
	}
}

func TestMetaSaveThenLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "model.sysml")

	first := NewSession()
	first.Submit("package Demo { part def Engine; part def Vehicle { part engine : Engine[1]; } }")
	if _, _, err := first.runMeta("%save " + path); err != nil {
		t.Fatal(err)
	}

	second := NewSession()
	if _, _, err := second.runMeta("%load " + path); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(second.List(), "\n"); !strings.Contains(got, "package Demo") {
		t.Errorf("the saved file did not load back: %v", second.List())
	}
}

// TestMetaSaveHelp checks the command is discoverable.
func TestMetaSaveHelp(t *testing.T) {
	s := NewSession()
	out, _, err := s.runMeta("%help")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(out, "\n"), "%save") {
		t.Error("%help should list the save command")
	}
}
