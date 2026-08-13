package runtime

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/semantics"
)

func intConstValue(n int64) Value {
	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: n}}
}

func realConstValue(x float64) Value {
	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: x}}
}

func sequenceValue(elements ...Value) Value {
	seq := NewSequence()
	for _, element := range elements {
		seq.Append(element)
	}
	return Value{Kind: ValSequence, Sequence: seq}
}

func setValue(elements ...Value) Value {
	set := NewSet()
	for _, element := range elements {
		set.Add(element)
	}
	return Value{Kind: ValSet, Set: set}
}

// TestCalcInputsKeyDistinguishesValues: the bindings part of a calc usage's key
// identifies the values the inputs bound to, so two argument tuples share a key
// only when they are the same values.
func TestCalcInputsKeyDistinguishesValues(t *testing.T) {
	shape := &calcShape{Params: []calcParameter{{Name: "a"}, {Name: "b"}}}

	key := func(a, b Value) string {
		return calcInputsKey(shape, map[string]Value{"a": a, "b": b})
	}

	same := []struct {
		name   string
		x1, x2 Value
		y1, y2 Value
	}{
		{"equal integers", intConstValue(3), intConstValue(3), intConstValue(4), intConstValue(4)},
		{"equal reals", realConstValue(1.5), realConstValue(1.5), realConstValue(-0.25), realConstValue(-0.25)},
		{"strings", Value{Kind: ValString, Str: "x"}, Value{Kind: ValString, Str: "x"}, Value{Kind: ValNull}, Value{Kind: ValNull}},
		{
			"sets differing only in insertion order",
			setValue(intConstValue(1), intConstValue(2)), setValue(intConstValue(2), intConstValue(1)),
			intConstValue(0), intConstValue(0),
		},
	}
	for _, c := range same {
		if got, want := key(c.x2, c.y2), key(c.x1, c.y1); got != want {
			t.Errorf("%s: key = %q, want %q", c.name, got, want)
		}
	}

	distinct := map[string][2][2]Value{
		"different integers":                 {{intConstValue(3), intConstValue(4)}, {intConstValue(4), intConstValue(3)}},
		"an integer and the same real":       {{intConstValue(1), intConstValue(0)}, {realConstValue(1), intConstValue(0)}},
		"zeros of different sign":            {{realConstValue(0), intConstValue(0)}, {realConstValue(negativeZero()), intConstValue(0)}},
		"null and a missing binding":         {{Value{Kind: ValNull}, intConstValue(0)}, {Value{}, intConstValue(0)}},
		"a string and an instance":           {{Value{Kind: ValString, Str: "1"}, intConstValue(0)}, {Value{Kind: ValInstance, Instance: 1}, intConstValue(0)}},
		"sequences of the same length":       {{sequenceValue(intConstValue(1), intConstValue(2)), intConstValue(0)}, {sequenceValue(intConstValue(2), intConstValue(1)), intConstValue(0)}},
		"a sequence and a set":               {{sequenceValue(intConstValue(1)), intConstValue(0)}, {setValue(intConstValue(1)), intConstValue(0)}},
		"an empty set and an empty sequence": {{setValue(), intConstValue(0)}, {sequenceValue(), intConstValue(0)}},
		"values swapped between inputs":      {{intConstValue(1), intConstValue(2)}, {intConstValue(2), intConstValue(1)}},
		"a string spelling a delimiter":      {{Value{Kind: ValString, Str: `x";b=s"y`}, Value{Kind: ValString, Str: ""}}, {Value{Kind: ValString, Str: "x"}, Value{Kind: ValString, Str: "y"}}},
	}
	for name, pair := range distinct {
		first, second := key(pair[0][0], pair[0][1]), key(pair[1][0], pair[1][1])
		if first == second {
			t.Errorf("%s: both tuples key as %q", name, first)
		}
	}
}

// negativeZero is the float whose bits differ from zero's while comparing equal
// to it, which the key has to keep apart from it.
func negativeZero() float64 {
	zero := 0.0
	return -zero
}

// TestCalcInputsKeyIgnoresUnboundParameters: a shape with no parameters has one
// key, whatever else the environment holds.
func TestCalcInputsKeyIgnoresUnboundParameters(t *testing.T) {
	shape := &calcShape{}
	if got := calcInputsKey(shape, map[string]Value{"stray": intConstValue(1)}); got != "" {
		t.Errorf("key = %q, want the empty key", got)
	}
}
