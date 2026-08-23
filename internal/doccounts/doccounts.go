// Package doccounts is the one census of the compliance map's status markers and
// the one statement of which documentation lines derive from it: the guard in
// cmd/pilot-diff checks those lines against it, cmd/doc-counts rewrites them from
// it, and neither parses the markers itself.
package doccounts

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/template"
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

// RefereedCounts is the five-figure census read from the committed baselines.
type RefereedCounts struct {
	RuleCounts             RuleCounts
	Files                  int
	FilesAgreeing          int
	OursOnly               int
	PilotOnly              int
	DeclaredErrors         int
	Silent                 int
	DeclaredAgree          int
	WordingOnly            int
	LocationOnly           int
	SeverityDiffers        int
	Elsewhere              int
	ScopeExact             int
	ScopeTotal             int
	RejectCases            int
	RejectPilotOnly        int
	RejectBoth             int
	RejectDefaultPilotOnly int
	RejectDefaultBoth      int
	RejectStrictOnly       int
	SelfAssessed           int
	PilotTag               string
	PilotArtifact          string
}

type differentialBaseline struct {
	PilotRelease string `json:"pilotRelease"`
	Totals       struct {
		Files         int `json:"files"`
		FilesAgreeing int `json:"filesFullyAgreeing"`
		OursOnly      int `json:"openSysMLOnly"`
		PilotOnly     int `json:"pilotOnly"`
	} `json:"totals"`
}

type xpectBaseline struct {
	Kinds []struct {
		Kind            string `json:"kind"`
		Assertions      int    `json:"assertions"`
		Rows            int    `json:"rows"`
		Agree           int    `json:"agree"`
		WordingOnly     int    `json:"wordingOnly"`
		SameLocation    int    `json:"sameLocation"`
		SameLine        int    `json:"sameLine"`
		SeverityDiffers int    `json:"severityDiffers"`
		Elsewhere       int    `json:"elsewhereInFile"`
	} `json:"kinds"`
}

type rejectionBaseline struct {
	Totals struct {
		Cases            int `json:"cases"`
		BothReject       int `json:"bothReject"`
		PilotOnlyRejects int `json:"pilotOnlyRejects"`
	} `json:"totals"`
	StrictOnlyAgreements []string `json:"strictOnlyAgreements"`
}

// ReadRefereedCounts reads and derives all five headline figures.
func ReadRefereedCounts(root string) (RefereedCounts, error) {
	compliance, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(SpecCompliancePath)))
	if err != nil {
		return RefereedCounts{}, err
	}
	ruleCounts := CountRules(string(compliance))
	if ruleCounts.Total == 0 {
		return RefereedCounts{}, fmt.Errorf("%s states no rule rows, so there is no census to write", SpecCompliancePath)
	}
	var differential differentialBaseline
	if err := readJSON(root, "docs/project/pilot-differential-baseline.json", &differential); err != nil {
		return RefereedCounts{}, err
	}
	var xpect xpectBaseline
	if err := readJSON(root, "docs/project/pilot-xpect-baseline.json", &xpect); err != nil {
		return RefereedCounts{}, err
	}
	var rejection rejectionBaseline
	if err := readJSON(root, "docs/project/pilot-rejection-baseline.json", &rejection); err != nil {
		return RefereedCounts{}, err
	}
	pilotTag, pilotArtifact, err := parsePilotRelease(differential.PilotRelease)
	if err != nil {
		return RefereedCounts{}, fmt.Errorf("docs/project/pilot-differential-baseline.json: %w", err)
	}
	counts := RefereedCounts{
		RuleCounts:       ruleCounts,
		Files:            differential.Totals.Files,
		FilesAgreeing:    differential.Totals.FilesAgreeing,
		OursOnly:         differential.Totals.OursOnly,
		PilotOnly:        differential.Totals.PilotOnly,
		RejectCases:      rejection.Totals.Cases,
		RejectPilotOnly:  rejection.Totals.PilotOnlyRejects,
		RejectBoth:       rejection.Totals.BothReject,
		RejectStrictOnly: len(rejection.StrictOnlyAgreements),
		PilotTag:         pilotTag,
		PilotArtifact:    pilotArtifact,
	}
	counts.RejectDefaultBoth = counts.RejectBoth - counts.RejectStrictOnly
	counts.RejectDefaultPilotOnly = counts.RejectPilotOnly + counts.RejectStrictOnly
	if counts.RejectDefaultBoth < 0 {
		return RefereedCounts{}, fmt.Errorf("docs/project/pilot-rejection-baseline.json: more strict-only agreements than agreements")
	}
	var foundErrors, foundScope bool
	for _, kind := range xpect.Kinds {
		switch kind.Kind {
		case "errors":
			foundErrors = true
			counts.DeclaredErrors = kind.Rows
			counts.DeclaredAgree = kind.Agree - kind.WordingOnly
			counts.WordingOnly = kind.WordingOnly
			counts.LocationOnly = kind.SameLine
			counts.SeverityDiffers = kind.SeverityDiffers
			counts.Elsewhere = kind.Elsewhere
			counts.Silent = kind.Rows - kind.Agree - kind.SameLocation - kind.SameLine - kind.SeverityDiffers - kind.Elsewhere
			if counts.DeclaredAgree < 0 || counts.Silent < 0 {
				return RefereedCounts{}, fmt.Errorf("docs/project/pilot-xpect-baseline.json: errors agreements and tolerances exceed %d rows", kind.Rows)
			}
		case "scope":
			foundScope = true
			counts.ScopeExact = kind.Agree
			counts.ScopeTotal = kind.Assertions
		}
	}
	if !foundErrors || !foundScope {
		return RefereedCounts{}, fmt.Errorf("docs/project/pilot-xpect-baseline.json: baseline states no errors or scope kind to derive the headline from")
	}
	selfAssessed, err := ReadSelfAssessedRows(root)
	if err != nil {
		return RefereedCounts{}, err
	}
	counts.SelfAssessed = selfAssessed
	return counts, nil
}

func readJSON(root, path string, into any) error {
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		return err
	}
	if err := json.Unmarshal(content, into); err != nil {
		return fmt.Errorf("%s: parse baseline: %w", path, err)
	}
	return nil
}

func parsePilotRelease(release string) (string, string, error) {
	match := pilotReleasePattern.FindStringSubmatch(release)
	if match == nil {
		return "", "", fmt.Errorf("pilotRelease %q does not match `TAG (jupyter-sysml-kernel ARTIFACT)`", release)
	}
	return match[1], match[2], nil
}

// ReadSelfAssessedRows counts rows in sections without an external referee.
func ReadSelfAssessedRows(root string) (int, error) {
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(SpecCompliancePath)))
	if err != nil {
		return 0, err
	}
	rows, sections := 0, 0
	unrefereed := false
	for _, line := range strings.Split(string(content), "\n") {
		text := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(text, "#"):
			unrefereed = false
		case strings.HasPrefix(text, "**No external referee:**"):
			unrefereed = true
			sections++
		case unrefereed && IsRuleRow(text):
			rows++
		}
	}
	if sections == 0 {
		return 0, fmt.Errorf("%s: no section declares %q, so the self-assessed headline has no source", SpecCompliancePath, "**No external referee:**")
	}
	return rows, nil
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
	mapHeaderPattern     = regexp.MustCompile(`^The map below tracks ([0-9]+) semantic rules: \*\*([0-9]+) ✅ faithful, ([0-9]+) ⚠️ approximate, ([0-9]+) ❌ not implemented, ([0-9]+) ⛔ deliberate divergence\.\*\*`)
	referenceLinePattern = regexp.MustCompile(`^\*\*Reference differential:\*\* ([0-9]+) files compared diagnostic-by-diagnostic against the pinned OMG pilot implementation \(` + "`" + `([^` + "`" + `]+)` + "`" + `\), ([0-9]+) in full agreement;`)
	rejectionLinePattern = regexp.MustCompile(`^\*\*Rejection oracle:\*\* the reverse direction — do we reject what the reference rejects\? ([0-9]+) hand-written invalid models validated by both implementations, ([0-9]+) rejected by both, ([0-9]+) the pinned pilot rejects and we accept;`)
	pilotReleasePattern  = regexp.MustCompile(`^([^ ]+) \(jupyter-sysml-kernel ([^)]+)\)$`)

	censusValues = func(counts RuleCounts) []int {
		return []int{counts.Total, counts.Faithful, counts.Approximate, counts.NotImplemented, counts.Deliberate}
	}
	censusLabels  = []string{"total", "✅ faithful", "⚠️ approximate", "❌ not implemented", "⛔ deliberate divergence"}
	censusSources = []string{
		SpecCompliancePath + " total rows",
		SpecCompliancePath + " ✅ rows",
		SpecCompliancePath + " ⚠️ rows",
		SpecCompliancePath + " ❌ rows",
		SpecCompliancePath + " ⛔ rows",
	}
)

// BaselineLine is a line whose values come from the committed oracle baselines.
type BaselineLine struct {
	Path    string
	Marker  string
	Pattern *regexp.Regexp
	Values  func(RefereedCounts) []string
}

// BaselineLines lists the single-copy oracle lines regenerated from baselines.
func BaselineLines() []BaselineLine {
	return []BaselineLine{{
		Path:    ReadmePath,
		Marker:  "**Reference differential:**",
		Pattern: referenceLinePattern,
		Values: func(counts RefereedCounts) []string {
			return []string{strconv.Itoa(counts.Files), counts.PilotTag, strconv.Itoa(counts.FilesAgreeing)}
		},
	}, {
		Path:    ReadmePath,
		Marker:  "**Rejection oracle:**",
		Pattern: rejectionLinePattern,
		Values: func(counts RefereedCounts) []string {
			return []string{strconv.Itoa(counts.RejectCases), strconv.Itoa(counts.RejectBoth), strconv.Itoa(counts.RejectPilotOnly)}
		},
	}}
}

// Block describes a generated named block and its consumer-relative links.
type Block struct {
	Path       string
	Name       string
	LinkPrefix string
}

// Blocks lists the consumers of the shared refereed-figures block.
func Blocks() []Block {
	return []Block{
		{Path: ReadmePath, Name: "refereed-figures", LinkPrefix: "docs/project/"},
		{Path: ArchitecturePath, Name: "refereed-figures", LinkPrefix: "../project/"},
	}
}

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

// RewriteBaselineLine restates one baseline-derived line without other changes.
func RewriteBaselineLine(content string, spec BaselineLine, counts RefereedCounts) (string, error) {
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
		return "", fmt.Errorf("%s:%d: line captures %d values, the baseline states %d", spec.Path, index+1, got, len(values))
	}
	rewritten := lines[index]
	for i := len(values) - 1; i >= 0; i-- {
		rewritten = rewritten[:match[2+i*2]] + values[i] + rewritten[match[3+i*2]:]
	}
	lines[index] = rewritten
	return strings.Join(lines, "\n"), nil
}

const (
	blockBeginFormat = "<!-- doc-counts:begin %s -->"
	blockEndFormat   = "<!-- doc-counts:end %s -->"
)

// RewriteBlock replaces a named generated block and preserves surrounding bytes.
func RewriteBlock(content string, spec Block, counts RefereedCounts) (string, error) {
	begin := fmt.Sprintf(blockBeginFormat, spec.Name)
	end := fmt.Sprintf(blockEndFormat, spec.Name)
	lines := strings.Split(content, "\n")
	beginIndex, endIndex := -1, -1
	for i, line := range lines {
		switch strings.TrimSpace(line) {
		case begin:
			if beginIndex >= 0 {
				return "", fmt.Errorf("%s: duplicate %q marker", spec.Path, begin)
			}
			beginIndex = i
		case end:
			if endIndex >= 0 {
				return "", fmt.Errorf("%s: duplicate %q marker", spec.Path, end)
			}
			endIndex = i
		}
	}
	if beginIndex < 0 || endIndex < 0 || endIndex <= beginIndex {
		return "", fmt.Errorf("%s: named block %q is missing or unterminated", spec.Path, spec.Name)
	}
	renderedBlock, err := renderBlock(spec, counts)
	if err != nil {
		return "", err
	}
	rendered := strings.Split(renderedBlock, "\n")
	updated := make([]string, 0, len(lines)-endIndex+beginIndex+len(rendered))
	updated = append(updated, lines[:beginIndex]...)
	updated = append(updated, rendered...)
	updated = append(updated, lines[endIndex+1:]...)
	return strings.Join(updated, "\n"), nil
}

type blockTemplateData struct {
	Name                   string
	LinkPrefix             string
	PilotTag               string
	PilotArtifact          string
	FilesAgreeing          int
	Files                  int
	OursOnly               int
	PilotOnly              int
	DeclaredErrors         int
	Silent                 int
	DeclaredAgree          int
	WordingOnly            int
	LocationOnly           int
	SeverityDiffers        int
	Elsewhere              int
	ScopeExact             int
	ScopeTotal             int
	RejectCases            int
	RejectDefaultPilotOnly int
	RejectDefaultBoth      int
	RejectStrictOnly       int
	SelfAssessed           int
	RuleTotal              int
}

const refereedBlockTemplateText = "<!-- doc-counts:begin {{.Name}} -->\n" +
	"**Measured against the pinned reference** (`PILOT_TAG={{.PilotTag}}`, artifact `{{.PilotArtifact}}`). Every number below is generated by `make docs-counts` from the committed baselines and gated; none of them is typed in by hand.\n\n" +
	"- **Corpus agreement:** {{.FilesAgreeing}} of {{.Files}} files agree diagnostic-by-diagnostic; {{.OursOnly}} diagnostics are ours alone and {{.PilotOnly}} the reference's alone, and the first number must be read by root: our diagnostics against the reference's own corpora fell while our non-standard-notation warnings on our own example models rose ([differential]({{.LinkPrefix}}pilot-differential.md), `go run ./cmd/pilot-diff`).\n" +
	"- **Declared-diagnostic silence:** of the {{.DeclaredErrors}} declared `errors` rows in the reference's own Xpect suites, we report nothing for {{.Silent}}. {{.DeclaredAgree}} we report word-for-word; {{.WordingOnly}} wording-only and {{.LocationOnly}} location-only differences are agreement in substance and are not counted as gaps; {{.SeverityDiffers}} more we report as a warning and {{.Elsewhere}} elsewhere in the file ([Xpect oracle]({{.LinkPrefix}}pilot-xpect.md), `go run ./cmd/pilot-xpect`).\n" +
	"- **Scope agreement:** {{.ScopeExact}} of {{.ScopeTotal}} declared scope assertions match exactly (same source).\n" +
	"- **Permissiveness gaps:** of {{.RejectCases}} invalid models we wrote ourselves, the reference rejects {{.RejectDefaultPilotOnly}} that we accept by default, and {{.RejectDefaultBoth}} both reject; {{.RejectStrictOnly}} further cases agree only when we are asked strictly. We authored every one of these cases ourselves, so the denominator measures the reach of our own corpus and not our conformance; agreement reached only under an opt-in strict mode is weaker evidence than agreement by default ([rejection oracle]({{.LinkPrefix}}pilot-rejection.md), `go run ./cmd/pilot-reject`).\n" +
	"- **Self-assessed surface:** {{.SelfAssessed}} of the tracked rules have no external referee at all — the action, state-machine and classifier-behavior rows, which the four numbers above cannot see, because the pinned artifact evaluates expressions but executes neither actions nor state machines.\n\n" +
	"What these numbers cannot show: the OMG corpora are demonstrations rather than an official conformance suite; the differential is one-directional, comparing the diagnostics the two implementations report on the same files; the Xpect suites are the pilot authors' test intent rather than a certification oracle; and none of these is a percentage of the specification — no global compliance figure is claimed anywhere.\n\n" +
	"**Row bookkeeping:** the ✅/⚠️/❌/⛔ status of each of the {{.RuleTotal}} tracked rules stays in [spec compliance]({{.LinkPrefix}}spec-compliance.md) as a census of our own row list. It moves when rows are rewritten and does not move when an oracle does, so it is not the progress measure.\n" +
	"<!-- doc-counts:end {{.Name}} -->"

var refereedBlockTemplate = template.Must(template.New("refereed-figures").Parse(refereedBlockTemplateText))

func renderBlock(spec Block, counts RefereedCounts) (string, error) {
	data := blockTemplateData{
		Name:                   spec.Name,
		LinkPrefix:             spec.LinkPrefix,
		PilotTag:               counts.PilotTag,
		PilotArtifact:          counts.PilotArtifact,
		FilesAgreeing:          counts.FilesAgreeing,
		Files:                  counts.Files,
		OursOnly:               counts.OursOnly,
		PilotOnly:              counts.PilotOnly,
		DeclaredErrors:         counts.DeclaredErrors,
		Silent:                 counts.Silent,
		DeclaredAgree:          counts.DeclaredAgree,
		WordingOnly:            counts.WordingOnly,
		LocationOnly:           counts.LocationOnly,
		SeverityDiffers:        counts.SeverityDiffers,
		Elsewhere:              counts.Elsewhere,
		ScopeExact:             counts.ScopeExact,
		ScopeTotal:             counts.ScopeTotal,
		RejectCases:            counts.RejectCases,
		RejectDefaultPilotOnly: counts.RejectDefaultPilotOnly,
		RejectDefaultBoth:      counts.RejectDefaultBoth,
		RejectStrictOnly:       counts.RejectStrictOnly,
		SelfAssessed:           counts.SelfAssessed,
		RuleTotal:              counts.RuleCounts.Total,
	}
	var rendered strings.Builder
	if err := refereedBlockTemplate.Execute(&rendered, data); err != nil {
		return "", fmt.Errorf("render refereed figures: %w", err)
	}
	return rendered.String(), nil
}
