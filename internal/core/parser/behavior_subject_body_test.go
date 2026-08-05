package parser

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestSubjectWithBody(t *testing.T) {
	input := `
	package Test {
		requirement def R {
			subject s : Thing[1] {
				doc /* doc inside subject body */
			}
		}
	}
	`

	sf := source.New("test.sysml", []byte(input))
	p := New(sf)
	root := p.ParseFile()

	if root == nil {
		t.Fatal("ParseFile returned nil")
	}

	if len(p.Diagnostics) > 0 {
		t.Errorf("Expected no diagnostics, got %d:", len(p.Diagnostics))
		for _, d := range p.Diagnostics {
			t.Logf("  %s", d.Message)
		}
	}
}
