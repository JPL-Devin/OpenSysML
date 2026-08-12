package parser

import (
	"regexp"
	"strings"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

var dumpedSuccession = regexp.MustCompile(`\(SuccessionEdge source="([^"]*)" target="([^"]*)"\)`)

// parseSuccessions returns the succession edges src parses to, as
// "source->target" pairs in tree order, with the parser that read it.
func parseSuccessions(t *testing.T, src string) ([]string, *Parser) {
	t.Helper()
	p := New(source.New("succession.sysml", []byte(src)))
	dump := ast.Dump(p.ParseFile())

	var edges []string
	for _, m := range dumpedSuccession.FindAllStringSubmatch(dump, -1) {
		edges = append(edges, m[1]+"->"+m[2])
	}
	return edges, p
}

// A member-attached `then` carries the same succession as the edge notation, so
// it is desugared into one: the member before it is the source and the member
// after it the target, whatever the members are and however they are laid out.
func TestMemberAttachedThenDesugars(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []string
	}{
		{
			"structural body",
			"part def P { part a; then part b; }",
			[]string{"a->b"},
		},
		{
			"newline before the member does not reverse the pair",
			"part def P { part a;\n\tthen\n\tpart b;\n}",
			[]string{"a->b"},
		},
		{
			"chained thens sequence each pair in declaration order",
			"action def A { action a; then action b; then action c; }",
			[]string{"a->b", "b->c"},
		},
		{
			"the edge notation is unchanged",
			"action def A { action a; action b; then a b; }",
			[]string{"a->b"},
		},
		{
			"an edge is not the source of the next succession",
			"action def A { action a; action b; then a b; then action c; }",
			[]string{"a->b", "b->c"},
		},
		{
			"a one-name edge takes the member before it as its source",
			"action def A { action a; action b; then a; }",
			[]string{"b->a"},
		},
		{
			"the short state form is named, so it is sequenced",
			"state def S { state a; then state b; }",
			[]string{"a->b"},
		},
		{
			"a state body's order statement is not the source of the next succession",
			"state def S { state a; state b; a then b; then state c; }",
			[]string{"b->c"},
		},
		{
			"a one-name edge whose target is a keyword the body declares",
			"action def A { done end; action a; then end; }",
			[]string{"a->end"},
		},
		{
			"a two-name edge whose source is a keyword the body declares",
			"action def A { action end; action b; then end b; }",
			[]string{"end->b"},
		},
		{
			"a member named after the feature it references is a succession end",
			"action def A { perform a; then action b; }",
			[]string{"a->b"},
		},
		{
			"a performed step after a named member is sequenced, not chained",
			"action A { perform v; then perform t; then perform s; }",
			[]string{"v->t", "t->s"},
		},
		{
			"a performed step after a statement stays a statement of the block",
			"action A { while i <= 2 { assign v := 1; then perform body; } }",
			nil,
		},
		{
			"a calculation body reads the edge form it is written back as",
			"calc def C { part a; part b; then a b; }",
			[]string{"a->b"},
		},
		{
			"a requirement body reads the edge form it is written back as",
			"requirement def R { part a; part b; then a b; }",
			[]string{"a->b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			edges, p := parseSuccessions(t, tt.src)
			if len(p.Diagnostics) != 0 {
				t.Fatalf("unexpected diagnostics: %v", p.Diagnostics)
			}
			if strings.Join(edges, " ") != strings.Join(tt.want, " ") {
				t.Errorf("succession edges %v, want %v", edges, tt.want)
			}
		})
	}
}

// A region carries the members of a state body, so a `then` attached to one of
// its states is the same succession it would be one level up.
func TestMemberAttachedThenInRegionDesugars(t *testing.T) {
	p := New(source.New("region.sysml", []byte("state def S { region R { state a; then state b; } }")))
	file := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %v", p.Diagnostics)
	}

	var edges []*ast.SuccessionEdge
	for _, member := range file.Members {
		m, ok := member.(*ast.Membership)
		if !ok {
			continue
		}
		def, ok := m.Member.(*ast.Definition)
		if !ok {
			continue
		}
		for _, stateMember := range def.Members {
			region, ok := stateMember.(*ast.StateRegion)
			if !ok {
				continue
			}
			for _, regionMember := range region.States {
				if edge, ok := regionMember.(*ast.SuccessionEdge); ok {
					edges = append(edges, edge)
				}
			}
		}
	}
	if len(edges) != 1 {
		t.Fatalf("succession edges in the region: %d, want 1", len(edges))
	}
	if got := qnText(edges[0].Source) + "->" + qnText(edges[0].Target); got != "a->b" {
		t.Errorf("succession %s, want a->b", got)
	}
}

// qnText spells a qualified name the way a succession end reads.
func qnText(qn *ast.QualifiedName) string {
	if qn == nil {
		return ""
	}
	var parts []string
	for _, part := range qn.Parts {
		parts = append(parts, part.Text)
	}
	return strings.Join(parts, "::")
}

// A succession edge names its ends, so a `then` beside a member with no name
// declares an order this representation cannot carry. The notation is legal, so
// it is a warning naming the end at fault rather than a syntax error.
func TestSuccessionUnnamedEndWarns(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{"unnamed source", "action def A { action; then action b; }", "sequences from a member with no name"},
		{"unnamed source after a named member", "action def A { action a; action; then action b; }", "sequences from a member with no name"},
		{"unnamed target", "action def A { action b; then send msg to port; }", "sequences to a member with no name"},
		{"anonymous member after the keyword", "action def A { action b; then action { } }", "sequences to a member with no name"},
		{"anonymous typed member after the keyword", "part def P { part a; then part : T; }", "sequences to a member with no name"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			edges, p := parseSuccessions(t, tt.src)
			if len(p.Diagnostics) != 0 {
				t.Fatalf("unexpected syntax errors: %v", p.Diagnostics)
			}
			if len(edges) != 0 {
				t.Errorf("synthesised %v for a succession with an unnamed end", edges)
			}
			var found bool
			for _, w := range p.Warnings {
				if strings.Contains(w.Message, tt.want) {
					found = true
					if w.Code != codeUnnamedSuccessionEnd {
						t.Errorf("warning code %q, want %q", w.Code, codeUnnamedSuccessionEnd)
					}
				}
			}
			if !found {
				t.Errorf("warnings %v, want one containing %q", p.Warnings, tt.want)
			}
		})
	}
}
