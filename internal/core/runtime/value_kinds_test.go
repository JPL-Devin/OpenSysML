package runtime

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// kindSamples is two distinct values of every kind. A kind added to value.go
// without an entry here fails TestEveryValueKindIsDispatched, which is the point.
func kindSamples() map[ValueKind][2]Value {
	seqOf := func(ns ...int64) Value {
		seq := NewSequence()
		for _, n := range ns {
			seq.Append(integerValue(n))
		}
		return NewSequenceValue(seq)
	}
	setOf := func(ns ...int64) Value {
		set := NewSet()
		for _, n := range ns {
			set.Add(integerValue(n))
		}
		return NewSetValue(set)
	}
	symA, symB := &symbols.Symbol{Name: "a"}, &symbols.Symbol{Name: "b"}
	baseUnit := func(text string, sym *symbols.Symbol) Unit {
		return Unit{Text: text, Term: semantics.UnitTerm{Scale: semantics.UnitScale(1), Factors: []semantics.UnitFactor{{Unit: sym, Exponent: 1}}}}
	}
	metre, second := baseUnit("m", symA), baseUnit("s", symB)
	return map[ValueKind][2]Value{
		ValConst:       {integerValue(1), integerValue(2)},
		ValNull:        {{Kind: ValNull}, {Kind: ValNull}},
		ValString:      {NewStringValue("a"), NewStringValue("b")},
		ValInstance:    {{Kind: ValInstance, Instance: 1}, {Kind: ValInstance, Instance: 2}},
		ValSequence:    {seqOf(1, 2), seqOf(2, 1)},
		ValSet:         {setOf(1, 2), setOf(1, 3)},
		ValExpr:        {NewExprValue(&ast.LiteralInteger{Value: "1"}, nil), NewExprValue(&ast.LiteralInteger{Value: "2"}, nil)},
		ValQuantity:    {NewQuantityValue(&Quantity{Num: integerValue(1).Const, Unit: metre}), NewQuantityValue(&Quantity{Num: integerValue(2).Const, Unit: metre})},
		ValVariant:     {NewVariantValue(symA, 0), NewVariantValue(symB, 0)},
		ValEnumLiteral: {NewEnumLiteral(symA), NewEnumLiteral(symB)},
		ValComplex:     {NewComplex(complex(1, 2)), NewComplex(complex(1, 3))},
		ValArray:       {NewArrayValue([]int64{2, 2}, []Value{integerValue(1), integerValue(2), integerValue(3), integerValue(4)}), NewArrayValue([]int64{4}, []Value{integerValue(1), integerValue(2), integerValue(3), integerValue(4)})},
		ValVector:      {NewVectorValue([]semantics.Value{integerValue(1).Const, integerValue(2).Const}), NewVectorValue([]semantics.Value{integerValue(2).Const, integerValue(1).Const})},
		ValVectorQuantity: {
			NewVectorQuantityValue([]semantics.Value{integerValue(1).Const, integerValue(2).Const}, []Unit{metre, metre}),
			NewVectorQuantityValue([]semantics.Value{integerValue(1).Const, integerValue(2).Const}, []Unit{second, second}),
		},
	}
}

// TestEveryValueKindIsDispatched walks every ValueKind through the surfaces that
// switch on it — its name, its renderings, its description, equality and set
// keying — so a new kind cannot fall through to a fallback arm unnoticed.
func TestEveryValueKindIsDispatched(t *testing.T) {
	samples := kindSamples()
	for kind := ValInvalid + 1; kind < valueKindCount; kind++ {
		pair, ok := samples[kind]
		if !ok {
			t.Errorf("kind %d has no sample values: add two to kindSamples so its dispatch is checked", kind)
			continue
		}
		a, b := pair[0], pair[1]
		if a.Kind != kind || b.Kind != kind {
			t.Errorf("kind %d: samples are of kinds %s and %s", kind, a.Kind, b.Kind)
			continue
		}
		name := kind.String()
		if name == "invalid" {
			t.Errorf("kind %d: String() falls back to %q", kind, name)
		}
		if s := FormatValue(a); s == "<unknown>" {
			t.Errorf("%s: FormatValue falls back to %q", name, s)
		}
		if s := FormatTraceValue(a); s == name && kind != ValNull {
			t.Errorf("%s: FormatTraceValue falls back to the kind name", name)
		}
		if s := describeOperand(a); s == "a value" {
			t.Errorf("%s: describeOperand falls back to %q", name, s)
		}
		// An expression is a deferred body, not a value with an equality.
		if kind != ValExpr && !valueEqual(a, a) {
			t.Errorf("%s: valueEqual(a, a) is false", name)
		}
		if first, again := valueKeyFunc(a), valueKeyFunc(a); kind != ValExpr && first != again {
			t.Errorf("%s: valueKeyFunc is not stable: %v then %v", name, first, again)
		}
		// Null is one value; every other kind must tell its two samples apart.
		if kind == ValNull || kind == ValExpr {
			continue
		}
		if valueEqual(a, b) {
			t.Errorf("%s: valueEqual(%s, %s) is true", name, FormatValue(a), FormatValue(b))
		}
		if valueKeyFunc(a) == valueKeyFunc(b) {
			t.Errorf("%s: valueKeyFunc does not tell %s from %s", name, FormatValue(a), FormatValue(b))
		}
	}
}
