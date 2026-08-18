package repl

import (
	"strings"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/lexer"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
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
	if !ok || ref.Name == nil || ref.Name.Global || len(ref.Name.Parts) == 0 {
		return "", false
	}
	segments := make([]string, 0, len(ref.Name.Parts))
	for _, part := range ref.Name.Parts {
		if part.Text == "" {
			return "", false
		}
		segments = append(segments, part.Text)
	}
	return strings.Join(segments, "::"), true
}

// notationName is a qualified name spelled as the notation writes it, so a name
// the prompt prints can be typed back into a command. It is the one rule every
// surface quotes with, `%render` included.
func notationName(fqn string) string {
	return lexer.QualifiedNameText(fqn)
}
