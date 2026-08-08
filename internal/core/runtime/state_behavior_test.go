package runtime

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/semantics"
)

func intValue(t *testing.T, data map[string]Value, name string) int64 {
	t.Helper()
	val, ok := data[name]
	if !ok {
		t.Fatalf("value %q missing from %v", name, data)
	}
	if val.Kind != ValConst || val.Const.Kind != semantics.ValInt {
		t.Fatalf("value %q = %v, want an integer", name, val)
	}
	return val.Const.Int
}

// Entry/do/exit behaviors written in a state body reach the executor only if
// lowering carries them onto the state node.
func TestStateBodyBehaviorsAreLowered(t *testing.T) {
	ctx, machine := loadState(t, `package test {
    state Machine {
        attribute entered : Integer = 0;
        attribute worked : Integer = 0;
        attribute exited : Integer = 0;

        initial init;
        state active {
            entry { entered = 1; }
            do { worked = 2; }
            exit { exited = 3; }
        }
        final done;

        init then active;
        active then done;
    }
}`, "Machine")

	data, _, err := ctx.ExecuteStateWithEvents(machine, nil)
	if err != nil {
		t.Fatalf("ExecuteStateWithEvents: %v", err)
	}
	for name, want := range map[string]int64{"entered": 1, "worked": 2, "exited": 3} {
		if got := intValue(t, data, name); got != want {
			t.Errorf("%s = %d, want %d", name, got, want)
		}
	}
}

func TestStateEntryPerformsAction(t *testing.T) {
	ctx, machine := loadState(t, `package test {
    action def Bump {
        inout counter : Integer;

        first start;
        action bumping {
            assign counter := counter + 10;
        }
        done end;

        then start bumping;
        then bumping end;
    }

    state Machine {
        attribute counter : Integer = 1;

        initial init;
        state active {
            entry perform action bump : Bump;
        }
        final done;

        init then active;
        active then done;
    }
}`, "Machine")

	data, _, err := ctx.ExecuteStateWithEvents(machine, nil)
	if err != nil {
		t.Fatalf("ExecuteStateWithEvents: %v", err)
	}
	if got := intValue(t, data, "counter"); got != 11 {
		t.Errorf("counter = %d, want 11 (entry action performed Bump)", got)
	}
}

func TestStateDoExitAndTransitionEffectPerformAction(t *testing.T) {
	ctx, machine := loadState(t, `package test {
    action def Bump {
        inout counter : Integer;

        first start;
        action bumping {
            assign counter := counter + 10;
        }
        done end;

        then start bumping;
        then bumping end;
    }

    state Machine {
        attribute counter : Integer = 1;

        initial init;
        state active {
            do perform action working : Bump;
            exit perform action bump : Bump;
        }
        final done;

        init then active;
        transition active to done do { perform action bumpAgain : Bump; }
    }
}`, "Machine")

	data, _, err := ctx.ExecuteStateWithEvents(machine, nil)
	if err != nil {
		t.Fatalf("ExecuteStateWithEvents: %v", err)
	}
	if got := intValue(t, data, "counter"); got != 31 {
		t.Errorf("counter = %d, want 31 (do, exit and effect each performed Bump)", got)
	}
}

// An entry/exit action named by reference (`entry Bump;`) is the same performed
// action usage as `entry perform Bump;`, so it invokes the referenced action.
func TestStateSubactionByReferencePerformsAction(t *testing.T) {
	ctx, machine := loadState(t, `package test {
    action def Bump {
        inout counter : Integer;

        first start;
        action bumping {
            assign counter := counter + 10;
        }
        done end;

        then start bumping;
        then bumping end;
    }

    state Machine {
        attribute counter : Integer = 1;

        initial init;
        state active {
            entry Bump;
            exit Bump;
        }
        final done;

        init then active;
        active then done;
    }
}`, "Machine")

	data, _, err := ctx.ExecuteStateWithEvents(machine, nil)
	if err != nil {
		t.Fatalf("ExecuteStateWithEvents: %v", err)
	}
	if got := intValue(t, data, "counter"); got != 21 {
		t.Errorf("counter = %d, want 21 (entry and exit each performed Bump)", got)
	}
}

func TestStateEntryPerformsUnresolvedAction(t *testing.T) {
	ctx, machine := loadState(t, `package test {
    state Machine {
        initial init;
        state active {
            entry perform action bump : NoSuchAction;
        }
        final done;

        init then active;
        active then done;
    }
}`, "Machine")

	_, _, err := ctx.ExecuteStateWithEvents(machine, nil)
	if err == nil || !strings.Contains(err.Error(), "unresolved action reference") {
		t.Fatalf("error = %v, want unresolved action reference", err)
	}
}
