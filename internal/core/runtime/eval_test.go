package runtime

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/resolve"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

func parseAndBuildModel(t *testing.T, code string) (*semantics.Model, *resolve.Resolver, *symbols.Scope) {
	t.Helper()
	src := source.New("test.sysml", []byte(code))
	p := parser.New(src)
	root := p.ParseFile()
	idx := symbols.NewIndex()
	idx.AddDocument("test.sysml", root)
	rootScope := idx.DocumentRoot("test.sysml")
	if rootScope == nil {
		t.Fatal("rootScope nil")
	}
	resolver := resolve.New(idx)
	model := semantics.NewModel(resolver)
	return model, resolver, rootScope
}

func resolveSymbol(t *testing.T, rootScope *symbols.Scope, name string) *symbols.Symbol {
	t.Helper()
	sym, ok := rootScope.LookupLocal(name)
	if !ok || sym == nil {
		t.Fatalf("symbol %q not found", name)
	}
	return sym
}

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

func TestEval_SequenceExpr(t *testing.T) {
	// Test (1, 2, 3) sequence construction
	src := `attribute test = (1, 2, 3);`
	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(model, resolver, 1000)
	
	attrSym := resolveSymbol(t, root, "test")
	attrDecl := attrSym.Decl.(*ast.Usage)
	expr := attrDecl.Value
	
	result, err := ctx.Eval(expr)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	
	if result.Kind != ValSequence {
		t.Fatalf("expected ValSequence, got %v", result.Kind)
	}
	if result.Sequence.Size() != 3 {
		t.Errorf("expected size 3, got %d", result.Sequence.Size())
	}
	
	// Check elements
	elem0, _ := result.Sequence.At(0)
	if elem0.Kind != ValConst || elem0.Const.Int != 1 {
		t.Errorf("elem[0] expected 1, got %v", elem0)
	}
}

func TestEval_CollectExpr(t *testing.T) {
	// Defer to integration tests — parser may not support body syntax yet
	t.Skip("defer collection operations to integration tests")
}

func TestEval_SelectExpr(t *testing.T) {
	// Defer to integration tests — parser may not support body syntax yet
	t.Skip("defer collection operations to integration tests")
}

func TestEval_BuiltinInvocation(t *testing.T) {
	// Verify builtin dispatch works (test with SequenceFunctions::size)
	t.Skip("defer to integration — requires InvocationExpr parse")
}

