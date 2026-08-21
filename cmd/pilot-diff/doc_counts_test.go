// This guards headline totals, Results cells and root rows, per-category only-ours/only-pilot prose, and the movement table's Now column against the committed baseline JSON without validators or corpora. Causal claims and attributions are out of scope; historical movement columns, narrative paragraphs, and adjudication-section counts remain unguarded.
// The test deliberately checks only mechanically comparable numbers: “this moved because #391 reached a trigger's payload” is not mechanically checkable and must not be implied by this guard.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

const (
	docPath      = "docs/project/pilot-differential.md"
	baselinePath = "docs/project/pilot-differential-baseline.json"
)

type numberedLine struct {
	number int
	text   string
}

type table struct {
	header     []string
	headerLine int
	rows       []tableRow
}

type tableRow struct {
	line  int
	cells []string
}

type numberToken struct {
	start int
	end   int
	text  string
	line  int
}

var (
	headlinePattern      = regexp.MustCompile(`^## Results \(pilot ` + "`" + `([^` + "`" + `]+)` + "`" + `, ([0-9]+) files\)$`)
	parentheticalPattern = regexp.MustCompile(`\s+\([^()]*\)$`)
	categoryItemPattern  = regexp.MustCompile("^(\\d+)\\s+(`?[A-Za-z][A-Za-z0-9-]*`?)")
	nextCategoryPattern  = regexp.MustCompile(",\\s*\\d+\\s+`?[A-Za-z][A-Za-z0-9-]*`?")
	integerPattern       = regexp.MustCompile(`^-?[0-9]+$`)
)

func TestPilotDifferentialDocumentCountsMatchBaseline(t *testing.T) {
	lines := readNumberedDocument(t)
	report := readBaselineReport(t)

	heading := requireResultsHeading(t, lines)
	assertHeadline(t, heading, report)

	results := requireTable(t, lines, heading.number+1, "")
	assertResultsTable(t, results, report)

	categoryStart := requireLineContaining(t, lines, "Per category, the only-ours totals are:")
	assertCategoryProse(t, lines, categoryStart, report)

	movementStart := requireLineContaining(t, lines, "What has moved since the adjudication")
	movement := requireTable(t, lines, movementStart.number+1, "Count")
	assertMovementTable(t, movement, report)
}

func readNumberedDocument(t *testing.T) []numberedLine {
	t.Helper()
	content, err := os.ReadFile(filepath.FromSlash("../../" + docPath))
	if err != nil {
		t.Fatalf("%s:1: read document: %v", docPath, err)
	}
	raw := strings.Split(string(content), "\n")
	lines := make([]numberedLine, len(raw))
	for i, text := range raw {
		lines[i] = numberedLine{number: i + 1, text: text}
	}
	return lines
}

func readBaselineReport(t *testing.T) Report {
	t.Helper()
	content, err := os.ReadFile(filepath.FromSlash("../../" + baselinePath))
	if err != nil {
		t.Fatalf("%s:1: read baseline: %v", docPath, err)
	}
	var report Report
	if err := json.Unmarshal(content, &report); err != nil {
		t.Fatalf("%s:1: parse baseline: %v", docPath, err)
	}
	return report
}

func requireResultsHeading(t *testing.T, lines []numberedLine) numberedLine {
	t.Helper()
	for _, line := range lines {
		if strings.HasPrefix(line.text, "## Results (pilot ") {
			return line
		}
	}
	failAt(t, 1, "missing Results heading")
	return numberedLine{}
}

func assertHeadline(t *testing.T, heading numberedLine, report Report) {
	t.Helper()
	match := headlinePattern.FindStringSubmatch(heading.text)
	if match == nil {
		failAt(t, heading.number, "malformed Results heading")
	}
	wantRelease := report.Pilot
	if before, _, ok := strings.Cut(report.Pilot, " ("); ok {
		wantRelease = before
	}
	if got := match[1]; got != wantRelease {
		failAt(t, heading.number, "headline release: want %q (baseline pilotRelease), got %q", wantRelease, got)
	}
	gotFiles, err := strconv.Atoi(match[2])
	if err != nil {
		failAt(t, heading.number, "headline files: malformed number %q", match[2])
	}
	if gotFiles != report.Totals.Files {
		failAt(t, heading.number, "headline files: want %d (baseline totals.files), got %d", report.Totals.Files, gotFiles)
	}
}

func requireLineContaining(t *testing.T, lines []numberedLine, marker string) numberedLine {
	t.Helper()
	for _, line := range lines {
		if strings.Contains(line.text, marker) {
			return line
		}
	}
	failAt(t, 1, "missing required section marker %q", marker)
	return numberedLine{}
}

func requireTable(t *testing.T, lines []numberedLine, from int, firstHeader string) table {
	t.Helper()
	for i := from - 1; i < len(lines); i++ {
		if !isTableLine(lines[i].text) {
			continue
		}
		header := splitTableCells(lines[i].text)
		if firstHeader != "" && (len(header) == 0 || header[0] != firstHeader) {
			continue
		}
		rows := make([]tableRow, 0)
		for j := i + 1; j < len(lines) && isTableLine(lines[j].text); j++ {
			rows = append(rows, tableRow{line: lines[j].number, cells: splitTableCells(lines[j].text)})
		}
		return table{header: header, headerLine: lines[i].number, rows: rows}
	}
	if firstHeader == "" {
		failAt(t, from, "missing Results table")
	} else {
		failAt(t, from, "missing movement table with first header %q", firstHeader)
	}
	return table{}
}

func isTableLine(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), "|")
}

func splitTableCells(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	parts := strings.Split(line, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func isSeparatorRow(cells []string) bool {
	if len(cells) == 0 {
		return false
	}
	for _, cell := range cells {
		cell = strings.TrimSpace(cell)
		if len(cell) < 3 || strings.Trim(cell, ":-") != "" {
			return false
		}
	}
	return true
}

func assertResultsTable(t *testing.T, results table, report Report) {
	t.Helper()
	wantHeader := []string{"Root", "Files", "Fully agreeing", "Ours", "Pilot", "Agreed", "Severity-only", "Only ours", "Only pilot"}
	if len(results.header) != len(wantHeader) {
		failAt(t, results.headerLine, "Results table header: want %q, got %q", wantHeader, results.header)
	}
	for i := range wantHeader {
		if results.header[i] != wantHeader[i] {
			failAt(t, results.headerLine, "Results table header column %d: want %q, got %q", i+1, wantHeader[i], results.header[i])
		}
	}

	rootByDir := make(map[string]RootReport, len(report.Roots))
	for _, root := range report.Roots {
		rootByDir[root.Dir] = root
	}
	foundDirs := make(map[string]bool)
	totalSeen := false
	for _, row := range results.rows {
		if isSeparatorRow(row.cells) {
			continue
		}
		if len(row.cells) != len(wantHeader) {
			failAt(t, row.line, "Results table row has %d cells, want %d", len(row.cells), len(wantHeader))
		}
		label := stripMarkdown(row.cells[0])
		label = parentheticalPattern.ReplaceAllString(label, "")
		if label == "Total" {
			if totalSeen {
				failAt(t, row.line, "Results table total row appears more than once")
			}
			totalSeen = true
			assertTotalsRow(t, row, report.Totals)
			continue
		}
		root, ok := rootByDir[label]
		if !ok {
			failAt(t, row.line, "Results table root %q is not in baseline roots", label)
		}
		if foundDirs[label] {
			failAt(t, row.line, "Results table root %q appears more than once", label)
		}
		foundDirs[label] = true
		assertTotalsCells(t, row, root.Totals, "roots["+root.Name+"].totals", "root "+root.Dir)
	}
	if !totalSeen {
		failAt(t, results.headerLine, "Results table is missing the Total row")
	}
	wantDirs := make([]string, 0, len(report.Roots))
	for _, root := range report.Roots {
		wantDirs = append(wantDirs, root.Dir)
	}
	gotDirs := make([]string, 0, len(foundDirs))
	for dir := range foundDirs {
		gotDirs = append(gotDirs, dir)
	}
	sort.Strings(wantDirs)
	sort.Strings(gotDirs)
	if strings.Join(wantDirs, "\x00") != strings.Join(gotDirs, "\x00") {
		failAt(t, results.headerLine, "Results table root set: want %q (baseline roots[].dir), got %q", wantDirs, gotDirs)
	}
}

func assertTotalsRow(t *testing.T, row tableRow, totals Totals) {
	t.Helper()
	assertTotalsCells(t, row, totals, "totals", "root Total")
}

func assertTotalsCells(t *testing.T, row tableRow, totals Totals, jsonPrefix, subject string) {
	t.Helper()
	columns := []struct {
		header string
		want   int
		field  string
	}{
		{"Files", totals.Files, "files"},
		{"Fully agreeing", totals.FilesAgreeing, "filesFullyAgreeing"},
		{"Ours", totals.OpenSysMLTotal, "openSysMLDiagnostics"},
		{"Pilot", totals.PilotTotal, "pilotDiagnostics"},
		{"Agreed", totals.Agreement, "agreement"},
		{"Severity-only", totals.SeverityMismatch, "severityMismatch"},
		{"Only ours", totals.OpenSysMLOnly, "openSysMLOnly"},
		{"Only pilot", totals.PilotOnly, "pilotOnly"},
	}
	for i, column := range columns {
		got := parseCellInteger(t, row, i+1, column.header)
		if got != column.want {
			failAt(t, row.line, `%s column %q: want %d (baseline %s.%s), got %d`, subject, column.header, column.want, jsonPrefix, column.field, got)
		}
	}
}

func parseCellInteger(t *testing.T, row tableRow, cell int, label string) int {
	t.Helper()
	value := stripMarkdown(row.cells[cell])
	if !integerPattern.MatchString(value) {
		failAt(t, row.line, "column %q: expected integer, got %q", label, value)
	}
	got, err := strconv.Atoi(value)
	if err != nil {
		failAt(t, row.line, "column %q: malformed integer %q", label, value)
	}
	return got
}

func stripMarkdown(value string) string {
	value = strings.ReplaceAll(value, "**", "")
	value = strings.ReplaceAll(value, "`", "")
	return strings.TrimSpace(value)
}

func assertCategoryProse(t *testing.T, lines []numberedLine, start numberedLine, report Report) {
	t.Helper()
	paragraphLines := []numberedLine{start}
	for i := start.number; i < len(lines) && strings.TrimSpace(lines[i].text) != ""; i++ {
		paragraphLines = append(paragraphLines, lines[i])
	}
	text := make([]string, len(paragraphLines))
	for i, line := range paragraphLines {
		text[i] = line.text
	}
	paragraph := strings.Join(text, "\n")
	onlyOursMarker := "only-ours totals are:"
	onlyPilotMarker := "Only-pilot:"
	oursAt := strings.Index(paragraph, onlyOursMarker)
	pilotAt := strings.Index(paragraph, onlyPilotMarker)
	if oursAt < 0 {
		failAt(t, start.number, "category paragraph is missing marker %q", onlyOursMarker)
	}
	if pilotAt < 0 {
		failAt(t, start.number, "category paragraph is missing marker %q", onlyPilotMarker)
	}
	if pilotAt <= oursAt+len(onlyOursMarker) {
		failAt(t, start.number, "category paragraph markers are out of order")
	}

	starts := paragraphLineStarts(paragraph)
	prose := map[string]map[string]map[Category]int{}
	proseLines := map[string]map[string]int{}
	var consumed []numberToken
	parseCategoryPart(t, paragraph, starts, paragraphLines, oursAt+len(onlyOursMarker), pilotAt, "only-ours", report, prose, proseLines, &consumed)
	parseCategoryPart(t, paragraph, starts, paragraphLines, pilotAt+len(onlyPilotMarker), len(paragraph), "only-pilot", report, prose, proseLines, &consumed)

	assertCategoryMaps(t, report, prose, proseLines)
	for _, token := range bareNumberTokens(paragraph, starts, paragraphLines) {
		if !numberWasConsumed(consumed, token) {
			failAt(t, token.line, "unaccounted number %q in category paragraph", token.text)
		}
	}
}

func parseCategoryPart(t *testing.T, paragraph string, starts []int, lines []numberedLine, from, to int, direction string, report Report, prose map[string]map[string]map[Category]int, proseLines map[string]map[string]int, consumed *[]numberToken) {
	t.Helper()
	part := paragraph[from:to]
	partOffset := from
	for len(part) > 0 {
		semicolon := strings.IndexByte(part, ';')
		rawSegment := part
		if semicolon >= 0 {
			rawSegment = part[:semicolon]
		}
		leading := len(rawSegment) - len(strings.TrimLeft(rawSegment, " \t\n"))
		segmentStart := partOffset + leading
		segment := strings.TrimSpace(rawSegment)
		if segment == "" {
			if semicolon < 0 {
				break
			}
			partOffset += semicolon + 1
			part = paragraph[partOffset:to]
			continue
		}
		parseCategorySegment(t, segment, segmentStart, starts, lines, direction, report, prose, proseLines, consumed)
		if semicolon < 0 {
			break
		}
		partOffset += semicolon + 1
		part = paragraph[partOffset:to]
	}
}

func parseCategorySegment(t *testing.T, segment string, segmentStart int, starts []int, lines []numberedLine, direction string, report Report, prose map[string]map[string]map[Category]int, proseLines map[string]map[string]int, consumed *[]numberToken) {
	t.Helper()
	rootMatch := regexp.MustCompile("(?s)^\\s*`([^`]+)`\\s*(.*)$").FindStringSubmatchIndex(segment)
	if rootMatch == nil {
		if tokens := bareNumberSpans(segment); len(tokens) > 0 {
			token := tokens[0]
			failAt(t, lineAt(starts, lines, segmentStart+token.start), "unaccounted number %q in category paragraph", token.text)
		}
		failAt(t, lineAt(starts, lines, segmentStart), "category %s segment must start with a backticked root name: %q", direction, segment)
	}
	rootName := segment[rootMatch[2]:rootMatch[3]]
	root, ok := rootByName(report, rootName)
	if !ok {
		failAt(t, lineAt(starts, lines, segmentStart+rootMatch[2]), "category %s names unknown root %q", direction, rootName)
	}
	restStart := segmentStart + rootMatch[4]
	rest := segment[rootMatch[4]:rootMatch[5]]
	if prose[direction] == nil {
		prose[direction] = map[string]map[Category]int{}
		proseLines[direction] = map[string]int{}
	}
	if prose[direction][rootName] == nil {
		prose[direction][rootName] = map[Category]int{}
	}
	if _, exists := proseLines[direction][rootName]; !exists {
		proseLines[direction][rootName] = lineAt(starts, lines, segmentStart+rootMatch[2])
	}

	knownCategories := reportCategories(report)
	parsed := 0
	for {
		leading := len(rest) - len(strings.TrimLeft(rest, " \t\n"))
		rest = rest[leading:]
		restStart += leading
		match := categoryItemPattern.FindStringSubmatchIndex(rest)
		if match == nil {
			if parsed == 0 {
				failAt(t, lineAt(starts, lines, restStart), "category %s root %q has no count/category items", direction, root.Name)
			}
			break
		}
		categoryText := strings.Trim(rest[match[4]:match[5]], "`")
		category := Category(categoryText)
		if !knownCategories[category] {
			failAt(t, lineAt(starts, lines, restStart+match[4]), "category %s root %q names unknown category %q", direction, root.Name, categoryText)
		}
		count, err := strconv.Atoi(rest[match[2]:match[3]])
		if err != nil {
			failAt(t, lineAt(starts, lines, restStart), "category %s root %q has malformed count", direction, root.Name)
		}
		numberStart := restStart + match[2]
		numberEnd := restStart + match[3]
		boundary := nextCategoryPattern.FindStringIndex(rest[match[1]:])
		tailEnd := len(rest)
		if boundary != nil {
			tailEnd = match[1] + boundary[0]
		}
		tail := rest[match[1]:tailEnd]
		if tokens := bareNumberSpans(tail); len(tokens) > 0 {
			token := tokens[0]
			failAt(t, lineAt(starts, lines, restStart+match[1]+token.start), "unaccounted number %q in %s %s tail", token.text, direction, root.Name)
		}
		prose[direction][rootName][category] += count
		*consumed = append(*consumed, numberToken{start: numberStart, end: numberEnd, text: rest[match[2]:match[3]], line: lineAt(starts, lines, numberStart)})
		parsed++
		if boundary == nil {
			break
		}
		nextStart := match[1] + boundary[0] + 1
		restStart += nextStart
		rest = rest[nextStart:]
	}
}

func assertCategoryMaps(t *testing.T, report Report, prose map[string]map[string]map[Category]int, proseLines map[string]map[string]int) {
	t.Helper()
	for _, direction := range []string{"only-ours", "only-pilot"} {
		for rootName, categories := range prose[direction] {
			root, ok := rootByName(report, rootName)
			if !ok {
				failAt(t, proseLines[direction][rootName], "root %s %s is not present in baseline roots", rootName, direction)
			}
			for category, got := range categories {
				want := categoryTotal(root, direction, category)
				if want == 0 || got != want {
					failAt(t, proseLines[direction][rootName], "root %s %s category %q: want %d (baseline roots[%s].files[].%s[].count), got %d", root.Name, direction, category, want, root.Name, directionJSONField(direction), got)
				}
			}
		}
		for _, root := range report.Roots {
			for category, want := range categoryTotals(root, direction) {
				if want == 0 || prose[direction][root.Name][category] == want {
					continue
				}
				got := prose[direction][root.Name][category]
				line := proseLines[direction][root.Name]
				if line == 0 {
					line = 1
				}
				failAt(t, line, "root %s %s category %q: want %d (baseline roots[%s].files[].%s[].count), got %d (missing from prose)", root.Name, direction, category, want, root.Name, directionJSONField(direction), got)
			}
		}
	}
}

func rootByName(report Report, name string) (RootReport, bool) {
	for _, root := range report.Roots {
		if root.Name == name {
			return root, true
		}
	}
	return RootReport{}, false
}

func reportCategories(report Report) map[Category]bool {
	categories := map[Category]bool{}
	for _, root := range report.Roots {
		for _, file := range root.Files {
			for _, entry := range file.OpenSysMLOnly {
				categories[entry.Category] = true
			}
			for _, entry := range file.PilotOnly {
				categories[entry.Category] = true
			}
		}
	}
	return categories
}

func categoryTotals(root RootReport, direction string) map[Category]int {
	totals := map[Category]int{}
	for _, file := range root.Files {
		var entries []Entry
		if direction == "only-ours" {
			entries = file.OpenSysMLOnly
		} else {
			entries = file.PilotOnly
		}
		for _, entry := range entries {
			totals[entry.Category] += entry.Count
		}
	}
	return totals
}

func categoryTotal(root RootReport, direction string, category Category) int {
	return categoryTotals(root, direction)[category]
}

func directionJSONField(direction string) string {
	if direction == "only-ours" {
		return "openSysMLOnly"
	}
	return "pilotOnly"
}

func paragraphLineStarts(paragraph string) []int {
	starts := []int{0}
	for i, char := range paragraph {
		if char == '\n' {
			starts = append(starts, i+1)
		}
	}
	return starts
}

func lineAt(starts []int, lines []numberedLine, offset int) int {
	line := 0
	for i, start := range starts {
		if start > offset {
			break
		}
		line = i
	}
	return lines[line].number
}

func bareNumberSpans(text string) []numberToken {
	var tokens []numberToken
	for i := 0; i < len(text); {
		if text[i] < '0' || text[i] > '9' || (i > 0 && isWordBefore(text, i)) {
			i++
			continue
		}
		j := i + 1
		for j < len(text) && text[j] >= '0' && text[j] <= '9' {
			j++
		}
		if j == len(text) || !isWordAt(text, j) {
			tokens = append(tokens, numberToken{start: i, end: j, text: text[i:j]})
		}
		i = j
	}
	return tokens
}

func bareNumberTokens(text string, starts []int, lines []numberedLine) []numberToken {
	tokens := bareNumberSpans(text)
	for i := range tokens {
		tokens[i].line = lineAt(starts, lines, tokens[i].start)
	}
	return tokens
}

func numberWasConsumed(consumed []numberToken, token numberToken) bool {
	for _, item := range consumed {
		if item.start == token.start && item.end == token.end {
			return true
		}
	}
	return false
}

func isWordBefore(text string, offset int) bool {
	_, size := utf8.DecodeLastRuneInString(text[:offset])
	r, _ := utf8.DecodeLastRuneInString(text[offset-size : offset])
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

func isWordAt(text string, offset int) bool {
	r, _ := utf8.DecodeRuneInString(text[offset:])
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

func assertMovementTable(t *testing.T, movement table, report Report) {
	t.Helper()
	now := -1
	for i, header := range movement.header {
		if header == "Now" {
			now = i
			break
		}
	}
	if now < 0 {
		failAt(t, movement.headerLine, "movement table is missing a Now column")
	}
	if len(movement.header) == 0 || movement.header[0] != "Count" {
		failAt(t, movement.headerLine, "movement table first header: want %q, got %q", "Count", movement.header)
	}
	seen := map[string]bool{}
	for _, row := range movement.rows {
		if isSeparatorRow(row.cells) {
			continue
		}
		if len(row.cells) <= now {
			failAt(t, row.line, "movement table row has %d cells, missing Now column", len(row.cells))
		}
		label := stripMarkdown(row.cells[0])
		if label == "new checks of ours" {
			seen[label] = true
			continue
		}
		if seen[label] {
			failAt(t, row.line, "movement table row %q appears more than once", label)
		}
		seen[label] = true
		if label == "overall: fully agreeing / only ours / our diagnostics" {
			values := strings.Split(stripMarkdown(row.cells[now]), "/")
			if len(values) != 3 {
				failAt(t, row.line, "movement row %q column %q: want three slash-separated counts, got %q", label, movement.header[now], row.cells[now])
			}
			wants := []struct {
				value int
				path  string
			}{
				{report.Totals.FilesAgreeing, "totals.filesFullyAgreeing"},
				{report.Totals.OpenSysMLOnly, "totals.openSysMLOnly"},
				{report.Totals.OpenSysMLTotal, "totals.openSysMLDiagnostics"},
			}
			for i, want := range wants {
				got := parseMovementInteger(t, row.line, strings.TrimSpace(values[i]), movement.header[now])
				if got != want.value {
					failAt(t, row.line, "movement row %q item %d column %q: want %d (baseline %s), got %d", label, i+1, movement.header[now], want.value, want.path, got)
				}
			}
			continue
		}
		want, jsonPath, ok := movementValue(report, label)
		if !ok {
			failAt(t, row.line, "movement table row %q is not mapped or allowlisted", label)
		}
		got := parseCellInteger(t, tableRow{line: row.line, cells: row.cells}, now, "Now")
		if got != want {
			failAt(t, row.line, "movement row %q column %q: want %d (baseline %s), got %d", label, movement.header[now], want, jsonPath, got)
		}
	}
}

func parseMovementInteger(t *testing.T, line int, value, column string) int {
	t.Helper()
	if !integerPattern.MatchString(value) {
		failAt(t, line, "column %q: expected integer, got %q", column, value)
	}
	got, err := strconv.Atoi(value)
	if err != nil {
		failAt(t, line, "column %q: malformed integer %q", column, value)
	}
	return got
}

func movementValue(report Report, label string) (int, string, bool) {
	switch label {
	case "unmapped, our side":
		total := 0
		for _, row := range report.Unmapped {
			if row.Side == "opensysml" {
				total += row.Count
			}
		}
		return total, "unmapped[side=opensysml].count", true
	}
	for _, root := range report.Roots {
		prefix := root.Name + ": "
		if !strings.HasPrefix(label, prefix) {
			continue
		}
		switch strings.TrimPrefix(label, prefix) {
		case "only ours":
			return root.Totals.OpenSysMLOnly, "roots[" + root.Name + "].totals.openSysMLOnly", true
		case "only pilot":
			return root.Totals.PilotOnly, "roots[" + root.Name + "].totals.pilotOnly", true
		case "fully agreeing":
			return root.Totals.FilesAgreeing, "roots[" + root.Name + "].totals.filesFullyAgreeing", true
		}
	}
	return 0, "", false
}

func failAt(t *testing.T, line int, format string, args ...any) {
	t.Helper()
	t.Fatalf("%s:%d: %s", docPath, line, fmt.Sprintf(format, args...))
}
