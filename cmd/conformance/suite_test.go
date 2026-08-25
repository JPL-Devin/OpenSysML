package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// suiteDir is the committed suite, read from the repository root.
const suiteDir = "../../conformance"

func loadSuite(t *testing.T) []*Scenario {
	t.Helper()
	scenarios, err := loadScenarios(filepath.Join(suiteDir, "scenarios"))
	if err != nil {
		t.Fatalf("the committed suite does not load: %v", err)
	}
	return scenarios
}

// TestEveryScenarioNamesAnRPCOfTheService verifies no scenario addresses an RPC
// the schema does not declare, which would otherwise fail only at run time.
func TestEveryScenarioNamesAnRPCOfTheService(t *testing.T) {
	for _, scenario := range loadSuite(t) {
		if _, err := methodByName(scenario.method()); err != nil {
			t.Errorf("%s: %v", scenario.ID, err)
		}
	}
}

// TestEveryRPCIsCovered verifies the suite reaches every RPC of the service, so
// adding one to the schema without covering it fails here.
func TestEveryRPCIsCovered(t *testing.T) {
	descriptor, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(serviceName))
	if err != nil {
		t.Fatal(err)
	}
	methods := descriptor.(protoreflect.ServiceDescriptor).Methods()

	covered := map[string]int{}
	for _, scenario := range loadSuite(t) {
		covered[scenario.method()]++
	}
	for i := 0; i < methods.Len(); i++ {
		name := string(methods.Get(i).Name())
		if covered[name] == 0 {
			t.Errorf("no scenario covers %s", name)
		}
	}
}

// TestTheSuiteCoversBothKindsOfFailure verifies it pins refused requests and
// failures the response reports in band, not only happy paths.
func TestTheSuiteCoversBothKindsOfFailure(t *testing.T) {
	statuses := map[string]int{}
	inBand := 0
	for _, scenario := range loadSuite(t) {
		expects := []*Expect{&scenario.Expect}
		if scenario.ExpectWithoutCapability != nil {
			expects = append(expects, scenario.ExpectWithoutCapability)
		}
		for _, expect := range expects {
			if expect.Status != "" {
				statuses[expect.Status]++
			}
			for _, path := range expect.NonEmpty {
				if strings.HasSuffix(path, "error") {
					inBand++
				}
			}
			for path := range expect.Contains {
				if strings.HasSuffix(path, "error") {
					inBand++
				}
			}
		}
	}
	for _, want := range []string{"INVALID_ARGUMENT", "NOT_FOUND", "UNIMPLEMENTED"} {
		if statuses[want] == 0 {
			t.Errorf("no scenario expects the status %s", want)
		}
	}
	if inBand == 0 {
		t.Error("no scenario expects a failure reported in the response's error field")
	}
}

// TestEveryFixtureIsUsed verifies the suite carries no fixture no scenario
// reads, which would be dead data.
func TestEveryFixtureIsUsed(t *testing.T) {
	fixtures, err := filepath.Glob(filepath.Join(suiteDir, "fixtures", "*.sysml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(fixtures) == 0 {
		t.Fatal("no fixtures found")
	}
	scenarios := loadSuite(t)
	for _, fixture := range fixtures {
		name := filepath.Base(fixture)
		used := false
		for _, scenario := range scenarios {
			if scenario.Model != nil && scenario.Model.Fixture == name {
				used = true
			}
			if strings.Contains(string(scenario.Request), "${fixture:"+name+"}") {
				used = true
			}
		}
		if !used {
			t.Errorf("fixture %s is read by no scenario", name)
		}
	}
}

// TestAMissingCapabilitySkipsRatherThanFails verifies a scenario needing a
// capability the service does not report is skipped, and that a scenario saying
// what such a service must answer instead is not skipped.
func TestAMissingCapabilitySkipsRatherThanFails(t *testing.T) {
	r := &runner{capabilities: []string{"query"}, out: &strings.Builder{}, scenarioLog: &strings.Builder{}}

	skipped := r.run(context.Background(), &Scenario{
		ID: "needs/absent", RPC: "Convert", RequiresCapabilities: []string{"convert"},
	})
	if skipped.Outcome != "skip" {
		t.Errorf("outcome = %s, want skip", skipped.Outcome)
	}
	if !strings.Contains(skipped.Reason, "convert") {
		t.Errorf("reason = %q, want it to name the missing capability", skipped.Reason)
	}

	present := r.missingCapabilities(&Scenario{RequiresCapabilities: []string{"query"}})
	if len(present) != 0 {
		t.Errorf("missing = %v, want none", present)
	}
}
