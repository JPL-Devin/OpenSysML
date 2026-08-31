package queryexec

import (
	"errors"
	"slices"
	"testing"
)

const computedBody = `
part def Subsystem {
	attribute mass : Real;
	attribute alloc : Real;
	attribute count : Integer;
}
part system {
	part a : Subsystem {
		attribute redefines mass = 2.5;
		attribute redefines alloc = 4.0;
		attribute redefines count = 3;
	}
	part b : Subsystem {
		attribute redefines mass = 1.5;
		attribute redefines count = 2;
	}
}
`

func computedFixture(t *testing.T, query string) executionFixture {
	t.Helper()
	return loadExecutionFixture(t, computedBody+query)
}

func computedRows(t *testing.T, fixture executionFixture, query string) *RowSet {
	t.Helper()
	result, err := fixture.execute(t, query, Bindings{
		"root": {ElementValue(fixture.symbol(t, "system"))},
	}, Options{})
	if err != nil {
		t.Fatalf("execute %s: %v", query, err)
	}
	return result
}

func TestExecuteComputedArithmeticAndConcatenation(t *testing.T) {
	fixture := computedFixture(t, `
calc def Margins :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		properties = ("name"),
		columns = (
			Column(name = "margin", expression = (Subsystem::alloc ?? 0.0) - Subsystem::mass),
			Column(name = "double", expression = Subsystem::count * 2),
			Column(name = "label", expression = "sub: " + Element::name)
		)
	)
}
`)
	result := computedRows(t, fixture, "Margins")
	var names []string
	for _, column := range result.Columns() {
		names = append(names, column.Name())
	}
	if !slices.Equal(names, []string{"name", "margin", "double", "label"}) {
		t.Fatalf("columns = %v", names)
	}
	rows := result.Rows()
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	margins := make([]float64, 0, 2)
	doubles := make([]int64, 0, 2)
	labels := make([]string, 0, 2)
	for _, row := range rows {
		cells := row.Cells()
		margin, ok := cells[1].Values()[0].Real()
		if !ok {
			t.Fatalf("margin cell = %+v", cells[1].Values())
		}
		double, ok := cells[2].Values()[0].Integer()
		if !ok {
			t.Fatalf("double cell = %+v", cells[2].Values())
		}
		label, ok := cells[3].Values()[0].String()
		if !ok {
			t.Fatalf("label cell = %+v", cells[3].Values())
		}
		margins = append(margins, margin)
		doubles = append(doubles, double)
		labels = append(labels, label)
		if !cells[1].Origin().Located() {
			t.Fatal("computed cells must retain provenance")
		}
	}
	if !slices.Equal(margins, []float64{1.5, -1.5}) {
		t.Fatalf("margins = %v", margins)
	}
	if !slices.Equal(doubles, []int64{6, 4}) {
		t.Fatalf("doubles = %v", doubles)
	}
	if !slices.Equal(labels, []string{"sub: a", "sub: b"}) {
		t.Fatalf("labels = %v", labels)
	}
}

func TestExecuteOrdersAndGroupsByComputedColumns(t *testing.T) {
	fixture := computedFixture(t, `
calc def Ordered :> Query {
	in root : Element;
	OrderBy(
		source = Project(
			source = Descendants(source = root, maxDepth = 1),
			properties = ("name"),
			columns = (Column(name = "margin", expression = (Subsystem::alloc ?? 0.0) - Subsystem::mass))
		),
		property = "margin",
		direction = "ascending",
		missing = "last",
		multiple = "error"
	)
}
`)
	result := computedRows(t, fixture, "Ordered")
	rows := result.Rows()
	var names []string
	for _, row := range rows {
		name, _ := row.Cells()[0].Values()[0].String()
		names = append(names, name)
	}
	if !slices.Equal(names, []string{"b", "a"}) {
		t.Fatalf("ordered names = %v", names)
	}
}

func TestExecuteComputedColumnsAreDeterministic(t *testing.T) {
	fixture := computedFixture(t, `
calc def Margins :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		columns = (Column(name = "margin", expression = (Subsystem::alloc ?? 0.0) - Subsystem::mass))
	)
}
`)
	first := computedRows(t, fixture, "Margins")
	second := computedRows(t, fixture, "Margins")
	for i, row := range first.Rows() {
		left, _ := row.Cells()[0].Values()[0].Real()
		right, _ := second.Rows()[i].Cells()[0].Values()[0].Real()
		if left != right {
			t.Fatalf("row %d: %v != %v", i, left, right)
		}
	}
}

func TestExecuteComputedResultsAreImmutableToCallers(t *testing.T) {
	fixture := computedFixture(t, `
calc def Margins :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		columns = (Column(name = "margin", expression = Subsystem::count * 2))
	)
}
`)
	result := computedRows(t, fixture, "Margins")
	rows := result.Rows()
	rows[0] = Row{}
	values := result.Rows()[0].Cells()[0].Values()
	if _, ok := values[0].Integer(); !ok {
		t.Fatal("row set must be immutable to callers")
	}
}

func TestExecuteComputedBareAbsentFeatureIsTyped(t *testing.T) {
	fixture := computedFixture(t, `
calc def Bad :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		columns = (Column(name = "alloc", expression = Subsystem::alloc))
	)
}`)
	_, err := fixture.execute(t, "Bad", Bindings{
		"root": {ElementValue(fixture.symbol(t, "system"))},
	}, Options{})
	var executionError *Error
	if !errors.As(err, &executionError) || executionError.Kind != ErrorColumnAbsent {
		t.Fatalf("error = %v", err)
	}
	if executionError.Property != "alloc" || executionError.Target == "" {
		t.Fatalf("error provenance = %+v", executionError)
	}
	if !executionError.Origin.Located() {
		t.Fatal("absent column errors must retain source provenance")
	}
}

func TestExecuteComputedColumnSkipsUnrelatedSameNamedFeatures(t *testing.T) {
	fixture := loadExecutionFixture(t, `
part def Tank {
	attribute mass : Real;
}
part def Report {
	attribute mass : String;
}
part mixed {
	part tank : Tank {
		attribute redefines mass = 3.5;
	}
	part report : Report {
		attribute redefines mass = "heavy";
	}
}
calc def Masses :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		properties = ("name"),
		columns = (Column(name = "m", expression = Tank::mass ?? 0.0))
	)
}`)
	result, err := fixture.execute(t, "Masses", Bindings{
		"root": {ElementValue(fixture.symbol(t, "mixed"))},
	}, Options{})
	if err != nil {
		t.Fatalf("execute Masses: %v", err)
	}
	values := make(map[string]Value)
	for _, row := range result.Rows() {
		name, _ := row.Cells()[0].Values()[0].String()
		values[name] = row.Cells()[1].Values()[0]
	}
	if mass, ok := values["tank"].Real(); !ok || mass != 3.5 {
		t.Fatalf("tank mass = %+v", values["tank"])
	}
	// Report::mass is unrelated to Tank::mass; the ?? default must apply.
	if mass, ok := values["report"].Real(); !ok || mass != 0.0 {
		t.Fatalf("report mass = %+v", values["report"])
	}
}

func TestExecuteComputedColumnFailuresAreTyped(t *testing.T) {
	cases := []struct {
		name  string
		query string
		kind  ErrorKind
	}{
		{
			name: "division by zero",
			kind: ErrorColumnDivisionByZero,
			query: `
calc def Bad :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		columns = (Column(name = "bad", expression = Subsystem::count / 0))
	)
}`,
		},
		{
			name: "absent operand without default",
			kind: ErrorColumnOperand,
			query: `
calc def Bad :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		columns = (Column(name = "bad", expression = Subsystem::alloc - Subsystem::mass))
	)
}`,
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			fixture := computedFixture(t, test.query)
			_, err := fixture.execute(t, "Bad", Bindings{
				"root": {ElementValue(fixture.symbol(t, "system"))},
			}, Options{})
			var executionError *Error
			if !errors.As(err, &executionError) || executionError.Kind != test.kind {
				t.Fatalf("error = %v", err)
			}
			if executionError.Property != "bad" || executionError.Target == "" {
				t.Fatalf("error provenance = %+v", executionError)
			}
			if !executionError.Origin.Located() {
				t.Fatal("computed column errors must retain source provenance")
			}
		})
	}
}
