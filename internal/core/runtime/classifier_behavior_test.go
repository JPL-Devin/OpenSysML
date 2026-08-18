package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
)

// intArgument is an integer argument of an invocation.
func intArgument(n int64) Value {
	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: n}}
}

// An object of a type exhibiting a machine runs that machine on materialization.
func TestInstantiateStartsExhibitedStateMachine(t *testing.T) {
	src := `
		part def Controller {
			attribute log: String;
			exhibit state modes {
				entry; then off;
				state off {
					entry action mark { assign log := "off"; }
				}
			}
		}
	`
	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(model, resolver, 10000)

	inst, err := ctx.Instantiate(resolveSymbol(t, root, "Controller"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}

	behavior, ok := inst.ExhibitedState()
	if !ok {
		t.Fatalf("object exhibits no state machine, behaviors: %v", inst.Behaviors())
	}
	if behavior.Name != "modes" {
		t.Errorf("behavior name = %q, want modes", behavior.Name)
	}
	if got := activeStateNames(behavior.State); got != "off" {
		t.Errorf("current state = %q, want off", got)
	}
}

// Two objects of one type exhibit two machines, with their own current states.
func TestExhibitedMachinesOfTwoObjectsAreIndependent(t *testing.T) {
	src := `
		part def Light {
			exhibit state modes {
				entry; then dark;
				state dark;
				state lit;
				transition dark_to_lit first dark accept Toggle then lit;
			}
		}
		attribute def Toggle;
	`
	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(model, resolver, 10000)
	sym := resolveSymbol(t, root, "Light")

	first, err := ctx.Instantiate(sym)
	if err != nil {
		t.Fatalf("Instantiate first: %v", err)
	}
	second, err := ctx.Instantiate(sym)
	if err != nil {
		t.Fatalf("Instantiate second: %v", err)
	}
	if first.ID == second.ID {
		t.Fatalf("two objects share identity %d", first.ID)
	}

	firstMachine, _ := first.ExhibitedState()
	secondMachine, _ := second.ExhibitedState()
	if firstMachine.State == secondMachine.State {
		t.Fatal("two objects share one machine execution")
	}

	firstMachine.State.SendSignal("Toggle", nil)
	if err := firstMachine.State.RunToCompletion(); err != nil {
		t.Fatalf("run first machine: %v", err)
	}
	if got := activeStateNames(firstMachine.State); got != "lit" {
		t.Errorf("first current state = %q, want lit", got)
	}
	if got := activeStateNames(secondMachine.State); got != "dark" {
		t.Errorf("second current state = %q, want dark", got)
	}
}

// A machine's entry action writes the feature values of the object exhibiting it.
func TestExhibitedMachineWritesItsObjectsFeatureValues(t *testing.T) {
	src := `
		part def Counter {
			attribute count: Integer = 0;
			exhibit state modes {
				entry; then running;
				state running {
					entry action bump { assign count := count + 1; }
				}
			}
		}
	`
	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(model, resolver, 10000)

	inst, err := ctx.Instantiate(resolveSymbol(t, root, "Counter"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	fv, err := inst.GetFeatureValue(ctx, "count")
	if err != nil {
		t.Fatalf("GetFeatureValue: %v", err)
	}
	if got := fv.HeldValue(); got.Const.Int != 1 {
		t.Errorf("count = %v, want 1", got.Const)
	}
}

// invokeFixture owns an operation, a machine, a calc and an attribute, so one
// object exercises both the executable and the rejected invocation paths.
const invokeFixture = `
	part def Tank {
		attribute level: Integer = 2;
		action fillBy { in n; out filled; first apply; action apply { assign level := level + n; assign filled := level; } }
		exhibit state modes { entry; then holding; state holding; }
		calc capacity { return : Integer = 10; }
	}
`

// An operation of the object's type runs with that object as performer: it reads
// and writes that object's feature values and answers its declared outputs.
func TestInvokeOperationPerformedByTheObject(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, invokeFixture)
	ctx := NewContext(model, resolver, 10000)
	inst, err := ctx.Instantiate(resolveSymbol(t, root, "Tank"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}

	results, err := ctx.InvokeOperation(inst, "fillBy", map[string]Value{"n": intArgument(3)})
	if err != nil {
		t.Fatalf("InvokeOperation: %v", err)
	}
	if got, ok := results["filled"]; !ok || got.Const.Int != 5 {
		t.Errorf("filled = %v, want 5", results)
	}
	fv, err := inst.GetFeatureValue(ctx, "level")
	if err != nil {
		t.Fatalf("GetFeatureValue: %v", err)
	}
	if got := fv.HeldValue(); got.Const.Int != 5 {
		t.Errorf("level = %v, want 5", got.Const)
	}
}

// Every path %invoke cannot run reports a typed error naming what it was asked.
func TestInvokeOperationFailureModes(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, invokeFixture)
	ctx := NewContext(model, resolver, 10000)
	inst, err := ctx.Instantiate(resolveSymbol(t, root, "Tank"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}

	cases := []struct {
		name      string
		object    *Instance
		operation string
		args      map[string]Value
		want      error
	}{
		{"no object", nil, "fillBy", nil, ErrNoSuchBehavior},
		{"unknown operation", inst, "drain", nil, ErrNoSuchBehavior},
		{"state machine", inst, "modes", nil, ErrUnsupportedClassifierBehavior},
		{"calc", inst, "capacity", nil, ErrUnsupportedClassifierBehavior},
		{"attribute", inst, "level", nil, ErrNotABehavior},
		{"unbound parameter", inst, "fillBy", nil, ErrUnboundParameter},
		{"argument naming no parameter", inst, "fillBy", map[string]Value{"n": intArgument(1), "m": intArgument(2)}, ErrUnboundParameter},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ctx.InvokeOperation(tc.object, tc.operation, tc.args)
			if !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want %v", err, tc.want)
			}
		})
	}
}

// An action stating no flow performs no step, so an object of a type performing
// one is still created, with that behavior of its own and nothing to run.
func TestPerformedActionWithoutAFlowStillMaterializes(t *testing.T) {
	src := `
		action def Report;
		part def Camera {
			perform action report : Report;
		}
	`
	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(model, resolver, 10000)

	inst, err := ctx.Instantiate(resolveSymbol(t, root, "Camera"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	behavior, ok := inst.Behavior("report")
	if !ok {
		t.Fatalf("object performs no report action, behaviors: %v", inst.Behaviors())
	}
	if behavior.Action == nil {
		t.Error("performed action has no execution")
	}
}

// A single value written to a many-valued feature is that collection's one
// element, the shape materialization gives such a feature.
func TestWritingOneValueToAManyValuedFeatureHoldsACollection(t *testing.T) {
	src := `
		part def Log {
			attribute entries: String[*];
			exhibit state modes {
				entry; then open;
				state open {
					entry action note { assign entries := "first"; }
				}
			}
		}
	`
	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(model, resolver, 10000)

	inst, err := ctx.Instantiate(resolveSymbol(t, root, "Log"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	fv, err := inst.GetFeatureValue(ctx, "entries")
	if err != nil {
		t.Fatalf("GetFeatureValue: %v", err)
	}
	held := fv.HeldValue()
	if held.Kind != ValSequence && held.Kind != ValSet {
		t.Fatalf("entries holds %v, want a collection", held.Kind)
	}
	if got := elementsOf(held); len(got) != 1 || got[0].Str != "first" {
		t.Errorf("entries = %v, want one element \"first\"", got)
	}
}

// A type that exhibits a machine no element states is reported, not ignored.
func TestExhibitedMachineNamingNoBodyIsReported(t *testing.T) {
	src := `
		part def Controller {
			exhibit state modes;
		}
	`
	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(model, resolver, 10000)

	_, err := ctx.Instantiate(resolveSymbol(t, root, "Controller"))
	if err == nil {
		t.Fatal("expected an error for a machine naming no body")
	}
	if !errors.Is(err, ErrUnresolvedClassifierBehavior) {
		t.Fatalf("error = %v, want ErrUnresolvedClassifierBehavior", err)
	}
	if !strings.Contains(err.Error(), "modes") {
		t.Errorf("error %q does not name the behavior", err)
	}
}
