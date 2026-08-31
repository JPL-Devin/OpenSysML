package docpdf

import (
	"errors"
	"strings"
	"testing"
)

// TestRenderWithInstalledEngines exercises each real converter when its tools
// are installed, and skips otherwise; the contract itself is tested with
// fakes in docpdf_test.go.
func TestRenderWithInstalledEngines(t *testing.T) {
	for _, engine := range Engines() {
		t.Run(engine, func(t *testing.T) {
			converter, err := EngineNamed(engine)
			if err != nil {
				t.Fatal(err)
			}
			if err := converter.Available(); err != nil {
				var docErr *Error
				if errors.As(err, &docErr) && docErr.Kind == ErrorToolMissing {
					t.Skipf("%s not installed: %v", engine, err)
				}
				t.Fatal(err)
			}
			pdf, err := Render("# Smoke Test\n\nOne paragraph\\.\n", engine, Options{TOC: true, NumberSections: true})
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			if !strings.HasPrefix(string(pdf), "%PDF-") {
				t.Fatalf("output is no PDF: %.16q", pdf)
			}
		})
	}
}

// TestRenderInlineRunsWithInstalledEngines renders a document using every
// inline construct — styled runs, links, reference links to anchors, and
// grouped-table headings — through each installed converter.
func TestRenderInlineRunsWithInstalledEngines(t *testing.T) {
	markdown := strings.Join([]string{
		"# Inline Report",
		"",
		`<a id="Inline.20Report-masses"></a>`,
		"",
		"## Masses",
		"",
		"The *margin* is **critical** for `m > 0` per [spec](<https://example.com/spec.md>)\\.",
		"",
		"See [the masses](#Inline.20Report-masses) above\\.",
		"",
		"**zone: hot**",
		"",
		"| name | zone |",
		"| --- | --- |",
		"| Mirror | hot |",
		"",
	}, "\n")
	for _, engine := range Engines() {
		t.Run(engine, func(t *testing.T) {
			converter, err := EngineNamed(engine)
			if err != nil {
				t.Fatal(err)
			}
			if err := converter.Available(); err != nil {
				var docErr *Error
				if errors.As(err, &docErr) && docErr.Kind == ErrorToolMissing {
					t.Skipf("%s not installed: %v", engine, err)
				}
				t.Fatal(err)
			}
			pdf, err := Render(markdown, engine, Options{})
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			if !strings.HasPrefix(string(pdf), "%PDF-") {
				t.Fatalf("output is no PDF: %.16q", pdf)
			}
		})
	}
}

// TestRenderDiagramsWithInstalledMermaid renders a real diagram when
// mermaid-cli and an engine are installed, and skips otherwise.
func TestRenderDiagramsWithInstalledMermaid(t *testing.T) {
	if _, err := mermaidTool.locate(""); err != nil {
		t.Skipf("mmdc not installed: %v", err)
	}
	converter, err := EngineNamed("")
	if err != nil {
		t.Fatal(err)
	}
	if err := converter.Available(); err != nil {
		t.Skipf("%s not installed: %v", converter.Name(), err)
	}
	markdown := "# Diagram Test\n\n```mermaid\nflowchart LR\n  a --> b\n```\n"
	pdf, err := Render(markdown, "", Options{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.HasPrefix(string(pdf), "%PDF-") {
		t.Fatalf("output is no PDF: %.16q", pdf)
	}
}
