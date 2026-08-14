package model

import "testing"

// exposeOwnerFindings returns the expose-owning-namespace diagnostics of a document.
func exposeOwnerFindings(t *testing.T, uri string, docs map[string]string) []string {
	t.Helper()
	ws := NewWorkspace()
	for name, src := range docs {
		ws.Open(name, []byte(src), 1)
		defer ws.Close(name)
	}
	var out []string
	for _, d := range ws.Diagnostics(uri) {
		if d.Code == "expose-owning-namespace" {
			out = append(out, d.Severity.String()+": "+d.Message)
		}
	}
	return out
}

// An expose must be owned by a view usage (SysML v2 8.3.26.2): legal in a view
// usage, a warning in a view def body, an error elsewhere.
func TestExposeOwnerAcrossDocuments(t *testing.T) {
	lib := `package Lib { part def Pub; }`
	docs := map[string]string{
		"lib.sysml":  lib,
		"ok.sysml":   `package Ok { view v { expose Lib::**; } }`,
		"warn.sysml": `package Warn { view def V { expose Lib::**; } }`,
		"bad.sysml":  `package Bad { part def D { expose Lib::**; } }`,
	}

	if found := exposeOwnerFindings(t, "ok.sysml", docs); len(found) != 0 {
		t.Errorf("expose in a view usage must be legal, got %v", found)
	}
	warn := exposeOwnerFindings(t, "warn.sysml", docs)
	if len(warn) != 1 || warn[0][:7] != "warning" {
		t.Errorf("expose in a view def body must warn once, got %v", warn)
	}
	bad := exposeOwnerFindings(t, "bad.sysml", docs)
	if len(bad) != 1 || bad[0][:5] != "error" {
		t.Errorf("expose outside a view must error once, got %v", bad)
	}
}
