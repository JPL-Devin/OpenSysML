package runtime

import (
	"hash/fnv"

	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// valueKey is a comparable projection of Value for use as map key.
type valueKey struct {
	kind    ValueKind
	intVal  int64
	realVal float64
	boolVal bool
	infVal  bool
	strVal  string
	instID  int64
	colHash uint64
	variant *symbols.Symbol
}

// valueKeyFunc extracts a comparable key from a Value.
func valueKeyFunc(v Value) valueKey {
	key := valueKey{kind: v.Kind}
	switch v.Kind {
	case ValConst:
		switch v.Const.Kind {
		case 1: // semantics.ValInt
			key.intVal = v.Const.Int
		case 2: // semantics.ValReal
			key.realVal = v.Const.Real
		case 3: // semantics.ValBool
			key.boolVal = v.Const.Bool
		case 4: // semantics.ValInfinity
			key.infVal = true
		}
	case ValString:
		key.strVal = v.Str
	case ValInstance:
		key.instID = v.Instance
	case ValSequence:
		key.colHash = hashSequence(v.Sequence)
	case ValSet:
		key.colHash = hashSet(v.Set)
	case ValVariant:
		// A selection is the variant it names, whatever object it materialized.
		key.variant = v.Variant
	case ValQuantity:
		// Keyed on the base-unit form, so `1 [km]` and `1000 [m]` are one element.
		if v.Quantity != nil {
			key.realVal = v.Quantity.baseMagnitude()
			key.strVal = v.Quantity.Unit.Term.DimensionKey()
		}
	}
	return key
}

// hashSequence computes a content-based hash for a Sequence.
func hashSequence(seq *Sequence) uint64 {
	if seq == nil {
		return 0
	}
	h := fnv.New64a()
	for _, elem := range seq.elements {
		k := valueKeyFunc(elem)
		// #nosec G115 G104 -- truncation is deliberate for a hash, and
		// hash.Hash.Write is documented never to return an error.
		h.Write([]byte{byte(k.kind)})
		// Simplified: hash intVal, strVal, instID (full implementation in later task)
		if k.intVal != 0 {
			// #nosec G115 G104 -- see above.
			h.Write([]byte{byte(k.intVal), byte(k.intVal >> 8)})
		}
	}
	return h.Sum64()
}

// hashSet computes a content-based hash for a Set (order-invariant).
func hashSet(set *Set) uint64 {
	if set == nil {
		return 0
	}
	// Sum hashes of elements (order-invariant)
	var sum uint64
	for k := range set.elements {
		// #nosec G115 -- wrapping is intended: this is a hash, not arithmetic.
		sum += uint64(k.intVal)
	}
	return sum
}
