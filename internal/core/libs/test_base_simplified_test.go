package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

func TestBaseSimplified(t *testing.T) {
	// Simplified - no nested doc
	code1 := `standard library package Base {
	doc 
	/*
	 * Package comment
	 */
	 		
	abstract classifier Anything {
	}
}`

	p1 := parser.New(source.New("test1.kerml", []byte(code1)))
	_ = p1.ParseFile()
	t.Logf("Test 1 (no nested doc): %d diagnostics", len(p1.Diagnostics))
	for _, d := range p1.Diagnostics {
		t.Logf("  - offset=%d, %s", d.Span.Offset, d.Message)
	}

	// With nested doc like real Base.kerml
	code2 := `standard library package Base {
	doc 
	/*
	 * Package comment
	 */
	 		
	abstract classifier Anything {
		doc
		/*
		 * Anything comment
		 */
	}
}`

	p2 := parser.New(source.New("test2.kerml", []byte(code2)))
	_ = p2.ParseFile()
	t.Logf("\nTest 2 (with nested doc): %d diagnostics", len(p2.Diagnostics))
	for _, d := range p2.Diagnostics {
		t.Logf("  - offset=%d, %s", d.Span.Offset, d.Message)
	}
}
