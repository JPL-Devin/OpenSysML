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
}

// renderSummary returns one summary line per top-level member: "<kind> <name>".
func renderSummary(members []ast.Node) []string {
	out := make([]string, 0, len(members))
	for _, m := range members {
		if line := renderMember(m); line != "" {
			out = append(out, line)
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
		return "import " + qnString(d.Imported)
	case *ast.Dependency:
		return "dependency " + nameOrAnon(d.Ident)
	case *ast.Comment:
		return "comment"
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
// Note: byte-column carets assume ASCII/monospace alignment; multi-byte runes
// before the caret will misalign by display width. Acceptable for v1 — the LSP
// server owns UTF-16 correctness; the REPL caret is only a terminal aid.
func renderDiagnostics(diags []passes.Diagnostic, src string) []string {
	if len(diags) == 0 {
		return nil
	}
	sf := source.New(docName, []byte(src))
	lines := strings.Split(src, "\n")
	var out []string
	for _, d := range diags {
		p := sf.Lines().PosAt(d.Span.Offset)
		out = append(out, fmt.Sprintf("%d:%d: %s: %s", p.Line, p.Col, d.Severity.String(), d.Message))
		if p.Line-1 >= 0 && p.Line-1 < len(lines) {
			srcLine := lines[p.Line-1]
			out = append(out, srcLine)
			out = append(out, caretLine(p.Col, d.Span.Len, len(srcLine)))
		}
	}
	return out
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
