package libs

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestTask76DoubleRedefines(t *testing.T) {
	// Pattern from FeatureReferencingPerformances.kerml line 78
	input := `package Test {
		function allFeatureValues {
			in argument : Anything;
			return resultValues : Anything [*] nonunique redefines result redefines values;
		}
	}`

	p := parser.New(source.New("test.kerml", []byte(input)))
	_ = p.ParseFile()
	
	t.Logf("Diagnostics: %d", len(p.Diagnostics))
	for _, d := range p.Diagnostics {
		t.Logf("  - offset=%d: %s", d.Span.Offset, d.Message)
	}
	
	if len(p.Diagnostics) > 0 {
		t.Errorf("Expected clean parse, got %d errors", len(p.Diagnostics))
	}
}
