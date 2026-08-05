package libs

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestDebug_Binding(t *testing.T) {
	tests := []struct {
		name string
		code string
	}{
		{"simple", `part def Test { binding [1] startShot = [1] endShot; }`},
		{"feature chain", `part def Test { binding [1] x.field = [1] target; }`},
		{"with of", `part def Test { binding loopBack of [0..1] x.field = [1] target; }`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := tt.code

			sf := source.New("test.sysml", []byte(code))
			p := parser.New(sf)
			_ = p.ParseFile()

			if len(p.Diagnostics) > 0 {
				t.Logf("Parse diagnostics (%d):", len(p.Diagnostics))
				for i, d := range p.Diagnostics {
					text := sf.Text(d.Span)
					if len(text) > 50 {
						text = text[:50] + "..."
					}
					t.Logf("  [%d] offset %d: %s [near: %q]", i+1, d.Span.Offset, d.Message, text)
				}
			} else {
				t.Log("Parsed cleanly!")
			}
		})
	}
}
