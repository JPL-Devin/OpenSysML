package libs

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestDocWhitespaceVariants(t *testing.T) {
	// With whitespace line after doc
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
	t.Logf("Test 1 (with whitespace line): %d diagnostics", len(p1.Diagnostics))
	
	// Without whitespace line after doc
	code2 := `standard library package Base {
	doc 
	/*
	 * Package comment
	 */
	abstract classifier Anything {
	}
}`
	
	p2 := parser.New(source.New("test2.kerml", []byte(code2)))
	_ = p2.ParseFile()
	t.Logf("Test 2 (no whitespace line): %d diagnostics", len(p2.Diagnostics))
	
	// Without doc at all
	code3 := `standard library package Base {
	abstract classifier Anything {
	}
}`
	
	p3 := parser.New(source.New("test3.kerml", []byte(code3)))
	_ = p3.ParseFile()
	t.Logf("Test 3 (no doc): %d diagnostics", len(p3.Diagnostics))
}
