package libs

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

func TestExpectedNameErrors(t *testing.T) {
	src := &embedSource{}
	files := src.List()

	targetError := "expected a name"

	var samples []struct {
		file    string
		offset  int
		context string
	}

	for _, name := range files {
		data, err := src.Read(name)
		if err != nil {
			continue
		}

		p := parser.New(source.New(name, data))
		_ = p.ParseFile()

		for _, d := range p.Diagnostics {
			if strings.Contains(d.Message, targetError) {
				offset := d.Span.Offset
				context := ""
				if offset >= 0 && offset < len(data) {
					start := offset - 50
					if start < 0 {
						start = 0
					}
					end := offset + 100
					if end > len(data) {
						end = len(data)
					}
					context = string(data[start:end])
				}
				samples = append(samples, struct {
					file    string
					offset  int
					context string
				}{name, offset, context})

				if len(samples) >= 10 {
					break
				}
			}
		}
		if len(samples) >= 10 {
			break
		}
	}

	t.Logf("Found %d 'expected a name' errors", len(samples))
	for i, s := range samples {
		t.Logf("\nSample %d - %s (offset %d):", i+1, s.file, s.offset)
		t.Logf("  %q", s.context)
	}
}
