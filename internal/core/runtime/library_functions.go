package runtime

import (
	"fmt"
	"math"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// libraryFunction is the built-in implementation of a KerML function library
// declaration the vendored library declares without a body. Its name is the
// fully-qualified name dispatch matches, and its parameters carry the names and
// the order the vendored signature declares, so a named argument binds to the
// parameter the library names.
type libraryFunction struct {
	name   string
	params []string
	apply  func(args []semantics.Value) (semantics.Value, error)
}

// libraryFunctions maps a function's fully-qualified name to its
// implementation. Dispatch is by qualified name, so a user-declared calc of the
// same local name resolves to itself and is never routed here.
var libraryFunctions = map[string]*libraryFunction{}

// libraryFunctionsByLocalName maps an unqualified name to the implementation a
// call denotes when the name resolves to no declaration in the model — the
// KerML function library is always in force, even in a model that imports no
// part of it. A name only appears here when every library declaration of it
// means the same operation.
var libraryFunctionsByLocalName = map[string]*libraryFunction{}

func init() {
	// RealFunctions (Kernel Function Library). `abs`, `max` and `min` take Real
	// parameters, so an Integer argument widens (ScalarValues declares
	// Integer :> Rational :> Real); `floor` and `round` return Integer.
	registerLibraryFunction("RealFunctions::sqrt", []string{"x"}, realUnary(math.Sqrt))
	registerLibraryFunction("RealFunctions::abs", []string{"x"}, realUnary(math.Abs))
	registerLibraryFunction("RealFunctions::floor", []string{"x"}, floorToInteger)
	registerLibraryFunction("RealFunctions::round", []string{"x"}, roundToInteger)
	registerLibraryFunction("RealFunctions::max", []string{"x", "y"}, realBinary(math.Max))
	registerLibraryFunction("RealFunctions::min", []string{"x", "y"}, realBinary(math.Min))

	// RationalFunctions declares the same three operations over Rational.
	registerLibraryFunction("RationalFunctions::abs", []string{"x"}, numericAbs)
	registerLibraryFunction("RationalFunctions::max", []string{"x", "y"}, numericExtremum(true))
	registerLibraryFunction("RationalFunctions::min", []string{"x", "y"}, numericExtremum(false))

	// NumericalFunctions declares them abstractly over NumericalValue, so the
	// result keeps the operands' kind: Integer arguments give an Integer.
	registerLibraryFunction("NumericalFunctions::abs", []string{"x"}, numericAbs)
	registerLibraryFunction("NumericalFunctions::max", []string{"x", "y"}, numericExtremum(true))
	registerLibraryFunction("NumericalFunctions::min", []string{"x", "y"}, numericExtremum(false))
	registerLibraryFunction("NumericalFunctions::isZero", []string{"x"}, isZero)
	registerLibraryFunction("NumericalFunctions::isUnit", []string{"x"}, isUnit)

	// IntegerFunctions and NaturalFunctions declare Integer parameters, so a
	// Real argument does not conform and is rejected rather than truncated.
	registerLibraryFunction("IntegerFunctions::abs", []string{"x"}, integerAbs)
	registerLibraryFunction("IntegerFunctions::max", []string{"x", "y"}, integerExtremum(true))
	registerLibraryFunction("IntegerFunctions::min", []string{"x", "y"}, integerExtremum(false))
	registerLibraryFunction("NaturalFunctions::max", []string{"x", "y"}, naturalExtremum(true))
	registerLibraryFunction("NaturalFunctions::min", []string{"x", "y"}, naturalExtremum(false))

	// TrigFunctions. The library names the angle `theta` and the inverse
	// functions `arcsin`/`arccos`/`arctan` over a `x` parameter; `tan` and `cot`
	// carry bodies there (sin/cos and cos/sin), which these compute directly.
	registerLibraryFunction("TrigFunctions::sin", []string{"theta"}, realUnary(math.Sin))
	registerLibraryFunction("TrigFunctions::cos", []string{"theta"}, realUnary(math.Cos))
	registerLibraryFunction("TrigFunctions::tan", []string{"theta"}, realUnary(tanReal))
	registerLibraryFunction("TrigFunctions::cot", []string{"theta"}, realUnary(cotReal))
	registerLibraryFunction("TrigFunctions::arcsin", []string{"x"}, realUnary(math.Asin))
	registerLibraryFunction("TrigFunctions::arccos", []string{"x"}, realUnary(math.Acos))
	registerLibraryFunction("TrigFunctions::arctan", []string{"x"}, realUnary(math.Atan))

	// The unqualified names, each mapped to the declaration a bare call denotes.
	// `abs`, `max` and `min` map to the kind-preserving NumericalFunctions
	// declaration the Real, Rational and Integer ones all specialize.
	registerLocalNames(map[string]string{
		"sqrt":   "RealFunctions::sqrt",
		"floor":  "RealFunctions::floor",
		"round":  "RealFunctions::round",
		"abs":    "NumericalFunctions::abs",
		"max":    "NumericalFunctions::max",
		"min":    "NumericalFunctions::min",
		"isZero": "NumericalFunctions::isZero",
		"isUnit": "NumericalFunctions::isUnit",
		"sin":    "TrigFunctions::sin",
		"cos":    "TrigFunctions::cos",
		"tan":    "TrigFunctions::tan",
		"cot":    "TrigFunctions::cot",
		"arcsin": "TrigFunctions::arcsin",
		"arccos": "TrigFunctions::arccos",
		"arctan": "TrigFunctions::arctan",
	})
}

// registerLibraryFunction adds one implementation to the registry.
func registerLibraryFunction(name string, params []string, apply func([]semantics.Value) (semantics.Value, error)) {
	libraryFunctions[name] = &libraryFunction{name: name, params: params, apply: apply}
}

// registerLocalNames records which declaration each unqualified name denotes.
func registerLocalNames(names map[string]string) {
	for local, fqn := range names {
		fn, ok := libraryFunctions[fqn]
		if !ok {
			panic("runtime: unqualified name " + local + " maps to unregistered library function " + fqn)
		}
		libraryFunctionsByLocalName[local] = fn
	}
}

// libraryFunctionByName returns the implementation of the function with that
// fully-qualified name.
func libraryFunctionByName(fqn string) (*libraryFunction, bool) {
	fn, ok := libraryFunctions[fqn]
	return fn, ok
}

// unresolvedLibraryFunction returns the library function a call denotes when its
// name resolves to no declaration: the library's own qualified name, or an
// unqualified name, which denotes the library function of that name. written is
// the name as the model wrote it.
func unresolvedLibraryFunction(qn *ast.QualifiedName, written string) (*libraryFunction, bool) {
	if fn, ok := libraryFunctions[written]; ok {
		return fn, true
	}
	if qn == nil || qn.Global || len(qn.Parts) != 1 {
		return nil, false
	}
	fn, ok := libraryFunctionsByLocalName[written]
	return fn, ok
}

// libraryFunctionFor returns the built-in implementation of sym when sym is a
// function library declaration the library gives no body to evaluate. A
// declaration that does carry a body is evaluated from that body, so a model
// that declares its own calc of the same name is never routed here.
func (ctx *Context) libraryFunctionFor(sym *symbols.Symbol) (*libraryFunction, bool) {
	// A declaration that is not a function is not one of these, whatever it is
	// named. A cached library symbol carries a kind and no Decl.
	if sym == nil || !isCalcSymbol(sym) {
		return nil, false
	}
	fn, ok := libraryFunctions[ctx.qualifiedSymbolName(sym)]
	if !ok || ctx.hasCalcBody(sym) {
		return nil, false
	}
	return fn, true
}

// invoke binds args to the declared parameters and applies the function,
// recording the trace events a calc invocation records so a built-in function
// appears in a trace like any other invocation.
func (fn *libraryFunction) invoke(ctx *Context, args calcArgs) (Value, error) {
	if ctx.trace != nil {
		ctx.trace.RecordCalcEnter(fn.name)
	}
	result, err := fn.bindAndApply(ctx, args)
	if ctx.trace != nil {
		if err != nil {
			ctx.trace.RecordCalcExitError(fn.name, err)
		} else {
			ctx.trace.RecordCalcExit(fn.name, result)
		}
	}
	return result, err
}

// bindAndApply resolves each declared parameter to an argument and applies the
// function. Every library parameter has multiplicity [1] and no default, so the
// argument count must match exactly.
func (fn *libraryFunction) bindAndApply(ctx *Context, args calcArgs) (Value, error) {
	if len(args.positional) > 0 && len(args.named) > 0 {
		return Value{}, fmt.Errorf("%w: function %s takes either positional or named arguments", ErrCalcArity, fn.name)
	}
	if len(args.positional) > 0 && len(args.positional) != len(fn.params) {
		return Value{}, fmt.Errorf(
			"%w: function %s takes %d argument(s), got %d",
			ErrCalcArity, fn.name, len(fn.params), len(args.positional),
		)
	}
	if len(args.named) > 0 && len(args.named) != len(fn.params) {
		return Value{}, fmt.Errorf(
			"%w: function %s takes %d argument(s), got %d",
			ErrCalcArity, fn.name, len(fn.params), len(args.named),
		)
	}
	if len(args.positional) == 0 && len(args.named) == 0 && len(fn.params) > 0 {
		return Value{}, fmt.Errorf(
			"%w: function %s takes %d argument(s), got 0",
			ErrCalcArity, fn.name, len(fn.params),
		)
	}

	values := make([]semantics.Value, len(fn.params))
	for i, param := range fn.params {
		var arg Value
		if len(args.positional) > 0 {
			arg = args.positional[i]
		} else {
			bound, ok := args.named[param]
			if !ok {
				return Value{}, fmt.Errorf(
					"%w: function %s has no input parameter matching the arguments given (expected %q)",
					ErrUnknownParameter, fn.name, param,
				)
			}
			arg = bound
		}
		if arg.Kind != ValConst || !arg.Const.IsNumeric() {
			return Value{}, fmt.Errorf(
				"%w: function %s parameter %q requires a numeric value",
				ErrTypeMismatch, fn.name, param,
			)
		}
		values[i] = arg.Const
		if ctx.trace != nil {
			ctx.trace.RecordCalcBind(param, arg, "argument")
		}
	}

	result, err := fn.apply(values)
	if err != nil {
		return Value{}, fmt.Errorf("function %s: %w", fn.name, err)
	}
	return Value{Kind: ValConst, Const: result}, nil
}

// realUnary adapts a one-argument function over the reals: the argument widens
// to Real and the result is a Real, which is what every such library
// declaration returns.
func realUnary(f func(float64) float64) func([]semantics.Value) (semantics.Value, error) {
	return func(args []semantics.Value) (semantics.Value, error) {
		x := asReal(args[0])
		res, err := realResult(f(x))
		if err != nil {
			return semantics.Value{}, fmt.Errorf("%w (argument %v)", err, x)
		}
		return res, nil
	}
}

// realBinary adapts a two-argument function over the reals.
func realBinary(f func(a, b float64) float64) func([]semantics.Value) (semantics.Value, error) {
	return func(args []semantics.Value) (semantics.Value, error) {
		x, y := asReal(args[0]), asReal(args[1])
		res, err := realResult(f(x, y))
		if err != nil {
			return semantics.Value{}, fmt.Errorf("%w (arguments %v, %v)", err, x, y)
		}
		return res, nil
	}
}

// tanReal is sin/cos, the ratio TrigFunctions::tan declares as its body. A zero
// cosine has no tangent and yields an infinity, which realResult reports.
func tanReal(theta float64) float64 { return math.Sin(theta) / math.Cos(theta) }

// cotReal is cos/sin, the ratio TrigFunctions::cot declares as its body.
func cotReal(theta float64) float64 { return math.Cos(theta) / math.Sin(theta) }

// floorToInteger is RealFunctions::floor, which returns Integer.
func floorToInteger(args []semantics.Value) (semantics.Value, error) {
	return integerResult(math.Floor(asReal(args[0])))
}

// roundToInteger is RealFunctions::round, which returns Integer. Halves round
// away from zero, as math.Round does.
func roundToInteger(args []semantics.Value) (semantics.Value, error) {
	return integerResult(math.Round(asReal(args[0])))
}

// numericAbs is the kind-preserving absolute value NumericalFunctions declares
// over NumericalValue: an Integer argument gives an Integer.
func numericAbs(args []semantics.Value) (semantics.Value, error) {
	if args[0].Kind == semantics.ValInt {
		return integerAbs(args)
	}
	return realResult(math.Abs(args[0].Real))
}

// integerAbs is the absolute value over Integer, which IntegerFunctions
// declares as returning Natural. The most negative int64 has no positive
// counterpart, so it overflows rather than wrapping to itself.
func integerAbs(args []semantics.Value) (semantics.Value, error) {
	x, err := asInteger(args[0])
	if err != nil {
		return semantics.Value{}, err
	}
	if x == math.MinInt64 {
		return semantics.Value{}, fmt.Errorf("%w: abs(%d) exceeds the Integer range", semantics.ErrArithmeticOverflow, x)
	}
	if x < 0 {
		x = -x
	}
	return semantics.Value{Kind: semantics.ValInt, Int: x}, nil
}

// numericExtremum is the kind-preserving max (larger=true) or min that
// NumericalFunctions declares over NumericalValue: two Integer arguments give
// an Integer, a mixed pair a Real.
func numericExtremum(larger bool) func([]semantics.Value) (semantics.Value, error) {
	return func(args []semantics.Value) (semantics.Value, error) {
		if args[0].Kind == semantics.ValInt && args[1].Kind == semantics.ValInt {
			return integerExtremum(larger)(args)
		}
		return realResult(pickReal(larger, asReal(args[0]), asReal(args[1])))
	}
}

// integerExtremum is max/min over Integer parameters.
func integerExtremum(larger bool) func([]semantics.Value) (semantics.Value, error) {
	return func(args []semantics.Value) (semantics.Value, error) {
		x, err := asInteger(args[0])
		if err != nil {
			return semantics.Value{}, err
		}
		y, err := asInteger(args[1])
		if err != nil {
			return semantics.Value{}, err
		}
		res := y
		if (larger && x > y) || (!larger && x < y) {
			res = x
		}
		return semantics.Value{Kind: semantics.ValInt, Int: res}, nil
	}
}

// naturalExtremum is max/min over the Natural parameters NaturalFunctions
// declares, so a negative argument does not conform.
func naturalExtremum(larger bool) func([]semantics.Value) (semantics.Value, error) {
	extremum := integerExtremum(larger)
	return func(args []semantics.Value) (semantics.Value, error) {
		for _, arg := range args {
			x, err := asInteger(arg)
			if err != nil {
				return semantics.Value{}, err
			}
			if x < 0 {
				return semantics.Value{}, fmt.Errorf("%w: requires Natural arguments, got %d", ErrTypeMismatch, x)
			}
		}
		return extremum(args)
	}
}

// isZero and isUnit are the NumericalFunctions predicates the library's sum0
// and product1 assert on their identity element.
func isZero(args []semantics.Value) (semantics.Value, error) {
	return semantics.Value{Kind: semantics.ValBool, Bool: asReal(args[0]) == 0}, nil
}

func isUnit(args []semantics.Value) (semantics.Value, error) {
	return semantics.Value{Kind: semantics.ValBool, Bool: asReal(args[0]) == 1}, nil
}

// pickReal returns the larger or the smaller of two reals.
func pickReal(larger bool, a, b float64) float64 {
	if larger {
		return math.Max(a, b)
	}
	return math.Min(a, b)
}

// asReal widens a numeric value to a float64. ScalarValues declares
// Integer :> Rational :> Real, so an Integer argument conforms to a Real
// parameter.
func asReal(v semantics.Value) float64 {
	if v.Kind == semantics.ValInt {
		return float64(v.Int)
	}
	return v.Real
}

// asInteger requires an Integer value: a Real does not conform to an Integer
// parameter, and silently truncating it would compute something the model did
// not ask for.
func asInteger(v semantics.Value) (int64, error) {
	if v.Kind != semantics.ValInt {
		return 0, fmt.Errorf("%w: requires an Integer argument, got a Real", ErrTypeMismatch)
	}
	return v.Int, nil
}

// realResult wraps a computed Real, reporting a result that is not a finite
// number instead of returning a NaN or an infinity: a NaN means the argument
// was outside the function's domain, an infinity that the result has no Real
// value.
func realResult(x float64) (semantics.Value, error) {
	switch {
	case math.IsNaN(x):
		return semantics.Value{}, fmt.Errorf("%w: argument outside the function's domain", semantics.ErrArithmeticDomain)
	case math.IsInf(x, 0):
		return semantics.Value{}, fmt.Errorf("%w: result is not a finite Real", semantics.ErrArithmeticOverflow)
	}
	return semantics.Value{Kind: semantics.ValReal, Real: x}, nil
}

// integerResult wraps a whole Real as an Integer, reporting a value outside the
// Integer range rather than wrapping it.
func integerResult(x float64) (semantics.Value, error) {
	if math.IsNaN(x) {
		return semantics.Value{}, fmt.Errorf("%w: argument outside the function's domain", semantics.ErrArithmeticDomain)
	}
	// MaxInt64 has no float64, so compare against 2^63, the next value up, which
	// does: a whole Real reaching it is already outside the Integer range.
	if math.IsInf(x, 0) || x >= -float64(math.MinInt64) || x < math.MinInt64 {
		return semantics.Value{}, fmt.Errorf("%w: %v exceeds the Integer range", semantics.ErrArithmeticOverflow, x)
	}
	return semantics.Value{Kind: semantics.ValInt, Int: int64(x)}, nil
}
