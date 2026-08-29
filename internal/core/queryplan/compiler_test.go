package queryplan

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

const queryImports = `
private import DocumentQueries::*;
private import KerML::Root::Element;
`

type queryFixture struct {
	index    *symbols.Index
	model    *semantics.Model
	resolver *resolve.Resolver
}

func loadQueryFixture(t *testing.T, body string) queryFixture {
	t.Helper()
	index := symbols.NewIndex()
	if err := libs.NewLoader(libs.DefaultSource(), nil).LoadAll(index); err != nil {
		t.Fatalf("load standard library: %v", err)
	}
	name := "queries.sysml"
	p := parser.New(source.New(name, []byte("package Fixture {"+queryImports+body+"}")))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse query fixture: %v", p.Diagnostics)
	}
	index.AddDocument(name, root)
	index.ExpandWildcardImports()
	resolver := resolve.New(index)
	model := semantics.NewModel(resolver)
	return queryFixture{index: index, model: model, resolver: resolver}
}

func (f queryFixture) symbol(t *testing.T, name string) *symbols.Symbol {
	t.Helper()
	matches := symbols.PreferDeclared(f.index.LookupQualified("Fixture::" + name))
	if len(matches) != 1 {
		t.Fatalf("lookup %s: got %d symbols", name, len(matches))
	}
	return matches[0]
}

func (f queryFixture) compile(t *testing.T, name string) *Program {
	t.Helper()
	program, err := Compile(f.index, f.model, f.resolver, f.symbol(t, name))
	if err != nil {
		t.Fatalf("compile %s: %v", name, err)
	}
	return program
}

func planningError(t *testing.T, err error, kind ErrorKind) *Error {
	t.Helper()
	var planning *Error
	if !errors.As(err, &planning) {
		t.Fatalf("got %T, want *queryplan.Error", err)
	}
	if planning.Kind != kind {
		t.Fatalf("error kind = %s, want %s (%v)", planning.Kind, kind, planning)
	}
	return planning
}

func TestCompileQueryCompositionDependencyOrder(t *testing.T) {
	fixture := loadQueryFixture(t, `
calc def InterfacesFor :> Query {
	in subsystem : Element;
	Descendants(source = subsystem, maxDepth = 2)
}
calc def APSInterfaces :> Query {
	in subsystem : Element;
	InterfacesFor(subsystem = subsystem)
}
calc def ProvidedAPSInterfaces :> Query {
	in subsystem : Element;
	WhereType(
		source = APSInterfaces(subsystem = subsystem),
		type = "InterfaceUsage"
	)
}
`)
	program := fixture.compile(t, "ProvidedAPSInterfaces")
	if program.Entry() != "Fixture::ProvidedAPSInterfaces" {
		t.Fatalf("entry = %q", program.Entry())
	}
	definitions := program.Definitions()
	var names []string
	for _, definition := range definitions {
		names = append(names, definition.Name())
	}
	want := []string{"Fixture::InterfacesFor", "Fixture::APSInterfaces", "Fixture::ProvidedAPSInterfaces"}
	if !slices.Equal(names, want) {
		t.Fatalf("definition order = %v, want %v", names, want)
	}
	if got := definitions[1].Dependencies(); !slices.Equal(got, []string{"Fixture::InterfacesFor"}) {
		t.Fatalf("APSInterfaces dependencies = %v", got)
	}
	if got := definitions[2].Dependencies(); !slices.Equal(got, []string{"Fixture::APSInterfaces"}) {
		t.Fatalf("ProvidedAPSInterfaces dependencies = %v", got)
	}
	if definitions[2].Expression().Operation() != OperationWhereType {
		t.Fatalf("provided operation = %s", definitions[2].Expression().Operation())
	}
	parameters := definitions[2].Parameters()
	if len(parameters) != 1 || parameters[0].Name != "subsystem" ||
		parameters[0].Type != "KerML::Root::Element" ||
		parameters[0].Multiplicity != (Multiplicity{Lower: 1, Upper: 1, Known: true}) {
		t.Fatalf("provided parameters = %+v", parameters)
	}
	result := definitions[2].Result()
	if result.Type != "KerML::Root::Element" ||
		result.Multiplicity != (Multiplicity{Lower: 0, UpperInfinite: true, Known: true}) {
		t.Fatalf("provided result = %+v", result)
	}
	if !definitions[2].Origin().Located() || !definitions[2].Expression().Origin().Located() {
		t.Fatal("compiled definition and expression must retain source provenance")
	}
}

func TestCompileMemoizesRepeatedDependency(t *testing.T) {
	fixture := loadQueryFixture(t, `
calc def Base :> Query {
	in subsystem : Element;
	OwnedElements(source = subsystem)
}
calc def Combined :> Query {
	in subsystem : Element;
	(Base(subsystem = subsystem), Base(subsystem = subsystem))
}
`)
	definitions := fixture.compile(t, "Combined").Definitions()
	if len(definitions) != 2 {
		t.Fatalf("compiled definitions = %d, want Base and Combined once each", len(definitions))
	}
	if got := definitions[1].Dependencies(); !slices.Equal(got, []string{"Fixture::Base"}) {
		t.Fatalf("Combined dependencies = %v", got)
	}
}

func TestCompiledProgramIsImmutableToCallers(t *testing.T) {
	fixture := loadQueryFixture(t, `
calc def InterfacesFor :> Query {
	in subsystem : Element;
	Descendants(source = subsystem, maxDepth = 2)
}
`)
	program := fixture.compile(t, "InterfacesFor")
	definitions := program.Definitions()
	definitions[0] = Definition{}
	args := program.Definitions()[0].Expression().Arguments()
	args[0] = Argument{}

	fresh := program.Definitions()
	if fresh[0].Name() != "Fixture::InterfacesFor" {
		t.Fatal("mutating returned definitions changed the compiled program")
	}
	if fresh[0].Expression().Arguments()[0].Name != "source" {
		t.Fatal("mutating returned arguments changed the compiled program")
	}
}

func TestCompileRejectsDirectAndIndirectCompositionCycles(t *testing.T) {
	tests := []struct {
		name string
		body string
		root string
		path []string
	}{
		{
			name: "direct",
			body: `
calc def DirectCycle :> Query {
	in subsystem : Element;
	DirectCycle(subsystem = subsystem)
}`,
			root: "DirectCycle",
			path: []string{"Fixture::DirectCycle", "Fixture::DirectCycle"},
		},
		{
			name: "indirect",
			body: `
calc def CycleA :> Query { in subsystem : Element; CycleB(subsystem = subsystem) }
calc def CycleB :> Query { in subsystem : Element; CycleC(subsystem = subsystem) }
calc def CycleC :> Query { in subsystem : Element; CycleA(subsystem = subsystem) }
`,
			root: "CycleA",
			path: []string{"Fixture::CycleA", "Fixture::CycleB", "Fixture::CycleC", "Fixture::CycleA"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := loadQueryFixture(t, test.body)
			_, err := Compile(fixture.index, fixture.model, fixture.resolver, fixture.symbol(t, test.root))
			planning := planningError(t, err, ErrorCompositionCycle)
			if !slices.Equal(planning.Path, test.path) {
				t.Fatalf("cycle path = %v, want %v", planning.Path, test.path)
			}
			if !strings.Contains(planning.Error(), strings.Join(test.path, " -> ")) {
				t.Fatalf("cycle error does not include full path: %v", planning)
			}
		})
	}
}

func TestCompileRejectsPositionalQueryInvocation(t *testing.T) {
	fixture := loadQueryFixture(t, `
calc def Base :> Query {
	in subsystem : Element;
	OwnedElements(source = subsystem)
}
calc def Positional :> Query {
	in subsystem : Element;
	Base(subsystem)
}
`)
	_, err := Compile(fixture.index, fixture.model, fixture.resolver, fixture.symbol(t, "Positional"))
	planningError(t, err, ErrorPositionalQueryArgs)
}

func TestCompileRetainsPositionalBuiltinInvocation(t *testing.T) {
	fixture := loadQueryFixture(t, `
calc def PositionalBuiltin :> Query {
	in subsystem : Element;
	Descendants(subsystem, 2)
}
`)
	expression := fixture.compile(t, "PositionalBuiltin").Definitions()[0].Expression()
	args := expression.Arguments()
	if len(args) != 2 || args[0].Named || args[1].Named {
		t.Fatalf("positional builtin arguments = %+v", args)
	}
}

func TestCompileReportsDistinctDefinitionFaults(t *testing.T) {
	fixture := loadQueryFixture(t, `
calc def Helper {
	in subsystem : Element;
	subsystem
}
calc def MissingResult :> Query {
	in subsystem : Element;
}
calc def UnsupportedResult :> Query {
	in subsystem : Element;
	subsystem.name
}
calc def UnknownTarget :> Query {
	in subsystem : Element;
	Helper(subsystem)
}
`)
	tests := []struct {
		name string
		kind ErrorKind
	}{
		{"MissingResult", ErrorMissingResult},
		{"UnsupportedResult", ErrorUnsupportedExpression},
		{"UnknownTarget", ErrorUnknownInvocation},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Compile(fixture.index, fixture.model, fixture.resolver, fixture.symbol(t, test.name))
			planningError(t, err, test.kind)
		})
	}
	_, err := Compile(fixture.index, fixture.model, fixture.resolver, fixture.symbol(t, "Helper"))
	planningError(t, err, ErrorNotQueryDefinition)
}

func TestCompileValidatesNamedQueryBindings(t *testing.T) {
	fixture := loadQueryFixture(t, `
calc def Base :> Query {
	in subsystem : Element;
	in status : String;
	OwnedElements(source = subsystem)
}
calc def Missing :> Query {
	in subsystem : Element;
	Base(subsystem = subsystem)
}
calc def Unknown :> Query {
	in subsystem : Element;
	Base(subsystem = subsystem, status = "ready", extra = "value")
}
calc def Duplicate :> Query {
	in subsystem : Element;
	Base(subsystem = subsystem, status = "ready", status = "again")
}
`)
	tests := []struct {
		name string
		kind ErrorKind
	}{
		{"Missing", ErrorMissingArgument},
		{"Unknown", ErrorUnknownArgument},
		{"Duplicate", ErrorDuplicateArgument},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Compile(fixture.index, fixture.model, fixture.resolver, fixture.symbol(t, test.name))
			planningError(t, err, test.kind)
		})
	}
}

func TestCompileRejectsNonInputQueryParameter(t *testing.T) {
	fixture := loadQueryFixture(t, `
calc def InvalidParameter :> Query {
	out scratch : Element;
	OwnedElements(source = scratch)
}
`)
	_, err := Compile(fixture.index, fixture.model, fixture.resolver, fixture.symbol(t, "InvalidParameter"))
	planningError(t, err, ErrorInvalidParameter)
}

func TestCompileRequiresSemanticContext(t *testing.T) {
	_, err := Compile(nil, nil, nil, nil)
	planningError(t, err, ErrorInvalidContext)
}

func TestDocumentQueryVocabularyIsBundled(t *testing.T) {
	fixture := loadQueryFixture(t, "")
	for _, name := range []string{
		queryBaseFQN,
		"DocumentQueries::OwnedElements",
		"DocumentQueries::Descendants",
		"DocumentQueries::Ancestors",
		"DocumentQueries::RelatedElements",
		"DocumentQueries::WhereType",
		"DocumentQueries::WhereMetadata",
		"DocumentQueries::WhereName",
		"DocumentQueries::WhereFeature",
		"DocumentQueries::OrderBy",
		"DocumentQueries::Project",
	} {
		if got := symbols.PreferDeclared(fixture.index.LookupQualified(name)); len(got) != 1 {
			t.Errorf("bundled declaration %s: got %d symbols", name, len(got))
		}
	}
}
