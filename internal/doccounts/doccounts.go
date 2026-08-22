// Package doccounts is the one census of the compliance map's status markers and
// the one statement of which documentation lines derive from it: the guard in
// cmd/pilot-diff checks those lines against it, cmd/doc-counts rewrites them from
// it, and neither parses the markers itself.
package doccounts

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Paths of the compliance map and of the files carrying a derived line, relative
// to the repository root.
const (
	SpecCompliancePath = "docs/project/spec-compliance.md"
	ReadmePath         = "README.md"
	ArchitecturePath   = "docs/internals/architecture.md"
)

// statusMarkers are the row statuses the compliance map uses. '⚠' is matched
// without its variation selector, as the map writes both spellings.
var statusMarkers = []string{"✅", "⚠", "❌", "⛔", "🚧"}

// RuleCounts is the census of the compliance map's rule rows: a row is one table
// row carrying exactly one status marker.
type RuleCounts struct {
	Total          int
	Faithful       int
	Approximate    int
	NotImplemented int
	Deliberate     int
	KnownFailure   int
}

// IsRuleRow reports whether a line is a compliance-map table row carrying exactly
// one status marker. Header, separator and prose lines carry none, and a row
// naming several statuses in its notes is not a census of one status.
func IsRuleRow(line string) bool {
	text := strings.TrimSpace(line)
	if !strings.HasPrefix(text, "|") {
		return false
	}
	found := 0
	for _, cell := range strings.Split(strings.Trim(text, "|"), "|") {
		for _, marker := range statusMarkers {
			found += strings.Count(cell, marker)
		}
	}
	return found == 1
}

// CountRules counts the status markers of every rule row of the compliance map.
func CountRules(content string) RuleCounts {
	counts := RuleCounts{}
	for _, line := range strings.Split(content, "\n") {
		if !IsRuleRow(line) {
			continue
		}
		switch {
		case strings.Contains(line, "✅"):
			counts.Faithful++
		case strings.Contains(line, "⚠"):
			counts.Approximate++
		case strings.Contains(line, "❌"):
			counts.NotImplemented++
		case strings.Contains(line, "⛔"):
			counts.Deliberate++
		case strings.Contains(line, "🚧"):
			counts.KnownFailure++
		}
	}
	counts.Total = counts.Faithful + counts.Approximate + counts.NotImplemented + counts.Deliberate + counts.KnownFailure
	return counts
}

// Line is a documentation line whose numbers are a function of the census. Marker
// locates it, Pattern captures its numbers in the order Values states them, and
// Labels names them for a mismatch message.
type Line struct {
	Path    string
	Marker  string
	Pattern *regexp.Regexp
	Values  func(RuleCounts) []int
	Labels  []string
	// Sources names, per value, the rows the census read it from.
	Sources []string
}

var (
	mapHeaderPattern      = regexp.MustCompile(`^The map below tracks ([0-9]+) semantic rules: \*\*([0-9]+) ✅ faithful, ([0-9]+) ⚠️ approximate, ([0-9]+) ❌ not implemented, ([0-9]+) ⛔ deliberate divergence\.\*\*`)
	rowBookkeepingPattern = regexp.MustCompile(`^\*\*Row bookkeeping:\*\* the ✅/⚠️/❌/⛔ status of each of the ([0-9]+) tracked rules`)

	censusValues = func(counts RuleCounts) []int {
		return []int{counts.Total, counts.Faithful, counts.Approximate, counts.NotImplemented, counts.Deliberate}
	}
	totalOnly = func(counts RuleCounts) []int { return []int{counts.Total} }

	censusLabels  = []string{"total", "✅ faithful", "⚠️ approximate", "❌ not implemented", "⛔ deliberate divergence"}
	censusSources = []string{
		SpecCompliancePath + " total rows",
		SpecCompliancePath + " ✅ rows",
		SpecCompliancePath + " ⚠️ rows",
		SpecCompliancePath + " ❌ rows",
		SpecCompliancePath + " ⛔ rows",
	}
	totalLabels  = []string{"tracked rules"}
	totalSources = []string{SpecCompliancePath + " total rows"}
)

// Lines are the documentation lines derived from the census, in the order the
// regenerator rewrites them.
func Lines() []Line {
	return []Line{{
		Path:    SpecCompliancePath,
		Marker:  "The map below tracks",
		Pattern: mapHeaderPattern,
		Values:  censusValues,
		Labels:  censusLabels,
		Sources: censusSources,
	}, {
		Path:    ReadmePath,
		Marker:  "**Row bookkeeping:**",
		Pattern: rowBookkeepingPattern,
		Values:  totalOnly,
		Labels:  totalLabels,
		Sources: totalSources,
	}, {
		Path:    ArchitecturePath,
		Marker:  "**Row bookkeeping:**",
		Pattern: rowBookkeepingPattern,
		Values:  totalOnly,
		Labels:  totalLabels,
		Sources: totalSources,
	}}
}

// FindLine returns the index of the first line of content carrying the marker.
func FindLine(content, marker string) (int, bool) {
	for i, line := range strings.Split(content, "\n") {
		if strings.Contains(line, marker) {
			return i, true
		}
	}
	return 0, false
}

// Rewrite returns content with spec's numbers restated from counts and every other
// byte untouched. Prose the pattern does not match is an error, not a rewrite.
func Rewrite(content string, spec Line, counts RuleCounts) (string, error) {
	if counts.KnownFailure != 0 {
		return "", fmt.Errorf("%s: %d 🚧 rows have no place in the derived lines; give them a status the lines state", SpecCompliancePath, counts.KnownFailure)
	}
	index, ok := FindLine(content, spec.Marker)
	if !ok {
		return "", fmt.Errorf("%s: no line carries %q", spec.Path, spec.Marker)
	}
	lines := strings.Split(content, "\n")
	match := spec.Pattern.FindStringSubmatchIndex(lines[index])
	if match == nil {
		return "", fmt.Errorf("%s:%d: line carrying %q does not match the derived-line pattern", spec.Path, index+1, spec.Marker)
	}
	values := spec.Values(counts)
	if got := len(match)/2 - 1; got != len(values) {
		return "", fmt.Errorf("%s:%d: line captures %d numbers, the census states %d", spec.Path, index+1, got, len(values))
	}
	rewritten := lines[index]
	for i := len(values) - 1; i >= 0; i-- {
		rewritten = rewritten[:match[2+i*2]] + strconv.Itoa(values[i]) + rewritten[match[3+i*2]:]
	}
	lines[index] = rewritten
	return strings.Join(lines, "\n"), nil
}
