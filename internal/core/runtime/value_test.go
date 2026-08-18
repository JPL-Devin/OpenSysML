package runtime

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

func TestValueConstWrapping(t *testing.T) {
	// Test that runtime.Value correctly wraps semantics.Value
	semVal := semantics.Value{Kind: semantics.ValInt, Int: 42}
	v := Value{Kind: ValConst, Const: semVal}

	if v.Kind != ValConst {
		t.Errorf("expected ValConst, got %v", v.Kind)
	}
	if v.Const.Int != 42 {
		t.Errorf("expected Int=42, got %d", v.Const.Int)
	}
}

func TestSequenceOperations(t *testing.T) {
	seq := NewSequence()
	v1 := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 1}}
	v2 := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 2}}

	seq.Append(v1)
	seq.Append(v2)

	if seq.Size() != 2 {
		t.Errorf("expected size 2, got %d", seq.Size())
	}

	elem, err := seq.At(0)
	if err != nil || elem.Const.Int != 1 {
		t.Errorf("expected elem[0]=1, got %v, err=%v", elem, err)
	}
}

func TestSetOperations(t *testing.T) {
	set := NewSet()
	v1 := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 1}}
	v2 := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 1}} // duplicate

	set.Add(v1)
	set.Add(v2) // should not increase size

	if set.Size() != 1 {
		t.Errorf("expected size 1 (dedupe), got %d", set.Size())
	}

	if !set.Contains(v1) {
		t.Error("expected set to contain v1")
	}
}

func TestSetOperationsUseExactValueEquality(t *testing.T) {
	sequenceValue := func(n int64) Value {
		seq := NewSequence()
		seq.Append(Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: n}})
		return Value{Kind: ValSequence, Sequence: seq}
	}
	first := sequenceValue(1)
	second := sequenceValue(65537)
	if valueKeyFunc(first) != valueKeyFunc(second) {
		t.Fatal("test values must share a collection hash bucket")
	}
	set := NewSet()
	set.Add(first)
	set.Add(second)

	if set.Size() != 2 {
		t.Fatalf("expected two distinct sequence elements, got %d", set.Size())
	}
	if !set.Contains(second) {
		t.Fatal("expected set to contain the second colliding sequence")
	}
}

func TestSetConstructionUsesBucketedLinearWork(t *testing.T) {
	const count = 2000
	set := NewSet()
	for i := 0; i < count; i++ {
		set.Add(Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: int64(i)}})
	}
	for i := 0; i < count; i++ {
		if !set.Contains(Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: int64(i)}}) {
			t.Fatalf("set does not contain %d", i)
		}
	}
	if len(set.elements) != count {
		t.Fatalf("set has %d buckets for %d elements", len(set.elements), count)
	}
	maxBucket := 0
	for _, bucket := range set.elements {
		if len(bucket) > maxBucket {
			maxBucket = len(bucket)
		}
	}
	if maxBucket > 1 {
		t.Fatalf("maximum bucket length = %d, want 1 for distinct integer keys", maxBucket)
	}
}

func TestSetElementsPreserveInsertionOrder(t *testing.T) {
	set := NewSet()
	for _, value := range []int64{2, 1, 2, 3} {
		set.Add(Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: value}})
	}
	elements := set.Elements()
	if len(elements) != 3 {
		t.Fatalf("set has %d elements, want 3", len(elements))
	}
	for i, want := range []int64{2, 1, 3} {
		if got := elements[i].Const.Int; got != want {
			t.Errorf("element %d = %d, want %d", i, got, want)
		}
	}
}

func TestSetDeduplicatesCommensurableQuantities(t *testing.T) {
	metre := &symbols.Symbol{Name: "metre"}
	unit := Unit{
		Text: "km",
		Term: semantics.UnitTerm{
			Scale:   semantics.UnitScale(1000),
			Factors: []semantics.UnitFactor{{Unit: metre, Exponent: 1}},
		},
	}
	base := Unit{
		Text: "m",
		Term: semantics.UnitTerm{
			Scale:   semantics.UnitScale(1),
			Factors: []semantics.UnitFactor{{Unit: metre, Exponent: 1}},
		},
	}
	set := NewSet()
	set.Add(Value{Kind: ValQuantity, Quantity: &Quantity{
		Num:  semantics.Value{Kind: semantics.ValInt, Int: 1},
		Unit: unit,
	}})
	set.Add(Value{Kind: ValQuantity, Quantity: &Quantity{
		Num:  semantics.Value{Kind: semantics.ValInt, Int: 1000},
		Unit: base,
	}})
	if set.Size() != 1 {
		t.Fatalf("set size = %d, want 1 for commensurable equal quantities", set.Size())
	}
}

func TestFormatValueCollections(t *testing.T) {
	inner := NewSequence()
	inner.Append(Value{Kind: ValString, Str: "nested"})
	outer := NewSequence()
	outer.Append(Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 1}})
	outer.Append(Value{Kind: ValSequence, Sequence: inner})

	set := NewSet()
	set.Add(Value{Kind: ValInstance, Instance: 3})
	set.Add(Value{Kind: ValSequence, Sequence: outer})

	if got, want := FormatValue(Value{Kind: ValSequence, Sequence: outer}),
		`[1, ["nested"]]`; got != want {
		t.Errorf("sequence formatting = %q, want %q", got, want)
	}
	if got, want := FormatValue(Value{Kind: ValSet, Set: set}),
		`Set{instance(3), [1, ["nested"]]}`; got != want {
		t.Errorf("set formatting = %q, want %q", got, want)
	}
}
