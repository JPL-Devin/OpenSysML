package runtime

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/libs"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/resolve"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// libCtx returns a runtime context over an empty model, enough to apply a
// library function, which needs no scope.
func libCtx(t *testing.T) *Context {
	t.Helper()
	idx := symbols.NewIndex()
	resolver := resolve.New(idx)
	return NewContext(semantics.NewModel(resolver), resolver, 10000)
}

func constInt(i int64) Value {
	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: i}}
}

func constReal(f float64) Value {
	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: f}}
}

// applyLibrary applies the function of that fully-qualified name to positional
// arguments.
func applyLibrary(t *testing.T, name string, args ...Value) (Value, error) {
	t.Helper()
	fn, ok := libraryFunctionByName(name)
	if !ok {
		t.Fatalf("no library function %s registered", name)
	}
	return fn.invoke(libCtx(t), calcArgs{positional: args})
}

func TestLibraryFunctionValues(t *testing.T) {
	cases := []struct {
		name string
		args []Value
		want semantics.Value
	}{
		{"RealFunctions::sqrt", []Value{constReal(16)}, semantics.Value{Kind: semantics.ValReal, Real: 4}},
		{"RealFunctions::sqrt", []Value{constInt(9)}, semantics.Value{Kind: semantics.ValReal, Real: 3}},
		{"RealFunctions::abs", []Value{constReal(-2.5)}, semantics.Value{Kind: semantics.ValReal, Real: 2.5}},
		{"RealFunctions::floor", []Value{constReal(2.7)}, semantics.Value{Kind: semantics.ValInt, Int: 2}},
		{"RealFunctions::floor", []Value{constReal(-2.1)}, semantics.Value{Kind: semantics.ValInt, Int: -3}},
		{"RealFunctions::round", []Value{constReal(2.5)}, semantics.Value{Kind: semantics.ValInt, Int: 3}},
		{"RealFunctions::round", []Value{constReal(-2.5)}, semantics.Value{Kind: semantics.ValInt, Int: -3}},
		{"RealFunctions::floor", []Value{constReal(math.MinInt64)}, semantics.Value{Kind: semantics.ValInt, Int: math.MinInt64}},
		{"RealFunctions::max", []Value{constReal(2), constInt(7)}, semantics.Value{Kind: semantics.ValReal, Real: 7}},
		{"RealFunctions::min", []Value{constReal(2), constInt(7)}, semantics.Value{Kind: semantics.ValReal, Real: 2}},
		{"RationalFunctions::abs", []Value{constReal(-0.5)}, semantics.Value{Kind: semantics.ValReal, Real: 0.5}},
		{"RationalFunctions::max", []Value{constReal(0.5), constReal(0.25)}, semantics.Value{Kind: semantics.ValReal, Real: 0.5}},
		{"RationalFunctions::min", []Value{constReal(0.5), constReal(0.25)}, semantics.Value{Kind: semantics.ValReal, Real: 0.25}},
		{"NumericalFunctions::abs", []Value{constInt(-3)}, semantics.Value{Kind: semantics.ValInt, Int: 3}},
		{"NumericalFunctions::abs", []Value{constReal(-3.5)}, semantics.Value{Kind: semantics.ValReal, Real: 3.5}},
		{"NumericalFunctions::max", []Value{constInt(2), constInt(7)}, semantics.Value{Kind: semantics.ValInt, Int: 7}},
		{"NumericalFunctions::max", []Value{constInt(2), constReal(7.5)}, semantics.Value{Kind: semantics.ValReal, Real: 7.5}},
		{"NumericalFunctions::min", []Value{constInt(2), constInt(7)}, semantics.Value{Kind: semantics.ValInt, Int: 2}},
		{"NumericalFunctions::isZero", []Value{constInt(0)}, semantics.Value{Kind: semantics.ValBool, Bool: true}},
		{"NumericalFunctions::isZero", []Value{constReal(0.5)}, semantics.Value{Kind: semantics.ValBool, Bool: false}},
		{"NumericalFunctions::isUnit", []Value{constReal(1)}, semantics.Value{Kind: semantics.ValBool, Bool: true}},
		{"NumericalFunctions::isUnit", []Value{constInt(2)}, semantics.Value{Kind: semantics.ValBool, Bool: false}},
		{"IntegerFunctions::abs", []Value{constInt(-3)}, semantics.Value{Kind: semantics.ValInt, Int: 3}},
		{"IntegerFunctions::max", []Value{constInt(2), constInt(7)}, semantics.Value{Kind: semantics.ValInt, Int: 7}},
		{"IntegerFunctions::min", []Value{constInt(2), constInt(7)}, semantics.Value{Kind: semantics.ValInt, Int: 2}},
		{"NaturalFunctions::max", []Value{constInt(2), constInt(7)}, semantics.Value{Kind: semantics.ValInt, Int: 7}},
		{"NaturalFunctions::min", []Value{constInt(2), constInt(7)}, semantics.Value{Kind: semantics.ValInt, Int: 2}},
		{"TrigFunctions::sin", []Value{constReal(0)}, semantics.Value{Kind: semantics.ValReal, Real: 0}},
		{"TrigFunctions::sin", []Value{constReal(math.Pi / 2)}, semantics.Value{Kind: semantics.ValReal, Real: 1}},
		{"TrigFunctions::cos", []Value{constReal(0)}, semantics.Value{Kind: semantics.ValReal, Real: 1}},
		{"TrigFunctions::tan", []Value{constReal(0)}, semantics.Value{Kind: semantics.ValReal, Real: 0}},
		{"TrigFunctions::cot", []Value{constReal(math.Pi / 2)}, semantics.Value{Kind: semantics.ValReal, Real: math.Cos(math.Pi / 2)}},
		{"TrigFunctions::arcsin", []Value{constReal(1)}, semantics.Value{Kind: semantics.ValReal, Real: math.Pi / 2}},
		{"TrigFunctions::arccos", []Value{constReal(1)}, semantics.Value{Kind: semantics.ValReal, Real: 0}},
		{"TrigFunctions::arctan", []Value{constReal(1)}, semantics.Value{Kind: semantics.ValReal, Real: math.Pi / 4}},
		{"SystemicaMathFunctions::exp", []Value{constReal(0)}, semantics.Value{Kind: semantics.ValReal, Real: 1}},
		{"SystemicaMathFunctions::exp", []Value{constReal(1)}, semantics.Value{Kind: semantics.ValReal, Real: math.E}},
		{"SystemicaMathFunctions::exp", []Value{constInt(0)}, semantics.Value{Kind: semantics.ValReal, Real: 1}},
		{"SystemicaMathFunctions::exp", []Value{constReal(-1)}, semantics.Value{Kind: semantics.ValReal, Real: 1 / math.E}},
		{"SystemicaMathFunctions::ln", []Value{constReal(1)}, semantics.Value{Kind: semantics.ValReal, Real: 0}},
		{"SystemicaMathFunctions::ln", []Value{constReal(math.E)}, semantics.Value{Kind: semantics.ValReal, Real: 1}},
		{"SystemicaMathFunctions::ln", []Value{constInt(1)}, semantics.Value{Kind: semantics.ValReal, Real: 0}},
		{"SystemicaMathFunctions::log", []Value{constReal(8), constReal(2)}, semantics.Value{Kind: semantics.ValReal, Real: 3}},
		{"SystemicaMathFunctions::log", []Value{constReal(1000), constReal(10)}, semantics.Value{Kind: semantics.ValReal, Real: 3}},
		{"SystemicaMathFunctions::log", []Value{constInt(8), constInt(2)}, semantics.Value{Kind: semantics.ValReal, Real: 3}},
		// A base below 1 gives a decreasing logarithm: log(0.5, 0.5) is 1.
		{"SystemicaMathFunctions::log", []Value{constReal(0.5), constReal(0.5)}, semantics.Value{Kind: semantics.ValReal, Real: 1}},
		{"SystemicaMathFunctions::atan2", []Value{constReal(1), constReal(1)}, semantics.Value{Kind: semantics.ValReal, Real: math.Pi / 4}},
		// The quadrant arctan(y/x) loses: (1, -1) and (-1, 1) share the ratio -1.
		{"SystemicaMathFunctions::atan2", []Value{constReal(1), constReal(-1)}, semantics.Value{Kind: semantics.ValReal, Real: 3 * math.Pi / 4}},
		{"SystemicaMathFunctions::atan2", []Value{constReal(-1), constReal(1)}, semantics.Value{Kind: semantics.ValReal, Real: -math.Pi / 4}},
		{"SystemicaMathFunctions::atan2", []Value{constReal(-1), constReal(-1)}, semantics.Value{Kind: semantics.ValReal, Real: -3 * math.Pi / 4}},
		{"SystemicaMathFunctions::atan2", []Value{constInt(1), constInt(0)}, semantics.Value{Kind: semantics.ValReal, Real: math.Pi / 2}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := applyLibrary(t, tc.name, tc.args...)
			if err != nil {
				t.Fatalf("%s%v = error %v", tc.name, tc.args, err)
			}
			if got.Kind != ValConst || got.Const != tc.want {
				t.Fatalf("%s%v = %+v, want %+v", tc.name, tc.args, got, tc.want)
			}
		})
	}
}

func TestLibraryFunctionErrors(t *testing.T) {
	cases := []struct {
		name string
		fn   string
		args []Value
		want error
	}{
		{"square root of a negative", "RealFunctions::sqrt", []Value{constReal(-1)}, semantics.ErrArithmeticDomain},
		{"arcsin outside the unit bound", "TrigFunctions::arcsin", []Value{constReal(2)}, semantics.ErrArithmeticDomain},
		{"arccos outside the unit bound", "TrigFunctions::arccos", []Value{constReal(-2)}, semantics.ErrArithmeticDomain},
		// No Real is exactly pi/2, so tan has no infinite argument to report;
		// cot does, at zero.
		{"cotangent of a zero sine", "TrigFunctions::cot", []Value{constReal(0)}, semantics.ErrArithmeticOverflow},
		{"floor beyond the Integer range", "RealFunctions::floor", []Value{constReal(1e300)}, semantics.ErrArithmeticOverflow},
		// 2^63 is the least Real above the Integer range, and the only one the
		// int64 conversion would silently wrap.
		{"floor at the Integer boundary", "RealFunctions::floor", []Value{constReal(-float64(math.MinInt64))}, semantics.ErrArithmeticOverflow},
		{"round at the Integer boundary", "RealFunctions::round", []Value{constReal(-float64(math.MinInt64))}, semantics.ErrArithmeticOverflow},
		{"absolute value of the least Integer", "IntegerFunctions::abs", []Value{constInt(math.MinInt64)}, semantics.ErrArithmeticOverflow},
		{"too few arguments", "RealFunctions::max", []Value{constReal(1)}, ErrCalcArity},
		{"too many arguments", "RealFunctions::sqrt", []Value{constReal(1), constReal(2)}, ErrCalcArity},
		{"no arguments", "RealFunctions::sqrt", nil, ErrCalcArity},
		{"boolean argument", "RealFunctions::sqrt", []Value{boolValue(true)}, ErrTypeMismatch},
		{"string argument", "RealFunctions::sqrt", []Value{{Kind: ValString, Str: "4"}}, ErrTypeMismatch},
		{"Real argument to an Integer parameter", "IntegerFunctions::abs", []Value{constReal(1.5)}, ErrTypeMismatch},
		{"negative argument to a Natural parameter", "NaturalFunctions::max", []Value{constInt(-1), constInt(2)}, ErrTypeMismatch},
		{"logarithm of zero", "SystemicaMathFunctions::ln", []Value{constReal(0)}, semantics.ErrArithmeticDomain},
		{"logarithm of a negative", "SystemicaMathFunctions::ln", []Value{constReal(-1)}, semantics.ErrArithmeticDomain},
		{"logarithm to base 1", "SystemicaMathFunctions::log", []Value{constReal(8), constReal(1)}, semantics.ErrArithmeticDomain},
		{"logarithm of a negative to a base", "SystemicaMathFunctions::log", []Value{constReal(-1), constReal(10)}, semantics.ErrArithmeticDomain},
		{"logarithm of zero to a base", "SystemicaMathFunctions::log", []Value{constReal(0), constReal(10)}, semantics.ErrArithmeticDomain},
		{"logarithm to a negative base", "SystemicaMathFunctions::log", []Value{constReal(8), constReal(-2)}, semantics.ErrArithmeticDomain},
		{"logarithm to base zero", "SystemicaMathFunctions::log", []Value{constReal(8), constReal(0)}, semantics.ErrArithmeticDomain},
		{"angle at the origin", "SystemicaMathFunctions::atan2", []Value{constReal(0), constReal(0)}, semantics.ErrArithmeticDomain},
		{"exponential beyond the Real range", "SystemicaMathFunctions::exp", []Value{constReal(1000)}, semantics.ErrArithmeticOverflow},
		{"exponential with no argument", "SystemicaMathFunctions::exp", nil, ErrCalcArity},
		{"logarithm with one argument", "SystemicaMathFunctions::log", []Value{constReal(8)}, ErrCalcArity},
		{"angle with three arguments", "SystemicaMathFunctions::atan2", []Value{constReal(1), constReal(1), constReal(1)}, ErrCalcArity},
		{"boolean argument to the exponential", "SystemicaMathFunctions::exp", []Value{boolValue(true)}, ErrTypeMismatch},
		{"string argument to the logarithm", "SystemicaMathFunctions::ln", []Value{{Kind: ValString, Str: "1"}}, ErrTypeMismatch},
		{"string base", "SystemicaMathFunctions::log", []Value{constReal(8), {Kind: ValString, Str: "2"}}, ErrTypeMismatch},
		{"boolean argument to the angle", "SystemicaMathFunctions::atan2", []Value{constReal(1), boolValue(false)}, ErrTypeMismatch},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := applyLibrary(t, tc.fn, tc.args...)
			if !errors.Is(err, tc.want) {
				t.Fatalf("%s%v = %+v, %v; want error %v", tc.fn, tc.args, got, err, tc.want)
			}
		})
	}
}

// A named argument binds to the parameter name the vendored signature declares.
func TestLibraryFunctionNamedArguments(t *testing.T) {
	fn, _ := libraryFunctionByName("TrigFunctions::sin")
	got, err := fn.invoke(libCtx(t), calcArgs{named: map[string]Value{"theta": constReal(0)}})
	if err != nil || got.Const.Real != 0 {
		t.Fatalf("sin(theta = 0.0) = %+v, %v", got, err)
	}

	if _, err := fn.invoke(libCtx(t), calcArgs{named: map[string]Value{"x": constReal(0)}}); !errors.Is(err, ErrUnknownParameter) {
		t.Fatalf("sin(x = 0.0) error = %v, want %v", err, ErrUnknownParameter)
	}
}

// atan2 takes its arguments in the order y then x, so a named argument that
// names them the other way round still computes the angle to (x, y).
func TestLibraryFunctionAtan2NamedArguments(t *testing.T) {
	fn, _ := libraryFunctionByName("SystemicaMathFunctions::atan2")
	got, err := fn.invoke(libCtx(t), calcArgs{named: map[string]Value{"x": constReal(-1), "y": constReal(1)}})
	if err != nil || got.Const.Real != 3*math.Pi/4 {
		t.Fatalf("atan2(x = -1.0, y = 1.0) = %+v, %v; want 3pi/4", got, err)
	}
}

// The Systemica extension library ships the declarations these implementations
// are registered against, so the shipped signatures and the registry cannot
// drift: every function the file declares has an implementation whose parameters
// carry the declared names in the declared order.
func TestSystemicaMathFunctionsMatchTheShippedDeclarations(t *testing.T) {
	const path = "Systemica Libraries/SystemicaMathFunctions.kerml"
	data, err := libs.DefaultSource().Read(path)
	if err != nil {
		t.Fatalf("Read(%q): %v", path, err)
	}
	p := parser.New(source.New(path, data))
	file := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("%s has %d parse diagnostics, want 0: %v", path, len(p.Diagnostics), p.Diagnostics)
	}

	idx := symbols.NewIndex()
	idx.AddDocument(path, file)
	resolver := resolve.New(idx)
	ctx := NewContext(semantics.NewModel(resolver), resolver, 10000)

	declared := 0
	for _, sym := range idx.LookupDirectChildren("SystemicaMathFunctions") {
		if !isCalcSymbol(sym) {
			continue
		}
		declared++
		fqn := ctx.qualifiedSymbolName(sym)
		fn, ok := ctx.libraryFunctionFor(sym)
		if !ok {
			t.Errorf("%s is declared in %s but has no built-in implementation", fqn, path)
			continue
		}
		var params []string
		for _, param := range calcParameters(ctx.calcChain(sym)) {
			params = append(params, param.Name)
		}
		if len(params) != len(fn.params) {
			t.Errorf("%s declares %v, implementation takes %v", fqn, params, fn.params)
			continue
		}
		for i, name := range params {
			if fn.params[i] != name {
				t.Errorf("%s parameter %d is %q, implementation names it %q", fqn, i, name, fn.params[i])
			}
		}
	}
	if declared != 4 {
		t.Errorf("%s declares %d functions, want 4 (exp, ln, log, atan2)", path, declared)
	}
}

// Every unqualified name denotes a registered function, and dispatch by
// unqualified name is what makes a call evaluable in a model that imports no
// part of the function library.
func TestLibraryFunctionUnqualifiedNames(t *testing.T) {
	for local, fn := range libraryFunctionsByLocalName {
		if _, ok := libraryFunctions[fn.name]; !ok {
			t.Errorf("unqualified name %q maps to unregistered %q", local, fn.name)
		}
	}

	for _, local := range []string{"sqrt", "abs", "max", "min", "floor", "round", "sin", "cos", "tan", "cot", "arcsin", "arccos", "arctan", "isZero", "isUnit", "exp", "ln", "log", "atan2"} {
		if _, ok := libraryFunctionsByLocalName[local]; !ok {
			t.Errorf("unqualified name %q is not dispatchable", local)
		}
	}
}

// A call written with a qualified name that resolves to the library declaration
// is applied by the built-in implementation, because the library gives that
// declaration no body.
func TestLibraryFunctionDispatchByResolvedSymbol(t *testing.T) {
	ctx, idx := contextForSource(t, `package RealFunctions {
	function sqrt { in x : Real[1]; return : Real[1]; }
}`)
	sym := lookupOne(t, idx, "RealFunctions::sqrt")

	if _, ok := ctx.libraryFunctionFor(sym); !ok {
		t.Fatalf("RealFunctions::sqrt did not dispatch to its built-in implementation")
	}
	got, err := ctx.InvokeCalc(sym, []Value{constReal(25)}, nil)
	if err != nil || got.Const.Real != 5 {
		t.Fatalf("InvokeCalc(sqrt, 25.0) = %+v, %v", got, err)
	}
}

// A declaration that carries a body is evaluated from that body, so a model's
// own calc is never hijacked by a built-in of the same name.
func TestLibraryFunctionDoesNotHijackADeclaredBody(t *testing.T) {
	ctx, idx := contextForSource(t, `package RealFunctions {
	calc sqrt { in x : Real; return : Real = x; }
}
package mine {
	calc sqrt { in x : Real; return : Real = 42.0; }
}`)

	libSym := lookupOne(t, idx, "RealFunctions::sqrt")
	if _, ok := ctx.libraryFunctionFor(libSym); ok {
		t.Fatalf("a declaration with a body dispatched to the built-in implementation")
	}
	got, err := ctx.InvokeCalc(libSym, []Value{constReal(25)}, nil)
	if err != nil || got.Const.Real != 25 {
		t.Fatalf("InvokeCalc(RealFunctions::sqrt, 25.0) = %+v, %v; want the declared body", got, err)
	}

	ownSym := lookupOne(t, idx, "mine::sqrt")
	got, err = ctx.InvokeCalc(ownSym, []Value{constReal(25)}, nil)
	if err != nil || got.Const.Real != 42 {
		t.Fatalf("InvokeCalc(mine::sqrt, 25.0) = %+v, %v; want the declared body", got, err)
	}
}

// A calc computing its result by assigning its output states a body too, so the
// built-in of the same name does not answer in its place.
func TestLibraryFunctionDoesNotHijackAnOutputAssignedInABody(t *testing.T) {
	ctx, idx := contextForSource(t, `package RealFunctions {
	calc sqrt { in x : Real; out r : Real; r = 42.0; }
}`)

	sym := lookupOne(t, idx, "RealFunctions::sqrt")
	if _, ok := ctx.libraryFunctionFor(sym); ok {
		t.Fatalf("a declaration assigning its output dispatched to the built-in implementation")
	}
	got, err := ctx.InvokeCalc(sym, []Value{constReal(25)}, nil)
	if err != nil || got.Const.Real != 42 {
		t.Fatalf("InvokeCalc(RealFunctions::sqrt, 25.0) = %+v, %v; want the declared body", got, err)
	}
}

// A library symbol loaded from the library index carries a kind and no
// declaration, and dispatches on that kind.
func TestLibraryFunctionDispatchByCachedSymbol(t *testing.T) {
	idx := symbols.NewIndex()
	idx.AddRecords("lib", []symbols.RecordEntry{
		{FQN: "RealFunctions", Kind: symbols.SymbolPackage},
		{FQN: "RealFunctions::sqrt", Kind: symbols.SymbolCalcDef},
		{FQN: "RealFunctions::tolerance", Kind: symbols.SymbolAttributeUsage},
	})
	resolver := resolve.New(idx)
	ctx := NewContext(semantics.NewModel(resolver), resolver, 10000)

	fnSym := lookupOne(t, idx, "RealFunctions::sqrt")
	if _, ok := ctx.libraryFunctionFor(fnSym); !ok {
		t.Fatalf("the cached RealFunctions::sqrt did not dispatch to its built-in implementation")
	}
	attrSym := lookupOne(t, idx, "RealFunctions::tolerance")
	if _, ok := ctx.libraryFunctionFor(attrSym); ok {
		t.Fatalf("a cached attribute dispatched to a built-in implementation")
	}
}

// A declaration that is not a calc keeps the not-a-calc diagnostic, however it
// is named: only a function is a library function.
func TestLibraryFunctionDoesNotClaimANonCalcDeclaration(t *testing.T) {
	ctx, idx := contextForSource(t, `package RealFunctions {
	attribute sqrt = 3.0;
}`)
	sym := lookupOne(t, idx, "RealFunctions::sqrt")

	if _, ok := ctx.libraryFunctionFor(sym); ok {
		t.Fatalf("an attribute named sqrt dispatched to the built-in implementation")
	}
	if _, err := ctx.InvokeCalc(sym, []Value{constReal(2)}, nil); !errors.Is(err, ErrNotACalc) {
		t.Fatalf("InvokeCalc(attribute sqrt) error = %v; want ErrNotACalc", err)
	}
}

// contextForSource indexes src as one document and returns a runtime context
// over it.
func contextForSource(t *testing.T, src string) (*Context, *symbols.Index) {
	t.Helper()
	file := parser.New(source.New("<test>", []byte(src))).ParseFile()
	idx := symbols.NewIndex()
	idx.AddDocument("<test>", file)
	resolver := resolve.New(idx)
	return NewContext(semantics.NewModel(resolver), resolver, 10000), idx
}

// lookupOne returns the single symbol with that fully-qualified name.
func lookupOne(t *testing.T, idx *symbols.Index, fqn string) *symbols.Symbol {
	t.Helper()
	syms := idx.LookupQualified(fqn)
	if len(syms) != 1 {
		t.Fatalf("LookupQualified(%q) returned %d symbols, want 1", fqn, len(syms))
	}
	return syms[0]
}

// vectorValues is the elements of a vector result, which is the sequence of its
// elements.
func vectorValues(t *testing.T, val Value) []semantics.Value {
	t.Helper()
	if val.Kind != ValSequence {
		t.Fatalf("result is %s, want a vector (a sequence of its elements)", val.Kind)
	}
	out := make([]semantics.Value, 0, val.Sequence.Size())
	for _, elem := range val.Sequence.Elements() {
		if elem.Kind != ValConst {
			t.Fatalf("vector element is %s, want a constant", elem.Kind)
		}
		out = append(out, elem.Const)
	}
	return out
}

func realConsts(reals ...float64) []semantics.Value {
	out := make([]semantics.Value, len(reals))
	for i, x := range reals {
		out[i] = semantics.Value{Kind: semantics.ValReal, Real: x}
	}
	return out
}

func intConsts(ints ...int64) []semantics.Value {
	out := make([]semantics.Value, len(ints))
	for i, n := range ints {
		out[i] = semantics.Value{Kind: semantics.ValInt, Int: n}
	}
	return out
}

// vec is a vector argument: the sequence of its elements.
func vec(elements ...Value) Value { return sequenceOf(elements) }

func realVec(reals ...float64) Value {
	elements := make([]Value, len(reals))
	for i, x := range reals {
		elements[i] = constReal(x)
	}
	return vec(elements...)
}

// A vector is the sequence of its elements, and the operations VectorFunctions
// declares abstractly over VectorValue compute for every vector this runtime
// represents, so the abstract name and its Cartesian specialization agree.
func TestVectorFunctionValues(t *testing.T) {
	cases := []struct {
		fn   string
		args []Value
		want []semantics.Value
	}{
		{"VectorFunctions::VectorOf", []Value{vec(constInt(1), constInt(2))}, intConsts(1, 2)},
		{"VectorFunctions::VectorOf", []Value{constInt(3)}, intConsts(3)},
		{"VectorFunctions::CartesianVectorOf", []Value{vec(constInt(1), constReal(2.5))}, realConsts(1, 2.5)},
		{"VectorFunctions::CartesianVectorOf", []Value{nullValue()}, nil},
		{"VectorFunctions::CartesianThreeVectorOf", []Value{realVec(1, 2, 3)}, realConsts(1, 2, 3)},
		// '+' and '-' keep the elements' kind, as the declaration over
		// NumericalValue does; the Cartesian specializations are Real.
		{"VectorFunctions::+", []Value{vec(constInt(1), constInt(2)), vec(constInt(3), constInt(4))}, intConsts(4, 6)},
		{"VectorFunctions::cartesian+", []Value{realVec(1, 2), realVec(3, 4)}, realConsts(4, 6)},
		{"VectorFunctions::-", []Value{vec(constInt(1), constInt(2)), vec(constInt(4), constInt(4))}, intConsts(-3, -2)},
		{"VectorFunctions::cartesian-", []Value{realVec(1, 2), realVec(0.5, 0.5)}, realConsts(0.5, 1.5)},
		{"VectorFunctions::scalarVectorMult", []Value{constInt(2), vec(constInt(1), constInt(2))}, intConsts(2, 4)},
		{"VectorFunctions::*", []Value{constReal(0.5), realVec(1, 2)}, realConsts(0.5, 1)},
		{"VectorFunctions::vectorScalarMult", []Value{realVec(1, 2), constReal(3)}, realConsts(3, 6)},
		{"VectorFunctions::cartesianVectorScalarMult", []Value{realVec(1, 2), constReal(3)}, realConsts(3, 6)},
		// The library defines the quotient as the product with 1.0 / x, whose
		// rounding a division does not carry: 1.0 / 3.0 * 3.0 is 0.9999999999999998.
		{"VectorFunctions::vectorScalarDiv", []Value{realVec(3, 6), constReal(3)}, realConsts(1, 2)},
	}

	for _, tc := range cases {
		t.Run(tc.fn, func(t *testing.T) {
			got, err := applyLibrary(t, tc.fn, tc.args...)
			if err != nil {
				t.Fatalf("%s = error %v", tc.fn, err)
			}
			elements := vectorValues(t, got)
			if len(elements) != len(tc.want) {
				t.Fatalf("%s = %v, want %v", tc.fn, elements, tc.want)
			}
			for i := range elements {
				if elements[i] != tc.want[i] {
					t.Fatalf("%s = %v, want %v", tc.fn, elements, tc.want)
				}
			}
		})
	}
}

// The scalar-valued vector operations: the inner product keeps the elements'
// kind, the norm and the angle are Reals.
func TestVectorFunctionScalarValues(t *testing.T) {
	cases := []struct {
		fn   string
		args []Value
		want semantics.Value
	}{
		{"VectorFunctions::inner", []Value{vec(constInt(1), constInt(2)), vec(constInt(3), constInt(4))}, semantics.Value{Kind: semantics.ValInt, Int: 11}},
		{"VectorFunctions::cartesianInner", []Value{realVec(1, 2), realVec(3, 4)}, semantics.Value{Kind: semantics.ValReal, Real: 11}},
		{"VectorFunctions::norm", []Value{realVec(3, 4)}, semantics.Value{Kind: semantics.ValReal, Real: 5}},
		{"VectorFunctions::cartesianNorm", []Value{realVec(3, 4)}, semantics.Value{Kind: semantics.ValReal, Real: 5}},
		{"VectorFunctions::angle", []Value{realVec(1, 0), realVec(0, 1)}, semantics.Value{Kind: semantics.ValReal, Real: math.Pi / 2}},
		// Two parallel vectors: the cosine rounds to just above 1.0, where the
		// arc cosine has no value, and the angle is 0.
		{"VectorFunctions::cartesianAngle", []Value{realVec(1, 2, 3), realVec(2, 4, 6)}, semantics.Value{Kind: semantics.ValReal, Real: 0}},
		{"VectorFunctions::isZeroVector", []Value{realVec(0, 0)}, semantics.Value{Kind: semantics.ValBool, Bool: true}},
		{"VectorFunctions::isZeroVector", []Value{realVec(0, 1)}, semantics.Value{Kind: semantics.ValBool, Bool: false}},
		{"VectorFunctions::isCartesianZeroVector", []Value{realVec(0, 0, 0)}, semantics.Value{Kind: semantics.ValBool, Bool: true}},
	}

	for _, tc := range cases {
		t.Run(tc.fn, func(t *testing.T) {
			got, err := applyLibrary(t, tc.fn, tc.args...)
			if err != nil {
				t.Fatalf("%s = error %v", tc.fn, err)
			}
			if got.Kind != ValConst || got.Const != tc.want {
				t.Fatalf("%s = %+v, want %+v", tc.fn, got, tc.want)
			}
		})
	}
}

// A Complex is the (re, im) pair rect constructs, and a Real is a Complex with a
// zero imaginary part (ScalarValues declares Real :> Complex), so both bind to a
// Complex parameter.
func TestComplexFunctionValues(t *testing.T) {
	cases := []struct {
		fn   string
		args []Value
		want []semantics.Value
	}{
		{"ComplexFunctions::rect", []Value{constReal(1), constReal(2)}, realConsts(1, 2)},
		{"ComplexFunctions::rect", []Value{constInt(1), constInt(2)}, realConsts(1, 2)},
		{"ComplexFunctions::polar", []Value{constReal(2), constReal(0)}, realConsts(2, 0)},
		{"ComplexFunctions::+", []Value{realVec(1, 2), realVec(3, 4)}, realConsts(4, 6)},
		{"ComplexFunctions::+", []Value{realVec(1, 2), constReal(3)}, realConsts(4, 2)},
		{"ComplexFunctions::-", []Value{realVec(1, 2), realVec(3, 4)}, realConsts(-2, -2)},
		{"ComplexFunctions::*", []Value{realVec(0, 1), realVec(0, 1)}, realConsts(-1, 0)},
		{"ComplexFunctions::*", []Value{realVec(1, 2), constReal(2)}, realConsts(2, 4)},
		{"ComplexFunctions::/", []Value{realVec(-1, 0), realVec(0, 1)}, realConsts(0, 1)},
		{"ComplexFunctions::**", []Value{realVec(0, 1), constInt(2)}, realConsts(-1, 0)},
		{"ComplexFunctions::^", []Value{constReal(2), constInt(3)}, realConsts(8, 0)},
	}

	for _, tc := range cases {
		t.Run(tc.fn, func(t *testing.T) {
			got, err := applyLibrary(t, tc.fn, tc.args...)
			if err != nil {
				t.Fatalf("%s = error %v", tc.fn, err)
			}
			elements := vectorValues(t, got)
			if len(elements) != len(tc.want) {
				t.Fatalf("%s = %v, want %v", tc.fn, elements, tc.want)
			}
			// The products and quotients of a Complex round, so compare the parts
			// as Reals rather than bit for bit.
			for i := range elements {
				if math.Abs(asReal(elements[i])-asReal(tc.want[i])) > 1e-12 {
					t.Fatalf("%s = %v, want %v", tc.fn, elements, tc.want)
				}
			}
		})
	}
}

func TestComplexFunctionScalarValues(t *testing.T) {
	cases := []struct {
		fn   string
		args []Value
		want semantics.Value
	}{
		{"ComplexFunctions::re", []Value{realVec(1, 2)}, semantics.Value{Kind: semantics.ValReal, Real: 1}},
		{"ComplexFunctions::im", []Value{realVec(1, 2)}, semantics.Value{Kind: semantics.ValReal, Real: 2}},
		{"ComplexFunctions::im", []Value{constReal(1)}, semantics.Value{Kind: semantics.ValReal, Real: 0}},
		{"ComplexFunctions::abs", []Value{realVec(3, 4)}, semantics.Value{Kind: semantics.ValReal, Real: 5}},
		{"ComplexFunctions::arg", []Value{realVec(0, 1)}, semantics.Value{Kind: semantics.ValReal, Real: math.Pi / 2}},
		{"ComplexFunctions::isZero", []Value{realVec(0, 0)}, semantics.Value{Kind: semantics.ValBool, Bool: true}},
		{"ComplexFunctions::isZero", []Value{constInt(0)}, semantics.Value{Kind: semantics.ValBool, Bool: true}},
		{"ComplexFunctions::isUnit", []Value{realVec(1, 0)}, semantics.Value{Kind: semantics.ValBool, Bool: true}},
		{"ComplexFunctions::isUnit", []Value{realVec(1, 1)}, semantics.Value{Kind: semantics.ValBool, Bool: false}},
		// A Real and the pair with a zero imaginary part are the same Complex.
		{"ComplexFunctions::==", []Value{constReal(2), realVec(2, 0)}, semantics.Value{Kind: semantics.ValBool, Bool: true}},
		{"ComplexFunctions::==", []Value{realVec(2, 1), realVec(2, 0)}, semantics.Value{Kind: semantics.ValBool, Bool: false}},
		// Both operands are declared [0..1]: two empty operands are equal.
		{"ComplexFunctions::==", []Value{nullValue(), nullValue()}, semantics.Value{Kind: semantics.ValBool, Bool: true}},
		{"ComplexFunctions::==", []Value{constReal(0), nullValue()}, semantics.Value{Kind: semantics.ValBool, Bool: false}},
	}

	for _, tc := range cases {
		t.Run(tc.fn, func(t *testing.T) {
			got, err := applyLibrary(t, tc.fn, tc.args...)
			if err != nil {
				t.Fatalf("%s = error %v", tc.fn, err)
			}
			if got.Kind != ValConst || got.Const != tc.want {
				t.Fatalf("%s = %+v, want %+v", tc.fn, got, tc.want)
			}
		})
	}
}

// TrigFunctions::deg and ::rad convert with the pi the feature seam supplies,
// which is what their library bodies read.
func TestTrigDegreesAndRadians(t *testing.T) {
	got, err := applyLibrary(t, "TrigFunctions::deg", constReal(math.Pi))
	if err != nil || got.Const.Real != 180 {
		t.Fatalf("deg(pi) = %+v, %v; want 180.0", got, err)
	}
	got, err = applyLibrary(t, "TrigFunctions::rad", constReal(180))
	if err != nil || got.Const.Real != math.Pi {
		t.Fatalf("rad(180.0) = %+v, %v; want pi", got, err)
	}
	got, err = applyLibrary(t, "TrigFunctions::rad", constInt(0))
	if err != nil || got.Const.Real != 0 {
		t.Fatalf("rad(0) = %+v, %v; want 0.0", got, err)
	}
}

// The library declares the second operand of '+' and '-' [0..1]: with one
// argument, '+' is that value and '-' is its inverse.
func TestLibraryFunctionOptionalOperand(t *testing.T) {
	got, err := applyLibrary(t, "VectorFunctions::cartesian+", realVec(1, 2))
	if err != nil {
		t.Fatalf("cartesian+((1.0, 2.0)) = error %v", err)
	}
	if elements := vectorValues(t, got); len(elements) != 2 || elements[0].Real != 1 || elements[1].Real != 2 {
		t.Fatalf("cartesian+((1.0, 2.0)) = %v, want (1.0, 2.0)", got)
	}

	got, err = applyLibrary(t, "VectorFunctions::-", vec(constInt(1), constInt(-2)))
	if err != nil {
		t.Fatalf("-((1, -2)) = error %v", err)
	}
	if elements := vectorValues(t, got); len(elements) != 2 || elements[0].Int != -1 || elements[1].Int != 2 {
		t.Fatalf("-((1, -2)) = %v, want (-1, 2)", got)
	}

	got, err = applyLibrary(t, "ComplexFunctions::-", realVec(1, 2))
	if err != nil {
		t.Fatalf("-(1.0 + 2.0i) = error %v", err)
	}
	if elements := vectorValues(t, got); len(elements) != 2 || elements[0].Real != -1 || elements[1].Real != -2 {
		t.Fatalf("-(1.0 + 2.0i) = %v, want -1.0 - 2.0i", got)
	}

	// A required operand is still required: '*' declares both [1].
	if _, err := applyLibrary(t, "ComplexFunctions::*", realVec(1, 2)); !errors.Is(err, ErrCalcArity) {
		t.Fatalf("*(1.0 + 2.0i) error = %v, want %v", err, ErrCalcArity)
	}
}

// An empty collection written for the optional operand is the same no value as
// null, so the call answers as the one-argument form rather than reporting.
func TestLibraryFunctionEmptyOptionalOperand(t *testing.T) {
	got, err := applyLibrary(t, "VectorFunctions::cartesian+", realVec(1, 2), vec())
	if err != nil {
		t.Fatalf("cartesian+((1.0, 2.0), ()) = error %v", err)
	}
	if elements := vectorValues(t, got); len(elements) != 2 || elements[0].Real != 1 || elements[1].Real != 2 {
		t.Fatalf("cartesian+((1.0, 2.0), ()) = %v, want (1.0, 2.0)", got)
	}

	got, err = applyLibrary(t, "VectorFunctions::cartesian-", realVec(1, 2), vec())
	if err != nil {
		t.Fatalf("cartesian-((1.0, 2.0), ()) = error %v", err)
	}
	if elements := vectorValues(t, got); len(elements) != 2 || elements[0].Real != -1 || elements[1].Real != -2 {
		t.Fatalf("cartesian-((1.0, 2.0), ()) = %v, want (-1.0, -2.0)", got)
	}

	got, err = applyLibrary(t, "ComplexFunctions::+", realVec(1, 2), vec())
	if err != nil {
		t.Fatalf("+(1.0 + 2.0i, ()) = error %v", err)
	}
	if elements := vectorValues(t, got); len(elements) != 2 || elements[0].Real != 1 || elements[1].Real != 2 {
		t.Fatalf("+(1.0 + 2.0i, ()) = %v, want 1.0 + 2.0i", got)
	}

	for _, tc := range []struct {
		args []Value
		want bool
	}{
		{[]Value{vec(), vec()}, true},
		{[]Value{vec(), nullValue()}, true},
		{[]Value{realVec(1, 2), vec()}, false},
	} {
		got, err := applyLibrary(t, "ComplexFunctions::==", tc.args...)
		if err != nil || got.Kind != ValConst || got.Const.Bool != tc.want {
			t.Errorf("==(%v) = (%v, %v), want %v", tc.args, got, err, tc.want)
		}
	}
}

// A named argument binds to the parameter name the vendored vector and Complex
// signatures declare, whichever order the call names them in.
func TestVectorAndComplexNamedArguments(t *testing.T) {
	fn, _ := libraryFunctionByName("VectorFunctions::scalarVectorMult")
	got, err := fn.invoke(libCtx(t), calcArgs{named: map[string]Value{"v": realVec(1, 2), "x": constReal(3)}})
	if err != nil {
		t.Fatalf("scalarVectorMult(v = (1.0, 2.0), x = 3.0) = error %v", err)
	}
	if elements := vectorValues(t, got); len(elements) != 2 || elements[0].Real != 3 || elements[1].Real != 6 {
		t.Fatalf("scalarVectorMult(v = (1.0, 2.0), x = 3.0) = %v, want (3.0, 6.0)", got)
	}

	fn, _ = libraryFunctionByName("ComplexFunctions::rect")
	got, err = fn.invoke(libCtx(t), calcArgs{named: map[string]Value{"im": constReal(2), "re": constReal(1)}})
	if err != nil {
		t.Fatalf("rect(im = 2.0, re = 1.0) = error %v", err)
	}
	if elements := vectorValues(t, got); len(elements) != 2 || elements[0].Real != 1 || elements[1].Real != 2 {
		t.Fatalf("rect(im = 2.0, re = 1.0) = %v, want 1.0 + 2.0i", got)
	}

	// An omitted optional parameter binds empty, so naming only `v` is a call
	// with one argument rather than an unknown-parameter error.
	fn, _ = libraryFunctionByName("VectorFunctions::+")
	if _, err := fn.invoke(libCtx(t), calcArgs{named: map[string]Value{"v": realVec(1, 2)}}); err != nil {
		t.Fatalf("+(v = (1.0, 2.0)) = error %v", err)
	}
	if _, err := fn.invoke(libCtx(t), calcArgs{named: map[string]Value{"u": realVec(1, 2)}}); !errors.Is(err, ErrUnknownParameter) {
		t.Fatalf("+(u = (1.0, 2.0)) error = %v, want %v", err, ErrUnknownParameter)
	}
}

// A name no parameter of the signature carries is reported, rather than absorbed
// by an optional parameter the call then leaves empty — which would answer the
// call as if the argument had not been written.
func TestVectorAndComplexUnknownNamedArgument(t *testing.T) {
	for _, tt := range []struct {
		fn    string
		named map[string]Value
	}{
		{"VectorFunctions::+", map[string]Value{"v": realVec(1, 2), "zz": realVec(3, 4)}},
		{"VectorFunctions::cartesian+", map[string]Value{"v": realVec(1, 2), "zz": realVec(3, 4)}},
		{"VectorFunctions::-", map[string]Value{"v": realVec(1, 2), "zz": realVec(3, 4)}},
		{"ComplexFunctions::+", map[string]Value{"x": realVec(1, 2), "zz": realVec(3, 4)}},
		{"ComplexFunctions::-", map[string]Value{"x": realVec(1, 2), "zz": realVec(3, 4)}},
		{"ComplexFunctions::==", map[string]Value{"zz": constReal(1)}},
		{"ComplexFunctions::==", map[string]Value{"x": realVec(1, 2), "zz": realVec(1, 2)}},
	} {
		fn, ok := libraryFunctionByName(tt.fn)
		if !ok {
			t.Fatalf("%s is not registered", tt.fn)
		}
		got, err := fn.invoke(libCtx(t), calcArgs{named: tt.named})
		if !errors.Is(err, ErrUnknownParameter) {
			t.Errorf("%s with an unknown name = (%v, %v), want %v", tt.fn, got, err, ErrUnknownParameter)
		}
	}
}

func TestVectorAndComplexFunctionErrors(t *testing.T) {
	cases := []struct {
		name string
		fn   string
		args []Value
		want error
	}{
		{"vectors of unequal dimension", "VectorFunctions::cartesian+", []Value{realVec(1, 2), realVec(1, 2, 3)}, ErrTypeMismatch},
		{"inner product of unequal dimensions", "VectorFunctions::inner", []Value{realVec(1), realVec(1, 2)}, ErrTypeMismatch},
		{"angle of unequal dimensions", "VectorFunctions::angle", []Value{realVec(1), realVec(1, 2)}, ErrTypeMismatch},
		{"a string element", "VectorFunctions::norm", []Value{vec(constReal(1), Value{Kind: ValString, Str: "2"})}, ErrTypeMismatch},
		{"a boolean element", "VectorFunctions::isZeroVector", []Value{vec(boolValue(true))}, ErrTypeMismatch},
		{"a vector where a scalar is declared", "VectorFunctions::scalarVectorMult", []Value{realVec(1, 2), realVec(1, 2)}, ErrTypeMismatch},
		{"no components", "VectorFunctions::VectorOf", []Value{nullValue()}, ErrMultiplicityViolation},
		{"two components of a three-vector", "VectorFunctions::CartesianThreeVectorOf", []Value{realVec(1, 2)}, ErrMultiplicityViolation},
		{"division of a vector by zero", "VectorFunctions::vectorScalarDiv", []Value{realVec(1, 2), constReal(0)}, ErrDivisionByZero},
		{"the angle to a zero vector", "VectorFunctions::angle", []Value{realVec(0, 0), realVec(1, 0)}, semantics.ErrArithmeticDomain},
		{"a norm beyond the Real range", "VectorFunctions::norm", []Value{realVec(1e200, 1e200)}, semantics.ErrArithmeticOverflow},
		{"a sum beyond the Real range", "VectorFunctions::cartesian+", []Value{realVec(1e308, 1), realVec(1e308, 1)}, semantics.ErrArithmeticOverflow},
		{"a difference beyond the Real range", "VectorFunctions::cartesian-", []Value{realVec(1e308, 1), realVec(-1e308, 1)}, semantics.ErrArithmeticOverflow},
		{"a scaled element beyond the Real range", "VectorFunctions::scalarVectorMult", []Value{constReal(1e300), realVec(1e300)}, semantics.ErrArithmeticOverflow},
		{"an inner product beyond the Real range", "VectorFunctions::cartesianInner", []Value{realVec(1e200), realVec(1e200)}, semantics.ErrArithmeticOverflow},
		{"an Integer sum beyond the Integer range", "VectorFunctions::+", []Value{vec(constInt(math.MaxInt64)), vec(constInt(1))}, semantics.ErrArithmeticOverflow},
		{"an Integer difference beyond the Integer range", "VectorFunctions::-", []Value{vec(constInt(math.MinInt64)), vec(constInt(1))}, semantics.ErrArithmeticOverflow},
		{"the negation of the least Integer", "VectorFunctions::-", []Value{vec(constInt(math.MinInt64))}, semantics.ErrArithmeticOverflow},
		{"an Integer scaling beyond the Integer range", "VectorFunctions::scalarVectorMult", []Value{constInt(2), vec(constInt(math.MaxInt64))}, semantics.ErrArithmeticOverflow},
		{"an Integer inner product beyond the Integer range", "VectorFunctions::inner", []Value{vec(constInt(math.MaxInt64), constInt(1)), vec(constInt(1), constInt(1))}, semantics.ErrArithmeticOverflow},
		{"three parts of a Complex", "ComplexFunctions::re", []Value{realVec(1, 2, 3)}, ErrTypeMismatch},
		{"an empty Complex", "ComplexFunctions::abs", []Value{nullValue()}, ErrTypeMismatch},
		{"a string part of a Complex", "ComplexFunctions::im", []Value{vec(constReal(1), Value{Kind: ValString, Str: "2"})}, ErrTypeMismatch},
		{"a vector where rect declares a Real", "ComplexFunctions::rect", []Value{realVec(1, 2), constReal(0)}, ErrTypeMismatch},
		{"the argument of zero", "ComplexFunctions::arg", []Value{realVec(0, 0)}, semantics.ErrArithmeticDomain},
		{"division of a Complex by zero", "ComplexFunctions::/", []Value{realVec(1, 2), realVec(0, 0)}, ErrDivisionByZero},
		{"zero to a negative power", "ComplexFunctions::**", []Value{constReal(0), constReal(-1)}, semantics.ErrArithmeticDomain},
		{"a power beyond the Real range", "ComplexFunctions::**", []Value{constReal(1e200), constReal(2)}, semantics.ErrArithmeticOverflow},
		{"too many arguments to a vector constructor", "VectorFunctions::CartesianVectorOf", []Value{realVec(1), realVec(2)}, ErrCalcArity},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := applyLibrary(t, tc.fn, tc.args...)
			if !errors.Is(err, tc.want) {
				t.Fatalf("%s = %+v, %v; want error %v", tc.fn, got, err, tc.want)
			}
		})
	}
}

// TestStringFunctionValues pins StringFunctions: Length counts characters (one
// per Unicode code point, not per byte) and Substring takes 1-based inclusive
// character positions.
func TestStringFunctionValues(t *testing.T) {
	cases := []struct {
		fn   string
		args []Value
		want Value
	}{
		{"StringFunctions::+", []Value{strValue("ab"), strValue("cd")}, strValue("abcd")},
		{"StringFunctions::+", []Value{strValue(""), strValue("")}, strValue("")},
		{"StringFunctions::Length", []Value{strValue("abc")}, integerValue(3)},
		{"StringFunctions::Length", []Value{strValue("")}, integerValue(0)},
		// "héllo" is 6 bytes and 5 characters; Length answers characters.
		{"StringFunctions::Length", []Value{strValue("héllo")}, integerValue(5)},
		{"StringFunctions::Length", []Value{strValue("日本語")}, integerValue(3)},
		{"StringFunctions::Substring", []Value{strValue("abc"), constInt(1), constInt(1)}, strValue("a")},
		{"StringFunctions::Substring", []Value{strValue("abc"), constInt(2), constInt(3)}, strValue("bc")},
		{"StringFunctions::Substring", []Value{strValue("abc"), constInt(1), constInt(3)}, strValue("abc")},
		// An upper below lower selects no character, as
		// SequenceFunctions::subsequence answers nothing for such a range.
		{"StringFunctions::Substring", []Value{strValue("abc"), constInt(2), constInt(1)}, strValue("")},
		{"StringFunctions::Substring", []Value{strValue(""), constInt(1), constInt(0)}, strValue("")},
		{"StringFunctions::Substring", []Value{strValue("héllo"), constInt(2), constInt(3)}, strValue("él")},
		{"StringFunctions::<", []Value{strValue("abc"), strValue("b")}, boolValue(true)},
		{"StringFunctions::<", []Value{strValue("abc"), strValue("abc")}, boolValue(false)},
		{"StringFunctions::<=", []Value{strValue("abc"), strValue("abc")}, boolValue(true)},
		{"StringFunctions::>", []Value{strValue("abc"), strValue("abb")}, boolValue(true)},
		{"StringFunctions::>=", []Value{strValue("abc"), strValue("abd")}, boolValue(false)},
		{"StringFunctions::==", []Value{strValue("abc"), strValue("abc")}, boolValue(true)},
		{"StringFunctions::==", []Value{strValue("abc"), strValue("abd")}, boolValue(false)},
		// '==' declares String[0..1], so an omitted operand is a value it has an
		// answer for: two of them are equal, one of them is not equal to a string.
		{"StringFunctions::==", []Value{nullValue(), nullValue()}, boolValue(true)},
		{"StringFunctions::==", []Value{strValue(""), nullValue()}, boolValue(false)},
		{"StringFunctions::ToString", []Value{strValue("héllo")}, strValue("héllo")},
	}

	for _, tc := range cases {
		got, err := applyLibrary(t, tc.fn, tc.args...)
		if err != nil {
			t.Errorf("%s%v = error %v", tc.fn, tc.args, err)
			continue
		}
		if !valueEqual(got, tc.want) {
			t.Errorf("%s%v = %+v, want %+v", tc.fn, tc.args, got, tc.want)
		}
	}
}

// TestStringFunctionErrors pins the reports of StringFunctions: a position
// naming no character, and an argument of another type, which is reported rather
// than rendered as a string.
func TestStringFunctionErrors(t *testing.T) {
	cases := []struct {
		name string
		fn   string
		args []Value
		want error
	}{
		{"substring below the first character", "StringFunctions::Substring",
			[]Value{strValue("abc"), constInt(0), constInt(2)}, ErrIndexOutOfRange},
		{"substring past the last character", "StringFunctions::Substring",
			[]Value{strValue("abc"), constInt(1), constInt(9)}, ErrIndexOutOfRange},
		// The bound is in characters: "héllo" has 5, though it is 6 bytes.
		{"substring past the last character of a multi-byte string", "StringFunctions::Substring",
			[]Value{strValue("héllo"), constInt(1), constInt(6)}, ErrIndexOutOfRange},
		{"substring of an empty string", "StringFunctions::Substring",
			[]Value{strValue(""), constInt(1), constInt(1)}, ErrIndexOutOfRange},
		{"length of a number", "StringFunctions::Length",
			[]Value{constInt(3)}, ErrTypeMismatch},
		{"concatenation with a number", "StringFunctions::+",
			[]Value{strValue("abc"), constInt(3)}, ErrTypeMismatch},
		{"comparison against a number", "StringFunctions::<",
			[]Value{strValue("abc"), constInt(3)}, ErrTypeMismatch},
		{"equality against a number", "StringFunctions::==",
			[]Value{strValue("abc"), constInt(3)}, ErrTypeMismatch},
		{"substring at a non-integer position", "StringFunctions::Substring",
			[]Value{strValue("abc"), strValue("a"), constInt(2)}, ErrTypeMismatch},
		{"substring of a collection", "StringFunctions::Substring",
			[]Value{vec(strValue("a")), constInt(1), constInt(1)}, ErrTypeMismatch},
		{"length without an argument", "StringFunctions::Length", nil, ErrCalcArity},
		{"concatenation of three strings", "StringFunctions::+",
			[]Value{strValue("a"), strValue("b"), strValue("c")}, ErrCalcArity},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := applyLibrary(t, tc.fn, tc.args...)
			if !errors.Is(err, tc.want) {
				t.Fatalf("%s%v = %+v, %v; want error %v", tc.fn, tc.args, got, err, tc.want)
			}
			if !strings.Contains(err.Error(), tc.fn) {
				t.Fatalf("%s error %q does not name the function", tc.fn, err)
			}
		})
	}
}

// A named argument binds to the parameter names StringFunctions declares.
func TestStringFunctionNamedArguments(t *testing.T) {
	fn, ok := libraryFunctionByName("StringFunctions::Substring")
	if !ok {
		t.Fatal("StringFunctions::Substring is not registered")
	}
	got, err := fn.invoke(libCtx(t), calcArgs{named: map[string]Value{
		"upper": constInt(3),
		"x":     {Kind: ValString, Str: "abcd"},
		"lower": constInt(2),
	}})
	if err != nil {
		t.Fatalf("Substring(x = \"abcd\", lower = 2, upper = 3) = error %v", err)
	}
	if !valueEqual(got, Value{Kind: ValString, Str: "bc"}) {
		t.Fatalf("Substring(x = \"abcd\", lower = 2, upper = 3) = %+v, want \"bc\"", got)
	}
}

// A declaration this runtime has no representation for the values of reports
// itself by name rather than computing something else.
func TestUnevaluableLibraryFunctionsNameThemselves(t *testing.T) {
	unevaluable := []struct {
		fn   string
		args []Value
	}{
		{"VectorFunctions::sum", []Value{realVec(1, 2, 3)}},
		{"VectorFunctions::sum0", []Value{realVec(1, 2, 3), realVec(0, 0, 0)}},
		{"ComplexFunctions::sum", []Value{realVec(1, 2)}},
		{"ComplexFunctions::product", []Value{realVec(1, 2)}},
		{"ComplexFunctions::ToString", []Value{realVec(1, 2)}},
		{"ComplexFunctions::ToComplex", []Value{{Kind: ValString, Str: "1.0"}}},
	}

	for _, tc := range unevaluable {
		t.Run(tc.fn, func(t *testing.T) {
			_, err := applyLibrary(t, tc.fn, tc.args...)
			if !errors.Is(err, ErrUnevaluableLibraryFunction) {
				t.Fatalf("%s error = %v, want %v", tc.fn, err, ErrUnevaluableLibraryFunction)
			}
			if !strings.Contains(err.Error(), tc.fn) {
				t.Fatalf("%s error %q does not name the function", tc.fn, err)
			}
		})
	}
}

// The vendored declarations these implementations are registered against cannot
// drift from the registry: every function VectorFunctions, ComplexFunctions,
// SequenceFunctions and TrigFunctions declare is either implemented here with
// the declared parameter names in the declared order, or implemented as a
// built-in over collections (builtins.go), which takes its arguments
// positionally.
func TestVendoredFunctionsAreAllDispatchable(t *testing.T) {
	packages := map[string]string{
		"VectorFunctions":   "Kernel Libraries/Kernel Function Library/VectorFunctions.kerml",
		"ComplexFunctions":  "Kernel Libraries/Kernel Function Library/ComplexFunctions.kerml",
		"SequenceFunctions": "Kernel Libraries/Kernel Function Library/SequenceFunctions.kerml",
		"TrigFunctions":     "Kernel Libraries/Kernel Function Library/TrigFunctions.kerml",
		"StringFunctions":   "Kernel Libraries/Kernel Function Library/StringFunctions.kerml",
	}

	for pkg, path := range packages {
		t.Run(pkg, func(t *testing.T) {
			data, err := libs.DefaultSource().Read(path)
			if err != nil {
				t.Fatalf("Read(%q): %v", path, err)
			}
			p := parser.New(source.New(path, data))
			file := p.ParseFile()
			if len(p.Diagnostics) > 0 {
				t.Fatalf("%s has %d parse diagnostics, want 0: %v", path, len(p.Diagnostics), p.Diagnostics)
			}
			idx := symbols.NewIndex()
			idx.AddDocument(path, file)
			resolver := resolve.New(idx)
			ctx := NewContext(semantics.NewModel(resolver), resolver, 10000)

			for _, sym := range idx.LookupDirectChildren(pkg) {
				if !isCalcSymbol(sym) {
					continue
				}
				fqn := ctx.qualifiedSymbolName(sym)
				fn, ok := libraryFunctionByName(fqn)
				if !ok {
					if _, isBuiltin := builtins[fqn]; !isBuiltin {
						t.Errorf("%s is declared in %s and is not dispatchable", fqn, path)
					}
					continue
				}
				var params []string
				for _, param := range calcParameters(ctx.calcChain(sym)) {
					params = append(params, param.Name)
				}
				if len(params) != len(fn.params) {
					t.Errorf("%s declares %v, implementation takes %v", fqn, params, fn.params)
					continue
				}
				for i, name := range params {
					if fn.params[i] != name {
						t.Errorf("%s parameter %d is %q, implementation names it %q", fqn, i, name, fn.params[i])
					}
				}
			}
		})
	}
}

// A model reads a library feature by name even where the library index cache
// restored the symbol without a declaration to evaluate.
func TestLibraryFeatureNameReadFromACachedSymbol(t *testing.T) {
	root := parser.New(source.New("test.sysml", []byte(`package test {
	attribute twoPi = 2 * TrigFunctions::pi;
}`))).ParseFile()
	idx := symbols.NewIndex()
	idx.AddDocument("test.sysml", root)
	idx.AddRecords("lib", []symbols.RecordEntry{
		{FQN: "TrigFunctions", Kind: symbols.SymbolPackage},
		{FQN: "TrigFunctions::pi", Kind: symbols.SymbolAttributeUsage},
	})
	idx.MarkLibrary("lib")
	resolver := resolve.New(idx)

	pkg, ok := idx.DocumentRoot("test.sysml").LookupLocal("test")
	if !ok || pkg == nil || pkg.Scope == nil {
		t.Fatal("package test not found")
	}
	sym, ok := pkg.Scope.LookupLocal("twoPi")
	if !ok || sym == nil {
		t.Fatal("attribute twoPi not found")
	}
	decl, ok := sym.Decl.(*ast.Usage)
	if !ok || decl.Value == nil {
		t.Fatalf("twoPi declares %T with no value", sym.Decl)
	}

	ctx := NewContext(semantics.NewModel(resolver), resolver, 10000)
	got, err := NewEvalContext(ctx, pkg.Scope).Eval(decl.Value)
	if err != nil {
		t.Fatalf("2 * TrigFunctions::pi = error %v", err)
	}
	if got.Kind != ValConst || got.Const.Real != 2*math.Pi {
		t.Fatalf("2 * TrigFunctions::pi = %+v, want %v", got, 2*math.Pi)
	}
}

// libraryContextForSource indexes src as a library document, which is what the
// feature-value seam answers for.
func libraryContextForSource(t *testing.T, src string) (*Context, *symbols.Index) {
	t.Helper()
	file := parser.New(source.New("lib.kerml", []byte(src))).ParseFile()
	idx := symbols.NewIndex()
	idx.AddDocument("lib.kerml", file)
	idx.MarkLibrary("lib.kerml")
	resolver := resolve.New(idx)
	return NewContext(semantics.NewModel(resolver), resolver, 10000), idx
}

// The library declares `feature pi : Real` with no value, so its value comes from
// the seam.
func TestLibraryFeatureValue(t *testing.T) {
	ctx, idx := libraryContextForSource(t, `package TrigFunctions {
	feature pi : Real;
}`)
	sym := lookupOne(t, idx, "TrigFunctions::pi")

	got, ok, err := ctx.libraryFeatureValue(sym)
	if err != nil || !ok {
		t.Fatalf("libraryFeatureValue(TrigFunctions::pi) = %+v, %v, %v", got, ok, err)
	}
	if got.Kind != ValConst || got.Const.Real != math.Pi {
		t.Fatalf("TrigFunctions::pi = %+v, want %v", got, math.Pi)
	}
}

// A library symbol restored from the library index cache carries no declaration,
// and the seam answers for it with the same value: a feature's value does not
// depend on whether the cache was warm.
func TestLibraryFeatureValueFromACachedSymbol(t *testing.T) {
	idx := symbols.NewIndex()
	idx.AddRecords("lib", []symbols.RecordEntry{
		{FQN: "TrigFunctions", Kind: symbols.SymbolPackage},
		{FQN: "TrigFunctions::pi", Kind: symbols.SymbolAttributeUsage},
	})
	idx.MarkLibrary("lib")
	resolver := resolve.New(idx)
	ctx := NewContext(semantics.NewModel(resolver), resolver, 10000)

	got, ok, err := ctx.libraryFeatureValue(lookupOne(t, idx, "TrigFunctions::pi"))
	if err != nil || !ok || got.Const.Real != math.Pi {
		t.Fatalf("cached TrigFunctions::pi = %+v, %v, %v; want pi", got, ok, err)
	}
}

// The seam is keyed by the resolved symbol and answers for a library declaration
// only, so a model that declares its own feature of the same name keeps its own
// value — name resolution decides, as it does for every other name.
func TestLibraryFeatureValueLeavesAModelsOwnFeatureAlone(t *testing.T) {
	ctx, idx := contextForSource(t, `package TrigFunctions {
	feature pi : Real = 3.0;
}`)
	if _, ok, _ := ctx.libraryFeatureValue(lookupOne(t, idx, "TrigFunctions::pi")); ok {
		t.Fatalf("the seam supplied a value for a model's own feature")
	}
}

// A library feature this runtime has no representation for the value of reports
// itself, rather than a value of another shape.
func TestLibraryFeatureValueUnrepresentable(t *testing.T) {
	ctx, idx := libraryContextForSource(t, `package VectorFunctions {
	feature cartesianZeroVector : Real[3];
	feature cartesian3DZeroVector : Real[3];
}`)

	if _, _, err := ctx.libraryFeatureValue(lookupOne(t, idx, "VectorFunctions::cartesianZeroVector")); !errors.Is(err, ErrUnevaluableLibraryFunction) {
		t.Fatalf("cartesianZeroVector error = %v, want %v", err, ErrUnevaluableLibraryFunction)
	}

	// The three-dimensional zero vector does have a representation, and every
	// read builds its own sequence: no reader can change another's value.
	first, ok, err := ctx.libraryFeatureValue(lookupOne(t, idx, "VectorFunctions::cartesian3DZeroVector"))
	if err != nil || !ok {
		t.Fatalf("cartesian3DZeroVector = %+v, %v, %v", first, ok, err)
	}
	if elements := vectorValues(t, first); len(elements) != 3 || elements[0].Real != 0 {
		t.Fatalf("cartesian3DZeroVector = %v, want (0.0, 0.0, 0.0)", first)
	}
	second, _, _ := ctx.libraryFeatureValue(lookupOne(t, idx, "VectorFunctions::cartesian3DZeroVector"))
	if first.Sequence == second.Sequence {
		t.Fatalf("two reads of cartesian3DZeroVector share one sequence")
	}
}

// Every feature the seam supplies is named by its fully-qualified name, which is
// how a model that imports no part of the libraries reads one.
func TestLibraryFeatureByName(t *testing.T) {
	for _, fqn := range []string{"TrigFunctions::pi", "ComplexFunctions::i", "VectorFunctions::cartesian3DZeroVector"} {
		feature, ok := libraryFeatureByName(fqn)
		if !ok {
			t.Fatalf("no library feature %s registered", fqn)
		}
		if _, err := feature.value(libCtx(t)); err != nil {
			t.Fatalf("%s = error %v", fqn, err)
		}
	}
	if _, ok := libraryFeatureByName("TrigFunctions::tau"); ok {
		t.Fatalf("TrigFunctions::tau is not a library feature")
	}
}

// ComplexFunctions declares `feature i: Complex[1] = rect(0.0, 1.0)`, which the
// seam supplies as that pair.
func TestLibraryFeatureImaginaryUnit(t *testing.T) {
	feature, ok := libraryFeatureByName("ComplexFunctions::i")
	if !ok {
		t.Fatal("no library feature ComplexFunctions::i registered")
	}
	got, err := feature.value(libCtx(t))
	if err != nil {
		t.Fatalf("ComplexFunctions::i = error %v", err)
	}
	elements := vectorValues(t, got)
	if len(elements) != 2 || elements[0].Real != 0 || elements[1].Real != 1 {
		t.Fatalf("ComplexFunctions::i = %v, want 0.0 + 1.0i", got)
	}
}

// A library function is answered by its built-in even where the vendored
// declaration carries a body: the body is not there to evaluate when the library
// index cache is warm, so dispatch must not depend on it.
func TestLibraryFunctionAnswersALibraryDeclarationWithABody(t *testing.T) {
	ctx, idx := libraryContextForSource(t, `package TrigFunctions {
	feature pi : Real;
	function deg { in theta_rad : Real[1]; return : Real[1] = theta_rad * 180 / pi; }
}`)
	sym := lookupOne(t, idx, "TrigFunctions::deg")

	if _, ok := ctx.libraryFunctionFor(sym); !ok {
		t.Fatalf("the library's own deg did not dispatch to its built-in implementation")
	}
	got, err := ctx.InvokeCalc(sym, []Value{constReal(math.Pi)}, nil)
	if err != nil || got.Const.Real != 180 {
		t.Fatalf("InvokeCalc(TrigFunctions::deg, pi) = %+v, %v; want 180.0", got, err)
	}
}
