package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

func TestTask70IdentifierMultiplicity(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantErrs int
	}{
		{
			name: "succession with identifier multiplicity",
			input: `package Test {
				feature accNum: Natural;
				connector TransitionLink {
					private succession [accNum] accept then [1] target;
				}
			}`,
			wantErrs: 0,
		},
		{
			name: "succession with feature reference multiplicity",
			input: `package Test {
				feature seBeforeNum: Natural;
				connector Flow {
					succession [seBeforeNum] first [0..1] source then [0..1] target;
				}
			}`,
			wantErrs: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := parser.New(source.New("test.kerml", []byte(tt.input)))
			_ = p.ParseFile()
			if len(p.Diagnostics) != tt.wantErrs {
				t.Errorf("got %d errors, want %d", len(p.Diagnostics), tt.wantErrs)
				for _, d := range p.Diagnostics {
					t.Logf("  - %s", d.Message)
				}
			}
		})
	}
}
