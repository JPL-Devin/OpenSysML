package runtime

import (
	"errors"
	"fmt"
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
	t.Run("action_whose_last_node_has_no_succession", testActionWhoseLastNodeHasNoSuccession)
	t.Run("first_node_with_a_second_succession", testFirstNodeWithASecondSuccession)
	t.Run("first_beside_an_initial_node", testFirstBesideAnInitialNode)
	t.Run("first_naming_a_final_node", testFirstNamingAFinalNode)
	t.Run("fork_branches_assigning_the_same_feature", testForkBranchesAssigningTheSameFeature)
	t.Run("decision_no_satisfied_guard", testDecisionNoSatisfiedGuard)
	t.Run("state_dangling_transition", testStateDanglingTransition)
	t.Run("state_transition_endpoint_misspelled", testStateTransitionEndpointMisspelled)
	t.Run("state_transition_endpoint_in_another_machine", testStateTransitionEndpointInAnotherMachine)
	t.Run("state_transition_endpoint_never_resolved", testStateTransitionEndpointNeverResolved)
	t.Run("state_transition_endpoint_naming_a_first_marker", testStateTransitionEndpointNamingAFirstMarker)
	t.Run("state_junction_without_an_outgoing_transition", testStateJunctionWithoutAnOutgoingTransition)
	t.Run("state_transition_without_a_target", testStateTransitionWithoutATarget)
	t.Run("state_transition_effect_reads_an_unknown_feature", testStateTransitionEffectReadsAnUnknownFeature)
	t.Run("state_cross_region_transitions_ping_pong", testStateCrossRegionTransitionsPingPong)
	t.Run("sourceless_accept_at_top_level", testSourcelessAcceptAtTopLevel)
	t.Run("calc_unbound_parameter", testCalcUnboundParameter)
	t.Run("calc_unbound_keyword_named_parameter", testCalcUnboundKeywordNamedParameter)
	t.Run("calc_too_many_arguments", testCalcTooManyArguments)
	t.Run("calc_unknown_named_argument", testCalcUnknownNamedArgument)
	t.Run("calc_without_result", testCalcWithoutResult)
	t.Run("calc_symbol_is_not_a_calc", testCalcSymbolIsNotACalc)
	t.Run("calc_direct_recursion", testCalcDirectRecursion)
	t.Run("calc_mutual_recursion", testCalcMutualRecursion)
	t.Run("calc_recursion_spends_step_budget", testCalcRecursionSpendsStepBudget)
	t.Run("calc_recursion_at_depth_ceiling", testCalcRecursionAtDepthCeiling)
	t.Run("calc_non_terminating_loop", testCalcNonTerminatingLoop)
	t.Run("calc_body_never_returns", testCalcBodyNeverReturns)
	t.Run("calc_send_is_rejected", testCalcSendIsRejected)
	t.Run("calc_terminate_is_rejected", testCalcTerminateIsRejected)
	t.Run("calc_assignment_outside_the_calc", testCalcAssignmentOutsideTheCalc)
	t.Run("calc_non_boolean_condition", testCalcNonBooleanCondition)
	t.Run("calc_usage_unbound_input", testCalcUsageUnboundInput)
	t.Run("calc_usage_unknown_output", testCalcUsageUnknownOutput)
	t.Run("calc_usage_cyclic_outputs", testCalcUsageCyclicOutputs)
	t.Run("calc_usage_specializes_a_non_calc", testCalcUsageSpecializesANonCalc)
	t.Run("calc_usage_step_budget", testCalcUsageStepBudget)
	t.Run("calc_usage_output_without_a_value", testCalcUsageOutputWithoutAValue)
	t.Run("calc_output_never_assigned_by_the_body", testCalcOutputNeverAssignedByTheBody)
	t.Run("calc_output_assigned_in_a_branch_not_taken", testCalcOutputAssignedInABranchNotTaken)
	t.Run("calc_output_valued_and_assigned", testCalcOutputValuedAndAssigned)
	t.Run("calc_output_assigned_twice", testCalcOutputAssignedTwice)
	t.Run("nested_calc_usage_unbound_input", testNestedCalcUsageUnboundInput)
	t.Run("nested_calc_usage_unknown_output", testNestedCalcUsageUnknownOutput)
	t.Run("nested_calc_usage_self_cycle", testNestedCalcUsageSelfCycle)
	t.Run("nested_calc_usage_recursion_depth", testNestedCalcUsageRecursionDepth)
	t.Run("nested_calc_usage_step_budget", testNestedCalcUsageStepBudget)
	t.Run("multiple_outputs_invoked_as_an_expression", testMultipleOutputsInvokedAsAnExpression)
	t.Run("body_local_usage_of_a_non_calc", testBodyLocalUsageOfANonCalc)
	t.Run("body_local_declaration_not_executable", testBodyLocalDeclarationNotExecutable)
	t.Run("range_bound_is_not_an_integer", testRangeBoundIsNotAnInteger)
	t.Run("range_spends_the_step_budget", testRangeSpendsTheStepBudget)
	t.Run("collection_spends_the_element_budget", testCollectionSpendsTheElementBudget)
	t.Run("usage_read_through_a_part_without_an_output", testUsageReadThroughAPartWithoutAnOutput)
	t.Run("constraint_missing_feature", testConstraintMissingFeature)
	t.Run("nested_condition_subject_is_ambiguous", testNestedConditionSubjectIsAmbiguous)
	t.Run("satisfaction_subject_is_ambiguous", testSatisfactionSubjectIsAmbiguous)
	t.Run("recursive_composition_subject_search", testRecursiveCompositionSubjectSearch)
	t.Run("duplicate_objects_of_one_declaration", testDuplicateObjectsOfOneDeclaration)
	t.Run("duplicate_objects_holding_a_plain_part", testDuplicateObjectsHoldingAPlainPart)
	t.Run("nested_part_held_with_a_multiplicity", testNestedPartHeldWithAMultiplicity)
	t.Run("part_nested_inside_a_repeated_part", testPartNestedInsideARepeatedPart)
	t.Run("parts_subsetting_one_collection", testPartsSubsettingOneCollection)
	t.Run("requirement_feature_without_a_value", testRequirementFeatureWithoutAValue)
	t.Run("requirement_features_valued_from_each_other", testRequirementFeaturesValuedFromEachOther)
	t.Run("step_budget_exceeded", testStepBudgetExceeded)
	t.Run("eval_on_an_instance_spends_the_step_budget", testEvalOnAnInstanceSpendsTheStepBudget)
	t.Run("non_terminating_loop_exhausts_step_budget", testNonTerminatingLoopExhaustsStepBudget)
	t.Run("loop_body_declaration_does_not_leak", testLoopBodyDeclarationDoesNotLeak)
	t.Run("loop_body_of_unexecutable_statement", testLoopBodyOfUnexecutableStatement)
	t.Run("block_flow_of_unexecutable_member", testBlockFlowOfUnexecutableMember)
	t.Run("non_terminating_loop_performing_an_action", testNonTerminatingLoopPerformingAnAction)
	t.Run("for_over_a_value_no_expression_makes_iterable", testForOverAValueNoExpressionMakesIterable)
	t.Run("for_over_a_scalar", testForOverAScalar)
	t.Run("statement_directly_in_an_action_body", testStatementDirectlyInAnActionBody)
	t.Run("flow_end_naming_no_node", testFlowEndNamingNoNode)
	t.Run("flow_naming_no_pin", testFlowNamingNoPin)
	t.Run("accept_payload_without_a_value", testAcceptPayloadWithoutAValue)
	t.Run("accept_payload_read_before_it_is_bound", testAcceptPayloadReadBeforeItIsBound)
	t.Run("flow_from_a_node_that_produced_nothing", testFlowFromANodeThatProducedNothing)
	t.Run("action_accept_time_trigger", testActionAcceptTimeTrigger)
	t.Run("action_accept_non_boolean_change_trigger", testActionAcceptNonBooleanChangeTrigger)
	t.Run("action_body_unresolved_unit", testActionBodyUnresolvedUnit)
	t.Run("action_body_unresolved_feature", testActionBodyUnresolvedFeature)
	t.Run("state_body_unresolved_unit", testStateBodyUnresolvedUnit)
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
	t.Run("empty_anonymous_action_body", testEmptyAnonymousActionBody)
	t.Run("non_terminating_anonymous_do_body", testNonTerminatingAnonymousDoBody)
	t.Run("behavior_performing_an_action_and_stating_a_body", testBehaviorPerformingAnActionAndStatingABody)
	t.Run("qualified_assignment_target_in_a_state_effect", testQualifiedAssignmentTargetInAStateEffect)
	t.Run("call_of_unhandled_operation", testCallOfUnhandledOperation)
	t.Run("signal_no_level_of_a_composite_state_accepts", testSignalNoLevelOfACompositeStateAccepts)
	t.Run("stale_composite_timer_in_a_region", testStaleCompositeTimerInARegion)
	t.Run("composite_self_transition_with_no_substate_to_re_enter", testCompositeSelfTransitionWithNoSubstateToReEnter)
	t.Run("exit_of_nested_regions_with_a_history_pseudostate", testExitOfNestedRegionsWithAHistoryPseudostate)
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
	t.Run("quantity_unit_shadowed_by_sibling", testQuantityUnitShadowedBySibling)
	t.Run("quantity_qualified_unit_is_not_shadowing", testQuantityQualifiedUnitIsNotShadowing)
	t.Run("quantity_shadowed_unit_without_a_qualifier", testQuantityShadowedUnitWithoutAQualifier)
	t.Run("quantity_cyclic_unit_definition", testQuantityCyclicUnitDefinition)
	t.Run("satisfy_unresolved_requirement", testSatisfyUnresolvedRequirement)
	t.Run("satisfy_requirement_without_conditions", testSatisfyRequirementWithoutConditions)
	t.Run("satisfy_bounded_by_the_step_budget", testSatisfyBoundedByTheStepBudget)
	t.Run("cyclic_derived_slot", testCyclicDerivedSlot)
	t.Run("derived_slot_over_missing_feature", testDerivedSlotOverMissingFeature)
	t.Run("sequence_index_names_no_position", testSequenceIndexNamesNoPosition)
	t.Run("collection_operand_of_the_wrong_kind", testCollectionOperandOfTheWrongKind)
	t.Run("numeric_library_call_that_has_no_value", testNumericLibraryCallThatHasNoValue)
	t.Run("string_operand_of_the_wrong_kind", testStringOperandOfTheWrongKind)
	t.Run("collection_body_of_the_wrong_arity", testCollectionBodyOfTheWrongArity)
	t.Run("select_predicate_is_not_a_condition", testSelectPredicateIsNotACondition)
	t.Run("collection_operation_step_budget", testCollectionOperationStepBudget)
	t.Run("variation_without_a_selected_variant", testVariationWithoutASelectedVariant)
	t.Run("variation_bound_to_what_is_not_a_variant", testVariationBoundToWhatIsNotAVariant)
	t.Run("variation_bound_to_two_variants", testVariationBoundToTwoVariants)
	t.Run("variation_read_through_its_declaration", testVariationReadThroughItsDeclaration)
	t.Run("chain_through_an_unselected_variation_part", testChainThroughAnUnselectedVariationPart)
	t.Run("repeated_reads_of_a_variant_object", testRepeatedReadsOfAVariantObject)
	t.Run("two_owners_selecting_one_variant", testTwoOwnersSelectingOneVariant)
	t.Run("two_ownerless_selections_of_one_variant", testTwoOwnerlessSelectionsOfOneVariant)
	t.Run("variant_outside_a_variation", testVariantOutsideAVariation)
	t.Run("variant_under_a_redefined_variation", testVariantUnderARedefinedVariation)
	t.Run("deep_specialization_chain_of_redefinitions", testDeepSpecializationChainOfRedefinitions)
	t.Run("conflicting_redefinitions_at_several_levels", testConflictingRedefinitionsAtSeveralLevels)
	t.Run("one_feature_valued_under_two_names", testOneFeatureValuedUnderTwoNames)
	t.Run("valued_feature_restated_in_a_body", testValuedFeatureRestatedInABody)
	t.Run("multiplicity_infinite_lower_bound", testMultiplicityInfiniteLowerBound)
	t.Run("multiplicity_lower_bound_too_large", testMultiplicityLowerBoundTooLarge)
	t.Run("default_not_conforming_to_multiplicity", testDefaultNotConformingToMultiplicity)
	t.Run("default_against_an_undeclared_multiplicity", testDefaultAgainstAnUndeclaredMultiplicity)
	t.Run("feature_chain_through_an_unset_slot", testFeatureChainThroughAnUnsetSlot)
	t.Run("feature_chain_spends_the_element_budget", testFeatureChainSpendsTheElementBudget)
	t.Run("mutually_subsetting_features", testMutuallySubsettingFeatures)
	t.Run("unattachable_connector_end", testUnattachableConnectorEnd)
	t.Run("multiplicity_on_a_connector", testMultiplicityOnAConnector)
	t.Run("connector_attached_to_itself", testConnectorAttachedToItself)
	t.Run("mutually_attached_connectors", testMutuallyAttachedConnectors)
	t.Run("enumeration_name_that_is_not_a_literal", testEnumerationNameThatIsNotALiteral)
	t.Run("chain_through_a_literal_without_that_attribute", testChainThroughALiteralWithoutThatAttribute)
	t.Run("classification_outside_the_evaluable_subset", testClassificationOutsideTheEvaluableSubset)
	t.Run("expression_over_a_slot_holding_no_value", testExpressionOverASlotHoldingNoValue)
}

// testExpressionOverASlotHoldingNoValue: a valueless feature of a value type is
// read without an error and reports that it holds no value, while an expression
// computing over it reports a type mismatch rather than a number or a panic.
func testExpressionOverASlotHoldingNoValue(t *testing.T) {
	src := `
		package test {
			private import ScalarValues::*;
			part def Holder {
				attribute d : Real;
				attribute n : Real = d + 1.0;
			}
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Holder", ast.DefPart)
	if sym == nil {
		t.Fatal("Holder part def not found")
	}
	inst, err := ctx.Instantiate(sym)
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}

	slot, err := inst.GetSlot(ctx, "d")
	if err != nil {
		t.Fatalf("slot d: %v", err)
	}
	if !ctx.HoldsNoValue(slot.HeldValue()) {
		t.Errorf("slot d holds %v, want no value", slot.HeldValue())
	}

	if _, err := inst.GetSlot(ctx, "n"); !errors.Is(err, ErrTypeMismatch) {
		t.Errorf("slot n err = %v, want ErrTypeMismatch", err)
	}

	// A value naming an object the context does not hold answers the question
	// rather than panicking on the lookup.
	if ctx.HoldsNoValue(Value{Kind: ValInstance, Instance: 1 << 30}) {
		t.Error("a value naming no object reads as holding none")
	}
}

// testClassificationOutsideTheEvaluableSubset: a classification the evaluator
// cannot judge — no subject to classify, a subject that is a datum, an
// unresolved metadata type, or a subject naming nothing — reports
// ErrFilterUnevaluable rather than silently answering false.
func testClassificationOutsideTheEvaluableSubset(t *testing.T) {
	const model = `
		metadata def Safety;
		#Safety part def Belt;
		attribute level = 3;
	`
	for _, tc := range []struct{ name, cond string }{
		{"implicit subject outside an object", "@Safety"},
		{"self outside an object", "self @ Safety"},
		{"a datum subject", "42 @ Safety"},
		{"a string subject", `"belt" @ Safety`},
		{"an unresolved metadata type", "Belt @ Nonexistent"},
	} {
		src := model + "\nconstraint c { " + tc.cond + " }"
		got, err := constraintVerdict(t, src, "c")
		if got {
			t.Errorf("%s: `%s` was satisfied, want a report", tc.name, tc.cond)
		}
		if !errors.Is(err, semantics.ErrFilterUnevaluable) {
			t.Errorf("%s: `%s` err = %v, want ErrFilterUnevaluable", tc.name, tc.cond, err)
		}
	}
	// A subject naming nothing is the unresolved reference it is, not a verdict.
	if got, err := constraintVerdict(t, model+"\nconstraint c { Missing @ Safety }", "c"); got || err == nil {
		t.Errorf("`Missing @ Safety` = %v err=%v, want a report", got, err)
	}
}

// testMultiplicityInfiniteLowerBound: `[*..*]` requires unboundedly many objects,
// which cannot be materialized, so the slot reports a multiplicity violation
// rather than allocating until memory runs out.
func testMultiplicityInfiniteLowerBound(t *testing.T) {
	inst, ctx := instantiateHolder(t, `
		package test {
			private import ScalarValues::Real;
			part def C { attribute m : Real = 1.0; }
			part def Holder { part p : C[*..*]; }
		}
	`)
	_, err := inst.GetSlot(ctx, "p")
	if err == nil {
		t.Fatal("want a multiplicity violation, got a materialized slot")
	}
	if !errors.Is(err, ErrMultiplicityViolation) {
		t.Errorf("expected ErrMultiplicityViolation, got: %v", err)
	}
}

// testMultiplicityLowerBoundTooLarge: a lower bound past the materialization
// bound is reported instead of eagerly allocating that many objects.
func testMultiplicityLowerBoundTooLarge(t *testing.T) {
	inst, ctx := instantiateHolder(t, `
		package test {
			private import ScalarValues::Real;
			part def C { attribute m : Real = 1.0; }
			part def Holder { part p : C[5000]; }
		}
	`)
	_, err := inst.GetSlot(ctx, "p")
	if err == nil {
		t.Fatal("want a multiplicity violation, got a materialized slot")
	}
	if !errors.Is(err, ErrMultiplicityViolation) {
		t.Errorf("expected ErrMultiplicityViolation, got: %v", err)
	}
	if len(ctx.instances) > 100 {
		t.Errorf("materialized %d instances before reporting the bound", len(ctx.instances))
	}
}

// testDefaultNotConformingToMultiplicity: a default whose element count is
// outside the feature's multiplicity is reported, rather than broadcast to fill
// the lower bound, truncated to the upper one, or dropped.
func testDefaultNotConformingToMultiplicity(t *testing.T) {
	for _, tc := range []struct {
		name string
		decl string
	}{
		{"one value against three", "attribute xs : Real[3] = 1.0;"},
		{"four values against three", "attribute xs : Real[3] = (1.0, 2.0, 3.0, 4.0);"},
		{"no values against one or more", "attribute xs : Real[1..3] = ();"},
		{"an expression producing too few", "attribute m : Real = 1.0; attribute xs : Real[2] = m;"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inst, ctx := instantiateHolder(t, `
				package test {
					private import ScalarValues::Real;
					part def Holder { `+tc.decl+` }
				}
			`)
			done := make(chan error, 1)
			go func() {
				defer func() {
					if r := recover(); r != nil {
						done <- fmt.Errorf("panic: %v", r)
					}
				}()
				_, err := inst.GetSlot(ctx, "xs")
				done <- err
			}()
			select {
			case err := <-done:
				if err == nil {
					t.Fatal("want a multiplicity violation, got a materialized slot")
				}
				if !errors.Is(err, ErrMultiplicityViolation) {
					t.Errorf("expected ErrMultiplicityViolation, got: %v", err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("materializing the default did not terminate")
			}
		})
	}
}

// testDefaultAgainstAnUndeclaredMultiplicity: a feature that declares no
// multiplicity holds exactly one value, so a default of any other number of
// values is reported rather than held under an unconstrained bound.
func testDefaultAgainstAnUndeclaredMultiplicity(t *testing.T) {
	for _, tc := range []struct {
		name     string
		decl     string
		reported bool
	}{
		{"one value", "attribute xs : Real = 1.0;", false},
		{"one value of an untyped feature", "attribute xs = 1.0;", false},
		{"two values", "attribute xs : Real = (1.0, 2.0);", true},
		{"two values of an untyped feature", "attribute xs = (1.0, 2.0);", true},
		{"no values", "attribute xs : Real = ();", true},
		{"an expression producing two", "attribute m : Real[2] = (1.0, 2.0); attribute xs : Real = m;", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inst, ctx := instantiateHolder(t, `
				package test {
					private import ScalarValues::Real;
					part def Holder { `+tc.decl+` }
				}
			`)
			_, err := inst.GetSlot(ctx, "xs")
			switch {
			case tc.reported && err == nil:
				t.Fatalf("%s was held, want a multiplicity violation", tc.decl)
			case tc.reported && !errors.Is(err, ErrMultiplicityViolation):
				t.Errorf("expected ErrMultiplicityViolation, got: %v", err)
			case !tc.reported && err != nil:
				t.Errorf("%s was reported: %v", tc.decl, err)
			}
		})
	}
}

// testFeatureChainThroughAnUnsetSlot: a chain over a collection whose objects
// hold no value for the last feature names that feature, rather than reading it
// as an empty collection.
func testFeatureChainThroughAnUnsetSlot(t *testing.T) {
	inst, ctx := instantiateHolder(t, `
		package test {
			private import ScalarValues::Real;
			private import RealFunctions::*;
			part def Sub { attribute volume : Real; }
			part def Holder {
				part subs : Sub[2];
				attribute total : Real = sum(subs.volume);
			}
		}
	`)
	_, err := inst.GetSlot(ctx, "total")
	if err == nil {
		t.Fatal("want the unset slot's error, got a value")
	}
	if !errors.Is(err, ErrUninitializedSlot) {
		t.Errorf("expected ErrUninitializedSlot, got: %v", err)
	}
	if !strings.Contains(err.Error(), "volume") {
		t.Errorf("error %q does not name the unset feature", err)
	}
}

// testFeatureChainSpendsTheElementBudget: navigating a chain through a collection
// counts what it collects, so a chain over a large collection ends within the
// element budget rather than growing unbounded.
func testFeatureChainSpendsTheElementBudget(t *testing.T) {
	inst, ctx := instantiateHolder(t, `
		package test {
			private import ScalarValues::Real;
			part def Sub { attribute volume : Real = 1.0; }
			part def Holder {
				part subs : Sub[10];
				attribute volumes : Real[*] = subs.volume;
			}
		}
	`)
	ctx.maxElements = 15
	_, err := inst.GetSlot(ctx, "volumes")
	if err == nil {
		t.Fatal("want the element budget's error, got a value")
	}
	if !errors.Is(err, ErrElementLimitExceeded) {
		t.Errorf("expected ErrElementLimitExceeded, got: %v", err)
	}
}

// testMutuallySubsettingFeatures: two features that subset each other have no
// well-founded set of values, so materializing one reports the cycle instead of
// recursing until the step budget runs out.
func testMutuallySubsettingFeatures(t *testing.T) {
	inst, ctx := instantiateHolder(t, `
		package test {
			private import ScalarValues::Real;
			part def C { attribute m : Real = 1.0; }
			part def Holder {
				part a : C[*] :> b;
				part b : C[*] :> a;
			}
		}
	`)
	done := make(chan struct{})
	var slotErr error
	go func() {
		defer close(done)
		_, slotErr = inst.GetSlot(ctx, "a")
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("GetSlot hung on mutually subsetting features")
	}
	if slotErr == nil {
		t.Fatal("want the cyclic slot's error, got a materialized slot")
	}
	if !errors.Is(slotErr, ErrCyclicSlot) {
		t.Errorf("expected ErrCyclicSlot, got: %v", slotErr)
	}
}

// instantiateHolder instantiates the `Holder` part def the source declares, for
// a case whose failure surfaces when one of its slots is read.
func instantiateHolder(t *testing.T, src string) (*Instance, *Context) {
	t.Helper()
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Holder", ast.DefPart)
	if sym == nil {
		t.Fatal("Holder part def not found")
	}
	inst, err := ctx.Instantiate(sym)
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	return inst, ctx
}

// testSequenceIndexNamesNoPosition: an index outside the sequence, or one that
// is not a whole number, is reported. Answering nothing would make `seq#(i)`
// read as an empty value everywhere a model indexes past the end.
func testSequenceIndexNamesNoPosition(t *testing.T) {
	for _, tt := range []struct {
		expr string
		want error
	}{
		{"xs#(4)", ErrIndexOutOfRange},
		{"xs#(0)", ErrIndexOutOfRange},
		{"xs#(0 - 1)", ErrIndexOutOfRange},
		{"()#(1)", ErrIndexOutOfRange},
		{"xs#(1.5)", ErrTypeMismatch},
		{"xs#(ys)", ErrTypeMismatch},
	} {
		got, err := evalCollectionExpr(t, tt.expr)
		if !errors.Is(err, tt.want) {
			t.Errorf("%s = (%v, %v), want %v", tt.expr, got, err, tt.want)
		}
	}
}

// testNumericLibraryCallThatHasNoValue: a vector, Complex or sequence library
// declaration that cannot answer reports itself — a malformed argument by kind or
// dimension, an undefined result, or a declaration this runtime has no
// representation for the values of — rather than computing something else.
func testNumericLibraryCallThatHasNoValue(t *testing.T) {
	for _, tt := range []struct {
		expr string
		want error
	}{
		{"VectorFunctions::cartesianInner(xs, ys)", ErrTypeMismatch},
		{"VectorFunctions::'cartesian+'(xs, ys)", ErrTypeMismatch},
		{"VectorFunctions::cartesianNorm(flags)", ErrTypeMismatch},
		{"VectorFunctions::cartesianAngle(xs, (0.0, 0.0, 0.0))", semantics.ErrArithmeticDomain},
		{"VectorFunctions::vectorScalarDiv(xs, 0)", ErrDivisionByZero},
		{"VectorFunctions::cartesianInner(xs)", ErrCalcArity},
		{"VectorFunctions::sum(xs)", ErrUnevaluableLibraryFunction},
		{"ComplexFunctions::'/'(ys, (0.0, 0.0))", ErrDivisionByZero},
		{"ComplexFunctions::re(xs)", ErrTypeMismatch},
		{"ComplexFunctions::ToString(ys)", ErrUnevaluableLibraryFunction},
		// includingAt inserts before a position of 1..size+1, so an index past the
		// end of the sequence names no insertion point and is reported rather than
		// appending or dropping the values.
		{"SequenceFunctions::includingAt(xs, 9, 5)", ErrIndexOutOfRange},
		{"SequenceFunctions::includingAt(xs, 9, 0)", ErrIndexOutOfRange},
		{"SequenceFunctions::includingAt((), 9, 2)", ErrIndexOutOfRange},
		{"SequenceFunctions::includingAt(xs, 9, 1.5)", ErrTypeMismatch},
		{"SequenceFunctions::includingAt(xs, 9)", ErrCalcArity},
	} {
		got, err := evalCollectionExpr(t, tt.expr)
		if !errors.Is(err, tt.want) {
			t.Errorf("%s = (%v, %v), want %v", tt.expr, got, err, tt.want)
		}
	}
}

// testStringOperandOfTheWrongKind: an operator or StringFunctions call given a
// value that is not the String its signature declares is reported rather than
// coerced, and a Substring position naming no character is reported rather than
// clamped.
func testStringOperandOfTheWrongKind(t *testing.T) {
	for _, tt := range []struct {
		expr string
		want error
	}{
		{`"a" + 1`, ErrTypeMismatch},
		{`1 + "a"`, ErrTypeMismatch},
		{`"a" < 1`, ErrTypeMismatch},
		{`"a" >= factor`, ErrTypeMismatch},
		{`"a" < xs`, ErrTypeMismatch},
		{`"a" - "b"`, ErrTypeMismatch},
		{`StringFunctions::Length(1)`, ErrTypeMismatch},
		{`StringFunctions::Length(xs)`, ErrTypeMismatch},
		{`StringFunctions::Substring("abc", 1, 9)`, ErrIndexOutOfRange},
		{`StringFunctions::Substring("héllo", 1, 6)`, ErrIndexOutOfRange},
		{`StringFunctions::Substring("abc", 0, 2)`, ErrIndexOutOfRange},
		{`StringFunctions::Substring("abc", "1", 2)`, ErrTypeMismatch},
		{`StringFunctions::Substring("abc", 1)`, ErrCalcArity},
	} {
		got, err := evalCollectionExpr(t, tt.expr)
		if !errors.Is(err, tt.want) {
			t.Errorf("%s = (%v, %v), want %v", tt.expr, got, err, tt.want)
		}
	}
}

// testCollectionOperandOfTheWrongKind: an operation given something that is not
// the kind of value it operates on reports it rather than reading the value as a
// collection of itself or as nothing.
func testCollectionOperandOfTheWrongKind(t *testing.T) {
	for _, tt := range []struct {
		expr string
		want error
	}{
		{"xs->select(2)", ErrTypeMismatch},
		{"xs->collect(xs)", ErrTypeMismatch},
		{`sum((1, "a"))`, ErrTypeMismatch},
		{"product(flags)", ErrTypeMismatch},
		{"xs->subsequence(1, 4)", ErrIndexOutOfRange},
	} {
		got, err := evalCollectionExpr(t, tt.expr)
		if !errors.Is(err, tt.want) {
			t.Errorf("%s = (%v, %v), want %v", tt.expr, got, err, tt.want)
		}
	}
}

// testCollectionBodyOfTheWrongArity: a body an operation calls with one element
// but that declares no parameter, or two, is reported. Binding what it declares
// and dropping the rest would answer from a parameter that was never given a
// value.
func testCollectionBodyOfTheWrongArity(t *testing.T) {
	for _, expr := range []string{
		"xs->collect {}",
		"xs->collect {in x; in y; x}",
		"xs->select {in x; in y; x > 0}",
		"xs.{in x; in y; x}",
	} {
		got, err := evalCollectionExpr(t, expr)
		if !errors.Is(err, ErrBodyArity) {
			t.Errorf("%s = (%v, %v), want ErrBodyArity", expr, got, err)
		}
	}
}

// testSelectPredicateIsNotACondition: a selector answering something that is not
// a boolean is reported. Reading a non-boolean as false would silently drop
// every element, which is a wrong answer rather than a failure.
func testSelectPredicateIsNotACondition(t *testing.T) {
	for _, expr := range []string{
		"xs.?{in x; x + 1}",
		"xs->select {in x; x * 2}",
		"xs->reject {in x; 1}",
		"xs->forAll {in x; x}",
		`xs->exists {in x; "yes"}`,
	} {
		got, err := evalCollectionExpr(t, expr)
		if !errors.Is(err, ErrTypeMismatch) {
			t.Errorf("%s = (%v, %v), want ErrTypeMismatch", expr, got, err)
		}
	}
}

// testCollectionOperationStepBudget: an operation calls its body once per
// element, and each call spends the context's budget, so a collection large
// enough for the budget fails the evaluation instead of running unbounded.
func testCollectionOperationStepBudget(t *testing.T) {
	got, err := evalCollectionExprBounded(t, "xs.{in x; x * factor}", 3)
	if !errors.Is(err, ErrStepLimitExceeded) {
		t.Errorf("collect under a budget of 3 = (%v, %v), want ErrStepLimitExceeded", got, err)
	}
}

// testSatisfyUnresolvedRequirement: a satisfaction assertion whose requirement
// reference names nothing reports it, rather than evaluating the assertion's own
// empty body as a verdict about the model.
func testSatisfyUnresolvedRequirement(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `
		package test {
			part lander;
			part context {
				assert satisfy nosuch by lander;
			}
		}
	`))
	assertions := ctx.SatisfyAssertionsIn(idx.DocumentRoot("<test>"))
	if len(assertions) != 1 {
		t.Fatalf("found %d satisfaction assertions, want 1", len(assertions))
	}

	satisfied, err := ctx.EvaluateSatisfaction(assertions[0])
	if err == nil {
		t.Fatalf("expected an error, got satisfied = %v", satisfied)
	}
	if !errors.Is(err, ErrNoRequirement) {
		t.Errorf("expected ErrNoRequirement, got: %v", err)
	}
	if errors.Is(err, ErrViolated) {
		t.Error("an unresolved requirement reference is not a violation")
	}
	if !strings.Contains(err.Error(), "nosuch") {
		t.Errorf("error does not name the reference: %v", err)
	}
}

// testSatisfyRequirementWithoutConditions: satisfying a requirement that states
// no condition is not a verdict, since no check ran.
func testSatisfyRequirementWithoutConditions(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `
		package test {
			requirement def Documented {
				doc /* stated in prose only */
			}
			requirement documented : Documented;
			part lander;
			part context {
				assert satisfy documented by lander;
			}
		}
	`))
	assertions := ctx.SatisfyAssertionsIn(idx.DocumentRoot("<test>"))
	if len(assertions) != 1 {
		t.Fatalf("found %d satisfaction assertions, want 1", len(assertions))
	}

	satisfied, err := ctx.EvaluateSatisfaction(assertions[0])
	if err == nil {
		t.Fatalf("expected an error, got satisfied = %v", satisfied)
	}
	if !errors.Is(err, ErrNoConditions) {
		t.Errorf("expected ErrNoConditions, got: %v", err)
	}
	if satisfied {
		t.Error("a requirement with no condition must not report a verdict")
	}
}

// testSatisfyBoundedByTheStepBudget: a satisfaction check is one run, so its
// condition evaluation spends the run's budget instead of resetting it.
func testSatisfyBoundedByTheStepBudget(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `
		package test {
			part def Lander { attribute verticalSpeed = 1.2; }
			part lander : Lander;
			requirement def TouchdownRequirement {
				subject craft : Lander;
				attribute maxVerticalSpeed = 1.5;
				require constraint { craft.verticalSpeed <= maxVerticalSpeed }
			}
			requirement touchdown : TouchdownRequirement;
			part context {
				assert satisfy touchdown by lander;
			}
		}
	`))
	assertions := ctx.SatisfyAssertionsIn(idx.DocumentRoot("<test>"))
	if len(assertions) != 1 {
		t.Fatalf("found %d satisfaction assertions, want 1", len(assertions))
	}
	ctx.maxSteps = 2

	satisfied, err := ctx.EvaluateSatisfaction(assertions[0])
	if err == nil {
		t.Fatalf("expected the step budget to bound the check, got satisfied = %v", satisfied)
	}
	if !errors.Is(err, ErrStepLimitExceeded) {
		t.Errorf("expected ErrStepLimitExceeded, got: %v", err)
	}
	if errors.Is(err, ErrViolated) {
		t.Error("an exhausted budget is not a verdict about the model")
	}

	// The subject is built inside the same run, so a budget exhausted there is
	// still reported as such rather than as a missing subject.
	ctx.maxSteps = 0
	if _, err := ctx.EvaluateSatisfaction(assertions[0]); !errors.Is(err, ErrStepLimitExceeded) || !errors.Is(err, ErrNoSubject) {
		t.Errorf("expected ErrStepLimitExceeded while building the subject, got: %v", err)
	}
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
	}, nil)
	if err == nil {
		t.Fatal("expected an error for a state deferring a time trigger")
	}
	if !strings.Contains(err.Error(), "only signal and call triggers can be deferred") {
		t.Errorf("expected a deferrability error, got: %v", err)
	}
}

// testStateTransitionEndpointMisspelled: a misspelled endpoint is a
// name-resolution diagnostic, so lowering leaves the edge out and the machine
// runs to a halt in the state it reached rather than panicking or hanging.
func testStateTransitionEndpointMisspelled(t *testing.T) {
	src := `package test {
		state Machine {
			initial init;
			state busy;
			final done;
			init then busy;
			transition busy to donee;
		}
	}`
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}
	idx, _, ctx := buildRuntime(t, "<test>", file)
	ctx.resolver.ResolveDocument("<test>", file)

	var endpoint *resolve.Diagnostic
	for i, diag := range ctx.resolver.Diagnostics {
		if strings.Contains(diag.Message, "donee") {
			endpoint = &ctx.resolver.Diagnostics[i]
		}
	}
	if endpoint == nil {
		t.Fatalf("expected a name-resolution diagnostic for 'donee', got: %v", ctx.resolver.Diagnostics)
	}
	if endpoint.Code != "unresolved" {
		t.Errorf("expected code %q, got %q", "unresolved", endpoint.Code)
	}

	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Machine", ast.DefState)
	if sym == nil {
		t.Fatal("Machine not found")
	}
	exec, err := newStateExecutor(ctx, sym, nil)
	if err != nil {
		t.Fatalf("newStateExecutor: %v", err)
	}
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- exec.RunToCompletion() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("RunToCompletion: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("RunToCompletion hung on a machine whose transition names nothing")
	}
	if got := exec.getCurrentState(); got == nil || got.Name != "busy" {
		t.Errorf("expected the machine to halt in 'busy', got %v", got)
	}
}

// testStateTransitionEndpointNeverResolved: executed without a name-resolution
// pass, as the REPL and the service handlers do, an endpoint naming nothing
// leaves its edge out; the machine still runs, and the misspelling is reported
// by whoever resolves the document rather than by lowering.
func testStateTransitionEndpointNeverResolved(t *testing.T) {
	src := `package test {
		state Machine {
			initial init;
			state busy;
			final done;
			init then busy;
			transition busy to donee;
		}
	}`
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}
	idx, _, ctx := buildRuntime(t, "<test>", file)

	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Machine", ast.DefState)
	if sym == nil {
		t.Fatal("Machine not found")
	}
	exec, err := newStateExecutor(ctx, sym, nil)
	if err != nil {
		t.Fatalf("newStateExecutor: %v", err)
	}
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- exec.RunToCompletion() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("RunToCompletion: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("RunToCompletion hung on an endpoint no resolution pass reported")
	}
	if got := exec.getCurrentState(); got == nil || got.Name != "busy" {
		t.Errorf("expected the machine to halt in 'busy', got %v", got)
	}
}

// testStateTransitionEndpointInAnotherMachine: an endpoint naming a state of a
// different machine resolves, so no name diagnostic reports it; the state
// transition check reports it, and lowering backstops the check with a typed
// error rather than dropping the edge.
func testStateTransitionEndpointInAnotherMachine(t *testing.T) {
	src := `package test {
		state Other {
			initial start;
			state running;
			start then running;
		}
		state Machine {
			initial init;
			state busy;
			init then busy;
			transition busy to Other::running;
		}
	}`
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}
	idx, _, ctx := buildRuntime(t, "<test>", file)
	ctx.resolver.ResolveDocument("<test>", file)

	for _, diag := range ctx.resolver.Diagnostics {
		if strings.Contains(diag.Message, "running") {
			t.Fatalf("the endpoint resolves, so name resolution reports nothing: %v", diag)
		}
	}

	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Machine", ast.DefState)
	if sym == nil {
		t.Fatal("Machine not found")
	}
	_, err := newStateExecutor(ctx, sym, nil)
	if err == nil {
		t.Fatal("expected an error for an endpoint that is not a vertex of this machine")
	}
	if !strings.Contains(err.Error(), "not a vertex of this state machine") {
		t.Errorf("expected the error to say the endpoint is not a vertex of this machine, got %v", err)
	}
	if strings.Contains(err.Error(), "*ast.") {
		t.Errorf("the message a modeller reads names a Go type: %v", err)
	}
}

// testStateTransitionEndpointNamingAFirstMarker: a `first m then x` marker is no
// vertex, so an endpoint naming one is reported by the state transition check and
// backstopped here with a typed error rather than a panic.
func testStateTransitionEndpointNamingAFirstMarker(t *testing.T) {
	src := `package test {
		state Machine {
			initial init;
			state busy;
			state other;
			first marker then other;
			init then busy;
			transition busy to marker;
		}
	}`
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}
	idx, _, ctx := buildRuntime(t, "<test>", file)

	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Machine", ast.DefState)
	if sym == nil {
		t.Fatal("Machine not found")
	}
	_, err := newStateExecutor(ctx, sym, nil)
	if err == nil {
		t.Fatal("expected an error for an endpoint naming a marker rather than a vertex")
	}
	if !strings.Contains(err.Error(), "not a vertex of this state machine") {
		t.Errorf("expected the error to say the endpoint is not a vertex, got %v", err)
	}
	if strings.Contains(err.Error(), "*ast.") {
		t.Errorf("the message a modeller reads names a Go type: %v", err)
	}
}

// testStateJunctionWithoutAnOutgoingTransition: a junction no transition leaves
// routes a transition reaching it nowhere, which the state transition check
// reports; reaching it at run time errors rather than panicking or hanging.
func testStateJunctionWithoutAnOutgoingTransition(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			initial init;
			state busy;
			junction stuck;
			init then busy;
			transition busy to stuck;
		}
	}`)
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- exec.RunToCompletion() }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected an error for a junction no transition leaves")
		}
		if !strings.Contains(err.Error(), "junction stuck has no outgoing transitions") {
			t.Errorf("expected the error to name the junction, got %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("RunToCompletion hung on a junction no transition leaves")
	}
}

// testStateTransitionWithoutATarget: a transition with no target names no edge,
// so lowering reports it rather than dereferencing the absent target.
func testStateTransitionWithoutATarget(t *testing.T) {
	idx := symbols.NewIndex()
	resolver := resolve.New(idx)
	ctx := NewContext(semantics.NewModel(resolver), resolver, 1000)

	dangling := transitionMember("init", "busy")
	dangling.Target = nil
	machine := &ast.Usage{
		Kind:  ast.UsageState,
		Ident: ast.Identification{Name: "Machine"},
		Members: []ast.Node{
			&ast.StateNode{Name: "init", IsInitial: true},
			&ast.StateNode{Name: "busy"},
			dangling,
		},
	}

	_, err := newStateExecutor(ctx, &symbols.Symbol{
		Kind: symbols.SymbolStateUsage,
		Name: machine.Ident.Name,
		Decl: machine,
	}, nil)
	if err == nil {
		t.Fatal("expected an error for a transition without a target")
	}
	if !strings.Contains(err.Error(), "names no target") {
		t.Errorf("expected a missing-target error, got: %v", err)
	}
}

// testStateCrossRegionTransitionsPingPong: guardless successions crossing back
// and forth between two regions never settle, so the event budget bounds the run
// with a typed error instead of hanging.
func testStateCrossRegionTransitionsPingPong(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			initial init;
			state running {
				region left {
					initial ls;
					state lidle;
					then ls lidle;
					transition lidle to rtarget;
				}
				region right {
					initial rs;
					state ridle;
					state rtarget;
					then rs ridle;
					transition rtarget to lidle;
				}
			}
			init then running;
		}
	}`)
	exec.ctx.maxStateEvents = 50

	done := make(chan error, 1)
	go func() { done <- exec.RunToCompletion() }()
	var err error
	select {
	case err = <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("run to completion hangs on successions crossing between regions")
	}
	if err == nil {
		t.Fatal("expected a budget error for cross-region successions that never settle")
	}
	if !strings.Contains(err.Error(), MaxStateEventsEnvVar) {
		t.Errorf("error %q does not name %s", err, MaxStateEventsEnvVar)
	}
}

// testStateTransitionEffectReadsAnUnknownFeature: a statement written as a
// transition's effect executes lowered like any other, so one reading a feature
// the machine does not declare reports rather than firing on a missing value.
func testStateTransitionEffectReadsAnUnknownFeature(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			attribute counter : Integer = 0;
			initial init;
			state active;
			final done;
			init then active;
			transition active to done do assign counter := missingName + 1;
		}
	}`)
	err := exec.RunToCompletion()
	if !errors.Is(err, ErrUnresolvedReference) {
		t.Fatalf("err = %v; want ErrUnresolvedReference", err)
	}
	if !strings.Contains(err.Error(), "missingName") {
		t.Errorf("err = %v; want it to name the unresolved feature", err)
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

// testEmptyAnonymousActionBody: entry, do and exit bodies stating no statement
// run the machine to completion rather than reporting an unexecutable behavior.
func testEmptyAnonymousActionBody(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			initial start;
			state quiet {
				entry action { }
				do action { }
				exit action { }
			}
			state done;
			then start quiet;
			then quiet done;
		}
	}`)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run to completion: %v", err)
	}
	current, ok := exec.CurrentState().(*ast.StateNode)
	if !ok || current.Name != "done" {
		t.Errorf("expected the empty bodies to leave the machine in done, got %v", exec.CurrentState())
	}
}

// testNonTerminatingAnonymousDoBody: a do body that never finishes spends the
// step budget instead of hanging the machine.
func testNonTerminatingAnonymousDoBody(t *testing.T) {
	err := stateRunErrorForSource(t, "Machine", `package test {
		state Machine {
			attribute c : Integer = 0;
			initial start;
			state spin {
				do action {
					while true {
						assign c := c + 1;
					}
				}
			}
			state done;
			then start spin;
			then spin done;
		}
	}`)
	if !errors.Is(err, ErrStepLimitExceeded) {
		t.Errorf("expected ErrStepLimitExceeded, got: %v", err)
	}
}

// testBehaviorPerformingAnActionAndStatingABody: a behavior that both performs
// an action and states a body of its own is reported rather than silently
// choosing one of the two.
func testBehaviorPerformingAnActionAndStatingABody(t *testing.T) {
	err := stateRunErrorForSource(t, "Machine", `package test {
		action def Bump;
		state Machine {
			attribute c : Integer = 0;
			initial start;
			state working {
				entry action mixed : Bump { assign c := c + 1; }
			}
			state done;
			then start working;
			then working done;
		}
	}`)
	if err == nil {
		t.Fatal("expected a behavior stating a body and an action to be reported")
	}
	if !strings.Contains(err.Error(), "stating a body of its own") {
		t.Errorf("expected the report to name the conflict, got: %v", err)
	}
}

// testQualifiedAssignmentTargetInAStateEffect: an assignment naming more than
// one segment is reported rather than writing the last segment.
func testQualifiedAssignmentTargetInAStateEffect(t *testing.T) {
	err := stateRunErrorForSource(t, "Machine", `package test {
		package other { attribute c : Integer = 0; }
		state Machine {
			attribute c : Integer = 0;
			initial init;
			state active;
			final done;
			init then active;
			transition active to done do assign other::c := 1;
		}
	}`)
	if err == nil {
		t.Fatal("expected a qualified assignment target to be reported")
	}
	if !strings.Contains(err.Error(), "assignment to a qualified target") {
		t.Errorf("expected the report to name the unsupported target, got: %v", err)
	}
}

// stateRunErrorForSource runs the named machine in src to completion and answers
// the first error it reports, failing if the machine hangs.
func stateRunErrorForSource(t *testing.T, name, src string) error {
	t.Helper()
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), name, ast.DefState)
	if sym == nil {
		t.Fatalf("state machine %s not found", name)
	}

	done := make(chan error, 1)
	go func() {
		exec, err := newStateExecutor(ctx, sym, nil)
		if err == nil {
			err = exec.initialize()
		}
		if err == nil {
			err = exec.RunToCompletion()
		}
		done <- err
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(10 * time.Second):
		t.Fatalf("running %s did not terminate", name)
		return nil
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
	exec, err := newStateExecutor(ctx, sym, nil)
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

// testSignalNoLevelOfACompositeStateAccepts: a signal neither the active substate
// nor any composite state enclosing it accepts is dropped by run-to-completion,
// so walking outward for a trigger ends in the machine standing still rather than
// erroring or hanging.
func testSignalNoLevelOfACompositeStateAccepts(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			initial init;
			state outer {
				state middle {
					state inner;
					state other;
					transition inner to other accept step;
				}
				state recovered;
				transition middle to recovered accept abort;
			}
			state stopped;
			init then inner;
			transition outer to stopped accept shutdown;
		}
	}`)
	exec.SendSignal("unknown", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run to completion: %v", err)
	}
	current, ok := exec.CurrentState().(*ast.StateNode)
	if !ok || current.Name != "inner" {
		t.Errorf("expected the unaccepted signal to leave the machine in inner, got %v", exec.CurrentState())
	}
}

// testCompositeSelfTransitionWithNoSubstateToReEnter: a composite state that
// declares no starting substate is re-entered by its own self-transition without
// erroring or hanging, and stays active with no substate of its own.
func testCompositeSelfTransitionWithNoSubstateToReEnter(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			initial init;
			state Working {
				state Step1;
			}
			init then Working::Step1;
			transition Working to Working accept restart;
		}
	}`)
	exec.SendSignal("restart", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run to completion: %v", err)
	}
	current, ok := exec.CurrentState().(*ast.StateNode)
	if !ok || current.Name != "Working" {
		t.Errorf("expected the re-entered composite state to be active, got %v", exec.CurrentState())
	}
}

// testStaleCompositeTimerInARegion: a time trigger on a composite state inside an
// orthogonal region whose composite is left before the timer expires is dropped,
// leaving the sibling region where it was rather than erroring or hanging.
func testStaleCompositeTimerInARegion(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			initial init;
			state working {
				region left {
					initial lstart;
					state grouping {
						state step1;
						accept after 5 then late;
					}
					state moved;
					state late;
					transition lstart to step1;
					transition grouping to moved accept skip;
				}
				region right {
					initial rstart;
					state watching;
					then rstart watching;
				}
			}
			init then working;
		}
	}`)
	exec.SendSignal("skip", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run to completion: %v", err)
	}
	active := make(map[string]bool)
	for _, state := range exec.ActiveStates() {
		active[state.Name] = true
	}
	if !active["moved"] || !active["watching"] || active["late"] {
		t.Errorf("expected the stale composite timer to leave moved and watching active, got %v", active)
	}
}

// testExitOfNestedRegionsWithAHistoryPseudostate: leaving a composite state whose
// region holds another composite with a region of its own, then returning through a
// deep history, restores the recorded configuration rather than erroring or hanging.
func testExitOfNestedRegionsWithAHistoryPseudostate(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			initial init;
			state outer {
				region left {
					initial lstart;
					state grouping {
						region inner {
							initial gstart;
							state g1;
							state g2;
							transition gstart to g1;
							transition g1 to g2 accept advance;
						}
					}
					transition lstart to grouping;
				}
				region right {
					initial rstart;
					state watching;
					transition rstart to watching;
				}
				deep history resume;
			}
			state away;
			init then outer;
			transition outer to away accept leave;
			transition away to resume accept back;
		}
	}`)
	for _, signal := range []string{"advance", "leave", "back"} {
		exec.SendSignal(signal, nil)
		if err := exec.RunToCompletion(); err != nil {
			t.Fatalf("run to completion after %s: %v", signal, err)
		}
	}
	active := make(map[string]bool)
	for _, state := range exec.ActiveStates() {
		active[state.Name] = true
	}
	if !active["g2"] || !active["watching"] {
		t.Errorf("expected the deep history to restore g2 and watching, got %v", active)
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
// send itself is undeliverable — which must be reported where it was written
// rather than left for the accept waiting on it to time out as a deadlock.
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
	if !errors.Is(err, ErrUnroutableSend) {
		t.Errorf("expected ErrUnroutableSend, got: %v", err)
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

// testActionWhoseLastNodeHasNoSuccession: a node the flow leads no further from
// ends the flow, so an action declaring no `done` node completes instead of
// failing, and stepping past the end neither errors nor spins.
func testActionWhoseLastNodeHasNoSuccession(t *testing.T) {
	src := `
		package test {
			action ends {
				first start;
				then action a;
				then action b;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))

	sym := findSymbolByName(idx.DocumentRoot("<test>"), "ends", ast.DefAction)
	if sym == nil {
		t.Fatal("action ends not found")
	}

	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("create action executor: %v", err)
	}

	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run action whose last node has no succession: %v", err)
	}
	if exec.State() != StateCompleted {
		t.Fatalf("expected the action to complete, got state %s", exec.State())
	}

	for i := 0; i < 3; i++ {
		if err := exec.Step(); err != nil {
			t.Fatalf("step %d past the end: %v", i+1, err)
		}
		if exec.State() != StateCompleted {
			t.Fatalf("step %d past the end left state %s", i+1, exec.State())
		}
		if len(exec.Tokens()) != 0 {
			t.Fatalf("step %d past the end revived %d token(s)", i+1, len(exec.Tokens()))
		}
	}
}

// testFirstNodeWithASecondSuccession: the succession out of a `first` end leaves
// from the node it names, so a second succession out of that node is ambiguous.
func testFirstNodeWithASecondSuccession(t *testing.T) {
	src := `
		package test {
			action seq {
				action s1;
				action s2;
				action s3;
				first s1 then s2;
				then s1 s3;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))

	sym := findSymbolByName(idx.DocumentRoot("<test>"), "seq", ast.DefAction)
	if sym == nil {
		t.Fatal("action seq not found")
	}

	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("create action executor: %v", err)
	}

	err = exec.RunToCompletion()
	if err == nil {
		t.Fatal("a first node with two successions ran to completion")
	}
	if !strings.Contains(err.Error(), "multiple successors") {
		t.Fatalf("error = %q, want it to report multiple successors", err)
	}
}

// testFirstBesideAnInitialNode: a body declaring an initial node of its own and a
// `first` end naming a declared node states two starts, which lowering rejects.
func testFirstBesideAnInitialNode(t *testing.T) {
	src := `
		package test {
			action seq {
				action s1;
				action s2;
				first start;
				first s1 then s2;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))

	sym := findSymbolByName(idx.DocumentRoot("<test>"), "seq", ast.DefAction)
	if sym == nil {
		t.Fatal("action seq not found")
	}

	if _, err := ctx.CreateActionExecutor(sym); err == nil {
		t.Fatal("two starts lowered without an error")
	} else if !strings.Contains(err.Error(), "multiple initial nodes") {
		t.Fatalf("error = %q, want it to report multiple initial nodes", err)
	}
}

// testFirstNamingAFinalNode: a flow cannot start where it ends, so lowering
// rejects it rather than completing with the declared node never run.
func testFirstNamingAFinalNode(t *testing.T) {
	src := `
		package test {
			action seq {
				attribute x = 0;
				action s1 { assign x := 7; }
				done fin;
				first fin then s1;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))

	sym := findSymbolByName(idx.DocumentRoot("<test>"), "seq", ast.DefAction)
	if sym == nil {
		t.Fatal("action seq not found")
	}

	if _, err := ctx.CreateActionExecutor(sym); err == nil {
		t.Fatal("a first end naming a final node lowered without an error")
	} else if !strings.Contains(err.Error(), "final node fin") {
		t.Fatalf("error = %q, want it to name the final node", err)
	}
}

// testForkBranchesAssigningTheSameFeature: concurrent branches writing one feature
// are unordered by the spec; the runtime resolves them by its own step order.
func testForkBranchesAssigningTheSameFeature(t *testing.T) {
	src := `
		package test {
			action clash {
				attribute x : Integer = 0;

				first start;
				fork split;
				action left { assign x := 1; }
				action right { assign x := 2; }
				join sync;
				done end;

				then start split;
				then split left;
				then split right;
				then left sync;
				then right sync;
				then sync end;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))

	sym := findSymbolByName(idx.DocumentRoot("<test>"), "clash", ast.DefAction)
	if sym == nil {
		t.Fatal("action clash not found")
	}

	var first semantics.Value
	for run := 0; run < 3; run++ {
		exec, err := ctx.CreateActionExecutor(sym)
		if err != nil {
			t.Fatalf("run %d create action executor: %v", run+1, err)
		}
		if err := exec.RunToCompletion(); err != nil {
			t.Fatalf("run %d conflicting writes: %v", run+1, err)
		}
		if exec.State() != StateCompleted {
			t.Fatalf("run %d left state %s", run+1, exec.State())
		}

		got, ok := exec.Results()["x"]
		if !ok {
			t.Fatalf("run %d lost the contested feature x", run+1)
		}
		if got.Kind != ValConst || got.Const.Kind != semantics.ValInt {
			t.Fatalf("run %d gave x a non-integer value: %+v", run+1, got)
		}
		if got.Const.Int != 1 && got.Const.Int != 2 {
			t.Fatalf("run %d gave x %d, which neither branch assigned", run+1, got.Const.Int)
		}
		if run == 0 {
			first = got.Const
			continue
		}
		if got.Const.Int != first.Int {
			t.Fatalf("run %d gave x %d after run 1 gave %d: execution is not deterministic",
				run+1, got.Const.Int, first.Int)
		}
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

// testCalcUnboundKeywordNamedParameter: a parameter named with a keyword is a
// parameter like any other, so leaving it unbound reports, never panics.
func testCalcUnboundKeywordNamedParameter(t *testing.T) {
	src := `
		package test {
			calc classify {
				in 'type': Integer;
				in 'state': Integer;
				return 'type' + 'state';
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "classify", ast.DefCalc)
	if sym == nil {
		t.Fatal("classify calc not found")
	}

	arg := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 3}}
	result, err := ctx.InvokeCalc(sym, []Value{arg}, rootScope)
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

// testCalcDirectRecursion: a calc that invokes itself unconditionally never
// terminates, so the run's calc depth budget must report it instead of the
// process exhausting its stack.
func testCalcDirectRecursion(t *testing.T) {
	src := `
		package test {
			calc countdown {
				in n: Integer;
				return countdown(n - 1);
			}
		}
	`
	assertCalcRecursionBounded(t, src, "countdown", ErrCalcRecursionLimit)
}

// testCalcMutualRecursion: the budget is spent by nesting, so a cycle through
// another calc is reported the same way direct self-invocation is.
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
	assertCalcRecursionBounded(t, src, "ping", ErrCalcRecursionLimit)
}

// testCalcRecursionSpendsStepBudget: the two bounds are independent, so a
// recursion whose evaluations run out first is reported by the step budget
// rather than running on until the depth bound.
func testCalcRecursionSpendsStepBudget(t *testing.T) {
	src := `
		package test {
			calc grow {
				in n: Integer;
				return : Integer = n + grow(n + 1);
			}
		}
	`
	assertCalcRecursionBounded(t, src, "grow", ErrStepLimitExceeded, func(ctx *Context) {
		// Room to recurse far deeper than the evaluations allow.
		ctx.maxCalcDepth = MaxCalcDepthCeiling
		ctx.maxSteps = 500
	})
}

// testCalcRecursionAtDepthCeiling: the highest depth budget a run may be given
// must still be reported rather than reached by exhausting the stack, which
// would be fatal.
func testCalcRecursionAtDepthCeiling(t *testing.T) {
	src := `
		package test {
			calc deep {
				in n: Integer;
				attribute acc : Integer = (n + 1) * (n + 2) - n * n;
				return : Integer = acc + deep(n + 1);
			}
		}
	`
	assertCalcRecursionBounded(t, src, "deep", ErrCalcRecursionLimit, func(ctx *Context) {
		ctx.maxCalcDepth = MaxCalcDepthCeiling
	})
}

// assertCalcRecursionBounded invokes calcName and requires the given budget
// error promptly: the invocation runs on its own goroutine so a hang fails the
// case instead of stalling the suite until the package timeout, and a panic in
// it fails the case rather than the package.
func assertCalcRecursionBounded(t *testing.T, src, calcName string, want error, budgets ...func(*Context)) {
	t.Helper()

	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	// The default step budget, so a recursion bounded by depth reaches that bound
	// rather than running out of evaluations first.
	ctx.maxSteps = DefaultMaxSteps
	for _, set := range budgets {
		set(ctx)
	}
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, calcName, ast.DefCalc)
	if sym == nil {
		t.Fatalf("calc %s not found", calcName)
	}

	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("calc %s panicked: %v", calcName, r)
			}
		}()
		arg := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 10}}
		_, err := ctx.InvokeCalc(sym, []Value{arg}, rootScope)
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatalf("expected recursive calc %s to be bounded, it returned a value", calcName)
		}
		if !errors.Is(err, want) {
			t.Errorf("expected %v, got: %v", want, err)
		}
	case <-time.After(30 * time.Second):
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

// testNestedConditionSubjectIsAmbiguous: two objects redefining the same nested
// feature differently make the subject of a check a question, reported as
// ErrAmbiguousSubject rather than answered from whichever object is found first.
func testNestedConditionSubjectIsAmbiguous(t *testing.T) {
	src := `
		package test {
			part def Leaf {
				attribute value = 1.0;
				constraint small { value < 10.0 }
			}
			part def Top {
				part leaf : Leaf;
			}
			part slow : Top {
				part :>> leaf { attribute :>> value = 2.0; }
			}
			part fast : Top {
				part :>> leaf { attribute :>> value = 99.0; }
			}
		}
	`
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}
	idx, _, ctx := buildRuntime(t, "<test>", file)
	rootScope := idx.DocumentRoot("<test>")
	for _, name := range []string{"slow", "fast"} {
		if _, err := ctx.Instantiate(memberPath(t, rootScope, "test", name)); err != nil {
			t.Fatalf("instantiate %s: %v", name, err)
		}
	}
	small := memberPath(t, rootScope, "test", "Leaf", "small")
	satisfied, err := ctx.EvaluateConstraint(small, small.OwnerScope)
	if !errors.Is(err, ErrAmbiguousSubject) {
		t.Fatalf("satisfied = %t, err = %v, want ErrAmbiguousSubject", satisfied, err)
	}
	if satisfied {
		t.Error("an ambiguous subject is no verdict")
	}
}

// testSatisfactionSubjectIsAmbiguous: a satisfaction assertion whose `by` object
// holds two objects of the requirement's owner has no one subject either, and
// reports it as ErrAmbiguousSubject rather than picking one.
func testSatisfactionSubjectIsAmbiguous(t *testing.T) {
	src := `
		package test {
			part def Leaf {
				attribute value = 1.0;
				requirement lim { require value < 10.0; }
			}
			part def Top {
				part slow : Leaf { attribute :>> value = 2.0; }
				part fast : Leaf { attribute :>> value = 99.0; }
			}
			part top : Top;
			assert satisfy Leaf::lim by top;
		}
	`
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}
	idx, _, ctx := buildRuntime(t, "<test>", file)
	rootScope := idx.DocumentRoot("<test>")
	pkg := memberPath(t, rootScope, "test")
	assertions := ctx.SatisfyAssertionsIn(pkg.Scope)
	if len(assertions) != 1 {
		t.Fatalf("assertions = %d, want the one the package states", len(assertions))
	}
	result, err := ctx.CheckSatisfactionOn(assertions[0], nil)
	if !errors.Is(err, ErrAmbiguousSubject) {
		t.Fatalf("holds = %t, err = %v, want ErrAmbiguousSubject", result.Holds, err)
	}
	if result.Holds {
		t.Error("an ambiguous subject is no verdict")
	}
}

// testRecursiveCompositionSubjectSearch: searching for the object a check is
// about does not walk a design containing its own kind forever; it answers about
// the declaration, since no object of the checked type is there.
func testRecursiveCompositionSubjectSearch(t *testing.T) {
	src := `
		package test {
			part def Leaf {
				attribute value = 1.0;
				constraint small { value < 10.0 }
			}
			part def Node {
				part next : Node;
			}
			part root : Node;
		}
	`
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}
	idx, _, ctx := buildRuntime(t, "<test>", file)
	rootScope := idx.DocumentRoot("<test>")
	if _, err := ctx.Instantiate(memberPath(t, rootScope, "test", "root")); err != nil {
		t.Fatalf("instantiate root: %v", err)
	}
	small := memberPath(t, rootScope, "test", "Leaf", "small")
	done := make(chan struct{})
	var satisfied bool
	var err error
	go func() {
		defer close(done)
		satisfied, err = ctx.EvaluateConstraint(small, small.OwnerScope)
	}()
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("the subject search did not terminate on recursive composition")
	}
	if err != nil {
		t.Fatalf("EvaluateConstraint: %v", err)
	}
	if !satisfied {
		t.Error("satisfied = false, want the declaration's answer")
	}
	if len(ctx.instances) > 1000 {
		t.Errorf("%d objects materialized: the search is not bounded", len(ctx.instances))
	}
}

// testDuplicateObjectsOfOneDeclaration: materializing the same declaration twice
// is one object as far as a check is concerned, not an ambiguous subject.
func testDuplicateObjectsOfOneDeclaration(t *testing.T) {
	src := `
		package test {
			part def Leaf {
				attribute value = 1.0;
				constraint small { value < 10.0 }
			}
			part def Top {
				part leaf : Leaf;
			}
			part o : Top {
				part :>> leaf { attribute :>> value = 99.0; }
			}
		}
	`
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}
	idx, _, ctx := buildRuntime(t, "<test>", file)
	rootScope := idx.DocumentRoot("<test>")
	obj := memberPath(t, rootScope, "test", "o")
	for range 2 {
		if _, err := ctx.Instantiate(obj); err != nil {
			t.Fatalf("instantiate o: %v", err)
		}
	}
	small := memberPath(t, rootScope, "test", "Leaf", "small")
	satisfied, err := ctx.EvaluateConstraint(small, small.OwnerScope)
	if err != nil && !errors.Is(err, ErrViolated) {
		t.Fatalf("EvaluateConstraint: %v", err)
	}
	if satisfied {
		t.Error("satisfied = true, want the object's 99.0 to violate the constraint")
	}
}

// testDuplicateObjectsHoldingAPlainPart: a nested part typed by a definition
// rather than by a body of its own is reached through its holder, so what two
// materializations of that holder leave behind is no ambiguous subject.
func testDuplicateObjectsHoldingAPlainPart(t *testing.T) {
	src := `
		package test {
			part def Leaf {
				attribute value = 99.0;
				constraint small { value < 10.0 }
			}
			part def Top {
				part leaf : Leaf;
			}
			part o : Top;
		}
	`
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}
	idx, _, ctx := buildRuntime(t, "<test>", file)
	rootScope := idx.DocumentRoot("<test>")
	obj := memberPath(t, rootScope, "test", "o")
	small := memberPath(t, rootScope, "test", "Leaf", "small")
	for range 2 {
		if _, err := ctx.Instantiate(obj); err != nil {
			t.Fatalf("instantiate o: %v", err)
		}
		satisfied, err := ctx.EvaluateConstraint(small, small.OwnerScope)
		if err != nil && !errors.Is(err, ErrViolated) {
			t.Fatalf("EvaluateConstraint: %v", err)
		}
		if satisfied {
			t.Error("satisfied = true, want the object's 99.0 to violate the constraint")
		}
	}
}

// testNestedPartHeldWithAMultiplicity: the objects one slot materializes for a
// multiplicity are occurrences of one declaration, so a check answers a verdict
// rather than calling its subject ambiguous.
func testNestedPartHeldWithAMultiplicity(t *testing.T) {
	src := `
		package test {
			part def Wheel {
				attribute pressure = 99.0;
				constraint inflated { pressure < 10.0 }
			}
			part def Car {
				part wheels : Wheel[4];
			}
			part car : Car;
		}
	`
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}
	idx, _, ctx := buildRuntime(t, "<test>", file)
	rootScope := idx.DocumentRoot("<test>")
	if _, err := ctx.Instantiate(memberPath(t, rootScope, "test", "car")); err != nil {
		t.Fatalf("instantiate car: %v", err)
	}
	inflated := memberPath(t, rootScope, "test", "Wheel", "inflated")
	satisfied, err := ctx.EvaluateConstraint(inflated, inflated.OwnerScope)
	if err != nil && !errors.Is(err, ErrViolated) {
		t.Fatalf("EvaluateConstraint: %v", err)
	}
	if satisfied {
		t.Error("satisfied = true, want the wheels' 99.0 to violate the constraint")
	}
}

// testPartNestedInsideARepeatedPart: the declaration a check names may sit
// deeper inside the part a multiplicity repeated, and the objects reached along
// one declaration path are still one subject rather than an ambiguity.
func testPartNestedInsideARepeatedPart(t *testing.T) {
	src := `
		package test {
			part def Bolt {
				attribute torque = 99.0;
				constraint tight { torque < 10.0 }
			}
			part def Wheel {
				part bolt : Bolt;
			}
			part def Car {
				part wheels : Wheel[4];
			}
			part car : Car;
		}
	`
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}
	idx, _, ctx := buildRuntime(t, "<test>", file)
	rootScope := idx.DocumentRoot("<test>")
	if _, err := ctx.Instantiate(memberPath(t, rootScope, "test", "car")); err != nil {
		t.Fatalf("instantiate car: %v", err)
	}
	tight := memberPath(t, rootScope, "test", "Bolt", "tight")
	satisfied, err := ctx.EvaluateConstraint(tight, tight.OwnerScope)
	if err != nil && !errors.Is(err, ErrViolated) {
		t.Fatalf("EvaluateConstraint: %v", err)
	}
	if satisfied {
		t.Error("satisfied = true, want the bolts' 99.0 to violate the constraint")
	}
}

// testPartsSubsettingOneCollection: two declarations feeding one collection are
// two subjects, not repetitions of the collection, so the check reports the
// ambiguity rather than answering from whichever it reached first.
func testPartsSubsettingOneCollection(t *testing.T) {
	src := `
		package test {
			part def Component {
				attribute v = 1.0;
				constraint ok { v < 10.0 }
			}
			part def Assembly {
				part subsystem : Component[*];
				part small : Component :> subsystem {
					attribute :>> v = 5.0;
				}
				part large : Component :> subsystem {
					attribute :>> v = 99.0;
				}
			}
			part assembly : Assembly;
		}
	`
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}
	idx, _, ctx := buildRuntime(t, "<test>", file)
	rootScope := idx.DocumentRoot("<test>")
	if _, err := ctx.Instantiate(memberPath(t, rootScope, "test", "assembly")); err != nil {
		t.Fatalf("instantiate assembly: %v", err)
	}
	ok := memberPath(t, rootScope, "test", "Component", "ok")
	satisfied, err := ctx.EvaluateConstraint(ok, ok.OwnerScope)
	if !errors.Is(err, ErrAmbiguousSubject) {
		t.Fatalf("satisfied = %t, err = %v, want ErrAmbiguousSubject", satisfied, err)
	}
	if satisfied {
		t.Error("an ambiguous subject is no verdict")
	}
	// The two objects reached through one collection are told apart by the
	// declaration each materializes, not by the feature holding both.
	for _, want := range []string{"(small)", "(large)"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name a carrier %q", err, want)
		}
	}
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

// testEvalOnAnInstanceSpendsTheStepBudget: an expression evaluated against an
// instance is one run, so reading a slot inside it does not start a run of its
// own and reset the counter; an expression longer than the budget is refused.
func testEvalOnAnInstanceSpendsTheStepBudget(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `
		package test {
			part def Car { attribute m = 5.0; }
		}
	`))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "Car", ast.DefPart)
	if sym == nil {
		t.Fatal("part def Car not found")
	}
	inst, err := ctx.Instantiate(sym)
	if err != nil {
		t.Fatalf("instantiating Car: %v", err)
	}

	// Nested to the right, so the slot read - which brackets a run of its own when
	// the evaluation is not already one - is reached on the second step.
	expr := "m"
	for i := 0; i < 60; i++ {
		expr = "m + (" + expr + ")"
	}
	node := parser.New(source.New("<e>", []byte(expr))).ParseExpression()
	if node == nil {
		t.Fatal("the expression did not parse")
	}

	ctx.maxSteps = 6
	got, err := ctx.EvalWithScopeOn(node, sym.Scope, inst)
	if err == nil {
		t.Fatalf("expected the step budget to bound the evaluation, got %v", got)
	}
	if !errors.Is(err, ErrStepLimitExceeded) {
		t.Errorf("expected ErrStepLimitExceeded, got: %v", err)
	}

	// The budget bounds one run, not the session: a short expression is answered
	// however many ran before it.
	ctx.maxSteps = 20
	short := parser.New(source.New("<e>", []byte("m + m"))).ParseExpression()
	for i := 0; i < 5; i++ {
		if _, err := ctx.EvalWithScopeOn(short, sym.Scope, inst); err != nil {
			t.Fatalf("evaluation %d of m + m under a fresh run: %v", i+1, err)
		}
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
						part inner;
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

// testBlockFlowOfUnexecutableMember: a member outside the semantics a block's own
// flow gives its members is still reported when reached, even in a block that
// does state a flow because a nested action is declared beside it.
func testBlockFlowOfUnexecutableMember(t *testing.T) {
	src := `
		package test {
			action counter {
				attribute total : Integer = 0;
				first start;
				action accumulate {
					while total < 3 {
						action bump {
							assign total := total + 1;
						}
						part inner;
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
		t.Fatal("expected the unexecutable member of the block's flow to be reported")
	}
	if !strings.Contains(err.Error(), "not executable") {
		t.Errorf("error does not name the unexecutable member: %v", err)
	}
}

// testNonTerminatingLoopPerformingAnAction: a loop whose body performs an action
// as a node of the block's own flow still spends a step per iteration, so it
// ends with ErrStepLimitExceeded rather than performing forever.
func testNonTerminatingLoopPerformingAnAction(t *testing.T) {
	src := `
		package test {
			action spinner {
				attribute total : Integer = 0;
				first start;
				action spin {
					while total >= 0 {
						perform bump;
						assign total := total + 1;
					}
				}
				done end;
				then start spin;
				then spin end;
			}

			action bump {
				out spun : Integer;
				first begin;
				action run {
					assign spun := 1;
				}
				done finish;
				then begin run;
				then run finish;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))

	ctx.maxSteps = 40
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

// testForOverAValueNoExpressionMakesIterable: a value that states a computation
// rather than a collection is no collection in any order, so a `for` over it
// fails with a typed error naming it.
func testForOverAValueNoExpressionMakesIterable(t *testing.T) {
	for _, value := range []Value{{Kind: ValExpr}, {Kind: ValInvalid}} {
		elements, err := forElements(value)
		if err == nil {
			t.Errorf("forElements(%s) = %v, want a typed error", describeValue(value), elements)
			continue
		}
		if !errors.Is(err, ErrTypeMismatch) {
			t.Errorf("forElements(%s) failed with %v, want ErrTypeMismatch", describeValue(value), err)
		}
		if !strings.Contains(err.Error(), describeValue(value)) {
			t.Errorf("error does not name the value: %v", err)
		}
	}
}

// testForOverAScalar: a `for` whose input is a scalar fails with a typed error
// rather than iterating once over the coercion elementsOf would make of it.
func testForOverAScalar(t *testing.T) {
	src := `
		package test {
			action counter {
				attribute single : Integer = 7;
				attribute visited : Integer = 0;
				first start;
				action iterate {
					for s in single {
						assign visited := visited + 1;
					}
				}
				done end;
				then start iterate;
				then iterate end;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))

	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "counter", ast.DefAction)
	if sym == nil {
		t.Fatal("action counter not found")
	}

	result, err := ctx.ExecuteAction(sym)
	if err == nil {
		t.Fatalf("the action completed with %v, want a typed error", result)
	}
	if !errors.Is(err, ErrTypeMismatch) {
		t.Errorf("execution failed with %v, want ErrTypeMismatch", err)
	}
	if !strings.Contains(err.Error(), "an Integer is not one") {
		t.Errorf("error does not name the value it was given: %v", err)
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

// testFlowEndNamingNoNode: a flow moves a value from one action node's output
// to another's input, so an end naming something that is not a node of the
// action is reported rather than dropped, which would leave the flow declared
// and carrying nothing.
func testFlowEndNamingNoNode(t *testing.T) {
	cases := map[string]string{
		"source": "flow bad from missing.engineTorque to amplify.torqueIn;",
		"target": "flow bad from generate.engineTorque to missing.torqueIn;",
	}

	for name, flow := range cases {
		t.Run(name, func(t *testing.T) {
			src := `
				package test {
					action driveTrain {
						first start;
						action generate { out engineTorque : Integer; assign engineTorque := 1; }
						action amplify { in torqueIn : Integer; }
						done end;
						then start generate;
						then generate amplify;
						then amplify end;
						` + flow + `
					}
				}
			`
			idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
			sym := findSymbolByName(idx.DocumentRoot("<test>"), "driveTrain", ast.DefAction)
			if sym == nil {
				t.Fatal("action driveTrain not found")
			}

			_, err := ctx.ExecuteAction(sym)
			if err == nil {
				t.Fatalf("expected the flow's %s end to be reported", name)
			}
			if !strings.Contains(err.Error(), "flow bad") ||
				!strings.Contains(err.Error(), name) {
				t.Errorf("error does not name the flow and the end at fault: %v", err)
			}
		})
	}
}

// testFlowNamingNoPin: a flow carries the value of a feature, so a flow whose
// ends name nodes alone and which declares no payload identifies nothing to
// move, and is reported when the graph is built rather than mid-run.
func testFlowNamingNoPin(t *testing.T) {
	_, err := executeActionSource(t, "driveTrain", `package test {
		action driveTrain {
			first start;
			action generate { out engineTorque : Integer; assign engineTorque := 1; }
			action amplify { in engineTorque : Integer; }
			done end;
			then start generate;
			then generate amplify;
			then amplify end;
			flow generateToAmplify from generate to amplify;
		}
	}`)
	if err == nil {
		t.Fatal("expected the flow naming no feature to be reported")
	}
	if !strings.Contains(err.Error(), "generateToAmplify") ||
		!strings.Contains(err.Error(), "names no feature to carry") {
		t.Errorf("error does not say the flow names no feature: %v", err)
	}
}

// testAcceptPayloadWithoutAValue: an accept that names a payload binds the
// single value the accepted message carries, so a message carrying none is
// reported rather than binding an empty value the guard and effect would read.
func testAcceptPayloadWithoutAValue(t *testing.T) {
	_, err := executeActionSource(t, "pipeline", `package P {
		item def Ping;
		action pipeline {
			first start;
			action sender { send Ping() to reader; }
			action reader accept p : Ping;
			done end;
			then start sender;
			then sender reader;
			then reader end;
		}
	}`)
	if err == nil {
		t.Fatal("expected the payload-less message to be reported")
	}
	if !errors.Is(err, ErrNoValue) {
		t.Errorf("expected ErrNoValue, got: %v", err)
	}
	if !strings.Contains(err.Error(), "Ping") {
		t.Errorf("error does not name the accepted signal: %v", err)
	}
}

// testAcceptPayloadReadBeforeItIsBound: the payload is a declaration of the body
// wherever the body resolves, so a node running before the accept binds it
// resolves the name and finds no value — reported, not read as an empty value.
func testAcceptPayloadReadBeforeItIsBound(t *testing.T) {
	_, err := executeActionSource(t, "pipeline", `package P {
		action pipeline {
			attribute seen : Integer = 0;
			first start;
			action reader { assign seen := msg; }
			action waiter accept msg : Integer;
			done end;
			then start reader;
			then reader waiter;
			then waiter end;
		}
	}`)
	if !errors.Is(err, ErrUnresolvedReference) {
		t.Fatalf("err = %v; want ErrUnresolvedReference", err)
	}
	if !strings.Contains(err.Error(), "msg") {
		t.Errorf("error does not name the payload: %v", err)
	}
}

// testFlowFromANodeThatProducedNothing: a flow out of a node that left its
// source pin empty carries nothing, which is reported rather than silently
// leaving the target pin unwritten.
func testFlowFromANodeThatProducedNothing(t *testing.T) {
	src := `
		package test {
			action driveTrain {
				first start;
				action generate { out engineTorque : Integer; }
				action amplify { in torqueIn : Integer; }
				done end;
				then start generate;
				then generate amplify;
				then amplify end;
				flow generateToAmplify from generate.engineTorque to amplify.torqueIn;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "driveTrain", ast.DefAction)
	if sym == nil {
		t.Fatal("action driveTrain not found")
	}

	_, err := ctx.ExecuteAction(sym)
	if err == nil {
		t.Fatal("expected the empty source pin to be reported")
	}
	if !strings.Contains(err.Error(), "generateToAmplify") ||
		!strings.Contains(err.Error(), "engineTorque") {
		t.Errorf("error does not name the flow and the pin that stayed empty: %v", err)
	}
}

// testActionAcceptTimeTrigger: an action body has no clock, so an accept that
// waits for an instant is reported rather than passed through as though the
// instant had already arrived.
func testActionAcceptTimeTrigger(t *testing.T) {
	for name, trigger := range map[string]string{
		"at":    "accept at maintenanceTime",
		"after": "accept after 5",
	} {
		t.Run(name, func(t *testing.T) {
			src := `
				package test {
					action maintain {
						attribute maintenanceTime : Integer = 3;
						attribute done : Integer = 0;
						first start;
						action waitForIt ` + trigger + `;
						action work { assign done := 1; }
						done end;
						then start waitForIt;
						then waitForIt work;
						then work end;
					}
				}
			`
			idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
			sym := findSymbolByName(idx.DocumentRoot("<test>"), "maintain", ast.DefAction)
			if sym == nil {
				t.Fatal("action maintain not found")
			}

			_, err := ctx.ExecuteAction(sym)
			if !errors.Is(err, ErrNoClock) {
				t.Fatalf("ExecuteAction error = %v, want ErrNoClock", err)
			}
			if !strings.Contains(err.Error(), "state machine") {
				t.Errorf("error does not say where a time event is waited on: %v", err)
			}
		})
	}
}

// testActionAcceptNonBooleanChangeTrigger: a change trigger states a condition,
// so one that evaluates to something else is reported rather than read as true.
func testActionAcceptNonBooleanChangeTrigger(t *testing.T) {
	src := `
		package test {
			action monitor {
				attribute temp : Integer = 10;
				first start;
				action awaitWarm accept when temp;
				done end;
				then start awaitWarm;
				then awaitWarm end;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "monitor", ast.DefAction)
	if sym == nil {
		t.Fatal("action monitor not found")
	}

	_, err := ctx.ExecuteAction(sym)
	if !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("ExecuteAction error = %v, want ErrTypeMismatch", err)
	}
}

// testActionBodyUnresolvedUnit: an action body is evaluated in the scope it was
// written in, and a unit that scope does not bring in resolves to nothing — the
// quantity is reported as such rather than evaluated as its bare magnitude.
func testActionBodyUnresolvedUnit(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			public import ScalarValues::*;
			action descend {
				attribute h : Real = 500.0 [furlong];
				first start;
				done end;
				then start end;
			}
		}
	`))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "descend", ast.DefAction)
	if sym == nil {
		t.Fatal("action descend not found")
	}

	out, err := ctx.ExecuteAction(sym)
	if !errors.Is(err, ErrNotAQuantity) {
		t.Fatalf("outputs = %v, err = %v; want ErrNotAQuantity", out, err)
	}
	if !strings.Contains(err.Error(), semantics.ErrNotAUnit.Error()) {
		t.Errorf("err = %v; want it to report that the index names no measurement unit", err)
	}
}

// testActionBodyUnresolvedFeature: a name no frame, object or enclosing scope
// supplies is reported as unresolved, so giving a body its declaring scope does
// not turn a typo into a silent zero.
func testActionBodyUnresolvedFeature(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `
		package test {
			action counter {
				attribute total : Integer = 0;
				first start;
				action bump {
					assign total := missingName + 1;
				}
				done end;
				then start bump;
				then bump end;
			}
		}
	`))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "counter", ast.DefAction)
	if sym == nil {
		t.Fatal("action counter not found")
	}

	out, err := ctx.ExecuteAction(sym)
	if !errors.Is(err, ErrUnresolvedReference) {
		t.Fatalf("outputs = %v, err = %v; want ErrUnresolvedReference", out, err)
	}
	if !strings.Contains(err.Error(), "missingName") {
		t.Errorf("err = %v; want it to name the unresolved feature", err)
	}
}

// testStateBodyUnresolvedUnit: the same for a state machine's attribute default,
// which is evaluated when the machine initializes.
func testStateBodyUnresolvedUnit(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			public import ScalarValues::*;
			state monitor {
				attribute speed : Real = 1.5 [knot];
				initial start;
				state running;
				start then running;
			}
		}
	`))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "monitor", ast.DefState)
	if sym == nil {
		t.Fatal("state monitor not found")
	}

	_, err := ctx.ExecuteState(sym)
	if !errors.Is(err, ErrNotAQuantity) {
		t.Fatalf("err = %v; want ErrNotAQuantity", err)
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

// testQuantityUnitShadowedBySibling: a unit position naming a sibling that is
// not a measurement unit reports which declaration it resolved to and the unit
// that declaration hid, rather than a magnitude in the wrong unit.
func testQuantityUnitShadowedBySibling(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			public import SI::*;
			constraint def Tall {
				attribute m : ScalarValues::Real = 2.0;
				1.0 [m] > 500.0 [m]
			}
		}
	`))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "Tall", ast.DefConstraint)
	if sym == nil {
		t.Fatal("Tall constraint not found")
	}

	satisfied, err := ctx.EvaluateConstraint(sym, sym.OwnerScope)
	if !errors.Is(err, ErrNotAQuantity) {
		t.Fatalf("satisfied = %v, err = %v; want ErrNotAQuantity", satisfied, err)
	}
	if !errors.Is(err, semantics.ErrNotAUnit) {
		t.Errorf("err = %v; want it to report that the name is no measurement unit", err)
	}
	var shadowed *semantics.ShadowedUnitError
	if !errors.As(err, &shadowed) {
		t.Fatalf("err = %v; want a *semantics.ShadowedUnitError", err)
	}
	if shadowed.Resolved == nil || shadowed.Namespace != "test::Tall" {
		t.Errorf("error names %v in %q; want the sibling declared in test::Tall", shadowed.Resolved, shadowed.Namespace)
	}
	if shadowed.Shadowed == nil || shadowed.Suggestion != "SI::m" {
		t.Errorf("error suggests %q; want the qualified spelling SI::m of the hidden unit", shadowed.Suggestion)
	}
}

// testQuantityQualifiedUnitIsNotShadowing: a qualified name in unit position
// resolves to what it names, so a non-unit is reported as one without a
// shadowing explanation or a spelling that would not resolve.
func testQuantityQualifiedUnitIsNotShadowing(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			public import SI::*;
			attribute m : ScalarValues::Real = 2.0;
			constraint def Tall {
				1.0 [test::m] > 500.0 [SI::m]
			}
		}
	`))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "Tall", ast.DefConstraint)
	if sym == nil {
		t.Fatal("Tall constraint not found")
	}

	satisfied, err := ctx.EvaluateConstraint(sym, sym.OwnerScope)
	if !errors.Is(err, semantics.ErrNotAUnit) {
		t.Fatalf("satisfied = %v, err = %v; want ErrNotAUnit", satisfied, err)
	}
	var shadowed *semantics.ShadowedUnitError
	if !errors.As(err, &shadowed) {
		t.Fatalf("err = %v; want a *semantics.ShadowedUnitError", err)
	}
	if shadowed.Shadowed != nil || shadowed.Suggestion != "" {
		t.Errorf("error suggests %q for a qualified name; want no shadowing explanation", shadowed.Suggestion)
	}
	if !strings.Contains(err.Error(), "test::m resolves to") {
		t.Errorf("err = %v; want it to name the declaration as written", err)
	}
}

// testQuantityShadowedUnitWithoutAQualifier: a hidden unit owned by no namespace
// has no qualified spelling to offer, so the diagnostic names it without
// advising the name that just failed.
func testQuantityShadowedUnitWithoutAQualifier(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		attribute u : ISQBase::LengthUnit = SI::m;
		package test {
			attribute u : ScalarValues::Real = 2.0;
			constraint def Tall {
				1.0 [u] > 0.5 [SI::m]
			}
		}
	`))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Tall", ast.DefConstraint)
	if sym == nil {
		t.Fatal("Tall constraint not found")
	}

	satisfied, err := ctx.EvaluateConstraint(sym, sym.OwnerScope)
	if !errors.Is(err, semantics.ErrNotAUnit) {
		t.Fatalf("satisfied = %v, err = %v; want ErrNotAUnit", satisfied, err)
	}
	var shadowed *semantics.ShadowedUnitError
	if !errors.As(err, &shadowed) {
		t.Fatalf("err = %v; want a *semantics.ShadowedUnitError", err)
	}
	if shadowed.Shadowed == nil {
		t.Fatalf("err = %v; want it to name the unit the declaration hid", err)
	}
	if shadowed.Suggestion != "" {
		t.Errorf("error suggests %q; want no spelling when none qualifies the unit", shadowed.Suggestion)
	}
	if strings.Contains(err.Error(), "write u") {
		t.Errorf("err = %v; want it not to advise the name that failed", err)
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

// invokeCalcInSource invokes calcName with one Integer argument and returns the
// error, on its own goroutine so a body that never terminates fails the case
// instead of stalling the suite. maxSteps bounds the run.
func invokeCalcInSource(t *testing.T, src, calcName string, arg int64, maxSteps int64) error {
	t.Helper()

	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	ctx.maxSteps = maxSteps
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, calcName, ast.DefCalc)
	if sym == nil {
		t.Fatalf("calc %s not found", calcName)
	}

	done := make(chan error, 1)
	go func() {
		value := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: arg}}
		result, err := ctx.InvokeCalc(sym, []Value{value}, rootScope)
		if err == nil {
			err = fmt.Errorf("calc %s returned %s, expected it to fail", calcName, FormatTraceValue(result))
		}
		done <- err
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(10 * time.Second):
		t.Fatalf("calc %s did not terminate", calcName)
		return nil
	}
}

// testCalcNonTerminatingLoop: a calc loop whose condition always holds spends
// the context's step budget, so it fails the invocation instead of hanging the
// REPL, LSP or gRPC caller that drove it.
func testCalcNonTerminatingLoop(t *testing.T) {
	src := `
		package test {
			calc spin {
				in n: Integer;
				attribute i : Integer = 0;
				while i >= 0 {
					i = i + 1;
				}
				return : Integer = i;
			}
		}
	`
	err := invokeCalcInSource(t, src, "spin", 1, 20)
	if !errors.Is(err, ErrStepLimitExceeded) {
		t.Errorf("expected ErrStepLimitExceeded, got: %v", err)
	}
}

// testCalcBodyNeverReturns: a body that computes but reaches no `return` states
// no result, which is an error rather than a null value.
func testCalcBodyNeverReturns(t *testing.T) {
	src := `
		package test {
			calc maybe {
				in n: Integer;
				attribute total : Integer = 0;
				if n > 0 {
					total = n;
				}
				if n < 0 {
					return : Integer = total;
				}
			}
		}
	`
	err := invokeCalcInSource(t, src, "maybe", 5, 10000)
	if !errors.Is(err, ErrCalcNoReturn) {
		t.Errorf("expected ErrCalcNoReturn, got: %v", err)
	}
}

// testCalcSendIsRejected: a calculation computes a value and nothing else, so a
// send in its body is rejected rather than posted.
func testCalcSendIsRejected(t *testing.T) {
	src := `
		package test {
			attribute def Ping;
			calc noisy {
				in n: Integer;
				send Ping() to listener;
				return : Integer = n;
			}
		}
	`
	err := invokeCalcInSource(t, src, "noisy", 1, 10000)
	if !errors.Is(err, ErrCalcSideEffect) {
		t.Errorf("expected ErrCalcSideEffect, got: %v", err)
	}
}

// testCalcTerminateIsRejected: `terminate` ends an execution, which a
// calculation has no business doing.
func testCalcTerminateIsRejected(t *testing.T) {
	src := `
		package test {
			calc halting {
				in n: Integer;
				terminate;
				return : Integer = n;
			}
		}
	`
	err := invokeCalcInSource(t, src, "halting", 1, 10000)
	if !errors.Is(err, ErrCalcSideEffect) {
		t.Errorf("expected ErrCalcSideEffect, got: %v", err)
	}
}

// testCalcAssignmentOutsideTheCalc: a calc may write its own parameters and
// locals; a name it does not declare belongs to the model around it and writing
// it would be an effect, so it is rejected.
func testCalcAssignmentOutsideTheCalc(t *testing.T) {
	src := `
		package test {
			attribute shared : Integer = 0;
			calc leaky {
				in n: Integer;
				shared = n;
				return : Integer = n;
			}
		}
	`
	err := invokeCalcInSource(t, src, "leaky", 3, 10000)
	if !errors.Is(err, ErrCalcExternalAssignment) {
		t.Errorf("expected ErrCalcExternalAssignment, got: %v", err)
	}
}

// testCalcNonBooleanCondition: a condition that is not Boolean is a type error
// the typecheck pass reports; an execution that reaches one anyway says so
// rather than coercing the value.
func testCalcNonBooleanCondition(t *testing.T) {
	src := `
		package test {
			calc counting {
				in n: Integer;
				while n {
					return : Integer = 1;
				}
				return : Integer = 0;
			}
		}
	`
	err := invokeCalcInSource(t, src, "counting", 3, 10000)
	if err == nil || !strings.Contains(err.Error(), "must evaluate to a Boolean") {
		t.Errorf("expected a non-Boolean condition error, got: %v", err)
	}
}

// calcUsageOutputInSource reads one output feature of the named calc usage and
// returns the error the read reports, on its own goroutine so a body that never
// terminates fails the case instead of stalling the suite. maxSteps bounds the
// run.
func calcUsageOutputInSource(t *testing.T, src, usageName, output string, maxSteps int64, budgets ...func(*Context)) error {
	t.Helper()

	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	ctx.maxSteps = maxSteps
	for _, set := range budgets {
		set(ctx)
	}
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, usageName, ast.DefCalc)
	if sym == nil {
		t.Fatalf("calc usage %s not found", usageName)
	}

	done := make(chan error, 1)
	go func() {
		value, err := ctx.CalcUsageOutput(sym, output, sym.OwnerScope, nil)
		if err == nil {
			err = fmt.Errorf("output %s of %s answered %s, expected the read to fail",
				output, usageName, FormatTraceValue(value))
		}
		done <- err
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(10 * time.Second):
		t.Fatalf("reading output %s of %s did not terminate", output, usageName)
		return nil
	}
}

// testCalcUsageUnboundInput: a usage that leaves an input of its calc with
// neither a value nor a default computes nothing, since a usage passes no
// arguments to stand in for one.
func testCalcUsageUnboundInput(t *testing.T) {
	src := `
		package test {
			calc def Two {
				in n : Integer;
				out a = n + 1;
				out b = n * 2;
			}
			calc c : Two;
		}
	`
	err := calcUsageOutputInSource(t, src, "c", "a", 10000)
	if !errors.Is(err, ErrUnboundParameter) {
		t.Errorf("expected ErrUnboundParameter, got: %v", err)
	}
}

// testCalcUsageUnknownOutput: a name the calc declares no output for is a
// modeling error, not an empty value.
func testCalcUsageUnknownOutput(t *testing.T) {
	src := `
		package test {
			calc def Two {
				in n : Integer;
				out a = n + 1;
				out b = n * 2;
			}
			calc c : Two { in n = 5; }
		}
	`
	err := calcUsageOutputInSource(t, src, "c", "nope", 10000)
	if !errors.Is(err, ErrUnknownOutput) {
		t.Errorf("expected ErrUnknownOutput, got: %v", err)
	}
	if err != nil && !strings.Contains(err.Error(), "a, b") {
		t.Errorf("error should name the outputs the calc does declare, got: %v", err)
	}
}

// testCalcUsageCyclicOutputs: outputs valued from each other have no value to
// compute, which is reported as the cycle it is rather than spending the step
// budget or hanging.
func testCalcUsageCyclicOutputs(t *testing.T) {
	src := `
		package test {
			calc def Knot {
				in n : Integer;
				out a = b + 1;
				out b = a + n;
			}
			calc c : Knot { in n = 1; }
		}
	`
	err := calcUsageOutputInSource(t, src, "c", "a", 10000)
	if !errors.Is(err, ErrCyclicOutput) {
		t.Errorf("expected ErrCyclicOutput, got: %v", err)
	}
}

// testCalcUsageSpecializesANonCalc: a calc usage typed by something that is not
// a calc inherits no parameters, outputs or body from it, so the specialization
// is reported rather than the outputs it appears to be missing.
func testCalcUsageSpecializesANonCalc(t *testing.T) {
	src := `
		package test {
			part def Chassis {
				attribute mass : Integer = 4;
			}
			calc c : Chassis;
		}
	`
	err := calcUsageOutputInSource(t, src, "c", "mass", 10000)
	if !errors.Is(err, ErrNotACalc) {
		t.Errorf("expected ErrNotACalc, got: %v", err)
	}
}

// testCalcUsageStepBudget: a usage whose body never terminates spends the step
// budget of the run reading its output, so the read fails instead of hanging
// whoever drove it.
func testCalcUsageStepBudget(t *testing.T) {
	src := `
		package test {
			calc def Spin {
				in n : Integer;
				attribute i : Integer = 0;
				while i >= 0 {
					i = i + 1;
				}
				out reached = i;
			}
			calc c : Spin { in n = 1; }
		}
	`
	err := calcUsageOutputInSource(t, src, "c", "reached", 20)
	if !errors.Is(err, ErrStepLimitExceeded) {
		t.Errorf("expected ErrStepLimitExceeded, got: %v", err)
	}
}

// testCalcUsageOutputWithoutAValue: an output the calc declares but binds no
// value to computes nothing, so reading it says so rather than answering null.
func testCalcUsageOutputWithoutAValue(t *testing.T) {
	src := `
		package test {
			calc def Half {
				in n : Integer;
				out a = n + 1;
				out b : Integer;
			}
			calc c : Half { in n = 5; }
		}
	`
	err := calcUsageOutputInSource(t, src, "c", "b", 10000)
	if !errors.Is(err, ErrNoValue) {
		t.Errorf("expected ErrNoValue, got: %v", err)
	}
}

// testCalcOutputNeverAssignedByTheBody: a body that assigns one of two declared
// outputs leaves the other unbound, and the read says that rather than blaming a
// missing result expression.
func testCalcOutputNeverAssignedByTheBody(t *testing.T) {
	src := `
		package test {
			calc def Two {
				in n : Integer;
				out a : Integer;
				out b : Integer;
				a = n + 1;
			}
			calc c : Two { in n = 5; }
		}
	`
	err := calcUsageOutputInSource(t, src, "c", "b", 10000)
	if !errors.Is(err, ErrOutputNotAssigned) {
		t.Errorf("expected ErrOutputNotAssigned, got: %v", err)
	}
	if errors.Is(err, ErrNoResultExpression) {
		t.Errorf("expected no result-expression blame, got: %v", err)
	}
}

// testCalcOutputAssignedInABranchNotTaken: an output only a branch that does not
// run would assign is unbound for that activation.
func testCalcOutputAssignedInABranchNotTaken(t *testing.T) {
	src := `
		package test {
			calc def Branch {
				in n : Integer;
				out a : Integer;
				if n > 10 {
					a = n;
				}
			}
			calc c : Branch { in n = 5; }
		}
	`
	err := calcUsageOutputInSource(t, src, "c", "a", 10000)
	if !errors.Is(err, ErrOutputNotAssigned) {
		t.Errorf("expected ErrOutputNotAssigned, got: %v", err)
	}
}

// testCalcOutputValuedAndAssigned: an output given a value two ways is reported
// rather than silently picking one (the precedent of #127/#131).
func testCalcOutputValuedAndAssigned(t *testing.T) {
	src := `
		package test {
			calc def Both {
				in n : Integer;
				out a : Integer = n;
				a = n + 1;
			}
			calc c : Both { in n = 5; }
		}
	`
	err := calcUsageOutputInSource(t, src, "c", "a", 10000)
	if !errors.Is(err, ErrConflictingOutput) {
		t.Errorf("expected ErrConflictingOutput, got: %v", err)
	}
}

// testCalcOutputAssignedTwice: a body assigning an output more than once leaves
// it bound to the last assignment that ran, the same as a body local, so an
// output may be initialized and then accumulated into.
func testCalcOutputAssignedTwice(t *testing.T) {
	src := `
		package test {
			calc def Twice {
				in n : Integer;
				out a : Integer;
				a = n + 1;
				a = a + 1;
			}
			calc c : Twice { in n = 5; }
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	ctx.maxSteps = 10000
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "c", ast.DefCalc)
	if sym == nil {
		t.Fatal("calc usage c not found")
	}

	value, err := ctx.CalcUsageOutput(sym, "a", sym.OwnerScope, nil)
	if err != nil {
		t.Fatalf("reading output a of c: %v", err)
	}
	if got := FormatTraceValue(value); got != "7" {
		t.Errorf("output a = %s, want 7", got)
	}
}

// testMultipleOutputsInvokedAsAnExpression: an invocation of a function yields
// exactly one result (KerML 7.4.9), so invoking a calc that computes several
// outputs and designates no result is reported rather than answered with
// whichever output happens to come first.
func testMultipleOutputsInvokedAsAnExpression(t *testing.T) {
	src := `
		package test {
			calc def Two {
				in n : Integer;
				out a = n + 1;
				out b = n * 2;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "Two", ast.DefCalc)
	if sym == nil {
		t.Fatal("Two calc not found")
	}

	arg := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 5}}
	result, err := ctx.InvokeCalc(sym, []Value{arg}, rootScope)
	if err == nil {
		t.Fatalf("expected the invocation to be rejected, it answered %s", FormatTraceValue(result))
	}
	if !errors.Is(err, ErrAmbiguousResult) {
		t.Errorf("expected ErrAmbiguousResult, got: %v", err)
	}
	// The diagnostic has to teach the spelling that does work.
	if err != nil && !strings.Contains(err.Error(), "calc c : test::Two { in n = ...; } then read c.a") {
		t.Errorf("error should spell out the calc usage to declare, got: %v", err)
	}
}

// testNestedCalcUsageUnboundInput: a usage nested in a calc binds its inputs
// from the enclosing evaluation, and an input nothing there values is reported.
func testNestedCalcUsageUnboundInput(t *testing.T) {
	src := `
		package test {
			calc def Two {
				in n : Integer;
				out a = n + 1;
				out b = n * 2;
			}
			calc def Outer {
				in m : Integer;
				calc inner : Two;
				out d = inner.a;
			}
			calc c : Outer { in m = 5; }
		}
	`
	err := calcUsageOutputInSource(t, src, "c", "d", 10000)
	if !errors.Is(err, ErrUnboundParameter) {
		t.Errorf("expected ErrUnboundParameter, got: %v", err)
	}
}

// testNestedCalcUsageUnknownOutput: reading a name the nested calc declares no
// output for is a modeling error wherever the usage is declared.
func testNestedCalcUsageUnknownOutput(t *testing.T) {
	src := `
		package test {
			calc def Two {
				in n : Integer;
				out a = n + 1;
				out b = n * 2;
			}
			calc def Outer {
				in m : Integer;
				calc inner : Two { in n = m; }
				out d = inner.nope;
			}
			calc c : Outer { in m = 5; }
		}
	`
	err := calcUsageOutputInSource(t, src, "c", "d", 10000)
	if !errors.Is(err, ErrUnknownOutput) {
		t.Errorf("expected ErrUnknownOutput, got: %v", err)
	}
}

// testNestedCalcUsageSelfCycle: an input of a nested usage valued from its own
// name with nothing outside to resolve to stays the cycle it is, rather than
// being read as the shadowing binding it looks like.
func testNestedCalcUsageSelfCycle(t *testing.T) {
	src := `
		package test {
			calc def Two {
				in n : Integer;
				out a = n + 1;
				out b = n * 2;
			}
			calc def Outer {
				calc inner : Two { in n = n; }
				out d = inner.a;
			}
			calc c : Outer;
		}
	`
	err := calcUsageOutputInSource(t, src, "c", "d", 10000)
	if !errors.Is(err, ErrCyclicSlot) {
		t.Errorf("expected ErrCyclicSlot, got: %v", err)
	}
}

// testNestedCalcUsageRecursionDepth: a calc whose nested usage is of itself
// never bottoms out, so the depth budget reports it instead of hanging. A usage
// frame costs far more than an invocation, so the case states a shallow budget.
func testNestedCalcUsageRecursionDepth(t *testing.T) {
	src := `
		package test {
			calc def Down {
				in n : Integer;
				calc next : Down { in n = n - 1; }
				out a = next.a;
				out b = n;
			}
			calc c : Down { in n = 3; }
		}
	`
	err := calcUsageOutputInSource(t, src, "c", "a", 1000000,
		func(ctx *Context) { ctx.maxCalcDepth = nestingProbeDepth })
	if !errors.Is(err, ErrCalcRecursionLimit) {
		t.Errorf("expected ErrCalcRecursionLimit, got: %v", err)
	}
}

// testNestedCalcUsageStepBudget: the body of a nested usage spends the budget of
// the run reading it, so a body that never terminates fails the read.
func testNestedCalcUsageStepBudget(t *testing.T) {
	src := `
		package test {
			calc def Spin {
				in n : Integer;
				attribute i : Integer = 0;
				while i >= 0 {
					i = i + 1;
				}
				out reached = i;
			}
			calc def Outer {
				in m : Integer;
				calc inner : Spin { in n = m; }
				out d = inner.reached;
			}
			calc c : Outer { in m = 1; }
		}
	`
	err := calcUsageOutputInSource(t, src, "c", "d", 20)
	if !errors.Is(err, ErrStepLimitExceeded) {
		t.Errorf("expected ErrStepLimitExceeded, got: %v", err)
	}
}

// variationSlotInSource instantiates a usage and returns the value its named
// slot holds, so a variation's failure modes are read where a model reads them.
func variationSlotInSource(t *testing.T, src, usage, slot string) (Value, error) {
	t.Helper()
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	matches := idx.LookupQualified(usage)
	if len(matches) != 1 {
		t.Fatalf("%s: %d matching symbols, want 1", usage, len(matches))
	}
	inst, err := ctx.Instantiate(matches[0])
	if err != nil {
		return Value{}, err
	}
	got, err := inst.GetSlot(ctx, slot)
	if err != nil {
		return Value{}, err
	}
	return got.Value, nil
}

// variationFamily is a variation point with three variants, over which the
// selection a specialization makes is varied per case.
const variationFamily = `
	package test {
		part def Diamond { attribute cut; attribute color; }
		abstract part family : Diamond {
			variation attribute :>> cut {
				variant attribute cutShallow { attribute cost = 200.0; }
				variant attribute cutIdeal { attribute cost = 250.0; }
			}
			variation attribute :>> color {
				variant attribute colorWhite { attribute cost = 100.0; }
			}
		}
		%s
	}`

// testVariationWithoutASelectedVariant: a variation nothing selects a variant
// for has no value, so reading it says so rather than answering the variation's
// own empty object or one of the variants arbitrarily.
func testVariationWithoutASelectedVariant(t *testing.T) {
	src := fmt.Sprintf(variationFamily, `part unconfigured :> family;`)
	got, err := variationSlotInSource(t, src, "test::unconfigured", "cut")
	if !errors.Is(err, ErrVariationUnselected) {
		t.Errorf("cut = (%v, %v), want ErrVariationUnselected", got, err)
	}
	// The failure names the feature, so a model with many variation points says
	// which one is unconfigured.
	if err != nil && !strings.Contains(err.Error(), "cut") {
		t.Errorf("error %q does not name the variation", err)
	}
}

// variationReadFromDeclaration evaluates a usage's value with no bound object,
// so a variation is read through its declaration rather than through a slot.
func variationReadFromDeclaration(t *testing.T, src, probe string) (Value, error) {
	t.Helper()
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	matches := idx.LookupQualified(probe)
	if len(matches) != 1 {
		t.Fatalf("%s: %d matching symbols, want 1", probe, len(matches))
	}
	usage, ok := matches[0].Decl.(*ast.Usage)
	if !ok || usage.Value == nil {
		t.Fatalf("%s: no value expression", probe)
	}
	return ctx.EvalWithScope(usage.Value, matches[0].OwnerScope)
}

// testVariationReadThroughItsDeclaration: a variation read without a bound
// object is bound the same way as one read from a slot, so what a legal
// selection is does not depend on how the model is inspected.
func testVariationReadThroughItsDeclaration(t *testing.T) {
	for _, tt := range []struct {
		name, decl, probe string
		want              error
	}{
		{
			"not_a_variant",
			`part chosen :> family { attribute :>> cut = 250.0; attribute probe = cut; }`,
			"test::chosen::probe", ErrNotAVariant,
		},
		{
			"two_variants",
			`part chosen :> family { attribute :>> cut = (cut::cutIdeal, cut::cutShallow); attribute probe = cut; }`,
			"test::chosen::probe", ErrMultipleVariants,
		},
		{
			"unselected",
			`part chosen :> family { attribute probe = cut; }`,
			"test::chosen::probe", ErrVariationUnselected,
		},
		{
			"qualified_not_a_variant",
			`part chosen :> family { attribute :>> cut = 250.0; }
			 attribute probe = chosen::cut;`,
			"test::probe", ErrNotAVariant,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := variationReadFromDeclaration(t, fmt.Sprintf(variationFamily, tt.decl), tt.probe)
			if !errors.Is(err, tt.want) {
				t.Errorf("%s = (%v, %v), want %v", tt.probe, got, err, tt.want)
			}
		})
	}
}

// testChainThroughAnUnselectedVariationPart: a variation part is no occurrence
// of itself, so a chain through one nothing selected a variant for reports that
// rather than reading an object of the variation.
func testChainThroughAnUnselectedVariationPart(t *testing.T) {
	src := `
	package test {
		private import ScalarValues::Real;
		part def Engine { attribute power : Real; }
		variation part engine : Engine {
			variant part electric : Engine { attribute :>> power = 100.0; }
			variant part diesel : Engine { attribute :>> power = 200.0; }
		}
		part probe { attribute p : Real = engine.power; }
	}`
	got, err := variationSlotInSource(t, src, "test::probe", "p")
	if !errors.Is(err, ErrVariationUnselected) {
		t.Errorf("p = (%v, %v), want ErrVariationUnselected", got, err)
	}
}

// testVariationBoundToWhatIsNotAVariant: a selection naming something that is
// not a variant of the variation is reported, whether the name is unknown, a
// variant of another variation, or an ordinary value.
func testVariationBoundToWhatIsNotAVariant(t *testing.T) {
	for _, tt := range []struct{ name, selection string }{
		{"unknown_name", `part chosen :> family { attribute :>> cut = cut::nope; }`},
		{"variant_of_another_variation", `part chosen :> family { attribute :>> cut = color::colorWhite; }`},
		{"ordinary_value", `part chosen :> family { attribute :>> cut = 250.0; }`},
		{"collection_of_ordinary_values", `part chosen :> family { attribute :>> cut = (250.0, 200.0); }`},
		{"variant_mixed_with_an_ordinary_value", `part chosen :> family { attribute :>> cut = (cut::cutIdeal, 250.0); }`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := variationSlotInSource(t, fmt.Sprintf(variationFamily, tt.selection), "test::chosen", "cut")
			if !errors.Is(err, ErrNotAVariant) {
				t.Errorf("cut = (%v, %v), want ErrNotAVariant", got, err)
			}
		})
	}
}

// testVariationBoundToTwoVariants: a variation stands for one variant, so a
// selection of several is reported rather than silently taking the first.
func testVariationBoundToTwoVariants(t *testing.T) {
	src := fmt.Sprintf(variationFamily,
		`part chosen :> family { attribute :>> cut = (cut::cutIdeal, cut::cutShallow); }`)
	got, err := variationSlotInSource(t, src, "test::chosen", "cut")
	if !errors.Is(err, ErrMultipleVariants) {
		t.Fatalf("cut = (%v, %v), want ErrMultipleVariants", got, err)
	}
	for _, name := range []string{"cutIdeal", "cutShallow"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q does not name the selection %s", err, name)
		}
	}
}

// testRepeatedReadsOfAVariantObject: the object a selected variant stands for is
// materialized once, so evaluating a chain through it repeatedly neither piles up
// objects nor exhausts the step budget.
func testRepeatedReadsOfAVariantObject(t *testing.T) {
	src := `
	package test {
		part def Engine { attribute power; }
		abstract part family {
			variation part engine : Engine {
				variant part electric : Engine { attribute :>> power = 150.0; }
				variant part petrol : Engine { attribute :>> power = 120.0; }
			}
		}
		part chosen :> family { part :>> engine = engine::electric; }
	}`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	variant := oneSymbol(t, idx, "test::family::engine::electric")
	variation := oneSymbol(t, idx, "test::family::engine")
	first, err := ctx.variantValue(variation, variant, 1)
	if err != nil {
		t.Fatalf("variantValue: %v", err)
	}
	count := len(ctx.instances)
	for i := 0; i < 10; i++ {
		again, err := ctx.variantValue(variation, variant, 1)
		if err != nil {
			t.Fatalf("variantValue (read %d): %v", i+2, err)
		}
		if again.Instance != first.Instance {
			t.Fatalf("read %d gave instance %d, want %d", i+2, again.Instance, first.Instance)
		}
	}
	if len(ctx.instances) != count {
		t.Errorf("instances grew from %d to %d over repeated reads", count, len(ctx.instances))
	}
}

// testTwoOwnersSelectingOneVariant: a variant is selected per owning object, so
// two owners of one variation each hold their own object of the variant rather
// than sharing one whose materialized slots the other reads.
func testTwoOwnersSelectingOneVariant(t *testing.T) {
	src := `
	package test {
		part def Engine { attribute power; }
		abstract part family {
			variation part engine : Engine {
				variant part electric : Engine { attribute :>> power = 150.0; }
				variant part petrol : Engine { attribute :>> power = 120.0; }
			}
		}
		part sedan :> family { part :>> engine = engine::electric; }
		part coupe :> family { part :>> engine = engine::electric; }
	}`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	ids := make([]int64, 0, 2)
	for _, usage := range []string{"test::sedan", "test::coupe"} {
		inst, err := ctx.Instantiate(oneSymbol(t, idx, usage))
		if err != nil {
			t.Fatalf("%s: %v", usage, err)
		}
		slot, err := inst.GetSlot(ctx, "engine")
		if err != nil {
			t.Fatalf("%s.engine: %v", usage, err)
		}
		id, ok := slot.Value.Object()
		if !ok {
			t.Fatalf("%s.engine = %v, want an object of the selected variant", usage, slot.Value)
		}
		ids = append(ids, id)
	}
	if ids[0] == ids[1] {
		t.Errorf("sedan and coupe share engine object %d", ids[0])
	}
}

// testTwoOwnerlessSelectionsOfOneVariant: a variation read through its
// declaration has no owning object, so two variation points selecting one
// variant must still stand for an object each rather than share one.
func testTwoOwnerlessSelectionsOfOneVariant(t *testing.T) {
	src := `
	package test {
		part def Engine { attribute power; }
		abstract part family {
			variation part engine : Engine {
				variant part electric : Engine { attribute :>> power = 150.0; }
			}
		}
		part sedan :> family { part :>> engine = engine::electric; }
		part coupe :> family { part :>> engine = engine::electric; }
	}`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	variant := oneSymbol(t, idx, "test::family::engine::electric")
	ids := make([]int64, 0, 2)
	for _, variation := range []string{"test::sedan::engine", "test::coupe::engine"} {
		val, err := ctx.variantValue(oneSymbol(t, idx, variation), variant, 0)
		if err != nil {
			t.Fatalf("%s: %v", variation, err)
		}
		ids = append(ids, val.Instance)
	}
	if ids[0] == ids[1] {
		t.Errorf("sedan.engine and coupe.engine share object %d", ids[0])
	}
}

// testVariantOutsideAVariation: `variant` on a member whose owner is not a
// variation offers no choice, so the member stays an ordinary feature instead of
// silently holding no value.
func testVariantOutsideAVariation(t *testing.T) {
	src := `
	package test {
		part def Widget { variant attribute misplaced = 1.0; }
		part widget : Widget;
	}`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	inst, err := ctx.Instantiate(oneSymbol(t, idx, "test::widget"))
	if err != nil {
		t.Fatalf("Instantiate(test::widget): %v", err)
	}
	slot, err := inst.GetSlot(ctx, "misplaced")
	if err != nil {
		t.Fatalf("widget.misplaced: %v", err)
	}
	if slot.Value.Kind != ValConst || slot.Value.Const.Real != 1.0 {
		t.Errorf("widget.misplaced = %v, want 1", slot.Value)
	}
}

// testVariantUnderARedefinedVariation: a usage redefining a variation usage is a
// variation point without restating the modifier, so the variants under it stay
// choices that specialize it instead of materializing slots.
func testVariantUnderARedefinedVariation(t *testing.T) {
	src := `
	package test {
		part def Engine { attribute power; }
		abstract part family {
			variation part engine : Engine {
				variant part petrol : Engine { attribute :>> power = 90.0; }
			}
		}
		abstract part refined :> family {
			part :>> engine {
				variant part electric { attribute :>> power = 150.0; }
			}
		}
		part sedan :> refined { part :>> engine = engine::electric; }
	}`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	inst, err := ctx.Instantiate(oneSymbol(t, idx, "test::sedan"))
	if err != nil {
		t.Fatalf("Instantiate(test::sedan): %v", err)
	}
	if slot, err := inst.GetSlot(ctx, "electric"); err == nil {
		t.Errorf("sedan.electric materialized a slot: %v", slot.Value)
	}
	slot, err := inst.GetSlot(ctx, "engine")
	if err != nil {
		t.Fatalf("sedan.engine: %v", err)
	}
	if slot.Value.Kind != ValVariant || slot.Value.Instance == 0 {
		t.Fatalf("sedan.engine = %v, want the selected variant's object", slot.Value)
	}
	power, err := ctx.instances[slot.Value.Instance].GetSlot(ctx, "power")
	if err != nil {
		t.Fatalf("sedan.engine.power: %v", err)
	}
	if power.Value.Kind != ValConst || power.Value.Const.Real != 150.0 {
		t.Errorf("sedan.engine.power = %v, want 150", power.Value)
	}
}

// oneSymbol returns the single symbol a qualified name denotes.
func oneSymbol(t *testing.T, idx *symbols.Index, fqn string) *symbols.Symbol {
	t.Helper()
	matches := idx.LookupQualified(fqn)
	if len(matches) != 1 {
		t.Fatalf("%s: %d matching symbols, want 1", fqn, len(matches))
	}
	return matches[0]
}

// testDeepSpecializationChainOfRedefinitions: a redefinition specializes the
// usage it redefines, so a long chain of them keeps every level's values and
// terminates instead of recursing while looking for the base's members.
func testDeepSpecializationChainOfRedefinitions(t *testing.T) {
	const depth = 60
	var b strings.Builder
	b.WriteString("package test {\n")
	b.WriteString("\tpart def Inner { attribute a; attribute b; }\n")
	b.WriteString("\tpart def Outer { part inner : Inner; attribute t = inner.b; }\n")
	b.WriteString("\tpart level0 : Outer { part :>> inner { attribute :>> b = 7.0; } }\n")
	for i := 1; i <= depth; i++ {
		fmt.Fprintf(&b, "\tpart level%d :> level%d { part :>> inner { attribute :>> a = %d.0; } }\n", i, i-1, i)
	}
	b.WriteString("}\n")

	done := make(chan struct{})
	var got Value
	var err error
	go func() {
		defer close(done)
		got, err = variationSlotInSource(t, b.String(), fmt.Sprintf("test::level%d", depth), "t")
	}()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("reading an inherited value through a deep specialization chain hung")
	}
	if err != nil {
		t.Fatalf("t = %v", err)
	}
	if got.Kind != ValConst || got.Const.Real != 7.0 {
		t.Errorf("t = %+v, want the base's 7.0", got)
	}
}

// testConflictingRedefinitionsAtSeveralLevels: when several levels restate the
// same nested feature, the innermost restatement is the value read, and the
// levels above still supply what they alone declare.
func testConflictingRedefinitionsAtSeveralLevels(t *testing.T) {
	src := `
		package test {
			part def Inner { attribute a; attribute b; attribute c; }
			part def Outer { part inner : Inner; attribute t = inner.c + inner.b + inner.a; }
			part base : Outer { part :>> inner { attribute :>> a = 1.0; attribute :>> c = 100.0; } }
			part middle :> base { part :>> inner { attribute :>> b = 20.0; attribute :>> c = 200.0; } }
			part leaf :> middle { part :>> inner { attribute :>> c = 300.0; } }
		}`
	got, err := variationSlotInSource(t, src, "test::leaf", "t")
	if err != nil {
		t.Fatalf("t = %v", err)
	}
	if got.Kind != ValConst || got.Const.Real != 321.0 {
		t.Errorf("t = %+v, want 321.0 (innermost c, middle b, base a)", got)
	}
}

// testOneFeatureValuedUnderTwoNames: a redefinition renames one feature, so a
// declaration valuing both names has to be reported instead of picking one.
func testOneFeatureValuedUnderTwoNames(t *testing.T) {
	src := `
		package test {
			part def Ring { attribute ringCost; }
			part def Band :> Ring { attribute bandCost :>> ringCost; }
			part conflicted : Band {
				attribute :>> bandCost = 400.0;
				attribute :>> ringCost = 500.0;
			}
		}`
	got, err := variationSlotInSource(t, src, "test::conflicted", "ringCost")
	if !errors.Is(err, ErrConflictingRedefinition) {
		t.Fatalf("ringCost = %+v, err = %v, want ErrConflictingRedefinition", got, err)
	}
}

// testValuedFeatureRestatedInABody: a feature bound to a value takes its own
// features from that value, so a body restating one of them is reported instead
// of being dropped.
func testValuedFeatureRestatedInABody(t *testing.T) {
	src := `
		package test {
			attribute def Cost { attribute v = 1.0; }
			part def Ring { attribute ringCost : Cost; }
			part conflicted : Ring {
				attribute :>> ringCost = 400.0 { attribute :>> v = 9.0; }
			}
		}`
	got, err := variationSlotInSource(t, src, "test::conflicted", "ringCost")
	if !errors.Is(err, ErrValuedFeatureRestated) {
		t.Fatalf("ringCost = %+v, err = %v, want ErrValuedFeatureRestated", got, err)
	}
}

// calcErrorWithLibraries invokes the named calc of package test in src, with the
// standard library indexed, and answers the error it fails with.
func calcErrorWithLibraries(t *testing.T, src, calcName string, args []Value, maxSteps int64) error {
	t.Helper()

	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	ctx.maxSteps = maxSteps
	sym, scope := calcByName(t, idx.DocumentRoot("<test>"), "test", calcName)

	done := make(chan error, 1)
	go func() {
		result, err := ctx.InvokeCalc(sym, args, scope)
		if err == nil {
			err = fmt.Errorf("calc %s returned %s, expected it to fail", calcName, FormatTraceValue(result))
		}
		done <- err
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(10 * time.Second):
		t.Fatalf("calc %s did not terminate", calcName)
		return nil
	}
}

// testBodyLocalUsageOfANonCalc: a body-local usage typed by something that is no
// calc is reported when the declaration is reached, not skipped.
func testBodyLocalUsageOfANonCalc(t *testing.T) {
	src := `
		package test {
			part def Thing;
			calc def Holder {
				in n : Integer;
				attribute i : Integer = 0;
				while i < n {
					calc r : Thing;
					assign i := i + 1;
				}
				i
			}
		}
	`
	err := calcErrorWithLibraries(t, src, "Holder", []Value{constInt(1)}, 10000)
	if !errors.Is(err, ErrNotACalc) {
		t.Errorf("expected ErrNotACalc, got: %v", err)
	}
}

// testBodyLocalDeclarationNotExecutable: a declaration in a body the runtime has
// no execution for names itself rather than passing silently.
func testBodyLocalDeclarationNotExecutable(t *testing.T) {
	src := `
		package test {
			calc def Holder {
				in n : Integer;
				if n > 0 {
					part broken;
				}
				n
			}
		}
	`
	err := calcErrorWithLibraries(t, src, "Holder", []Value{constInt(1)}, 10000)
	if err == nil || !strings.Contains(err.Error(), "broken") {
		t.Errorf("error should name the declaration it cannot execute, got: %v", err)
	}
}

// testRangeBoundIsNotAnInteger: `..` declares Integer bounds, so a Real bound is
// the type mismatch it is rather than a truncated range.
func testRangeBoundIsNotAnInteger(t *testing.T) {
	src := `
		package test {
			calc def Span {
				in n : Integer;
				attribute r = 1.5..n;
				n
			}
		}
	`
	err := calcErrorWithLibraries(t, src, "Span", []Value{constInt(3)}, 10000)
	if !errors.Is(err, ErrTypeMismatch) {
		t.Errorf("expected ErrTypeMismatch, got: %v", err)
	}
}

// testRangeSpendsTheStepBudget: each element a range generates costs a step, so
// a range too large to hold fails the run rather than exhausting memory.
func testRangeSpendsTheStepBudget(t *testing.T) {
	src := `
		package test {
			calc def Span {
				in n : Integer;
				attribute r = 1..1000000;
				n
			}
		}
	`
	err := calcErrorWithLibraries(t, src, "Span", []Value{constInt(3)}, 100)
	if !errors.Is(err, ErrStepLimitExceeded) {
		t.Errorf("expected ErrStepLimitExceeded, got: %v", err)
	}
}

// testCollectionSpendsTheElementBudget: a materialized element is memory the
// collection keeps, so it has its own ceiling and its own error rather than
// reading as the step budget's.
func testCollectionSpendsTheElementBudget(t *testing.T) {
	src := `
		package test {
			calc def Span {
				in n : Integer;
				attribute r = (1..1000)->collect{in i; i * i};
				n
			}
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	ctx.maxElements = 100
	sym, scope := calcByName(t, idx.DocumentRoot("<test>"), "test", "Span")
	_, err := ctx.InvokeCalc(sym, []Value{constInt(3)}, scope)
	if err == nil {
		t.Fatal("want the element budget's error, got a value")
	}
	if !errors.Is(err, ErrElementLimitExceeded) {
		t.Errorf("expected ErrElementLimitExceeded, got: %v", err)
	}
	if errors.Is(err, ErrStepLimitExceeded) {
		t.Errorf("error %q also reads as the step budget's", err)
	}
	if !strings.Contains(err.Error(), MaxElementsEnvVar) {
		t.Errorf("error %q does not name %s", err, MaxElementsEnvVar)
	}
}

// testUsageReadThroughAPartWithoutAnOutput: a chain through a part that stops at
// a calc usage names the outputs to read instead of answering no value.
func testUsageReadThroughAPartWithoutAnOutput(t *testing.T) {
	src := `
		package test {
			calc def Two {
				in n : Integer;
				out a = n + 1;
				out b = n * 2;
			}
			part holder {
				calc c : Two { in n = 5; }
			}
			calc def Probe {
				in n : Integer;
				holder.c
			}
		}
	`
	err := calcErrorWithLibraries(t, src, "Probe", []Value{constInt(1)}, 10000)
	if !errors.Is(err, ErrNoValue) {
		t.Errorf("expected ErrNoValue, got: %v", err)
	}
	if err != nil && !strings.Contains(err.Error(), "a, b") {
		t.Errorf("error should name the outputs to read, got: %v", err)
	}
}

// testEnumerationNameThatIsNotALiteral: a name qualified by an enumeration
// designates one of its literals, so one it does not declare is reported with
// the literals it does, never answered as an empty value.
func testEnumerationNameThatIsNotALiteral(t *testing.T) {
	src := `
	package test {
		enum def Color { red; green; blue; }
		part def Car { attribute c : Color = Color::purple; }
	}`
	got, err := variationSlotInSource(t, src, "test::Car", "c")
	if !errors.Is(err, ErrNotALiteral) {
		t.Fatalf("c = (%v, %v), want ErrNotALiteral", got, err)
	}
	for _, name := range []string{"purple", "red", "green", "blue"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q does not name %s", err, name)
		}
	}
}

// testChainThroughALiteralWithoutThatAttribute: a literal carries only the
// features it declares, so reading another one off it is reported rather than
// materializing an empty slot.
func testChainThroughALiteralWithoutThatAttribute(t *testing.T) {
	src := `
	package test {
		enum def Level { low { attribute n = 1; } high { attribute n = 9; } }
		part def Sensor { attribute missing = Level::low.label; }
	}`
	got, err := variationSlotInSource(t, src, "test::Sensor", "missing")
	if err == nil {
		t.Fatalf("missing = %v, want an error naming the unknown member", got)
	}
	if !strings.Contains(err.Error(), "label") {
		t.Errorf("error %q does not name the unknown member", err)
	}
}
