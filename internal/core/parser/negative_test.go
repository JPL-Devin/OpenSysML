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
