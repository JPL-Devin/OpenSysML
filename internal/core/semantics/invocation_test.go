package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// untyped is an ArgumentTyper that compares by value, as the checker's does.
type untyped struct{ tag int }

func (untyped) InvocationArguments(_ *symbols.Scope, e *ast.InvocationExpr) []Argument {
	return untypedArguments(e)
}

// Installing the typing already in place keeps the selections made under it;
// a different typing drops them.
func TestSetArgumentTyperKeepsSelectionsUnderTheSameTyping(t *testing.T) {
	m := NewModel(nil)
	m.SetArgumentTyper(untyped{1})
	m.invocations[invocationKey{}] = &InvocationSelection{}
	m.SetArgumentTyper(untyped{1})
	if m.MemoSize() != 1 {
		t.Fatal("reinstalling the same typing dropped the selections")
	}
	m.SetArgumentTyper(untyped{2})
	if m.MemoSize() != 0 {
		t.Fatal("a different typing kept the selections made under the old one")
	}
}
