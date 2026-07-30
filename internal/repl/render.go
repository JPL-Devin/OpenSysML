package repl

import (
	"strings"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/passes"
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
