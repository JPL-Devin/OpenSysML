package docir

import (
	"errors"
	"os"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/docplan"
	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/queryexec"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

const evaluationImports = `
private import DocumentQueries::*;
private import KerML::Root::Element;
private import ScalarValues::*;
`

const fixtureDoc = "document-evaluation.sysml"

type evaluationFixture struct {
	index    *symbols.Index
	model    *semantics.Model
	resolver *resolve.Resolver
}

func loadEvaluationFixture(t *testing.T, body string) evaluationFixture {
	t.Helper()
	return loadEvaluationSource(t, "package Observatory {"+evaluationImports+body+"}")
}

func loadEvaluationFixtureFile(t *testing.T, path string) evaluationFixture {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return loadEvaluationSource(t, string(content))
}

func loadEvaluationSource(t *testing.T, content string) evaluationFixture {
	t.Helper()
	index := symbols.NewIndex()
	if err := libs.NewLoader(libs.DefaultSource(), nil).LoadAll(index); err != nil {
		t.Fatalf("load standard library: %v", err)
	}
	p := parser.New(source.New(fixtureDoc, []byte(content)))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse fixture: %v", p.Diagnostics)
	}
	index.AddDocument(fixtureDoc, root)
	index.ExpandWildcardImports()
	resolver := resolve.New(index)
	return evaluationFixture{
		index:    index,
		model:    semantics.NewModel(resolver),
		resolver: resolver,
	}
}

func (f evaluationFixture) symbol(t *testing.T, name string) *symbols.Symbol {
	t.Helper()
	matches := symbols.PreferDeclared(f.index.LookupQualified("Observatory::" + name))
	if len(matches) != 1 {
		t.Fatalf("lookup %s: got %d symbols", name, len(matches))
	}
	return matches[0]
}

func (f evaluationFixture) context() queryexec.Context {
	return queryexec.Context{Index: f.index, Resolver: f.resolver, Model: f.model}
}

func (f evaluationFixture) plan(t *testing.T, name string) *docplan.Plan {
	t.Helper()
	plan, err := docplan.Compile(f.index, f.model, f.resolver, f.symbol(t, name))
	if err != nil {
		t.Fatalf("compile document %s: %v", name, err)
	}
	return plan
}

func (f evaluationFixture) evaluate(t *testing.T, name string) (*Document, error) {
	t.Helper()
	return Evaluate(f.plan(t, name), f.context(), queryexec.Options{})
}

func (f evaluationFixture) mustEvaluate(t *testing.T, name string) *Document {
	t.Helper()
	document, err := f.evaluate(t, name)
	if err != nil {
		t.Fatalf("evaluate %s: %v", name, err)
	}
	return document
}

func runTexts(runs []TextRun) []string {
	texts := make([]string, len(runs))
	for i, run := range runs {
		texts[i] = run.Text()
	}
	return texts
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestEvaluateTelescopeDocument(t *testing.T) {
	fixture := loadEvaluationFixtureFile(t, "testdata/telescope_document.sysml")
	document := fixture.mustEvaluate(t, "MassReport")
	if document.Name() != "Observatory::MassReport" || document.Title() != "Telescope Mass Report" {
		t.Fatalf("document = %q %q", document.Name(), document.Title())
	}
	if !document.Origin().Located() || document.Origin().Doc != fixtureDoc {
		t.Fatalf("origin = %+v", document.Origin())
	}
	content := document.Content()
	if len(content) != 2 {
		t.Fatalf("content = %d nodes, want 2", len(content))
	}

	intro := content[0]
	if intro.Kind() != ContentParagraph || intro.Query() != "" {
		t.Fatalf("intro = %s %q", intro.Kind(), intro.Query())
	}
	if !equalStrings(runTexts(intro.Runs()), []string{"Mass rollup for the telescope assembly."}) {
		t.Fatalf("intro runs = %v", runTexts(intro.Runs()))
	}
	if !intro.Runs()[0].Origin().Located() {
		t.Fatalf("intro run origin = %+v", intro.Runs()[0].Origin())
	}

	breakdown := content[1]
	if breakdown.Kind() != ContentSection || breakdown.Title() != "Subsystem Masses" {
		t.Fatalf("breakdown = %s %q", breakdown.Kind(), breakdown.Title())
	}
	children := breakdown.Children()
	if len(children) != 3 {
		t.Fatalf("breakdown children = %d", len(children))
	}

	masses := children[0]
	if masses.Kind() != ContentTable || masses.Caption() != "All subsystems by mass" {
		t.Fatalf("masses = %s %q", masses.Kind(), masses.Caption())
	}
	if masses.Query() != "Observatory::SubsystemTable" || !masses.QueryOrigin().Located() {
		t.Fatalf("masses query = %q origin = %+v", masses.Query(), masses.QueryOrigin())
	}
	columns := masses.Columns()
	if len(columns) != 2 || columns[0].Name() != "name" || columns[1].Name() != "mass" {
		t.Fatalf("masses columns = %+v", columns)
	}
	rows := masses.Rows()
	if len(rows) != 3 {
		t.Fatalf("masses rows = %d, want 3", len(rows))
	}
	var names []string
	for _, row := range rows {
		if !row.Origin().Located() {
			t.Fatalf("row origin = %+v", row.Origin())
		}
		cells := row.Cells()
		if len(cells) != 2 {
			t.Fatalf("row cells = %d", len(cells))
		}
		name, ok := cells[0].Values()[0].String()
		if !ok {
			t.Fatalf("name cell = %+v", cells[0].Values()[0])
		}
		if !cells[0].Values()[0].Origin().Located() {
			t.Fatalf("cell value origin = %+v", cells[0].Values()[0].Origin())
		}
		if _, ok := cells[1].Values()[0].Real(); !ok {
			t.Fatalf("mass cell = %+v", cells[1].Values()[0])
		}
		names = append(names, name)
	}
	if !equalStrings(names, []string{"mount", "optics", "segmentControl"}) {
		t.Fatalf("row names = %v", names)
	}

	heavy := children[1]
	heavyChildren := heavy.Children()
	if len(heavyChildren) != 2 {
		t.Fatalf("heavy children = %d", len(heavyChildren))
	}
	summary := heavyChildren[0]
	if summary.Kind() != ContentParagraph || summary.Query() != "Observatory::HeavySubsystemNames" {
		t.Fatalf("summary = %s %q", summary.Kind(), summary.Query())
	}
	if !equalStrings(runTexts(summary.Runs()), []string{"mount", "segmentControl"}) {
		t.Fatalf("summary runs = %v", runTexts(summary.Runs()))
	}
	for _, run := range summary.Runs() {
		if !run.Origin().Located() {
			t.Fatalf("summary run origin = %+v", run.Origin())
		}
	}
	heavyItems := heavyChildren[1]
	if heavyItems.Kind() != ContentList || heavyItems.Style() != ListNumber {
		t.Fatalf("heavyItems = %s %q", heavyItems.Kind(), heavyItems.Style())
	}
	items := heavyItems.Items()
	if len(items) != 2 {
		t.Fatalf("heavy items = %d", len(items))
	}
	if !equalStrings(runTexts(items[0].Runs()), []string{"mount"}) ||
		!equalStrings(runTexts(items[1].Runs()), []string{"segmentControl"}) {
		t.Fatalf("heavy items = %v %v", runTexts(items[0].Runs()), runTexts(items[1].Runs()))
	}
	for _, item := range items {
		if !item.Origin().Located() {
			t.Fatalf("item origin = %+v", item.Origin())
		}
	}

	missing := children[2]
	missingChildren := missing.Children()
	if len(missingChildren) != 2 {
		t.Fatalf("missing children = %d", len(missingChildren))
	}
	emptyTable := missingChildren[0]
	if emptyTable.Kind() != ContentTable || len(emptyTable.Rows()) != 0 {
		t.Fatalf("empty table = %s %d rows", emptyTable.Kind(), len(emptyTable.Rows()))
	}
	emptyColumns := emptyTable.Columns()
	if len(emptyColumns) != 2 || emptyColumns[0].Name() != "name" || emptyColumns[1].Name() != "mass" {
		t.Fatalf("empty table columns = %+v", emptyColumns)
	}
	emptyList := missingChildren[1]
	if emptyList.Kind() != ContentList || len(emptyList.Items()) != 0 {
		t.Fatalf("empty list = %s %d items", emptyList.Kind(), len(emptyList.Items()))
	}
}

func TestEvaluatedDocumentIsImmutable(t *testing.T) {
	fixture := loadEvaluationFixtureFile(t, "testdata/telescope_document.sysml")
	document := fixture.mustEvaluate(t, "MassReport")
	content := document.Content()
	content[0] = Content{}
	if document.Content()[0].Kind() != ContentParagraph {
		t.Fatal("mutating returned content changed the document")
	}
	section := document.Content()[1]
	children := section.Children()
	children[0] = Content{}
	if section.Children()[0].Kind() != ContentTable {
		t.Fatal("mutating returned children changed the section")
	}
	table := section.Children()[0]
	rows := table.Rows()
	if len(rows) > 0 {
		rows[0] = queryexec.Row{}
		if len(table.Rows()) == 0 || table.Rows()[0].Cells() == nil {
			t.Fatal("mutating returned rows changed the table")
		}
	}
	list := section.Children()[1].Children()[1]
	items := list.Items()
	if len(items) > 0 {
		items[0] = ListItem{}
		if len(list.Items()[0].Runs()) == 0 {
			t.Fatal("mutating returned items changed the list")
		}
	}
}

func TestEvaluateRequiresPlanAndContext(t *testing.T) {
	_, err := Evaluate(nil, queryexec.Context{}, queryexec.Options{})
	var evaluation *Error
	if !errors.As(err, &evaluation) || evaluation.Kind != ErrorInvalidPlan {
		t.Fatalf("error = %v", err)
	}
	_, err = Evaluate(&docplan.Plan{}, queryexec.Context{}, queryexec.Options{})
	if !errors.As(err, &evaluation) || evaluation.Kind != ErrorInvalidPlan {
		t.Fatalf("error = %v", err)
	}
	fixture := loadEvaluationFixtureFile(t, "testdata/telescope_document.sysml")
	plan := fixture.plan(t, "MassReport")
	_, err = Evaluate(plan, queryexec.Context{}, queryexec.Options{})
	if !errors.As(err, &evaluation) || evaluation.Kind != ErrorInvalidContext {
		t.Fatalf("error = %v", err)
	}
}

func TestEvaluateWrapsQueryExecutionFailure(t *testing.T) {
	fixture := loadEvaluationFixture(t, `
		calc def Related :> Query {
			in root : Element;
			RelatedElements(source = root, relationshipKind = "satisfy", direction = "out", maxDepth = 1)
		}
		part telescope;
		part def Report :> Document {
			attribute redefines title = "Report";
			part table : Table {
				calc rows : Related {
					in root = telescope;
				}
			}
		}
	`)
	_, err := fixture.evaluate(t, "Report")
	var evaluation *Error
	if !errors.As(err, &evaluation) || evaluation.Kind != ErrorQueryExecution {
		t.Fatalf("error = %v", err)
	}
	if evaluation.Query != "Observatory::Related" || !evaluation.Origin.Located() {
		t.Fatalf("error = %+v", evaluation)
	}
	var execution *queryexec.Error
	if !errors.As(err, &execution) || execution.Kind != queryexec.ErrorUnknownRelationship {
		t.Fatalf("inner = %v", evaluation.Err)
	}
}

func TestEvaluateHonorsExecutionBudget(t *testing.T) {
	fixture := loadEvaluationFixtureFile(t, "testdata/telescope_document.sysml")
	plan := fixture.plan(t, "MassReport")
	_, err := Evaluate(plan, fixture.context(), queryexec.Options{VisitBudget: 1})
	var evaluation *Error
	if !errors.As(err, &evaluation) || evaluation.Kind != ErrorQueryExecution {
		t.Fatalf("error = %v", err)
	}
	var execution *queryexec.Error
	if !errors.As(err, &execution) || execution.Kind != queryexec.ErrorVisitBudget {
		t.Fatalf("inner = %v", evaluation.Err)
	}
}
