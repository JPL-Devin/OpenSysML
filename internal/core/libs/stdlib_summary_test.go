package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

func TestStdlibErrorSummary(t *testing.T) {
	src := &embedSource{}
	files := src.List()

	totalErrors := make(map[string]int)
	sampleFiles := make(map[string][]string)
	failCount := 0

	for _, path := range files {
		data, err := src.Read(path)
		if err != nil {
			continue
		}

		sf := source.New(path, data)
		p := parser.New(sf)
		_ = p.ParseFile()

		if len(p.Diagnostics) > 0 {
			failCount++
			for _, d := range p.Diagnostics {
				totalErrors[d.Message]++
				if len(sampleFiles[d.Message]) < 3 {
					sampleFiles[d.Message] = append(sampleFiles[d.Message], path)
				}
			}
		}
	}

	t.Logf("\nParse coverage: %d/%d (%.1f%%) clean, %d failures\n",
		len(files)-failCount, len(files),
		100.0*float64(len(files)-failCount)/float64(len(files)),
		failCount)

	// Top 10 errors
	type errCount struct {
		msg   string
		count int
		files []string
	}
	var errors []errCount
	for msg, count := range totalErrors {
		errors = append(errors, errCount{msg, count, sampleFiles[msg]})
	}
	// Simple sort by count
	for i := 0; i < len(errors); i++ {
		for j := i + 1; j < len(errors); j++ {
			if errors[j].count > errors[i].count {
				errors[i], errors[j] = errors[j], errors[i]
			}
		}
	}

	t.Logf("\nTop 10 errors:")
	for i := 0; i < 10 && i < len(errors); i++ {
		e := errors[i]
		t.Logf("  %d: %s", e.count, e.msg)
		for _, f := range e.files {
			t.Logf("      - %s", f)
		}
	}
}
