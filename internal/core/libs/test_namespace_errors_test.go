package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// TestNamespaceErrorDetail - Analyze "expected a namespace member" errors
func TestNamespaceErrorDetail(t *testing.T) {
	src := &embedSource{}
	files := src.List()

	type ErrorInfo struct {
		File    string
		Offset  int
		Message string
		Context string // 60 chars before + after
	}

	var namespaceErrors []ErrorInfo

	for _, name := range files {
		data, err := src.Read(name)
		if err != nil {
			continue
		}

		p := parser.New(source.New(name, data))
		_ = p.ParseFile()

		for _, d := range p.Diagnostics {
			if d.Message == "expected a namespace member" {
				offset := d.Span.Offset
				contextStart := offset - 60
				if contextStart < 0 {
					contextStart = 0
				}
				contextEnd := offset + 60
				if contextEnd > len(data) {
					contextEnd = len(data)
				}

				// Extract context
				ctx := string(data[contextStart:contextEnd])

				namespaceErrors = append(namespaceErrors, ErrorInfo{
					File:    name,
					Offset:  offset,
					Message: d.Message,
					Context: ctx,
				})
			}
		}
	}

	t.Logf("Found %d 'expected a namespace member' errors\n", len(namespaceErrors))

	// Show first 10
	for i, e := range namespaceErrors {
		if i >= 10 {
			t.Logf("... and %d more\n", len(namespaceErrors)-10)
			break
		}
		t.Logf("\n=== Error %d ===", i+1)
		t.Logf("File: %s", e.File)
		t.Logf("Location: offset=%d", e.Offset)
		t.Logf("Context: ...%q...", e.Context)
	}

	// Pattern analysis
	patterns := make(map[string]int)
	for _, e := range namespaceErrors {
		// Extract first 20 chars after offset as pattern
		offset := e.Offset
		data, _ := src.Read(e.File)
		end := offset + 20
		if end > len(data) {
			end = len(data)
		}
		pattern := string(data[offset:end])
		patterns[pattern]++
	}

	t.Logf("\n=== Pattern Analysis ===")
	for pattern, count := range patterns {
		t.Logf("%3d: %q", count, pattern)
	}
}
