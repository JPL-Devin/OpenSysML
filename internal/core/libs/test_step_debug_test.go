package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

func TestStepDebug(t *testing.T) {
	input := `
package Test {
	action A {
		step entry;
	}
}
`
	p := parser.New(source.New("test.kerml", []byte(input)))
	_ = p.ParseFile()
	t.Logf("Total diagnostics: %d", len(p.Diagnostics))
	for i, d := range p.Diagnostics {
		t.Logf("  [%d] %s at offset %d", i, d.Message, d.Span.Offset)
	}
}

func TestStepWithMult(t *testing.T) {
	input := `
package Test {
	action A {
		step entry[1];
	}
}
`
	p := parser.New(source.New("test.kerml", []byte(input)))
	_ = p.ParseFile()
	t.Logf("Total diagnostics: %d", len(p.Diagnostics))
	for i, d := range p.Diagnostics {
		t.Logf("  [%d] %s at offset %d", i, d.Message, d.Span.Offset)
	}
}
