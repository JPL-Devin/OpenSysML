package libs

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestTask87ShorthandBodyParam(t *testing.T) {
	// Test shorthand body param syntax: {name : Type; expr}
	// Used in collection operators like ->exists, ->forAll, ->select
	input := `package Test {
		predicate test {
			in vertices: Point[*];
			vertices->exists{p : Point; p.x > 0}
		}
	}`

	p := parser.New(source.New("test.sysml", []byte(input)))
	_ = p.ParseFile()

	if len(p.Diagnostics) > 0 {
		t.Errorf("Expected clean parse, got %d errors:", len(p.Diagnostics))
		for _, d := range p.Diagnostics {
			t.Logf("  - %s", d.Message)
		}
	}
}

// Test actual ShapeItems.sysml pattern
func TestTask87ShapeItemsPattern(t *testing.T) {
	input := `package Test {
		feature vertices: Point[*];
		predicate isDistinct {
			in p1 : Point;
			vertices->exists{p2 : Point; p1 != p2 and includes(p1.matingOccurrences, p2) }
		}
	}`

	p := parser.New(source.New("test.sysml", []byte(input)))
	_ = p.ParseFile()

	if len(p.Diagnostics) > 0 {
		t.Errorf("Expected clean parse, got %d errors:", len(p.Diagnostics))
		for _, d := range p.Diagnostics {
			t.Logf("  - offset=%d: %s", d.Span.Offset, d.Message)
		}
	}
}
