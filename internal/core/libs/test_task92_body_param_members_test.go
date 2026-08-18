package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

func TestTask92BodyParamMembers(t *testing.T) {
	// Body param with nested doc comments (TradeStudies.sysml pattern)
	input := `package Test {
		function selectBest {
			in alternatives: Alternative[*];
			return selected = alternatives->selectOne {in ref a {
				doc
				/*
				 * The selected alternative that meets the objective.
				 */
			} objective(a)};
		}
	}`

	p := parser.New(source.New("test.sysml", []byte(input)))
	_ = p.ParseFile()

	if len(p.Diagnostics) > 0 {
		for _, d := range p.Diagnostics {
			t.Logf("Error at offset %d: %s", d.Span.Offset, d.Message)
		}
		t.Errorf("Expected clean parse, got %d errors", len(p.Diagnostics))
	}
}
