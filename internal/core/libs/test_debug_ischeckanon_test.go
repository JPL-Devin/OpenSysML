package libs

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"testing"
)

func TestDebugIsAnonCheck(t *testing.T) {
	code := `
		state TestState {
			succession [1] do.startShot then [*] nonDoMiddle.startShot;
		}
	`

	file := source.New("test.kerml", []byte(code))
	p := parser.New(file)
	root := p.ParseFile()

	if root == nil {
		t.Fatal("ParseFile returned nil")
	}

	t.Log("Diagnostics:")
	for _, d := range p.Diagnostics {
		t.Logf("  offset %d: %s", d.Span.Offset, d.Message)
	}

	if len(p.Diagnostics) > 0 {
		t.Fatal("Still has errors")
	}
}
