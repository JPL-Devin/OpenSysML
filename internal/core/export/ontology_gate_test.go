package export_test

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/core/rdf/ontology"
)

// knownViolationsFile lists the disagreements between the graphs this tool
// writes and the OMG metamodel as the Open-MBEE OWL rendering states it. It is
// committed so that the gate below fails on a *new* disagreement while the ones
// already recorded in the roadmap are reported rather than hidden.
const knownViolationsFile = "testdata/ontology-known-violations.txt"

// TestGoldenGraphsMatchOntology checks every golden Turtle graph against the
// generated ontology table: each SysML-namespace predicate must be a property
// the metamodel declares, on a metaclass the subject conforms to, carrying the
// kind of object (IRI or literal) the property's range allows.
//
// The fixtures are read from disk rather than embedded, so the gate keeps
// working when element identity changes underneath it.
func TestGoldenGraphsMatchOntology(t *testing.T) {
	known, err := readKnownViolations(knownViolationsFile)
	if err != nil {
		t.Fatal(err)
	}
	graphs := goldenGraphs(t)
	total := 0
	occurrences := make(map[string]int)
	examples := make(map[string]ontology.Violation)
	for _, path := range graphs {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		graph, err := rdf.ParseTurtle(data)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		for _, violation := range ontology.Check(graph) {
			key := violation.Key()
			occurrences[key]++
			total++
			if _, seen := examples[key]; !seen {
				examples[key] = violation
			}
		}
	}
	for _, key := range sortedKeys(occurrences) {
		reason, isKnown := known[key]
		if !isKnown {
			t.Errorf("new ontology violation in %d triple(s): %s",
				occurrences[key], examples[key])
			continue
		}
		t.Logf("known violation in %d triple(s): %s — %s", occurrences[key], key, reason)
	}
	for _, key := range sortedKeys(known) {
		if occurrences[key] == 0 {
			t.Errorf("%s lists %q, which no golden graph violates any more: remove the entry",
				knownViolationsFile, key)
		}
	}
	t.Logf("checked %d golden graphs against ontology version %s (commit %s): %d triples in %d distinct violations",
		len(graphs), ontology.Version, ontology.SourceCommit, total, len(occurrences))
}

// goldenGraphs returns the golden Turtle fixtures, failing when none are found
// so that a moved fixture directory cannot make the gate vacuous.
func goldenGraphs(t *testing.T) []string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("testdata", "convert", "*.golden.ttl"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("no golden Turtle fixtures under testdata/convert")
	}
	sort.Strings(paths)
	return paths
}

// readKnownViolations reads the committed list: one violation key per line,
// followed by '#' and the reason it is expected.
func readKnownViolations(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	known := make(map[string]string)
	for number, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, reason, ok := strings.Cut(line, "#")
		if !ok || strings.TrimSpace(reason) == "" {
			return nil, fmt.Errorf("%s: line %d: no '# reason' on the entry", path, number+1)
		}
		fields := strings.Fields(key)
		if len(fields) != 3 {
			return nil, fmt.Errorf("%s: line %d: entry is not '<kind> <metaclass> <property>'",
				path, number+1)
		}
		entry := strings.Join(fields, " ")
		if _, duplicate := known[entry]; duplicate {
			return nil, fmt.Errorf("%s: line %d: %q is listed twice", path, number+1, entry)
		}
		known[entry] = strings.TrimSpace(reason)
	}
	if len(known) == 0 {
		return nil, fmt.Errorf("%s: no entries", path)
	}
	return known, nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
