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

	direct := &w8cWalker{}
	var want []*symbols.Symbol
	direct.walk(root, func(sym *symbols.Symbol) {
		want = append(want, sym)
	})
	ctx := NewContext("test.sysml", symbols.NewIndex(), nil)
	w := &w8cWalker{ctx: ctx}
	var got []*symbols.Symbol
	w.walk(root, func(sym *symbols.Symbol) {
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

func TestW8CWalkerDeduplicatesOverlappingScopes(t *testing.T) {
	root := symbols.NewScope(nil, nil)
	shared := &symbols.Symbol{Name: "shared"}
	overlap := symbols.NewScope(root, nil)
	overlap.Define("shared", shared)
	root.Define("shared", shared)

	w := &w8cWalker{}
	var got []*symbols.Symbol
	w.walk(root, func(sym *symbols.Symbol) {
		got = append(got, sym)
	})
	w.walk(overlap, func(sym *symbols.Symbol) {
		got = append(got, sym)
	})
	if len(got) != 1 || got[0] != shared {
		t.Fatalf("overlapping W8C walk visited %v, want one shared symbol", got)
	}
}
