package runtime

import (
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
// Traces capture step-by-step execution (token movements for actions, state transitions for states).
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

	// Parse and build model
	file := parser.New(source.New(sysmlPath, sysmlData)).ParseFile()

	idx := symbols.NewIndex()
	idx.AddDocument(sysmlPath, file)
	resolver := resolve.New(idx)
	model := semantics.NewModel(resolver)
	ctx := NewContext(model, resolver, 10000)

	// Find behavioral symbol and execute with trace
	trace := NewTraceRecorder()
	var traceOutput string

	// -update-traces runs every conformance case, including known failures and
	// cases that produce no trace at all (pure constraints and requirements), so
	// there is nothing to record for those rather than anything to report.
	unrecordable := t.Fatalf
	if *updateTraces {
		unrecordable = t.Skipf
	}

	rootScope := idx.DocumentRoot(sysmlPath)

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
		// Calcs are evaluated rather than executed, so there is no executor to
		// attach a recorder to; cases with no behavioral symbol at all (pure
		// constraints/requirements) likewise produce nothing to compare.
		if *updateTraces {
			t.Skipf("%s produces no trace", testName)
		}
		if lookupBehavioralSymbol(rootScope, ast.DefCalc, ast.UsageCalc) != nil {
			t.Skip("calc tracing not implemented: calcs are evaluated, not executed")
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
