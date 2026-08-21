package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadCaseFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.cases")
	content := "# comment\nmodel: models/a.sysml\nmodel: models/b.sysml\none ::  :: 2 + 3\ntwo :: P::Q :: value::member\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := readCaseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Models) != 2 || got.Models[1] != "models/b.sysml" || len(got.Cases) != 2 {
		t.Fatalf("readCaseFile() = %+v", got)
	}
	if got.Cases[0].Target != "" || got.Cases[1].Target != "P::Q" {
		t.Fatalf("targets = %+v", got.Cases)
	}
}

func TestReadCaseFileMalformed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.cases")
	if err := os.WriteFile(path, []byte("bad line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := readCaseFile(path)
	if err == nil || !strings.Contains(err.Error(), path+":1:") {
		t.Fatalf("readCaseFile() error = %v", err)
	}
}

func TestReadCaseFileEmptyFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.cases")
	if err := os.WriteFile(path, []byte("id :: target\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := readCaseFile(path)
	if err == nil || !strings.Contains(err.Error(), "expected id :: target :: expression") {
		t.Fatalf("readCaseFile() error = %v", err)
	}
}
