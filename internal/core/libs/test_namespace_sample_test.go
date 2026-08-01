package libs
import (
	"testing"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)
func TestNamespaceErrorSample(t *testing.T) {
	src := &embedSource{}
	files := src.List()
	for _, f := range files {
		data, _ := src.Read(f)
		sf := source.New(f, data)
		p := parser.New(sf)
		_ = p.ParseFile()
		found := 0
		for _, d := range p.Diagnostics {
			if d.Message == "expected a namespace member" && found < 3 {
				text := sf.Text(d.Span)
				if len(text) > 60 {
					text = text[:60] + "..."
				}
				t.Logf("%s: %s [near: %q]", f, d.Message, text)
				found++
			}
		}
		if found > 0 {
			return
		}
	}
}
