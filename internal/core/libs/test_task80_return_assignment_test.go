package libs

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestTask80ReturnAssignment(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name: "return with assignment",
			input: `package Test {
				function includes {
					in seq: Integer[*];
					in elem: Integer;
					return result: Boolean = true;
				}
			}`,
		},
		{
			name: "return with expression assignment",
			input: `package Test {
				function earlierFirstIncomingTransferSort {
					in t1: Transfer [1];
					in t2: Transfer [1];
					return t1First: Boolean = includes(t1.endShot.successors, t2.endShot);
				}
			}`,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := parser.New(source.New("test.kerml", []byte(tt.input)))
			_ = p.ParseFile()
			
			if len(p.Diagnostics) > 0 {
				t.Errorf("Expected clean parse, got %d errors:", len(p.Diagnostics))
				for _, d := range p.Diagnostics {
					t.Logf("  - %s", d.Message)
				}
			}
		})
	}
}
