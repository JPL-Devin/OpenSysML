package libs

import (
	"github.com/Open-MBEE/Systemica/internal/core/lexer"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"testing"
)

func TestDebugAnonSuccession(t *testing.T) {
	code := `succession [1] do.startShot then [*] nonDoMiddle.startShot;`

	file := source.New("test.kerml", []byte(code))
	l := lexer.New(file)

	t.Log("Token stream:")
	for i := 0; i < 30; i++ {
		tok := l.Next()
		t.Logf("  [%d] %s (%v)", i, tok.Kind, tok.KeywordID)
		if tok.Kind == lexer.EOF {
			break
		}
	}
}
