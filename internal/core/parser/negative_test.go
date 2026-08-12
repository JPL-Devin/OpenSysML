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
		// name is legal notation this representation cannot carry and warns
		// instead (TestSuccessionUnnamedEndWarns).
		{"leading_then_has_no_source", "action a { then action b; }"},
		{"trailing_then_has_no_target", "action a { action b; then }"},
		{"then_then", "action a { action b; then then action c; }"},
		{"then_before_definition", "action a { action b; then action def C; }"},
		{"then_before_package", "part def P { part a; then package Inner { } }"},
		{"then_before_attribute", "part def P { part a; then attribute x; }"},
		{"then_before_import", "part def P { part a; then import Other::*; }"},
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
