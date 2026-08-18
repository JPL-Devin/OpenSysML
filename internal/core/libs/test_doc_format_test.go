package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

func TestDocCommentFormat(t *testing.T) {
	// Test 1: inline comment
	code1 := `package Test {
		doc /* inline comment */
		
		feature x;
	}`

	p1 := parser.New(source.New("test1.kerml", []byte(code1)))
	_ = p1.ParseFile()
	t.Logf("Test 1 (inline comment): %d diagnostics", len(p1.Diagnostics))
	for _, d := range p1.Diagnostics {
		t.Logf("  - %s", d.Message)
	}

	// Test 2: multi-line comment
	code2 := `package Test {
		doc 
		/*
		 * Multi-line comment
		 */
		
		feature x;
	}`

	p2 := parser.New(source.New("test2.kerml", []byte(code2)))
	_ = p2.ParseFile()
	t.Logf("\nTest 2 (multi-line comment): %d diagnostics", len(p2.Diagnostics))
	for _, d := range p2.Diagnostics {
		t.Logf("  - offset=%d, %s", d.Span.Offset, d.Message)
	}
}
