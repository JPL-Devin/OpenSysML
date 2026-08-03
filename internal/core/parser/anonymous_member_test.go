package parser

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/source"
)

// TestAnonymousTypedMembers tests anonymous members with type-only syntax
// (subject: Type; end: Type; in: Type;) found in stdlib allowlist failures
func TestAnonymousTypedMembers(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			"anonymous_subject",
			`requirement def R { subject: Action; }`,
		},
		{
			"anonymous_end",
			`connection def C { end: Part; }`,
		},
		{
			"anonymous_param",
			`action def A { in: Real; out: Real; inout: Real; }`,
		},
		{
			"anonymous_param_with_multiplicity",
			`action def A { inout payload[0..*]; }`,
		},
		{
			"anonymous_param_with_subsetting",
			`action def A { in :>> receiver; }`,
		},
		{
			"requirement_self_redefinition",
			`requirement def R { ref requirement :>> self: RequirementCheck; }`,
		},
		{
			"calc_return_with_binding",
			`calc def C { return result = expr(); }`,
		},
		{
			"calc_return_with_binding_and_body",
			`calc def C { return result = allTrue(assumptions()) implies allTrue(constraints()) { doc /* test */ } }`,
		},
		{
			"constraint_return_with_binding",
			`constraint def C { return result = allTrue(assumptions()) implies allTrue(constraints()) { doc /* test */ } }`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sf := source.New(tt.name+".sysml", []byte(tt.input))
			p := New(sf)
			root := p.ParseFile()

			if len(p.Diagnostics) > 0 {
				t.Errorf("Unexpected parse errors:")
				for _, d := range p.Diagnostics {
					t.Logf("  %s", d.Message)
				}
				t.FailNow()
			}

			if root == nil {
				t.Fatal("ParseFile returned nil")
			}
		})
	}
}
