package main

import (
	"os"
	"os/exec"
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

	// A sheet that declares nothing is still the sheet the run named.
	empty := filepath.Join(t.TempDir(), "empty.css")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	wantReport(t, check(t, binary, documentModel, "-render-document", "Reports::MassReport",
		"-doc-form", "html", "-html-css", empty), 0, "<!DOCTYPE html>", "@layer opensysml")

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

	sheet := runCommand(t, exec.Command(binary, "-html-default-css"))
	wantReport(t, sheet, 0, "@layer opensysml;", "--sysml-font-body")
	if strings.Contains(sheet.stdout, "<article") {
		t.Errorf("-html-default-css writes CSS, not a document:\n%s", sheet.stdout)
	}

	// Writing the sheet is the whole run, so it stands in for no other.
	wantReport(t, check(t, binary, documentModel, "-html-default-css"),
		2, "ask for it without model files")
	wantReport(t, runCommand(t, exec.Command(binary, "-html-default-css", "-render-document", "Reports::MassReport")),
		2, "ask for it in its own run")
	wantReport(t, runCommand(t, exec.Command(binary, "-html-default-css", "-convert", "kerml")),
		2, "ask for it in its own run")
	wantReport(t, runCommand(t, exec.Command(binary, "-html-default-css", "-html-no-default-css")),
		2, "not the sheet")
	wantReport(t, runCommand(t, exec.Command(binary, "-html-default-css", "-sync-diff", "repo.ttl")),
		2, "ask for it in its own run")
	wantReport(t, runCommand(t, exec.Command(binary, "-html-default-css", "-sync-apply", "https://example.test/api")),
		2, "ask for it in its own run")
	for _, unrelated := range [][]string{
		{"-from", "kerml"},
		{"-strict"},
		{"-sync-base", "base.ttl"},
		{"-sync-state", "state.json"},
		{"-sync-confirm-deletes"},
		{"-sync-mint-ids"},
		{"-sync-annotate", "all"},
	} {
		wantReport(t, runCommand(t, exec.Command(binary, append([]string{"-html-default-css"}, unrelated...)...)),
			2, "do not apply")
	}

	// -o is where the sheet is written, so it stays supported.
	destination := filepath.Join(t.TempDir(), "sysml-document.css")
	wantReport(t, runCommand(t, exec.Command(binary, "-html-default-css", "-o", destination)), 0)
	written, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("read written stylesheet: %v", err)
	}
	if !strings.Contains(string(written), "@layer opensysml;") {
		t.Errorf("-o did not receive the default stylesheet:\n%s", written)
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

// TestRenderDocumentHTMLStylesheetURLScheme checks a stylesheet URL is linked
// whatever case its scheme is written in, rather than read as a file.
func TestRenderDocumentHTMLStylesheetURLScheme(t *testing.T) {
	binary := buildCLI(t)
	wantReport(t, check(t, binary, documentModel, "-render-document", "Reports::MassReport",
		"-doc-form", "html", "-html-css", "HTTPS://Example.test/Site.css"),
		0, `<link rel="stylesheet" href="HTTPS://Example.test/Site.css">`)
}

// TestSetStylesheetNameLength checks a stylesheet name whose escaping outgrows
// the file name limit is shortened to a distinct, still-suffixed name.
func TestSetStylesheetNameLength(t *testing.T) {
	taken := map[string]bool{}
	long := strings.Repeat("é", 120) + ".css"
	name := setStylesheetName(long, taken)
	if len(name) > 255 {
		t.Errorf("name is %d bytes: %q", len(name), name)
	}
	if !strings.HasSuffix(name, ".css") {
		t.Errorf("name lost its extension: %q", name)
	}
	other := setStylesheetName(strings.Repeat("é", 121)+".css", taken)
	if other == name {
		t.Errorf("two stylesheets share the name %q", name)
	}
	if len(other) > 255 {
		t.Errorf("name is %d bytes: %q", len(other), other)
	}
}
