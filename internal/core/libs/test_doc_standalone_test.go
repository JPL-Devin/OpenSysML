package libs

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestDocCommentStandalone(t *testing.T) {
	// Test 1: doc comment before declaration (should work)
	code1 := `package Test {
		doc /* This is a doc */
		feature x;
	}`
	
	p1 := parser.New(source.New("test1.kerml", []byte(code1)))
	_ = p1.ParseFile()
	t.Logf("Test 1 (doc before feature): %d diagnostics", len(p1.Diagnostics))
	for _, d := range p1.Diagnostics {
		t.Logf("  - %s", d.Message)
	}
	
	// Test 2: standalone doc comment (might fail)
	code2 := `package Test {
		doc /* This is a doc */
		
		feature x;
	}`
	
	p2 := parser.New(source.New("test2.kerml", []byte(code2)))
	_ = p2.ParseFile()
	t.Logf("\nTest 2 (doc + blank line + feature): %d diagnostics", len(p2.Diagnostics))
	for _, d := range p2.Diagnostics {
		t.Logf("  - %s", d.Message)
	}
	
	// Test 3: minimal Base.kerml reproduction
	code3 := `standard library package Base {
		doc 
		/*
		 * This package defines the classifiers.
		 */
		 		
		abstract classifier Anything {
		}
	}`
	
	p3 := parser.New(source.New("test3.kerml", []byte(code3)))
	_ = p3.ParseFile()
	t.Logf("\nTest 3 (Base.kerml pattern): %d diagnostics", len(p3.Diagnostics))
	for _, d := range p3.Diagnostics {
		t.Logf("  - offset=%d, %s", d.Span.Offset, d.Message)
	}
}
