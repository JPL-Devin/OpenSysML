package main

import (
	"encoding/json"
	"testing"
)

// queryModel declares a document query over a small part tree, so -run-query
// decides both ways: rows reported, and a failure that gates a build.
const queryModel = `package Observatory {
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
		part mount : Subsystem {
			attribute redefines mass = 15.0;
		}
	}

	calc def HeavySubsystems :> Query {
		in root : Element;
		Project(
			source = WhereFeature(
				source = WhereType(
					source = Descendants(source = root, maxDepth = 3),
					type = "PartUsage"
				),
				'feature' = "mass",
				operator = ">=",
				value = "10"
			),
			properties = ("name", "mass")
		)
	}

	calc def NamedTelescopeParts :> Query {
		in root : Element;
		in pattern : String;
		WhereName(source = OwnedElements(source = root), operator = "startsWith", value = pattern)
	}

	calc def ComposedQuery :> Query {
		in root : Element;
		HeavySubsystems(root = root)
	}

	calc def RelatedQuery :> Query {
		in root : Element;
		RelatedElements(
			source = root,
			relationshipKind = "typing",
			direction = "incoming",
			maxDepth = 1
		)
	}

	calc def UnknownRelatedQuery :> Query {
		in root : Element;
		RelatedElements(
			source = root,
			relationshipKind = "refinement",
			direction = "outgoing",
			maxDepth = 1
		)
	}
}
`

// TestRunQueryFlag checks the scripted surface of document queries: rows on
// stdout for a query that ran, repeatability, and an unresolved status when
// the query could not be run at all.
func TestRunQueryFlag(t *testing.T) {
	binary := buildCLI(t)

	wantReport(t, check(t, binary, queryModel, "-run-query", "HeavySubsystems root=Observatory::telescope"),
		0, "✓ Query Observatory::HeavySubsystems returned 1 row",
		"Columns: name, mass",
		"Row 1: Observatory::telescope::mount",
		`name = "mount"`,
		"mass = 15.0")

	// A binding expression may contain unquoted spaces.
	wantReport(t, check(t, binary, queryModel, "-run-query", `NamedTelescopeParts root=telescope pattern="m" + "o"`),
		0, "✓ Query Observatory::NamedTelescopeParts returned 1 row")

	// The flag repeats, and a later failure gates the run.
	wantReport(t, check(t, binary, queryModel,
		"-run-query", "HeavySubsystems root=telescope",
		"-run-query", "NoSuchQuery root=telescope"),
		2, "✓ Query Observatory::HeavySubsystems returned 1 row", "NoSuchQuery")

	wantReport(t, check(t, binary, queryModel, "-run-query", "HeavySubsystems"),
		2, "requires binding root")

	// A composed query runs through its invoked definition.
	wantReport(t, check(t, binary, queryModel, "-run-query", "ComposedQuery root=telescope"),
		0, "✓ Query Observatory::ComposedQuery returned 1 row",
		"Row 1: Observatory::telescope::mount")

	// A named relationship traversal reports its related elements.
	wantReport(t, check(t, binary, queryModel, "-run-query", "RelatedQuery root=Subsystem"),
		0, "✓ Query Observatory::RelatedQuery returned 2 rows",
		"Row 1: Observatory::telescope::optics",
		"Row 2: Observatory::telescope::mount")

	// An unsupported relationship kind is surfaced, not silently skipped.
	wantReport(t, check(t, binary, queryModel, "-run-query", "UnknownRelatedQuery root=telescope"),
		2, `does not support relationship kind "refinement"`)
}

// TestRunQueryJSON checks that a query's verdict is reported as data a build
// step reads: its row count and projected columns as named values.
func TestRunQueryJSON(t *testing.T) {
	binary := buildCLI(t)

	got := check(t, binary, queryModel, "-run-query", "HeavySubsystems root=telescope", "-json")
	if got.status != 0 {
		t.Fatalf("exit status = %d, want 0\n%s", got.status, got.output())
	}
	var report struct {
		Status string `json:"status"`
		Checks []struct {
			Subject string `json:"subject"`
			Status  string `json:"status"`
			Values  []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"values"`
		} `json:"checks"`
	}
	if err := json.Unmarshal([]byte(got.stdout), &report); err != nil {
		t.Fatalf("stdout is not the reported JSON: %v\n%s", err, got.stdout)
	}
	if report.Status != "holds" || len(report.Checks) != 1 {
		t.Fatalf("report = %q with %d checks, want holds with 1:\n%s", report.Status, len(report.Checks), got.stdout)
	}
	values := map[string]string{}
	for _, value := range report.Checks[0].Values {
		values[value.Name] = value.Value
	}
	if values["rows"] != "1" || values["columns"] != "name, mass" {
		t.Errorf("values = %v, want rows = 1 and columns = name, mass", report.Checks[0].Values)
	}
}
