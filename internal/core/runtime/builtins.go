package runtime

import (
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// builtins maps the fully-qualified name of a Kernel Function Library function
// to its implementation. These are the functions whose parameters or results are
// collections, or whose arguments are bodies rather than values, which the
// scalar library registry (library_functions.go) cannot express: it applies to
// semantics.Values, while these need runtime values and an evaluation context to
// call a body with.
var builtins map[string]func(*EvalContext, []Value) (Value, error)

// builtinsByLocalName maps an unqualified name to the declaration a bare or
// arrow-form call denotes — `(1,2,3)->size()` and `size((1,2,3))` are
// `SequenceFunctions::size`. A name appears here only where every library
// declaration of it means the same operation, and dispatch reaches it only for
// a name the model itself declares nothing for, so a user-declared calc of the
// same name still resolves to itself.
var builtinsByLocalName map[string]func(*EvalContext, []Value) (Value, error)

// builtinLocalNames records which library declaration each unqualified name
// denotes, so the mapping can be listed as well as dispatched on.
var builtinLocalNames = map[string]string{
	"size":         "SequenceFunctions::size",
	"isEmpty":      "SequenceFunctions::isEmpty",
	"notEmpty":     "SequenceFunctions::notEmpty",
	"includes":     "SequenceFunctions::includes",
	"includesOnly": "SequenceFunctions::includesOnly",
	"excludes":     "SequenceFunctions::excludes",
	"equals":       "SequenceFunctions::equals",
	"same":         "SequenceFunctions::same",
	"union":        "SequenceFunctions::union",
	"intersection": "SequenceFunctions::intersection",
	"including":    "SequenceFunctions::including",
	"includingAt":  "SequenceFunctions::includingAt",
	"excluding":    "SequenceFunctions::excluding",
	"subsequence":  "SequenceFunctions::subsequence",
	"excludingAt":  "SequenceFunctions::excludingAt",
	"head":         "SequenceFunctions::head",
	"tail":         "SequenceFunctions::tail",
	"last":         "SequenceFunctions::last",
	"contains":     "CollectionFunctions::contains",
	"containsAll":  "CollectionFunctions::containsAll",
	"select":       "ControlFunctions::select",
	"selectOne":    "ControlFunctions::selectOne",
	"reject":       "ControlFunctions::reject",
	"collect":      "ControlFunctions::collect",
	"forAll":       "ControlFunctions::forAll",
	"exists":       "ControlFunctions::exists",
	"allTrue":      "ControlFunctions::allTrue",
	"anyTrue":      "ControlFunctions::anyTrue",
	"reduce":       "ControlFunctions::reduce",
	"minimize":     "ControlFunctions::minimize",
	"maximize":     "ControlFunctions::maximize",
	"sum":          "NumericalFunctions::sum",
	"product":      "NumericalFunctions::product",
}

func init() {
	builtins = map[string]func(*EvalContext, []Value) (Value, error){
		// SequenceFunctions: the operations on general sequences of values.
		"SequenceFunctions::#":            builtinSequenceIndex,
		"SequenceFunctions::size":         builtinSequenceSize,
		"SequenceFunctions::isEmpty":      builtinSequenceIsEmpty,
		"SequenceFunctions::notEmpty":     builtinSequenceNotEmpty,
		"SequenceFunctions::includes":     builtinSequenceIncludes,
		"SequenceFunctions::includesOnly": builtinSequenceIncludesOnly,
		"SequenceFunctions::excludes":     builtinSequenceExcludes,
		"SequenceFunctions::equals":       builtinSequenceEquals,
		"SequenceFunctions::same":         builtinSequenceSame,
		"SequenceFunctions::union":        builtinSequenceUnion,
		"SequenceFunctions::intersection": builtinSequenceIntersection,
		"SequenceFunctions::including":    builtinSequenceIncluding,
		"SequenceFunctions::includingAt":  builtinSequenceIncludingAt,
		"SequenceFunctions::excluding":    builtinSequenceExcluding,
		"SequenceFunctions::subsequence":  builtinSequenceSubsequence,
		"SequenceFunctions::excludingAt":  builtinSequenceExcludingAt,
		"SequenceFunctions::head":         builtinSequenceHead,
		"SequenceFunctions::tail":         builtinSequenceTail,
		"SequenceFunctions::last":         builtinSequenceLast,

		// CollectionFunctions: the same operations on a Collection, each defined
		// by the library as the sequence operation over the collection's
		// elements.
		"CollectionFunctions::#":           builtinSequenceIndex,
		"CollectionFunctions::size":        builtinSequenceSize,
		"CollectionFunctions::isEmpty":     builtinSequenceIsEmpty,
		"CollectionFunctions::notEmpty":    builtinSequenceNotEmpty,
		"CollectionFunctions::contains":    builtinCollectionContains,
		"CollectionFunctions::containsAll": builtinCollectionContainsAll,
		"CollectionFunctions::head":        builtinSequenceHead,
		"CollectionFunctions::tail":        builtinSequenceTail,
		"CollectionFunctions::last":        builtinSequenceLast,

		// ControlFunctions: the operations whose argument is a body the
		// operation itself decides the evaluation of.
		"ControlFunctions::select":    builtinControlSelect,
		"ControlFunctions::selectOne": builtinControlSelectOne,
		"ControlFunctions::reject":    builtinControlReject,
		"ControlFunctions::collect":   builtinControlCollect,
		"ControlFunctions::forAll":    builtinControlForAll,
		"ControlFunctions::exists":    builtinControlExists,
		"ControlFunctions::allTrue":   builtinControlAllTrue,
		"ControlFunctions::anyTrue":   builtinControlAnyTrue,
		"ControlFunctions::reduce":    builtinControlReduce,
		"ControlFunctions::minimize":  builtinControlMinimize,
		"ControlFunctions::maximize":  builtinControlMaximize,

		// NumericalFunctions::sum and ::product, and the specializations that
		// fix the identity element of an empty aggregation: sum of Integers is
		// `sum0(collection, 0)` and product is `product1(collection, 1)`. The
		// result keeps the elements' kind, so IntegerFunctions::sum of Integers
		// is an Integer, as its `return : Integer[1]` declares.
		// ComplexFunctions and VectorFunctions declare sum and product too, over
		// values this runtime has no representation of; they are left out rather
		// than answered wrongly (docs/project/spec-compliance.md).
		"NumericalFunctions::sum":     builtinNumericalSum,
		"NumericalFunctions::product": builtinNumericalProduct,
		// IntegerFunctions::'..', the range, whose result the library declares
		// `Integer[0..*]`: an ordered sequence, not a value kind of its own.
		"IntegerFunctions::..": builtinIntegerRange,

		"IntegerFunctions::sum":      builtinNumericalSum,
		"IntegerFunctions::product":  builtinNumericalProduct,
		"RationalFunctions::sum":     builtinNumericalSum,
		"RationalFunctions::product": builtinNumericalProduct,
		"RealFunctions::sum":         builtinNumericalSum,
		"RealFunctions::product":     builtinNumericalProduct,
	}

	builtinsByLocalName = map[string]func(*EvalContext, []Value) (Value, error){}
	for local, fqn := range builtinLocalNames {
		fn, ok := builtins[fqn]
		if !ok {
			panic("runtime: unqualified name " + local + " maps to unregistered built-in " + fqn)
		}
		builtinsByLocalName[local] = fn
	}
}

// builtinFor returns the implementation of the library declaration sym is,
// where it is one of the collection functions. Unlike the scalar library
// functions, these declarations do carry bodies — SequenceFunctions::size is
// defined recursively as `if isEmpty(seq)? 0 else size(tail(seq)) + 1` — but
// the body is the specification of the operation, not the way to compute it, so
// a name that denotes the library declaration is computed by the implementation
// of that operation. A calc the model declares itself resolves to its own
// symbol, whose qualified name is not one of these.
func (ctx *Context) builtinFor(sym *symbols.Symbol) (func(*EvalContext, []Value) (Value, error), bool) {
	if sym == nil {
		return nil, false
	}
	fn, ok := builtins[ctx.qualifiedSymbolName(sym)]
	return fn, ok
}

// boolValue wraps a Boolean result.
func boolValue(b bool) Value {
	return Value{
		Kind: ValConst,
		Const: semantics.Value{
			Kind: semantics.ValBool,
			Bool: b,
		},
	}
}
