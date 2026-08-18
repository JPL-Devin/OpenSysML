package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

func TestTask78ActionsPattern(t *testing.T) {
	// Exact pattern from Actions.sysml offset 9760
	input := `package Test {
		action a {
			ref acceptedMessage : MessageTransfer, MessageAction :>> trigger {
				in :>> MessageTransfer::payload, MessageAction::payload;
			}
		}
	}`

	p := parser.New(source.New("test.kerml", []byte(input)))
	_ = p.ParseFile()

	t.Logf("Diagnostics: %d", len(p.Diagnostics))
	for i, d := range p.Diagnostics {
		t.Logf("  %d. %s", i+1, d.Message)
	}

	if len(p.Diagnostics) > 0 {
		t.Errorf("Expected clean parse, got %d errors", len(p.Diagnostics))
	}
}
