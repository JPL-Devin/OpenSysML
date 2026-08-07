package parser

import (
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
		{"message_payload_declaration_no_type", "message m of pay : from a to b;"},
		{"message_payload_declaration_no_target", "message m of pay : T from a;"},
		{"defer_no_event", "state s { defer ; }"},
		{"defer_no_semicolon", "state s { defer Ping state t; }"},
		{"defer_trailing_comma", "state s { defer Ping, ; }"},
		{"history_no_name", "state s { history ; }"},
		{"deep_without_history", "state s { deep resume; }"},
		{"shallow_without_history", "state s { shallow resume; }"},
		{"history_no_semicolon", "state s { history resume state t; }"},
		{"entry_point_no_name", "state s { entry point ; }"},
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
