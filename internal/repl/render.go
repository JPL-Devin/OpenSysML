package repl

import (
	"fmt"
	"strings"

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
	Notices     []string            // side effects of the submission, e.g. a debugging session it ended
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
// baseLine is the buffer line the reported submission starts on, so reported
// line numbers count from what the user typed rather than from the top of the
// accumulated buffer. Pass 1 to number against the whole buffer. When origin is
// set each block also carries the pass that produced the diagnostic.
//
// Note: byte-column carets assume ASCII/monospace alignment; multi-byte runes
// before the caret will misalign by display width. Acceptable for v1 — the LSP
// server owns UTF-16 correctness; the REPL caret is only a terminal aid.
func renderDiagnostics(diags []passes.Diagnostic, src string, baseLine int, origin bool) []string {
	if len(diags) == 0 {
		return nil
	}
	sf := source.New(docName, []byte(src))
	lines := strings.Split(src, "\n")
	var out []string
	for _, d := range diags {
		p := sf.Lines().PosAt(d.Span.Offset)
		head := fmt.Sprintf("%d:%d: %s: %s", p.Line-baseLine+1, p.Col, d.Severity.String(), d.Message)
		if origin {
			head += fmt.Sprintf(" [%s]", diagOrigin(d))
		}
		out = append(out, head)
		if p.Line-1 >= 0 && p.Line-1 < len(lines) {
			srcLine := lines[p.Line-1]
			out = append(out, srcLine)
			out = append(out, caretLine(p.Col, d.Span.Len, len(srcLine)))
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

// renderExprMessage reports msg against a one-line expression the way a
// declaration diagnostic is reported: position, source echo and caret. base is
// the offset the expression starts at in the text span was measured in.
func renderExprMessage(expr, msg string, span source.Span, base int) []string {
	col := span.Offset - base + 1
	if col < 1 {
		col = 1
	}
	if col > len(expr)+1 {
		col = len(expr) + 1
	}
	return []string{
		fmt.Sprintf("error: 1:%d: %s", col, msg),
		expr,
		caretLine(col, span.Len, len(expr)),
	}
}

// caretLine builds "   ^~~~" with (col-1) leading spaces and a caret span of
// width max(1, spanLen), clamped so it never runs past the source line.
func caretLine(col, spanLen, lineLen int) string {
	if col < 1 {
		col = 1
	}
	lead := col - 1
	width := spanLen
	if width < 1 {
		width = 1
	}
	if lead+width > lineLen {
		width = lineLen - lead
		if width < 1 {
			width = 1
		}
	}
	var b strings.Builder
	b.WriteString(strings.Repeat(" ", lead))
	b.WriteByte('^')
	if width > 1 {
		b.WriteString(strings.Repeat("~", width-1))
	}
	return b.String()
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
	out = append(out, renderDiagnostics(diags, r.Source, r.baseLine(), false)...)
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
	out = append(out, renderDiagnostics(r.Diagnostics, r.Source, 1, true)...)
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
	return strings.Count(r.Source[:r.Offset], "\n") + 1
}
