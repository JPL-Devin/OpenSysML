package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/resolve"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

func TestConstraintEvaluation_Assert(t *testing.T) {
	src := `
		package test {
			constraint PositiveValue {
				assert value > 0;
			}
		}
	`

	// Parse
	file := parser.New(source.New("test.sysml", []byte(src))).ParseFile()

	// Build symbol index
	idx := symbols.NewIndex()
	idx.AddDocument("test.sysml", file)

	// Create resolver and semantic model
	resolver := resolve.New(idx)
	model := semantics.NewModel(resolver)

	// Create runtime context
	ctx := NewContext(model, resolver, 10000)

	// Resolve constraint
	rootScope := idx.DocumentRoot("test.sysml")
	testPkg := rootScope.Children()[0]
	constraintSym, ok := testPkg.LookupLocal("PositiveValue")
	if !ok {
		t.Fatal("PositiveValue constraint not found")
	}

	// Note: This test will fail because 'value' is unbound
	// In real usage, constraints are evaluated with bindings
	_, err := ctx.EvaluateConstraint(constraintSym, testPkg)
	if err == nil {
		t.Fatal("Expected error for unbound 'value'")
	}

	if !strings.Contains(err.Error(), "unresolved feature") {
		t.Logf("✓ Got expected error: %v", err)
	}
}

func TestConstraintEvaluation_AssertWithLiteral(t *testing.T) {
	src := `
		package test {
			constraint AlwaysTrue {
				assert 5 > 3;
			}
			
			constraint AlwaysFalse {
				assert 2 > 10;
			}
		}
	`

	// Parse
	file := parser.New(source.New("test.sysml", []byte(src))).ParseFile()

	// Build symbol index
	idx := symbols.NewIndex()
	idx.AddDocument("test.sysml", file)

	// Create resolver and semantic model
	resolver := resolve.New(idx)
	model := semantics.NewModel(resolver)

	// Create runtime context
	ctx := NewContext(model, resolver, 10000)

	// Resolve constraints
	rootScope := idx.DocumentRoot("test.sysml")
	testPkg := rootScope.Children()[0]

	// Test AlwaysTrue
	alwaysTrue, ok := testPkg.LookupLocal("AlwaysTrue")
	if !ok {
		t.Fatal("AlwaysTrue not found")
	}

	satisfied, err := ctx.EvaluateConstraint(alwaysTrue, testPkg)
	if err != nil {
		t.Fatalf("AlwaysTrue evaluation failed: %v", err)
	}
	if !satisfied {
		t.Fatal("AlwaysTrue should be satisfied")
	}
	t.Logf("✓ AlwaysTrue: assertion passed")

	// Test AlwaysFalse
	alwaysFalse, ok := testPkg.LookupLocal("AlwaysFalse")
	if !ok {
		t.Fatal("AlwaysFalse not found")
	}

	_, err = ctx.EvaluateConstraint(alwaysFalse, testPkg)
	if err == nil {
		t.Fatal("AlwaysFalse should fail")
	}
	if !errors.Is(err, ErrViolated) {
		t.Fatalf("Expected a violation verdict, got: %v", err)
	}
	t.Logf("✓ AlwaysFalse: assertion failed (as expected)")
}

func TestConstraintEvaluation_Assume(t *testing.T) {
	src := `
		package test {
			constraint WithAssumption {
				assume 1 > 5;  // false assumption, but should pass
			}
		}
	`

	// Parse
	file := parser.New(source.New("test.sysml", []byte(src))).ParseFile()

	// Build symbol index
	idx := symbols.NewIndex()
	idx.AddDocument("test.sysml", file)

	// Create resolver and semantic model
	resolver := resolve.New(idx)
	model := semantics.NewModel(resolver)

	// Create runtime context
	ctx := NewContext(model, resolver, 10000)

	// Resolve constraint
	rootScope := idx.DocumentRoot("test.sysml")
	testPkg := rootScope.Children()[0]
	constraintSym, ok := testPkg.LookupLocal("WithAssumption")
	if !ok {
		t.Fatal("WithAssumption not found")
	}

	// Evaluate - should pass even though assumption is false
	satisfied, err := ctx.EvaluateConstraint(constraintSym, testPkg)
	if err != nil {
		t.Fatalf("Assumption should not fail: %v", err)
	}
	if !satisfied {
		t.Fatal("Constraint with assumption should be satisfied")
	}
	t.Logf("✓ Assumption passed (false assumptions are trusted)")
}

func TestConstraintEvaluation_Negation(t *testing.T) {
	src := `
		package test {
			constraint NotNegative {
				assert not (3 < 0);
			}
		}
	`

	// Parse
	file := parser.New(source.New("test.sysml", []byte(src))).ParseFile()

	// Build symbol index
	idx := symbols.NewIndex()
	idx.AddDocument("test.sysml", file)

	// Create resolver and semantic model
	resolver := resolve.New(idx)
	model := semantics.NewModel(resolver)

	// Create runtime context
	ctx := NewContext(model, resolver, 10000)

	// Resolve constraint
	rootScope := idx.DocumentRoot("test.sysml")
	testPkg := rootScope.Children()[0]
	constraintSym, ok := testPkg.LookupLocal("NotNegative")
	if !ok {
		t.Fatal("NotNegative not found")
	}

	// Evaluate - assert not (3 < 0) → assert not false → assert true → pass
	satisfied, err := ctx.EvaluateConstraint(constraintSym, testPkg)
	if err != nil {
		t.Fatalf("Negated assertion failed: %v", err)
	}
	if !satisfied {
		t.Fatal("Negated assertion should pass")
	}
	t.Logf("✓ Negated assertion passed")
}

// A constraint with nothing to check has no verdict: reporting one would claim
// a check that never ran.
func TestConstraintWithoutConditionsIsNotAVerdict(t *testing.T) {
	src := `
		package test {
			constraint def Empty { }
			part def Rig {
				constraint nothing : Empty;
			}
		}
	`
	file := parser.New(source.New("test.sysml", []byte(src))).ParseFile()
	idx := symbols.NewIndex()
	idx.AddDocument("test.sysml", file)
	resolver := resolve.New(idx)
	ctx := NewContext(semantics.NewModel(resolver), resolver, 10000)

	testPkg := idx.DocumentRoot("test.sysml").Children()[0]
	rig, ok := testPkg.LookupLocal("Rig")
	if !ok {
		t.Fatal("Rig not found")
	}
	feat := featureNamed(ctx, rig, "nothing")
	if feat == nil || feat.Symbol == nil {
		t.Fatal("constraint feature not found")
	}

	satisfied, err := ctx.EvaluateConstraintOn(feat.Symbol, feat.DeclScope(), nil)
	if !errors.Is(err, ErrNoConditions) {
		t.Fatalf("err = %v, want ErrNoConditions", err)
	}
	if satisfied {
		t.Error("an unevaluated constraint reported as satisfied")
	}
}
