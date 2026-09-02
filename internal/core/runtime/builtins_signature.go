package runtime

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

// builtinFunc is the implementation of a built-in: a function over runtime
// values that the evaluation context calls with its arguments in parameter order.
type builtinFunc = func(*EvalContext, []Value) (Value, error)

// builtinParam is one input parameter of a built-in, as the library declares it.
type builtinParam struct {
	name     string
	optional bool // its multiplicity admits no value, so an unbound argument is null
	deferred bool // declared `expr`: the argument binds unevaluated for the function to evaluate
}

func param(name string) builtinParam         { return builtinParam{name: name} }
func optionalParam(name string) builtinParam { return builtinParam{name: name, optional: true} }
func exprParam(name string) builtinParam {
	return builtinParam{name: name, optional: true, deferred: true}
}

// builtinSignatures lists each built-in's parameters in the library's declared
// order, which is what an argument written by name binds against. The
// dispatch gate (library_functions_test.go) holds them to the declarations.
var builtinSignatures = map[string][]builtinParam{
	"SequenceFunctions::#":            {optionalParam("seq"), param("index")},
	"SequenceFunctions::size":         {optionalParam("seq")},
	"SequenceFunctions::isEmpty":      {optionalParam("seq")},
	"SequenceFunctions::notEmpty":     {optionalParam("seq")},
	"SequenceFunctions::includes":     {optionalParam("seq1"), optionalParam("seq2")},
	"SequenceFunctions::includesOnly": {optionalParam("seq1"), optionalParam("seq2")},
	"SequenceFunctions::excludes":     {optionalParam("seq1"), optionalParam("seq2")},
	"SequenceFunctions::equals":       {optionalParam("x"), optionalParam("y")},
	"SequenceFunctions::same":         {optionalParam("x"), optionalParam("y")},
	"SequenceFunctions::union":        {optionalParam("seq1"), optionalParam("seq2")},
	"SequenceFunctions::intersection": {optionalParam("seq1"), optionalParam("seq2")},
	"SequenceFunctions::including":    {optionalParam("seq"), optionalParam("values")},
	"SequenceFunctions::includingAt":  {optionalParam("seq"), optionalParam("values"), param("index")},
	"SequenceFunctions::excluding":    {optionalParam("seq"), optionalParam("values")},
	"SequenceFunctions::subsequence":  {optionalParam("seq"), param("startIndex"), optionalParam("endIndex")},
	"SequenceFunctions::excludingAt":  {optionalParam("seq"), param("startIndex"), optionalParam("endIndex")},
	"SequenceFunctions::head":         {optionalParam("seq")},
	"SequenceFunctions::tail":         {optionalParam("seq")},
	"SequenceFunctions::last":         {optionalParam("seq")},

	"CollectionFunctions::#":           {param("col"), param("index")},
	"CollectionFunctions::==":          {optionalParam("col1"), optionalParam("col2")},
	"CollectionFunctions::size":        {param("col")},
	"CollectionFunctions::isEmpty":     {param("col")},
	"CollectionFunctions::notEmpty":    {param("col")},
	"CollectionFunctions::contains":    {param("col"), optionalParam("values")},
	"CollectionFunctions::containsAll": {param("col1"), param("col2")},
	"CollectionFunctions::head":        {param("col")},
	"CollectionFunctions::tail":        {param("col")},
	"CollectionFunctions::last":        {param("col")},

	"BaseFunctions::#": {optionalParam("seq"), param("index")},
	"BaseFunctions::,": {optionalParam("seq1"), optionalParam("seq2")},

	"DataFunctions::..":    {param("lower"), param("upper")},
	"ScalarFunctions::..":  {param("lower"), param("upper")},
	"IntegerFunctions::..": {param("lower"), param("upper")},

	"ControlFunctions::if":        {param("test"), exprParam("thenValue"), exprParam("elseValue")},
	"ControlFunctions::??":        {optionalParam("firstValue"), exprParam("secondValue")},
	"ControlFunctions::and":       {param("firstValue"), exprParam("secondValue")},
	"ControlFunctions::or":        {param("firstValue"), exprParam("secondValue")},
	"ControlFunctions::implies":   {param("firstValue"), exprParam("secondValue")},
	"ControlFunctions::select":    {optionalParam("collection"), optionalParam("selector")},
	"ControlFunctions::selectOne": {optionalParam("collection"), optionalParam("selector1")},
	"ControlFunctions::reject":    {optionalParam("collection"), optionalParam("rejector")},
	"ControlFunctions::collect":   {optionalParam("collection"), optionalParam("mapper")},
	"ControlFunctions::forAll":    {optionalParam("collection"), optionalParam("test")},
	"ControlFunctions::exists":    {optionalParam("collection"), optionalParam("test")},
	"ControlFunctions::allTrue":   {optionalParam("collection")},
	"ControlFunctions::anyTrue":   {optionalParam("collection")},
	"ControlFunctions::reduce":    {optionalParam("collection"), optionalParam("reducer")},
	"ControlFunctions::minimize":  {param("collection"), optionalParam("fn")},
	"ControlFunctions::maximize":  {param("collection"), optionalParam("fn")},

	"NumericalFunctions::sum":      {optionalParam("collection")},
	"NumericalFunctions::product":  {optionalParam("collection")},
	"NumericalFunctions::sum0":     {optionalParam("collection"), param("zero")},
	"NumericalFunctions::product1": {optionalParam("collection"), param("one")},
	"IntegerFunctions::sum":        {optionalParam("collection")},
	"IntegerFunctions::product":    {optionalParam("collection")},
	"RationalFunctions::sum":       {optionalParam("collection")},
	"RationalFunctions::product":   {optionalParam("collection")},
	"RealFunctions::sum":           {optionalParam("collection")},
	"RealFunctions::product":       {optionalParam("collection")},
}

// invokeBuiltin binds the arguments of a call to the built-in name to its
// declared parameters and applies fn to them.
func (ec *EvalContext) invokeBuiltin(name string, fn builtinFunc, exprs []ast.Node, named []ast.NamedArg) (Value, error) {
	args, err := ec.bindBuiltinArgs(name, exprs, named)
	if err != nil {
		return Value{}, err
	}
	return fn(ec, args)
}

// bindBuiltinArgs evaluates a call's arguments into parameter order. Positional
// arguments bind in sequence; named ones bind the parameter of their name, an
// unbound optional parameter ahead of them binding null. An `expr` parameter's
// argument is bound unevaluated either way.
func (ec *EvalContext) bindBuiltinArgs(name string, exprs []ast.Node, named []ast.NamedArg) ([]Value, error) {
	params := builtinSignatures[name]
	if len(named) == 0 {
		args := make([]Value, len(exprs))
		for i, arg := range exprs {
			val, err := ec.evalArgument(params, i, arg)
			if err != nil {
				return nil, err
			}
			args[i] = val
		}
		return args, nil
	}
	written := writtenName(name)
	if len(exprs) > 0 {
		return nil, fmt.Errorf("%w: function %s takes either positional or named arguments", ErrCalcArity, written)
	}
	args := make([]Value, len(params))
	bound := make([]bool, len(params))
	for _, arg := range named {
		if arg.Name == nil || len(arg.Name.Parts) == 0 {
			return nil, fmt.Errorf("unnamed argument in invocation of %s", written)
		}
		argName := arg.Name.Parts[len(arg.Name.Parts)-1].Text
		i := slices.IndexFunc(params, func(p builtinParam) bool { return p.name == argName })
		if i < 0 {
			return nil, fmt.Errorf("%w: function %s has no input parameter %q (expected %s)",
				ErrUnknownParameter, written, argName, builtinParameterList(params))
		}
		val, err := ec.evalArgument(params, i, arg.Value)
		if err != nil {
			return nil, err
		}
		args[i], bound[i] = val, true
	}
	last := len(params)
	for last > 0 && !bound[last-1] {
		last--
	}
	for i := 0; i < last; i++ {
		if bound[i] {
			continue
		}
		if !params[i].optional {
			return nil, fmt.Errorf("%w: function %s is called without an argument for parameter %q",
				ErrCalcArity, written, params[i].name)
		}
		args[i] = nullValue()
	}
	return args[:last], nil
}

// evalArgument evaluates the argument bound to parameter i, or keeps it as
// written for an `expr` parameter. A position past the declared parameters is
// evaluated for the built-in's own arity report.
func (ec *EvalContext) evalArgument(params []builtinParam, i int, arg ast.Node) (Value, error) {
	if i < len(params) && params[i].deferred {
		return NewExprValue(arg), nil
	}
	return ec.Eval(arg)
}

// builtinParameterList renders the declared parameter names for an error.
func builtinParameterList(params []builtinParam) string {
	names := make([]string, len(params))
	for i, p := range params {
		names[i] = p.name
	}
	return strings.Join(names, ", ")
}
