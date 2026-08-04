package runtime

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/resolve"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// ExpectedValue represents a typed value in expected.json
type ExpectedValue struct {
	Type  string      `json:"type"`
	Value interface{} `json:"value"`
}

// ExpectedOutcome represents expected execution result
type ExpectedOutcome struct {
	Type string `json:"type"` // "action", "state", "calc", "constraint", "requirement"

	// Action fields
	Outputs    map[string]ExpectedValue `json:"outputs,omitempty"`
	TokenCount *int                     `json:"tokenCount,omitempty"`

	// State fields
	FinalState  string   `json:"finalState,omitempty"`
	StateVisits []string `json:"stateVisits,omitempty"`

	// Calc fields
	Inputs []ExpectedValue `json:"inputs,omitempty"`
	Result *ExpectedValue  `json:"result,omitempty"`

	// Constraint/Requirement fields
	Bindings  map[string]ExpectedValue `json:"bindings,omitempty"`
	Satisfied *bool                    `json:"satisfied,omitempty"`
}

// TestExecutionConformance runs all behavioral execution conformance tests.
// Each case consists of <name>.sysml and <name>.expected.json.
// Cases listed in known_failures.txt are skipped with SKIP log.
func TestExecutionConformance(t *testing.T) {
	conformanceDir := filepath.Join("testdata", "conformance")

	// Load known failures
	knownFailures := loadKnownFailures(t, conformanceDir)

	// Walk conformance directory for .expected.json files
	entries, err := os.ReadDir(conformanceDir)
	if err != nil {
		if os.IsNotExist(err) {
			t.Skip("conformance directory does not exist yet")
		}
		t.Fatalf("failed to read conformance directory: %v", err)
	}

	testCount := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".expected.json") {
			continue
		}

		caseName := strings.TrimSuffix(entry.Name(), ".expected.json")

		// Check if known failure
		if knownFailures[caseName] {
			t.Logf("SKIP %s (known failure)", caseName)
			continue
		}

		testCount++
		t.Run(caseName, func(t *testing.T) {
			runConformanceCase(t, conformanceDir, caseName)
		})
	}

	if testCount == 0 {
		t.Skip("no conformance cases found")
	}
}

// loadKnownFailures reads known_failures.txt and returns set of case names to skip
func loadKnownFailures(t *testing.T, conformanceDir string) map[string]bool {
	knownPath := filepath.Join(conformanceDir, "known_failures.txt")
	data, err := os.ReadFile(knownPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // no known failures file
		}
		t.Logf("warning: failed to read known_failures.txt: %v", err)
		return nil
	}

	failures := make(map[string]bool)
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		failures[line] = true
	}
	return failures
}

// runConformanceCase executes a single conformance test case
func runConformanceCase(t *testing.T, conformanceDir, caseName string) {
	// Load .sysml file
	sysmlPath := filepath.Join(conformanceDir, caseName+".sysml")
	sysmlData, err := os.ReadFile(sysmlPath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", sysmlPath, err)
	}

	// Load .expected.json
	expectedPath := filepath.Join(conformanceDir, caseName+".expected.json")
	expectedData, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", expectedPath, err)
	}

	var expected ExpectedOutcome
	if err := json.Unmarshal(expectedData, &expected); err != nil {
		t.Fatalf("failed to parse expected.json: %v", err)
	}

	// Parse and build model
	file := parser.New(source.New(sysmlPath, sysmlData)).ParseFile()
	// Note: Parser diagnostics not directly accessible from file
	// Syntax errors will manifest as nil symbols or malformed AST

	idx := symbols.NewIndex()
	idx.AddDocument(sysmlPath, file)
	resolver := resolve.New(idx)
	model := semantics.NewModel(resolver)
	ctx := NewContext(model, resolver, 10000)

	// Dispatch based on type
	switch expected.Type {
	case "action":
		runActionConformance(t, ctx, idx, sysmlPath, expected)
	case "state":
		runStateConformance(t, ctx, idx, sysmlPath, expected)
	case "calc":
		runCalcConformance(t, ctx, idx, sysmlPath, expected)
	case "constraint":
		runConstraintConformance(t, ctx, idx, sysmlPath, expected)
	case "requirement":
		runRequirementConformance(t, ctx, idx, sysmlPath, expected)
	default:
		t.Fatalf("unknown test type: %s", expected.Type)
	}
}

// runActionConformance executes action and validates outputs
func runActionConformance(t *testing.T, ctx *Context, idx *symbols.Index, path string, expected ExpectedOutcome) {
	// Find action definition/usage in root scope
	rootScope := idx.DocumentRoot(path)
	actionSym := findBehavioralSymbol(t, rootScope, ast.DefAction, ast.UsageAction)

	// Execute action
	outputs, err := ctx.ExecuteAction(actionSym)
	if err != nil {
		t.Fatalf("ExecuteAction failed: %v", err)
	}

	// Validate outputs
	if expected.Outputs != nil {
		for name, expectedVal := range expected.Outputs {
			actual, ok := outputs[name]
			if !ok {
				t.Errorf("missing output %q", name)
				continue
			}
			validateValue(t, name, expectedVal, actual)
		}
	}

	// Optional: validate token count
	if expected.TokenCount != nil {
		// Token count validation requires instrumentation in executor
		// For now, skip - can add later if needed
		t.Logf("token count validation not yet implemented (expected %d)", *expected.TokenCount)
	}
}

// runStateConformance executes state machine and validates final state
func runStateConformance(t *testing.T, ctx *Context, idx *symbols.Index, path string, expected ExpectedOutcome) {
	// Find state definition/usage
	rootScope := idx.DocumentRoot(path)
	stateSym := findBehavioralSymbol(t, rootScope, ast.DefState, ast.UsageState)

	// Execute state machine
	outputs, err := ctx.ExecuteState(stateSym)
	if err != nil {
		t.Fatalf("ExecuteState failed: %v", err)
	}

	// Validate final state
	if expected.FinalState != "" {
		// Extract final state from executor (requires instrumentation)
		// For now, log and skip
		t.Logf("finalState validation not yet implemented (expected %s)", expected.FinalState)
	}

	// Validate state visits
	if len(expected.StateVisits) > 0 {
		t.Logf("stateVisits validation not yet implemented")
	}

	// Validate outputs
	if expected.Outputs != nil {
		for name, expectedVal := range expected.Outputs {
			actual, ok := outputs[name]
			if !ok {
				t.Errorf("missing output %q", name)
				continue
			}
			validateValue(t, name, expectedVal, actual)
		}
	}
}

// runCalcConformance invokes calc and validates result
func runCalcConformance(t *testing.T, ctx *Context, idx *symbols.Index, path string, expected ExpectedOutcome) {
	// Find calc definition/usage
	rootScope := idx.DocumentRoot(path)
	calcSym := findBehavioralSymbol(t, rootScope, ast.DefCalc, ast.UsageCalc)

	// Convert expected inputs to runtime Values
	args := make([]Value, len(expected.Inputs))
	for i, input := range expected.Inputs {
		args[i] = expectedToRuntimeValue(t, input)
	}

	// Invoke calc
	result, err := ctx.InvokeCalc(calcSym, args, rootScope)
	if err != nil {
		t.Fatalf("InvokeCalc failed: %v", err)
	}

	// Validate result
	if expected.Result != nil {
		validateValue(t, "result", *expected.Result, result)
	}
}

// runConstraintConformance evaluates constraint and validates satisfaction
func runConstraintConformance(t *testing.T, ctx *Context, idx *symbols.Index, path string, expected ExpectedOutcome) {
	// Find constraint definition/usage
	rootScope := idx.DocumentRoot(path)
	constraintSym := findBehavioralSymbol(t, rootScope, ast.DefConstraint, ast.UsageConstraint)

	// Apply bindings to context (if any)
	if expected.Bindings != nil {
		// Bindings need to be added to scope or instance
		// For now, assume bindings are already in model
		t.Logf("constraint bindings application not yet implemented")
	}

	// Evaluate constraint
	satisfied, err := ctx.EvaluateConstraint(constraintSym, rootScope)
	if err != nil {
		t.Fatalf("EvaluateConstraint failed: %v", err)
	}

	// Validate satisfaction
	if expected.Satisfied != nil {
		if satisfied != *expected.Satisfied {
			t.Errorf("constraint satisfied = %v, want %v", satisfied, *expected.Satisfied)
		}
	}
}

// runRequirementConformance evaluates requirement and validates satisfaction
func runRequirementConformance(t *testing.T, ctx *Context, idx *symbols.Index, path string, expected ExpectedOutcome) {
	// Find requirement definition/usage
	rootScope := idx.DocumentRoot(path)
	reqSym := findBehavioralSymbol(t, rootScope, ast.DefRequirement, ast.UsageRequirement)

	// Apply bindings (if any)
	if expected.Bindings != nil {
		t.Logf("requirement bindings application not yet implemented")
	}

	// Evaluate requirement
	satisfied, err := ctx.EvaluateRequirement(reqSym, rootScope)
	if err != nil {
		t.Fatalf("EvaluateRequirement failed: %v", err)
	}

	// Validate satisfaction
	if expected.Satisfied != nil {
		if satisfied != *expected.Satisfied {
			t.Errorf("requirement satisfied = %v, want %v", satisfied, *expected.Satisfied)
		}
	}
}

// findBehavioralSymbol searches for first symbol matching defKind or usageKind
func findBehavioralSymbol(t *testing.T, scope *symbols.Scope, defKind ast.DefinitionKind, usageKind ast.UsageKind) *symbols.Symbol {
	// Check all child scopes (packages/namespaces)
	for _, child := range scope.Children() {
		// Look for named symbols
		for _, name := range child.MemberNames() {
			sym, ok := child.LookupLocal(name)
			if !ok {
				continue
			}
			if def, ok := sym.Decl.(*ast.Definition); ok && def.Kind == defKind {
				return sym
			}
			if usage, ok := sym.Decl.(*ast.Usage); ok && usage.Kind == usageKind {
				return sym
			}
		}
	}

	// Also check root scope directly
	for _, name := range scope.MemberNames() {
		sym, ok := scope.LookupLocal(name)
		if !ok {
			continue
		}
		if def, ok := sym.Decl.(*ast.Definition); ok && def.Kind == defKind {
			return sym
		}
		if usage, ok := sym.Decl.(*ast.Usage); ok && usage.Kind == usageKind {
			return sym
		}
	}

	t.Fatalf("no behavioral symbol found (defKind=%v, usageKind=%v)", defKind, usageKind)
	return nil
}

// expectedToRuntimeValue converts ExpectedValue to runtime Value
func expectedToRuntimeValue(t *testing.T, ev ExpectedValue) Value {
	switch ev.Type {
	case "Integer":
		var intVal int64
		switch v := ev.Value.(type) {
		case float64:
			intVal = int64(v)
		case int64:
			intVal = v
		default:
			t.Fatalf("invalid Integer value type: %T", ev.Value)
		}
		return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: intVal}}
	case "Real":
		if v, ok := ev.Value.(float64); ok {
			return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: v}}
		}
		t.Fatalf("invalid Real value type: %T", ev.Value)
	case "Boolean":
		if v, ok := ev.Value.(bool); ok {
			return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValBool, Bool: v}}
		}
		t.Fatalf("invalid Boolean value type: %T", ev.Value)
	case "String":
		if v, ok := ev.Value.(string); ok {
			return Value{Kind: ValString, Str: v}
		}
		t.Fatalf("invalid String value type: %T", ev.Value)
	case "Null":
		return Value{Kind: ValNull}
	default:
		t.Fatalf("unknown type: %s", ev.Type)
	}
	return Value{}
}

// validateValue checks if runtime Value matches ExpectedValue
func validateValue(t *testing.T, name string, expected ExpectedValue, actual Value) {
	switch expected.Type {
	case "Integer":
		if actual.Kind != ValConst || actual.Const.Kind != semantics.ValInt {
			t.Errorf("%s: type = %v (Const.Kind=%v), want Integer", name, actual.Kind, actual.Const.Kind)
			return
		}
		want := int64(expected.Value.(float64))
		if actual.Const.Int != want {
			t.Errorf("%s: value = %d, want %d", name, actual.Const.Int, want)
		}
	case "Real":
		if actual.Kind != ValConst || actual.Const.Kind != semantics.ValReal {
			t.Errorf("%s: type = %v (Const.Kind=%v), want Real", name, actual.Kind, actual.Const.Kind)
			return
		}
		want := expected.Value.(float64)
		if actual.Const.Real != want {
			t.Errorf("%s: value = %f, want %f", name, actual.Const.Real, want)
		}
	case "Boolean":
		if actual.Kind != ValConst || actual.Const.Kind != semantics.ValBool {
			t.Errorf("%s: type = %v (Const.Kind=%v), want Boolean", name, actual.Kind, actual.Const.Kind)
			return
		}
		want := expected.Value.(bool)
		if actual.Const.Bool != want {
			t.Errorf("%s: value = %v, want %v", name, actual.Const.Bool, want)
		}
	case "String":
		if actual.Kind != ValString {
			t.Errorf("%s: type = %v, want String", name, actual.Kind)
			return
		}
		want := expected.Value.(string)
		if actual.Str != want {
			t.Errorf("%s: value = %q, want %q", name, actual.Str, want)
		}
	default:
		t.Errorf("%s: unknown expected type %s", name, expected.Type)
	}
}

