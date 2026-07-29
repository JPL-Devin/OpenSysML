package lexer

import "testing"

func TestKeywordsExportsList(t *testing.T) {
	kws := Keywords()
	if len(kws) == 0 {
		t.Fatal("Keywords() returned empty")
	}
	found := false
	for _, k := range kws {
		if k == "package" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Keywords() missing 'package'")
	}
	// Must be a copy: mutating the result must not affect subsequent calls.
	kws[0] = "MUTATED"
	if Keywords()[0] == "MUTATED" {
		t.Error("Keywords() leaked internal slice")
	}
}
