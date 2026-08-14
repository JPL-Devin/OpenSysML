package runtime

import (
	"encoding/json"
	"flag"
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

var updateTraces = flag.Bool("update-traces", false, "Update golden trace files")

// TestExecutionTrace runs golden trace tests against conformance cases.
// Traces capture step-by-step execution (token movements for actions, state
// transitions for states, parameter binding and sub-expression order for calc
// and constraint evaluation).
// Use -update-traces flag to regenerate golden files after reviewing diffs.
func TestExecutionTrace(t *testing.T) {
	conformanceDir := filepath.Join("testdata", "conformance")

	entries, err := os.ReadDir(conformanceDir)
	if err != nil {
		t.Fatalf("read conformance dir: %v", err)
	}

	// Find all .sysml files
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sysml") {
			continue
		}

		testName := strings.TrimSuffix(entry.Name(), ".sysml")
		goldenPath := filepath.Join(conformanceDir, testName+".trace.golden")

		// Skip if no golden file exists and not updating
		if !*updateTraces {
			if _, err := os.Stat(goldenPath); os.IsNotExist(err) {
				continue // Golden doesn't exist yet, skip
			}
		}

		t.Run(testName, func(t *testing.T) {
			runTraceTest(t, conformanceDir, testName, goldenPath)
		})
	}
}

func runTraceTest(t *testing.T, conformanceDir, testName, goldenPath string) {
	sysmlPath := filepath.Join(conformanceDir, testName+".sysml")

	// Load file
	sysmlData, err := os.ReadFile(sysmlPath)
	if err != nil {
		t.Fatalf("load source: %v", err)
	}

	expected := loadExpectedOutcome(t, conformanceDir, testName)

	// Parse and build model
	p := parser.New(source.New(sysmlPath, sysmlData))
	file := p.ParseFile()
	checkDiagnostics(t, p.Diagnostics, expected.Diagnostics)

	idx := symbols.NewIndex()
	// A case whose model names library elements — the measurement unit of a
	// quantity is one — resolves them only with the standard library indexed,
	// exactly as the conformance harness loads it.
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

	// Find behavioral symbol and execute with trace
	trace := NewTraceRecorder()
	var traceOutput string

	// -update-traces runs every conformance case, including known failures and
	// cases that produce no trace at all (requirements), so there is nothing to
	// record for those rather than anything to report.
	unrecordable := t.Fatalf
	if *updateTraces {
		unrecordable = t.Skipf
	}

	rootScope := idx.DocumentRoot(sysmlPath)

	// Evaluation-based cases (calc, constraint) trace through the context rather
	// than an executor, and the expected outcome supplies the calc arguments.
	switch expected.Type {
	case "calc":
		ctx.SetTrace(trace)
		calcSym := namedOrFoundSymbol(t, idx, expected.Evaluate, rootScope, ast.DefCalc, ast.UsageCalc)
		args := make([]Value, len(expected.Inputs))
		for i, input := range expected.Inputs {
			args[i] = expectedToRuntimeValue(t, input)
		}
		if _, err := ctx.InvokeCalc(calcSym, args, rootScope); err != nil {
			unrecordable("invoke calc: %v", err)
		}
		traceOutput = trace.String()
	case "calcUsage":
		// Reading every output of the usage traces one evaluation of its body,
		// which is what makes the evaluate-once guarantee visible in the golden.
		ctx.SetTrace(trace)
		usageSym := namedOrFoundSymbol(t, idx, expected.Evaluate, rootScope, ast.DefCalc, ast.UsageCalc)
		if _, err := ctx.CalcUsageOutputs(usageSym, usageSym.OwnerScope, nil); err != nil {
			unrecordable("evaluate calc usage: %v", err)
		}
		traceOutput = trace.String()
	case "constraint":
		ctx.SetTrace(trace)
		constraintSym := findBehavioralSymbol(t, rootScope, ast.DefConstraint, ast.UsageConstraint)
		if _, err := ctx.EvaluateConstraint(constraintSym, rootScope); err != nil {
			unrecordable("evaluate constraint: %v", err)
		}
		traceOutput = trace.String()
	}

	// Try action execution
	if actionSym := lookupBehavioralSymbol(rootScope, ast.DefAction, ast.UsageAction); actionSym != nil {
		exec, err := ctx.CreateActionExecutor(actionSym)
		if err != nil {
			t.Fatalf("create action executor: %v", err)
		}
		exec.SetTrace(trace)

		// Drive the traced executor itself: ctx.ExecuteAction would build a
		// second, untraced one and leave the recorder empty.
		if err := exec.RunToCompletion(); err != nil {
			unrecordable("action execution: %v", err)
		}
		traceOutput = trace.String()
	}

	// Try state execution
	if stateSym := lookupBehavioralSymbol(rootScope, ast.DefState, ast.UsageState); stateSym != nil {
		exec, err := ctx.CreateStateExecutor(stateSym)
		if err != nil {
			t.Fatalf("create state executor: %v", err)
		}
		exec.SetTrace(trace)

		if err := exec.RunToCompletion(); err != nil {
			unrecordable("state execution: %v", err)
		}
		traceOutput = trace.String()
	}
	if traceOutput == "" {
		// Cases with no traced behavior at all (requirements) produce nothing to
		// compare.
		if *updateTraces {
			t.Skipf("%s produces no trace", testName)
		}
		t.Fatalf("%s has a golden trace but produced none", testName)
	}

	// Update or compare golden
	if *updateTraces {
		if err := os.WriteFile(goldenPath, []byte(traceOutput+"\n"), 0644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("updated golden trace: %s", goldenPath)
	} else {
		golden, err := os.ReadFile(goldenPath)
		if err != nil {
			t.Fatalf("read golden: %v", err)
		}

		want := strings.TrimSpace(string(golden))
		got := strings.TrimSpace(traceOutput)

		if got != want {
			t.Errorf("trace mismatch for %s\n=== WANT ===\n%s\n=== GOT ===\n%s\n", testName, want, got)
		}
	}
}

// loadExpectedOutcome reads a case's conformance expectation, which tells the
// trace harness how the case is driven. A case without one is untyped and only
// the executor paths apply.
func loadExpectedOutcome(t *testing.T, conformanceDir, testName string) ExpectedOutcome {
	data, err := os.ReadFile(filepath.Join(conformanceDir, testName+".expected.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return ExpectedOutcome{}
		}
		t.Fatalf("read expected outcome: %v", err)
	}

	var expected ExpectedOutcome
	if err := json.Unmarshal(data, &expected); err != nil {
		t.Fatalf("parse expected outcome: %v", err)
	}
	return expected
}

// A trace names control nodes by what they do, so an unnamed fork or final
// node does not surface a Go type name to whoever reads the trace.
func TestNodeIdentifierNamesControlNodes(t *testing.T) {
	cases := []struct {
		node ast.Node
		want string
	}{
		{&ast.InitialNode{}, "initial"},
		{&ast.FinalNode{}, "final"},
		{&ast.ForkNode{}, "fork"},
		{&ast.JoinNode{}, "join"},
		{&ast.MergeNode{}, "merge"},
		{&ast.DecisionNode{}, "decision"},
		{&ast.ForkNode{Name: "split"}, "split"},
	}

	for _, tc := range cases {
		if got := nodeIdentifier(tc.node); got != tc.want {
			t.Errorf("nodeIdentifier(%T) = %q, want %q", tc.node, got, tc.want)
		}
	}
}
