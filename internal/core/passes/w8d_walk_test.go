package passes

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

func TestW8DSymbolWalkCachePreservesOrder(t *testing.T) {
	root := symbols.NewScope(nil, nil)
	nested := &symbols.Symbol{Name: "nested"}
	nested.Scope = symbols.NewScope(root, nil)
	nested.Scope.Define("nested", nested)
	root.Define("nested", nested)
	child := symbols.NewScope(root, nil)
	childSymbol := &symbols.Symbol{Name: "child"}
	child.Define("child", childSymbol)
	root.AddChild(child)

	want := w8dCollectSymbols(root)
	ctx := NewContext("test.sysml", symbols.NewIndex(), nil)
	var got []*symbols.Symbol
	w8dWalkSymbols(ctx, root, func(sym *symbols.Symbol) {
		got = append(got, sym)
	})
	if len(got) != len(want) {
		t.Fatalf("cached walk visited %d symbols, direct walk visited %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("cached walk symbol %d = %p, direct walk = %p", i, got[i], want[i])
		}
	}
	if cached := w8dSymbols(ctx, root); len(cached) != len(want) {
		t.Fatalf("cached symbol slice has %d symbols, want %d", len(cached), len(want))
	}
}

func TestW8CSymbolWalkCachePreservesOrder(t *testing.T) {
	root := symbols.NewScope(nil, nil)
	nested := &symbols.Symbol{Name: "nested"}
	nested.Scope = symbols.NewScope(root, nil)
	nested.Scope.Define("leaf", &symbols.Symbol{Name: "leaf"})
	root.Define("nested", nested)

	want := w8cCollectSymbols(root)
	ctx := NewContext("test.sysml", symbols.NewIndex(), nil)
	var got []*symbols.Symbol
	w8cWalkSymbols(ctx, root, func(sym *symbols.Symbol) {
		got = append(got, sym)
	})
	if len(got) != len(want) {
		t.Fatalf("cached W8C walk visited %d symbols, direct walk visited %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("cached W8C walk symbol %d = %p, direct walk = %p", i, got[i], want[i])
		}
	}
}

// One declaration registered under several keys, in its own scope and in a
// nested one, is still visited once.
func TestW8CWalkDeduplicatesAliasedSymbols(t *testing.T) {
	root := symbols.NewScope(nil, nil)
	shared := &symbols.Symbol{Name: "shared"}
	nested := &symbols.Symbol{Name: "nested"}
	nested.Scope = symbols.NewScope(root, nil)
	nested.Scope.Define("shared", shared)
	root.Define("shared", shared)
	root.Define("alias", shared)
	root.Define("nested", nested)

	var got []*symbols.Symbol
	w8cWalkSymbols(nil, root, func(sym *symbols.Symbol) {
		if sym == shared {
			got = append(got, sym)
		}
	})
	if len(got) != 1 {
		t.Fatalf("W8C walk visited the shared symbol %d times, want once", len(got))
	}
}
