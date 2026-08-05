package libs

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestStandardLibraryKeyword(t *testing.T) {
	// With "standard library"
	code1 := `standard library package Base {
	abstract classifier Anything {
	}
}`

	p1 := parser.New(source.New("test1.kerml", []byte(code1)))
	_ = p1.ParseFile()
	t.Logf("Test 1 (standard library): %d diagnostics", len(p1.Diagnostics))
	for _, d := range p1.Diagnostics {
		t.Logf("  - offset=%d, %s", d.Span.Offset, d.Message)
	}

	// With "library" only
	code2 := `library package Base {
	abstract classifier Anything {
	}
}`

	p2 := parser.New(source.New("test2.kerml", []byte(code2)))
	_ = p2.ParseFile()
	t.Logf("\nTest 2 (library only): %d diagnostics", len(p2.Diagnostics))
	for _, d := range p2.Diagnostics {
		t.Logf("  - offset=%d, %s", d.Span.Offset, d.Message)
	}

	// Without "library"
	code3 := `package Base {
	abstract classifier Anything {
	}
}`

	p3 := parser.New(source.New("test3.kerml", []byte(code3)))
	_ = p3.ParseFile()
	t.Logf("\nTest 3 (no library): %d diagnostics", len(p3.Diagnostics))
	for _, d := range p3.Diagnostics {
		t.Logf("  - offset=%d, %s", d.Span.Offset, d.Message)
	}
}
