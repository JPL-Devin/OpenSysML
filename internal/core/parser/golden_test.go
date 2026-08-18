package parser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// TestGolden verifies parser AST output matches golden snapshots (Phase 2, Task 2.1)
func TestGolden(t *testing.T) {
	fixtures := filepath.Join("testdata", "parse")
	entries, err := os.ReadDir(fixtures)
	if err != nil {
		t.Fatalf("Failed to read fixtures dir %s: %v", fixtures, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sysml") {
			continue
		}

		name := entry.Name()
		t.Run(name, func(t *testing.T) {
			sysmlPath := filepath.Join(fixtures, name)
			goldenPath := strings.TrimSuffix(sysmlPath, ".sysml") + ".golden"

			// Parse
			data, err := os.ReadFile(sysmlPath)
			if err != nil {
				t.Fatalf("Failed to read %s: %v", sysmlPath, err)
			}

			sf := source.New(name, data)
			p := New(sf)
			root := p.ParseFile()

			if len(p.Diagnostics) > 0 {
				t.Errorf("Parse errors in %s:", name)
				for _, d := range p.Diagnostics {
					t.Errorf("  %s", d.Message)
				}
			}

			// Generate AST dump
			actual := ast.Dump(root)

			if *update {
				// Update mode: write golden file
				if err := os.WriteFile(goldenPath, []byte(actual), 0644); err != nil {
					t.Fatalf("Failed to write golden %s: %v", goldenPath, err)
				}
				t.Logf("Updated golden: %s", goldenPath)
				return
			}

			// Verify mode: compare with golden
			expected, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("Failed to read golden %s (run with -update to create): %v", goldenPath, err)
			}

			if actual != string(expected) {
				t.Errorf("AST mismatch for %s\nRun 'go test -update' to regenerate golden", name)
				t.Logf("Expected:\n%s", string(expected))
				t.Logf("Actual:\n%s", actual)
			}
		})
	}
}
