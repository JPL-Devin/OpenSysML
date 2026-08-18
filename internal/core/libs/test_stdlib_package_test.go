package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

func TestStandardLibraryPackage(t *testing.T) {
	// Test with "standard library package"
	code1 := `standard library package Base {
		doc 
		/*
		 * This package defines the classifiers.
		 */
		 		
		abstract classifier Anything {
		}
	}`

	p1 := parser.New(source.New("test1.kerml", []byte(code1)))
	_ = p1.ParseFile()
	t.Logf("Test 1 (standard library package): %d diagnostics", len(p1.Diagnostics))
	for _, d := range p1.Diagnostics {
		t.Logf("  - offset=%d, %s", d.Span.Offset, d.Message)
	}

	// Test without "standard library"
	code2 := `package Base {
		doc 
		/*
		 * This package defines the classifiers.
		 */
		 		
		abstract classifier Anything {
		}
	}`

	p2 := parser.New(source.New("test2.kerml", []byte(code2)))
	_ = p2.ParseFile()
	t.Logf("\nTest 2 (package without standard library): %d diagnostics", len(p2.Diagnostics))
	for _, d := range p2.Diagnostics {
		t.Logf("  - offset=%d, %s", d.Span.Offset, d.Message)
	}
}
