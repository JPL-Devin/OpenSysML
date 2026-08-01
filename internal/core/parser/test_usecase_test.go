package parser

import (
	"testing"
	
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestParseUseCaseKeyword(t *testing.T) {
	code := `package Test {
    use case def UseCase;
    
    use case def Test2 {
        abstract use case subUseCases;
        ref use case start;
        abstract ref use case includedUseCases;
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
