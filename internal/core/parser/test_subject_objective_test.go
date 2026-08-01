package parser

import (
	"testing"
	
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestParseSubjectObjective(t *testing.T) {
	code := `package Test {
    case def TestCase {
        subject subj : Anything[1];
        objective obj : RequirementCheck[1];
    }
}`
	sf := source.New("test.sysml", []byte(code))
	p := New(sf)
	_ = p.ParseFile()
	
	if len(p.Diagnostics) > 0 {
		t.Errorf("Expected no diagnostics, got %d:", len(p.Diagnostics))
		for _, d := range p.Diagnostics {
			text := sf.Text(d.Span)
			if len(text) > 60 {
				text = text[:60] + "..."
			}
			t.Logf("  %s [near: %q]", d.Message, text)
		}
	}
}
