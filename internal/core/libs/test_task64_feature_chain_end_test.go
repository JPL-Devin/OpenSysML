package libs

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"testing"
)

func TestFeatureChainConnectorEnd(t *testing.T) {
	tests := []struct {
		name string
		code string
	}{
		{
			name: "succession with feature chain ends",
			code: `
				state TestState {
					succession [1] do.startShot then [*] nonDoMiddle.startShot;
				}
			`,
		},
		{
			name: "connection with feature chain ends",
			code: `
				part TestPart {
					connection connect [1] source.port to [1] target.port;
				}
			`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := source.New("test.kerml", []byte(tt.code))
			p := parser.New(file)
			root := p.ParseFile()

			if root == nil {
				t.Fatal("ParseFile returned nil")
			}

			for _, d := range p.Diagnostics {
				t.Errorf("offset %d: %s", d.Span.Offset, d.Message)
			}
		})
	}
}
