package libs

import (
	"bytes"
	"encoding/gob"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

func indexOf(t *testing.T, name, src string) *symbols.Index {
	t.Helper()
	p := parser.New(source.New(name, []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("unexpected parse diagnostics: %v", p.Diagnostics)
	}
	idx := symbols.NewIndex()
	idx.AddDocument(name, root)
	return idx
}

func fqnSet(rec *IndexRecord) map[string]symbols.SymbolKind {
	m := map[string]symbols.SymbolKind{}
	for _, s := range rec.Symbols {
		m[s.FQN] = s.Kind
	}
	return m
}

func TestRecordFromIndexCollectsReducedSymbols(t *testing.T) {
	idx := indexOf(t, "ScalarValues.kerml",
		"standard library package ScalarValues { namespace Boolean; namespace Real; }")
	rec := recordFromIndex("ScalarValues.kerml", idx)
	if rec == nil {
		t.Fatal("recordFromIndex returned nil")
	}
	if rec.Name != "ScalarValues.kerml" {
		t.Fatalf("Name = %q", rec.Name)
	}
	got := fqnSet(rec)
	if got["ScalarValues"] != symbols.SymbolPackage {
		t.Errorf("ScalarValues kind = %v, want package", got["ScalarValues"])
	}
	if got["ScalarValues::Boolean"] != symbols.SymbolNamespace {
		t.Errorf("ScalarValues::Boolean kind = %v, want namespace", got["ScalarValues::Boolean"])
	}
	if _, ok := got["ScalarValues::Real"]; !ok {
		t.Errorf("ScalarValues::Real missing from record")
	}
}

func TestIndexRecordGobRoundTrip(t *testing.T) {
	idx := indexOf(t, "a.kerml", "package P { namespace N; }")
	rec := recordFromIndex("a.kerml", idx)

	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(rec); err != nil {
		t.Fatalf("encode: %v", err)
	}
	var got IndexRecord
	if err := gob.NewDecoder(&buf).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Name != rec.Name || len(got.Symbols) != len(rec.Symbols) {
		t.Fatalf("round-trip mismatch: got %+v want %+v", got, rec)
	}
	for i := range rec.Symbols {
		a, b := got.Symbols[i], rec.Symbols[i]
		if a.FQN != b.FQN || a.Kind != b.Kind || a.Span != b.Span || len(a.Supers) != len(b.Supers) {
			t.Errorf("symbol[%d] = %+v, want %+v", i, a, b)
		}
	}
}

func TestRecordSupersFromSpecializationEdges(t *testing.T) {
	src := "part def Car specializes Vehicle, Machine; part def Vehicle; part def Machine;"
	root := parser.New(source.New("lib", []byte(src))).ParseFile()
	idx := symbols.NewIndex()
	idx.AddDocument("lib", root)

	rec := recordFromIndex("lib", idx)
	if rec == nil {
		t.Fatalf("expected a record")
	}
	var car *symRecord
	for i := range rec.Symbols {
		if rec.Symbols[i].FQN == "Car" {
			car = &rec.Symbols[i]
		}
	}
	if car == nil {
		t.Fatalf("Car record not found")
	}
	if len(car.Supers) != 2 || car.Supers[0] != "Vehicle" || car.Supers[1] != "Machine" {
		t.Fatalf("Supers = %v, want [Vehicle Machine]", car.Supers)
	}
}

func TestRecordSupersExcludesTypingAndReferences(t *testing.T) {
	src := "part def Engine; part e : Engine subsets Engine;"
	root := parser.New(source.New("lib", []byte(src))).ParseFile()
	idx := symbols.NewIndex()
	idx.AddDocument("lib", root)

	rec := recordFromIndex("lib", idx)
	var e *symRecord
	for i := range rec.Symbols {
		if rec.Symbols[i].FQN == "e" {
			e = &rec.Symbols[i]
		}
	}
	if e == nil {
		t.Fatalf("e record not found")
	}
	if len(e.Supers) != 1 || e.Supers[0] != "Engine" {
		t.Fatalf("Supers = %v, want [Engine] (subsets only)", e.Supers)
	}
}
