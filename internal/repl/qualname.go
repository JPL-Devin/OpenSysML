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

// notationName is a qualified name spelled as the notation writes it, quoting
// every segment that is not a plain identifier so a name the prompt prints can
// be typed back into a command.
func notationName(fqn string) string {
	if fqn == "" {
		return fqn
	}
	segments := strings.Split(fqn, "::")
	for i, segment := range segments {
		if !isPlainIdentifier(segment) {
			// The text came from the notation, whose escapes it still carries, so
			// quoting it is the exact inverse of parsing it.
			segments[i] = "'" + segment + "'"
		}
	}
	return strings.Join(segments, "::")
}

// isPlainIdentifier reports whether a name segment can be written unquoted: one
// identifier token and nothing else, so a keyword or a name holding a space,
// punctuation or nothing at all needs quoting.
func isPlainIdentifier(segment string) bool {
	if segment == "" {
		return false
	}
	lx := lexer.New(source.New("name", []byte(segment)))
	tok := lx.Next()
	if tok.Kind != lexer.Identifier || tok.Span.Offset != 0 || tok.Span.Len != len(segment) {
		return false
	}
	return lx.Next().Kind == lexer.EOF
}
