package main

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/model"
	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Verdicts. Agreement is strict: the declared severity at the declared offset,
// with the declared message. Tolerance records what a weaker rule would have
// accepted, and never turns a disagreement into an agreement.
const (
	verdictAgree          = "agree"
	verdictDisagree       = "disagree"
	verdictUnlocated      = "unlocated"
	verdictNotAdjudicated = "not-adjudicated"
)

// Tolerances, reported as secondary columns beside a strict disagreement.
const (
	toleranceNone     = ""
	toleranceMessage  = "same-location"     // right severity and offset, other wording
	toleranceLine     = "same-line"         // right severity on the declared line
	toleranceSeverity = "severity-differs"  // a diagnostic there, of the other severity
	toleranceAnywhere = "elsewhere-in-file" // right severity, but not near the declaration
)

// row is one adjudicated expectation: one item of an errors/warnings assertion,
// or one whole noErrors/linkedName assertion.
type row struct {
	Kind string `json:"kind"`
	// Block records a `//* ... */` note, which the brief's line-anchored census
	// of the suites does not count.
	Block     bool   `json:"block,omitempty"`
	Line      int    `json:"line"`
	At        string `json:"at,omitempty"`
	Verdict   string `json:"verdict"`
	Tolerance string `json:"tolerance,omitempty"`
	Declared  string `json:"declared,omitempty"`
	Actual    string `json:"ours,omitempty"`
}

// fileResult is one .xt file's adjudication.
type fileResult struct {
	Path       string `json:"path"`
	SetupClass string `json:"setupClass"`
	// Expectations and Agree summarize Rows, which the report prunes to the
	// rows that are not agreements.
	Expectations int      `json:"expectations"`
	Agree        int      `json:"agree"`
	Rows         []row    `json:"rows,omitempty"`
	Problems     []string `json:"problems,omitempty"`
	// Ignored lists XPECT-shaped text that opens no note, so it is not run.
	Ignored []string `json:"ignored,omitempty"`
	// Missing lists declared resources that are absent from the download.
	Missing []string `json:"missing,omitempty"`
}

// diag is one of our diagnostics, reduced to what an assertion declares.
type diag struct {
	Offset   int
	Line     int
	Severity string
	Message  string
}

// compareFile loads the resource set the file's setup declares and adjudicates
// every assertion in it.
func compareFile(suiteDir string, f xtFile) fileResult {
	res := fileResult{Path: f.Path, SetupClass: f.SetupClass, Problems: f.Problems, Ignored: f.Ignored}
	if len(f.Problems) > 0 {
		return res
	}

	ws := model.NewWorkspace()
	main := strings.TrimSuffix(f.Path, ".xt")
	for _, r := range f.Resources {
		if r.ThisFile || isLibrary(r.From) {
			continue
		}
		// A declared path is either project-relative (`/a/b.kerml`) or beside
		// the .xt file itself.
		rel := strings.TrimPrefix(r.From, "/")
		if !strings.HasPrefix(r.From, "/") {
			rel = path.Join(path.Dir(f.Path), r.From)
		}
		// #nosec G304 -- the suite directory is named on the command line.
		content, err := os.ReadFile(filepath.Join(suiteDir, filepath.FromSlash(rel)))
		if err != nil {
			res.Missing = append(res.Missing, r.From)
			continue
		}
		ws.Open(rel, content, 1)
	}
	ws.Open(main, f.Content, 1)
	sort.Strings(res.Missing)
	res.Missing = dedupe(res.Missing)

	src := squeeze(f.Masked)
	lines := source.New(main, f.Content).Lines()
	var diags []diag
	for _, d := range ws.Diagnostics(main) {
		diags = append(diags, diag{
			Offset:   d.Span.Offset,
			Line:     lines.PosAt(d.Span.Offset).Line,
			Severity: severityName(d.Severity),
			Message:  d.Message,
		})
	}

	for _, a := range f.Assertions {
		res.Rows = append(res.Rows, adjudicate(ws, main, f, a, diags, lines, src)...)
	}
	return res
}

// adjudicate turns one assertion into its rows.
func adjudicate(ws *model.Workspace, main string, f xtFile, a assertion, diags []diag, lines *source.LineIndex, src squeezed) []row {
	switch a.Kind {
	case kindErrors, kindWarnings:
		want := "error"
		if a.Kind == kindWarnings {
			want = "warning"
		}
		rows := make([]row, 0, len(a.Expect))
		for _, item := range a.Expect {
			rows = append(rows, diagnosticRow(a, item, want, diags, lines, src))

		}
		return rows
	case kindNoErrors:
		var errs []diag
		for _, d := range diags {
			if d.Severity == "error" {
				errs = append(errs, d)
			}
		}
		r := row{Kind: a.Kind, Block: a.Block, Line: a.Line, Verdict: verdictAgree, Declared: "no error anywhere in the file"}
		if len(errs) > 0 {
			r.Verdict = verdictDisagree
			r.Actual = fmt.Sprintf("%d error(s), first: line %d: %s", len(errs), errs[0].Line, errs[0].Message)
		}
		return []row{r}
	case kindLinkedName:
		return []row{linkedNameRow(ws, main, a, src)}
	default:
		return []row{{
			Kind: a.Kind, Block: a.Block, Line: a.Line, At: a.At, Verdict: verdictNotAdjudicated,
			Declared: a.Expected,
			Actual:   fmt.Sprintf("XPECT %s is read but not adjudicated by this harness", a.Kind),
		}}
	}
}

// diagnosticRow adjudicates one declared diagnostic.
func diagnosticRow(a assertion, item expectation, want string, diags []diag, lines *source.LineIndex, src squeezed) row {
	r := row{Kind: a.Kind, Block: a.Block, Line: a.Line, At: item.At, Declared: fmt.Sprintf("%s: %q", want, item.Message)}

	offset, _, ok := src.locate(a.Region, item.At)
	if !ok {
		r.Verdict = verdictUnlocated
		r.Actual = fmt.Sprintf("the declared text %q does not occur after the assertion", item.At)
		return r
	}
	line := lines.PosAt(offset).Line

	var atOffset, atLine, elsewhere, otherSeverity []diag
	for _, d := range diags {
		if d.Severity != want {
			if d.Offset == offset || d.Line == line {
				otherSeverity = append(otherSeverity, d)
			}
			continue
		}
		switch {
		case d.Offset == offset:
			atOffset = append(atOffset, d)
		case d.Line == line:
			atLine = append(atLine, d)
		default:
			elsewhere = append(elsewhere, d)
		}
	}

	for _, d := range atOffset {
		if sameMessage(d.Message, item.Message) {
			r.Verdict = verdictAgree
			r.Actual = fmt.Sprintf("line %d offset %d: %s", d.Line, d.Offset, d.Message)
			return r
		}
	}
	r.Verdict = verdictDisagree
	switch {
	case len(atOffset) > 0:
		r.Tolerance = toleranceMessage
		r.Actual = fmt.Sprintf("line %d offset %d: %s", atOffset[0].Line, atOffset[0].Offset, atOffset[0].Message)
	case len(atLine) > 0:
		r.Tolerance = toleranceLine
		r.Actual = fmt.Sprintf("line %d offset %d: %s", atLine[0].Line, atLine[0].Offset, atLine[0].Message)
	case len(otherSeverity) > 0:
		r.Tolerance = toleranceSeverity
		r.Actual = fmt.Sprintf("no %s at line %d, but a %s: line %d offset %d: %s", want, line,
			otherSeverity[0].Severity, otherSeverity[0].Line, otherSeverity[0].Offset, otherSeverity[0].Message)
	case len(elsewhere) > 0:
		r.Tolerance = toleranceAnywhere
		r.Actual = fmt.Sprintf("no %s at line %d; nearest is line %d: %s", want, line, elsewhere[0].Line, elsewhere[0].Message)
	default:
		r.Actual = fmt.Sprintf("no %s anywhere in the file", want)
	}
	return r
}

// linkedNameRow resolves the reference the assertion points at and compares its
// qualified name with the declared one.
func linkedNameRow(ws *model.Workspace, main string, a assertion, src squeezed) row {
	r := row{Kind: a.Kind, Block: a.Block, Line: a.Line, At: a.At, Declared: a.Expected}

	offset, end, ok := src.locate(a.Region, a.At)
	if !ok {
		r.Verdict = verdictUnlocated
		r.Actual = fmt.Sprintf("the declared text %q does not occur after the assertion", a.At)
		return r
	}

	doc := ws.Document(main)
	if doc == nil {
		r.Verdict = verdictUnlocated
		r.Actual = "the file did not parse into a document"
		return r
	}
	ref, part, ok := referenceAt(resolve.References(doc.AST, doc.Scope), offset, end)
	if !ok {
		// The text is there but we index no reference at it: either we do not
		// parse the construct or we do not treat it as a name reference.
		r.Verdict = verdictDisagree
		r.Actual = fmt.Sprintf("we index no name reference at offset %d", offset)
		return r
	}

	var sym *symbols.Symbol
	if part == len(ref.QN.Parts)-1 {
		sym, _ = ws.ResolveReferenceInDoc(main, ref)
	} else if segs := ws.ResolveReferenceSegmentsInDoc(main, ref); part < len(segs) {
		sym = segs[part]
	}
	if sym == nil {
		r.Verdict = verdictDisagree
		r.Actual = "unresolved"
		return r
	}

	actual := dotted(symbols.FQNOf(sym))
	r.Actual = actual
	r.Verdict = verdictDisagree
	if actual == a.Expected {
		r.Verdict = verdictAgree
	}
	return r
}

// referenceAt picks the segment a declared `at` text covers: the assertion
// names a whole qualified name, so the segment it asks about is the last one
// ending inside [offset, end). Sorting keeps the choice deterministic when a
// name is nested in another reference's chain.
func referenceAt(refs []resolve.Reference, offset, end int) (resolve.Reference, int, bool) {
	type candidate struct {
		ref  resolve.Reference
		part int
	}
	var found []candidate
	for _, ref := range refs {
		if ref.QN == nil || len(ref.QN.Parts) == 0 {
			continue
		}
		if ref.QN.Parts[0].Span.Offset != offset {
			for i, seg := range ref.QN.Parts {
				if seg.Span.Offset == offset && seg.Span.End() >= end {
					found = append(found, candidate{ref, i})
				}
			}
			continue
		}
		last := -1
		for i, seg := range ref.QN.Parts {
			if seg.Span.End() <= end {
				last = i
			}
		}
		if last >= 0 {
			found = append(found, candidate{ref, last})
		}
	}
	if len(found) == 0 {
		return resolve.Reference{}, 0, false
	}
	sort.SliceStable(found, func(i, j int) bool {
		a, b := found[i], found[j]
		if a.ref.QN.Span().Offset != b.ref.QN.Span().Offset {
			return a.ref.QN.Span().Offset < b.ref.QN.Span().Offset
		}
		if len(a.ref.QN.Parts) != len(b.ref.QN.Parts) {
			return len(a.ref.QN.Parts) > len(b.ref.QN.Parts)
		}
		return a.part < b.part
	})
	return found[0].ref, found[0].part, true
}

// squeezed is model source with the notes blanked out and all whitespace
// dropped, keeping each kept byte's original offset. Declared target texts are
// matched against it because the suites space them freely: `attribute un : A::p;`
// is declared for the source `attribute un: A::p;`.
type squeezed struct {
	Original []byte
	Text     string
	Offsets  []int
}

func squeeze(content []byte) squeezed {
	s := squeezed{Original: content, Offsets: make([]int, 0, len(content))}
	var b strings.Builder
	for i := 0; i < len(content); i++ {
		if isSpace(content[i]) {
			continue
		}
		b.WriteByte(content[i])
		s.Offsets = append(s.Offsets, i)
	}
	s.Text = b.String()
	return s
}

// locate returns the original span of the first occurrence of text at or after
// from. An occurrence inside a longer identifier does not count.
func (s squeezed) locate(from int, text string) (int, int, bool) {
	start := sort.SearchInts(s.Offsets, from)
	if text == "" {
		// No `at` clause: the assertion targets the source it precedes.
		if start >= len(s.Offsets) {
			return 0, 0, false
		}
		return s.Offsets[start], s.Offsets[start] + 1, true
	}
	want := strings.Join(strings.FieldsFunc(text, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r'
	}), "")
	if want == "" {
		return 0, 0, false
	}
	for i := start; i+len(want) <= len(s.Text); i++ {
		if s.Text[i:i+len(want)] != want {
			continue
		}
		begin, end := s.Offsets[i], s.Offsets[i+len(want)-1]+1
		if identChar(want[0]) && begin > 0 && identChar(s.Original[begin-1]) {
			continue
		}
		if identChar(want[len(want)-1]) && end < len(s.Original) && identChar(s.Original[end]) {
			continue
		}
		return begin, end, true
	}
	return 0, 0, false
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

func identChar(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

// isLibrary reports whether a declared resource is one of the suite's copies of
// the standard library, which our workspace already has embedded.
func isLibrary(from string) bool {
	return strings.HasPrefix(from, "/library")
}

// sameMessage compares a declared message with ours, ignoring whitespace and
// the trailing period the pilot's messages carry.
func sameMessage(ours, declared string) bool {
	norm := func(s string) string {
		return strings.TrimSuffix(strings.Join(strings.Fields(s), " "), ".")
	}
	return norm(ours) == norm(declared)
}

// dotted rewrites our `::`-separated qualified name in the pilot's notation.
func dotted(fqn string) string {
	return strings.ReplaceAll(fqn, "::", ".")
}

func severityName(s passes.Severity) string {
	return strings.ToLower(s.String())
}

func dedupe(in []string) []string {
	var out []string
	for i, s := range in {
		if i == 0 || in[i-1] != s {
			out = append(out, s)
		}
	}
	return out
}
