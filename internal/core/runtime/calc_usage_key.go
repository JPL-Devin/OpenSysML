package runtime

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/Open-MBEE/Systemica/internal/core/semantics"
)

// A calc usage nested in a calc binds its inputs from the evaluation reading it,
// so the same usage read from two invocations of the enclosing calc computes two
// different things. The values it bound to are therefore part of what identifies
// its evaluation, and are rendered here as a key: two argument tuples render
// alike only when every value in them is the same value.

// calcInputsKey renders the values a calc usage's inputs bound to, in parameter
// order, as the bindings part of the key identifying one evaluation of it.
func calcInputsKey(shape *calcShape, env map[string]Value) string {
	var b strings.Builder
	for _, param := range shape.Params {
		b.WriteString(param.Name)
		b.WriteByte('=')
		b.WriteString(valueKeyString(env[param.Name]))
		b.WriteByte(';')
	}
	return b.String()
}

// calcEnvKey renders the bindings of the evaluation reading a calc usage, for a
// usage whose own output bindings that environment answers.
func calcEnvKey(reader *EvalContext) string {
	var b strings.Builder
	for _, frame := range reader.frames {
		names := make([]string, 0, len(frame))
		for name := range frame {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			b.WriteString(name)
			b.WriteByte('=')
			b.WriteString(valueKeyString(frame[name]))
			b.WriteByte(';')
		}
		b.WriteByte('|')
	}
	return b.String()
}

// valueKeyString renders a value so that no two values of different kinds or
// contents render alike: each kind is tagged, collections are rendered
// element-wise (a set by its sorted elements, which do not depend on insertion
// order), and an unevaluated expression by the node it holds.
func valueKeyString(v Value) string {
	switch v.Kind {
	case ValConst:
		return "c" + strconv.Itoa(int(v.Const.Kind)) + ":" + constKeyString(v.Const)
	case ValNull:
		return "null"
	case ValString:
		return "s" + strconv.Quote(v.Str)
	case ValInstance:
		return "i" + strconv.FormatInt(v.Instance, 10)
	case ValSequence:
		if v.Sequence == nil {
			return "q()"
		}
		return "q(" + strings.Join(elementKeyStrings(v.Sequence.Elements(), false), ",") + ")"
	case ValSet:
		if v.Set == nil {
			return "t{}"
		}
		return "t{" + strings.Join(elementKeyStrings(v.Set.Elements(), true), ",") + "}"
	case ValQuantity:
		if v.Quantity == nil {
			return "u"
		}
		return "u" + constKeyString(v.Quantity.Num) +
			"[" + v.Quantity.Unit.String() + "|" + v.Quantity.Unit.Term.DimensionKey() + "]"
	case ValExpr:
		// Two delayed expressions are the same only as the same node: an
		// unevaluated body has no value to compare.
		return fmt.Sprintf("e%p", v.Expr)
	default:
		// An invalid value carries no content of its own to render.
		return "k" + strconv.Itoa(int(v.Kind))
	}
}

// elementKeyStrings renders the elements of a collection, sorted when the
// collection is unordered.
func elementKeyStrings(elements []Value, unordered bool) []string {
	rendered := make([]string, 0, len(elements))
	for _, element := range elements {
		rendered = append(rendered, valueKeyString(element))
	}
	if unordered {
		sort.Strings(rendered)
	}
	return rendered
}

// constKeyString renders a model-level constant, a real by its bits so that two
// reals render alike exactly when they are the same float.
func constKeyString(c semantics.Value) string {
	switch c.Kind {
	case semantics.ValInt:
		return strconv.FormatInt(c.Int, 10)
	case semantics.ValReal:
		return strconv.FormatUint(math.Float64bits(c.Real), 16)
	case semantics.ValBool:
		return strconv.FormatBool(c.Bool)
	default:
		// Infinity and the invalid constant carry no magnitude of their own.
		return "-"
	}
}
