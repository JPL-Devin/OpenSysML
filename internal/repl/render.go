package repl

import (
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/text/width"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/passes"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

// Result is the outcome of one Submit: the top-level members parsed from the
// accumulated buffer (for the success summary), the names this submission
// declared, and any analysis diagnostics over the whole <repl> document.
type Result struct {
	Members     []ast.Node          // top-level members of the <repl> AST (Task 5 renders these)
	Declared    []string            // names introduced by THIS submission
	Diagnostics []passes.Diagnostic // eager analysis over the whole buffer
	Source      string              // the full joined <repl> content (Task 6 caret rendering)
	Offset      int                 // byte offset in Source where THIS submission begins
	Origins     []Origin            // the files of THIS submission, in buffer order
	Notices     []string            // side effects of the submission, e.g. a debugging session it ended
}

// Origin locates one file of a submission in the buffer, so a diagnostic is
// reported against that file and its own line numbering.
type Origin struct {
	Name   string
	Offset int
}

// locate returns the file a buffer offset belongs to and the offset that file
// starts at. An offset outside the files of this submission belongs to the
// submission as a whole, which is reported without a file name.
func (r Result) locate(offset int) (string, int) {
	for i, o := range r.Origins {
		if offset < o.Offset {
			continue
		}
		if i+1 == len(r.Origins) || offset < r.Origins[i+1].Offset {
			return o.Name, o.Offset
		}
	}
	return "", r.Offset
}

// lineOf is the 1-based buffer line a byte offset falls on.
func (r Result) lineOf(offset int) int {
	if offset > len(r.Source) {
		offset = len(r.Source)
	}
	return strings.Count(r.Source[:offset], "\n") + 1
}

// diagLocation names the file a diagnostic came from and the buffer line that
// file — or, for a prompt submission, the submission — starts on.
func (r Result) diagLocation(offset int) (string, int) {
	name, start := r.locate(offset)
	return name, r.lineOf(start)
}

// mine reports whether a span belongs to the submission just made rather than
// to an earlier one still sitting in the buffer.
func (r Result) mine(span source.Span) bool { return span.Offset >= r.Offset }

// renderSummary returns one accepted line per top-level member: "✓ <kind> <name>".
func renderSummary(members []ast.Node) []string {
	out := make([]string, 0, len(members))
	for _, m := range members {
		if line := renderMember(m); line != "" {
			out = append(out, "✓ "+line)
		}
	}
	return out
}

// renderMember maps a top-level member (possibly wrapped in a Membership) to a
// "<kind> <name>" summary, or "" for members that carry no useful summary.
func renderMember(m ast.Node) string {
	node := m
	if mem, ok := m.(*ast.Membership); ok {
		node = mem.Member
	}
	switch d := node.(type) {
	case *ast.Package:
		return "package " + nameOrAnon(d.Ident)
	case *ast.Namespace:
		return "namespace " + nameOrAnon(d.Ident)
	case *ast.Alias:
		return "alias " + nameOrAnon(d.Ident)
	case *ast.Import:
		return "import " + importTarget(d)
	case *ast.Dependency:
		return "dependency " + nameOrAnon(d.Ident)
	case *ast.Comment:
		return "comment"
	case *ast.Definition:
		return d.Kind.String() + " def " + nameOrAnon(d.Ident)
	case *ast.Usage:
		return d.Kind.String() + " " + nameOrAnon(d.Ident)
	default:
		return ""
	}
}

func nameOrAnon(id ast.Identification) string {
	if id.Name != "" {
		return id.Name
	}
	if id.ShortName != "" {
		return "<" + id.ShortName + ">"
	}
	return "<anonymous>"
}

// importTarget echoes what an import names, wildcards included, so the
// confirmation matches what was typed.
func importTarget(imp *ast.Import) string {
	name := qnString(imp.Imported)
	switch {
	case imp.Kind == ast.ImportNamespace && imp.IsRecursive:
		return name + "::*::**"
	case imp.IsRecursive:
		return name + "::**"
	case imp.Kind == ast.ImportNamespace:
		return name + "::*"
	}
	return name
}

func qnString(qn *ast.QualifiedName) string {
	if qn == nil {
		return "<?>"
	}
	parts := make([]string, len(qn.Parts))
	for i, p := range qn.Parts {
		parts[i] = p.Text
	}
	return strings.Join(parts, "::")
}

// renderDiagnostics formats each diagnostic as a two-line block:
//
//	<line>:<col>: <severity>: <message>
//	    <source line>
//	    <caret span>
//
// locate maps a diagnostic's buffer offset to the file it came from and the
// buffer line that file starts on, so a block names its file and counts lines
// from what the user submitted rather than from the top of the accumulated
// buffer; pass wholeBuffer to number against the buffer instead. When origin is
// set each block also carries the pass that produced the diagnostic.
//
// Columns and carets are counted in printed cells, so a line with multi-byte
// runes before the finding still points at it; the LSP server owns UTF-16
// correctness separately.
func renderDiagnostics(diags []passes.Diagnostic, src string, locate func(offset int) (string, int), origin bool) []string {
	if len(diags) == 0 {
		return nil
	}
	sf := source.New(docName, []byte(src))
	lines := strings.Split(src, "\n")
	var out []string
	for _, d := range diags {
		p := sf.Lines().PosAt(d.Span.Offset)
		file, baseLine := locate(d.Span.Offset)
		where := ""
		if file != "" {
			where = file + ":"
		}
		srcLine := ""
		if p.Line-1 >= 0 && p.Line-1 < len(lines) {
			srcLine = lines[p.Line-1]
		}
		col := p.Col
		if p.Col-1 <= len(srcLine) {
			col = displayWidth(srcLine[:p.Col-1]) + 1
		}
		head := fmt.Sprintf("%s%d:%d: %s: %s", where, p.Line-baseLine+1, col, d.Severity.String(), d.Message)
		if origin {
			head += fmt.Sprintf(" [%s]", diagOrigin(d))
		}
		out = append(out, head)
		if p.Line-1 >= 0 && p.Line-1 < len(lines) {
			out = append(out, srcLine)
			out = append(out, caretLine(srcLine, p.Col-1, d.Span.Len))
		}
	}
	return out
}

// diagOrigin names the pass and code behind a diagnostic, for debug output.
func diagOrigin(d passes.Diagnostic) string {
	parts := make([]string, 0, 2)
	for _, p := range []string{d.Source, d.Code} {
		if p != "" {
			parts = append(parts, p)
		}
	}
	if len(parts) == 0 {
		return "unattributed"
	}
	return strings.Join(parts, "/")
}

// exprError reports msg against a one-line expression the way a declaration
// diagnostic is reported: position, source echo and caret. base is the offset
// the expression starts at in the text span was measured in.
// The reported column counts printed cells, not bytes, so it agrees with the
// caret under the echo.
func exprError(expr, msg string, span source.Span, base int) error {
	start := span.Offset - base
	if start < 0 {
		start = 0
	}
	if start > len(expr) {
		start = len(expr)
	}
	return fmt.Errorf("1:%d: %s\n%s\n%s", displayWidth(expr[:start])+1, msg, expr, caretLine(expr, start, span.Len))
}

// caretLine builds "   ^~~~" under the span starting at byte offset start of
// line, measured in printed cells so multi-byte runes stay aligned.
func caretLine(line string, start, spanLen int) string {
	if start < 0 {
		start = 0
	}
	if start > len(line) {
		start = len(line)
	}
	end := start + spanLen
	if end > len(line) {
		end = len(line)
	}
	width := 0
	if end > start {
		width = displayWidth(line[start:end])
	}
	if width < 1 {
		width = 1
	}
	var b strings.Builder
	b.WriteString(strings.Repeat(" ", displayWidth(line[:start])))
	b.WriteByte('^')
	if width > 1 {
		b.WriteString(strings.Repeat("~", width-1))
	}
	return b.String()
}

// displayWidth is the number of terminal cells s occupies: two for the East
// Asian wide and fullwidth runes, one for everything else, and none for the
// combining marks that render on the rune before them.
func displayWidth(s string) int {
	cells := 0
	for _, r := range s {
		switch {
		case unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r):
		case width.LookupRune(r).Kind() == width.EastAsianWide,
			width.LookupRune(r).Kind() == width.EastAsianFullwidth:
			cells += 2
		default:
			cells++
		}
	}
	return cells
}

// renderResult produces the printable lines for a submission at the given
// verbosity: the notices it caused, the diagnostics that verbosity admits, and
// the summary of what it declared. A submission that failed to analyse gets
// diagnostics instead of a summary, since it declared nothing usable.
func renderResult(r Result, v Verbosity) []string {
	if v >= VerbosityDebug {
		return renderDebug(r)
	}
	diags := scopedDiagnostics(r, v)
	out := append([]string(nil), r.Notices...)
	out = append(out, renderDiagnostics(diags, r.Source, r.diagLocation, false)...)
	if hasError(diags) {
		return out
	}
	// A validation tier is skipped once a lower tier errors anywhere in the
	// buffer, so a clean report on this submission would otherwise read as a
	// full check when the deeper passes never ran.
	if r.analysisBlocked() {
		out = append(out, blockedNote)
	}
	return append(out, renderSummary(r.ownMembers())...)
}

// renderDebug reports everything the analysis produced over the whole buffer,
// at buffer-absolute positions, plus where this submission landed in it.
func renderDebug(r Result) []string {
	out := append([]string(nil), r.Notices...)
	out = append(out, fmt.Sprintf("[debug] submission at buffer line %d; %d diagnostic(s) over the whole buffer",
		r.baseLine(), len(r.Diagnostics)))
	out = append(out, renderDiagnostics(r.Diagnostics, r.Source, wholeBuffer, true)...)
	return append(out, renderSummary(r.Members)...)
}

// scopedDiagnostics keeps the diagnostics of this submission that the verbosity
// admits: errors always, warnings and below only above quiet.
func scopedDiagnostics(r Result, v Verbosity) []passes.Diagnostic {
	var out []passes.Diagnostic
	for _, d := range r.Diagnostics {
		if !r.mine(d.Span) {
			continue
		}
		if d.Severity != passes.SeverityError && v <= VerbosityQuiet {
			continue
		}
		out = append(out, d)
	}
	return out
}

// blockedNote warns that a clean report is not a full check.
const blockedNote = "note: an earlier session error is unresolved, so deeper checks may not have run here (see it with -debug)"

// analysisBlocked reports whether an error outside this submission stopped the
// higher validation tiers from running over it.
func (r Result) analysisBlocked() bool {
	for _, d := range r.Diagnostics {
		if d.Severity == passes.SeverityError && !r.mine(d.Span) {
			return true
		}
	}
	return false
}

func hasError(diags []passes.Diagnostic) bool {
	for _, d := range diags {
		if d.Severity == passes.SeverityError {
			return true
		}
	}
	return false
}

// ownMembers returns the top-level members this submission contributed, so a
// summary does not re-announce everything typed earlier in the session.
func (r Result) ownMembers() []ast.Node {
	out := make([]ast.Node, 0, len(r.Members))
	for _, m := range r.Members {
		if r.mine(m.Span()) {
			out = append(out, m)
		}
	}
	return out
}

// baseLine is the 1-based buffer line this submission starts on.
func (r Result) baseLine() int {
	return r.lineOf(r.Offset)
}

// wholeBuffer numbers diagnostics against the accumulated buffer, naming no file.
func wholeBuffer(int) (string, int) { return "", 1 }

// inFile reports every diagnostic against file, numbering from its first line,
// for a caller that already knows which source it is rendering.
func inFile(file string) func(int) (string, int) {
	return func(int) (string, int) { return file, 1 }
}
