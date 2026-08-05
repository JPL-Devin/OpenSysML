package libs

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestDocCommentWhitespace(t *testing.T) {
	// Test 1: tabs only (should work)
	code1 := `package Test {
		doc 
		/*
		 * Comment
		 */
		
		feature x;
	}`

	p1 := parser.New(source.New("test1.kerml", []byte(code1)))
	_ = p1.ParseFile()
	t.Logf("Test 1 (tabs only): %d diagnostics", len(p1.Diagnostics))

	// Test 2: space+tabs (Base.kerml pattern)
	code2 := `package Test {
		doc 
		/*
		 * Comment
		 */
		 		
		feature x;
	}`

	p2 := parser.New(source.New("test2.kerml", []byte(code2)))
	_ = p2.ParseFile()
	t.Logf("Test 2 (space+tabs like Base.kerml): %d diagnostics", len(p2.Diagnostics))
	for _, d := range p2.Diagnostics {
		t.Logf("  - offset=%d, %s", d.Span.Offset, d.Message)
	}
}
