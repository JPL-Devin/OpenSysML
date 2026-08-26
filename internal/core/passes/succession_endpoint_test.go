package passes

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
)

// endpointDiags runs the name-resolution tier over src, the tier an endpoint
// name is resolved at.
func endpointDiags(t *testing.T, src string) []Diagnostic {
	t.Helper()
	ctx, root := nameresCtx(t, "a.sysml", src)
	return NameResolutionPass{}.Run(ctx, "a.sysml", root)
}

// An endpoint naming a member the body does not declare is reported where the
// name is written, for every spelling the notation admits, rather than only when
// the model is executed.
func TestEndpointNamingNoMemberIsReported(t *testing.T) {
	cases := map[string]string{
		"succession": `package P { action def A {
			first start;
			action a;
			done;
			succession first start then zzz;
		} }`,
		"guarded succession": `package P { action def A {
			first start;
			action a;
			done;
			succession first start if true then zzz;
		} }`,
		"implicit source": `package P { action def A {
			first start;
			action a;
			then zzz;
		} }`,
		"decision branch": `package P { action def A {
			first start;
			action pick;
			if true then zzz;
		} }`,
		"default decision branch": `package P { action def A {
			first start;
			action pick;
			if true then pick;
			else zzz;
		} }`,
		"transition": `package P { state def M {
			entry; then idle;
			state idle;
			transition first idle then zzz;
		} }`,
		"transition source": `package P { state def M {
			entry; then idle;
			state idle;
			transition first zzz then idle;
		} }`,
	}

	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			got := endpointDiags(t, src)
			if len(got) != 1 {
				t.Fatalf("expected one diagnostic for the endpoint, got %+v", got)
			}
			d := got[0]
			if d.Severity != SeverityError {
				t.Errorf("expected an error, got %v", d.Severity)
			}
			if d.Code != "unresolved" || d.Source != "name-resolution" {
				t.Errorf("got code %q source %q, want unresolved at the name-resolution tier", d.Code, d.Source)
			}
			if !strings.Contains(d.Message, "zzz") {
				t.Errorf("expected the message to name zzz, got %q", d.Message)
			}
			if got := strings.TrimSpace(src[d.Span.Offset : d.Span.Offset+d.Span.Len]); got != "zzz" {
				t.Errorf("expected the span to cover the endpoint name, it covers %q", got)
			}
		})
	}
}

// `then final;` is a reference to a member named final now that the keyword is
// gone, so a body which declares no such member is reported rather than parsed
// as a final node and accepted.
func TestEndpointNamingRemovedFinalKeywordIsReported(t *testing.T) {
	src := `package P { action def A {
	first start;
	action a;
	then final;
} }`
	got := endpointDiags(t, src)
	if len(got) != 1 || !strings.Contains(got[0].Message, "final") {
		t.Fatalf("expected one diagnostic naming final, got %+v", got)
	}
	if covered := src[got[0].Span.Offset : got[0].Span.Offset+got[0].Span.Len]; covered != "final" {
		t.Errorf("expected the span to cover the endpoint name, it covers %q", covered)
	}
}

// Every endpoint the notation admits and the lowerer resolves stays silent: a
// declared member, the implicit `start` and `done` the standard library declares
// on Action, and the ends a member-attached `then` binds by position.
func TestEndpointsThatResolveStaySilent(t *testing.T) {
	cases := map[string]string{
		"declared members": `package P { action def A {
			action a;
			action b;
			succession first a then b;
		} }`,
		"implicit start": `package P { action def A {
			action a;
			succession first start then a;
		} }`,
		"initial node supplies start": `package P { action def A {
			first start;
			action a;
			done;
			succession first start then a;
		} }`,
		"implicit done": `package P { action def A {
			first start;
			action a;
			then done;
		} }`,
		"declared final node": `package P { action def A {
			first start;
			action a;
			done;
			succession first a then done;
		} }`,
		"guarded branches": `package P { action def A {
			first start;
			action a;
			action b;
			succession first a if true then b;
		} }`,
		"positional send end": `package P { action def A {
			attribute x;
			first start;
			action a;
			then send x() to self;
			then done;
		} }`,
		"positional loop end": `package P { action def A {
			first start;
			then loop action { action b; } until true;
			then done;
		} }`,
		"nested vertex of a machine": `package P { state def M {
			entry; then alpha;
			state alpha { state work; }
			state beta;
			then work;
		} }`,
		"machine end naming no vertex": `package P { state def M {
			attribute t = 0;
			entry; then s1;
			state s1;
			then t;
		} }`,
		"end outside a machine or an action body": `package P { part def D {
			action def A { action x; }
			perform action p;
			then p;
		} }`,
		"transition ends": `package P { state def M {
			entry; then idle;
			state idle;
			state busy;
			attribute ready;
			transition first idle if ready then busy;
		} }`,
		"transition with a trigger and an effect": `package P {
			attribute def Ping;
			state def M {
				entry; then idle;
				state idle;
				state busy;
				action log;
				transition first idle accept Ping do action log then busy;
			}
		}`,
	}

	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			if got := endpointDiags(t, src); len(got) != 0 {
				t.Fatalf("expected no diagnostics, got %+v", got)
			}
		})
	}
}

// The tier agrees with the lowerer about which endpoint names are implicit: a
// name lowering resolves is silent here, and a name lowering fails on is
// reported here, so the diagnostic arrives at validation time instead.
func TestEndpointDiagnosticAgreesWithLowering(t *testing.T) {
	cases := map[string]struct {
		src      string
		resolves bool
	}{
		"start": {`package P { action def A { action a; succession first start then a; } }`, true},
		"done":  {`package P { action def A { first start; action a; then done; } }`, true},
		"declared final": {`package P { action def A {
			first start;
			action a;
			done;
			succession first a then done;
		} }`, true},
		"undeclared": {`package P { action def A {
			first start;
			action a;
			done;
			succession first start then zzz;
		} }`, false},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := lower.ToActionGraph(actionDefIn(t, parseDoc(t, "a.sysml", c.src)), nil)
			diags := endpointDiags(t, c.src)
			if c.resolves {
				if err != nil {
					t.Fatalf("expected lowering to resolve the endpoint, got %v", err)
				}
				if len(diags) != 0 {
					t.Fatalf("lowering resolves the endpoint but the tier reports %+v", diags)
				}
				return
			}
			if err == nil {
				t.Fatal("expected lowering to fail on the endpoint")
			}
			if len(diags) != 1 {
				t.Fatalf("lowering fails on the endpoint but the tier reports %+v", diags)
			}
		})
	}
}

// In a machine an end names a vertex the way a transition's does, so a nested or
// region-local one lowering reaches must stay silent here too.
func TestStateMachineEndpointAgreesWithLowering(t *testing.T) {
	const src = `package P { state def M {
		entry; then alpha;
		state alpha { state work; }
		state beta;
		then work;
	} }`

	ctx, root := nameresCtx(t, "a.sysml", src)
	if got := (NameResolutionPass{}).Run(ctx, "a.sysml", root); len(got) != 0 {
		t.Fatalf("expected no diagnostics for a nested vertex, got %+v", got)
	}

	machine := ctx.Index.LookupQualified("P::M")
	if len(machine) != 1 {
		t.Fatalf("looked up %d symbols for P::M, want the machine", len(machine))
	}
	sym := machine[0]
	scope := sym.Scope
	if scope == nil {
		scope = sym.OwnerScope
	}
	graph, err := lower.ToStateGraphWithEndpoints(sym.Decl, scope, ctx.Resolver())
	if err != nil {
		t.Fatalf("lowering the machine: %v", err)
	}
	var found bool
	for _, transitions := range graph.Transitions {
		for _, trans := range transitions {
			if state, ok := trans.Target.(*ast.StateNode); ok && state.Name == "work" {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("lowering resolved no transition into the nested work, so the case does not test agreement")
	}
}

// actionDefIn returns the first action definition declared in root, the
// declaration lowering builds a graph from.
func actionDefIn(t *testing.T, root *ast.RootNamespace) ast.Node {
	t.Helper()
	var walk func(members []ast.Node) ast.Node
	walk = func(members []ast.Node) ast.Node {
		for _, member := range members {
			if ms, ok := member.(*ast.Membership); ok {
				member = ms.Member
			}
			switch n := member.(type) {
			case *ast.Definition:
				if n.Kind == ast.DefAction {
					return n
				}
				if found := walk(n.Members); found != nil {
					return found
				}
			case *ast.Package:
				if found := walk(n.Members); found != nil {
					return found
				}
			case *ast.Namespace:
				if found := walk(n.Members); found != nil {
					return found
				}
			}
		}
		return nil
	}
	def := walk(root.Members)
	if def == nil {
		t.Fatal("no action definition found")
	}
	return def
}
