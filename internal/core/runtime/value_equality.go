package runtime

import (
	"hash/fnv"
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
	}
	return key
}

// hashSequence computes a content-based hash for a Sequence.
func hashSequence(seq *Sequence) uint64 {
	h := fnv.New64a()
	for _, elem := range seq.elements {
		k := valueKeyFunc(elem)
		h.Write([]byte{byte(k.kind)})
		// Simplified: hash intVal, strVal, instID (full implementation in later task)
		if k.intVal != 0 {
			h.Write([]byte{byte(k.intVal), byte(k.intVal >> 8)})
		}
	}
	return h.Sum64()
}

// hashSet computes a content-based hash for a Set (order-invariant).
func hashSet(set *Set) uint64 {
	// Sum hashes of elements (order-invariant)
	var sum uint64
	for k := range set.elements {
		sum += uint64(k.intVal) // Simplified
	}
	return sum
}
