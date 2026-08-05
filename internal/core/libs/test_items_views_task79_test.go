package libs

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestItemsDetailTask79(t *testing.T) {
	src := &embedSource{}
	data, err := src.Read("Systems Library/Items.sysml")
	if err != nil {
		t.Fatalf("Failed to load Items.sysml: %v", err)
	}

	p := parser.New(source.New("Items.sysml", data))
	_ = p.ParseFile()

	t.Logf("Items.sysml: %d diagnostics", len(p.Diagnostics))

	for i, d := range p.Diagnostics {
		byteOffset := d.Span.Offset
		char := ""
		if byteOffset < len(data) {
			char = string(data[byteOffset])
		}

		contextStart := byteOffset - 50
		if contextStart < 0 {
			contextStart = 0
		}
		contextEnd := byteOffset + 50
		if contextEnd > len(data) {
			contextEnd = len(data)
		}
		context := string(data[contextStart:contextEnd])

		t.Logf("  %d. offset=%d (char=%q): %s", i+1, byteOffset, char, d.Message)
		t.Logf("     context: %q", context)
	}
}

func TestViewsDetailTask79(t *testing.T) {
	src := &embedSource{}
	data, err := src.Read("Systems Library/Views.sysml")
	if err != nil {
		t.Fatalf("Failed to load Views.sysml: %v", err)
	}

	p := parser.New(source.New("Views.sysml", data))
	_ = p.ParseFile()

	t.Logf("Views.sysml: %d diagnostics", len(p.Diagnostics))

	for i, d := range p.Diagnostics {
		byteOffset := d.Span.Offset
		char := ""
		if byteOffset < len(data) {
			char = string(data[byteOffset])
		}

		contextStart := byteOffset - 50
		if contextStart < 0 {
			contextStart = 0
		}
		contextEnd := byteOffset + 50
		if contextEnd > len(data) {
			contextEnd = len(data)
		}
		context := string(data[contextStart:contextEnd])

		t.Logf("  %d. offset=%d (char=%q): %s", i+1, byteOffset, char, d.Message)
		t.Logf("     context: %q", context)
	}
}
