package parser

import (
	"sync"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

func TestParseFileReturnsDiagnosticsAndWarnings(t *testing.T) {
	root, diagnostics, warnings := ParseFile(source.New("test.sysml", []byte(
		"package P { action flow { assign x := ; } }",
	)))
	if root == nil {
		t.Fatal("ParseFile returned nil root")
	}
	if len(diagnostics) == 0 {
		t.Fatal("ParseFile returned no syntax diagnostics")
	}
	if len(warnings) == 0 {
		t.Fatal("ParseFile returned no warnings")
	}
}

func TestParseFileIsSafeForConcurrentCalls(t *testing.T) {
	t.Parallel()
	const workers = 8
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			root, diagnostics, warnings := ParseFile(source.New("test.sysml", []byte(
				"package P { part def A { attribute x; } }",
			)))
			if root == nil || len(diagnostics) != 0 || len(warnings) != 0 {
				t.Errorf("concurrent parse returned root=%v diagnostics=%v warnings=%v",
					root != nil, diagnostics, warnings)
			}
		}()
	}
	wg.Wait()
}
