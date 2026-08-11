package runtime

import (
	"errors"
	"math"
	"testing"

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
