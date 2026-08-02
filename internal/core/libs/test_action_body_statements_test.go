package libs

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestActionBodyStatements(t *testing.T) {
	tests := []struct{
		name string
		input string
	}{
		{
			name: "action with assign statement",
			input: `package Test {
				action Init {
					assign x := 1;
				}
			}`,
		},
		{
			name: "action with then keyword",
			input: `package Test {
				action Flow {
					assign x := 1;
					then assign y := 2;
				}
			}`,
		},
		{
			name: "action with while loop",
			input: `package Test {
				action Loop {
					while x < 10 {
						assign x := x + 1;
					}
				}
			}`,
		},
		{
			name: "action with perform statement",
			input: `package Test {
				action Invoke {
					perform SomeAction;
				}
			}`,
		},
		{
			name: "action succession at namespace level",
			input: `package Test {
				action Init
					assign x := 1;
				then action Process
					assign y := x + 1;
			}`,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := parser.New(source.New("test.sysml", []byte(tt.input)))
			_ = p.ParseFile()
			
			if len(p.Diagnostics) > 0 {
				t.Logf("FAILED - %d errors:", len(p.Diagnostics))
				for _, d := range p.Diagnostics {
					t.Logf("  - %s", d.Message)
				}
				t.Fail()
			} else {
				t.Logf("PASS")
			}
		})
	}
}
