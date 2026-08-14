package parser

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/source"
)

// TestNegative verifies parser REJECTS malformed input (Phase 2, Task 2.3)
// Guards against silently accepting garbage
func TestNegative(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"unclosed_brace", "part {"},
		{"empty_requirement", "requirement r { require ; }"},
		{"empty_expression", "attribute x = ;"},
		{"numeric_name", "part def 123;"},
		{"missing_semicolon", "part def Engine"},
		{"invalid_keyword_combo", "def usage MyPart;"},
		{"incomplete_connection", "connector c connect a"},
		{"unterminated_string", `part p { doc /* comment `},
		{"double_colon_only", "attribute ::x;"},

		// Behavioral negatives (Phase B1.2)
		{"state_entry_no_keyword", "state s { entry }"},
		{"action_dangling_fork", "action a { fork }"},
		{"transition_then_only", "transition first then"},
		{"requirement_empty_require", "requirement r { require }"},
		{"calc_empty_return", "calc c { return }"},
		{"calc_while_no_condition", "calc def C { while { i = i + 1; } }"},
		{"calc_while_unclosed_body", "calc def C { while i < 2 { i = i + 1; }"},
		{"calc_for_no_variable", "calc def C { for in xs { } }"},
		{"calc_assignment_no_value", "calc def C { i = ; }"},
		{"calc_if_no_body", "calc def C { if i < 2 }"},
		{"constraint_incomplete", "constraint c { assert }"},
		{"state_fork_no_name", "state s { fork ; }"},
		{"state_join_no_semicolon", "state s { join sync state t; }"},
		{"call_trigger_unclosed_params", "state s { accept op(a then t; }"},
		{"call_trigger_missing_param_name", "state s { accept op(,) then t; }"},
		{"perform_no_reference", "action a { perform ; }"},
		{"perform_dangling_chain", "action a { perform b.; }"},
		{"allocate_missing_target", "package q { allocate a to ; }"},
		{"message_payload_declaration_no_type", "message m of pay : from a to b;"},
		{"message_payload_declaration_no_target", "message m of pay : T from a;"},
		{"defer_no_event", "state s { defer ; }"},
		{"defer_no_semicolon", "state s { defer Ping state t; }"},
		{"defer_trailing_comma", "state s { defer Ping, ; }"},
		{"history_no_name", "state s { history ; }"},
		{"deep_without_history", "state s { deep resume; }"},
		{"shallow_without_history", "state s { shallow resume; }"},
		{"history_no_semicolon", "state s { history resume state t; }"},
		// `entry point ;` is not malformed: `point` is not reserved, so it is an
		// entry action referencing a feature named `point`. Dropping the ';' too
		// leaves a pseudostate declaration that is missing its name.
		{"entry_point_no_name", "state s { entry point }"},
		{"entry_reference_no_semicolon", "state s { entry warmUp state t; }"},
		{"exit_reference_no_semicolon", "state s { exit coolDown state t; }"},
		{"do_reference_dangling_chain", "state s { do warmUp.; }"},
		{"end_no_feature", "connection def C { end ; }"},
		{"end_unclosed_multiplicity", "connection def C { end [1 part bead : T; }"},
		{"connector_end_no_reference_target", "part p { connection : C connect bead references to rim; }"},
		{"flow_source_without_target", "part def C { item Fuel; part a; flow f of Fuel from a; }"},
		{"nary_connect_unclosed", "part def C { part a; part b; connection conn connect (a, b; }"},
		{"nary_connect_trailing_comma", "part def C { part a; part b; connection conn connect (a, b, ); }"},
		{"nary_connect_empty", "part def C { connection conn connect (); }"},
		{"anonymous_nary_connect_unclosed", "part def C { part a; part b; connect (a, b; }"},
		{"anonymous_nary_connect_empty", "part def C { connect (); }"},

		// Occurrence modifiers (`individual`, `snapshot`) on a usage.
		{"individual_modifier_no_member", "individual ;"},
		{"individual_usage_no_type", "individual testSystem : ;"},
		{"individual_usage_no_body", "individual testSystem : TestSystem"},
		{"snapshot_usage_no_type", "snapshot occurrence takeoff : ;"},
		{"individual_parameter_no_type", "action a { in individual v : ; }"},

		// A member-attached `then` sequences the members either side of it, so
		// a body with nothing on one side, or a member the notation does not
		// allow one before, declares no order and is rejected rather than
		// parsed with the keyword dropped. A `then` beside a member with no
		// name is legal notation, bound by position
		// (TestSuccessionBindsUnnamedEndsByPosition).
		{"leading_then_has_no_source", "action a { then action b; }"},
		{"trailing_then_has_no_target", "action a { action b; then }"},
		{"then_then", "action a { action b; then then action c; }"},
		{"then_before_definition", "action a { action b; then action def C; }"},
		{"then_before_package", "part def P { part a; then package Inner { } }"},
		{"then_before_attribute", "part def P { part a; then attribute x; }"},
		{"then_before_import", "part def P { part a; then import Other::*; }"},

		// A feature specialization keyword after a short name states a
		// relationship, so a missing target is an error rather than a name.
		{"short_name_redefines_no_target", "part p { attribute <sn> redefines; }"},
		{"short_name_redefines_symbol_no_target", "part p { attribute <sn> :>>; }"},
		{"short_name_defined_by_no_type", "part p { attribute <sn> defined by ; }"},

		// The notation has no definition of a rendering a view names, of a
		// concern a body frames, or of a stakeholder or actor: those keywords own
		// a usage, not a definition (SysML.xtext ViewRenderingUsage,
		// FramedConcernUsage, StakeholderUsage, ActorUsage). `render ;` and
		// `frame ;` are absent deliberately: `frame` and `render` are legal
		// names, so those declare a feature so named rather than being errors.
		{"render_definition", "view def V { render def R; }"},
		{"frame_definition", "viewpoint def V { frame def C; }"},
		{"stakeholder_definition", "stakeholder def Reviewer;"},
		{"actor_definition", "actor def Operator;"},
		{"stakeholder_no_declaration", "viewpoint def V { stakeholder ; }"},
		{"actor_no_declaration", "requirement def R { actor ; }"},
		// A rendering reference takes no value: ViewRenderingUsage has no
		// ValuePart, unlike the performed action reference that shares its shape.
		{"render_reference_value", "view def V { render r = 3; }"},

		// The sequence index and the collection notations: `#` indexes through a
		// parenthesized index and `.?` selects through a body, so each is
		// rejected where the notation it needs is absent rather than parsed as
		// the operand alone.
		{"index_no_paren", "attribute x = xs#3;"},
		{"index_no_index", "attribute x = xs#();"},
		{"index_unclosed", "attribute x = xs#(1;"},
		{"index_bracket_unclosed", "attribute x = 5 [m;"},
		{"index_bracket_empty", "attribute x = 5 [];"},
		{"select_no_body", "attribute x = xs.?;"},
		{"select_expression_body", "attribute x = xs.? x > 1;"},
		{"select_unclosed_body", "attribute x = xs.?{in x; x > 1;"},
		{"collect_unclosed_body", "attribute x = xs.{in x; x * 2;"},
		{"body_param_no_name", "attribute x = xs.{in ; 1};"},
		{"body_param_no_type", "attribute x = xs.{in y : ; 1};"},
		{"receiver_no_operation", "attribute x = xs->;"},
		{"receiver_unclosed_args", "attribute x = xs->union((1, 2);"},

		// `variation` and `variant` qualify a declaration, so each is rejected
		// where the declaration it qualifies is absent or malformed.
		{"variation_no_declaration", "variation ;"},
		{"variation_attribute_no_name", "part p { variation attribute : ; }"},
		{"variant_unclosed_body", "part p { variation attribute cut { variant attribute cutIdeal { :>> cost = 1.0; } }"},
		{"variant_selection_no_variant_name", "part p { attribute :>> cut = cut::; }"},

		// Behavioral notation: a flow states both of its ends, an accept its
		// payload, a loop the condition its `until` promises and a succession a
		// target, so each is reported where one is missing rather than read as
		// the shorter form it is not.
		{"flow_from_without_to", "action def A { action a; flow x from a; }"},
		{"flow_named_from_no_source", "action def A { flow x from to b; }"},
		{"accept_when_no_condition", "action def A { accept when; }"},
		{"accept_at_no_instant", "action def A { accept at; }"},
		{"accept_no_payload", "action def A { accept; }"},
		{"accept_subsets_no_event", "action def A { action i accept :>; }"},
		{"loop_until_no_condition", "action def A { loop action { } until; }"},
		{"loop_until_no_semicolon", "action def A { action b; then loop action { } until x }"},
		{"then_done_no_semicolon", "action def A { action b; then done }"},
		{"send_via_no_port", "action def A { send Data() via; }"},
		{"send_no_target", "action def A { send Data() to; }"},
		{"decision_else_no_target", "action def A { action m; first m; then decide; else; }"},
		{"transition_trigger_no_target", "state def S { state a; transition first a accept Ping; }"},
		{"transition_two_triggers", "state def S { state a; state b; transition first a accept Ping accept Pong then b; }"},
		{"transition_two_targets", "state def S { state a; state b; transition a to b then b; }"},
		{"transition_do_without_action", "state def S { state a; state b; transition first a do then b; }"},
		{"exhibit_state_unclosed_body", "part def P { exhibit state modes { state off; }"},
		{"namespace_succession_no_target", "package Q { part p; first p then; }"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sf := source.New(tt.name+".sysml", []byte(tt.input))
			p := New(sf)
			_ = p.ParseFile()

			if len(p.Diagnostics) == 0 {
				t.Errorf("Expected parse errors for malformed input, got none.\nInput: %s", tt.input)
			}
		})
	}
}

// An unterminated comment swallows the rest of the document, so the parser says
// so rather than silently returning a tree that is missing everything after it.
func TestUnterminatedCommentIsReported(t *testing.T) {
	for _, src := range []string{
		"part def A;\n/* oops",
		"part def A;\n//* oops",
		"part def A;\n/*/",
		"part def A;\n/* oops\npart def B;\n",
	} {
		p := New(source.New("t.sysml", []byte(src)))
		p.ParseFile()
		found := false
		for _, d := range p.Diagnostics {
			if strings.Contains(d.Message, "unterminated comment") {
				found = true
			}
		}
		if !found {
			t.Errorf("ParseFile(%q) diagnostics = %v, want an unterminated comment reported", src, p.Diagnostics)
		}
	}
	// A closed comment is not reported.
	p := New(source.New("t.sysml", []byte("part def A;\n/* fine */\n//* also fine */\n")))
	p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Errorf("a closed comment produced %v", p.Diagnostics)
	}
}
