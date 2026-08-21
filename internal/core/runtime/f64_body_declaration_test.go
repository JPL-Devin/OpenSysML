package runtime

import (
	"errors"
	"testing"
)

// F64: the parser now keeps the features a body expression declares, but the
// evaluator binds only its parameters — so applying such a body must fail with a
// typed error rather than evaluate its result against unbound names.
func TestF64BodyDeclarationIsUnsupported(t *testing.T) {
	for _, expr := range []string{
		"xs->select { in i; private attribute k = i * 2; k > 2 }",
		"xs->collect { in i; attribute k = i + 1; k }",
		"xs->reject { in i; private attribute k = i; k > 1 }",
	} {
		t.Run(expr, func(t *testing.T) {
			_, err := evalCollectionExpr(t, expr)
			if !errors.Is(err, ErrUnsupportedBodyDeclaration) {
				t.Fatalf("err = %v, want ErrUnsupportedBodyDeclaration", err)
			}
		})
	}
}

// A body that declares nothing still evaluates.
func TestF64BodyWithoutDeclarationStillEvaluates(t *testing.T) {
	got, err := evalCollectionExpr(t, "xs->select { in i; i > 1 }")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Sequence == nil || len(got.Sequence.Elements()) != 2 {
		t.Errorf("result = %v, want the two elements above 1", got)
	}
}
