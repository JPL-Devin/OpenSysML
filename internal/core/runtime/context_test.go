package runtime

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/resolve"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

func TestContextIDAllocation(t *testing.T) {
	idx := symbols.NewIndex()
	resolver := resolve.New(idx)
	model := semantics.NewModel(resolver)
	ctx := NewContext(model, resolver, 100000)
	
	id1 := ctx.allocateID()
	id2 := ctx.allocateID()
	
	if id1 == id2 {
		t.Error("expected unique IDs, got duplicates")
	}
	if id1 != 1 || id2 != 2 {
		t.Errorf("expected sequential IDs 1,2; got %d,%d", id1, id2)
	}
}

func TestContextStepCounter(t *testing.T) {
	idx := symbols.NewIndex()
	resolver := resolve.New(idx)
	model := semantics.NewModel(resolver)
	ctx := NewContext(model, resolver, 10)
	
	for i := 0; i < 10; i++ {
		if err := ctx.incrementStep(); err != nil {
			t.Fatalf("step %d failed: %v", i, err)
		}
	}
	
	// 11th step should error
	if err := ctx.incrementStep(); err == nil {
		t.Error("expected step limit error, got nil")
	}
}
