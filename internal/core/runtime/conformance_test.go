package runtime

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/libs"
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

// ExpectedEvent represents an event to inject during state machine execution:
// either a signal (`signal`) or an operation invocation (`call`).
type ExpectedEvent struct {
	Signal string                   `json:"signal,omitempty"` // Signal type name
	Call   string                   `json:"call,omitempty"`   // Invoked operation name
	Args   map[string]ExpectedValue `json:"args,omitempty"`   // Signal payload or call arguments
}

// ExpectedOutcome represents expected execution result
type ExpectedOutcome struct {
	Type string `json:"type"` // "action", "state", "calc", "constraint", "requirement", "instance"
	// Libraries loads the standard library into the case's index, for a case
	// whose model names library elements the runtime resolves — the measurement
	// unit of a quantity expression is one.
	Libraries bool `json:"libraries,omitempty"`

	// Action fields
	Outputs    map[string]ExpectedValue `json:"outputs,omitempty"`
	TokenCount *int                     `json:"tokenCount,omitempty"`
	// Error is the text the execution is expected to fail with, for a case whose
	// contract is a diagnostic rather than a result (a loop that never
	// terminates). Empty means the execution must succeed.
	Error string `json:"error,omitempty"`

	// State fields
	Events      []ExpectedEvent `json:"events,omitempty"` // Events to inject
	FinalState  string          `json:"finalState,omitempty"`
	StateVisits []string        `json:"stateVisits,omitempty"`

	// Calc fields
	Inputs []ExpectedValue `json:"inputs,omitempty"`
	Result *ExpectedValue  `json:"result,omitempty"`

	// Constraint/Requirement fields
	Bindings  map[string]ExpectedValue `json:"bindings,omitempty"`
	Satisfied *bool                    `json:"satisfied,omitempty"`
	// Evaluate names the element to evaluate, for a case that declares more than
	// one — a usage and the definition it is typed by. Empty searches the model.
	Evaluate string `json:"evaluate,omitempty"`

	// Satisfy fields: the verdict expected of each satisfaction assertion the
	// case states, keyed by the assertion as written ("satisfy r by p"), since
	// such an assertion is anonymous.
	Assertions map[string]bool `json:"assertions,omitempty"`

	// Instance fields
	Instantiate string                   `json:"instantiate,omitempty"` // qualified name of the type to instantiate
	Slots       map[string]ExpectedValue `json:"slots,omitempty"`       // expected slot values, derived ones included
	Constraints map[string]bool          `json:"constraints,omitempty"` // constraint feature name -> satisfied on this instance
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
		t.Fatalf("no runnable conformance cases in %s", conformanceDir)
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
	if expected.Libraries {
		loadLibraries(t, idx)
	}
	idx.AddDocument(sysmlPath, file)
	if expected.Libraries {
		idx.ExpandWildcardImports()
	}
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
	case "satisfy":
		runSatisfyConformance(t, ctx, idx, sysmlPath, expected)
	case "instance":
		runInstanceConformance(t, ctx, idx, expected)
	default:
		t.Fatalf("unknown test type: %s", expected.Type)
	}
}

// loadLibraries loads the standard library into idx, for a case that names its
// elements.
func loadLibraries(t *testing.T, idx *symbols.Index) {
	t.Helper()
	src := libs.DefaultSource()
	cache, err := libs.NewCache()
	if err != nil {
		cache = nil
	}
	loader := libs.NewLoader(src, cache)
	for _, name := range src.List() {
		if err := loader.Load(name, idx); err != nil {
			t.Fatalf("load library %s: %v", name, err)
		}
	}
}

// runActionConformance executes action and validates outputs
func runActionConformance(t *testing.T, ctx *Context, idx *symbols.Index, path string, expected ExpectedOutcome) {
	// Find action definition/usage in root scope
	rootScope := idx.DocumentRoot(path)
	actionSym := findBehavioralSymbol(t, rootScope, ast.DefAction, ast.UsageAction)

	// Execute action
	outputs, err := ctx.ExecuteAction(actionSym)
	if expected.Error != "" {
		if err == nil {
			t.Fatalf("expected execution to fail with %q, it completed with outputs %v", expected.Error, outputs)
		}
		if !strings.Contains(err.Error(), expected.Error) {
			t.Fatalf("execution failed with %q, want an error containing %q", err, expected.Error)
		}
		return
	}
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

	// Create executor manually to inject events
	exec, err := newStateExecutor(ctx, stateSym)
	if err != nil {
		t.Fatalf("create state executor: %v", err)
	}

	// Initialize (enters initial state)
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize state machine: %v", err)
	}

	// Inject events from schema
	for _, event := range expected.Events {
		args := make(map[string]Value, len(event.Args))
		for name, val := range event.Args {
			args[name] = expectedToRuntimeValue(t, val)
		}
		switch {
		case event.Call != "" && event.Signal != "":
			t.Fatalf("event declares both signal %q and call %q", event.Signal, event.Call)
		case event.Call != "":
			exec.InvokeOperation(event.Call, args)
		case event.Signal != "":
			exec.SendSignal(event.Signal, args)
		default:
			t.Fatalf("event declares neither a signal nor a call")
		}
	}

	// Process events until completion or suspension, through the executor's own
	// loop: a harness-local copy drifts from the semantics under test.
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run state machine: %v", err)
	}

	// Validate final state
	if expected.FinalState != "" {
		// Get final state from executor
		// For orthogonal regions, build "State1+State2+..." string (sorted by region name)
		var finalState string
		if len(exec.activeConfig.regionStates) > 0 {
			// Multi-region: collect all region states, sort by region name
			type regionStatePair struct {
				regionName string
				stateName  string
			}
			var pairs []regionStatePair
			for region, regionState := range exec.activeConfig.regionStates {
				pairs = append(pairs, regionStatePair{region.Name, regionState.Name})
			}
			sort.Slice(pairs, func(i, j int) bool {
				return pairs[i].regionName < pairs[j].regionName
			})
			var stateNames []string
			for _, pair := range pairs {
				stateNames = append(stateNames, pair.stateName)
			}
			finalState = strings.Join(stateNames, "+")
		} else {
			// Simple state
			finalStateNode := exec.CurrentState()
			if finalStateNode == nil {
				finalState = ""
			} else if stateNode, ok := finalStateNode.(*ast.StateNode); ok {
				finalState = stateNode.Name
			} else {
				t.Errorf("expected StateNode, got %T", finalStateNode)
			}
		}

		if finalState == "" {
			t.Errorf("expected finalState %q, got empty", expected.FinalState)
		} else if finalState != expected.FinalState {
			t.Errorf("finalState mismatch: expected %q, got %q", expected.FinalState, finalState)
		}
	}

	// Validate state visits
	if len(expected.StateVisits) > 0 {
		actual := exec.GetStateVisits()
		if len(actual) != len(expected.StateVisits) {
			t.Errorf("stateVisits length mismatch: expected %d, got %d", len(expected.StateVisits), len(actual))
			t.Logf("  expected: %v", expected.StateVisits)
			t.Logf("  actual:   %v", actual)
		} else {
			for i, expectedName := range expected.StateVisits {
				if actual[i] != expectedName {
					t.Errorf("stateVisits[%d] mismatch: expected %q, got %q", i, expectedName, actual[i])
				}
			}
		}
	}

	// Validate outputs
	if expected.Outputs != nil {
		for name, expectedVal := range expected.Outputs {
			actual, ok := exec.stateData[name]
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
	constraintSym := namedOrFoundSymbol(t, idx, expected.Evaluate, rootScope, ast.DefConstraint, ast.UsageConstraint)

	// Apply bindings to context (if any)
	if expected.Bindings != nil {
		// Bindings need to be added to scope or instance
		// For now, assume bindings are already in model
		t.Logf("constraint bindings application not yet implemented")
	}

	// Evaluate constraint. A violated assertion is a verdict, not a failure.
	satisfied, err := ctx.EvaluateConstraint(constraintSym, constraintSym.OwnerScope)
	if err != nil && !errors.Is(err, ErrViolated) {
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
	reqSym := namedOrFoundSymbol(t, idx, expected.Evaluate, rootScope, ast.DefRequirement, ast.UsageRequirement)

	// Evaluate requirement using symbol's defining scope (where sibling features
	// visible). A violated condition is a verdict, not a failure.
	satisfied, err := ctx.EvaluateRequirement(reqSym, reqSym.OwnerScope)
	if err != nil && !errors.Is(err, ErrViolated) {
		t.Fatalf("EvaluateRequirement failed: %v", err)
	}

	// Validate satisfaction
	if expected.Satisfied != nil {
		if satisfied != *expected.Satisfied {
			t.Errorf("requirement satisfied = %v, want %v", satisfied, *expected.Satisfied)
		}
	}
}

// runSatisfyConformance evaluates the satisfaction assertions the case states
// and validates the verdict of each: the assertion binds the requirement's
// subject to the object its `by` operand names, so the verdict is about that
// object's values.
func runSatisfyConformance(t *testing.T, ctx *Context, idx *symbols.Index, path string, expected ExpectedOutcome) {
	scope := idx.DocumentRoot(path)
	if expected.Evaluate != "" {
		matches := idx.LookupQualified(expected.Evaluate)
		if len(matches) != 1 {
			t.Fatalf("evaluate %q: %d matching symbols, want 1", expected.Evaluate, len(matches))
		}
		if matches[0].Scope == nil {
			t.Fatalf("evaluate %q: the element owns no scope, so it states no assertion", expected.Evaluate)
		}
		scope = matches[0].Scope
	}

	assertions := ctx.SatisfyAssertionsIn(scope)
	if len(assertions) == 0 {
		t.Fatalf("no satisfaction assertion found")
	}
	if expected.Error != "" || expected.Satisfied != nil {
		if len(assertions) != 1 {
			t.Fatalf("%d satisfaction assertions found, want 1: name each verdict under \"assertions\"", len(assertions))
		}
	}

	verdicts := make(map[string]bool, len(assertions))
	for _, a := range assertions {
		satisfied, err := ctx.EvaluateSatisfaction(a)
		if expected.Error != "" {
			if err == nil {
				t.Fatalf("expected evaluation to fail with %q, it reported satisfied = %v", expected.Error, satisfied)
			}
			if !strings.Contains(err.Error(), expected.Error) {
				t.Fatalf("evaluation failed with %q, want an error containing %q", err, expected.Error)
			}
			return
		}
		if err != nil && !errors.Is(err, ErrViolated) {
			t.Fatalf("EvaluateSatisfaction(%s) failed: %v", a.Text(), err)
		}
		verdicts[a.Text()] = satisfied
	}

	if expected.Satisfied != nil {
		for text, satisfied := range verdicts {
			if satisfied != *expected.Satisfied {
				t.Errorf("%s: satisfied = %v, want %v", text, satisfied, *expected.Satisfied)
			}
		}
	}
	for text, want := range expected.Assertions {
		satisfied, ok := verdicts[text]
		if !ok {
			t.Errorf("no assertion %q among %v", text, sortedKeys(verdicts))
			continue
		}
		if satisfied != want {
			t.Errorf("%s: satisfied = %v, want %v", text, satisfied, want)
		}
	}
}

// sortedKeys returns the keys of m in order, for a deterministic message.
func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// runInstanceConformance instantiates a type and validates the values its slots
// hold, including derived defaults, plus the verdict of each constraint the
// instance carries.
func runInstanceConformance(t *testing.T, ctx *Context, idx *symbols.Index, expected ExpectedOutcome) {
	if expected.Instantiate == "" {
		t.Fatalf("instance case declares no \"instantiate\" type")
	}
	matches := idx.LookupQualified(expected.Instantiate)
	if len(matches) != 1 {
		t.Fatalf("instantiate %q: %d matching symbols, want 1", expected.Instantiate, len(matches))
	}
	typeSym := matches[0]

	inst, err := ctx.Instantiate(typeSym)
	if err != nil {
		t.Fatalf("Instantiate(%s) failed: %v", expected.Instantiate, err)
	}

	for name, expectedVal := range expected.Slots {
		slot, err := inst.GetSlot(ctx, name)
		if err != nil {
			t.Errorf("slot %q: %v", name, err)
			continue
		}
		validateValue(t, name, expectedVal, slot.Value)
	}

	for name, wantSatisfied := range expected.Constraints {
		feat := featureNamed(ctx, typeSym, name)
		if feat == nil || feat.Symbol == nil {
			t.Errorf("constraint %q: no such feature on %s", name, expected.Instantiate)
			continue
		}
		satisfied, err := ctx.EvaluateConstraintOn(feat.Symbol, feat.DeclScope(), inst)
		if err != nil && !errors.Is(err, ErrViolated) {
			t.Errorf("constraint %q: %v", name, err)
			continue
		}
		if satisfied != wantSatisfied {
			t.Errorf("constraint %q: satisfied = %v, want %v", name, satisfied, wantSatisfied)
		}
	}
}

// featureNamed returns the effective feature of typeSym called name, or nil.
func featureNamed(ctx *Context, typeSym *symbols.Symbol, name string) *EffectiveFeature {
	features := ctx.FeaturesOf(typeSym)
	for i := range features {
		if features[i].Name == name {
			return &features[i]
		}
	}
	return nil
}

// findBehavioralSymbol searches for first symbol matching defKind or usageKind,
// failing the test when there is none.
func findBehavioralSymbol(t *testing.T, scope *symbols.Scope, defKind ast.DefinitionKind, usageKind ast.UsageKind) *symbols.Symbol {
	sym := lookupBehavioralSymbol(scope, defKind, usageKind)
	if sym == nil {
		t.Fatalf("no behavioral symbol found (defKind=%v, usageKind=%v)", defKind, usageKind)
	}
	return sym
}

// namedOrFoundSymbol returns the symbol the case names, or searches the model
// when it names none.
func namedOrFoundSymbol(t *testing.T, idx *symbols.Index, fqn string, scope *symbols.Scope, defKind ast.DefinitionKind, usageKind ast.UsageKind) *symbols.Symbol {
	if fqn == "" {
		return findBehavioralSymbol(t, scope, defKind, usageKind)
	}
	matches := idx.LookupQualified(fqn)
	if len(matches) != 1 {
		t.Fatalf("evaluate %q: %d matching symbols, want 1", fqn, len(matches))
	}
	return matches[0]
}

// lookupBehavioralSymbol is findBehavioralSymbol for callers that probe several
// kinds and treat absence as "not this kind of model".
func lookupBehavioralSymbol(scope *symbols.Scope, defKind ast.DefinitionKind, usageKind ast.UsageKind) *symbols.Symbol {
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
