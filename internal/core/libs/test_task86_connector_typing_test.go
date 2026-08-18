package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

func TestTask86ConnectorTyping(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name: "connector with 'to' typing",
			input: `package Test {
				private connector [0..1] transitionLink to [1..*] trigger;
			}`,
		},
		{
			name: "connector with standard typing",
			input: `package Test {
				private connector [0..1] transitionLink : TriggerType;
			}`,
		},
		{
			name: "connector with from/to ends",
			input: `package Test {
				private connector transitionLink from [0..1] source to [1..*] target;
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := parser.New(source.New("test.kerml", []byte(tt.input)))
			_ = p.ParseFile()

			t.Logf("Diagnostics: %d", len(p.Diagnostics))
			for _, d := range p.Diagnostics {
				t.Logf("  - %s", d.Message)
			}

			if len(p.Diagnostics) > 0 {
				t.Errorf("Expected clean parse, got %d errors", len(p.Diagnostics))
			}
		})
	}
}
