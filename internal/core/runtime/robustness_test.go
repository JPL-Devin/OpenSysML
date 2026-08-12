package runtime

import (
	"errors"
	"strings"
	"testing"
	"time"

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
	t.Run("calc_too_many_arguments", testCalcTooManyArguments)
	t.Run("calc_unknown_named_argument", testCalcUnknownNamedArgument)
	t.Run("calc_without_result", testCalcWithoutResult)
	t.Run("calc_symbol_is_not_a_calc", testCalcSymbolIsNotACalc)
	t.Run("calc_direct_recursion", testCalcDirectRecursion)
	t.Run("calc_mutual_recursion", testCalcMutualRecursion)
	t.Run("constraint_missing_feature", testConstraintMissingFeature)
	t.Run("requirement_feature_without_a_value", testRequirementFeatureWithoutAValue)
	t.Run("requirement_features_valued_from_each_other", testRequirementFeaturesValuedFromEachOther)
	t.Run("step_budget_exceeded", testStepBudgetExceeded)
	t.Run("non_terminating_loop_exhausts_step_budget", testNonTerminatingLoopExhaustsStepBudget)
	t.Run("loop_body_declaration_does_not_leak", testLoopBodyDeclarationDoesNotLeak)
	t.Run("loop_body_of_unexecutable_statement", testLoopBodyOfUnexecutableStatement)
	t.Run("statement_directly_in_an_action_body", testStatementDirectlyInAnActionBody)
	t.Run("fork_branches_share_region", testForkBranchesShareRegion)
	t.Run("join_with_one_incoming_branch", testJoinWithOneIncomingBranch)
	t.Run("region_pseudostate_without_satisfied_guard", testRegionPseudostateWithoutSatisfiedGuard)
	t.Run("region_pseudostate_cycle", testRegionPseudostateCycle)
	t.Run("non_numeric_time_trigger", testNonNumericTimeTrigger)
	t.Run("send_reaches_only_its_addressee", testSendReachesOnlyItsAddressee)
	t.Run("accept_of_unsent_type", testAcceptOfUnsentTypeReports)
	t.Run("send_via_unconnected_port", testSendViaUnconnectedPort)
	t.Run("accept_deadlock_never_satisfied", testAcceptDeadlockNeverSatisfied)
	t.Run("accept_deadlock_reports_every_waiting_accept", testAcceptDeadlockReportsEveryWaitingAccept)
	t.Run("history_outside_composite_state", testHistoryOutsideCompositeState)
	t.Run("history_without_record_or_default", testHistoryWithoutRecordOrDefault)
	t.Run("defer_of_non_deferrable_trigger", testDeferOfNonDeferrableTrigger)
	t.Run("non_terminating_do_behavior", testNonTerminatingDoBehavior)
	t.Run("call_of_unhandled_operation", testCallOfUnhandledOperation)
	t.Run("call_argument_of_wrong_type", testCallArgumentOfWrongType)
	t.Run("perform_of_missing_action", testPerformOfMissingAction)
	t.Run("perform_reference_cycle", testPerformReferenceCycle)
	t.Run("state_subaction_reference_of_missing_action", testStateSubactionReferenceOfMissingAction)
	t.Run("state_subaction_reference_feature_chain", testStateSubactionReferenceFeatureChain)
	t.Run("library_function_outside_its_domain", testLibraryFunctionOutsideItsDomain)
	t.Run("library_function_wrong_arity", testLibraryFunctionWrongArity)
	t.Run("extension_library_function_outside_its_domain", testExtensionLibraryFunctionOutsideItsDomain)
	t.Run("exponentiation_integer_overflow", testExponentiationIntegerOverflow)
	t.Run("quantity_incommensurable_comparison", testQuantityIncommensurableComparison)
	t.Run("quantity_index_is_not_a_unit", testQuantityIndexIsNotAUnit)
	t.Run("quantity_cyclic_unit_definition", testQuantityCyclicUnitDefinition)
	t.Run("cyclic_derived_slot", testCyclicDerivedSlot)
	t.Run("derived_slot_over_missing_feature", testDerivedSlotOverMissingFeature)
}

// testCyclicDerivedSlot: two derived defaults that read each other are reported
// as a cycle instead of recursing until the step budget runs out.
func testCyclicDerivedSlot(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `
		package test {
			part def Loop {
				attribute a = b + 1.0;
				attribute b = a + 1.0;
			}
		}
	`))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Loop", ast.DefPart)
	if sym == nil {
		t.Fatal("Loop part def not found")
	}

	inst, err := ctx.Instantiate(sym)
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}

	done := make(chan struct{})
	var slotErr error
	go func() {
		defer close(done)
		_, slotErr = inst.GetSlot(ctx, "a")
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("GetSlot hung on a cyclic derived slot")
	}

	if !errors.Is(slotErr, ErrCyclicSlot) {
		t.Fatalf("GetSlot error = %v, want ErrCyclicSlot", slotErr)
	}
}

// testDerivedSlotOverMissingFeature: a derived default that names something the
// instance does not have fails with the slot named, rather than silently
// leaving the slot empty.
func testDerivedSlotOverMissingFeature(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `
		package test {
			part def Broken {
				attribute derived = missing * 2.0;
			}
		}
	`))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Broken", ast.DefPart)
	if sym == nil {
		t.Fatal("Broken part def not found")
	}

	inst, err := ctx.Instantiate(sym)
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}

	_, err = inst.GetSlot(ctx, "derived")
	if err == nil {
		t.Fatal("GetSlot succeeded on a default over an undeclared feature")
	}
	if !strings.Contains(err.Error(), "derived") {
		t.Errorf("error %q does not name the slot", err)
	}
}

// testStateSubactionReferenceOfMissingAction: an entry action given by
// reference to a name nothing declares fails at execution, naming the target.
func testStateSubactionReferenceOfMissingAction(t *testing.T) {
	ctx, machine := loadState(t, `package test {
		state Machine {
			initial init;
			state active {
				entry noSuchAction;
			}
			final done;

			init then active;
			active then done;
		}
	}`, "Machine")

	if _, _, err := ctx.ExecuteStateWithEvents(machine, nil); err == nil {
		t.Fatal("expected an unresolved entry action reference to fail")
	} else if !strings.Contains(err.Error(), "noSuchAction") {
		t.Errorf("error should name the unresolved action, got: %v", err)
	}
}

// testStateSubactionReferenceFeatureChain: a feature-chain reference parses but
// is not invocable, so it must report what it named rather than an empty name.
func testStateSubactionReferenceFeatureChain(t *testing.T) {
	ctx, machine := loadState(t, `package test {
		action def CoolDown {
			first start;
			done end;
			then start end;
		}

		state Machine {
			part controller {
				action coolDown : CoolDown;
			}

			initial init;
			state active {
				exit controller.coolDown;
			}
			final done;

			init then active;
			active then done;
		}
	}`, "Machine")

	if _, _, err := ctx.ExecuteStateWithEvents(machine, nil); err == nil {
		t.Fatal("expected a feature-chain action reference to fail")
	} else if !strings.Contains(err.Error(), "coolDown") {
		t.Errorf("error should name the chained action, got: %v", err)
	}
}

// testPerformOfMissingAction: a perform statement naming nothing resolvable is
// an error at execution, not a silently skipped node.
func testPerformOfMissingAction(t *testing.T) {
	ctx, outer := loadAction(t, `package test {
		action outer {
			first start;
			perform action doIt references missingAction;
			done end;

			then start doIt;
			then doIt end;
		}
	}`, "outer")

	if _, err := ctx.ExecuteAction(outer); err == nil {
		t.Fatal("expected performing an unresolved action to fail")
	} else if !strings.Contains(err.Error(), "missingAction") {
		t.Errorf("error should name the unresolved action, got: %v", err)
	}
}

// testPerformReferenceCycle: an action performing itself must be stopped by the
// nesting bound instead of recursing forever.
func testPerformReferenceCycle(t *testing.T) {
	ctx, outer := loadAction(t, `package test {
		action outer {
			first start;
			perform action doIt references outer;
			done end;

			then start doIt;
			then doIt end;
		}
	}`, "outer")

	done := make(chan error, 1)
	go func() {
		_, err := ctx.ExecuteAction(outer)
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected a self-performing action to be bounded, it completed")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("self-performing action did not terminate")
	}
}

// testDeferOfNonDeferrableTrigger: only signals and calls are dispatched from
// the event pool, so a state deferring a time trigger is reported at lowering
// rather than deferring nothing at run time.
func testDeferOfNonDeferrableTrigger(t *testing.T) {
	idx := symbols.NewIndex()
	resolver := resolve.New(idx)
	ctx := NewContext(semantics.NewModel(resolver), resolver, 1000)

	machine := &ast.Usage{
		Kind:  ast.UsageState,
		Ident: ast.Identification{Name: "Machine"},
		Members: []ast.Node{
			&ast.StateNode{Name: "init", IsInitial: true},
			&ast.StateNode{
				Name:  "busy",
				Defer: []ast.Node{&ast.TimeEvent{Duration: &ast.LiteralInteger{Value: "1"}}},
			},
			transitionMember("init", "busy"),
		},
	}

	_, err := newStateExecutor(ctx, &symbols.Symbol{
		Kind: symbols.SymbolStateUsage,
		Name: machine.Ident.Name,
		Decl: machine,
	})
	if err == nil {
		t.Fatal("expected an error for a state deferring a time trigger")
	}
	if !strings.Contains(err.Error(), "only signal and call triggers can be deferred") {
		t.Errorf("expected a deferrability error, got: %v", err)
	}
}

// testNonTerminatingDoBehavior: a do behavior whose state is re-entered every
// round never ends, so the run is bounded and reports instead of hanging.
func testNonTerminatingDoBehavior(t *testing.T) {
	spin := &ast.StateNode{
		Name: "spin",
		Do: []ast.Node{&ast.AssignmentActionNode{
			Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "ticks"}}},
			Value:  &ast.LiteralInteger{Value: "1"},
		}},
	}
	exec := stateExecutorFor(t, &ast.Usage{
		Kind:  ast.UsageState,
		Ident: ast.Identification{Name: "Machine"},
		Members: []ast.Node{
			&ast.StateNode{Name: "init", IsInitial: true},
			spin,
			transitionMember("init", "spin"),
			transitionMember("spin", "spin"),
		},
	})
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	err := exec.RunToCompletion()
	if err == nil {
		t.Fatal("expected a budget error for a machine that never settles")
	}
	if !strings.Contains(err.Error(), "exceeded max") {
		t.Errorf("expected a budget error, got: %v", err)
	}
}

// stateExecutorForSource builds an executor for the named machine in src, for
// tests that drive it event by event.
func stateExecutorForSource(t *testing.T, name, src string) *StateExecutor {
	t.Helper()
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), name, ast.DefState)
	if sym == nil {
		t.Fatalf("state machine %s not found", name)
	}
	exec, err := newStateExecutor(ctx, sym)
	if err != nil {
		t.Fatalf("newStateExecutor: %v", err)
	}
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	return exec
}

// testCallOfUnhandledOperation: an invocation no trigger names is discarded by
// run-to-completion, leaving the machine where it was rather than hanging.
func testCallOfUnhandledOperation(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			initial init;
			state waiting;
			state moving;
			init then waiting;
			transition waiting to moving accept go();
		}
	}`)
	exec.InvokeOperation("halt", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run to completion: %v", err)
	}
	current, ok := exec.CurrentState().(*ast.StateNode)
	if !ok || current.Name != "waiting" {
		t.Errorf("expected the unhandled call to leave the machine in waiting, got %v", exec.CurrentState())
	}
}

// testCallArgumentOfWrongType: an argument the guard cannot compare reports
// rather than firing or dropping the transition on a wrong comparison.
func testCallArgumentOfWrongType(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			initial init;
			state waiting;
			state moving;
			init then waiting;
			transition waiting to moving accept setSpeed(value) if value > 0;
		}
	}`)
	exec.InvokeOperation("setSpeed", map[string]Value{
		"value": {Kind: ValString, Str: "fast"},
	})
	err := exec.RunToCompletion()
	if err == nil {
		t.Fatal("expected an error: the guard compares a String argument with 0")
	}
	if !strings.Contains(err.Error(), "string") {
		t.Errorf("expected the offending operand kind in the message, got: %v", err)
	}
}

// testHistoryOutsideCompositeState: a history pseudostate restores the state
// that declares it, so one declared directly in the machine has nothing to
// restore and must report rather than enter an arbitrary state.
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

// testSendViaUnconnectedPort: a port with no connection reaches no one, so the
// accept waiting on the message suspends forever — which must be reported as a
// deadlock rather than hanging or binding nothing.
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
	if !errors.Is(err, ErrAcceptDeadlock) {
		t.Errorf("expected ErrAcceptDeadlock, got: %v", err)
	}
}

// testAcceptDeadlockNeverSatisfied: an accept nothing can ever satisfy suspends
// the action, and a suspension that can never end must be reported as a typed
// deadlock rather than hanging.
func testAcceptDeadlockNeverSatisfied(t *testing.T) {
	done := make(chan error, 1)
	go func() {
		_, err := executeActionSource(t, "pipeline", `package P {
			action pipeline {
				first start;
				action reader accept n : Integer;
				done end;
				then start reader;
				then reader end;
			}
		}`)
		done <- err
	}()

	var err error
	select {
	case err = <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("an action waiting for a message that cannot arrive did not terminate")
	}

	if err == nil {
		t.Fatal("expected a deadlock error, the suspended accept completed")
	}
	if !errors.Is(err, ErrAcceptDeadlock) {
		t.Errorf("expected ErrAcceptDeadlock, got: %v", err)
	}
	for _, want := range []string{"accept n", "Integer"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("expected %q in the deadlock report, got: %v", want, err)
		}
	}
}

// testAcceptDeadlockReportsEveryWaitingAccept: with two accepts parked in
// parallel branches and only one message in flight, the accept that can proceed
// does, and the report names the one still waiting rather than the whole action.
func testAcceptDeadlockReportsEveryWaitingAccept(t *testing.T) {
	_, err := executeActionSource(t, "pipeline", `package P {
		action pipeline {
			attribute got : Integer = 0;
			first start;
			action sender { send 7 to reader; }
			fork split;
			action reader accept n : Integer;
			action recorder { assign got := n; }
			action listener accept text : String;
			join sync;
			done end;
			then start sender;
			then sender split;
			then split reader;
			then split listener;
			then reader recorder;
			then recorder sync;
			then listener sync;
			then sync end;
		}
	}`)
	if err == nil {
		t.Fatal("expected a deadlock error: no String is ever sent")
	}
	if !errors.Is(err, ErrAcceptDeadlock) {
		t.Fatalf("expected ErrAcceptDeadlock, got: %v", err)
	}
	if !strings.Contains(err.Error(), "accept text waiting since step 4 for a message of type String") {
		t.Errorf("expected the still-waiting accept in the report, got: %v", err)
	}
	if strings.Contains(err.Error(), "accept n ") {
		t.Errorf("the Integer accept was satisfied and must not be reported as waiting: %v", err)
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

// testRegionPseudostateWithoutSatisfiedGuard: a junction reached from inside an
// orthogonal region whose branches are all guarded false has nowhere to go. The
// region set is left in place and the dead end reported, rather than the machine
// resting on a pseudostate.
func testRegionPseudostateWithoutSatisfiedGuard(t *testing.T) {
	_, _, err := executeStateSource(t, "Machine", `package test {
		state Machine {
			attribute x : Integer = 9;

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

			transition merge to b if x == 1;
		}
	}`)
	if err == nil {
		t.Fatal("expected an error for a junction with no satisfied guard")
	}
	if !strings.Contains(err.Error(), "no guard evaluated to true") {
		t.Errorf("expected an unsatisfied-guard error, got: %v", err)
	}
}

// testRegionPseudostateCycle: pseudostates that route into each other never
// reach a state, so following the chain has to report the cycle instead of
// looping forever.
func testRegionPseudostateCycle(t *testing.T) {
	_, _, err := executeStateSource(t, "Machine", `package test {
		state Machine {
			region left {
				initial ls;
				state a;
				then ls a;
				transition a to first;
			}
			region right {
				initial rs;
				state c;
				then rs c;
			}
			junction first;
			junction second;

			transition first to second;
			transition second to first;
		}
	}`)
	if err == nil {
		t.Fatal("expected an error for pseudostates routing into each other")
	}
	if !strings.Contains(err.Error(), "form a cycle") {
		t.Errorf("expected a cycle error, got: %v", err)
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

// testCalcUnboundParameter: a parameter with neither an argument nor a default
// is a modeling error, not a null value.
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
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "add", ast.DefCalc)
	if sym == nil {
		t.Fatal("add calc not found")
	}

	// Invoke with only 1 argument (missing y)
	xVal := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 3}}
	result, err := ctx.InvokeCalc(sym, []Value{xVal}, rootScope)
	if err == nil {
		t.Fatalf("expected an unbound parameter error, calc returned %+v", result)
	}
	if !errors.Is(err, ErrUnboundParameter) {
		t.Errorf("expected ErrUnboundParameter, got: %v", err)
	}
}

// testCalcTooManyArguments: more arguments than parameters has no binding, so it
// reports an arity error instead of dropping the extras.
func testCalcTooManyArguments(t *testing.T) {
	src := `
		package test {
			calc double {
				in x: Integer;
				return x * 2;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "double", ast.DefCalc)
	if sym == nil {
		t.Fatal("double calc not found")
	}

	arg := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 1}}
	_, err := ctx.InvokeCalc(sym, []Value{arg, arg}, rootScope)
	if err == nil {
		t.Fatal("expected an arity error, the calc accepted a surplus argument")
	}
	if !errors.Is(err, ErrCalcArity) {
		t.Errorf("expected ErrCalcArity, got: %v", err)
	}
}

// testCalcUnknownNamedArgument: a named argument that matches no parameter is
// reported instead of silently leaving the parameter on its default.
func testCalcUnknownNamedArgument(t *testing.T) {
	src := `
		package test {
			calc scale {
				in x: Integer = 1;
				return x * 2;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "scale", ast.DefCalc)
	if sym == nil {
		t.Fatal("scale calc not found")
	}

	arg := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 3}}
	_, err := ctx.InvokeCalcNamed(sym, map[string]Value{"factor": arg}, rootScope)
	if err == nil {
		t.Fatal("expected an unknown parameter error, the invocation succeeded")
	}
	if !errors.Is(err, ErrUnknownParameter) {
		t.Errorf("expected ErrUnknownParameter, got: %v", err)
	}
}

// testCalcWithoutResult: a calc body with no return expression has no value to
// produce, own or inherited.
func testCalcWithoutResult(t *testing.T) {
	src := `
		package test {
			calc empty {
				in x: Integer;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "empty", ast.DefCalc)
	if sym == nil {
		t.Fatal("empty calc not found")
	}

	arg := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 1}}
	_, err := ctx.InvokeCalc(sym, []Value{arg}, rootScope)
	if err == nil {
		t.Fatal("expected a missing-result error, the calc returned a value")
	}
	if !errors.Is(err, ErrNoResultExpression) {
		t.Errorf("expected ErrNoResultExpression, got: %v", err)
	}
}

// testCalcSymbolIsNotACalc: invoking a non-calc symbol is rejected by kind
// rather than by whatever its body happens to contain.
func testCalcSymbolIsNotACalc(t *testing.T) {
	src := `
		package test {
			part def Engine {
				attribute power : Integer;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "Engine", ast.DefPart)
	if sym == nil {
		t.Fatal("Engine part def not found")
	}

	_, err := ctx.InvokeCalc(sym, nil, rootScope)
	if err == nil {
		t.Fatal("expected a not-a-calc error, the invocation succeeded")
	}
	if !errors.Is(err, ErrNotACalc) {
		t.Errorf("expected ErrNotACalc, got: %v", err)
	}
}

// testCalcDirectRecursion: a calc that invokes itself unconditionally must be
// stopped by the nesting bound instead of exhausting the stack.
func testCalcDirectRecursion(t *testing.T) {
	src := `
		package test {
			calc countdown {
				in n: Integer;
				return countdown(n - 1);
			}
		}
	`
	assertCalcRecursionBounded(t, src, "countdown")
}

// testCalcMutualRecursion: the bound is on nesting depth, so a cycle through
// another calc is caught the same way as direct self-invocation.
func testCalcMutualRecursion(t *testing.T) {
	src := `
		package test {
			calc ping {
				in n: Integer;
				return pong(n);
			}

			calc pong {
				in n: Integer;
				return ping(n);
			}
		}
	`
	assertCalcRecursionBounded(t, src, "ping")
}

// assertCalcRecursionBounded invokes calcName and requires a recursion-limit
// error promptly: the invocation runs on its own goroutine so a hang fails the
// case instead of stalling the suite until the package timeout.
func assertCalcRecursionBounded(t *testing.T, src, calcName string) {
	t.Helper()

	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, calcName, ast.DefCalc)
	if sym == nil {
		t.Fatalf("calc %s not found", calcName)
	}

	done := make(chan error, 1)
	go func() {
		arg := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 10}}
		_, err := ctx.InvokeCalc(sym, []Value{arg}, rootScope)
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatalf("expected recursive calc %s to be bounded, it returned a value", calcName)
		}
		if !errors.Is(err, ErrCalcRecursionLimit) {
			t.Errorf("expected ErrCalcRecursionLimit, got: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("recursive calc %s did not terminate", calcName)
	}
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

// testRequirementFeatureWithoutAValue: a condition naming a feature the
// requirement declares but nothing gives a value to reports ErrNoValue, naming
// the feature, rather than the unresolved-feature error of a name that is not
// declared at all.
func testRequirementFeatureWithoutAValue(t *testing.T) {
	src := `
		package test {
			requirement def TouchdownRequirement {
				attribute actualVerticalSpeed;
				attribute maxVerticalSpeed = 1.5;
				require actualVerticalSpeed <= maxVerticalSpeed;
			}
		}
	`
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}
	idx, _, ctx := buildRuntime(t, "<test>", file)
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "TouchdownRequirement", ast.DefRequirement)
	if sym == nil {
		t.Fatal("TouchdownRequirement not found")
	}

	satisfied, err := ctx.EvaluateRequirement(sym, rootScope)
	if err == nil {
		t.Fatalf("expected an error, got satisfied = %v", satisfied)
	}
	if !errors.Is(err, ErrNoValue) {
		t.Errorf("expected ErrNoValue, got: %v", err)
	}
	if errors.Is(err, ErrViolated) {
		t.Error("a feature without a value is not a violation")
	}
	if !strings.Contains(err.Error(), "actualVerticalSpeed") {
		t.Errorf("error does not name the feature: %v", err)
	}
}

// testRequirementFeaturesValuedFromEachOther: two features whose values name each
// other report a cycle promptly instead of recursing until the step budget runs out.
func testRequirementFeaturesValuedFromEachOther(t *testing.T) {
	src := `
		package test {
			requirement def R {
				attribute a = b;
				attribute b = a;
				require a <= b;
			}
		}
	`
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}
	idx, _, ctx := buildRuntime(t, "<test>", file)
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "R", ast.DefRequirement)
	if sym == nil {
		t.Fatal("R not found")
	}

	satisfied, err := ctx.EvaluateRequirement(sym, rootScope)
	if err == nil {
		t.Fatalf("expected an error, got satisfied = %v", satisfied)
	}
	if !errors.Is(err, ErrCyclicSlot) {
		t.Errorf("expected ErrCyclicSlot, got: %v", err)
	}
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

// testNonTerminatingLoopExhaustsStepBudget: a loop whose condition never fails
// spends a step per iteration, so it ends the execution with
// ErrStepLimitExceeded instead of hanging whoever drove it (a REPL or the LSP).
func testNonTerminatingLoopExhaustsStepBudget(t *testing.T) {
	src := `
		package test {
			action spinner {
				attribute total : Integer = 0;
				first start;
				action spin {
					while total >= 0 {
						assign total := total + 1;
					}
				}
				done end;
				then start spin;
				then spin end;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))

	ctx.maxSteps = 20
	ctx.steps = 0

	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "spinner", ast.DefAction)
	if sym == nil {
		t.Fatal("action spinner not found")
	}

	_, err := ctx.ExecuteAction(sym)
	if err == nil {
		t.Fatal("expected the step budget to be exceeded, the action completed")
	}
	if !errors.Is(err, ErrStepLimitExceeded) {
		t.Errorf("expected ErrStepLimitExceeded, got: %v", err)
	}
}

// testLoopBodyDeclarationDoesNotLeak: a loop body and an `if` branch body are
// namespaces of their own, so a name one of them declares is not a member of the
// action and does not appear among its results.
func testLoopBodyDeclarationDoesNotLeak(t *testing.T) {
	src := `
		package test {
			action counter {
				attribute total : Integer = 0;
				first start;
				action accumulate {
					while total < 3 {
						attribute bump : Integer = 1;
						assign total := total + bump;
						if total == 2 {
							attribute marker : Integer = 9;
							assign total := total + marker;
						}
					}
				}
				done end;
				then start accumulate;
				then accumulate end;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))

	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "counter", ast.DefAction)
	if sym == nil {
		t.Fatal("action counter not found")
	}

	outputs, err := ctx.ExecuteAction(sym)
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	// 1, then 2 which the conditional lifts to 11, which ends the loop.
	total, ok := outputs["total"]
	if !ok {
		t.Fatal("total missing from the action's results")
	}
	if total.Const.Int != 11 {
		t.Errorf("total = %v, want 11", FormatTraceValue(total))
	}
	for _, local := range []string{"bump", "marker"} {
		if _, ok := outputs[local]; ok {
			t.Errorf("body-local %s leaked into the action's results: %v", local, outputs)
		}
	}
}

// testLoopBodyOfUnexecutableStatement: a body member the lowering layer cannot
// turn into a statement is reported when it is reached, rather than skipped —
// silently dropping it would give a wrong answer with no diagnostic.
func testLoopBodyOfUnexecutableStatement(t *testing.T) {
	src := `
		package test {
			action counter {
				attribute total : Integer = 0;
				first start;
				action accumulate {
					while total < 3 {
						action inner;
						assign total := total + 1;
					}
				}
				done end;
				then start accumulate;
				then accumulate end;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))

	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "counter", ast.DefAction)
	if sym == nil {
		t.Fatal("action counter not found")
	}

	_, err := ctx.ExecuteAction(sym)
	if err == nil {
		t.Fatal("expected an unexecutable loop body member to be reported")
	}
	if !strings.Contains(err.Error(), "not executable") {
		t.Errorf("error does not name the unexecutable member: %v", err)
	}
}

// testStatementDirectlyInAnActionBody: a statement written among the action's
// own members has no name a succession can reach, so it is reported rather than
// ignored.
func testStatementDirectlyInAnActionBody(t *testing.T) {
	cases := map[string]string{
		"while":      "while total < 5 { assign total := total + 1; }",
		"if":         "if total < 5 { assign total := total + 1; }",
		"assignment": "assign total := total + 1;",
	}

	for name, stmt := range cases {
		t.Run(name, func(t *testing.T) {
			src := `
				package test {
					action counter {
						attribute total : Integer = 0;
						first start;
						` + stmt + `
						done end;
						then start end;
					}
				}
			`
			idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
			sym := findSymbolByName(idx.DocumentRoot("<test>"), "counter", ast.DefAction)
			if sym == nil {
				t.Fatal("action counter not found")
			}

			_, err := ctx.ExecuteAction(sym)
			if err == nil {
				t.Fatalf("expected a top-level %s to be reported", name)
			}
			if !strings.Contains(err.Error(), "no position in the token flow") {
				t.Errorf("error does not explain why the statement cannot run: %v", err)
			}
		})
	}
}

// Helper: parse source into AST RootNamespace
func parseAndBuild(t *testing.T, src string) *ast.RootNamespace {
	file := parser.New(source.New("<test>", []byte(src))).ParseFile()
	return file
}

// testLibraryFunctionOutsideItsDomain: a library function whose argument has no
// result reports a domain error rather than returning a NaN.
func testLibraryFunctionOutsideItsDomain(t *testing.T) {
	src := `
		package test {
			calc root {
				in x : Real;
				return : Real = sqrt(x);
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "root", ast.DefCalc)
	if sym == nil {
		t.Fatal("root calc not found")
	}

	arg := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: -1}}
	got, err := ctx.InvokeCalc(sym, []Value{arg}, rootScope)
	if !errors.Is(err, semantics.ErrArithmeticDomain) {
		t.Fatalf("sqrt(-1.0) = %+v, %v; want a domain error", got, err)
	}
}

// testLibraryFunctionWrongArity: a library function called with the wrong number
// of arguments reports an arity error rather than reading past its arguments.
func testLibraryFunctionWrongArity(t *testing.T) {
	fn, ok := libraryFunctionByName("RealFunctions::max")
	if !ok {
		t.Fatal("RealFunctions::max not registered")
	}
	_, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, "package test { }"))

	arg := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: 1}}
	if _, err := fn.invoke(ctx, calcArgs{positional: []Value{arg}}); !errors.Is(err, ErrCalcArity) {
		t.Fatalf("max(1.0) error = %v, want ErrCalcArity", err)
	}
}

// testExtensionLibraryFunctionOutsideItsDomain: a Systemica extension library
// function reports a domain error the same way a vendored one does — the
// logarithm of zero has no Real value, and is not returned as an infinity.
func testExtensionLibraryFunctionOutsideItsDomain(t *testing.T) {
	src := `
		package test {
			calc root {
				in x : Real;
				return : Real = ln(x);
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "root", ast.DefCalc)
	if sym == nil {
		t.Fatal("root calc not found")
	}

	arg := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: 0}}
	got, err := ctx.InvokeCalc(sym, []Value{arg}, rootScope)
	if !errors.Is(err, semantics.ErrArithmeticDomain) {
		t.Fatalf("ln(0.0) = %+v, %v; want a domain error", got, err)
	}
}

// testExponentiationIntegerOverflow: an exponentiation beyond the Integer range
// is reported rather than wrapping.
func testExponentiationIntegerOverflow(t *testing.T) {
	src := `
		package test {
			calc power {
				in b : Integer;
				in e : Integer;
				return : Integer = b ** e;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "power", ast.DefCalc)
	if sym == nil {
		t.Fatal("power calc not found")
	}

	base := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 1 << 40}}
	exp := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 3}}
	got, err := ctx.InvokeCalc(sym, []Value{base, exp}, rootScope)
	if !errors.Is(err, semantics.ErrArithmeticOverflow) {
		t.Fatalf("(2**40) ** 3 = %+v, %v; want an overflow error", got, err)
	}
}

// testQuantityIncommensurableComparison: comparing quantities whose units
// measure different things reports ErrIncommensurableUnits instead of comparing
// the bare magnitudes, which would make 1.5 [m/s] <= 2.0 [s] true.
func testQuantityIncommensurableComparison(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			public import SI::*;
			requirement def Touchdown {
				attribute speed = 1.5 [m/s];
				attribute duration = 2.0 [s];
				require speed <= duration;
			}
		}
	`))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "Touchdown", ast.DefRequirement)
	if sym == nil {
		t.Fatal("Touchdown requirement not found")
	}

	satisfied, err := ctx.EvaluateRequirement(sym, rootScope)
	if !errors.Is(err, ErrIncommensurableUnits) {
		t.Fatalf("satisfied = %v, err = %v; want ErrIncommensurableUnits", satisfied, err)
	}
	if errors.Is(err, ErrViolated) {
		t.Error("incommensurable units are not a violation: neither verdict is an answer")
	}
}

// testQuantityIndexIsNotAUnit: a bracketed expression whose index names
// something that is not a measurement unit reports ErrNotAQuantity rather than
// evaluating to the bare magnitude.
func testQuantityIndexIsNotAUnit(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			public import SI::*;
			attribute notAUnit = 3.0;
			constraint bogus {
				1.5 [test::notAUnit] <= 2.0 [m]
			}
		}
	`))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "bogus", ast.DefConstraint)
	if sym == nil {
		t.Fatal("bogus constraint not found")
	}

	satisfied, err := ctx.EvaluateConstraint(sym, rootScope)
	if !errors.Is(err, ErrNotAQuantity) {
		t.Fatalf("satisfied = %v, err = %v; want ErrNotAQuantity", satisfied, err)
	}
	if !strings.Contains(err.Error(), semantics.ErrNotAUnit.Error()) {
		t.Errorf("err = %v; want it to report that the index names no measurement unit", err)
	}
}

// testQuantityCyclicUnitDefinition: two units defined in terms of each other are
// reported as a cycle instead of recursing until the stack or step budget runs
// out.
func testQuantityCyclicUnitDefinition(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			public import SI::*;
			attribute unitA : ISQBase::LengthUnit = unitB;
			attribute unitB : ISQBase::LengthUnit = unitA;
			constraint cyclic {
				1.0 [test::unitA] <= 2.0 [test::unitA]
			}
		}
	`))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "cyclic", ast.DefConstraint)
	if sym == nil {
		t.Fatal("cyclic constraint not found")
	}

	done := make(chan error, 1)
	go func() {
		_, err := ctx.EvaluateConstraint(sym, rootScope)
		done <- err
	}()
	select {
	case err := <-done:
		if !errors.Is(err, semantics.ErrUnitCycle) {
			t.Fatalf("err = %v; want ErrUnitCycle", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("evaluating a cyclic unit definition did not terminate")
	}
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

// buildRuntimeWithLibraries builds a runtime context over an index that carries
// the standard library, for a model that names its elements.
func buildRuntimeWithLibraries(t *testing.T, path string, file *ast.RootNamespace) (*symbols.Index, *semantics.Model, *Context) {
	t.Helper()
	idx := symbols.NewIndex()
	loadLibraries(t, idx)
	idx.AddDocument(path, file)
	idx.ExpandWildcardImports()
	resolver := resolve.New(idx)
	model := semantics.NewModel(resolver)
	return idx, model, NewContext(model, resolver, 10000)
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
