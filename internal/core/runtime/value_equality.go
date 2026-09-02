package runtime

import (
	"hash/fnv"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// valueKey is a comparable projection of Value for use as a map key.
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
	literal *symbols.Symbol
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
		key.strVal = v.Str()
	case ValInstance:
		key.instID = v.Instance
	case ValSequence:
		key.colHash = hashSequence(v.Sequence())
	case ValSet:
		key.colHash = hashSet(v.Set())
	case ValVariant:
		key.variant = v.Variant()
	case ValEnumLiteral:
		key.literal = v.Literal()
	case ValQuantity:
		if v.Quantity() != nil {
			key.realVal = v.Quantity().baseMagnitude()
			key.strVal = v.Quantity().Unit.Term.DimensionKey()
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
	var sum uint64
	for _, bucket := range set.elements {
		for _, elem := range bucket {
			k := valueKeyFunc(elem)
			// #nosec G115 -- wrapping is intended: this is a hash, not arithmetic.
			sum += uint64(k.intVal)
		}
	}
	return sum
}
