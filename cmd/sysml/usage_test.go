package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/export"
)

// The help states the RDF mapping's status in the same wording a conversion
// reports, wrapped rather than reworded.
func TestPrintUsageStatesTheExperimentalNotice(t *testing.T) {
	var help bytes.Buffer
	printUsage(&help)

	unwrapped := strings.Join(strings.Fields(help.String()), " ")
	if !strings.Contains(unwrapped, export.ExperimentalNotice) {
		t.Errorf("the help does not state the notice:\n%s", help.String())
	}
	for _, line := range strings.Split(help.String(), "\n") {
		if len(line) > 96 {
			t.Errorf("help line is %d characters wide:\n%s", len(line), line)
		}
	}
}

func TestWrappedKeepsWordsWholeWithinTheWidth(t *testing.T) {
	text := "one two three four five six seven"
	got := wrapped(text, 10)
	for _, line := range strings.Split(got, "\n") {
		if len(line) > 10 {
			t.Errorf("line %q is wider than 10", line)
		}
	}
	if strings.Join(strings.Fields(got), " ") != text {
		t.Errorf("wrapping changed the words: %q", got)
	}
}
