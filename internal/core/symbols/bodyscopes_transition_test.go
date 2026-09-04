package symbols

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

// A body expression written in a triggered transition's trailing body declares
// its parameters beneath the trigger's scope, where the body's members were
// built, so the resolver finds both the body's and the accept's parameter.
func TestTransitionBodyExpressionScopesNestInTriggerScope(t *testing.T) {
	root := build(t, `package P {
	private import ScalarValues::*;
	private import ControlFunctions::*;
	item def Request { attribute ids : Integer[*]; }
	state def Server {
		state idle; state busy;
		transition t first idle accept origin : Request then busy {
			attribute positive = origin.ids->forAll { in i : Integer; i > 0 };
		}
	}
}`)
	bodies := findBodyScopes(root)
	if len(bodies) != 1 {
		t.Fatalf("body-expression scopes = %d, want 1", len(bodies))
	}
	p, _ := root.LookupLocal("P")
	server, _ := p.Scope.LookupLocal("Server")
	trans, ok := server.Scope.LookupLocal("t")
	if !ok {
		t.Fatal("transition t not defined")
	}
	trigger := TriggerScope(server.Scope, trans.Decl.(*ast.TransitionMember))
	if trigger == trans.Scope {
		t.Fatal("the accept's parameter scope was not built")
	}
	if bodies[0].Parent() != trigger {
		t.Errorf("body scope parent = %v, want the trigger's parameter scope", bodies[0].Parent().Node())
	}
	if _, ok := bodies[0].LookupLocal("i"); !ok {
		t.Error("parameter i not defined in the body scope")
	}
	if _, ok := trigger.LookupLocal("origin"); !ok {
		t.Error("the accept's parameter is not declared in the trigger scope")
	}
}
