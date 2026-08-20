package passes

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// analyzeSrc returns every diagnostic src produces.
func analyzeSrc(t *testing.T, src string) []Diagnostic {
	t.Helper()
	root := parser.New(source.New("<t>", []byte(src))).ParseFile()
	idx := symbols.NewIndex()
	idx.AddDocument("<t>", root)
	return Analyze("<t>", root, nil, idx)
}

// importVisibilityDiags returns the import-visibility findings of src.
func importVisibilityDiags(t *testing.T, src string) []Diagnostic {
	t.Helper()
	var out []Diagnostic
	for _, d := range analyzeSrc(t, src) {
		if d.Code == "import-visibility" {
			out = append(out, d)
		}
	}
	return out
}

// The indicator is mandatory on an import at any nesting depth; an explicit one
// conforms, and an expose carries an implicit protected visibility instead.
func TestImportVisibilityIndicatorRequired(t *testing.T) {
	cases := []struct {
		name  string
		src   string
		count int
	}{
		{"bare at the root", "import P::*;", 1},
		{"bare in a package", "package Q { import P::*; }", 1},
		{"bare in a definition body", "package Q { part def D { import P::*; } }", 1},
		{"bare in an import body", "package Q { import P::* { import R::*; } }", 2},
		{"public", "public import P::*;", 0},
		{"private", "private import P::*;", 0},
		{"protected", "protected import P::*;", 0},
		{"recursive membership", "import P::**;", 1},
		{"expose", "package P { part p; view v { expose P::**; } }", 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if diags := importVisibilityDiags(t, tc.src); len(diags) != tc.count {
				t.Fatalf("got %d import-visibility diagnostics, want %d: %v", len(diags), tc.count, diags)
			}
		})
	}
}

// The finding is a warning spanning the `import` keyword, so an editor
// underlines the keyword and never marks the document as failing.
func TestImportVisibilityWarnsOnTheKeyword(t *testing.T) {
	src := "package Q {\n\timport P::*;\n}"
	diags := importVisibilityDiags(t, src)
	if len(diags) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %v", len(diags), diags)
	}
	d := diags[0]
	if d.Severity != SeverityWarning {
		t.Errorf("severity = %v, want warning", d.Severity)
	}
	if d.Source != "syntax" {
		t.Errorf("source = %q, want syntax", d.Source)
	}
	if got := src[d.Span.Offset:d.Span.End()]; got != "import" {
		t.Errorf("span covers %q, want \"import\"", got)
	}
}

// The warning does not gate the higher tiers: a bare import still resolves, so
// what it brings in type-checks and an unresolved target is still reported.
func TestImportVisibilityDoesNotGateHigherTiers(t *testing.T) {
	src := "package Lib { part def Widget; }\npackage App { import Lib::*; part w : Widget; import Nowhere::*; }"
	var warned, unresolved bool
	for _, d := range analyzeSrc(t, src) {
		switch d.Code {
		case "import-visibility":
			warned = true
		case "unresolved":
			unresolved = true
		}
	}
	if !warned {
		t.Error("the bare imports produced no import-visibility warning")
	}
	if !unresolved {
		t.Error("the unresolved import target was not reported, so the warning gated name resolution")
	}
}
