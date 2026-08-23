package parser

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// A feature value is written `= expr`, `:= expr` or `default` followed by either
// of them (KerML `FeatureValue`), wherever a feature can carry a value.
func TestFeatureValueOperators(t *testing.T) {
	cases := []struct {
		body string
		op   string
	}{
		{"attribute m = 10;", "="},
		{"attribute m := 10;", ":="},
		{"attribute m default 10;", "default"},
		{"attribute m default = 10;", "default ="},
		{"attribute m default := 10;", "default :="},
		{"attribute m default   = 10;", "default   ="},
		{"attribute m : Real default = 10;", "default ="},
		{"attribute m :> base default = 10;", "default ="},
	}
	for _, tc := range cases {
		t.Run(tc.body, func(t *testing.T) {
			src := "package P { attribute def Real; attribute base; part def D { " + tc.body + " } }"
			p := New(source.New("t.sysml", []byte(src)))
			root := p.ParseFile()
			if len(p.Diagnostics) != 0 {
				t.Fatalf("parse diagnostics: %v", p.Diagnostics)
			}
			usage := findUsageNamed(root, "m")
			if usage == nil {
				t.Fatal("attribute m not found")
			}
			if usage.Value == nil {
				t.Error("the feature value was not read")
			}
			if got := src[usage.ValueOperatorSpan.Offset:usage.ValueOperatorSpan.End()]; got != tc.op {
				t.Errorf("value operator span %q, want %q", got, tc.op)
			}
			if !strings.HasPrefix(src[usage.ValueOperatorSpan.Offset:], tc.op) {
				t.Errorf("value operator span starts at %d, not at %q", usage.ValueOperatorSpan.Offset, tc.op)
			}
		})
	}
}

// The same operators are read on the members whose value parts are parsed apart
// from an ordinary usage: parameters, results and a requirement's subject.
func TestFeatureValueOperatorsOnSpecialMembers(t *testing.T) {
	cases := []struct {
		name, src, param string
	}{
		{
			"parameter",
			"package P { action def A { in speed default = 10; } }",
			"speed",
		},
		{
			"result",
			"package P { calc def C { return total default = 0; } }",
			"total",
		},
		{
			"subject",
			"package P { part vehicle; requirement def R { subject target default = vehicle; } }",
			"target",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := New(source.New("t.sysml", []byte(tc.src)))
			p.ParseFile()
			if len(p.Diagnostics) != 0 {
				t.Fatalf("parse diagnostics: %v", p.Diagnostics)
			}
		})
	}
}
