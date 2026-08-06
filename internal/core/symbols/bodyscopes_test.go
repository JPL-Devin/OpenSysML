package symbols

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
)

// findBodyScopes returns every scope in the tree owned by a body expression.
func findBodyScopes(scope *Scope) []*Scope {
	var out []*Scope
	if _, ok := scope.Node().(*ast.BodyExpr); ok {
		out = append(out, scope)
	}
	for _, ch := range scope.Children() {
		out = append(out, findBodyScopes(ch)...)
	}
	return out
}

// Build links a body expression's parameters into the document scope tree, so
// every consumer — resolution, the type checker and the editor — sees the same
// scope rather than one built on the side.
func TestBuildLinksBodyExpressionScopes(t *testing.T) {
	src := `package P {
	import ScalarValues::*;
	import ControlFunctions::*;
	action def Sample {
		in attribute samples : Real[*];
		assert constraint { samples->forAll { in s : Real; s > 0 } }
	}
}`
	root := build(t, src)

	bodies := findBodyScopes(root)
	if len(bodies) != 1 {
		t.Fatalf("body-expression scopes = %d, want 1", len(bodies))
	}
	sym, ok := bodies[0].LookupLocal("s")
	if !ok {
		t.Fatalf("parameter s not defined in the body scope")
	}
	// The parameter's own identifier, not the body's braces: the editor renames
	// through NameSpan and jumps to DeclSpan.
	if want := strings.Index(src, "s : Real; s > 0"); sym.NameSpan.Offset != want || sym.DeclSpan.Offset != want {
		t.Errorf("spans at %d/%d, want the parameter name at %d", sym.NameSpan.Offset, sym.DeclSpan.Offset, want)
	}
	if BodyExprScope(bodies[0].Parent(), bodies[0].Node().(*ast.BodyExpr)) != bodies[0] {
		t.Error("BodyExprScope did not return the linked scope")
	}
}

// Nested bodies nest their scopes, so an inner parameter shadows an outer one.
func TestBuildNestsBodyExpressionScopes(t *testing.T) {
	root := build(t, `package P {
	import ScalarValues::*;
	import ControlFunctions::*;
	action def Sample {
		in attribute samples : Real[*];
		assert constraint { samples->forAll { in s : Real; samples->exists { in s : Real; s > 0 } } }
	}
}`)

	bodies := findBodyScopes(root)
	if len(bodies) != 2 {
		t.Fatalf("body-expression scopes = %d, want 2", len(bodies))
	}
	if bodies[1].Parent() != bodies[0] {
		t.Fatalf("inner body scope parent = %v, want the outer body scope", bodies[1].Parent().Node())
	}
	outer, _ := bodies[0].LookupLocal("s")
	inner, _ := bodies[1].LookupLocal("s")
	if outer == nil || inner == nil || outer == inner {
		t.Errorf("nested parameters share a symbol: outer=%v inner=%v", outer, inner)
	}
}

// A body expression without parameters declares nothing, so it owns no scope
// and its result resolves in the enclosing scope.
func TestBodyExprScopeWithoutParameters(t *testing.T) {
	root := build(t, `package P {
	import ScalarValues::*;
	import ControlFunctions::*;
	action def Sample {
		in attribute samples : Real[*];
		assert constraint { samples->forAll { samples > 0 } }
	}
}`)

	if bodies := findBodyScopes(root); len(bodies) != 0 {
		t.Fatalf("body-expression scopes = %d, want 0", len(bodies))
	}
}
