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
