package runtime

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/resolve"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// loadAction parses src and returns a context plus the named action symbol.
func loadAction(t *testing.T, src, actionName string) (*Context, *symbols.Symbol) {
	t.Helper()
	path := "invoke.sysml"
	file := parser.New(source.New(path, []byte(src))).ParseFile()

	idx := symbols.NewIndex()
	idx.AddDocument(path, file)
	resolver := resolve.New(idx)
	ctx := NewContext(semantics.NewModel(resolver), resolver, 100000)

	sym := findSymbolOfKind(idx.DocumentRoot(path), actionName, symbols.SymbolActionDef, symbols.SymbolActionUsage)
	if sym == nil {
		t.Fatalf("action %q not found", actionName)
	}
	return ctx, sym
}

// findSymbolOfKind searches scope and its nested scopes for a symbol of one of
// the given symbol kinds.
func findSymbolOfKind(scope *symbols.Scope, name string, kinds ...symbols.SymbolKind) *symbols.Symbol {
	if scope == nil {
		return nil
	}
	for _, sym := range scope.Members() {
		if sym.Name == name {
			for _, kind := range kinds {
				if sym.Kind == kind {
					return sym
				}
			}
		}
		if found := findSymbolOfKind(sym.Scope, name, kinds...); found != nil {
			return found
		}
	}
	return nil
}

// loadState parses src and returns a context plus the named state machine symbol.
func loadState(t *testing.T, src, stateName string) (*Context, *symbols.Symbol) {
	t.Helper()
	path := "invoke_state.sysml"
	file := parser.New(source.New(path, []byte(src))).ParseFile()

	idx := symbols.NewIndex()
	idx.AddDocument(path, file)
	resolver := resolve.New(idx)
	ctx := NewContext(semantics.NewModel(resolver), resolver, 100000)

	sym := findSymbolOfKind(idx.DocumentRoot(path), stateName, symbols.SymbolStateDef, symbols.SymbolStateUsage)
	if sym == nil {
		t.Fatalf("state machine %q not found", stateName)
	}
	return ctx, sym
}

func intOutput(t *testing.T, outputs map[string]Value, name string) int64 {
	t.Helper()
	val, ok := outputs[name]
	if !ok {
		t.Fatalf("output %q missing from %v", name, outputs)
	}
	if val.Kind != ValConst || val.Const.Kind != semantics.ValInt {
		t.Fatalf("output %q = %v, want an integer", name, val)
	}
	return val.Const.Int
}

const doublerModel = `package test {
    action def Doubler {
        in v : Integer;
        out doubled : Integer;

        first start;
        action compute {
            assign doubled := v * 2;
        }
        done end;

        then start compute;
        then compute end;
    }
}`

func TestInvokeActionPassesParametersBothWays(t *testing.T) {
	ctx, outer := loadAction(t, doublerModel+`
package caller {
    action def Outer {
        attribute v : Integer = 21;
        attribute doubled : Integer = 0;

        first start;
        action call : test::Doubler;
        done end;

        then start call;
        then call end;
    }
}`, "Outer")

	outputs, err := ctx.ExecuteAction(outer)
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	if got := intOutput(t, outputs, "doubled"); got != 42 {
		t.Errorf("doubled = %d, want 42 (in parameter seeded, out parameter returned)", got)
	}
}

func TestInvokeActionLeavesNonParametersAlone(t *testing.T) {
	// `hidden` is a local attribute of the callee, not a parameter, so the
	// caller's value of the same name must survive the invocation.
	ctx, outer := loadAction(t, `package test {
    action def Callee {
        attribute hidden : Integer = 99;

        first start;
        done end;
        then start end;
    }

    action def Outer {
        attribute hidden : Integer = 1;

        first start;
        action call : Callee;
        done end;

        then start call;
        then call end;
    }
}`, "Outer")

	outputs, err := ctx.ExecuteAction(outer)
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	if got := intOutput(t, outputs, "hidden"); got != 1 {
		t.Errorf("hidden = %d, want 1 (callee locals must not leak into the caller)", got)
	}
}

func TestInvokeActionPerformForm(t *testing.T) {
	ctx, outer := loadAction(t, doublerModel+`
package caller {
    action def Outer {
        attribute v : Integer = 4;
        attribute doubled : Integer = 0;

        first start;
        perform action doubling : test::Doubler;
        done end;

        then start doubling;
        then doubling end;
    }
}`, "Outer")

	outputs, err := ctx.ExecuteAction(outer)
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	if got := intOutput(t, outputs, "doubled"); got != 8 {
		t.Errorf("doubled = %d, want 8", got)
	}
}

func TestInvokeActionBindsPositionalArguments(t *testing.T) {
	ctx, outer := loadAction(t, doublerModel+`
package caller {
    action def Outer {
        attribute v : Integer = 1;
        attribute doubled : Integer = 0;

        first start;
        action call = test::Doubler(10);
        done end;

        then start call;
        then call end;
    }
}`, "Outer")

	outputs, err := ctx.ExecuteAction(outer)
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	// The argument wins over the caller's own `v`, which would have given 2.
	if got := intOutput(t, outputs, "doubled"); got != 20 {
		t.Errorf("doubled = %d, want 20", got)
	}
}

func TestInvokeActionBindsNamedArguments(t *testing.T) {
	ctx, outer := loadAction(t, doublerModel+`
package caller {
    action def Outer {
        attribute doubled : Integer = 0;

        first start;
        action call = test::Doubler(v = 7);
        done end;

        then start call;
        then call end;
    }
}`, "Outer")

	outputs, err := ctx.ExecuteAction(outer)
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	if got := intOutput(t, outputs, "doubled"); got != 14 {
		t.Errorf("doubled = %d, want 14", got)
	}
}

func TestInvokeActionRejectsBadArguments(t *testing.T) {
	for name, src := range map[string]string{
		"too many arguments": `action call = test::Doubler(1, 2);`,
		"unknown parameter":  `action call = test::Doubler(nope = 1);`,
	} {
		ctx, outer := loadAction(t, doublerModel+`
package caller {
    action def Outer {
        first start;
        `+src+`
        done end;

        then start call;
        then call end;
    }
}`, "Outer")

		if _, err := ctx.ExecuteAction(outer); err == nil {
			t.Errorf("%s: ExecuteAction succeeded, want error", name)
		}
	}
}

func TestInvokeActionUnresolved(t *testing.T) {
	ctx, outer := loadAction(t, `package test {
    action def Outer {
        first start;
        action call : NoSuchAction;
        done end;

        then start call;
        then call end;
    }
}`, "Outer")

	_, err := ctx.ExecuteAction(outer)
	if err == nil || !strings.Contains(err.Error(), "unresolved action reference") {
		t.Fatalf("error = %v, want unresolved action reference", err)
	}
}

func TestInvokeActionNotAnAction(t *testing.T) {
	ctx, outer := loadAction(t, `package test {
    part def Wheel;

    action def Outer {
        first start;
        action call : Wheel;
        done end;

        then start call;
        then call end;
    }
}`, "Outer")

	_, err := ctx.ExecuteAction(outer)
	if err == nil || !strings.Contains(err.Error(), "is not an action") {
		t.Fatalf("error = %v, want 'is not an action'", err)
	}
}

func TestInvokeActionRecursionIsBounded(t *testing.T) {
	// Self-invocation would recurse until the stack ran out, since each nested
	// invocation runs on a fresh executor with its own step budget.
	ctx, outer := loadAction(t, `package test {
    action def Outer {
        first start;
        action call : Outer;
        done end;

        then start call;
        then call end;
    }
}`, "Outer")

	_, err := ctx.ExecuteAction(outer)
	if err == nil || !strings.Contains(err.Error(), "nested more than") {
		t.Fatalf("error = %v, want a nesting-depth error", err)
	}
}

// A bare perform statement is named after the action it performs
// (symbols.effectiveIdent), so its target must be resolved outside that
// binding — otherwise it performs itself, which does nothing.
func TestPerformShorthandRunsTheReferencedAction(t *testing.T) {
	ctx, outer := loadAction(t, `package test {
	action increment {
		in base : Integer;
		out result : Integer;

		first begin;
		action bump {
			assign result := base + 5;
		}
		done finish;

		then begin bump;
		then bump finish;
	}

	action outer {
		attribute base : Integer = 7;
		attribute result : Integer = 0;

		first start;

		perform increment;

		done end;

		then start increment;
		then increment end;
	}
}`, "outer")

	outputs, err := ctx.ExecuteAction(outer)
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	if got := intOutput(t, outputs, "result"); got != 12 {
		t.Errorf("result = %d, want 12 (the performed action ran)", got)
	}
}
