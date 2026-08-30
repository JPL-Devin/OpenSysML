package lsp

import (
	"context"
	"strings"

	"go.lsp.dev/protocol"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Hover returns type/kind information for the declaration under the cursor.
func (s *Server) Hover(ctx context.Context, params *protocol.HoverParams) (*protocol.Hover, error) {
	name := uriToName(params.TextDocument.URI)
	doc := s.ws.Document(name)
	if doc == nil || doc.Scope == nil {
		return nil, nil
	}
	content := doc.Content
	offset := positionToOffset(content, params.Position)
	sym := symbolAtOffset(doc.Scope, offset)
	if sym == nil {
		return nil, nil
	}

	signature := sym.Notation()
	if sym.Name != "" {
		signature += " " + lexer.NameText(sym.Name)
	}
	// A metadata body declaration implicitly redefines a feature of the
	// annotation's metadata definition (KerML 7.4.7); name it and its type.
	if isMetadataBodyScope(sym.OwnerScope) {
		if target, fqn, ok := s.ws.MetadataBodyRedefines(sym); ok {
			signature += " redefines " + fqn
			if t := declaredTypeText(target); t != "" {
				signature += " : " + t
			}
		}
	}
	comments := leadingDocComments(content, sym.LeadingTrivia)

	rng := spanToRange(content, sym.DeclSpan)
	return &protocol.Hover{
		Contents: s.hoverContents(signature, comments),
		Range:    &rng,
	}, nil
}

// hoverContents renders the hover as Markdown when the client supports it,
// plain text otherwise.
func (s *Server) hoverContents(signature string, comments []string) protocol.MarkupContent {
	if s.wantsMarkdownHover() {
		var b strings.Builder
		b.WriteString("```sysml\n")
		b.WriteString(signature)
		b.WriteString("\n```")
		if prose := docCommentProse(comments); prose != "" {
			b.WriteString("\n\n")
			b.WriteString(prose)
		}
		return protocol.MarkupContent{Kind: protocol.Markdown, Value: b.String()}
	}

	value := signature
	if doc := strings.Join(comments, "\n"); doc != "" {
		value += "\n\n" + doc
	}
	return protocol.MarkupContent{Kind: protocol.PlainText, Value: value}
}

// docCommentProse strips the delimiters and per-line decoration from each doc
// comment so it renders as Markdown prose rather than as source. Comments are
// separate paragraphs; the lines within one keep the breaks they were written
// with, which Markdown would otherwise fold into a single line.
func docCommentProse(comments []string) string {
	var paragraphs []string
	for _, comment := range comments {
		if prose := commentBody(comment); prose != "" {
			paragraphs = append(paragraphs, strings.ReplaceAll(prose, "\n", "  \n"))
		}
	}
	return strings.Join(paragraphs, "\n\n")
}

// commentBody is one comment's text without its delimiters and per-line
// decoration.
func commentBody(comment string) string {
	comment = strings.TrimSpace(comment)
	for _, open := range []string{"//*", "/*"} {
		if rest, ok := strings.CutPrefix(comment, open); ok {
			comment = strings.TrimSuffix(rest, "*/")
			break
		}
	}
	var lines []string
	for _, line := range strings.Split(comment, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "//")
		// A doubled star opens Markdown emphasis the author wrote; a single one
		// is the decoration a block comment runs down its left edge.
		if !strings.HasPrefix(line, "**") {
			line = strings.TrimPrefix(line, "*")
		}
		lines = append(lines, strings.TrimSpace(line))
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// symbolAtOffset finds the innermost symbol whose DeclSpan contains offset.
// Nested scopes are searched first, including those an anonymous declaration
// owns and those no symbol owns at all — a loop body or the parameters of a
// body expression.
func symbolAtOffset(scope *symbols.Scope, offset int) *symbols.Symbol {
	for _, child := range scope.Children() {
		node := child.Node()
		if node == nil {
			continue
		}
		sp := node.Span()
		if offset < sp.Offset || offset >= sp.End() {
			continue
		}
		if inner := symbolAtOffset(child, offset); inner != nil {
			return inner
		}
	}
	for _, sym := range scope.Members() {
		sp := sym.DeclSpan
		if offset >= sp.Offset && offset < sp.End() {
			return sym
		}
	}
	return nil
}

// leadingDocComments returns the text of each comment/note trivia preceding a
// declaration, kept apart so each keeps its own delimiters.
func leadingDocComments(content []byte, trivia []ast.Trivia) []string {
	if len(trivia) == 0 {
		return nil
	}
	var parts []string
	for _, tr := range trivia {
		switch tr.Kind {
		case ast.TriviaComment, ast.TriviaBlockNote, ast.TriviaLineNote:
			start, end := tr.Span.Offset, tr.Span.End()
			if start < 0 || end > len(content) || start > end {
				continue
			}
			if text := strings.TrimSpace(string(content[start:end])); text != "" {
				parts = append(parts, text)
			}
		}
	}
	return parts
}
