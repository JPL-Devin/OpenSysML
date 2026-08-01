package libs

import (
	"testing"
	"strings"
	
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestExpressionErrors(t *testing.T) {
	src := &embedSource{}
	files := src.List()
	
	type errLoc struct {
		file string
		line int
		context string
	}
	var exprErrors []errLoc
	
	for _, path := range files {
		data, _ := src.Read(path)
		if data == nil {
			continue
		}
		
		sf := source.New(path, data)
		p := parser.New(sf)
		_ = p.ParseFile()
		
		for _, d := range p.Diagnostics {
			if d.Message == "expected an expression" {
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
				exprErrors = append(exprErrors, errLoc{path, lineIdx+1, ctx})
			}
		}
	}
	
	t.Logf("Expression errors (%d):", len(exprErrors))
	for i := 0; i < 5 && i < len(exprErrors); i++ {
		t.Logf("%s:%d - %s", exprErrors[i].file, exprErrors[i].line, exprErrors[i].context)
	}
}
