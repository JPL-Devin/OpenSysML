package model

import (
	"testing"
)

func TestPackageFQNIndexing(t *testing.T) {
	ws := NewWorkspace()

	src := `package 'My Package' {
		part def Engine;
	}`
	ws.Open("some/dir/file.sysml", []byte(src), 1)

	// Check what FQN is indexed
	syms := ws.index.LookupQualified("My Package")
	t.Logf("LookupQualified(\"My Package\"): %d symbols", len(syms))
	for _, s := range syms {
		t.Logf("  - %s (kind=%s)", s.Name, s.Kind)
	}

	syms2 := ws.index.LookupQualified("some/dir/file.sysml")
	t.Logf("LookupQualified(\"some/dir/file.sysml\"): %d symbols", len(syms2))
	
	// Check if Engine is under package name
	engine := ws.index.LookupQualified("My Package::Engine")
	t.Logf("LookupQualified(\"My Package::Engine\"): %d symbols", len(engine))
}
