package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRenderDocumentHTMLFlag checks the scripted surface of HTML rendering:
// a whole page on stdout, the model facts on the markup, and an artifact
// written to -o.
func TestRenderDocumentHTMLFlag(t *testing.T) {
	binary := buildCLI(t)

	got := check(t, binary, documentModel, "-render-document", "Reports::MassReport", "-doc-form", "html")
	wantReport(t, got, 0,
		"<!DOCTYPE html>",
		"@layer opensysml",
		`<article class="sysml-document" data-document="Reports::MassReport">`,
		`<h1 class="sysml-title">Telescope Mass Report</h1>`,
		`<section class="sysml-section"`,
		`<caption class="sysml-caption">Heavy subsystems by mass</caption>`,
		`<th scope="col" data-column="mass">mass</th>`,
		`data-element-kind="partUsage"`)
	if strings.Contains(got.stderr, "<article") {
		t.Errorf("the artifact belongs on stdout, not stderr:\n%s", got.stderr)
	}

	out := filepath.Join(t.TempDir(), "report.html")
	wantReport(t, check(t, binary, documentModel,
		"-render-document", "Reports::MassReport", "-doc-form", "html", "-o", out),
		0, "wrote "+out, "(html,")
	written, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(written), "<!DOCTYPE html>") {
		t.Errorf("written artifact is not a page:\n%s", written)
	}
}

// TestRenderDocumentHTMLStylesheets checks the stylesheet options: a file is
// inlined after the default layer, a URL is linked, the default sheet can be
// left out, a fragment carries neither, and the default sheet is writable on
// its own.
func TestRenderDocumentHTMLStylesheets(t *testing.T) {
	binary := buildCLI(t)
	css := filepath.Join(t.TempDir(), "theme.css")
	if err := os.WriteFile(css, []byte(".sysml-document { color: rebeccapurple; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := check(t, binary, documentModel, "-render-document", "Reports::MassReport",
		"-doc-form", "html", "-html-css", css, "-html-css", "https://example.test/site.css")
	wantReport(t, got, 0,
		"@layer opensysml",
		"rebeccapurple",
		`<link rel="stylesheet" href="https://example.test/site.css">`)
	if strings.Index(got.stdout, "@layer opensysml {") > strings.Index(got.stdout, "rebeccapurple") {
		t.Errorf("supplied CSS must follow the default layer:\n%s", got.stdout)
	}

	bare := check(t, binary, documentModel, "-render-document", "Reports::MassReport",
		"-doc-form", "html", "-html-no-default-css")
	wantReport(t, bare, 0, "<!DOCTYPE html>")
	if strings.Contains(bare.stdout, "@layer opensysml") {
		t.Errorf("-html-no-default-css left the default sheet in:\n%s", bare.stdout)
	}

	fragment := check(t, binary, documentModel, "-render-document", "Reports::MassReport",
		"-doc-form", "html", "-html-fragment")
	wantReport(t, fragment, 0, `<article class="sysml-document"`)
	if strings.Contains(fragment.stdout, "<!DOCTYPE html>") || strings.Contains(fragment.stdout, "<style>") {
		t.Errorf("a fragment carries neither page shell nor stylesheet:\n%s", fragment.stdout)
	}

	sheet := check(t, binary, documentModel, "-html-default-css")
	wantReport(t, sheet, 0, "@layer opensysml;", "--sysml-font-body")
	if strings.Contains(sheet.stdout, "<article") {
		t.Errorf("-html-default-css writes CSS, not a document:\n%s", sheet.stdout)
	}
}

// TestRenderDocumentHTMLDocumentOptions checks the title page, contents and
// numbering options shape HTML output too, under their -doc- names.
func TestRenderDocumentHTMLDocumentOptions(t *testing.T) {
	binary := buildCLI(t)
	wantReport(t, check(t, binary, documentModel, "-render-document", "Reports::MassReport",
		"-doc-form", "html", "-doc-title-page", "-doc-toc", "-doc-number-sections"),
		0,
		`<header class="sysml-title-page">`,
		`<nav class="sysml-toc"`,
		`<span class="sysml-section-number">1</span>`)
}

// TestRenderDocumentHTMLFlagConflicts checks the HTML flag combinations the
// run refuses.
func TestRenderDocumentHTMLFlagConflicts(t *testing.T) {
	binary := buildCLI(t)

	wantReport(t, check(t, binary, documentModel, "-render-document", "Reports::MassReport", "-html-css", "theme.css"),
		2, "-doc-form html")
	wantReport(t, check(t, binary, documentModel, "-render-document", "Reports::MassReport",
		"-doc-form", "html", "-pdf-engine", "weasyprint"),
		2, "-doc-form html needs no external converter")
	wantReport(t, check(t, binary, documentModel, "-render-document", "Reports::MassReport",
		"-doc-form", "html", "-html-fragment", "-html-css", "theme.css"),
		2, "style the page you embed it in")
	wantReport(t, check(t, binary, documentModel, "-render-document", "Reports::MassReport",
		"-doc-form", "html", "-html-fragment", "-html-no-default-css"),
		2, "-html-fragment already writes no stylesheet")
	wantReport(t, check(t, binary, documentModel, "-render-document", "Reports::MassReport",
		"-doc-form", "html", "-html-css", filepath.Join(t.TempDir(), "absent.css")),
		2, "read stylesheet")
	wantReport(t, check(t, binary, documentModel, "-html-css", "theme.css"),
		2, "apply to -render-document")
}
