package semantics

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
)

// TestImplicitBaseNeedsTheLibrary covers a model resolved without the standard
// library: no base is in the index, so an untyped usage stays untyped rather
// than reporting a phantom supertype.
func TestImplicitBaseWithoutLibrary(t *testing.T) {
	m, root := buildModel(t, "part p;")
	if supers := m.DirectSupertypes(sym(t, root, "p")); len(supers) != 0 {
		t.Fatalf("DirectSupertypes(p) = %v, want none without the library", supers)
	}
}

// TestImplicitBaseResolvesThroughIndex covers the lookup itself against a
// stand-in library declared in the same document.
func TestImplicitBaseResolvesThroughIndex(t *testing.T) {
	m, root := buildModel(t, "package Parts { part def Part; } part p; part q : Parts::Part;")
	parts := sym(t, root, "Parts")
	part, _ := parts.Scope.LookupLocal("Part")

	supers := m.DirectSupertypes(sym(t, root, "p"))
	if len(supers) != 1 || supers[0] != part {
		t.Fatalf("DirectSupertypes(p) = %v, want [Parts::Part]", supers)
	}
	if supers := m.DirectSupertypes(sym(t, root, "q")); len(supers) != 1 || supers[0] != part {
		t.Fatalf("DirectSupertypes(q) = %v, want the declared [Parts::Part] only", supers)
	}
}

// TestUsageKindsWithoutImplicitBase pins the kinds deliberately left out of the
// mapping: connector, succession, flow, binding, satisfy, subject and objective
// take their type from the element they relate to, and the KerML structural
// kinds are definitions in usage position.
func TestUsageKindsWithoutImplicitBase(t *testing.T) {
	for _, k := range []ast.UsageKind{
		ast.UsageConnector, ast.UsageSuccession, ast.UsageFlow, ast.UsageBinding,
		ast.UsageSatisfy, ast.UsageSubject, ast.UsageObjective, ast.UsageInteraction,
		ast.UsageBehavior, ast.UsageAssoc, ast.UsageStruct, ast.UsageClass,
		ast.UsagePredicate, ast.UsageBool,
	} {
		if fqn, ok := implicitUsageBases[k]; ok {
			t.Errorf("usage kind %v unexpectedly has implicit base %q", k, fqn)
		}
	}
}
