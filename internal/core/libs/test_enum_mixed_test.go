package libs

import (
	"testing"
	
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestEnumMixedLiterals(t *testing.T) {
	src := `
	package test {
		enum def VerdictKind {
			pass;
			fail;
			inconclusive;
			error;
		}
		
		enum def ProbabilityLevel {
			low = 0.25;
			medium = 0.50;
			high = 0.75;
		}
	}
	`
	
	p := parser.New(source.New("test.sysml", []byte(src)))
	_ = p.ParseFile()
	
	if len(p.Diagnostics) > 0 {
		t.Fatalf("Expected clean parse, got %d diagnostics:", len(p.Diagnostics))
		for _, d := range p.Diagnostics {
			t.Logf("  %s", d.Message)
		}
	}
	
	t.Logf("Parsed cleanly!")
}
