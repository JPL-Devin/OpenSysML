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
		{FormatXMI, FormatSysML, true},
		{FormatXMI, FormatTurtle, true},
	}
	for _, c := range cases {
		if got := IsExperimental(c.from, c.to); got != c.want {
			t.Errorf("IsExperimental(%s, %s) = %t, want %t", c.from, c.to, got, c.want)
		}
	}
}

// TestNotices checks each experimental conversion names what is experimental
// about it: the migration, the RDF mapping, or both for XMI to Turtle.
func TestNotices(t *testing.T) {
	cases := []struct {
		from, to Format
		want     []string
	}{
		{FormatSysML, FormatSysML, nil},
		{FormatSysML, FormatTurtle, []string{ExperimentalNotice}},
		{FormatXMI, FormatSysML, []string{MigrationNotice}},
		{FormatXMI, FormatTurtle, []string{MigrationNotice, ExperimentalNotice}},
	}
	for _, c := range cases {
		got := Notices(c.from, c.to)
		if strings.Join(got, "|") != strings.Join(c.want, "|") {
			t.Errorf("Notices(%s, %s) = %q, want %q", c.from, c.to, got, c.want)
		}
		if IsExperimental(c.from, c.to) != (len(got) > 0) {
			t.Errorf("IsExperimental(%s, %s) disagrees with its notices", c.from, c.to)
		}
	}
	for _, want := range []string{"experimental", "docs/reference/sysml-v1-migration.md"} {
		if !strings.Contains(MigrationNotice, want) {
			t.Errorf("migration notice is missing %q:\n%s", want, MigrationNotice)
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
