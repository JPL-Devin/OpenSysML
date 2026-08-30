package repl

import (
	"reflect"
	"strings"
	"testing"
)

// docQueryModel declares document queries over a small part tree: a projecting
// query, one relying on a default, and one composing another by name, which
// execution refuses as a typed failure.
const docQueryModel = `package Observatory {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;

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
			properties = ("name", "mass")
		)
	}

	calc def PlainCalc {
		in x : Real;
		x
	}

	calc def NamedSubsystems :> Query {
		in root : Element;
		in pattern : String default "mo";
		WhereName(source = OwnedElements(source = root), operator = "startsWith", value = pattern)
	}

	calc def ComposedQuery :> Query {
		in root : Element;
		HeavySubsystems(root = root)
	}
}
`

func docQuerySession(t *testing.T) *Session {
	t.Helper()
	s := NewSession()
	if res := s.Submit(docQueryModel); len(errorDiagnostics(res.Diagnostics)) > 0 {
		t.Fatalf("model did not analyse cleanly: %v", res.Diagnostics)
	}
	return s
}

func TestRunQueryProjectsOrderedRows(t *testing.T) {
	s := docQuerySession(t)
	got := run(t, s, "%run-query HeavySubsystems root=Observatory::telescope")
	wants(t, got,
		"✓ Query Observatory::HeavySubsystems returned 2 rows",
		"Columns: name, mass",
		"Row 1: Observatory::telescope::mount",
		`name = "mount"`,
		"mass = 15.0",
		"Row 2: Observatory::telescope::segmentControl",
		"mass = 20.0",
	)
	if strings.Index(got, "mount") > strings.Index(got, "segmentControl") {
		t.Errorf("rows are not in the query's order:\n%s", got)
	}
}

func TestRunDocumentQueryVerdict(t *testing.T) {
	s := docQuerySession(t)
	v := s.RunDocumentQuery("HeavySubsystems root=telescope")
	if !v.Holds() {
		t.Fatalf("verdict = %s: %v", v.Status, v.Lines)
	}
	values := map[string]string{}
	for _, nv := range v.Values {
		values[nv.Name] = nv.Value
	}
	if values["rows"] != "2" || values["columns"] != "name, mass" {
		t.Errorf("values = %v", v.Values)
	}
}

func TestRunQueryUsageAndUnknownName(t *testing.T) {
	s := docQuerySession(t)
	wants(t, run(t, s, "%run-query"), "usage: %run-query <name> [<parameter>=<expression> ...]")
	wants(t, run(t, s, "%run-query NoSuchQuery"), "error:", "NoSuchQuery")
	wants(t, run(t, s, "%run-query HeavySubsystems telescope"),
		"error:", "<parameter>=<expression>")
}

func TestRunQueryRejectsNonQueryDefinition(t *testing.T) {
	s := docQuerySession(t)
	wants(t, run(t, s, "%run-query PlainCalc x=1.0"),
		"error:", "not a document query", "DocumentQueries::Query")
}

func TestRunQuerySurfacesTypedExecutionFailures(t *testing.T) {
	s := docQuerySession(t)
	wants(t, run(t, s, "%run-query HeavySubsystems"),
		"error:", "requires binding root")
	wants(t, run(t, s, "%run-query HeavySubsystems root=telescope depth=3"),
		"error:", "unknown binding depth")
	wants(t, run(t, s, "%run-query HeavySubsystems root=1"),
		"error:", "binding root has type integer, expected")
	// A default is intentionally unavailable at execution time, not evaluated.
	wants(t, run(t, s, "%run-query NamedSubsystems root=telescope"),
		"error:", "relies on a default not retained in the plan")
	// Named query invocation is a documented execution limitation.
	wants(t, run(t, s, "%run-query ComposedQuery root=telescope"),
		"error:", "not executable in this engine version")
}

func TestRunQueryBindingExpressions(t *testing.T) {
	s := docQuerySession(t)
	wants(t, run(t, s, `%run-query NamedSubsystems root=telescope pattern="mo"`),
		"✓ Query Observatory::NamedSubsystems returned 1 row",
		"Row 1: Observatory::telescope::mount")
	wants(t, run(t, s, "%run-query HeavySubsystems root=noSuchName"),
		"error:", "binding root")
}

func TestRunQuerySpacedBindingExpressions(t *testing.T) {
	s := docQuerySession(t)
	wants(t, run(t, s, `%run-query NamedSubsystems root=telescope pattern="m" + "o"`),
		"✓ Query Observatory::NamedSubsystems returned 1 row",
		"Row 1: Observatory::telescope::mount")
	wants(t, run(t, s, `%run-query NamedSubsystems root=telescope pattern=("m" + "o")`),
		"✓ Query Observatory::NamedSubsystems returned 1 row")
	wants(t, run(t, s, `%run-query NamedSubsystems pattern="m" + "o" root=telescope`),
		"✓ Query Observatory::NamedSubsystems returned 1 row")
}

func TestRegroupBindings(t *testing.T) {
	cases := []struct {
		tokens []string
		want   []string
	}{
		{[]string{"limit=1", "+", "2"}, []string{"limit=1 + 2"}},
		{[]string{"a=1", "b=2"}, []string{"a=1", "b=2"}},
		{[]string{"a=(1", "+", "2)", "b=3"}, []string{"a=(1 + 2)", "b=3"}},
		{[]string{`s="a b"`, "+", `"c"`}, []string{`s="a b" + "c"`}},
		{[]string{"a=x", "==", "y"}, []string{"a=x == y"}},
		{[]string{"telescope"}, []string{"telescope"}},
	}
	for _, c := range cases {
		if got := regroupBindings(c.tokens); !reflect.DeepEqual(got, c.want) {
			t.Errorf("regroupBindings(%q) = %q, want %q", c.tokens, got, c.want)
		}
	}
}

func TestRunQueryListedInHelpAndCompletion(t *testing.T) {
	s := docQuerySession(t)
	wants(t, run(t, s, "%help"), "%run-query <name> [<p>=<expr>...]")
	comp := s.Complete("%run-que", len("%run-que"))
	found := false
	for _, cand := range comp.Candidates {
		if cand == "%run-query" {
			found = true
		}
	}
	if !found {
		t.Errorf("%%run-query is not completed: %v", comp.Candidates)
	}
}
