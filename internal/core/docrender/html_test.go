package docrender

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// htmlClassVocabulary is the documented class surface: a theme may rely on
// these names, and the backend emits no others.
var htmlClassVocabulary = map[string]bool{
	"sysml-document": true, "sysml-title": true, "sysml-title-page": true,
	"sysml-toc": true, "sysml-toc-title": true,
	"sysml-section": true, "sysml-section-number": true, "sysml-paragraph": true,
	"sysml-table": true, "sysml-group": true, "sysml-group-heading": true,
	"sysml-group-column": true, "sysml-group-key": true,
	"sysml-row": true, "sysml-cell": true, "sysml-value": true, "sysml-element": true,
	"sysml-separator": true, "sysml-list": true, "sysml-item": true,
	"sysml-diagram": true, "sysml-caption": true, "sysml-link": true, "sysml-ref": true,
	"mermaid": true,
}

// renderFixtureHTML evaluates a fixture document and renders it as HTML.
func renderFixtureHTML(t *testing.T, path, name string, opts HTMLOptions) string {
	t.Helper()
	out, err := HTML(fixtureDocument(t, path, name), opts)
	if err != nil {
		t.Fatalf("render document %s as HTML: %v", name, err)
	}
	return out
}

func checkGolden(t *testing.T, got, golden string) {
	t.Helper()
	if *update {
		if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
			t.Fatalf("update golden: %v", err)
		}
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden (run with -update to create): %v", err)
	}
	if got != string(want) {
		t.Errorf("rendered HTML differs from %s (run with -update after intentional changes)\ngot:\n%s", golden, got)
	}
}

// TestHTMLTelescopeReportGolden locks the standalone rendering of the
// telescope report: page shell, layered stylesheet, sections, tables, lists
// and escaped content.
func TestHTMLTelescopeReportGolden(t *testing.T) {
	got := renderFixtureHTML(t, filepath.Join("testdata", "telescope_report.sysml"),
		"Observatory::MassReport", HTMLOptions{})
	checkGolden(t, got, filepath.Join("testdata", "telescope_report.golden.html"))
}

// TestHTMLTelescopeReportFragmentGolden locks the fragment rendering with a
// title page, a table of contents and numbered sections.
func TestHTMLTelescopeReportFragmentGolden(t *testing.T) {
	got := renderFixtureHTML(t, filepath.Join("testdata", "telescope_report.sysml"),
		"Observatory::MassReport",
		HTMLOptions{Fragment: true, TitlePage: true, TOC: true, NumberSections: true})
	checkGolden(t, got, filepath.Join("testdata", "telescope_report.fragment.golden.html"))
}

// TestHTMLSemanticStructure checks the semantic skeleton and the model facts
// carried on it: the document element, nested sections at valid heading
// levels, typed rows and cells, and a diagram figure.
func TestHTMLSemanticStructure(t *testing.T) {
	got := renderFixtureHTML(t, filepath.Join("testdata", "telescope_report.sysml"),
		"Observatory::MassReport", HTMLOptions{})
	for _, want := range []string{
		`<article class="sysml-document" data-document="Observatory::MassReport">`,
		`<h1 class="sysml-title">Telescope Mass Report</h1>`,
		`<section class="sysml-section"`,
		` data-content="section"`,
		`<th scope="col" data-column="mass">mass</th>`,
		`<tr class="sysml-row" data-element="Observatory::telescope::baffle|shroud *tricky*" data-element-kind="partUsage">`,
		`<td class="sysml-cell" data-column="mass" data-value-kind="real"><span class="sysml-value" data-value-kind="real">1.5</span></td>`,
		`<li class="sysml-item" data-element="Observatory::telescope::mount" data-element-kind="partUsage">`,
		`<pre class="mermaid">`,
		`<span class="sysml-value sysml-element" data-value-kind="element" data-element="Observatory::Assembly *frame*" data-element-kind="partDef">`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rendering does not contain %q\n%s", want, got)
		}
	}
	if strings.Contains(got, "<h7") {
		t.Error("rendering writes a heading level HTML has not")
	}
	// Sections nest as elements, so every one that opens is closed.
	if open, close := strings.Count(got, "<section "), strings.Count(got, "</section>"); open != close {
		t.Errorf("%d sections opened, %d closed", open, close)
	}
}

// TestHTMLNoInlineStylesOrUnknownClasses checks the override contract on the
// markup: nothing carries a style attribute, and every class is one the
// documented vocabulary names.
func TestHTMLNoInlineStylesOrUnknownClasses(t *testing.T) {
	for _, opts := range []HTMLOptions{{}, {Fragment: true, TitlePage: true, TOC: true, NumberSections: true}} {
		got := renderFixtureHTML(t, filepath.Join("testdata", "telescope_report.sysml"),
			"Observatory::MassReport", opts)
		if strings.Contains(got, "style=\"") {
			t.Error("rendering carries an inline style attribute, which reader CSS cannot override")
		}
		for _, match := range regexp.MustCompile(`class="([^"]*)"`).FindAllStringSubmatch(got, -1) {
			for _, class := range strings.Fields(match[1]) {
				if !htmlClassVocabulary[class] {
					t.Errorf("rendering emits undocumented class %q", class)
				}
			}
		}
	}
}

// TestHTMLIdentifiersAreAnchorsOnly checks every in-document link resolves to
// an identifier the document declares, and that no two nodes share one.
func TestHTMLIdentifiersAreAnchorsOnly(t *testing.T) {
	got := renderFixtureHTML(t, filepath.Join("testdata", "telescope_report.sysml"),
		"Observatory::MassReport", HTMLOptions{TOC: true})
	ids := map[string]bool{}
	for _, match := range regexp.MustCompile(`\sid="([^"]*)"`).FindAllStringSubmatch(got, -1) {
		if ids[match[1]] {
			t.Errorf("identifier %q is declared twice", match[1])
		}
		ids[match[1]] = true
	}
	for _, match := range regexp.MustCompile(`href="#([^"]*)"`).FindAllStringSubmatch(got, -1) {
		if !ids[match[1]] {
			t.Errorf("link to #%s resolves to nothing; declared: %v", match[1], ids)
		}
	}
}

// TestHTMLDefaultStylesheetIsOverridable checks the cascade contract of the
// default stylesheet: it is declared as a layer before use, every rule sits
// inside it, and every value it sets comes from a --sysml-* token.
func TestHTMLDefaultStylesheetIsOverridable(t *testing.T) {
	css := DefaultStylesheet()
	declaration := strings.Index(css, "@layer opensysml;")
	block := strings.Index(css, "@layer opensysml {")
	if declaration < 0 || block < 0 || declaration > block {
		t.Fatalf("default stylesheet must declare @layer opensysml before using it:\n%s", css)
	}
	if strings.Count(css, "@layer") != 2 {
		t.Errorf("default stylesheet writes more than the one layer:\n%s", css)
	}
	literal := regexp.MustCompile(`(#[0-9a-fA-F]{3}|[0-9.]+(px|rem|em|ch|vh|vw|pt|%)|["'])`)
	for _, line := range strings.Split(css[block:], "\n") {
		text := strings.TrimSpace(line)
		property, value, ok := strings.Cut(text, ":")
		if !ok || strings.HasPrefix(strings.TrimSpace(property), "--sysml-") {
			continue
		}
		if literal.MatchString(value) {
			t.Errorf("declaration %q hardcodes a value; take it from a --sysml-* token", text)
		}
	}
}

// TestHTMLSuppliedStylesheets checks supplied CSS lands after the default
// layer and unlayered, that a URL is linked rather than inlined, and that
// leaving the default out leaves the document unstyled.
func TestHTMLSuppliedStylesheets(t *testing.T) {
	got := renderFixtureHTML(t, filepath.Join("testdata", "telescope_report.sysml"),
		"Observatory::MassReport", HTMLOptions{
			Stylesheets: []Stylesheet{
				{Content: ".sysml-document { color: rebeccapurple; }"},
				{Href: "https://example.test/theme.css"},
			},
		})
	layer := strings.Index(got, "@layer opensysml {")
	supplied := strings.Index(got, "rebeccapurple")
	link := strings.Index(got, `<link rel="stylesheet" href="https://example.test/theme.css">`)
	if layer < 0 || supplied < layer || link < supplied {
		t.Fatalf("supplied stylesheets must follow the default layer in order:\n%s", got)
	}
	if strings.Contains(got[supplied-200:supplied], "@layer") {
		t.Error("supplied CSS is layered; it must stay unlayered to win on cascade origin")
	}
	bare := renderFixtureHTML(t, filepath.Join("testdata", "telescope_report.sysml"),
		"Observatory::MassReport", HTMLOptions{NoDefaultStylesheet: true})
	if strings.Contains(bare, "<style>") {
		t.Error("-html-no-default-css must leave no stylesheet behind")
	}
	fragment := renderFixtureHTML(t, filepath.Join("testdata", "telescope_report.sysml"),
		"Observatory::MassReport", HTMLOptions{Fragment: true})
	if strings.Contains(fragment, "<html") || strings.Contains(fragment, "<style>") {
		t.Error("a fragment carries neither the page shell nor a stylesheet")
	}
	if !strings.HasPrefix(fragment, `<article class="sysml-document"`) {
		t.Errorf("a fragment starts at the document element:\n%s", fragment)
	}
}

// TestHTMLStylesheetErrors checks the typed errors for a stylesheet that is
// neither content nor URL, both at once, or would close its style element.
func TestHTMLStylesheetErrors(t *testing.T) {
	document := fixtureDocument(t, filepath.Join("testdata", "telescope_report.sysml"), "Observatory::MassReport")
	for _, c := range []struct {
		sheet Stylesheet
		kind  ErrorKind
	}{
		{Stylesheet{}, ErrorEmptyStylesheet},
		{Stylesheet{Content: "a{}", Href: "theme.css"}, ErrorAmbiguousStylesheet},
		{Stylesheet{Content: "a{}</STYLE><script>alert(1)</script>"}, ErrorUnsafeStylesheet},
	} {
		_, err := HTML(document, HTMLOptions{Stylesheets: []Stylesheet{c.sheet}})
		var typed *Error
		if !errors.As(err, &typed) || typed.Kind != c.kind {
			t.Errorf("HTML(%+v) error = %v, want %s", c.sheet, err, c.kind)
		}
	}
}

// TestHTMLEscaping checks no content can corrupt the structure: markup
// characters, quotes, closing tags and newlines in text, attributes and
// comments.
func TestHTMLEscaping(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{`<b>&plain</b>`, `&lt;b&gt;&amp;plain&lt;/b&gt;`},
		{`quote " apostrophe '`, `quote &#34; apostrophe &#39;`},
		{`</script><script>alert(1)</script>`, `&lt;/script&gt;&lt;script&gt;alert(1)&lt;/script&gt;`},
		{"two\nlines\r\nand\rmore", "two lines and more"},
	} {
		if got := htmlText(c.in); got != c.want {
			t.Errorf("htmlText(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	if got := attr("data-name", `a"b`); got != ` data-name="a&#34;b"` {
		t.Errorf("attr = %q", got)
	}
	if got := attr("data-name", ""); got != "" {
		t.Errorf("an empty value writes no attribute, got %q", got)
	}
	if got := htmlComment("closes --> early\nand wraps"); got != "closes - -> early and wraps" {
		t.Errorf("htmlComment = %q", got)
	}
}

// TestHTMLLinkSchemes checks a link is an href only for a scheme a document
// navigates to; a script URL is kept as data instead.
func TestHTMLLinkSchemes(t *testing.T) {
	for _, target := range []string{"https://example.test/a", "mailto:a@example.test", "#anchor", "report.html", "./a:b/c"} {
		if _, ok := navigableURL(target); !ok {
			t.Errorf("navigableURL(%q) = false, want true", target)
		}
	}
	for _, target := range []string{"javascript:alert(1)", "JavaScript:alert(1)", "data:text/html,<script>"} {
		if _, ok := navigableURL(target); ok {
			t.Errorf("navigableURL(%q) = true, want false", target)
		}
	}
	if got, _ := navigableURL("https://example.test/a\nb"); got != "https://example.test/ab" {
		t.Errorf("newlines are stripped from a URL, got %q", got)
	}
}

// TestHTMLDeterministic checks repeated rendering of one document is
// byte-identical.
func TestHTMLDeterministic(t *testing.T) {
	document := fixtureDocument(t, filepath.Join("testdata", "telescope_report.sysml"), "Observatory::MassReport")
	opts := HTMLOptions{TOC: true, NumberSections: true, TitlePage: true}
	first, err := HTML(document, opts)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	for i := 0; i < 3; i++ {
		again, err := HTML(document, opts)
		if err != nil {
			t.Fatalf("render again: %v", err)
		}
		if again != first {
			t.Fatal("repeated rendering is not byte-identical")
		}
	}
}

// TestHTMLNilDocument checks the typed error for a missing document.
func TestHTMLNilDocument(t *testing.T) {
	_, err := HTML(nil, HTMLOptions{})
	var typed *Error
	if !errors.As(err, &typed) || typed.Kind != ErrorNilDocument {
		t.Fatalf("HTML(nil) error = %v, want %s", err, ErrorNilDocument)
	}
}

// TestHTMLAnonymousSections checks a section with no name is still addressable:
// every one gets its own identifier, its contents link resolves, and numbering
// follows the document order.
func TestHTMLAnonymousSections(t *testing.T) {
	document := fixtureDocument(t, filepath.Join("testdata", "anonymous_sections.sysml"), "Anonymous::AnonymousReport")
	got, err := HTML(document, HTMLOptions{TOC: true, NumberSections: true})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	ids := regexp.MustCompile(`<section class="sysml-section" id="([^"]*)"`).FindAllStringSubmatch(got, -1)
	if len(ids) != 4 {
		t.Fatalf("sections with identifiers = %d, want 4:\n%s", len(ids), got)
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if id[1] == "" {
			t.Errorf("a section was written without an identifier:\n%s", got)
		}
		if seen[id[1]] {
			t.Errorf("identifier %q is written twice:\n%s", id[1], got)
		}
		seen[id[1]] = true
	}

	for _, entry := range regexp.MustCompile(`<a href="#([^"]*)">`).FindAllStringSubmatch(got, -1) {
		if !seen[entry[1]] {
			t.Errorf("contents links to #%s, which no section carries:\n%s", entry[1], got)
		}
	}
	for _, number := range []string{"1", "1.1", "1.2", "2"} {
		if !strings.Contains(got, `<span class="sysml-section-number">`+number+`</span>`) {
			t.Errorf("section number %s is missing:\n%s", number, got)
		}
	}
}
