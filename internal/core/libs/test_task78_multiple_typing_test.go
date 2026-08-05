package libs

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestTask78MultipleTyping(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name: "multiple typing in feature",
			input: `package Test {
				feature acceptedMessage : MessageTransfer, MessageAction :>> trigger;
			}`,
		},
		{
			name: "multiple typing with ref modifier",
			input: `package Test {
				ref acceptedMessage : MessageTransfer, MessageAction :>> trigger;
			}`,
		},
		{
			name: "multiple typing in body",
			input: `package Test {
				feature f {
					ref acceptedMessage : MessageTransfer, MessageAction :>> trigger;
				}
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := parser.New(source.New("test.kerml", []byte(tt.input)))
			_ = p.ParseFile()

			if len(p.Diagnostics) > 0 {
				t.Errorf("Expected clean parse, got %d diagnostics:", len(p.Diagnostics))
				for _, d := range p.Diagnostics {
					t.Logf("  - %s", d.Message)
				}
			}
		})
	}
}
