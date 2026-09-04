package runtime

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
)

// TestArrayAtRowMajor: one-based indexes address the row-major element, the
// last index varying fastest; a rank-0 array holds its one element.
func TestArrayAtRowMajor(t *testing.T) {
	grid := NewArrayValue([]int64{2, 3}, integers(1, 2, 3, 4, 5, 6)).Array()
	for _, tc := range []struct {
		indexes []int64
		want    int64
	}{
		{[]int64{1, 1}, 1},
		{[]int64{1, 3}, 3},
		{[]int64{2, 1}, 4},
		{[]int64{2, 3}, 6},
	} {
		got, err := grid.at("test", tc.indexes)
		if err != nil || got.Const.Int != tc.want {
			t.Errorf("at(%v) = %s, %v; want %d", tc.indexes, FormatValue(got), err, tc.want)
		}
	}
	scalar := NewArrayValue(nil, []Value{integerValue(7)}).Array()
	if got, err := scalar.at("test", nil); err != nil || got.Const.Int != 7 {
		t.Errorf("rank-0 at() = %s, %v; want 7", FormatValue(got), err)
	}
}

// TestArrayAtReportsRankAndRange: the wrong number of indexes and an index
// outside its dimension are typed errors naming the position.
func TestArrayAtReportsRankAndRange(t *testing.T) {
	grid := NewArrayValue([]int64{2, 3}, integers(1, 2, 3, 4, 5, 6)).Array()
	if _, err := grid.at("test", []int64{1}); !errors.Is(err, ErrMultiplicityViolation) || !strings.Contains(err.Error(), "1 indexes address an array of rank 2") {
		t.Errorf("at(1) = %v, want %v naming the rank", err, ErrMultiplicityViolation)
	}
	for _, indexes := range [][]int64{{0, 1}, {3, 1}, {1, 4}, {1, -1}} {
		if _, err := grid.at("test", indexes); !errors.Is(err, ErrIndexOutOfRange) {
			t.Errorf("at(%v) = %v, want %v", indexes, err, ErrIndexOutOfRange)
		}
	}
	// An array whose elements fall short of its shape reports the element, not a panic.
	ragged := NewArrayValue([]int64{2, 2}, integers(1, 2, 3)).Array()
	if _, err := ragged.at("test", []int64{2, 2}); !errors.Is(err, ErrIndexOutOfRange) || !strings.Contains(err.Error(), "[2, 2] addresses row-major offset 3, the array holds 3 element(s)") {
		t.Errorf("ragged at(2, 2) = %v, want %v naming offset 3 of 3 elements", err, ErrIndexOutOfRange)
	}
}

// TestArrayOverflowIsTyped: a shape whose flattened size, or an index whose
// row-major offset, does not fit an Integer is ErrArithmeticOverflow, never a
// wrapped size or a negative offset.
func TestArrayOverflowIsTyped(t *testing.T) {
	huge := int64(1) << 61
	if _, ok := flattenedSize([]int64{huge, 4}); ok {
		t.Errorf("flattenedSize(2^61, 4) fits, want overflow")
	}
	if size, ok := flattenedSize([]int64{huge, 3}); !ok || size != 3*huge {
		t.Errorf("flattenedSize(2^61, 3) = %d, %v; want 3 * 2^61", size, ok)
	}
	if _, err := arrayOf("test", []int64{huge, 4}, nil); !errors.Is(err, semantics.ErrArithmeticOverflow) || !strings.Contains(err.Error(), "flattenedSize") {
		t.Errorf("arrayOf(2^61, 4) = %v, want %v naming flattenedSize", err, semantics.ErrArithmeticOverflow)
	}

	// The far corner of a shape whose size fits, and the one past it whose
	// offset is exactly MaxInt64, are out of range and stay positive.
	for _, dims := range [][]int64{{huge, 3}, {huge, 4}} {
		corner := NewArrayValue(dims, nil).Array()
		_, err := corner.at("test", dims)
		if !errors.Is(err, ErrIndexOutOfRange) || strings.Contains(err.Error(), "offset -") {
			t.Errorf("at%v = %v, want %v with a non-negative offset", dims, err, ErrIndexOutOfRange)
		}
	}
	// A shape built around arrayOf reports the offset rather than wrapping it.
	vast := NewArrayValue([]int64{huge, 5}, nil).Array()
	if _, err := vast.at("test", []int64{huge, 5}); !errors.Is(err, semantics.ErrArithmeticOverflow) || !strings.Contains(err.Error(), "row-major offset") {
		t.Errorf("at(2^61, 5) = %v, want %v naming the row-major offset", err, semantics.ErrArithmeticOverflow)
	}
	if _, ok := rowMajorOffset(math.MaxInt64, 1, 1); ok {
		t.Errorf("rowMajorOffset(MaxInt64, 1, 1) fits, want overflow on the addition")
	}
	if got, ok := rowMajorOffset(3, 4, 2); !ok || got != 14 {
		t.Errorf("rowMajorOffset(3, 4, 2) = %d, %v; want 14", got, ok)
	}
}

func integers(ns ...int64) []Value {
	values := make([]Value, len(ns))
	for i, n := range ns {
		values[i] = integerValue(n)
	}
	return values
}
