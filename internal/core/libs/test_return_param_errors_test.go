package libs

import (
	"testing"
	"strings"
	
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestReturnParamErrors(t *testing.T) {
	src := &embedSource{}
	files := src.List()
	
	type errLoc struct {
		file string
		line int
		context string
	}
	var errors []errLoc
	
	for _, path := range files {
		data, _ := src.Read(path)
		if data == nil {
			continue
		}
		
		sf := source.New(path, data)
		p := parser.New(sf)
		_ = p.ParseFile()
		
		for _, d := range p.Diagnostics {
			if strings.Contains(d.Message, "after return parameter") {
				lines := strings.Split(string(data), "\n")
				lineIdx := 0
				pos := 0
				for pos < d.Span.Offset && lineIdx < len(lines) {
					pos += len(lines[lineIdx]) + 1
					lineIdx++
				}
				ctx := ""
				if lineIdx < len(lines) {
					ctx = strings.TrimSpace(lines[lineIdx])
					if len(ctx) > 80 {
						ctx = ctx[:80] + "..."
					}
				}
				errors = append(errors, errLoc{path, lineIdx+1, ctx})
			}
		}
	}
	
	t.Logf("Return parameter errors (%d):", len(errors))
	for i := 0; i < 5 && i < len(errors); i++ {
		t.Logf("%s:%d - %s", errors[i].file, errors[i].line, errors[i].context)
	}
}
