package libs

import (
	"testing"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestFlowsDetail(t *testing.T) {
	src := &embedSource{}
	data, err := src.Read("Systems Library/Flows.sysml")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	
	p := parser.New(source.New("Flows.sysml", data))
	_ = p.ParseFile()
	
	t.Logf("Flows.sysml: %d diagnostics", len(p.Diagnostics))
	for _, d := range p.Diagnostics {
		t.Logf("  offset %d: %s", d.Span.Offset, d.Message)
	}
}
