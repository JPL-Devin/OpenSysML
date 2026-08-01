package libs

import (
	"testing"
	"sort"
	
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestStdlibErrorSummary(t *testing.T) {
	src := &embedSource{}
	files := src.List()
	
	errorCounts := make(map[string]int)
	
	for _, path := range files {
		data, _ := src.Read(path)
		if data == nil {
			continue
		}
		
		sf := source.New(path, data)
		p := parser.New(sf)
		_ = p.ParseFile()
		
		for _, d := range p.Diagnostics {
			errorCounts[d.Message]++
		}
	}
	
	type errCount struct {
		msg string
		count int
	}
	var sorted []errCount
	for msg, cnt := range errorCounts {
		sorted = append(sorted, errCount{msg, cnt})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].count > sorted[j].count
	})
	
	t.Logf("Top 15 error messages:")
	for i := 0; i < 15 && i < len(sorted); i++ {
		t.Logf("%4d: %s", sorted[i].count, sorted[i].msg)
	}
}
