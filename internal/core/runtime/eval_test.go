package runtime

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
)

func TestEval_Literals(t *testing.T) {
	tests := []struct {
		src      string
		expected Value
	}{
		{"42", Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 42}}},
		{"3.14", Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: 3.14}}},
		{"true", Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValBool, Bool: true}}},
		{`"hello"`, Value{Kind: ValString, Str: "hello"}},
		{"null", Value{Kind: ValNull}},
	}

	for _, tt := range tests {
		t.Run(tt.src, func(t *testing.T) {
			// Wrap expression in attribute default
			model, resolver, root := parseAndBuildModel(t, "attribute test = "+tt.src+";")
			ctx := NewContext(model, resolver, 1000)

			// Extract expression from attribute value
			attrSym := resolveSymbol(t, root, "test")
			attrDecl := attrSym.Decl.(*ast.Usage)
			expr := attrDecl.Value

			result, err := ctx.Eval(expr)
			if err != nil {
				t.Fatalf("Eval failed: %v", err)
			}

			if result.Kind != tt.expected.Kind {
				t.Errorf("expected Kind %v, got %v", tt.expected.Kind, result.Kind)
			}
		})
	}
}

func TestEval_Arithmetic(t *testing.T) {
	src := `attribute test = 1 + 2;`
	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(model, resolver, 1000)

	attrSym := resolveSymbol(t, root, "test")
	attrDecl := attrSym.Decl.(*ast.Usage)
	expr := attrDecl.Value

	result, err := ctx.Eval(expr)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}

	if result.Kind != ValConst || result.Const.Int != 3 {
		t.Errorf("expected 3, got %v", result)
	}
}
