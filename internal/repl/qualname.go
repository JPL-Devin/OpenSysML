package repl

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// plainName is a name spelled as the symbol index registers it: the notation
// parsed, so a quoted segment ('My Pkg') keeps its text and loses its quotes.
// It reports false for text that is no name, which the caller then uses as
// typed.
func plainName(text string) (string, bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", false
	}
	p := parser.New(source.New("name", []byte(text)))
	expr := p.ParseExpression()
	if len(p.Diagnostics) > 0 || p.Offset() != len(text) {
		return "", false
	}
	ref, ok := expr.(*ast.FeatureReference)
	if !ok || ref.Name == nil || ref.Name.Global {
		return "", false
	}
	name := qualifiedText(ref.Name)
	return name, name != ""
}

// qualifiedText spells a qualified name as the index registers it, "" for one
// with an empty segment.
func qualifiedText(qn *ast.QualifiedName) string {
	if qn == nil || len(qn.Parts) == 0 {
		return ""
	}
	segments := make([]string, 0, len(qn.Parts))
	for _, part := range qn.Parts {
		if part.Text == "" {
			return ""
		}
		segments = append(segments, part.Text)
	}
	return strings.Join(segments, "::")
}

// notationName is a qualified name spelled as the notation writes it, so a name
// the prompt prints can be typed back into a command. It is the one rule every
// surface quotes with, `%render` included.
func notationName(fqn string) string {
	return lexer.QualifiedNameText(fqn)
}
