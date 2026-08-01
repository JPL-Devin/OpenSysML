package libs
import (
	"testing"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)
func TestSampledFunctionsDetail(t *testing.T) {
	src := &embedSource{}
	data, _ := src.Read("Domain Libraries/Analysis/SampledFunctions.sysml")
	sf := source.New("SampledFunctions.sysml", data)
	p := parser.New(sf)
	_ = p.ParseFile()
	
	if len(p.Diagnostics) > 0 {
		t.Logf("Parse diagnostics (%d):", len(p.Diagnostics))
		for i, d := range p.Diagnostics {
			if i >= 15 { break }
			text := sf.Text(d.Span)
			if len(text) > 50 {
				text = text[:50] + "..."
			}
			t.Logf("  %s [near: %q]", d.Message, text)
		}
	}
}
