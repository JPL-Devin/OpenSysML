package passes

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// SelectInvocation is the declaration e calls in scope, chosen among every visible
// declaration of its name the call site can run by its arguments' static types; the
// runtime dispatches on it too.
func SelectInvocation(resolver *resolve.Resolver, model *semantics.Model, scope *symbols.Scope, e *ast.InvocationExpr, performs semantics.Performs) *semantics.InvocationSelection {
	silent := exprChecker{resolver: resolver, model: model}
	return silent.selectInvocation(scope, e, silent.argumentTypes(scope, e), performs)
}

// argumentTyper is the checker's argument typing as the semantic model consumes
// it, so a call it reads on its own selects what the checker selects.
type argumentTyper struct {
	resolver *resolve.Resolver
	model    *semantics.Model
}

func (t argumentTyper) InvocationArguments(scope *symbols.Scope, e *ast.InvocationExpr) []semantics.Argument {
	silent := exprChecker{resolver: t.resolver, model: t.model}
	return silent.arguments(scope, e, silent.argumentTypes(scope, e))
}

// argumentTypes are the types of e's arguments: the positional ones, the
// receiver of `x->f(a)` first, and the named ones in the order written.
type argumentTypes struct {
	positional []semantics.PrimType
	named      []semantics.PrimType
}

// argumentTypes types e's arguments once, so nested errors report once.
func (ec *exprChecker) argumentTypes(scope *symbols.Scope, e *ast.InvocationExpr) argumentTypes {
	args := invocationArgs(e)
	types := argumentTypes{
		positional: make([]semantics.PrimType, len(args)),
		named:      make([]semantics.PrimType, len(e.NamedArgs)),
	}
	for i, arg := range args {
		types.positional[i] = ec.argumentType(scope, arg)
	}
	for i, arg := range e.NamedArgs {
		types.named[i] = ec.argumentType(scope, arg.Value)
	}
	return types
}

// argumentType is the scalar type an argument binds as: a collection literal
// binds its elements, so it is typed by the type they have in common.
func (ec *exprChecker) argumentType(scope *symbols.Scope, arg ast.Node) semantics.PrimType {
	if seq, ok := arg.(*ast.SequenceExpr); ok {
		return ec.commonElementType(scope, seq)
	}
	return ec.infer(scope, arg)
}

// invocationArgs returns e's positional arguments, the receiver first.
func invocationArgs(e *ast.InvocationExpr) []ast.Node {
	if e.Operand == nil {
		return e.Args
	}
	return append([]ast.Node{e.Operand}, e.Args...)
}

// selectInvocation records the declaration e calls given the types of its arguments.
func (ec *exprChecker) selectInvocation(scope *symbols.Scope, e *ast.InvocationExpr, argTypes argumentTypes, performs semantics.Performs) *semantics.InvocationSelection {
	return ec.model.SelectInvocation(scope, e, ec.arguments(scope, e, argTypes), performs)
}

// performs is the kind of call site e is: an action performance when it is the
// value of an action usage the checker is reading, else an expression.
func (ec *exprChecker) performs(e *ast.InvocationExpr) semantics.Performs {
	if ec.performed[e] {
		return semantics.PerformsAction
	}
	return semantics.PerformsBehavior
}

// arguments describes e's arguments for selection: the positional ones, then the named.
func (ec *exprChecker) arguments(scope *symbols.Scope, e *ast.InvocationExpr, argTypes argumentTypes) []semantics.Argument {
	args := invocationArgs(e)
	typed := make([]semantics.Argument, 0, len(args)+len(e.NamedArgs))
	for i, arg := range args {
		typed = append(typed, ec.argument(scope, arg, argTypes.positional[i], ""))
	}
	for i, arg := range e.NamedArgs {
		if name, ok := namedArgumentName(arg); ok {
			typed = append(typed, ec.argument(scope, arg.Value, argTypes.named[i], name))
		}
	}
	return typed
}

// namedArgumentName is the parameter name a named argument binds to.
func namedArgumentName(arg ast.NamedArg) (string, bool) {
	if arg.Name == nil || len(arg.Name.Parts) != 1 {
		return "", false
	}
	return arg.Name.Parts[0].Text, true
}

// argument describes value as an argument of scalar type prim: the declared type
// of the feature it names, or of the result of the call it makes, is carried too.
func (ec *exprChecker) argument(scope *symbols.Scope, value ast.Node, prim semantics.PrimType, name string) semantics.Argument {
	if seq, ok := value.(*ast.SequenceExpr); ok {
		return semantics.Argument{
			Prim:  prim,
			Type:  ec.commonElementTypeSymbol(scope, seq),
			Exact: len(seq.Elements) > 0 && allSpellOneValue(seq.Elements),
			Name:  name,
		}
	}
	return semantics.Argument{Prim: prim, Type: ec.declaredValueType(scope, value), Exact: spellsOneValue(value), Name: name}
}

// declaredValueType is the declared type of the feature value names, or of the
// result of the call it makes; nil when neither.
func (ec *exprChecker) declaredValueType(scope *symbols.Scope, value ast.Node) *symbols.Symbol {
	if declared := ec.valueTypeSymbol(scope, value); declared != nil {
		return declared
	}
	return ec.invocationResultTypeSymbol(scope, value)
}

// commonElementTypeSymbol is the declared type every element of seq conforms to,
// nil when one has none or they share none.
func (ec *exprChecker) commonElementTypeSymbol(scope *symbols.Scope, seq *ast.SequenceExpr) *symbols.Symbol {
	var common *symbols.Symbol
	for _, el := range seq.Elements {
		elem := ec.declaredValueType(scope, el)
		switch {
		case elem == nil:
			return nil
		case common == nil, ec.model.Conforms(common, elem):
			common = elem
		case !ec.model.Conforms(elem, common):
			return nil
		}
	}
	return common
}

func allSpellOneValue(elements []ast.Node) bool {
	for _, el := range elements {
		if !spellsOneValue(el) {
			return false
		}
	}
	return true
}

// candidateNames lists declarations for a diagnostic by qualified name, in order.
func candidateNames(syms []*symbols.Symbol) string {
	names := make([]string, 0, len(syms))
	for _, sym := range syms {
		names = append(names, symbols.FQNOf(sym))
	}
	return strings.Join(names, ", ")
}
