package runtime

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
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
