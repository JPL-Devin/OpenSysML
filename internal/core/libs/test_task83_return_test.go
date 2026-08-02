package libs

import (
	"testing"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestTask83ReturnDeclaration(t *testing.T) {
	// Test 1: return with assignment (already works)
	input1 := `package Test {
		bool test1 {
			return result = 5;
		}
	}`
	
	p1 := parser.New(source.New("test1.kerml", []byte(input1)))
	_ = p1.ParseFile()
	t.Logf("Test 1 (return with assignment) - Diagnostics: %d", len(p1.Diagnostics))
	
	// Test 2: return with typing only (fails)
	input2 := `package Test {
		predicate IncomingTransferSort {
			in t1: Transfer [1];
			return t1First: Boolean [1];
		}
	}`
	
	p2 := parser.New(source.New("test2.kerml", []byte(input2)))
	_ = p2.ParseFile()
	t.Logf("Test 2 (return with typing only) - Diagnostics: %d", len(p2.Diagnostics))
	for _, d := range p2.Diagnostics {
		t.Logf("  - offset=%d: %s", d.Span.Offset, d.Message)
	}
	
	if len(p2.Diagnostics) > 0 {
		t.Errorf("Expected clean parse for return declaration, got %d errors", len(p2.Diagnostics))
	}
}
