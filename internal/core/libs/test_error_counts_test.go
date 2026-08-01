package libs

import (
	"strings"
	"testing"
	
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestStdlibErrorCounts(t *testing.T) {
	src := &embedSource{}
	files := src.List()
	
	counts := make(map[string]int)
	for _, name := range files {
		if !strings.HasSuffix(name, ".kerml") && !strings.HasSuffix(name, ".sysml") {
			continue
		}
		data, err := src.Read(name)
		if err != nil {
			continue
		}
		sf := source.New(name, data)
		p := parser.New(sf)
		_ = p.ParseFile()
		
		for _, d := range p.Diagnostics {
			counts[d.Message]++
		}
	}
	
	t.Logf("Top 15 error types:")
	var topErrors []struct{msg string; count int}
	for msg, count := range counts {
		topErrors = append(topErrors, struct{msg string; count int}{msg, count})
	}
	// Simple bubble sort
	for i := 0; i < len(topErrors); i++ {
		for j := i+1; j < len(topErrors); j++ {
			if topErrors[j].count > topErrors[i].count {
				topErrors[i], topErrors[j] = topErrors[j], topErrors[i]
			}
		}
	}
	for i := 0; i < 15 && i < len(topErrors); i++ {
		t.Logf("  %d: %s", topErrors[i].count, topErrors[i].msg)
	}
}
