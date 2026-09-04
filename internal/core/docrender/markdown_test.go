package docrender

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/docir"
	"github.com/Open-MBEE/OpenSysML/internal/core/docplan"
	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/queryexec"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

var update = flag.Bool("update", false, "rewrite golden Markdown files")

// renderFixtureDocument evaluates a fixture document and renders it as Markdown.
func renderFixtureDocument(t *testing.T, path, name string) string {
	t.Helper()
	markdown, err := Markdown(fixtureDocument(t, path, name))
	if err != nil {
		t.Fatalf("render document %s: %v", name, err)
	}
	return markdown
}

// fixtureDocument runs the whole pipeline on a fixture: parse, resolve,
// semantics, docplan, then document IR evaluation.
func fixtureDocument(t *testing.T, path, name string) *docir.Document {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	index := libs.NewModelIndex()
	p := parser.New(source.New(filepath.Base(path), []byte(content)))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse fixture: %v", p.Diagnostics)
	}
	index.AddDocument(filepath.Base(path), root)
	index.ExpandWildcardImports()
	resolver := resolve.New(index)
	model := semantics.NewModel(resolver)
	matches := symbols.PreferDeclared(index.LookupQualified(name))
	if len(matches) != 1 {
		t.Fatalf("lookup %s: got %d symbols", name, len(matches))
	}
	plan, err := docplan.Compile(index, model, resolver, matches[0])
	if err != nil {
		t.Fatalf("compile document %s: %v", name, err)
	}
	document, err := docir.Evaluate(plan, queryexec.Context{Index: index, Resolver: resolver, Model: model}, queryexec.Options{}, nil)
	if err != nil {
		t.Fatalf("evaluate document %s: %v", name, err)
	}
	return document
}

// TestMarkdownTelescopeReportGolden locks the end-to-end rendering of a
// telescope report — nested sections, projected and unprojected query tables,
// a composed query, a relationship traversal, lists, empty results, and
// metacharacter-laden content — against a committed golden file.
func TestMarkdownTelescopeReportGolden(t *testing.T) {
	got := renderFixtureDocument(t,
		filepath.Join("testdata", "telescope_report.sysml"),
		"Observatory::MassReport")
	golden := filepath.Join("testdata", "telescope_report.golden.md")
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
		t.Errorf("rendered Markdown differs from %s (run with -update after intentional changes)\ngot:\n%s", golden, got)
	}
}

// TestMarkdownGoldenStructure spot-checks structural invariants of the golden
// rendering: heading depths, table shapes, and escaped metacharacters.
func TestMarkdownGoldenStructure(t *testing.T) {
	got := renderFixtureDocument(t,
		filepath.Join("testdata", "telescope_report.sysml"),
		"Observatory::MassReport")
	lines := strings.Split(got, "\n")
	if lines[0] != "# Telescope Mass Report" {
		t.Errorf("title line = %q", lines[0])
	}
	for _, want := range []string{
		"\n## Subsystem Masses \\| by \\*name\\*\n",
		"\n### Heavy Subsystems\n",
		"\n### Missing Subsystems\n",
		"\n## Declared Types\n",
		"| name | mass |\n| --- | --- |\n",
		"| baffle\\|shroud \\*tricky\\* | 1.5 |",
		"kg \\| not \\#grams, \\*not\\* \\_lbs\\_, \\`raw\\`, \\<b>\\&plain\\</b>",
		"| element |\n| --- |\n| Observatory::Assembly \\*frame\\* |",
		"1. mount\n2. segmentControl",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rendering does not contain %q\n%s", want, got)
		}
	}
	// Every pipe inside cell content is escaped, so each table row has the
	// same number of structural pipes as its header.
	for _, line := range lines {
		if strings.HasPrefix(line, "| ") && strings.Contains(line, "\\|") && !strings.HasSuffix(line, " |") {
			t.Errorf("table row not terminated: %q", line)
		}
	}
}

// TestMarkdownDefaultedQueryParameters renders content that omits defaulted
// query parameters: element, inherited, and redefining defaults all apply.
func TestMarkdownDefaultedQueryParameters(t *testing.T) {
	got := renderFixtureDocument(t,
		filepath.Join("testdata", "defaulted_queries.sysml"),
		"Observatory::DefaultedReport")
	want := "# Defaulted Queries\n\n" +
		"<!-- caption -->\n*Heavy subsystems by default*\n\n" +
		"| name | mass |\n| --- | --- |\n| mount | 15 |\n| segmentControl | 20 |\n\n" +
		"- mount 15\n- optics 8.5\n- segmentControl 20\n\n" +
		"- spectrograph 4\n"
	if got != want {
		t.Errorf("rendered Markdown = \n%s\nwant:\n%s", got, want)
	}
}

// TestMarkdownEscaping checks the escaping contract on raw content: table
// cells and prose with every metacharacter class render without opening
// Markdown or HTML structure.
func TestMarkdownEscaping(t *testing.T) {
	cases := []struct{ in, want string }{
		{"a|b", `a\|b`},
		{"*em* _u_ #h `c`", "\\*em\\* \\_u\\_ \\#h \\`c\\`"},
		{"<script>&amp;", `\<script>\&amp;`},
		{`back\slash`, `back\\slash`},
		{"[link]", `\[link\]`},
	}
	for _, c := range cases {
		if got := inline(c.in); got != c.want {
			t.Errorf("inline(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	if got := tableCell("two\nlines|cell"); got != `two<br>lines\|cell` {
		t.Errorf("tableCell = %q", got)
	}
	if got := inline("two\nlines"); got != "two lines" {
		t.Errorf("inline newline = %q", got)
	}
	// Carriage returns are line endings too: CRLF and lone CR fold like LF.
	if got := inline("a\r\nb\rc"); got != "a b c" {
		t.Errorf("inline carriage returns = %q", got)
	}
	if got := tableCell("a\r\nb\rc"); got != "a<br>b<br>c" {
		t.Errorf("tableCell carriage returns = %q", got)
	}
	for _, c := range []struct{ in, want string }{
		{"- bullet", `\- bullet`},
		{"+ plus", `\+ plus`},
		{"> quote", `\> quote`},
		{"12. ordered", `12\. ordered`},
		{"3) ordered", `3\) ordered`},
		{"42 plain", "42 plain"},
		{"    indented code", "indented code"},
		{"\tindented code", "indented code"},
		{"   > quote", `\> quote`},
		{"  - bullet", `\- bullet`},
		{" + plus", `\+ plus`},
		{"   7. ordered", `7\. ordered`},
	} {
		if got := blockStart(c.in); got != c.want {
			t.Errorf("blockStart(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestMarkdownNilDocument checks the typed error for a missing document.
func TestMarkdownNilDocument(t *testing.T) {
	_, err := Markdown(nil)
	var typed *Error
	if !errors.As(err, &typed) || typed.Kind != ErrorNilDocument {
		t.Fatalf("Markdown(nil) error = %v, want %s", err, ErrorNilDocument)
	}
}
