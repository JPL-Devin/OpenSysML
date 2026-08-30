package queryexec

import (
	"errors"
	"os"
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/queryplan"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

const executionImports = `
private import DocumentQueries::*;
private import KerML::Root::Element;
private import ScalarValues::*;
`

type executionFixture struct {
	index    *symbols.Index
	model    *semantics.Model
	resolver *resolve.Resolver
}

func loadExecutionFixture(t *testing.T, body string) executionFixture {
	t.Helper()
	return loadExecutionSource(t, "package Observatory {"+executionImports+body+"}")
}

func loadExecutionFixtureFile(t *testing.T, path string) executionFixture {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return loadExecutionSource(t, string(content))
}

func loadExecutionSource(t *testing.T, content string) executionFixture {
	t.Helper()
	index := symbols.NewIndex()
	if err := libs.NewLoader(libs.DefaultSource(), nil).LoadAll(index); err != nil {
		t.Fatalf("load standard library: %v", err)
	}
	name := "query-execution.sysml"
	p := parser.New(source.New(name, []byte(content)))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse fixture: %v", p.Diagnostics)
	}
	index.AddDocument(name, root)
	index.ExpandWildcardImports()
	resolver := resolve.New(index)
	return executionFixture{
		index:    index,
		model:    semantics.NewModel(resolver),
		resolver: resolver,
	}
}

func (f executionFixture) symbol(t *testing.T, name string) *symbols.Symbol {
	t.Helper()
	matches := symbols.PreferDeclared(f.index.LookupQualified("Observatory::" + name))
	if len(matches) != 1 {
		t.Fatalf("lookup %s: got %d symbols", name, len(matches))
	}
	return matches[0]
}

func (f executionFixture) program(t *testing.T, name string) *queryplan.Program {
	t.Helper()
	program, err := queryplan.Compile(f.index, f.model, f.resolver, f.symbol(t, name))
	if err != nil {
		t.Fatalf("compile %s: %v", name, err)
	}
	return program
}

func (f executionFixture) execute(
	t *testing.T,
	queryName string,
	bindings Bindings,
	options Options,
) (*RowSet, error) {
	t.Helper()
	return Execute(
		f.program(t, queryName),
		Context{Index: f.index, Resolver: f.resolver, Model: f.model},
		bindings,
		options,
	)
}

func TestExecuteTraversalFilteringOrderingAndProjection(t *testing.T) {
	fixture := loadExecutionFixtureFile(t, "testdata/tmt_collection.sysml")
	result, err := fixture.execute(t, "HeavySubsystems", Bindings{
		"root": {ElementValue(fixture.symbol(t, "telescope"))},
	}, Options{})
	if err != nil {
		t.Fatalf("execute query: %v", err)
	}
	columns := result.Columns()
	var names []string
	for _, column := range columns {
		names = append(names, column.Name())
	}
	if !slices.Equal(names, []string{"name", "qualifiedName", "mass"}) {
		t.Fatalf("columns = %v", names)
	}
	rows := result.Rows()
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	var rowNames []string
	var masses []float64
	for _, row := range rows {
		cells := row.Cells()
		name, ok := cells[0].Values()[0].String()
		if !ok {
			t.Fatalf("name cell = %+v", cells[0].Values())
		}
		mass, ok := cells[2].Values()[0].Real()
		if !ok {
			t.Fatalf("mass cell = %+v", cells[2].Values())
		}
		rowNames = append(rowNames, name)
		masses = append(masses, mass)
		if !row.Origin().Located() || !cells[0].Origin().Located() {
			t.Fatal("rows and cells must preserve model provenance")
		}
	}
	if !slices.Equal(rowNames, []string{"mount", "segmentControl"}) {
		t.Fatalf("row names = %v", rowNames)
	}
	if !slices.Equal(masses, []float64{15, 20}) {
		t.Fatalf("masses = %v", masses)
	}
	if !result.Origin().Located() {
		t.Fatal("row set must preserve entry-query provenance")
	}
}

func TestExecuteDescendantsBreadthFirstAndBounded(t *testing.T) {
	fixture := loadExecutionFixture(t, `
part root {
	part left {
		part leftLeaf;
	}
	part right {
		part rightLeaf;
	}
}
calc def DescendantQuery :> Query {
	in source : Element;
	Descendants(source = source, maxDepth = 2)
}
`)
	result, err := fixture.execute(t, "DescendantQuery", Bindings{
		"source": {ElementValue(fixture.symbol(t, "root"))},
	}, Options{})
	if err != nil {
		t.Fatalf("execute descendants: %v", err)
	}
	var names []string
	for _, row := range result.Rows() {
		sym, _ := row.Element().Element()
		names = append(names, sym.Name)
	}
	if !slices.Equal(names, []string{"left", "right", "leftLeaf", "rightLeaf"}) {
		t.Fatalf("breadth-first descendants = %v", names)
	}

	_, err = fixture.execute(t, "DescendantQuery", Bindings{
		"source": {ElementValue(fixture.symbol(t, "root"))},
	}, Options{VisitBudget: 2})
	var executionError *Error
	if !errors.As(err, &executionError) || executionError.Kind != ErrorVisitBudget {
		t.Fatalf("budget error = %v", err)
	}
}

func TestExecuteAncestorsBreadthFirstAndDeduplicated(t *testing.T) {
	fixture := loadExecutionFixture(t, `
part root {
	part branch {
		part childOne;
		part childTwo;
	}
}
calc def AncestorQuery :> Query {
	in source : Element[1..*] ordered;
	Ancestors(source = source, maxDepth = 2)
}
`)
	result, err := fixture.execute(t, "AncestorQuery", Bindings{
		"source": {
			ElementValue(fixture.symbol(t, "root::branch::childOne")),
			ElementValue(fixture.symbol(t, "root::branch::childTwo")),
		},
	}, Options{})
	if err != nil {
		t.Fatalf("execute ancestors: %v", err)
	}
	var names []string
	for _, row := range result.Rows() {
		sym, _ := row.Element().Element()
		names = append(names, sym.Name)
	}
	if !slices.Equal(names, []string{"branch", "root"}) {
		t.Fatalf("ancestors = %v", names)
	}
}

func TestExecuteMetadataAndNameFilters(t *testing.T) {
	fixture := loadExecutionFixture(t, `
metadata def Critical;
metadata def MissionCritical :> Critical;
part root {
	part primaryMirror { @MissionCritical; }
	part secondaryMirror;
}
calc def CriticalMirrors :> Query {
	in source : Element;
	WhereName(
		source = WhereMetadata(
			source = OwnedElements(source = source),
			'metadata' = "Critical"
		),
		operator = "endsWith",
		value = "Mirror"
	)
}
`)
	result, err := fixture.execute(t, "CriticalMirrors", Bindings{
		"source": {ElementValue(fixture.symbol(t, "root"))},
	}, Options{})
	if err != nil {
		t.Fatalf("execute metadata query: %v", err)
	}
	rows := result.Rows()
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	sym, _ := rows[0].Element().Element()
	if sym.Name != "primaryMirror" {
		t.Fatalf("selected %s", sym.Name)
	}
}

func TestExecuteOrderPoliciesAndProjectedCellAlignment(t *testing.T) {
	fixture := loadExecutionFixture(t, `
part root {
	part alpha {
		attribute tags : String[1..*] ordered = ("z", "a");
	}
	part beta {
		attribute tags : String[1..*] ordered = ("b", "y");
	}
	part missing;
}
calc def SortFirst :> Query {
	in source : Element;
	OrderBy(
		source = Project(
			source = OwnedElements(source = source),
			properties = ("name")
		),
		property = "tags",
		direction = "ascending",
		missing = "last",
		multiple = "first"
	)
}
calc def SortLast :> Query {
	in source : Element;
	OrderBy(
		source = OwnedElements(source = source),
		property = "tags",
		direction = "ascending",
		missing = "last",
		multiple = "last"
	)
}
calc def RejectMultiple :> Query {
	in source : Element;
	OrderBy(
		source = OwnedElements(source = source),
		property = "tags",
		direction = "ascending",
		missing = "last",
		multiple = "error"
	)
}
`)
	bindings := Bindings{"source": {ElementValue(fixture.symbol(t, "root"))}}
	first, err := fixture.execute(t, "SortFirst", bindings, Options{})
	if err != nil {
		t.Fatalf("sort by first value: %v", err)
	}
	if got := projectedNames(t, first); !slices.Equal(got, []string{"beta", "alpha", "missing"}) {
		t.Fatalf("first-value order = %v", got)
	}
	last, err := fixture.execute(t, "SortLast", bindings, Options{})
	if err != nil {
		t.Fatalf("sort by last value: %v", err)
	}
	if got := elementNames(last); !slices.Equal(got, []string{"alpha", "beta", "missing"}) {
		t.Fatalf("last-value order = %v", got)
	}
	_, err = fixture.execute(t, "RejectMultiple", bindings, Options{})
	var executionError *Error
	if !errors.As(err, &executionError) || executionError.Kind != ErrorUnevaluableFeature {
		t.Fatalf("multiple-value error = %v", err)
	}

	rows := first.Rows()
	rows[0] = Row{}
	if got := projectedNames(t, first); !slices.Equal(got, []string{"beta", "alpha", "missing"}) {
		t.Fatalf("mutating returned rows changed row set: %v", got)
	}
}

func TestExecuteReportsUnknownAndUnevaluableFeatures(t *testing.T) {
	fixture := loadExecutionFixture(t, `
part root {
	part child {
		attribute computed = root.child;
	}
}
calc def UnknownFeature :> Query {
	in source : Element;
	WhereFeature(
		source = OwnedElements(source = source),
		'feature' = "absent",
		operator = "=",
		value = "anything"
	)
}
calc def UnevaluableFeature :> Query {
	in source : Element;
	WhereFeature(
		source = OwnedElements(source = source),
		'feature' = "computed",
		operator = "=",
		value = "anything"
	)
}
`)
	bindings := Bindings{"source": {ElementValue(fixture.symbol(t, "root"))}}
	_, err := fixture.execute(t, "UnknownFeature", bindings, Options{})
	var executionError *Error
	if !errors.As(err, &executionError) || executionError.Kind != ErrorUnknownProperty {
		t.Fatalf("unknown property error = %v", err)
	}
	if !executionError.Origin.Located() {
		t.Fatal("unknown property error must retain query provenance")
	}
	_, err = fixture.execute(t, "UnevaluableFeature", bindings, Options{})
	if !errors.As(err, &executionError) || executionError.Kind != ErrorUnevaluableFeature {
		t.Fatalf("unevaluable feature error = %v", err)
	}
}

func TestExecuteRejectsInvalidBindingsAndComposition(t *testing.T) {
	fixture := loadExecutionFixture(t, `
part root;
calc def Children :> Query {
	in source : Element;
	OwnedElements(source = source)
}
calc def Composed :> Query {
	in source : Element;
	Children(source = source)
}
calc def Defaulted :> Query {
	in source : Element = root;
	OwnedElements(source = source)
}
calc def Relationships :> Query {
	in source : Element;
	RelatedElements(
		source = source,
		relationshipKind = "specialization",
		direction = "outgoing",
		maxDepth = 1
	)
}
`)
	_, err := fixture.execute(t, "Children", Bindings{}, Options{})
	var executionError *Error
	if !errors.As(err, &executionError) || executionError.Kind != ErrorMissingBinding {
		t.Fatalf("missing binding error = %v", err)
	}
	_, err = fixture.execute(t, "Children", Bindings{
		"source": {StringValue("root")},
	}, Options{})
	if !errors.As(err, &executionError) || executionError.Kind != ErrorBindingType {
		t.Fatalf("binding type error = %v", err)
	}
	_, err = fixture.execute(t, "Children", Bindings{
		"source": {
			ElementValue(fixture.symbol(t, "root")),
			ElementValue(fixture.symbol(t, "root")),
		},
	}, Options{})
	if !errors.As(err, &executionError) || executionError.Kind != ErrorBindingMultiplicity {
		t.Fatalf("binding multiplicity error = %v", err)
	}
	_, err = fixture.execute(t, "Children", Bindings{
		"source": {ElementValue(fixture.symbol(t, "root"))},
		"extra":  {ElementValue(fixture.symbol(t, "root"))},
	}, Options{})
	if !errors.As(err, &executionError) || executionError.Kind != ErrorUnknownBinding {
		t.Fatalf("unknown binding error = %v", err)
	}
	_, err = fixture.execute(t, "Defaulted", Bindings{}, Options{})
	if !errors.As(err, &executionError) || executionError.Kind != ErrorDefaultUnavailable {
		t.Fatalf("default binding error = %v", err)
	}
	_, err = fixture.execute(t, "Composed", Bindings{
		"source": {ElementValue(fixture.symbol(t, "root"))},
	}, Options{})
	if !errors.As(err, &executionError) || executionError.Kind != ErrorUnsupportedOperation {
		t.Fatalf("composition error = %v", err)
	}
	_, err = fixture.execute(t, "Relationships", Bindings{
		"source": {ElementValue(fixture.symbol(t, "root"))},
	}, Options{})
	if !errors.As(err, &executionError) || executionError.Kind != ErrorUnsupportedOperation {
		t.Fatalf("relationship traversal error = %v", err)
	}
}

func elementNames(result *RowSet) []string {
	var names []string
	for _, row := range result.Rows() {
		sym, _ := row.Element().Element()
		names = append(names, sym.Name)
	}
	return names
}

func projectedNames(t *testing.T, result *RowSet) []string {
	t.Helper()
	var names []string
	for _, row := range result.Rows() {
		cells := row.Cells()
		if len(cells) != 1 || len(cells[0].Values()) != 1 {
			t.Fatalf("projected cells = %+v", cells)
		}
		name, ok := cells[0].Values()[0].String()
		if !ok {
			t.Fatalf("projected name = %+v", cells[0].Values()[0])
		}
		names = append(names, name)
	}
	return names
}
