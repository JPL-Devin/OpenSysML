package parser

import (
	"regexp"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

var dumpedSuccession = regexp.MustCompile(`(?s)\(SuccessionEdge source="([^"]*)" target="([^"]*)"\)|\(Usage kind="succession".*?\(ConnectorEnd target="([^"]*)".*?\(ConnectorEnd target="([^"]*)"`)

// parseSuccessions returns the succession edges src parses to, as
// "source->target" pairs in tree order, with the parser that read it.
func parseSuccessions(t *testing.T, src string) ([]string, *Parser) {
	t.Helper()
	p := New(source.New("succession.sysml", []byte(src)))
	dump := ast.Dump(p.ParseFile())

	var edges []string
	for _, m := range dumpedSuccession.FindAllStringSubmatch(dump, -1) {
		if m[1] != "" {
			edges = append(edges, m[1]+"->"+m[2])
		} else {
			edges = append(edges, m[3]+"->"+m[4])
		}
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
			"action def A { action a; action b; succession first a then b; }",
			[]string{"a->b"},
		},
		{
			"an edge is not the source of the next succession",
			"action def A { action a; action b; succession first a then b; then action c; }",
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
			"a one-name edge whose target is a keyword the body declares",
			"action def A { action a; then done; }",
			[]string{"a->@done"},
		},
		{
			"a two-name edge whose source is a keyword the body declares",
			"action def A { action end; action b; succession first end then b; }",
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
			"calc def C { part a; part b; succession first a then b; }",
			[]string{"a->b"},
		},
		{
			"a requirement body reads the edge form it is written back as",
			"requirement def R { part a; part b; succession first a then b; }",
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

// A nested state body carries the members of a state body, so a `then` attached
// to one of its states is the same succession it would be one level up.
func TestMemberAttachedThenInNestedStateDesugars(t *testing.T) {
	p := New(source.New("nested.sysml", []byte("state def S { state R { state a; then state b; } }")))
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
			nested, ok := unwrapStateUsage(stateMember)
			if !ok {
				continue
			}
			for _, nestedMember := range nested.Members {
				if edge, ok := nestedMember.(*ast.SuccessionEdge); ok {
					edges = append(edges, edge)
				}
			}
		}
	}
	if len(edges) != 1 {
		t.Fatalf("succession edges in the nested state: %d, want 1", len(edges))
	}
	if got := qnText(edges[0].Source) + "->" + qnText(edges[0].Target); got != "a->b" {
		t.Errorf("succession %s, want a->b", got)
	}
}

// unwrapStateUsage returns the state usage a body member declares.
func unwrapStateUsage(member ast.Node) (*ast.Usage, bool) {
	if m, ok := member.(*ast.Membership); ok {
		member = m.Member
	}
	usage, ok := member.(*ast.Usage)
	if !ok || usage.Kind != ast.UsageState {
		return nil, false
	}
	return usage, true
}

// A one-name succession takes the member before it as its source whether or not
// it carries a guard, so the two spellings reach lowering alike.
func TestOneNameGuardedEdgeTakesTheMemberBefore(t *testing.T) {
	p := New(source.New("guard.sysml", []byte("action def A { action a; action b; then a if x; }")))
	file := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %v", p.Diagnostics)
	}

	var edge *ast.ControlFlowEdge
	for _, member := range file.Members {
		m, ok := member.(*ast.Membership)
		if !ok {
			continue
		}
		def, ok := m.Member.(*ast.Definition)
		if !ok {
			continue
		}
		for _, defMember := range def.Members {
			if e, ok := defMember.(*ast.ControlFlowEdge); ok {
				edge = e
			}
		}
	}
	if edge == nil {
		t.Fatal("a guarded succession should be a control flow edge")
	}
	if got := qnText(edge.Source) + "->" + qnText(edge.Target); got != "b->a" {
		t.Errorf("guarded succession %s, want b->a", got)
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

// A member beside a `then` need not declare a name: the notation binds such an
// end by position (SysML.xtext EmptySuccessionMember), which the edge carries as
// the member itself. The dump reads a positional end as `@<kind>`.
func TestSuccessionBindsUnnamedEndsByPosition(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []string
	}{
		{"unnamed source", "action def A { action; then action b; }", []string{"@action->b"}},
		{"unnamed source after a named member", "action def A { action a; action; then action b; }", []string{"@action->b"}},
		{"a send is a node of the flow", "action def A { action b; then send msg to port; }", []string{"b->@send"}},
		{"anonymous member after the keyword", "action def A { action b; then action { } }", []string{"b->@action"}},
		{"anonymous typed member after the keyword", "part def P { part a; then part : T; }", []string{"a->@part"}},
		{"a loop node reached by a succession", "action def A { action b; then loop action { } until x; }", []string{"b->@loop"}},
		{"the final node a `then done` reaches", "action def A { action b; then done; }", []string{"b->@done"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			edges, p := parseSuccessions(t, tt.src)
			if len(p.Diagnostics) != 0 {
				t.Fatalf("unexpected syntax errors: %v", p.Diagnostics)
			}
			if len(p.Warnings) != 0 {
				t.Errorf("warnings %v for a succession the notation binds by position", p.Warnings)
			}
			if strings.Join(edges, " ") != strings.Join(tt.want, " ") {
				t.Errorf("succession edges %v, want %v", edges, tt.want)
			}
		})
	}
}

// The member a positional end binds to is that member itself, not another of the
// same kind: a consumer resolves the end by identity, so the edge has to point at
// the node the author wrote it beside.
func TestPositionalSuccessionEndIsTheMemberItself(t *testing.T) {
	p := New(source.New("positional.sysml", []byte(
		"action def A { action a; then send first() to p; then send second() to p; }")))
	file := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %v", p.Diagnostics)
	}

	var sends []ast.Node
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
		for _, defMember := range def.Members {
			switch n := defMember.(type) {
			case *ast.SendStatement:
				sends = append(sends, n)
			case *ast.SuccessionEdge:
				edges = append(edges, n)
			}
		}
	}
	if len(sends) != 2 || len(edges) != 2 {
		t.Fatalf("parsed %d sends and %d successions, want 2 and 2", len(sends), len(edges))
	}
	if edges[0].TargetMember != sends[0] {
		t.Errorf("the first succession targets %p, want the first send %p", edges[0].TargetMember, sends[0])
	}
	if edges[1].SourceMember != sends[0] || edges[1].TargetMember != sends[1] {
		t.Errorf("the second succession runs %p->%p, want %p->%p",
			edges[1].SourceMember, edges[1].TargetMember, sends[0], sends[1])
	}
}
