package runtime

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/semantics"
)

func TestBuiltin_SequenceSize(t *testing.T) {
	seq := NewSequence()
	seq.Append(Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 1}})
	seq.Append(Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 2}})
	
	args := []Value{{Kind: ValSequence, Sequence: seq}}
	
	fn := builtins["SequenceFunctions::size"]
	result, err := fn(nil, args)
	if err != nil {
		t.Fatalf("size failed: %v", err)
	}
	
	if result.Const.Int != 2 {
		t.Errorf("expected size 2, got %d", result.Const.Int)
	}
}
