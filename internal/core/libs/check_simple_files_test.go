package libs

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestSimpleFailures(t *testing.T) {
	src := &embedSource{}
	files := src.List()

	type fileErr struct {
		name  string
		count int
	}
	var failures []fileErr

	for _, path := range files {
		data, err := src.Read(path)
		if err != nil {
			continue
		}

		sf := source.New(path, data)
		p := parser.New(sf)
		_ = p.ParseFile()

		if len(p.Diagnostics) > 0 && len(p.Diagnostics) <= 10 {
			failures = append(failures, fileErr{path, len(p.Diagnostics)})
		}
	}

	// Sort by error count
	for i := 0; i < len(failures); i++ {
		for j := i + 1; j < len(failures); j++ {
			if failures[j].count < failures[i].count {
				failures[i], failures[j] = failures[j], failures[i]
			}
		}
	}

	t.Logf("\nFiles with ≤10 errors (sorted by count):")
	for _, f := range failures {
		t.Logf("  %d: %s", f.count, f.name)
	}
}
