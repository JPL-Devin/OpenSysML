package libs
import (
	"fmt"
	"testing"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)
func TestStdlibErrorSummary(t *testing.T) {
	src := &embedSource{}
	files := src.List()
	counts := make(map[string]int)
	for _, f := range files {
		data, _ := src.Read(f)
		sf := source.New(f, data)
		p := parser.New(sf)
		_ = p.ParseFile()
		for _, d := range p.Diagnostics {
			counts[d.Message]++
		}
	}
	for msg, cnt := range counts {
		fmt.Printf("%d: %s\n", cnt, msg)
	}
}
