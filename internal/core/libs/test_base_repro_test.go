package libs

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestBaseReproduction(t *testing.T) {
	// Exact first 10 lines from Base.kerml
	code := `standard library package Base {
	doc 
	/*
	 * This package defines the classifiers and features that provide the bases for the typing
	 * of all elements in the language.
	 */
	 		
	abstract classifier Anything {
		doc
		/*
		 * Comment
		 */
	}
}`

	p := parser.New(source.New("base.kerml", []byte(code)))
	_ = p.ParseFile()
	t.Logf("Diagnostics: %d", len(p.Diagnostics))
	for _, d := range p.Diagnostics {
		t.Logf("  - offset=%d, %s", d.Span.Offset, d.Message)

		offset := d.Span.Offset
		start := offset - 20
		if start < 0 {
			start = 0
		}
		end := offset + 20
		if end > len(code) {
			end = len(code)
		}
		ctx := []byte(code)[start:end]
		t.Logf("    Context: %q", ctx)
	}
}
