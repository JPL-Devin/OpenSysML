package parser

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestRefSimple(t *testing.T) {
	input := `action def Test {
ref stateSpace: StateSpace;
}`
	sf := source.New("test.sysml", []byte(input))
	p := New(sf)
	_ = p.ParseFile()

	if len(p.Diagnostics) > 0 {
		for _, d := range p.Diagnostics {
			t.Errorf("Parse error: %s", d.Message)
		}
	}
}
