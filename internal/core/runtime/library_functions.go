package runtime

import (
	"errors"
	"fmt"
	"math"
	"math/cmplx"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// ErrUnevaluableLibraryFunction is returned for a function library declaration
// this runtime has no representation for the values of. It names the function,
// so a model is told which declaration it is rather than answered wrongly.
var ErrUnevaluableLibraryFunction = errors.New("library function is not evaluable")

// ErrUnimportedExtensionFunction is returned for an unqualified call to a
// OpenSysML extension function the model imports no declaration of.
var ErrUnimportedExtensionFunction = errors.New("function is not in scope")

// noVectorCollection is why the aggregations over a collection of vectors are
// not evaluable: a sequence of them flattens, losing the grouping the
// aggregation sums over.
const noVectorCollection = "a collection of vectors has no representation: " +
	"a sequence of vectors flattens into one sequence of elements"

// libraryPi is the value of TrigFunctions::pi. The library fixes it by an
// invariant rather than a value (round(pi * 1E20) == 314159265358979323846.0),
// 21 significant digits, of which a Real carries the nearest float64.
const libraryPi = math.Pi

// libraryFunction is the built-in implementation of a function library
// declaration. Its name is the fully-qualified name dispatch matches, and its
// parameters carry the names and order the declared signature gives.
type libraryFunction struct {
	name   string
	params []string
	// required is how many leading parameters the signature declares [1]; the
	// rest are declared [0..1] and bind null where a call omits them.
	required int
	apply    libraryApply
	// scalar marks a function over numeric scalars alone, which the compiled
	// calc tier may call with unboxed arguments.
	scalar bool
}

// libraryApply computes one library function. It is passed the name it was
// dispatched under, so an implementation shared by several declarations reports
// the one the model called.
type libraryApply func(name string, ctx *Context, args []Value) (Value, error)

// writtenName is fqn as a model writes it: a segment that is no basic name, an
// operator or a keyword, in the quotes of an unrestricted name.
func writtenName(fqn string) string {
	parts := strings.Split(fqn, "::")
	for i, part := range parts {
		if !lexer.IsIdentifier(part) || lexer.IsKeywordIn(part, source.KindKerML) {
			parts[i] = "'" + part + "'"
		}
	}
	return strings.Join(parts, "::")
}

// libraryFunctions maps a function's fully-qualified name to its
// implementation. Dispatch is by qualified name, so a user-declared calc of the
// same local name resolves to itself and is never routed here.
var libraryFunctions = map[string]*libraryFunction{}

// libraryFunctionsByLocalName maps an unqualified name to the implementation a
// call denotes when the name resolves to no declaration in the model — the OMG
// function libraries are always in force, even in a model that imports no part
// of them. A name only appears here when every library declaration of it
// means the same operation.
var libraryFunctionsByLocalName = map[string]*libraryFunction{}

// extensionLocalNames maps the unqualified name of an OpenSysML extension
// function to the package an import must name for such a call to be legal. No
// OMG library declares these names, so nothing puts them in scope on its own.
var extensionLocalNames = map[string]string{
	"exp":   "OpenSysMLMathFunctions",
	"ln":    "OpenSysMLMathFunctions",
	"log":   "OpenSysMLMathFunctions",
	"atan2": "OpenSysMLMathFunctions",
}

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

	// `deg` and `rad` carry bodies in the library — theta * 180 / pi and
	// theta * pi / 180 — whose only unknown is TrigFunctions::pi, which the
	// feature-value seam below supplies. These compute the same conversion.
	registerLibraryFunction("TrigFunctions::deg", []string{"theta_rad"}, degreesFromRadians)
	registerLibraryFunction("TrigFunctions::rad", []string{"theta_deg"}, radiansFromDegrees)

	registerVectorFunctions()
	registerComplexFunctions()
	registerStringFunctions()
	registerConversionFunctions()
	registerGenericExtrema()
	registerOperatorFunctions()
	registerUnevaluableDeclarations()

	// OpenSysMLMathFunctions is the non-normative OpenSysML extension library
	// (internal/core/libs/stdlib/OpenSysML Libraries/OpenSysMLMathFunctions.kerml),
	// which declares the exponential, logarithmic and two-argument arctangent
	// functions the OMG Kernel Function Library omits.
	registerLibraryFunction("OpenSysMLMathFunctions::exp", []string{"x"}, realUnary(math.Exp))
	registerLibraryFunction("OpenSysMLMathFunctions::ln", []string{"x"}, naturalLog)
	registerLibraryFunction("OpenSysMLMathFunctions::log", []string{"x", "base"}, logToBase)
	registerLibraryFunction("OpenSysMLMathFunctions::atan2", []string{"y", "x"}, atan2Real)

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

		"deg": "TrigFunctions::deg",
		"rad": "TrigFunctions::rad",

		// VectorFunctions and ComplexFunctions: the names only one of the two
		// packages declares, each mapped to the declaration that computes it for
		// every vector or Complex this runtime represents.
		"VectorOf":               "VectorFunctions::VectorOf",
		"CartesianVectorOf":      "VectorFunctions::CartesianVectorOf",
		"CartesianThreeVectorOf": "VectorFunctions::CartesianThreeVectorOf",
		"isZeroVector":           "VectorFunctions::isZeroVector",
		"isCartesianZeroVector":  "VectorFunctions::isCartesianZeroVector",
		"scalarVectorMult":       "VectorFunctions::scalarVectorMult",
		"vectorScalarMult":       "VectorFunctions::vectorScalarMult",
		"vectorScalarDiv":        "VectorFunctions::vectorScalarDiv",
		"inner":                  "VectorFunctions::inner",
		"norm":                   "VectorFunctions::norm",
		"angle":                  "VectorFunctions::angle",
		"rect":                   "ComplexFunctions::rect",
		"polar":                  "ComplexFunctions::polar",
		"re":                     "ComplexFunctions::re",
		"im":                     "ComplexFunctions::im",
		"arg":                    "ComplexFunctions::arg",

		// StringFunctions: the two names no other function library declares.
		"Length":    "StringFunctions::Length",
		"Substring": "StringFunctions::Substring",
	})
}

// registerVectorFunctions registers VectorFunctions (Kernel Function Library).
// A vector is the sequence of its elements, which is what the library's own
// NumericalVectorValue is: `elements` with a `dimension` equal to their number.
// The abstract declarations over VectorValue and their Cartesian specializations
// compute alike over that representation, so both are registered.
func registerVectorFunctions() {
	registerValueFunction("VectorFunctions::isZeroVector", []string{"v"}, 1, vectorIsZero)
	registerValueFunction("VectorFunctions::isCartesianZeroVector", []string{"v"}, 1, vectorIsZero)
	registerValueFunction("VectorFunctions::+", []string{"v", "w"}, 1, vectorAdd)
	registerValueFunction("VectorFunctions::cartesian+", []string{"v", "w"}, 1, vectorAdd)
	registerValueFunction("VectorFunctions::-", []string{"v", "w"}, 1, vectorSubtract)
	registerValueFunction("VectorFunctions::cartesian-", []string{"v", "w"}, 1, vectorSubtract)
	registerValueFunction("VectorFunctions::VectorOf", []string{"components"}, 1, vectorOf)
	registerValueFunction("VectorFunctions::CartesianVectorOf", []string{"components"}, 1, cartesianVectorOf)
	registerValueFunction("VectorFunctions::CartesianThreeVectorOf", []string{"components"}, 1, cartesianThreeVectorOf)
	registerValueFunction("VectorFunctions::inner", []string{"v", "w"}, 2, vectorInner)
	registerValueFunction("VectorFunctions::cartesianInner", []string{"v", "w"}, 2, vectorInner)
	registerValueFunction("VectorFunctions::norm", []string{"v"}, 1, vectorNorm)
	registerValueFunction("VectorFunctions::cartesianNorm", []string{"v"}, 1, vectorNorm)
	registerValueFunction("VectorFunctions::angle", []string{"v", "w"}, 2, vectorAngle)
	registerValueFunction("VectorFunctions::cartesianAngle", []string{"v", "w"}, 2, vectorAngle)

	// scalarVectorMult takes the scalar first and vectorScalarMult the vector,
	// and the library aliases '*' for the former.
	registerValueFunction("VectorFunctions::scalarVectorMult", []string{"x", "v"}, 2, scalarVectorMult)
	registerValueFunction("VectorFunctions::*", []string{"x", "v"}, 2, scalarVectorMult)
	registerValueFunction("VectorFunctions::cartesianScalarVectorMult", []string{"x", "v"}, 2, scalarVectorMult)
	registerValueFunction("VectorFunctions::vectorScalarMult", []string{"v", "x"}, 2, vectorScalarMult)
	registerValueFunction("VectorFunctions::cartesianVectorScalarMult", []string{"v", "x"}, 2, vectorScalarMult)
	registerValueFunction("VectorFunctions::vectorScalarDiv", []string{"v", "x"}, 2, vectorScalarDiv)

	registerUnevaluable("VectorFunctions::sum0", []string{"coll", "zero"}, 2, noVectorCollection)
	registerUnevaluable("VectorFunctions::sum", []string{"coll"}, 1, noVectorCollection)
}

// registerComplexFunctions registers ComplexFunctions (Kernel Function Library).
// A Complex is one ValComplex value, and a Real is a Complex with a zero
// imaginary part (ScalarValues declares Real :> Complex), so both bind to a
// Complex parameter.
func registerComplexFunctions() {
	registerValueFunction("ComplexFunctions::rect", []string{"re", "im"}, 2, complexRect)
	registerValueFunction("ComplexFunctions::polar", []string{"abs", "arg"}, 2, complexPolar)
	registerValueFunction("ComplexFunctions::re", []string{"x"}, 1, complexRealPart)
	registerValueFunction("ComplexFunctions::im", []string{"x"}, 1, complexImagPart)
	registerValueFunction("ComplexFunctions::isZero", []string{"x"}, 1, complexIsZero)
	registerValueFunction("ComplexFunctions::isUnit", []string{"x"}, 1, complexIsUnit)
	registerValueFunction("ComplexFunctions::abs", []string{"x"}, 1, complexModulus)
	registerValueFunction("ComplexFunctions::arg", []string{"x"}, 1, complexArgument)
	registerValueFunction("ComplexFunctions::+", []string{"x", "y"}, 1, complexAdd)
	registerValueFunction("ComplexFunctions::-", []string{"x", "y"}, 1, complexSubtract)
	registerValueFunction("ComplexFunctions::*", []string{"x", "y"}, 2, complexMultiply)
	registerValueFunction("ComplexFunctions::/", []string{"x", "y"}, 2, complexDivide)
	registerValueFunction("ComplexFunctions::**", []string{"x", "y"}, 2, complexPower)
	registerValueFunction("ComplexFunctions::^", []string{"x", "y"}, 2, complexPower)
	registerValueFunction("ComplexFunctions::==", []string{"x", "y"}, 0, complexEquals)

	registerValueFunction("ComplexFunctions::sum", []string{"collection"}, 1, complexSum)
	registerValueFunction("ComplexFunctions::product", []string{"collection"}, 1, complexProduct)

	// The two string conversions of a Complex: the library defines no notation
	// for one, and inventing a rendering would make ToComplex(ToString(x)) a
	// value nothing else in the library agrees on.
	registerUnevaluable("ComplexFunctions::ToString", []string{"x"}, 1,
		"no string notation for a Complex value is defined")
	registerUnevaluable("ComplexFunctions::ToComplex", []string{"x"}, 1,
		"no string notation for a Complex value is defined")
}

// registerStringFunctions registers every function StringFunctions declares:
// '+', the four comparisons, '==', Length, Substring and ToString.
func registerStringFunctions() {
	registerValueFunction("StringFunctions::+", []string{"x", "y"}, 2, stringConcat)
	registerValueFunction("StringFunctions::Length", []string{"x"}, 1, stringLength)
	registerValueFunction("StringFunctions::Substring", []string{"x", "lower", "upper"}, 3, stringSubstring)
	registerValueFunction("StringFunctions::<", []string{"x", "y"}, 2, stringOrdering(ast.OpLt))
	registerValueFunction("StringFunctions::>", []string{"x", "y"}, 2, stringOrdering(ast.OpGt))
	registerValueFunction("StringFunctions::<=", []string{"x", "y"}, 2, stringOrdering(ast.OpLe))
	registerValueFunction("StringFunctions::>=", []string{"x", "y"}, 2, stringOrdering(ast.OpGe))
	registerValueFunction("StringFunctions::==", []string{"x", "y"}, 0, stringEquals)
	registerValueFunction("StringFunctions::ToString", []string{"x"}, 1, stringToString)
}

// registerLibraryFunction adds one implementation over scalar numeric
// arguments, which is what most of the numeric library declares.
func registerLibraryFunction(name string, params []string, apply func([]semantics.Value) (semantics.Value, error)) {
	registerValueFunction(name, params, len(params), numericScalars(params, apply))
	libraryFunctions[name].scalar = true
}

// registerValueFunction adds one implementation over runtime values, for the
// declarations whose parameters or results are not scalars: a vector, a Complex,
// or a parameter the signature declares [0..1].
func registerValueFunction(name string, params []string, required int, apply libraryApply) {
	libraryFunctions[name] = &libraryFunction{name: name, params: params, required: required, apply: apply}
}

// registerUnevaluable registers a declaration this runtime cannot evaluate, so
// that a call to it is reported by name with the reason rather than resolving to
// a library body that computes something else or to no declaration at all.
func registerUnevaluable(name string, params []string, required int, reason string) {
	registerValueFunction(name, params, required, func(called string, _ *Context, _ []Value) (Value, error) {
		return Value{}, fmt.Errorf("%w: %s: %s", ErrUnevaluableLibraryFunction, called, reason)
	})
}

// numericScalars adapts an implementation over scalar numeric values: every
// parameter of such a declaration is one number, so a collection, a string or an
// instance does not conform to it.
func numericScalars(params []string, apply func([]semantics.Value) (semantics.Value, error)) libraryApply {
	return func(name string, _ *Context, args []Value) (Value, error) {
		values := make([]semantics.Value, len(args))
		for i, arg := range args {
			if arg.Kind != ValConst || !arg.Const.IsNumeric() {
				return Value{}, fmt.Errorf(
					"%w: function %s parameter %q requires a numeric value",
					ErrTypeMismatch, name, params[i],
				)
			}
			values[i] = arg.Const
		}
		result, err := apply(values)
		if err != nil {
			return Value{}, fmt.Errorf("function %s: %w", name, err)
		}
		return Value{Kind: ValConst, Const: result}, nil
	}
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
// unqualified name of an OMG library function. written is the name as the model
// wrote it. An unqualified name only an OpenSysML extension declares gives a
// typed error naming the import that makes the call legal.
func unresolvedLibraryFunction(qn *ast.QualifiedName, written string) (*libraryFunction, error) {
	if fn, ok := libraryFunctions[written]; ok {
		return fn, nil
	}
	if qn == nil || qn.Global || len(qn.Parts) != 1 {
		return nil, nil
	}
	if fn, ok := libraryFunctionsByLocalName[written]; ok {
		return fn, nil
	}
	if pkg, ok := extensionLocalNames[written]; ok {
		return nil, fmt.Errorf(
			"%w: %s is declared by %s, an OpenSysML extension no OMG library declares: write `import %s::*;` to call it",
			ErrUnimportedExtensionFunction, written, pkg, pkg,
		)
	}
	return nil, nil
}

// libraryFunctionFor returns the built-in implementation of sym when sym is a
// function library declaration this runtime implements.
func (ctx *Context) libraryFunctionFor(sym *symbols.Symbol) (*libraryFunction, bool) {
	// A declaration that is not a function is not one of these, whatever it is
	// named.
	if sym == nil || !isCalcSymbol(sym) {
		return nil, false
	}
	fn, ok := libraryFunctions[ctx.qualifiedSymbolName(sym)]
	if !ok {
		return nil, false
	}
	// A library declaration is answered by its built-in even where it carries a
	// body: the built-in is the implementation of that normative function, and the
	// body it is declared with is a specification of it rather than one.
	if ctx.libraryDeclared(sym) {
		return fn, true
	}
	// Outside the libraries the name is the model's own: a calc that carries a
	// body is evaluated from that body and never routed here.
	if ctx.hasCalcBody(sym) {
		return nil, false
	}
	return fn, true
}

// libraryDeclared reports whether sym was declared by one of the library
// documents rather than by the model under evaluation.
func (ctx *Context) libraryDeclared(sym *symbols.Symbol) bool {
	if ctx == nil || ctx.resolver == nil {
		return false
	}
	idx := ctx.resolver.Index()
	return idx != nil && idx.Library(sym)
}

// written is the function's name as a model writes it, which it reports itself by.
func (fn *libraryFunction) written() string { return writtenName(fn.name) }

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
// function. A parameter the signature declares [1] must be given an argument; a
// parameter it declares [0..1] binds null where the call omits it, which is how
// the library's own one-argument `'+'` and `'-'` are called.
func (fn *libraryFunction) bindAndApply(ctx *Context, args calcArgs) (Value, error) {
	if len(args.positional) > 0 && len(args.named) > 0 {
		return Value{}, fmt.Errorf("%w: function %s takes either positional or named arguments", ErrCalcArity, fn.written())
	}
	given := len(args.positional) + len(args.named)
	if given < fn.required || given > len(fn.params) {
		return Value{}, fmt.Errorf(
			"%w: function %s takes %s argument(s), got %d",
			ErrCalcArity, fn.written(), fn.arity(), given,
		)
	}

	if err := fn.checkNamedArguments(args); err != nil {
		return Value{}, err
	}

	values := make([]Value, len(fn.params))
	for i, param := range fn.params {
		arg, err := fn.argumentFor(i, param, args)
		if err != nil {
			return Value{}, err
		}
		values[i] = arg
		if ctx.trace != nil {
			ctx.trace.RecordCalcBind(param, arg, "argument")
		}
	}
	return fn.apply(fn.written(), ctx, values)
}

// checkNamedArguments rejects an argument named for a parameter the signature
// does not declare, which an omitted optional parameter would otherwise absorb.
func (fn *libraryFunction) checkNamedArguments(args calcArgs) error {
	unknown := make([]string, 0, len(args.named))
	for name := range args.named {
		if !slices.Contains(fn.params, name) {
			unknown = append(unknown, name)
		}
	}
	if len(unknown) == 0 {
		return nil
	}
	slices.Sort(unknown)
	return fmt.Errorf(
		"%w: function %s has no input parameter %q (expected %s)",
		ErrUnknownParameter, fn.written(), unknown[0], fn.parameterList(),
	)
}

// parameterList renders the declared parameter names for an error message.
func (fn *libraryFunction) parameterList() string {
	quoted := make([]string, len(fn.params))
	for i, param := range fn.params {
		quoted[i] = fmt.Sprintf("%q", param)
	}
	return strings.Join(quoted, ", ")
}

// argumentFor returns the argument bound to the i-th declared parameter: the
// positional argument in that place, the named argument the library's own
// parameter name matches, or null for an omitted [0..1] parameter.
func (fn *libraryFunction) argumentFor(i int, param string, args calcArgs) (Value, error) {
	if len(args.named) > 0 {
		if bound, ok := args.named[param]; ok {
			return bound, nil
		}
		if i < fn.required {
			return Value{}, fmt.Errorf(
				"%w: function %s has no input parameter matching the arguments given (expected %q)",
				ErrUnknownParameter, fn.written(), param,
			)
		}
		return nullValue(), nil
	}
	if i < len(args.positional) {
		return args.positional[i], nil
	}
	return nullValue(), nil
}

// arity reports the argument count the signature accepts, as one number or as
// the range an optional parameter opens.
func (fn *libraryFunction) arity() string {
	if fn.required == len(fn.params) {
		return fmt.Sprintf("%d", fn.required)
	}
	return fmt.Sprintf("%d..%d", fn.required, len(fn.params))
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

// naturalLog is OpenSysMLMathFunctions::ln. The logarithm is defined for a
// positive argument only: zero and a negative have no Real logarithm, so both
// are reported rather than returned as an infinity or a NaN.
func naturalLog(args []semantics.Value) (semantics.Value, error) {
	x := asReal(args[0])
	if x <= 0 {
		return semantics.Value{}, fmt.Errorf("%w: the logarithm of %v is not a Real (requires x > 0.0)", semantics.ErrArithmeticDomain, x)
	}
	return realResult(math.Log(x))
}

// logToBase is OpenSysMLMathFunctions::log, the logarithm of x to the given
// base, computed as ln(x)/ln(base). Base 1.0 has no logarithm — every power of
// it is 1.0 — and neither the argument nor the base may be zero or negative.
func logToBase(args []semantics.Value) (semantics.Value, error) {
	x, base := asReal(args[0]), asReal(args[1])
	switch {
	case x <= 0:
		return semantics.Value{}, fmt.Errorf("%w: the logarithm of %v is not a Real (requires x > 0.0)", semantics.ErrArithmeticDomain, x)
	case base <= 0:
		return semantics.Value{}, fmt.Errorf("%w: base %v has no logarithm (requires base > 0.0)", semantics.ErrArithmeticDomain, base)
	case base == 1:
		return semantics.Value{}, fmt.Errorf("%w: base 1.0 has no logarithm", semantics.ErrArithmeticDomain)
	}
	// Base 10 and base 2 have their own library functions, which are exact where
	// the ratio of logarithms is not: log10(1000.0) is 3.0, ln(1000.0)/ln(10.0)
	// is 2.9999999999999996.
	switch base {
	case 10:
		return realResult(math.Log10(x))
	case 2:
		return realResult(math.Log2(x))
	}
	return realResult(math.Log(x) / math.Log(base))
}

// atan2Real is OpenSysMLMathFunctions::atan2, the angle to the point (x, y)
// with the parameters ordered y then x as math.Atan2 orders them. The origin has
// no angle, which math.Atan2 answers 0 for, so it is reported instead.
func atan2Real(args []semantics.Value) (semantics.Value, error) {
	y, x := asReal(args[0]), asReal(args[1])
	if y == 0 && x == 0 {
		return semantics.Value{}, fmt.Errorf("%w: atan2(0.0, 0.0) has no angle", semantics.ErrArithmeticDomain)
	}
	return realResult(math.Atan2(y, x))
}

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

// libraryFeature is the value this runtime supplies for one library feature: a
// named constant a library declares and gives no evaluable value of its own,
// which function dispatch cannot answer because a feature is not a call.
type libraryFeature struct {
	name  string
	value func(ctx *Context) (Value, error)
}

// libraryFeatures maps a feature's fully-qualified name to its value. The value
// is recomputed per read, so no two readers share a sequence and no value
// depends on whether the library index cache was warm.
var libraryFeatures = map[string]*libraryFeature{}

func init() {
	registerLibraryFeature("TrigFunctions::pi", func(*Context) (Value, error) {
		return checkedReal(libraryPi)
	})

	// ComplexFunctions::i = rect(0.0, 1.0), the imaginary unit.
	registerLibraryFeature("ComplexFunctions::i", func(*Context) (Value, error) {
		return NewComplex(complex(0, 1)), nil
	})

	// cartesian3DZeroVector = cartesianZeroVector#(3). cartesianZeroVector itself
	// is the 1-, 2- and 3-dimensional zero vectors as one feature of three
	// vectors, which has no representation here.
	registerLibraryFeature("VectorFunctions::cartesian3DZeroVector", func(ctx *Context) (Value, error) {
		return ctx.realVector([]float64{0, 0, 0})
	})
	registerLibraryFeature("VectorFunctions::cartesianZeroVector", func(*Context) (Value, error) {
		return Value{}, fmt.Errorf(
			"%w: VectorFunctions::cartesianZeroVector: %s",
			ErrUnevaluableLibraryFunction, noVectorCollection,
		)
	})
}

// registerLibraryFeature adds one feature value to the seam.
func registerLibraryFeature(name string, value func(ctx *Context) (Value, error)) {
	libraryFeatures[name] = &libraryFeature{name: name, value: value}
}

// libraryFeatureValue returns the value the seam supplies for sym. It is keyed by
// the resolved symbol and answers for a library declaration only, so ordinary
// name resolution decides and a model's own feature keeps its own value.
func (ctx *Context) libraryFeatureValue(sym *symbols.Symbol) (Value, bool, error) {
	if sym == nil || !ctx.libraryDeclared(sym) {
		return Value{}, false, nil
	}
	feature, ok := libraryFeatures[ctx.qualifiedSymbolName(sym)]
	if !ok {
		return Value{}, false, nil
	}
	val, err := feature.value(ctx)
	if err != nil {
		return Value{}, true, err
	}
	return val, true, nil
}

// libraryFeatureByName returns the seam's entry for a fully-qualified feature
// name, which is how a name that resolves to no declaration — a model that
// imports no part of the libraries — reads a library constant.
func libraryFeatureByName(fqn string) (*libraryFeature, bool) {
	feature, ok := libraryFeatures[fqn]
	return feature, ok
}

// degreesFromRadians is TrigFunctions::deg, whose library body is
// `theta_rad * 180 / pi` over the pi the feature seam supplies.
func degreesFromRadians(args []semantics.Value) (semantics.Value, error) {
	return realResult(asReal(args[0]) * 180 / libraryPi)
}

// radiansFromDegrees is TrigFunctions::rad, `theta_deg * pi / 180`.
func radiansFromDegrees(args []semantics.Value) (semantics.Value, error) {
	return realResult(asReal(args[0]) * libraryPi / 180)
}

// ---------------------------------------------------------------------------
// VectorFunctions.
// ---------------------------------------------------------------------------

// vectorElements views a vector argument as the sequence of its elements, which
// is what VectorValues declares a NumericalVectorValue to be: a sequence is that
// vector, one number the one-dimensional vector of it, and null the empty vector.
func vectorElements(name, param string, val Value) ([]semantics.Value, error) {
	elements := elementsOf(val)
	out := make([]semantics.Value, len(elements))
	for i, elem := range elements {
		if elem.Kind != ValConst || !elem.Const.IsNumeric() {
			return nil, fmt.Errorf(
				"%w: function %s parameter %q requires a vector of numeric values, element %d is %s",
				ErrTypeMismatch, name, param, i+1, describeValue(elem),
			)
		}
		out[i] = elem.Const
	}
	return out, nil
}

// vectorElementsFeature is the KerML feature holding a NumericalVectorValue's
// components, which the runtime represents as the collection of them.
const vectorElementsFeature = "elements"

// isNumericVector reports whether a value is a numerical vector: a non-empty
// collection of numeric elements.
func isNumericVector(val Value) bool {
	elements := elementsOf(val)
	if len(elements) == 0 {
		return false
	}
	for _, elem := range elements {
		if elem.Kind != ValConst || !elem.Const.IsNumeric() {
			return false
		}
	}
	return true
}

// realElements is vectorElements widened to Real, for the CartesianVectorValue
// operations, whose elements the library declares Real.
func realElements(name, param string, val Value) ([]float64, error) {
	elements, err := vectorElements(name, param, val)
	if err != nil {
		return nil, err
	}
	out := make([]float64, len(elements))
	for i, elem := range elements {
		out[i] = asReal(elem)
	}
	return out, nil
}

// vectorValue builds a vector from its elements, charging them against the run's
// element budget as any other materialized collection. An element that is not a
// finite value of its kind is reported rather than carried into the vector.
func (ctx *Context) vectorValue(elements []semantics.Value) (Value, error) {
	if err := ctx.chargeElements(int64(len(elements))); err != nil {
		return Value{}, err
	}
	out := make([]Value, len(elements))
	for i, elem := range elements {
		checked, err := checkedNumeric(elem)
		if err != nil {
			return Value{}, err
		}
		out[i] = Value{Kind: ValConst, Const: checked}
	}
	return sequenceOf(out), nil
}

// checkedNumeric screens a computed numeric value, so an arithmetic result that
// overflowed the Real range is reported rather than returned as an infinity.
func checkedNumeric(v semantics.Value) (semantics.Value, error) {
	if v.Kind != semantics.ValReal {
		return v, nil
	}
	return realResult(v.Real)
}

// isNumeric reports whether a value is an Integer or a Real.
func isNumeric(v semantics.Value) bool {
	return v.Kind == semantics.ValInt || v.Kind == semantics.ValReal
}

// elementArith applies an arithmetic operator to two numeric elements, reporting
// a result outside the range of its kind rather than wrapping or infinite.
func elementArith(name string, op ast.OperatorKind, a, b semantics.Value) (semantics.Value, error) {
	if a.Kind == semantics.ValInt && b.Kind == semantics.ValInt {
		result, ok := semantics.IntArith(op, a.Int, b.Int)
		if !ok {
			return semantics.Value{}, fmt.Errorf(
				"%w: function %s has a result outside the Integer range",
				semantics.ErrArithmeticOverflow, name,
			)
		}
		return semantics.Value{Kind: semantics.ValInt, Int: result}, nil
	}
	if a.IsNumeric() && b.IsNumeric() {
		res, ok := semantics.RealArith(op, toReal(a), toReal(b))
		if !ok {
			return semantics.Value{}, fmt.Errorf(
				"%w: function %s cannot combine its arguments", ErrTypeMismatch, name,
			)
		}
		return realResult(res)
	}
	res, ok := semantics.EvalBinary(op, a, b)
	if !ok {
		// A sum, difference or product of two numbers is declined only for leaving
		// the Real range; anything else is an operator the arguments do not define.
		switch op {
		case ast.OpAdd, ast.OpSub, ast.OpMul:
			if isNumeric(a) && isNumeric(b) {
				return semantics.Value{}, fmt.Errorf(
					"%w: function %s has a result outside the Real range",
					semantics.ErrArithmeticOverflow, name,
				)
			}
		}
		return semantics.Value{}, fmt.Errorf(
			"%w: function %s cannot combine its arguments", ErrTypeMismatch, name,
		)
	}
	return checkedNumeric(res)
}

// realVector builds a vector of Reals, reporting an element that is not a finite
// Real rather than carrying an infinity into it.
func (ctx *Context) realVector(components []float64) (Value, error) {
	elements := make([]semantics.Value, len(components))
	for i, x := range components {
		elem, err := realResult(x)
		if err != nil {
			return Value{}, err
		}
		elements[i] = elem
	}
	return ctx.vectorValue(elements)
}

// checkedReal wraps a computed Real as a runtime value, reporting a result that
// is not a finite number.
func checkedReal(x float64) (Value, error) {
	res, err := realResult(x)
	if err != nil {
		return Value{}, err
	}
	return Value{Kind: ValConst, Const: res}, nil
}

// argumentOmitted reports an argument as not given for a [0..1] parameter: null,
// and equally an empty collection, which KerML holds to be the same no value.
func argumentOmitted(val Value) bool {
	return len(elementsOf(val)) == 0
}

// scalarArg reads a scalar numeric argument: the NumericalValue a scalar-vector
// product or a Complex component is declared as.
func scalarArg(name, param string, val Value) (semantics.Value, error) {
	if val.Kind != ValConst || !val.Const.IsNumeric() {
		return semantics.Value{}, fmt.Errorf(
			"%w: function %s parameter %q requires a numeric value",
			ErrTypeMismatch, name, param,
		)
	}
	return val.Const, nil
}

// combineElements applies an arithmetic operator elementwise to two vectors of
// equal dimension, keeping the elements' kind as the library's declaration over
// NumericalValue does: two Integer vectors give an Integer vector.
func combineElements(name string, op ast.OperatorKind, v, w []semantics.Value) ([]semantics.Value, error) {
	if len(v) != len(w) {
		return nil, fmt.Errorf(
			"%w: function %s requires vectors of equal dimension, got %d and %d",
			ErrTypeMismatch, name, len(v), len(w),
		)
	}
	out := make([]semantics.Value, len(v))
	for i := range v {
		res, err := elementArith(name, op, v[i], w[i])
		if err != nil {
			return nil, err
		}
		out[i] = res
	}
	return out, nil
}

// zeroLike is the zero of a numeric value's kind, which negation subtracts from.
func zeroLike(v semantics.Value) semantics.Value {
	if v.Kind == semantics.ValInt {
		return semantics.Value{Kind: semantics.ValInt}
	}
	return semantics.Value{Kind: semantics.ValReal}
}

// vectorIsZero is isZeroVector and its Cartesian specialization: a zero vector
// is one whose every element is zero.
func vectorIsZero(name string, _ *Context, args []Value) (Value, error) {
	elements, err := vectorElements(name, "v", args[0])
	if err != nil {
		return Value{}, err
	}
	for _, elem := range elements {
		if asReal(elem) != 0 {
			return boolValue(false), nil
		}
	}
	return boolValue(true), nil
}

// vectorAdd is VectorFunctions::'+' and 'cartesian+': the sum of two vectors of
// equal dimension, or, given one argument, that vector.
func vectorAdd(name string, ctx *Context, args []Value) (Value, error) {
	v, err := vectorElements(name, "v", args[0])
	if err != nil {
		return Value{}, err
	}
	if argumentOmitted(args[1]) {
		return ctx.vectorValue(v)
	}
	w, err := vectorElements(name, "w", args[1])
	if err != nil {
		return Value{}, err
	}
	sum, err := combineElements(name, ast.OpAdd, v, w)
	if err != nil {
		return Value{}, err
	}
	return ctx.vectorValue(sum)
}

// vectorSubtract is VectorFunctions::'-' and 'cartesian-': the difference of two
// vectors of equal dimension, or, given one argument, the vector that added to it
// gives the zero vector.
func vectorSubtract(name string, ctx *Context, args []Value) (Value, error) {
	v, err := vectorElements(name, "v", args[0])
	if err != nil {
		return Value{}, err
	}
	if argumentOmitted(args[1]) {
		zeros := make([]semantics.Value, len(v))
		for i, elem := range v {
			zeros[i] = zeroLike(elem)
		}
		negated, err := combineElements(name, ast.OpSub, zeros, v)
		if err != nil {
			return Value{}, err
		}
		return ctx.vectorValue(negated)
	}
	w, err := vectorElements(name, "w", args[1])
	if err != nil {
		return Value{}, err
	}
	difference, err := combineElements(name, ast.OpSub, v, w)
	if err != nil {
		return Value{}, err
	}
	return ctx.vectorValue(difference)
}

// vectorOf is VectorFunctions::VectorOf, the NumericalVectorValue of a non-empty
// list of components, whose kind it keeps.
func vectorOf(name string, ctx *Context, args []Value) (Value, error) {
	components, err := vectorElements(name, "components", args[0])
	if err != nil {
		return Value{}, err
	}
	if len(components) == 0 {
		return Value{}, fmt.Errorf(
			"%w: function %s requires at least one component (components: NumericalValue[1..*])",
			ErrMultiplicityViolation, name,
		)
	}
	return ctx.vectorValue(components)
}

// cartesianVectorOf is VectorFunctions::CartesianVectorOf, whose components the
// library declares Real, so an Integer component widens.
func cartesianVectorOf(name string, ctx *Context, args []Value) (Value, error) {
	components, err := realElements(name, "components", args[0])
	if err != nil {
		return Value{}, err
	}
	return ctx.realVector(components)
}

// cartesianThreeVectorOf is VectorFunctions::CartesianThreeVectorOf, which
// declares its components Real[3].
func cartesianThreeVectorOf(name string, ctx *Context, args []Value) (Value, error) {
	components, err := realElements(name, "components", args[0])
	if err != nil {
		return Value{}, err
	}
	if len(components) != 3 {
		return Value{}, fmt.Errorf(
			"%w: function %s requires 3 components (components: Real[3]), got %d",
			ErrMultiplicityViolation, name, len(components),
		)
	}
	return ctx.realVector(components)
}

// scalarVectorMult is the scalar product with the scalar first, which the library
// also aliases as VectorFunctions::'*'.
func scalarVectorMult(name string, ctx *Context, args []Value) (Value, error) {
	return scaleVector(name, ctx, args[0], "x", args[1], "v")
}

// vectorScalarMult is the same product with the vector first, which the library
// defines as scalarVectorMult(x, v).
func vectorScalarMult(name string, ctx *Context, args []Value) (Value, error) {
	return scaleVector(name, ctx, args[1], "x", args[0], "v")
}

// scaleVector multiplies every element of a vector by a scalar, keeping the
// elements' kind.
func scaleVector(name string, ctx *Context, scalar Value, scalarParam string, vector Value, vectorParam string) (Value, error) {
	x, err := scalarArg(name, scalarParam, scalar)
	if err != nil {
		return Value{}, err
	}
	elements, err := vectorElements(name, vectorParam, vector)
	if err != nil {
		return Value{}, err
	}
	scaled := make([]semantics.Value, len(elements))
	for i, elem := range elements {
		res, err := elementArith(name, ast.OpMul, x, elem)
		if err != nil {
			return Value{}, err
		}
		scaled[i] = res
	}
	return ctx.vectorValue(scaled)
}

// vectorScalarDiv is VectorFunctions::vectorScalarDiv. The library defines it as
// scalarVectorMult(1.0 / x, v); dividing each element is the same quotient
// without the reciprocal's rounding.
func vectorScalarDiv(name string, ctx *Context, args []Value) (Value, error) {
	elements, err := vectorElements(name, "v", args[0])
	if err != nil {
		return Value{}, err
	}
	x, err := scalarArg(name, "x", args[1])
	if err != nil {
		return Value{}, err
	}
	if asReal(x) == 0 {
		return Value{}, fmt.Errorf("%w: function %s divides by zero", ErrDivisionByZero, name)
	}
	quotients := make([]float64, len(elements))
	for i, elem := range elements {
		quotients[i] = asReal(elem) / asReal(x)
	}
	return ctx.realVector(quotients)
}

// vectorInner is the inner product of two vectors of equal dimension, keeping the
// elements' kind: two Integer vectors have an Integer inner product.
func vectorInner(name string, _ *Context, args []Value) (Value, error) {
	v, err := vectorElements(name, "v", args[0])
	if err != nil {
		return Value{}, err
	}
	w, err := vectorElements(name, "w", args[1])
	if err != nil {
		return Value{}, err
	}
	products, err := combineElements(name, ast.OpMul, v, w)
	if err != nil {
		return Value{}, err
	}
	sum := semantics.Value{Kind: semantics.ValInt}
	for _, product := range products {
		next, err := elementArith(name, ast.OpAdd, sum, product)
		if err != nil {
			return Value{}, err
		}
		sum = next
	}
	checked, err := checkedNumeric(sum)
	if err != nil {
		return Value{}, err
	}
	return Value{Kind: ValConst, Const: checked}, nil
}

// vectorNorm is the norm (magnitude) of a vector, the square root of its inner
// product with itself.
func vectorNorm(name string, _ *Context, args []Value) (Value, error) {
	elements, err := realElements(name, "v", args[0])
	if err != nil {
		return Value{}, err
	}
	return checkedReal(euclideanNorm(elements))
}

// euclideanNorm is the square root of the sum of the squares.
func euclideanNorm(elements []float64) float64 {
	sum := 0.0
	for _, x := range elements {
		sum += x * x
	}
	return math.Sqrt(sum)
}

// vectorAngle is the angle between two vectors of equal dimension,
// arccos(inner(v, w) / (norm(v) * norm(w))). A zero vector points nowhere, so
// there is no angle to it.
func vectorAngle(name string, _ *Context, args []Value) (Value, error) {
	v, err := realElements(name, "v", args[0])
	if err != nil {
		return Value{}, err
	}
	w, err := realElements(name, "w", args[1])
	if err != nil {
		return Value{}, err
	}
	if len(v) != len(w) {
		return Value{}, fmt.Errorf(
			"%w: function %s requires vectors of equal dimension, got %d and %d",
			ErrTypeMismatch, name, len(v), len(w),
		)
	}
	normV, normW := euclideanNorm(v), euclideanNorm(w)
	if normV == 0 || normW == 0 {
		return Value{}, fmt.Errorf(
			"%w: function %s has no angle to a zero vector",
			semantics.ErrArithmeticDomain, name,
		)
	}
	inner := 0.0
	for i := range v {
		inner += v[i] * w[i]
	}
	// Rounding can carry the cosine of two parallel vectors just outside
	// [-1.0, 1.0], where the arc cosine has no value; the angle there is 0 or pi.
	cosine := math.Max(-1, math.Min(1, inner/(normV*normW)))
	return checkedReal(math.Acos(cosine))
}

// ---------------------------------------------------------------------------
// ComplexFunctions.
// ---------------------------------------------------------------------------

// asComplex reads a Complex argument: a Complex value, or a Real, which
// ScalarValues declares a Complex (Real :> Complex) with a zero imaginary part.
func asComplex(name, param string, val Value) (complex128, error) {
	z, ok := complexOf(val)
	if !ok {
		return 0, fmt.Errorf(
			"%w: function %s parameter %q requires a Complex value, got %s",
			ErrTypeMismatch, name, param, describeValue(val),
		)
	}
	return z, nil
}

// complexRect is ComplexFunctions::rect, the Complex with the given real and
// imaginary parts.
func complexRect(name string, _ *Context, args []Value) (Value, error) {
	re, err := scalarArg(name, "re", args[0])
	if err != nil {
		return Value{}, err
	}
	im, err := scalarArg(name, "im", args[1])
	if err != nil {
		return Value{}, err
	}
	return complexResult(complex(asReal(re), asReal(im)))
}

// complexPolar is ComplexFunctions::polar, the Complex with the given modulus
// and argument.
func complexPolar(name string, _ *Context, args []Value) (Value, error) {
	abs, err := scalarArg(name, "abs", args[0])
	if err != nil {
		return Value{}, err
	}
	arg, err := scalarArg(name, "arg", args[1])
	if err != nil {
		return Value{}, err
	}
	return complexResult(cmplx.Rect(asReal(abs), asReal(arg)))
}

// complexRealPart is ComplexFunctions::re.
func complexRealPart(name string, _ *Context, args []Value) (Value, error) {
	z, err := asComplex(name, "x", args[0])
	if err != nil {
		return Value{}, err
	}
	return checkedReal(real(z))
}

// complexImagPart is ComplexFunctions::im.
func complexImagPart(name string, _ *Context, args []Value) (Value, error) {
	z, err := asComplex(name, "x", args[0])
	if err != nil {
		return Value{}, err
	}
	return checkedReal(imag(z))
}

// complexIsZero is ComplexFunctions::isZero, `re(x) == 0.0 and im(x) == 0.0`.
func complexIsZero(name string, _ *Context, args []Value) (Value, error) {
	z, err := asComplex(name, "x", args[0])
	if err != nil {
		return Value{}, err
	}
	return boolValue(z == 0), nil
}

// complexIsUnit is ComplexFunctions::isUnit, `re(x) == 1.0 and im(x) == 0.0`.
func complexIsUnit(name string, _ *Context, args []Value) (Value, error) {
	z, err := asComplex(name, "x", args[0])
	if err != nil {
		return Value{}, err
	}
	return boolValue(z == 1), nil
}

// complexModulus is ComplexFunctions::abs, the distance from the origin.
func complexModulus(name string, _ *Context, args []Value) (Value, error) {
	z, err := asComplex(name, "x", args[0])
	if err != nil {
		return Value{}, err
	}
	return checkedReal(cmplx.Abs(z))
}

// complexArgument is ComplexFunctions::arg, the angle to the point. The origin
// has no angle, which is reported rather than answered 0.
func complexArgument(name string, _ *Context, args []Value) (Value, error) {
	z, err := asComplex(name, "x", args[0])
	if err != nil {
		return Value{}, err
	}
	if z == 0 {
		return Value{}, fmt.Errorf(
			"%w: function %s has no argument for %s",
			semantics.ErrArithmeticDomain, name, FormatComplex(z),
		)
	}
	return checkedReal(cmplx.Phase(z))
}

// complexAdd is ComplexFunctions::'+', whose second operand the library declares
// [0..1]: given one argument it is that value.
func complexAdd(name string, _ *Context, args []Value) (Value, error) {
	x, y, given, err := complexOptionalOperands(name, args)
	if err != nil {
		return Value{}, err
	}
	if !given {
		return complexResult(x)
	}
	return complexResult(x + y)
}

// complexSubtract is ComplexFunctions::'-': given one argument, the value that
// added to it gives zero.
func complexSubtract(name string, _ *Context, args []Value) (Value, error) {
	x, y, given, err := complexOptionalOperands(name, args)
	if err != nil {
		return Value{}, err
	}
	if !given {
		return complexResult(-x)
	}
	return complexResult(x - y)
}

// complexMultiply is ComplexFunctions::'*'.
func complexMultiply(name string, _ *Context, args []Value) (Value, error) {
	x, y, err := complexBinaryOperands(name, args)
	if err != nil {
		return Value{}, err
	}
	return complexResult(x * y)
}

// complexDivide is ComplexFunctions::'/'.
func complexDivide(name string, _ *Context, args []Value) (Value, error) {
	x, y, err := complexBinaryOperands(name, args)
	if err != nil {
		return Value{}, err
	}
	if y == 0 {
		return Value{}, fmt.Errorf("%w: function %s divides by zero", ErrDivisionByZero, name)
	}
	return complexResult(x / y)
}

// complexPower is ComplexFunctions::'**' and its '^' synonym.
func complexPower(name string, _ *Context, args []Value) (Value, error) {
	x, y, err := complexBinaryOperands(name, args)
	if err != nil {
		return Value{}, err
	}
	res, err := complexPow(x, y)
	if err != nil {
		return Value{}, fmt.Errorf("function %s: %w", name, err)
	}
	return res, nil
}

// complexEquals is ComplexFunctions::'==', which declares both operands [0..1]:
// two empty operands are equal, an empty one and a value are not.
func complexEquals(name string, _ *Context, args []Value) (Value, error) {
	xGiven, yGiven := !argumentOmitted(args[0]), !argumentOmitted(args[1])
	if !xGiven || !yGiven {
		return boolValue(xGiven == yGiven), nil
	}
	x, y, err := complexBinaryOperands(name, args)
	if err != nil {
		return Value{}, err
	}
	return boolValue(x == y), nil
}

// complexSum is ComplexFunctions::sum: the sum of the collection's elements, and
// `rect(0.0, 0.0)` for an empty collection, as the library's `sum0` computes.
func complexSum(name string, _ *Context, args []Value) (Value, error) {
	return aggregateComplex(name, args[0], ast.OpAdd)
}

// complexProduct is ComplexFunctions::product, with `rect(1.0, 0.0)` for an
// empty collection, as the library's `product1` computes.
func complexProduct(name string, _ *Context, args []Value) (Value, error) {
	return aggregateComplex(name, args[0], ast.OpMul)
}

// aggregateComplex folds a collection's elements as complex numbers under the
// sum or product, from its identity; an element that is no number is reported.
func aggregateComplex(name string, collection Value, operator ast.OperatorKind) (Value, error) {
	acc := complex(0, 0)
	if operator == ast.OpMul {
		acc = 1
	}
	for _, elem := range elementsOf(collection) {
		z, err := asComplex(name, "collection", elem)
		if err != nil {
			return Value{}, err
		}
		if operator == ast.OpMul {
			acc *= z
		} else {
			acc += z
		}
	}
	return complexResult(acc)
}

// complexBinaryOperands reads the two operands of a Complex function that
// declares both as one value each.
func complexBinaryOperands(name string, args []Value) (x, y complex128, err error) {
	if x, err = asComplex(name, "x", args[0]); err != nil {
		return 0, 0, err
	}
	if y, err = asComplex(name, "y", args[1]); err != nil {
		return 0, 0, err
	}
	return x, y, nil
}

// complexOptionalOperands reads the operands of '+' and '-', whose second operand
// the library declares [0..1], reporting whether it was given.
func complexOptionalOperands(name string, args []Value) (x, y complex128, given bool, err error) {
	x, err = asComplex(name, "x", args[0])
	if err != nil {
		return 0, 0, false, err
	}
	if argumentOmitted(args[1]) {
		return x, 0, false, nil
	}
	y, err = asComplex(name, "y", args[1])
	if err != nil {
		return 0, 0, false, err
	}
	return x, y, true, nil
}

// concatStrings is StringFunctions::'+': the characters of x then those of y.
func concatStrings(x, y string) Value {
	return NewStringValue(x + y)
}

// compareStrings applies one of StringFunctions' comparisons. UTF-8 orders bytes
// as it orders code points, so Go's own comparison is character order.
func compareStrings(op ast.OperatorKind, x, y string) (bool, error) {
	switch op {
	case ast.OpLt:
		return x < y, nil
	case ast.OpLe:
		return x <= y, nil
	case ast.OpGt:
		return x > y, nil
	case ast.OpGe:
		return x >= y, nil
	}
	return false, fmt.Errorf("%w: '%s' does not order two strings", ErrUnsupportedOperator, op)
}

// stringArg reads a String argument, reporting another kind rather than
// rendering it as a string.
func stringArg(name, param string, val Value) (string, error) {
	if val.Kind != ValString {
		return "", fmt.Errorf(
			"%w: function %s parameter %q requires a string value, got %s",
			ErrTypeMismatch, name, param, describeOperand(val),
		)
	}
	return val.Str(), nil
}

// stringPositionArg reads an Integer position argument of Substring.
func stringPositionArg(name, param string, val Value) (int64, error) {
	if val.Kind != ValConst || val.Const.Kind != semantics.ValInt {
		return 0, fmt.Errorf(
			"%w: function %s parameter %q requires an Integer value, got %s",
			ErrTypeMismatch, name, param, describeOperand(val),
		)
	}
	return val.Const.Int, nil
}

// stringConcat is StringFunctions::'+', which declares both operands String[1].
func stringConcat(name string, _ *Context, args []Value) (Value, error) {
	x, y, err := stringPair(name, args)
	if err != nil {
		return Value{}, err
	}
	return concatStrings(x, y), nil
}

// stringLength is StringFunctions::Length: characters of x, one per Unicode code
// point, so a multi-byte character counts once.
func stringLength(name string, _ *Context, args []Value) (Value, error) {
	x, err := stringArg(name, "x", args[0])
	if err != nil {
		return Value{}, err
	}
	count := int64(utf8.RuneCountInString(x))
	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: count}}, nil
}

// stringSubstring is StringFunctions::Substring: characters lower to upper
// inclusive, 1-based, bounded as SequenceFunctions::subsequence is.
func stringSubstring(name string, _ *Context, args []Value) (Value, error) {
	x, err := stringArg(name, "x", args[0])
	if err != nil {
		return Value{}, err
	}
	lower, err := stringPositionArg(name, "lower", args[1])
	if err != nil {
		return Value{}, err
	}
	upper, err := stringPositionArg(name, "upper", args[2])
	if err != nil {
		return Value{}, err
	}
	chars := []rune(x)
	if lower < 1 {
		return Value{}, fmt.Errorf("%w: function %s lower character %d is outside 1..%d",
			ErrIndexOutOfRange, name, lower, len(chars))
	}
	if lower > upper {
		return Value{Kind: ValString}, nil
	}
	if upper > int64(len(chars)) {
		return Value{}, fmt.Errorf("%w: function %s upper character %d is outside 1..%d",
			ErrIndexOutOfRange, name, upper, len(chars))
	}
	return NewStringValue(string(chars[lower-1 : upper])), nil
}

// stringOrdering is one of StringFunctions' comparisons over two String[1].
func stringOrdering(op ast.OperatorKind) libraryApply {
	return func(name string, _ *Context, args []Value) (Value, error) {
		x, y, err := stringPair(name, args)
		if err != nil {
			return Value{}, err
		}
		ordered, err := compareStrings(op, x, y)
		if err != nil {
			return Value{}, err
		}
		return boolValue(ordered), nil
	}
}

// stringEquals is StringFunctions::'==', whose operands are String[0..1]: two
// empty operands are equal, an empty one and a string are not.
func stringEquals(name string, _ *Context, args []Value) (Value, error) {
	xGiven, yGiven := !argumentOmitted(args[0]), !argumentOmitted(args[1])
	if !xGiven || !yGiven {
		return boolValue(xGiven == yGiven), nil
	}
	x, y, err := stringPair(name, args)
	if err != nil {
		return Value{}, err
	}
	return boolValue(x == y), nil
}

// stringToString is StringFunctions::ToString, whose vendored body is `x`.
func stringToString(name string, _ *Context, args []Value) (Value, error) {
	x, err := stringArg(name, "x", args[0])
	if err != nil {
		return Value{}, err
	}
	return NewStringValue(x), nil
}

// stringPair reads the two String operands x and y.
func stringPair(name string, args []Value) (x, y string, err error) {
	if x, err = stringArg(name, "x", args[0]); err != nil {
		return "", "", err
	}
	if y, err = stringArg(name, "y", args[1]); err != nil {
		return "", "", err
	}
	return x, y, nil
}
