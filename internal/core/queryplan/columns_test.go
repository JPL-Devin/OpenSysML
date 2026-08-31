package queryplan

import (
	"testing"
)

func entryDefinition(t *testing.T, program *Program) Definition {
	t.Helper()
	for _, definition := range program.Definitions() {
		if definition.Name() == program.Entry() {
			return definition
		}
	}
	t.Fatal("missing entry definition")
	return Definition{}
}

const computedFixture = `
part def Subsystem {
	attribute mass : Real;
	attribute alloc : Real;
	attribute count : Integer;
}
`

func TestCompileComputedColumns(t *testing.T) {
	fixture := loadQueryFixture(t, computedFixture+`
calc def Margins :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		properties = ("name"),
		columns = (
			Column(name = "margin", expression = Subsystem::alloc - Subsystem::mass),
			Column(name = "label", expression = "sub: " + Element::name)
		)
	)
}
`)
	program := fixture.compile(t, "Margins")
	definition := entryDefinition(t, program)
	project := definition.Expression()
	if project.Operation() != OperationProject {
		t.Fatalf("operation = %s", project.Operation())
	}
	var columns Expression
	for _, argument := range project.Arguments() {
		if argument.Name == "columns" {
			columns = argument.Value
		}
	}
	if columns.Operation() != OperationSequence {
		t.Fatalf("columns operation = %s", columns.Operation())
	}
	elements := columns.Arguments()
	if len(elements) != 2 {
		t.Fatalf("columns = %d, want 2", len(elements))
	}
	var names []string
	for _, element := range elements {
		if element.Value.Operation() != OperationColumn {
			t.Fatalf("element operation = %s", element.Value.Operation())
		}
		if !element.Value.Origin().Located() {
			t.Fatal("planned columns must carry source provenance")
		}
		names = append(names, element.Value.Target())
	}
	if names[0] != "margin" || names[1] != "label" {
		t.Fatalf("column names = %v", names)
	}
}

func TestCompiledColumnsAreImmutableToCallers(t *testing.T) {
	fixture := loadQueryFixture(t, computedFixture+`
calc def Margins :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		columns = (Column(name = "margin", expression = Subsystem::alloc - Subsystem::mass))
	)
}
`)
	program := fixture.compile(t, "Margins")
	definition := entryDefinition(t, program)
	arguments := definition.Expression().Arguments()
	for i := range arguments {
		arguments[i].Name = "mutated"
	}
	for _, argument := range definition.Expression().Arguments() {
		if argument.Name == "mutated" {
			t.Fatal("plan arguments must be immutable to callers")
		}
	}
}

func TestCompileComputedColumnDiagnostics(t *testing.T) {
	cases := []struct {
		name string
		body string
		kind ErrorKind
	}{
		{
			name: "unknown property reference",
			kind: ErrorUnknownColumnProperty,
			body: `
calc def Bad :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		columns = (Column(name = "margin", expression = Subsystem::missing - Subsystem::mass))
	)
}`,
		},
		{
			name: "non-literal column name",
			kind: ErrorColumnName,
			body: `
calc def Bad :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		columns = (Column(name = 42, expression = Subsystem::mass))
	)
}`,
		},
		{
			name: "static type mismatch",
			kind: ErrorColumnType,
			body: `
calc def Bad :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		columns = (Column(name = "bad", expression = Subsystem::mass + "kg"))
	)
}`,
		},
		{
			name: "unsupported operator",
			kind: ErrorColumnOperator,
			body: `
calc def Bad :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		columns = (Column(name = "bad", expression = Subsystem::mass ** 2.0))
	)
}`,
		},
		{
			name: "duplicate column name",
			kind: ErrorDuplicateColumn,
			body: `
calc def Bad :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		properties = ("name"),
		columns = (Column(name = "name", expression = Subsystem::mass))
	)
}`,
		},
		{
			name: "empty projection",
			kind: ErrorEmptyProjection,
			body: `
calc def Bad :> Query {
	in root : Element;
	Project(source = Descendants(source = root, maxDepth = 1))
}`,
		},
		{
			name: "explicit null properties",
			kind: ErrorEmptyProjection,
			body: `
calc def Bad :> Query {
	in root : Element;
	Project(source = Descendants(source = root, maxDepth = 1), properties = null)
}`,
		},
		{
			name: "explicit null columns",
			kind: ErrorEmptyProjection,
			body: `
calc def Bad :> Query {
	in root : Element;
	Project(source = Descendants(source = root, maxDepth = 1), columns = null)
}`,
		},
		{
			name: "explicit null properties and columns",
			kind: ErrorEmptyProjection,
			body: `
calc def Bad :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		properties = null,
		columns = null
	)
}`,
		},
		{
			name: "missing column expression",
			kind: ErrorMissingArgument,
			body: `
calc def Bad :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		columns = (Column(name = "empty"))
	)
}`,
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			fixture := loadQueryFixture(t, computedFixture+test.body)
			_, err := Compile(fixture.index, fixture.model, fixture.resolver, fixture.symbol(t, "Bad"))
			planning := planningError(t, err, test.kind)
			if !planning.Origin.Located() {
				t.Fatal("planning diagnostics must carry source spans")
			}
		})
	}
}
