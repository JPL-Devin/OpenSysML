package libs

import (
	"testing"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestSubsetStatement(t *testing.T) {
	input := `
		feature TimeEnclosed {
			feature laterOccurrence: Occurrence[1] subsets self;
			subset laterOccurrence.successors subsets earlierOccurrence.successors;
		}
	`
	
	p := parser.New(source.New("test.kerml", []byte(input)))
	root := p.ParseFile()
	
	if len(p.Diagnostics) > 0 {
		t.Errorf("Expected no diagnostics, got %d:", len(p.Diagnostics))
		for _, d := range p.Diagnostics {
			t.Logf("  %s at offset %d", d.Message, d.Span.Offset)
		}
	}
	
	if root == nil {
		t.Fatal("root is nil")
	}
}

func TestDisjointStatement(t *testing.T) {
	input := `
		feature TimeDuring {
			disjoint earlierOccurrence.successors from laterOccurrence.predecessors;
		}
	`
	
	p := parser.New(source.New("test.kerml", []byte(input)))
	root := p.ParseFile()
	
	if len(p.Diagnostics) > 0 {
		t.Errorf("Expected no diagnostics, got %d:", len(p.Diagnostics))
		for _, d := range p.Diagnostics {
			t.Logf("  %s at offset %d", d.Message, d.Span.Offset)
		}
	}
	
	if root == nil {
		t.Fatal("root is nil")
	}
}
