package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

func TestLexerIncludes(t *testing.T) {
	input := `includes(x, y)`
	src := source.New("test.kerml", []byte(input))
	lex := lexer.New(src)

	for {
		tok := lex.Next()
		t.Logf("Token: kind=%v, text=%q", tok.Kind, src.Text(tok.Span))
		if tok.Kind == lexer.EOF {
			break
		}
		if tok.Kind == lexer.Keyword {
			t.Logf("  Keyword ID: %s", tok.KeywordID)
		}
	}
}
