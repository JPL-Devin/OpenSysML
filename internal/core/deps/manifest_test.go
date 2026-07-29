package deps

import "testing"

func TestParseManifestLibraryPaths(t *testing.T) {
	src := `
# comment
library-paths = ["libs", "vendor/sysml"]
`
	m, err := ParseManifest([]byte(src))
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}
	if len(m.LibraryPaths) != 2 || m.LibraryPaths[0] != "libs" || m.LibraryPaths[1] != "vendor/sysml" {
		t.Fatalf("library-paths = %#v", m.LibraryPaths)
	}
}

func TestParseManifestLocalDependency(t *testing.T) {
	src := `
[dependencies.geometry]
path = "../geometry"
`
	m, err := ParseManifest([]byte(src))
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}
	d, ok := m.Dependencies["geometry"]
	if !ok {
		t.Fatalf("dependency geometry missing: %#v", m.Dependencies)
	}
	if d.Path != "../geometry" {
		t.Fatalf("path = %q", d.Path)
	}
}

func TestParseManifestGitDependency(t *testing.T) {
	src := `
[dependencies.si]
git = "https://example.com/si.git"
rev = "abc123"
`
	m, err := ParseManifest([]byte(src))
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}
	d := m.Dependencies["si"]
	if d.Git != "https://example.com/si.git" || d.Rev != "abc123" {
		t.Fatalf("git dep = %#v", d)
	}
}

func TestParseManifestInlineTableDependency(t *testing.T) {
	src := `
[dependencies]
si = { git = "https://example.com/si.git", tag = "v1.0" }
local = { path = "./local" }
`
	m, err := ParseManifest([]byte(src))
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}
	if m.Dependencies["si"].Git != "https://example.com/si.git" || m.Dependencies["si"].Tag != "v1.0" {
		t.Fatalf("si = %#v", m.Dependencies["si"])
	}
	if m.Dependencies["local"].Path != "./local" {
		t.Fatalf("local = %#v", m.Dependencies["local"])
	}
}

func TestParseManifestEmpty(t *testing.T) {
	m, err := ParseManifest([]byte("\n#only a comment\n"))
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}
	if len(m.LibraryPaths) != 0 || len(m.Dependencies) != 0 {
		t.Fatalf("expected empty manifest, got %#v", m)
	}
}
