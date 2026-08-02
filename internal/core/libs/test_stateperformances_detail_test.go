package libs

import (
	"fmt"
	"testing"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestStatePerformancesDetail(t *testing.T) {
	filename := "Kernel Libraries/Kernel Semantic Library/StatePerformances.kerml"
	
	src := &embedSource{}
	data, err := src.Read(filename)
	if err != nil {
		t.Fatalf("Failed to load %s: %v", filename, err)
	}
	
	p := parser.New(source.New(filename, data))
	root := p.ParseFile()
	
	if root == nil {
		t.Fatal("ParseFile returned nil")
	}
	
	fmt.Printf("StatePerformances.kerml diagnostics: %d\n", len(p.Diagnostics))
	for _, d := range p.Diagnostics {
		fmt.Printf("  offset %d: %s\n", d.Span.Offset, d.Message)
	}
	
	if len(p.Diagnostics) > 0 {
		t.Errorf("%d diagnostics (expected 0 after feature chain connector fix)", len(p.Diagnostics))
	}
}
