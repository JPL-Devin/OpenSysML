package libs

import (
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestRemainingFiles(t *testing.T) {
	src := &embedSource{}
	files := src.List()

	var failedFiles []struct {
		name  string
		count int
	}

	for _, name := range files {
		data, err := src.Read(name)
		if err != nil {
			t.Logf("SKIP %s: %v", name, err)
			continue
		}

		p := parser.New(source.New(name, data))
		_ = p.ParseFile()

		if len(p.Diagnostics) > 0 {
			failedFiles = append(failedFiles, struct {
				name  string
				count int
			}{name, len(p.Diagnostics)})
		}
	}

	t.Logf("Remaining files with errors: %d", len(failedFiles))
	for _, f := range failedFiles {
		t.Logf("  %s (%d errors)", filepath.Base(f.name), f.count)
	}
}
