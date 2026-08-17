package export

import (
	"strings"
	"testing"
)

// TestIsExperimental checks that the RDF mapping is what marks a conversion
// experimental, in either direction, and that notation alone does not.
func TestIsExperimental(t *testing.T) {
	cases := []struct {
		from, to Format
		want     bool
	}{
		{FormatSysML, FormatTurtle, true},
		{FormatTurtle, FormatSysML, true},
		{FormatTurtle, FormatTurtle, true},
		{FormatSysML, FormatSysML, false},
	}
	for _, c := range cases {
		if got := IsExperimental(c.from, c.to); got != c.want {
			t.Errorf("IsExperimental(%s, %s) = %t, want %t", c.from, c.to, got, c.want)
		}
	}
}

// TestExperimentalNoticeNamesTheStatusSection checks the notice points at the
// documentation that carries the status, so every surface quoting it does.
func TestExperimentalNoticeNamesTheStatusSection(t *testing.T) {
	for _, want := range []string{"experimental", "docs/reference/rdf-mapping.md"} {
		if !strings.Contains(ExperimentalNotice, want) {
			t.Errorf("notice is missing %q:\n%s", want, ExperimentalNotice)
		}
	}
}
