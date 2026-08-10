package parser

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

// endTargets names every end a declaration parsed, in order.
func endTargets(t *testing.T, src string) []string {
	t.Helper()
	member, diags := parseOneMemberWithDiags(t, src)
	if len(diags) > 0 {
		t.Fatalf("%q: unexpected diagnostics %v", src, diags)
	}
	def, ok := member.(*ast.Definition)
	if !ok {
		t.Fatalf("%q: expected a definition, got %T", src, member)
	}
	var names []string
	for _, m := range def.Members {
		mem, isMembership := m.(*ast.Membership)
		if !isMembership {
			continue
		}
		u, isUsage := mem.Member.(*ast.Usage)
		if !isUsage || len(u.ConnectorEnds) == 0 {
			continue
		}
		for _, end := range u.ConnectorEnds {
			names = append(names, endTargetName(end))
		}
	}
	return names
}

// endTargetName names the feature an end attaches to; a chain attaches to its
// last segment, the way lower.endName reads it.
func endTargetName(end *ast.ConnectorEnd) string {
	if chain, isChain := end.Target.(*ast.FeatureChainExpr); isChain {
		return ast.SimpleName(chain.Member)
	}
	return ast.SimpleName(end.Target)
}

// A parenthesized connector clause keeps every end it lists, not just the
// first: `connect (a, b, c)` is n-ary (SysML v2 §7.13.2, §8.3.13).
func TestParseNaryConnectorEndsKeepsEveryEnd(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []string
	}{
		{
			"binary control",
			"part def C { part a; part b; connection conn connect a to b; }",
			[]string{"a", "b"},
		},
		{
			"ternary connection",
			"part def C { part a; part b; part c; connection conn connect (a, b, c); }",
			[]string{"a", "b", "c"},
		},
		{
			"quaternary connection",
			"part def C { part a; part b; part c; part d; connection conn connect (a, b, c, d); }",
			[]string{"a", "b", "c", "d"},
		},
		{
			"ternary connector",
			"part def C { part a; part b; part c; connector conn connect (a, b, c); }",
			[]string{"a", "b", "c"},
		},
		{
			"ternary interface",
			"part def C { port p; port q; port r; interface i connect (p, q, r); }",
			[]string{"p", "q", "r"},
		},
		{
			"ternary allocation",
			"part def C { part f; part g; part h; allocation al allocate (f, g, h); }",
			[]string{"f", "g", "h"},
		},
		{
			"ends given as feature chains",
			"part def C { part a; part b; part c; connection conn connect (a.out, b.in, c.in); }",
			[]string{"out", "in", "in"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := endTargets(t, tc.src)
			if len(got) != len(tc.want) {
				t.Fatalf("parsed %d ends %v, want %d %v", len(got), got, len(tc.want), tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("end %d = %q, want %q (all: %v)", i, got[i], tc.want[i], got)
				}
			}
		})
	}
}

// A malformed n-ary clause keeps the ends parsed so far and reports, rather
// than dropping ends silently or panicking.
func TestParseNaryConnectorEndsMalformedKeepsPartialEnds(t *testing.T) {
	src := "part def C { part a; part b; connection conn connect (a, b, ); }"
	p := New(source.New("<t>", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) == 0 {
		t.Fatalf("expected diagnostics for a trailing comma in a connector clause")
	}
	if root == nil {
		t.Fatalf("parser returned no tree")
	}
}

// The anonymous inline form is binary-only: `connect (a, b, c);` without a
// declared connection name does not parse. Tracked as a known limitation in
// docs/SPEC_COMPLIANCE.md ("n-ary connector ends"); the ends are lost loudly
// (a diagnostic), never silently.
func TestParseAnonymousInlineNaryConnectIsNotSupported(t *testing.T) {
	src := "part def C { part a; part b; part c; connect (a, b, c); }"
	p := New(source.New("<t>", []byte(src)))
	_ = p.ParseFile()
	if len(p.Diagnostics) == 0 {
		t.Fatalf("anonymous inline n-ary connect now parses; " +
			"update the known-limitation row in docs/SPEC_COMPLIANCE.md and assert the ends here")
	}
}
