package libs

import (
	"testing"
	
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestStdlibCoverageCount(t *testing.T) {
	src := &embedSource{}
	files := src.List()
	
	clean := 0
	for _, path := range files {
		data, _ := src.Read(path)
		if data == nil {
			continue
		}
		
		sf := source.New(path, data)
		p := parser.New(sf)
		_ = p.ParseFile()
		
		if len(p.Diagnostics) == 0 {
			clean++
		}
	}
	
	total := len(files)
	pct := float64(clean) * 100.0 / float64(total)
	t.Logf("Clean files: %d/%d (%.1f%%)", clean, total, pct)
}
