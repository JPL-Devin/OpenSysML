package model

import "testing"

// A qualified redefinition of a feature inherited from the standard library is
// legitimate: the library supertype is restored from the cache without a scope,
// so membership cannot be decided by comparing declaring scopes.
func TestRedefineInheritedLibraryFeatureQualified(t *testing.T) {
	ws := NewWorkspace()
	src := `package test {
	part def C {
		snapshot start :>> Parts::Part::start;
		part sub :>> Parts::Part::suboccurrences;
	}
}`
	ws.Open("test.sysml", []byte(src), 1)
	defer ws.Close("test.sysml")

	for _, d := range ws.Diagnostics("test.sysml") {
		t.Errorf("unexpected diagnostic: %v", d)
	}
}

// A qualified redefinition of a feature the owner does not inherit is still an
// error.
func TestRedefineUninheritedFeatureReported(t *testing.T) {
	ws := NewWorkspace()
	src := `package test {
	part def A { part w; }
	part def B { part w2 :>> A::w; }
}`
	ws.Open("test.sysml", []byte(src), 1)
	defer ws.Close("test.sysml")

	diags := ws.Diagnostics("test.sysml")
	for _, d := range diags {
		if d.Code == "redefinition-no-inherited" {
			return
		}
	}
	t.Errorf("expected a redefinition-no-inherited diagnostic, got %v", diags)
}
