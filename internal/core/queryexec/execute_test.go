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
	index := libs.NewModelIndex()
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

func TestExecuteTraversalBudgetsCountSemanticElements(t *testing.T) {
	fixture := loadExecutionFixture(t, `
part root {
	part <c> child {
		part grandchild;
		part sibling;
	}
}
calc def Owned :> Query {
	in source : Element;
	OwnedElements(source = source)
}
calc def Recursive :> Query {
	in source : Element;
	Descendants(source = source, maxDepth = 2)
}
calc def Parents :> Query {
	in source : Element[1..*] ordered;
	Ancestors(source = source, maxDepth = 2)
}
`)
	for _, tc := range []struct {
		query  string
		source []Value
		budget int
		want   []string
	}{
		{
			query:  "Owned",
			source: []Value{ElementValue(fixture.symbol(t, "root"))},
			budget: 1,
			want:   []string{"child"},
		},
		{
			query:  "Recursive",
			source: []Value{ElementValue(fixture.symbol(t, "root"))},
			budget: 3,
			want:   []string{"child", "grandchild", "sibling"},
		},
		{
			query: "Parents",
			source: []Value{
				ElementValue(fixture.symbol(t, "root::child::grandchild")),
				ElementValue(fixture.symbol(t, "root::child::sibling")),
			},
			budget: 2,
			want:   []string{"child", "root"},
		},
	} {
		rows, err := fixture.execute(t, tc.query, Bindings{
			"source": tc.source,
		}, Options{VisitBudget: tc.budget})
		if err != nil {
			t.Fatalf("execute %s with exact budget: %v", tc.query, err)
		}
		if got := elementNames(rows); !slices.Equal(got, tc.want) {
			t.Fatalf("%s rows = %v, want %v", tc.query, got, tc.want)
		}
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

func TestExecuteComparesAndOrdersInfiniteUpperBounds(t *testing.T) {
	fixture := loadExecutionFixture(t, `
part root {
	part single;
	part many[0..*];
}
calc def InfiniteOnly :> Query {
	in source : Element;
	OrderBy(
		source = WhereFeature(
			source = OwnedElements(source = source),
			'feature' = "multiplicityUpper",
			operator = ">",
			value = "1"
		),
		property = "multiplicityUpper",
		direction = "ascending",
		missing = "error",
		multiple = "error"
	)
}
`)
	result, err := fixture.execute(t, "InfiniteOnly", Bindings{
		"source": {ElementValue(fixture.symbol(t, "root"))},
	}, Options{})
	if err != nil {
		t.Fatalf("execute infinite-bound query: %v", err)
	}
	if got := elementNames(result); !slices.Equal(got, []string{"many"}) {
		t.Fatalf("infinite-bound result = %v", got)
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

func TestExecuteEmptyFeaturePipelinesReturnEmptyResults(t *testing.T) {
	fixture := loadExecutionFixture(t, `
part root {
	part child {
		attribute score : Integer = 1;
	}
}
calc def EmptyFeatureFilter :> Query {
	in source : Element;
	WhereFeature(
		source = Project(
			source = WhereName(
				source = OwnedElements(source = source),
				operator = "=",
				value = "absent"
			),
			properties = ("name")
		),
		'feature' = "score",
		operator = ">",
		value = "0"
	)
}
calc def EmptyOrdering :> Query {
	in source : Element;
	OrderBy(
		source = Project(
			source = WhereName(
				source = OwnedElements(source = source),
				operator = "=",
				value = "absent"
			),
			properties = ("name")
		),
		property = "score",
		direction = "ascending",
		missing = "error",
		multiple = "error"
	)
}
`)
	bindings := Bindings{"source": {ElementValue(fixture.symbol(t, "root"))}}
	for _, query := range []string{"EmptyFeatureFilter", "EmptyOrdering"} {
		rows, err := fixture.execute(t, query, bindings, Options{})
		if err != nil {
			t.Fatalf("execute %s: %v", query, err)
		}
		if len(rows.Rows()) != 0 {
			t.Fatalf("%s rows = %d, want 0", query, len(rows.Rows()))
		}
		columns := rows.Columns()
		if len(columns) != 1 || columns[0].Name() != "name" {
			t.Fatalf("%s columns = %+v", query, columns)
		}
	}
}

func TestExecuteValidatesEmptyFilters(t *testing.T) {
	fixture := loadExecutionFixture(t, `
part root {
	part child;
}
calc def InvalidNameOperator :> Query {
	in source : Element;
	WhereName(
		source = WhereName(
			source = OwnedElements(source = source),
			operator = "=",
			value = "absent"
		),
		operator = "invalid",
		value = "child"
	)
}
calc def InvalidNamePattern :> Query {
	in source : Element;
	WhereName(
		source = WhereName(
			source = OwnedElements(source = source),
			operator = "=",
			value = "absent"
		),
		operator = "matches",
		value = "["
	)
}
calc def InvalidFeatureOperator :> Query {
	in source : Element;
	WhereFeature(
		source = WhereName(
			source = OwnedElements(source = source),
			operator = "=",
			value = "absent"
		),
		'feature' = "score",
		operator = "invalid",
		value = "1"
	)
}
calc def InvalidFeatureOperand :> Query {
	in source : Element;
	WhereFeature(
		source = WhereName(
			source = OwnedElements(source = source),
			operator = "=",
			value = "absent"
		),
		'feature' = "score",
		operator = ">",
		value = "not-a-number"
	)
}
`)
	bindings := Bindings{"source": {ElementValue(fixture.symbol(t, "root"))}}
	for _, tc := range []struct {
		query string
		kind  ErrorKind
	}{
		{query: "InvalidNameOperator", kind: ErrorInvalidOperator},
		{query: "InvalidNamePattern", kind: ErrorInvalidArgument},
		{query: "InvalidFeatureOperator", kind: ErrorInvalidOperator},
		{query: "InvalidFeatureOperand", kind: ErrorInvalidArgument},
	} {
		_, err := fixture.execute(t, tc.query, bindings, Options{})
		var executionError *Error
		if !errors.As(err, &executionError) || executionError.Kind != tc.kind {
			t.Fatalf("%s error = %v, want %s", tc.query, err, tc.kind)
		}
	}
}

func TestExecuteRejectsInvalidBindingsAndRelationshipTraversal(t *testing.T) {
	fixture := loadExecutionFixture(t, `
part root;
calc def Children :> Query {
	in source : Element;
	OwnedElements(source = source)
}
calc def Defaulted :> Query {
	in source : Element = root;
	OwnedElements(source = source)
}
calc def UnknownKind :> Query {
	in source : Element;
	RelatedElements(
		source = source,
		relationshipKind = "refinement",
		direction = "outgoing",
		maxDepth = 1
	)
}
calc def BadDirection :> Query {
	in source : Element;
	RelatedElements(
		source = source,
		relationshipKind = "specialization",
		direction = "sideways",
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
	_, err = fixture.execute(t, "UnknownKind", Bindings{
		"source": {ElementValue(fixture.symbol(t, "root"))},
	}, Options{})
	if !errors.As(err, &executionError) || executionError.Kind != ErrorUnknownRelationship {
		t.Fatalf("unknown relationship kind error = %v", err)
	}
	_, err = fixture.execute(t, "BadDirection", Bindings{
		"source": {ElementValue(fixture.symbol(t, "root"))},
	}, Options{})
	if !errors.As(err, &executionError) || executionError.Kind != ErrorInvalidOperator {
		t.Fatalf("invalid direction error = %v", err)
	}
}

func TestExecuteInvokesNamedQueriesPreservingOrderAndIdentity(t *testing.T) {
	fixture := loadExecutionFixture(t, `
part def Base {
	part engine;
	part motor;
}
part root : Base {
	part namedOne;
	part :>> engine :>> motor;
	part namedTwo;
}
calc def Children :> Query {
	in source : Element;
	OwnedElements(source = source)
}
calc def Composed :> Query {
	in source : Element;
	Children(source = source)
}
`)
	bindings := Bindings{"source": {ElementValue(fixture.symbol(t, "root"))}}
	want := []string{"namedOne", "", "namedTwo"}
	direct, err := fixture.execute(t, "Children", bindings, Options{})
	if err != nil {
		t.Fatalf("execute direct query: %v", err)
	}
	composed, err := fixture.execute(t, "Composed", bindings, Options{})
	if err != nil {
		t.Fatalf("execute composed query: %v", err)
	}
	if got := elementNames(composed); !slices.Equal(got, want) {
		t.Fatalf("composed rows = %v, want %v", got, want)
	}
	if !slices.Equal(elementNames(composed), elementNames(direct)) {
		t.Fatalf("composed rows = %v, direct rows = %v", elementNames(composed), elementNames(direct))
	}
	for _, row := range composed.Rows() {
		if !row.Origin().Located() {
			t.Fatal("invoked rows must preserve model provenance")
		}
	}
	if !composed.Origin().Located() {
		t.Fatal("composed row set must preserve entry-query provenance")
	}
}

func TestExecuteNestedInvocationPreservesProjectedColumnsAndCells(t *testing.T) {
	fixture := loadExecutionFixtureFile(t, "testdata/tmt_collection.sysml")
	direct, err := fixture.execute(t, "HeavySubsystems", Bindings{
		"root": {ElementValue(fixture.symbol(t, "telescope"))},
	}, Options{})
	if err != nil {
		t.Fatalf("execute direct query: %v", err)
	}
	nested := loadExecutionFixture(t, `
part def Subsystem {
	attribute mass : Real;
}
part telescope {
	part optics : Subsystem {
		attribute redefines mass = 8.5;
	}
	part segmentControl : Subsystem {
		attribute redefines mass = 20.0;
	}
	part mount : Subsystem {
		attribute redefines mass = 15.0;
	}
}
calc def HeavySubsystems :> Query {
	in root : Element;
	Project(
		source = OrderBy(
			source = WhereFeature(
				source = WhereType(
					source = Descendants(source = root, maxDepth = 3),
					type = "PartUsage"
				),
				'feature' = "mass",
				operator = ">=",
				value = "10"
			),
			property = "name",
			direction = "ascending",
			missing = "last",
			multiple = "error"
		),
		properties = ("name", "qualifiedName", "mass")
	)
}
calc def Middle :> Query {
	in root : Element;
	HeavySubsystems(root = root)
}
calc def Outer :> Query {
	in root : Element;
	Middle(root = root)
}
`)
	result, err := nested.execute(t, "Outer", Bindings{
		"root": {ElementValue(nested.symbol(t, "telescope"))},
	}, Options{})
	if err != nil {
		t.Fatalf("execute nested invocation: %v", err)
	}
	var columns []string
	for _, column := range result.Columns() {
		columns = append(columns, column.Name())
	}
	if !slices.Equal(columns, []string{"name", "qualifiedName", "mass"}) {
		t.Fatalf("nested columns = %v", columns)
	}
	rows := result.Rows()
	if len(rows) != len(direct.Rows()) {
		t.Fatalf("nested rows = %d, want %d", len(rows), len(direct.Rows()))
	}
	var names []string
	for _, row := range rows {
		cells := row.Cells()
		if len(cells) != 3 {
			t.Fatalf("nested cells = %+v", cells)
		}
		name, ok := cells[0].Values()[0].String()
		if !ok {
			t.Fatalf("nested name cell = %+v", cells[0].Values())
		}
		names = append(names, name)
		if !row.Origin().Located() || !cells[0].Origin().Located() {
			t.Fatal("nested rows and cells must preserve model provenance")
		}
	}
	if !slices.Equal(names, []string{"mount", "segmentControl"}) {
		t.Fatalf("nested row names = %v", names)
	}
}

func TestExecuteInvocationPropagatesEmptyResults(t *testing.T) {
	fixture := loadExecutionFixture(t, `
part root {
	part child;
}
calc def NamedProjection :> Query {
	in source : Element;
	Project(
		source = WhereName(
			source = OwnedElements(source = source),
			operator = "=",
			value = "absent"
		),
		properties = ("name")
	)
}
calc def Wrapped :> Query {
	in source : Element;
	NamedProjection(source = source)
}
`)
	result, err := fixture.execute(t, "Wrapped", Bindings{
		"source": {ElementValue(fixture.symbol(t, "root"))},
	}, Options{})
	if err != nil {
		t.Fatalf("execute empty invocation: %v", err)
	}
	if len(result.Rows()) != 0 {
		t.Fatalf("rows = %d, want 0", len(result.Rows()))
	}
	columns := result.Columns()
	if len(columns) != 1 || columns[0].Name() != "name" {
		t.Fatalf("columns = %+v", columns)
	}
}

func TestExecuteInvocationBindsProjectedArgumentRowElements(t *testing.T) {
	fixture := loadExecutionFixture(t, `
part root {
	part alpha;
	part beta;
}
calc def ProjectedChildren :> Query {
	in source : Element;
	Project(
		source = OwnedElements(source = source),
		properties = ("name")
	)
}
calc def Named :> Query {
	in items : Element[0..*];
	WhereName(source = items, operator = "!=", value = "absent")
}
calc def Composed :> Query {
	in source : Element;
	Named(items = ProjectedChildren(source = source))
}
`)
	result, err := fixture.execute(t, "Composed", Bindings{
		"source": {ElementValue(fixture.symbol(t, "root"))},
	}, Options{})
	if err != nil {
		t.Fatalf("execute projected-argument invocation: %v", err)
	}
	if got := elementNames(result); !slices.Equal(got, []string{"alpha", "beta"}) {
		t.Fatalf("rows = %v", got)
	}
	if len(result.Columns()) != 0 {
		t.Fatalf("columns = %+v, want none across the binding", result.Columns())
	}
	for _, row := range result.Rows() {
		if !row.Origin().Located() {
			t.Fatal("rows bound through a projected argument must preserve provenance")
		}
	}
}

func TestExecuteValidatesInvokedResultsAtTheirDeclaration(t *testing.T) {
	fixture := loadExecutionFixture(t, `
part def Target;
part def Other;
part invalidRoot {
	part target : Target;
	part other : Other;
}
part emptyRoot;
calc def TargetResults :> Query {
	in source : Element;
	return result : Target[0..*] ordered;
	OwnedElements(source = source)
}
calc def RequiredResults :> Query {
	in source : Element;
	return result : Target[1..*] ordered;
	OwnedElements(source = source)
}
calc def InvokesTargets :> Query {
	in source : Element;
	TargetResults(source = source)
}
calc def InvokesRequired :> Query {
	in source : Element;
	return result : Target[0..*] ordered;
	RequiredResults(source = source)
}
`)
	_, err := fixture.execute(t, "InvokesTargets", Bindings{
		"source": {ElementValue(fixture.symbol(t, "invalidRoot"))},
	}, Options{})
	var executionError *Error
	if !errors.As(err, &executionError) || executionError.Kind != ErrorResultType {
		t.Fatalf("invoked result type error = %v", err)
	}
	if executionError.Query != "Observatory::TargetResults" {
		t.Fatalf("result type error query = %s", executionError.Query)
	}
	_, err = fixture.execute(t, "InvokesRequired", Bindings{
		"source": {ElementValue(fixture.symbol(t, "emptyRoot"))},
	}, Options{})
	if !errors.As(err, &executionError) || executionError.Kind != ErrorResultMultiplicity {
		t.Fatalf("invoked result multiplicity error = %v", err)
	}
	if executionError.Query != "Observatory::RequiredResults" {
		t.Fatalf("result multiplicity error query = %s", executionError.Query)
	}
}

func TestExecuteInvocationBindingMismatchFailsAtPlanning(t *testing.T) {
	fixture := loadExecutionFixture(t, `
calc def Children :> Query {
	in source : Element;
	OwnedElements(source = source)
}
calc def Mismatched :> Query {
	in source : Element;
	Children(source = "root")
}
`)
	_, err := queryplan.Compile(fixture.index, fixture.model, fixture.resolver, fixture.symbol(t, "Mismatched"))
	var planningError *queryplan.Error
	if !errors.As(err, &planningError) || planningError.Kind != queryplan.ErrorArgumentType {
		t.Fatalf("planning binding mismatch = %v", err)
	}
}

func TestExecuteInvocationDepthIsBounded(t *testing.T) {
	fixture := loadExecutionFixture(t, `
part root;
calc def Level4 :> Query {
	in source : Element;
	OwnedElements(source = source)
}
calc def Level3 :> Query {
	in source : Element;
	Level4(source = source)
}
calc def Level2 :> Query {
	in source : Element;
	Level3(source = source)
}
calc def Level1 :> Query {
	in source : Element;
	Level2(source = source)
}
`)
	bindings := Bindings{"source": {ElementValue(fixture.symbol(t, "root"))}}
	if _, err := fixture.execute(t, "Level1", bindings, Options{}); err != nil {
		t.Fatalf("execute within default depth: %v", err)
	}
	if _, err := fixture.execute(t, "Level1", bindings, Options{InvocationDepth: 3}); err != nil {
		t.Fatalf("execute at exact depth: %v", err)
	}
	_, err := fixture.execute(t, "Level1", bindings, Options{InvocationDepth: 2})
	var executionError *Error
	if !errors.As(err, &executionError) || executionError.Kind != ErrorInvocationDepth {
		t.Fatalf("depth error = %v", err)
	}
	if executionError.Query != "Observatory::Level3" || executionError.Target != "Observatory::Level4" {
		t.Fatalf("depth error detail = %+v", executionError)
	}
	if !executionError.Origin.Located() {
		t.Fatal("depth error must retain query provenance")
	}
	_, err = fixture.execute(t, "Level1", bindings, Options{InvocationDepth: -1})
	if !errors.As(err, &executionError) || executionError.Kind != ErrorInvalidContext {
		t.Fatalf("negative depth error = %v", err)
	}
}

func TestExecuteInvocationSharesTheVisitBudget(t *testing.T) {
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
calc def Composed :> Query {
	in source : Element;
	DescendantQuery(source = source)
}
`)
	bindings := Bindings{"source": {ElementValue(fixture.symbol(t, "root"))}}
	if _, err := fixture.execute(t, "Composed", bindings, Options{VisitBudget: 4}); err != nil {
		t.Fatalf("execute with exact shared budget: %v", err)
	}
	_, err := fixture.execute(t, "Composed", bindings, Options{VisitBudget: 3})
	var executionError *Error
	if !errors.As(err, &executionError) || executionError.Kind != ErrorVisitBudget {
		t.Fatalf("shared budget error = %v", err)
	}
	if executionError.Query != "Observatory::DescendantQuery" {
		t.Fatalf("shared budget error query = %s", executionError.Query)
	}
}

func TestExecuteInvocationCountIsBounded(t *testing.T) {
	fixture := loadExecutionFixture(t, `
part root {
	part child;
}
calc def Leaf :> Query {
	in source : Element;
	OwnedElements(source = source)
}
calc def Fan1 :> Query {
	in source : Element;
	(Leaf(source = source), Leaf(source = source))
}
calc def Fan2 :> Query {
	in source : Element;
	(Fan1(source = source), Fan1(source = source))
}
`)
	bindings := Bindings{"source": {ElementValue(fixture.symbol(t, "root"))}}
	if _, err := fixture.execute(t, "Fan2", bindings, Options{}); err != nil {
		t.Fatalf("execute within default invocation budget: %v", err)
	}
	if _, err := fixture.execute(t, "Fan2", bindings, Options{InvocationBudget: 6}); err != nil {
		t.Fatalf("execute at exact invocation budget: %v", err)
	}
	_, err := fixture.execute(t, "Fan2", bindings, Options{InvocationBudget: 5})
	var executionError *Error
	if !errors.As(err, &executionError) || executionError.Kind != ErrorInvocationBudget {
		t.Fatalf("invocation budget error = %v", err)
	}
	if executionError.Query != "Observatory::Fan1" || executionError.Target != "Observatory::Leaf" {
		t.Fatalf("invocation budget error detail = %+v", executionError)
	}
	if !executionError.Origin.Located() {
		t.Fatal("invocation budget error must retain query provenance")
	}
	_, err = fixture.execute(t, "Fan2", bindings, Options{InvocationBudget: -1})
	if !errors.As(err, &executionError) || executionError.Kind != ErrorInvalidContext {
		t.Fatalf("negative invocation budget error = %v", err)
	}
}

func TestExecuteUsesSemanticScalarBindingConformance(t *testing.T) {
	fixture := loadExecutionFixture(t, `
part root;
calc def WidenedScalars :> Query {
	in source : Element;
	in rational : Rational;
	in complexFromInteger : Complex;
	in complexFromReal : Complex;
	in numberFromInteger : Number;
	in numberFromReal : Number;
	OwnedElements(source = source)
}
calc def RationalInput :> Query {
	in source : Element;
	in value : Rational;
	OwnedElements(source = source)
}
calc def NaturalInput :> Query {
	in source : Element;
	in value : Natural;
	OwnedElements(source = source)
}
`)
	_, err := fixture.execute(t, "WidenedScalars", Bindings{
		"source":             {ElementValue(fixture.symbol(t, "root"))},
		"rational":           {IntegerValue(1)},
		"complexFromInteger": {IntegerValue(2)},
		"complexFromReal":    {RealValue(2.5)},
		"numberFromInteger":  {IntegerValue(3)},
		"numberFromReal":     {RealValue(3.5)},
	}, Options{})
	if err != nil {
		t.Fatalf("execute widened scalar bindings: %v", err)
	}
	for _, test := range []struct {
		query string
		value Value
	}{
		{query: "RationalInput", value: RealValue(1.5)},
		{query: "NaturalInput", value: IntegerValue(1)},
	} {
		_, err = fixture.execute(t, test.query, Bindings{
			"source": {ElementValue(fixture.symbol(t, "root"))},
			"value":  {test.value},
		}, Options{})
		var executionError *Error
		if !errors.As(err, &executionError) || executionError.Kind != ErrorBindingType {
			t.Fatalf("%s narrowing error = %v", test.query, err)
		}
	}
}

func TestExecuteEnforcesNarrowedResultTypes(t *testing.T) {
	fixture := loadExecutionFixture(t, `
part def Target;
part def Other;
part validRoot {
	part target : Target;
}
part invalidRoot {
	part target : Target;
	part other : Other;
}
calc def TargetResults :> Query {
	in source : Element;
	return result : Target[0..*] ordered;
	OwnedElements(source = source)
}
`)
	rows, err := fixture.execute(t, "TargetResults", Bindings{
		"source": {ElementValue(fixture.symbol(t, "validRoot"))},
	}, Options{})
	if err != nil {
		t.Fatalf("execute conforming narrowed result: %v", err)
	}
	if got := elementNames(rows); !slices.Equal(got, []string{"target"}) {
		t.Fatalf("conforming rows = %v", got)
	}
	_, err = fixture.execute(t, "TargetResults", Bindings{
		"source": {ElementValue(fixture.symbol(t, "invalidRoot"))},
	}, Options{})
	var executionError *Error
	if !errors.As(err, &executionError) || executionError.Kind != ErrorResultType {
		t.Fatalf("narrowed result error = %v", err)
	}
	if executionError.Expected != "Observatory::Target" ||
		executionError.Actual != "Observatory::invalidRoot::other" {
		t.Fatalf("narrowed result detail = %+v", executionError)
	}
}

func TestExecuteValidatesUserClassifiersNamedElement(t *testing.T) {
	fixture := loadExecutionFixture(t, `
package Domain {
	part def Element;
}
part conforming : Domain::Element;
part unrelated;
part validRoot {
	part child : Domain::Element;
}
part invalidRoot {
	part child;
}
calc def ElementInput :> Query {
	in source : Domain::Element;
	in root : Element;
	OwnedElements(source = root)
}
calc def ElementResults :> Query {
	in source : Element;
	return result : Domain::Element[0..*] ordered;
	OwnedElements(source = source)
}
`)
	if _, err := fixture.execute(t, "ElementInput", Bindings{
		"source": {ElementValue(fixture.symbol(t, "conforming"))},
		"root":   {ElementValue(fixture.symbol(t, "validRoot"))},
	}, Options{}); err != nil {
		t.Fatalf("execute conforming Element binding: %v", err)
	}
	_, err := fixture.execute(t, "ElementInput", Bindings{
		"source": {ElementValue(fixture.symbol(t, "unrelated"))},
		"root":   {ElementValue(fixture.symbol(t, "validRoot"))},
	}, Options{})
	var executionError *Error
	if !errors.As(err, &executionError) || executionError.Kind != ErrorBindingType {
		t.Fatalf("unrelated Element binding error = %v", err)
	}
	if _, err = fixture.execute(t, "ElementResults", Bindings{
		"source": {ElementValue(fixture.symbol(t, "validRoot"))},
	}, Options{}); err != nil {
		t.Fatalf("execute conforming Element results: %v", err)
	}
	_, err = fixture.execute(t, "ElementResults", Bindings{
		"source": {ElementValue(fixture.symbol(t, "invalidRoot"))},
	}, Options{})
	if !errors.As(err, &executionError) || executionError.Kind != ErrorResultType {
		t.Fatalf("unrelated Element result error = %v", err)
	}
}

func TestExecuteUsesInheritedRedefinedFeatureValues(t *testing.T) {
	fixture := loadExecutionFixture(t, `
part def Base {
	attribute values : Integer[1..*] ordered = (10, 20);
}
part def Derived :> Base {
	attribute redefines values;
}
part root {
	part child : Derived;
}
calc def Matching :> Query {
	in source : Element;
	Project(
		source = WhereFeature(
			source = OwnedElements(source = source),
			'feature' = "values",
			operator = "=",
			value = "20"
		),
		properties = ("name", "values")
	)
}
`)
	rows, err := fixture.execute(t, "Matching", Bindings{
		"source": {ElementValue(fixture.symbol(t, "root"))},
	}, Options{})
	if err != nil {
		t.Fatalf("execute inherited feature query: %v", err)
	}
	if got := elementNames(rows); !slices.Equal(got, []string{"child"}) {
		t.Fatalf("matching rows = %v", got)
	}
	cells := rows.Rows()[0].Cells()
	if len(cells) != 2 || len(cells[1].Values()) != 2 {
		t.Fatalf("inherited projected cells = %+v", cells)
	}
}

func TestExecuteUsesEffectiveNamesForAnonymousRedefinitions(t *testing.T) {
	fixture := loadExecutionFixture(t, `
calc def Base {
	in zeta : Integer;
}
calc def Derived :> Base {
	in : Integer;
	in alpha : Integer;
}
calc def FilterName :> Query {
	in source : Element;
	Project(
		source = WhereName(
			source = OwnedElements(source = source),
			operator = "=",
			value = "zeta"
		),
		properties = ("name")
	)
}
calc def ProjectNames :> Query {
	in source : Element;
	Project(
		source = OwnedElements(source = source),
		properties = ("name", "declaredName")
	)
}
calc def OrderNames :> Query {
	in source : Element;
	Project(
		source = OrderBy(
			source = OwnedElements(source = source),
			property = "name",
			direction = "ascending",
			missing = "error",
			multiple = "error"
		),
		properties = ("name")
	)
}
`)
	bindings := Bindings{"source": {ElementValue(fixture.symbol(t, "Derived"))}}
	filtered, err := fixture.execute(t, "FilterName", bindings, Options{})
	if err != nil {
		t.Fatalf("filter by effective name: %v", err)
	}
	if got := projectedNames(t, filtered); !slices.Equal(got, []string{"zeta"}) {
		t.Fatalf("filtered names = %v", got)
	}

	projected, err := fixture.execute(t, "ProjectNames", bindings, Options{})
	if err != nil {
		t.Fatalf("project effective names: %v", err)
	}
	rows := projected.Rows()
	if len(rows) != 2 {
		t.Fatalf("projected rows = %d, want 2", len(rows))
	}
	first := rows[0].Cells()
	if len(first) != 2 || len(first[0].Values()) != 1 || len(first[1].Values()) != 0 {
		t.Fatalf("anonymous redefinition cells = %+v", first)
	}
	name, ok := first[0].Values()[0].String()
	if !ok || name != "zeta" {
		t.Fatalf("anonymous redefinition name = %q, %v", name, ok)
	}

	ordered, err := fixture.execute(t, "OrderNames", bindings, Options{})
	if err != nil {
		t.Fatalf("order by effective name: %v", err)
	}
	if got := projectedNames(t, ordered); !slices.Equal(got, []string{"alpha", "zeta"}) {
		t.Fatalf("ordered names = %v", got)
	}
}

func TestExecutePreservesInterleavedOwnedDeclarationOrder(t *testing.T) {
	fixture := loadExecutionFixture(t, `
part def Base {
	part engine;
	part motor;
}
part root : Base {
	part namedOne;
	part :>> engine :>> motor;
	part namedTwo;
}
calc def Direct :> Query {
	in source : Element;
	OwnedElements(source = source)
}
calc def Recursive :> Query {
	in source : Element;
	Descendants(source = source, maxDepth = 1)
}
`)
	want := []string{"namedOne", "", "namedTwo"}
	for _, query := range []string{"Direct", "Recursive"} {
		rows, err := fixture.execute(t, query, Bindings{
			"source": {ElementValue(fixture.symbol(t, "root"))},
		}, Options{})
		if err != nil {
			t.Fatalf("execute %s: %v", query, err)
		}
		if got := elementNames(rows); !slices.Equal(got, want) {
			t.Fatalf("%s rows = %v, want %v", query, got, want)
		}
	}
}

func TestExecuteComparesLargeIntegersWithoutLosingPrecision(t *testing.T) {
	fixture := loadExecutionFixture(t, `
part root {
	part lower {
		attribute score : Integer = 9007199254740992;
	}
	part higher {
		attribute score : Integer = 9007199254740993;
	}
}
calc def ExactScore :> Query {
	in source : Element;
	WhereFeature(
		source = OwnedElements(source = source),
		'feature' = "score",
		operator = "=",
		value = "9007199254740993"
	)
}
`)
	rows, err := fixture.execute(t, "ExactScore", Bindings{
		"source": {ElementValue(fixture.symbol(t, "root"))},
	}, Options{})
	if err != nil {
		t.Fatalf("execute large-integer query: %v", err)
	}
	if got := elementNames(rows); !slices.Equal(got, []string{"higher"}) {
		t.Fatalf("matching rows = %v", got)
	}
}

func TestExecuteComparesIntegerFeaturesWithRealOperands(t *testing.T) {
	fixture := loadExecutionFixture(t, `
part root {
	part lower {
		attribute score : Integer = 2;
	}
	part higher {
		attribute score : Integer = 3;
	}
}
calc def EqualReal :> Query {
	in source : Element;
	WhereFeature(
		source = OwnedElements(source = source),
		'feature' = "score",
		operator = "=",
		value = "2.0"
	)
}
calc def LessThanReal :> Query {
	in source : Element;
	WhereFeature(
		source = OwnedElements(source = source),
		'feature' = "score",
		operator = "<",
		value = "2.5"
	)
}
`)
	bindings := Bindings{"source": {ElementValue(fixture.symbol(t, "root"))}}
	for _, tc := range []struct {
		query string
		want  []string
	}{
		{query: "EqualReal", want: []string{"lower"}},
		{query: "LessThanReal", want: []string{"lower"}},
	} {
		rows, err := fixture.execute(t, tc.query, bindings, Options{})
		if err != nil {
			t.Fatalf("execute %s: %v", tc.query, err)
		}
		if got := elementNames(rows); !slices.Equal(got, tc.want) {
			t.Fatalf("%s rows = %v, want %v", tc.query, got, tc.want)
		}
	}
}

func TestExecuteOrdersMixedNumericValuesExactly(t *testing.T) {
	fixture := loadExecutionFixture(t, `
part root {
	part integerKey {
		attribute score : Integer = 9007199254740993;
	}
	part realKey {
		attribute score : Real = 9007199254740992.0;
	}
}
calc def ExactOrder :> Query {
	in source : Element;
	OrderBy(
		source = OwnedElements(source = source),
		property = "score",
		direction = "ascending",
		missing = "error",
		multiple = "error"
	)
}
`)
	rows, err := fixture.execute(t, "ExactOrder", Bindings{
		"source": {ElementValue(fixture.symbol(t, "root"))},
	}, Options{})
	if err != nil {
		t.Fatalf("execute mixed numeric ordering: %v", err)
	}
	if got := elementNames(rows); !slices.Equal(got, []string{"realKey", "integerKey"}) {
		t.Fatalf("ordered rows = %v", got)
	}
}

func TestExecuteWhereTypeUsesMetaclassConformance(t *testing.T) {
	fixture := loadExecutionFixture(t, `
part root {
	part child;
}
calc def Usages :> Query {
	in source : Element;
	WhereType(
		source = OwnedElements(source = source),
		type = "Usage"
	)
}
`)
	rows, err := fixture.execute(t, "Usages", Bindings{
		"source": {ElementValue(fixture.symbol(t, "root"))},
	}, Options{})
	if err != nil {
		t.Fatalf("execute metaclass query: %v", err)
	}
	if got := elementNames(rows); !slices.Equal(got, []string{"child"}) {
		t.Fatalf("usage rows = %v", got)
	}
}

// TestExecuteWhereTypeKnowsEveryMetamodelTypeName checks that a type filter that
// selects nothing distinguishes a real metamodel type from a misspelt one, also
// when a model element shares the metaclass's name so no classifier is unique.
func TestExecuteWhereTypeKnowsEveryMetamodelTypeName(t *testing.T) {
	fixture := loadExecutionFixture(t, `
part root {
	part child;
}
attribute def Multiplicity;
attribute def Class;
calc def Multiplicities :> Query {
	in source : Element;
	WhereType(source = OwnedElements(source = source), type = "Multiplicity")
}
calc def Classes :> Query {
	in source : Element;
	WhereType(source = OwnedElements(source = source), type = "Class")
}
calc def Misspelt :> Query {
	in source : Element;
	WhereType(source = OwnedElements(source = source), type = "Multiplicty")
}
`)
	bindings := Bindings{"source": {ElementValue(fixture.symbol(t, "root"))}}
	for _, name := range []string{"Multiplicities", "Classes"} {
		rows, err := fixture.execute(t, name, bindings, Options{})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(rows.Rows()) != 0 {
			t.Fatalf("%s rows = %v, want none", name, elementNames(rows))
		}
	}
	_, err := fixture.execute(t, "Misspelt", bindings, Options{})
	var executionError *Error
	if !errors.As(err, &executionError) || executionError.Kind != ErrorUnknownClassification {
		t.Fatalf("Misspelt error = %v, want %s", err, ErrorUnknownClassification)
	}
}

// TestExecuteFilterKeepsProjectedColumnsWhenEmpty checks that a filter over a
// projection keeps the projected columns even when it selects no row.
func TestExecuteFilterKeepsProjectedColumnsWhenEmpty(t *testing.T) {
	fixture := loadExecutionFixture(t, `
part root {
	part child;
}
calc def Names :> Query {
	in source : Element;
	WhereName(
		source = Project(
			source = OwnedElements(source = source),
			properties = ("name")
		),
		operator = "==",
		value = "nomatch"
	)
}
`)
	rows, err := fixture.execute(t, "Names", Bindings{
		"source": {ElementValue(fixture.symbol(t, "root"))},
	}, Options{})
	if err != nil {
		t.Fatalf("execute filtered projection: %v", err)
	}
	if len(rows.Rows()) != 0 {
		t.Fatalf("rows = %v", elementNames(rows))
	}
	columns := rows.Columns()
	if len(columns) != 1 || columns[0].Name() != "name" {
		t.Fatalf("columns = %+v", columns)
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
