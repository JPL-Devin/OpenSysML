package libs

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"strings"
	"testing"
)

func TestFindDocErrors(t *testing.T) {
	src := &embedSource{}
	files := src.List()

	for _, fname := range files {
		data, _ := src.Read(fname)
		p := parser.New(source.New(fname, data))
		_ = p.ParseFile()
		diags := p.Diagnostics

		for _, d := range diags {
			if strings.Contains(d.Message, "unknown action keyword: doc") {
				t.Logf("File: %s: %s", fname, d.Message)
			}
		}
	}
}
