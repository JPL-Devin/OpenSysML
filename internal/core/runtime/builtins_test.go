package runtime

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
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

func TestSequenceFunctions_Includes(t *testing.T) {
	tests := []struct {
		src      string
		expected bool
	}{
		{`attribute result = SequenceFunctions::includes((1, 2, 3), 2);`, true},
		{`attribute result = SequenceFunctions::includes((1, 2, 3), 4);`, false},
		{`attribute result = SequenceFunctions::includes((), 1);`, false},
	}
	for _, tt := range tests {
		model, resolver, root := parseAndBuildModel(t, tt.src)
		ctx := NewContext(model, resolver, 1000)
		sym := resolveSymbol(t, root, "result")
		decl := sym.Decl.(*ast.Usage)
		result, err := ctx.Eval(decl.Value)
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if result.Kind != ValConst || result.Const.Kind != semantics.ValBool {
			t.Fatalf("expected bool, got %+v", result)
		}
		if result.Const.Bool != tt.expected {
			t.Errorf("includes(%s) = %v, expected %v", tt.src, result.Const.Bool, tt.expected)
		}
	}
}

func TestBuiltin_ControlSelect(t *testing.T) {
	// Stub test - full impl in integration
	t.Skip("defer to integration tests")
}
