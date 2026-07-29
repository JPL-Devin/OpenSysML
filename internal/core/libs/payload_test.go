package libs

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

func TestBundledPayloadParsesCleanly(t *testing.T) {
	src := &embedSource{}
	for _, name := range src.List() {
		data, err := src.Read(name)
		if err != nil {
			t.Fatalf("Read(%q): %v", name, err)
		}
		p := parser.New(source.New(name, data))
		root := p.ParseFile()
		if len(p.Diagnostics) != 0 {
			t.Fatalf("bundled %q produced %d parse diagnostics, want 0: %v", name, len(p.Diagnostics), p.Diagnostics)
		}
		idx := symbols.NewIndex()
		idx.AddDocument(name, root)
		if len(idx.LookupQualified("ScalarValues")) == 0 && name == "ScalarValues.kerml" {
			t.Fatalf("expected ScalarValues package indexed from %q", name)
		}
	}
}

func TestBundledScalarValuesHasMembers(t *testing.T) {
	src := &embedSource{}
	data, err := src.Read("ScalarValues.kerml")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(source.New("ScalarValues.kerml", data))
	root := p.ParseFile()
	idx := symbols.NewIndex()
	idx.AddDocument("ScalarValues.kerml", root)
	if len(idx.LookupQualified("ScalarValues::Boolean")) != 1 {
		t.Fatal("expected ScalarValues::Boolean to be indexed")
	}
}
