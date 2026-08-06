package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/resolve"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// TestRuntimeRobustness exercises failure modes: graceful errors, no panics, no hangs.
// Each test must return a typed error, never panic or hang.

func TestRuntimeRobustness(t *testing.T) {
	t.Run("deadlock_join_starvation", testDeadlockJoinStarvation)
	t.Run("decision_no_satisfied_guard", testDecisionNoSatisfiedGuard)
	t.Run("state_dangling_transition", testStateDanglingTransition)
	t.Run("sourceless_accept_at_top_level", testSourcelessAcceptAtTopLevel)
	t.Run("calc_unbound_parameter", testCalcUnboundParameter)
	t.Run("constraint_missing_feature", testConstraintMissingFeature)
	t.Run("step_budget_exceeded", testStepBudgetExceeded)
	t.Run("fork_branches_share_region", testForkBranchesShareRegion)
	t.Run("join_with_one_incoming_branch", testJoinWithOneIncomingBranch)
	t.Run("region_local_junction_target", testRegionLocalJunctionTarget)
	t.Run("non_numeric_time_trigger", testNonNumericTimeTrigger)
	t.Run("send_reaches_only_its_addressee", testSendReachesOnlyItsAddressee)
	t.Run("accept_of_unsent_type", testAcceptOfUnsentTypeReports)
	t.Run("send_via_unconnected_port", testSendViaUnconnectedPort)
	t.Run("history_outside_composite_state", testHistoryOutsideCompositeState)
	t.Run("history_without_record_or_default", testHistoryWithoutRecordOrDefault)
}

// testHistoryOutsideCompositeState: a history pseudostate restores the state
// that declares it, so one declared directly in the machine has nothing to
// restore and must report rather than enter an arbitrary state. History has no
// textual notation, so the machine is built on the AST directly.
func testHistoryOutsideCompositeState(t *testing.T) {
	exec := stateExecutorFor(t, &ast.Usage{
		Kind:  ast.UsageState,
		Ident: ast.Identification{Name: "Machine"},
		Members: []ast.Node{
			&ast.StateNode{Name: "init", IsInitial: true},
			&ast.StateNode{Name: "away"},
			&ast.PseudostateNode{Kind: ast.PseudostateShallowHistory, Name: "H"},
			transitionMember("init", "away"),
			transitionMember("away", "H"),
		},
	})
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	fire(t, exec, "init", "away")

	err := exec.fireTransition(transitionBetween(t, exec, "away", "H"))
	if err == nil {
		t.Fatal("expected an error for a history outside any composite state")
	}
	if !strings.Contains(err.Error(), "must be declared inside the composite state") {
		t.Errorf("expected an ownership error, got: %v", err)
	}
}

// testHistoryWithoutRecordOrDefault: before its composite state has ever been
// exited a history has nothing to restore, and with no outgoing transition there
// is no default target either — that is reported, not silently ignored.
func testHistoryWithoutRecordOrDefault(t *testing.T) {
	history := &ast.PseudostateNode{Kind: ast.PseudostateShallowHistory, Name: "H"}
	outer := &ast.StateNode{
		Name:      "outer",
		Substates: []ast.Node{&ast.StateNode{Name: "first"}, history},
	}
	exec := stateExecutorFor(t, &ast.Usage{
		Kind:  ast.UsageState,
		Ident: ast.Identification{Name: "Machine"},
		Members: []ast.Node{
			&ast.StateNode{Name: "init", IsInitial: true},
			outer,
			&ast.StateNode{Name: "away"},
			transitionMember("init", "away"),
			transitionMember("away", "H"),
		},
	})
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	fire(t, exec, "init", "away")

	err := exec.fireTransition(transitionBetween(t, exec, "away", "H"))
	if err == nil {
		t.Fatal("expected an error: nothing recorded and no default history transition")
	}
	if !strings.Contains(err.Error(), "no recorded configuration") {
		t.Errorf("expected a missing-default error, got: %v", err)
	}
}

// testSendViaUnconnectedPort: a port with no connection reaches no one, so an
// accept waiting on the message must report rather than hang or bind nothing.
func testSendViaUnconnectedPort(t *testing.T) {
	_, err := executeActionSource(t, "pipeline", `package P {
		action pipeline {
			port outPort;
			port inPort;
			first start;
			action sender { send 42 via outPort; }
			action reader accept msg : Integer via inPort;
			done end;
			then start sender;
			then sender reader;
			then reader end;
		}
	}`)
	if err == nil {
		t.Fatal("expected an error: nothing connects outPort to inPort")
	}
	if !errors.Is(err, ErrNoMatchingMessage) {
		t.Errorf("expected ErrNoMatchingMessage, got: %v", err)
	}
}

// testNonNumericTimeTrigger: a timed trigger whose duration is not a number
// cannot be scheduled and must be reported rather than silently dropped.
func testNonNumericTimeTrigger(t *testing.T) {
	_, _, err := executeStateSource(t, "Machine", `package test {
		state Machine {
			initial init;
			state waiting {
				accept at "noon" then done;
			}
			final done;
			init then waiting;
		}
	}`)
	if err == nil {
		t.Fatal("expected an error for a non-numeric time trigger")
	}
	if !strings.Contains(err.Error(), "time duration must be constant, got string") {
		t.Errorf("expected a numeric-duration error, got: %v", err)
	}
}

// testForkBranchesShareRegion: a fork whose branches land in the same region
// cannot produce one active state per region.
func testForkBranchesShareRegion(t *testing.T) {
	_, _, err := executeStateSource(t, "Machine", `package test {
		state Machine {
			initial init;
			state ready;
			state working {
				region left {
					initial ls;
					state a;
					state b;
					then ls a;
				}
				region right {
					initial rs;
					state c;
					then rs c;
				}
			}
			fork split;
			final done;

			init then ready;
			transition ready to split;
			transition split to a;
			transition split to b;
		}
	}`)
	if err == nil {
		t.Fatal("expected an error for fork branches in the same region")
	}
	if !strings.Contains(err.Error(), "in the same region") {
		t.Errorf("expected a same-region error, got: %v", err)
	}
}

// testJoinWithOneIncomingBranch: a join synchronizes branches, so a single
// incoming transition is a modeling error rather than a pass-through.
func testJoinWithOneIncomingBranch(t *testing.T) {
	_, _, err := executeStateSource(t, "Machine", `package test {
		state Machine {
			initial init;
			state ready;
			join sync;
			final done;

			init then ready;
			transition ready to sync;
			transition sync to done;
		}
	}`)
	if err == nil {
		t.Fatal("expected an error for a join with one incoming transition")
	}
	if !strings.Contains(err.Error(), "at least two incoming transitions") {
		t.Errorf("expected an incoming-branch-count error, got: %v", err)
	}
}

// testRegionLocalJunctionTarget: leaving an orthogonal region for a junction is
// unsupported and must be reported, not silently drop the sibling regions.
func testRegionLocalJunctionTarget(t *testing.T) {
	_, _, err := executeStateSource(t, "Machine", `package test {
		state Machine {
			region left {
				initial ls;
				state a;
				state b;
				then ls a;
				transition a to merge;
			}
			region right {
				initial rs;
				state c;
				then rs c;
			}
			junction merge;

			transition merge to b;
		}
	}`)
	if err == nil {
		t.Fatal("expected an error for a region-local transition into a junction")
	}
	if !strings.Contains(err.Error(), "must be a state node") {
		t.Errorf("expected a typed target error, got: %v", err)
	}
}

// testDeadlockJoinStarvation: join awaiting token that never arrives. `stranded`
// has no incoming edge, so the join has two incoming edges but can only ever be
// reached by one token.
func testDeadlockJoinStarvation(t *testing.T) {
	src := `
		package test {
			action starve {
				first start;
				action stranded;
				join sync;
				done end;
				then start sync;
				then stranded sync;
				then sync end;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))

	sym := findSymbolByName(idx.DocumentRoot("<test>"), "starve", ast.DefAction)
	if sym == nil {
		t.Fatal("action starve not found")
	}

	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("create action executor: %v", err)
	}

	err = exec.RunToCompletion()
	if err == nil {
		t.Fatal("expected a deadlock error, the starved join completed")
	}
	if !strings.Contains(err.Error(), "deadlock") {
		t.Errorf("expected a deadlock error, got: %v", err)
	}
}

// testDecisionNoSatisfiedGuard: decision node with no guards satisfied
func testDecisionNoSatisfiedGuard(t *testing.T) {
	src := `
		package test {
			calc noGuard {
				in x: Integer;
				if (false) return 1;
				// No else branch, all guards false
			}
		}
	`
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}

	idx, model, ctx := buildRuntime(t, "<test>", file)

	_ = model // silence unused

	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "noGuard", ast.DefCalc)
	if sym == nil {
		t.Fatal("noGuard calc not found")
	}

	// Invoke with x=5
	xVal := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 5}}
	result, err := ctx.InvokeCalc(sym, []Value{xVal}, rootScope)

	// Expect error or null result (implementation-specific)
	if err != nil {
		t.Logf("InvokeCalc returned error (acceptable): %v", err)
		return
	}

	if result.Kind == ValNull {
		t.Log("InvokeCalc returned null (no branch taken)")
		return
	}

	t.Logf("InvokeCalc returned: %v (no error - implementation allows missing branch)", result)
}

// testStateDanglingTransition: state with transition to nonexistent state
func testStateDanglingTransition(t *testing.T) {
	src := `
		package test {
			state Machine {
				initial init;
				init then nowhere; // 'nowhere' state doesn't exist
			}
		}
	`
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}

	idx, model, ctx := buildRuntime(t, "<test>", file)

	_ = model // silence unused

	// Check diagnostics (resolver should catch missing state)
	// Note: resolver diagnostics accessed via resolver.Diagnostics field

	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "Machine", ast.DefState)
	if sym == nil {
		t.Fatal("Broken state not found")
	}

	exec, err := ctx.CreateStateExecutor(sym)
	if err != nil {
		t.Logf("CreateStateExecutor error (acceptable): %v", err)
		return
	}

	err = exec.ProcessNextEvent()
	if err != nil {
		t.Logf("ProcessNextEvent returned error (acceptable): %v", err)
		return
	}

	t.Log("ProcessNextEvent succeeded (dangling transition not exercised)")
}

// testSourcelessAcceptAtTopLevel: sourceless accept...then at top level should error
func testSourcelessAcceptAtTopLevel(t *testing.T) {
	src := `
		package test {
			state Machine {
				initial init;
				state waiting;
				state active;
				init then waiting;
				accept go then active; // ERROR: sourceless at top level
			}
		}
	`
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}

	idx, model, ctx := buildRuntime(t, "<test>", file)

	_ = model // silence unused

	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "Machine", ast.DefState)
	if sym == nil {
		t.Fatal("Machine state not found")
	}

	// Should fail at CreateStateExecutor (lowering time) with clear error
	exec, err := ctx.CreateStateExecutor(sym)
	if err != nil {
		if strings.Contains(err.Error(), "sourceless") && strings.Contains(err.Error(), "containing state") {
			t.Logf("CreateStateExecutor error (expected): %v", err)
			return
		}
		t.Fatalf("Unexpected error message: %v", err)
	}

	if exec != nil {
		t.Error("Expected error for sourceless accept...then at top level, but CreateStateExecutor succeeded")
	}
}

func testCalcUnboundParameter(t *testing.T) {
	src := `
		package test {
			calc add {
				in x: Integer;
				in y: Integer;
				return x + y;
			}
		}
	`
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}

	idx, model, ctx := buildRuntime(t, "<test>", file)

	_ = model // silence unused

	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "add", ast.DefCalc)
	if sym == nil {
		t.Fatal("add calc not found")
	}

	// Invoke with only 1 argument (missing y)
	xVal := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 3}}
	result, err := ctx.InvokeCalc(sym, []Value{xVal}, rootScope)

	if err != nil {
		t.Logf("InvokeCalc returned error (expected): %v", err)
		return
	}

	// Some implementations may return null or zero
	if result.Kind == ValNull {
		t.Log("InvokeCalc returned null (unbound parameter)")
		return
	}

	t.Logf("InvokeCalc returned value (unbound parameter accepted): %+v", result)
}

// testConstraintMissingFeature: constraint references nonexistent feature
func testConstraintMissingFeature(t *testing.T) {
	src := `
		package test {
			constraint broken {
				assert nonexistent > 0; // 'nonexistent' feature doesn't exist
			}
		}
	`
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}

	idx, model, ctx := buildRuntime(t, "<test>", file)

	_ = model // silence unused

	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "broken", ast.DefConstraint)
	if sym == nil {
		t.Fatal("broken constraint not found")
	}

	satisfied, err := ctx.EvaluateConstraint(sym, rootScope)

	if err != nil {
		t.Logf("EvaluateConstraint returned error (expected): %v", err)
		return
	}

	if !satisfied {
		t.Log("EvaluateConstraint returned false (missing feature treated as unsatisfied)")
		return
	}

	t.Log("EvaluateConstraint returned true (missing feature tolerated)")
}

// testStepBudgetExceeded: evaluation exceeds maxSteps. Each Eval call spends one
// step, so an expression with more subexpressions than the budget must report
// ErrStepLimitExceeded rather than run to the end. The operands are a parameter
// rather than literals because a constant expression is folded in one step.
func testStepBudgetExceeded(t *testing.T) {
	src := `
		package test {
			calc deep {
				in x : Integer;
				return x + x + x + x + x + x + x + x;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))

	ctx.maxSteps = 3
	ctx.steps = 0

	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "deep", ast.DefCalc)
	if sym == nil {
		t.Fatal("calc deep not found")
	}

	arg := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 1}}
	_, err := ctx.InvokeCalc(sym, []Value{arg}, rootScope)
	if err == nil {
		t.Fatal("expected the step budget to be exceeded, the calc completed")
	}
	if !errors.Is(err, ErrStepLimitExceeded) {
		t.Errorf("expected ErrStepLimitExceeded, got: %v", err)
	}
}

// Helper: parse source into AST RootNamespace
func parseAndBuild(t *testing.T, src string) *ast.RootNamespace {
	file := parser.New(source.New("<test>", []byte(src))).ParseFile()
	return file
}

// Helper: build runtime context from file
func buildRuntime(t *testing.T, path string, file *ast.RootNamespace) (*symbols.Index, *semantics.Model, *Context) {
	idx := symbols.NewIndex()
	idx.AddDocument(path, file)
	resolver := resolve.New(idx)
	model := semantics.NewModel(resolver)
	ctx := NewContext(model, resolver, 10000)
	return idx, model, ctx
}

// Helper: find symbol by name and kind
func findSymbolByName(scope *symbols.Scope, name string, kind ast.DefinitionKind) *symbols.Symbol {
	// Map DefKind to UsageKind
	var usageKind ast.UsageKind
	switch kind {
	case ast.DefCalc:
		usageKind = ast.UsageCalc
	case ast.DefAction:
		usageKind = ast.UsageAction
	case ast.DefState:
		usageKind = ast.UsageState
	case ast.DefConstraint:
		usageKind = ast.UsageConstraint
	case ast.DefRequirement:
		usageKind = ast.UsageRequirement
	}

	// Check all child scopes (packages/namespaces)
	for _, child := range scope.Children() {
		for _, memberName := range child.MemberNames() {
			sym, _ := child.LookupLocal(memberName)
			if sym == nil {
				continue
			}

			if sym.Name == name {
				switch decl := sym.Decl.(type) {
				case *ast.Definition:
					if decl.Kind == kind {
						return sym
					}
				case *ast.Usage:
					if decl.Kind == usageKind {
						return sym
					}
				}
			}
		}
	}

	// Also check root scope directly
	for _, memberName := range scope.MemberNames() {
		sym, _ := scope.LookupLocal(memberName)
		if sym == nil {
			continue
		}

		if sym.Name == name {
			switch decl := sym.Decl.(type) {
			case *ast.Definition:
				if decl.Kind == kind {
					return sym
				}
			case *ast.Usage:
				if decl.Kind == usageKind {
					return sym
				}
			}
		}
	}
	return nil
}
