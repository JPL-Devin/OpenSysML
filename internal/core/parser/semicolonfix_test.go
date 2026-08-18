package parser

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// diagnose parses src and returns its syntax diagnostics.
func diagnose(t *testing.T, src string) []Diagnostic {
	t.Helper()
	p := New(source.New("test.sysml", []byte(src)))
	p.ParseFile()
	return p.Diagnostics
}

// A missing semicolon the parser can place exactly carries the edit that inserts
// it, right after the construct that needed it.
func TestMissingSemicolonCarriesAnInsertFix(t *testing.T) {
	const src = "package P {\n    action def A {\n        first start\n    }\n}\n"
	diags := diagnose(t, src)
	var found bool
	for _, d := range diags {
		if !strings.Contains(d.Message, "expected ';'") {
			continue
		}
		found = true
		if len(d.Fixes) != 1 {
			t.Fatalf("diagnostic %q carries %d fixes, want one", d.Message, len(d.Fixes))
		}
		fix := d.Fixes[0]
		if fix.Title != "Insert ';'" || !fix.Preferred {
			t.Errorf("fix = %+v", fix)
		}
		if len(fix.Edits) != 1 {
			t.Fatalf("fix carries %d edits, want one", len(fix.Edits))
		}
		edit := fix.Edits[0]
		if edit.NewText != ";" || edit.Span.Len != 0 {
			t.Errorf("edit = %+v, want an insertion of ';'", edit)
		}
		// The insertion point is the end of the text preceding it, so applying
		// the edit repairs the source.
		want := strings.Index(src, "first start") + len("first start")
		if edit.Span.Offset != want {
			t.Errorf("edit offset = %d, want %d (just after %q)", edit.Span.Offset, want,
				src[max(0, edit.Span.Offset-11):edit.Span.Offset])
		}
	}
	if !found {
		t.Fatalf("no missing-semicolon diagnostic for %q: %v", src, diags)
	}
}

// A syntax error with more than one repair — a declaration may take a body or a
// semicolon — carries no fix, since neither is unambiguous.
func TestAmbiguousTerminatorCarriesNoFix(t *testing.T) {
	diags := diagnose(t, "package P {\n    part def Wheel { attribute pressure }\n}\n")
	if len(diags) == 0 {
		t.Fatal("no diagnostic for a member missing its terminator")
	}
	for _, d := range diags {
		if len(d.Fixes) != 0 {
			t.Errorf("diagnostic %q carries fixes %+v, want none", d.Message, d.Fixes)
		}
	}
}

// Fixes are extra data on a diagnostic: a well-formed document has neither.
func TestCleanParseHasNoDiagnostics(t *testing.T) {
	if diags := diagnose(t, "package P {\n    part def Wheel;\n}\n"); len(diags) != 0 {
		t.Errorf("diagnostics = %+v, want none", diags)
	}
}
