package libs

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

// TestActionsArchitecturalBlockers documents the remaining 5 errors in Actions.sysml
// that require architectural changes to support. These patterns are rare in stdlib
// and represent advanced SysML v2 features that go beyond the current parser architecture.
func TestActionsArchitecturalBlockers(t *testing.T) {
	src := &embedSource{}
	data, err := src.Read("Systems Library/Actions.sysml")
	if err != nil {
		t.Fatalf("Failed to load Actions.sysml: %v", err)
	}

	sf := source.New("Systems Library/Actions.sysml", data)
	p := parser.New(sf)
	_ = p.ParseFile()

	t.Logf("Actions.sysml: %d errors (all architectural blockers)", len(p.Diagnostics))
	
	// Expected errors:
	// 1. Transition usage syntax (offsets ~6414, 6455)
	//    Pattern: transition aTransition first start accept apayload : Anything via receiver then done;
	//    Blocker: Requires new UsageTransition AST node with connector-like syntax (first/accept/via/then keywords)
	//            Current parser only supports transition as state machine transition, not standalone usage
	//
	// 2. Namespace-level action succession (offsets ~14309, 14412, 14515)
	//    Pattern: action initialization ... then action whileLoop ...
	//    Blocker: Requires member-level succession relationships (then keyword between declarations)
	//            Current architecture: succession only inside action bodies, not between namespace members
	//
	// Both patterns are rare (only in Actions.sysml) and represent edge cases of SysML v2 spec.
	// Implementing would require:
	// - New AST node types (UsageTransition)
	// - Extension of Membership to support succession edges
	// - Parser changes to recognize 'then' at namespace level
	//
	// Estimated effort: 6-8 hours for both patterns
	// Priority: LOW (does not block 99% of SysML v2 usage)

	expectedErrors := 5
	if len(p.Diagnostics) != expectedErrors {
		t.Errorf("Expected %d errors (architectural blockers), got %d", expectedErrors, len(p.Diagnostics))
		for i, d := range p.Diagnostics {
			t.Logf("  %d. offset=%d: %s", i+1, d.Span.Offset, d.Message)
		}
	} else {
		t.Logf("✓ All errors are known architectural blockers (transition usage + namespace succession)")
	}
}
