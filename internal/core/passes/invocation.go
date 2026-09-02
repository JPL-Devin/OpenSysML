package passes

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// SelectInvocation is the declaration e calls in scope, chosen from every
// declaration its name is visible as by the static types of its arguments. The
// runtime dispatches on the same selection, so what the checker accepts is what runs.
func SelectInvocation(resolver *resolve.Resolver, model *semantics.Model, scope *symbols.Scope, e *ast.InvocationExpr) *semantics.InvocationSelection {
	silent := exprChecker{resolver: resolver, model: model}
	return silent.selectInvocation(scope, e, silent.argumentTypes(scope, e))
}

// argumentTypes types e's arguments, the receiver of `x->f(a)` first.
func (ec *exprChecker) argumentTypes(scope *symbols.Scope, e *ast.InvocationExpr) []semantics.PrimType {
	args := invocationArgs(e)
	types := make([]semantics.PrimType, len(args))
	for i, arg := range args {
		types[i] = ec.infer(scope, arg)
	}
	return types
}

// invocationArgs returns e's positional arguments, the receiver first.
func invocationArgs(e *ast.InvocationExpr) []ast.Node {
	if e.Operand == nil {
		return e.Args
	}
	return append([]ast.Node{e.Operand}, e.Args...)
}

// selectInvocation records the declaration e calls, from argTypes, the types of
// its positional arguments; the named arguments are typed here.
func (ec *exprChecker) selectInvocation(scope *symbols.Scope, e *ast.InvocationExpr, argTypes []semantics.PrimType) *semantics.InvocationSelection {
	args := invocationArgs(e)
	typed := make([]semantics.Argument, 0, len(args)+len(e.NamedArgs))
	for i, arg := range args {
		typed = append(typed, ec.argument(scope, arg, argTypes[i], ""))
	}
	for _, arg := range e.NamedArgs {
		got := ec.infer(scope, arg.Value)
		if arg.Name == nil || len(arg.Name.Parts) != 1 {
			continue
		}
		typed = append(typed, ec.argument(scope, arg.Value, got, arg.Name.Parts[0].Text))
	}
	return ec.model.SelectInvocation(scope, e, typed)
}

// argument describes value as an argument of scalar type prim: the declared type
// of the feature it names, or of the result of the call it makes, is carried too.
func (ec *exprChecker) argument(scope *symbols.Scope, value ast.Node, prim semantics.PrimType, name string) semantics.Argument {
	declared := ec.valueTypeSymbol(scope, value)
	if declared == nil {
		declared = ec.invocationResultTypeSymbol(scope, value)
	}
	return semantics.Argument{Prim: prim, Type: declared, Exact: spellsOneValue(value), Name: name}
}

// candidateNames lists declarations for a diagnostic by qualified name, in order.
func candidateNames(syms []*symbols.Symbol) string {
	names := make([]string, 0, len(syms))
	for _, sym := range syms {
		names = append(names, symbols.FQNOf(sym))
	}
	return strings.Join(names, ", ")
}
