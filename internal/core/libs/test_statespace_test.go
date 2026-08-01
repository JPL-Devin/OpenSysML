package libs

import (
	"testing"
	
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestStateSpaceRepresentation(t *testing.T) {
	src := &embedSource{}
	data, err := src.Read("Domain Libraries/Analysis/StateSpaceRepresentation.sysml")
	if err != nil {
		t.Fatal(err)
	}
	
	sf := source.New("StateSpaceRepresentation.sysml", data)
	p := parser.New(sf)
	_ = p.ParseFile()
	
	if len(p.Diagnostics) > 0 {
		t.Logf("Parsed with %d diagnostics:", len(p.Diagnostics))
		for i, d := range p.Diagnostics {
			if i < 10 {
				t.Logf("  %s", d.Message)
			}
		}
	} else {
		t.Log("Parsed cleanly!")
	}
}
