package libs

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestTradeStudiesDetail(t *testing.T) {
	src := &embedSource{}
	data, err := src.Read("Domain Libraries/Analysis/TradeStudies.sysml")
	if err != nil {
		t.Fatalf("Failed to load TradeStudies.sysml: %v", err)
	}

	p := parser.New(source.New("TradeStudies.sysml", data))
	_ = p.ParseFile()

	t.Logf("TradeStudies.sysml: %d diagnostics", len(p.Diagnostics))
	
	for i, d := range p.Diagnostics {
		byteOffset := d.Span.Offset
		char := ""
		if byteOffset < len(data) {
			char = string(data[byteOffset])
		}
		
		// Show 40 chars of context around error
		contextStart := byteOffset - 20
		if contextStart < 0 {
			contextStart = 0
		}
		contextEnd := byteOffset + 20
		if contextEnd > len(data) {
			contextEnd = len(data)
		}
		context := string(data[contextStart:contextEnd])
		
		t.Logf("  %d. offset=%d (char=%q): %s", i+1, byteOffset, char, d.Message)
		t.Logf("     context: %q", context)
	}
}
