package flexo

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/export"
	"github.com/Open-MBEE/OpenSysML/internal/core/rdf"
)

// These run everywhere: they check that a report of a given measurement is the
// same text every time, and that the payload views the comparison rests on
// describe the fixtures. The stack itself is measured by TestFlexoInterop.

func TestReportTextIsDeterministic(t *testing.T) {
	report := &Report{
		Fixture: "model.sysml",
		Graph: GraphStats{Subjects: 2, Triples: 5, Bytes: 100,
			ByNamespace: []PropertyStat{{Property: "rdf", Written: 2}, {Property: "sysml", Written: 3}}},
		Load: SideReport{
			Name: "graph-load", Accepted: true, Commits: 2, Written: 2, Listed: 2, Pages: 1,
			Direct: 1, Roots: 2, RootsInModel: 1,
			Elements: []ElementStat{
				{ID: "B", Type: "PartUsage", Listed: true, Direct: "refused(400)", Written: 2, Delivered: 1,
					Lost: []string{"sysx:hasBody"}, Shape: []string{"sysml:type:reference-as-literal"}},
				{ID: "A", Type: "Package", Listed: true, Direct: "ok", Written: 1, Delivered: 1},
			},
			Properties: []PropertyStat{
				{Property: "sysx:hasBody", Written: 1},
				{Property: "sysml:declaredName", Written: 2, Delivered: 2},
			},
		},
		Findings: []string{"graph-load: 2 of 2 elements listed"},
	}

	first := report.Text("# header\n")
	// Rendering must not depend on the order the measurements arrived in.
	report.Load.Elements[0], report.Load.Elements[1] = report.Load.Elements[1], report.Load.Elements[0]
	report.Load.Properties[0], report.Load.Properties[1] = report.Load.Properties[1], report.Load.Properties[0]
	if second := report.Text("# header\n"); second != first {
		t.Errorf("the report changed when the measurements were reordered:\n%s",
			firstDifference(first, second))
	}

	for _, want := range []string{
		"[graph-load]\naccepted\tyes\n",
		"A\ttype=Package\tlisted=yes\tdirect=ok\tproperties=1/1\n",
		"B\ttype=PartUsage\tlisted=yes\tdirect=refused(400)\tproperties=1/2\tlost=sysx:hasBody\tshape=sysml:type:reference-as-literal\n",
		"sysml:declaredName\twritten=2\tdelivered=2\tmulti-valued=0\n",
	} {
		if !strings.Contains(first, want) {
			t.Errorf("the report does not record %q:\n%s", want, first)
		}
	}
}

// The fixture is what the round trip is measured on, so its graph must hold the
// shapes the measurement is about: an extension property, a multi-valued
// standard property, and an expression node the service cannot address.
func TestFixtureGraphCoversTheGaps(t *testing.T) {
	model, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read %s: %v", fixturePath, err)
	}
	graph, err := export.SysMLToRDF("model.sysml", model)
	if err != nil {
		t.Fatalf("convert the fixture: %v", err)
	}
	written := writtenFromGraph(graph)

	if len(written) < 10 {
		t.Errorf("the fixture writes %d subjects, too few to measure anything", len(written))
	}

	var extension, multiValued, unaddressable int
	for id, element := range written {
		if !validElementID(id) {
			unaddressable++
		}
		for property, values := range element.props {
			if strings.HasPrefix(property, "sysx:") {
				extension++
			}
			if len(values) > 1 {
				multiValued++
			}
		}
		if element.metaclass == "" {
			t.Errorf("%s has no metaclass, so the read path would return it typeless", id)
		}
	}
	if extension == 0 {
		t.Error("the fixture writes no sysx: property, so the dropped-namespace gap is unmeasured")
	}
	if multiValued == 0 {
		t.Error("the fixture writes no multi-valued property, so that gap is unmeasured")
	}
	if unaddressable == 0 {
		t.Error("the fixture writes no id the service would refuse, so that gap is unmeasured")
	}
	if _, ok := written[rdf.EncodeElementID("Interop")]; !ok {
		t.Error("the fixture's root package is missing from its graph")
	}
}

// The reference fixture is the ground truth, so it must be a commit request the
// service accepts: every change carries an @id and a @type.
func TestReferenceFixtureIsAPostableCommit(t *testing.T) {
	fixture, err := os.ReadFile(referencePath)
	if err != nil {
		t.Fatalf("read %s: %v", referencePath, err)
	}
	changes, written, err := referencePayload(fixture)
	if err != nil {
		t.Fatalf("read the reference payload: %v", err)
	}

	var request struct {
		Type   string `json:"@type"`
		Change []struct {
			Payload map[string]json.RawMessage `json:"payload"`
		} `json:"change"`
	}
	if err := json.Unmarshal(changes, &request); err != nil {
		t.Fatalf("the posted request is not JSON: %v", err)
	}
	if request.Type != "Commit" {
		t.Errorf("the posted request is a %q, want a Commit", request.Type)
	}
	if len(request.Change) != len(written) {
		t.Errorf("the request posts %d changes for %d elements", len(request.Change), len(written))
	}
	// The documentation in the fixture is for a reader and must not be posted.
	if strings.Contains(string(changes), "requireValidId") {
		t.Error("the posted request carries the fixture's documentation")
	}

	for _, change := range request.Change {
		for _, key := range []string{"@id", "@type"} {
			if _, ok := change.Payload[key]; !ok {
				t.Errorf("a change has no %s: %v", key, change.Payload)
			}
		}
	}
	for id, element := range written {
		if !validElementID(id) {
			t.Errorf("%s is not an id the service accepts, so its side of the "+
				"comparison could not be read back", id)
		}
		if element.metaclass == "" {
			t.Errorf("%s posts no metaclass", id)
		}
	}
}

func TestDeliveredKind(t *testing.T) {
	for _, test := range []struct{ raw, want string }{
		{`{"@id":"A"}`, "reference"},
		{`{"body":"x"}`, "object"},
		{`["a","b"]`, "array"},
		{`null`, "null"},
		{`"text"`, "literal"},
		{`6`, "literal"},
		{`true`, "literal"},
	} {
		if got := deliveredKind(json.RawMessage(test.raw)); got != test.want {
			t.Errorf("deliveredKind(%s) = %s, want %s", test.raw, got, test.want)
		}
	}
}

func TestValidElementID(t *testing.T) {
	for id, want := range map[string]bool{
		"Interop__Rover":                    true,
		"Interop__Rover__wheels.upperBound": false,
		"a-b_0":                             true,
		"":                                  false,
		"Interop::Rover":                    false,
	} {
		if got := validElementID(id); got != want {
			t.Errorf("validElementID(%q) = %v, want %v", id, got, want)
		}
	}
}

func TestRootsInModelCountsUnownedElements(t *testing.T) {
	written := map[string]*writtenElement{
		"A": {id: "A", props: map[string][]value{"sysml:declaredName": {{kind: "literal"}}}},
		"B": {id: "B", props: map[string][]value{"sysml:owningNamespace": {{kind: "reference"}}}},
		"C": {id: "C", props: map[string][]value{"owner": {{kind: "reference"}}}},
	}
	if got := rootsInModel(written); got != 1 {
		t.Errorf("rootsInModel = %d, want 1", got)
	}
}
