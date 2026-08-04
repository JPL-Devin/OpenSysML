package model

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/passes"
)

func TestTrainingExamplesSemanticErrors(t *testing.T) {
	ws := NewWorkspace()

	trainingDir := filepath.Join("..", "..", "..", "examples", "sysml-v2-training")
	var files []string

	err := filepath.Walk(trainingDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".sysml") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to scan training dir: %v", err)
	}

	t.Logf("Found %d training .sysml files", len(files))

	// Load all files
	errorsByFile := make(map[string][]string)
	var crashFile string
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("CRASH in file %s: %v", crashFile, r)
		}
	}()
	for _, path := range files {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Logf("SKIP %s: %v", path, err)
			continue
		}

		relPath, _ := filepath.Rel(trainingDir, path)
		crashFile = relPath // Track current file for crash debugging
		ws.Open(relPath, content, 1)

		diags := ws.Diagnostics(relPath)
		if len(diags) > 0 {
			var errs []string
			for _, d := range diags {
				if d.Severity == passes.SeverityError {
					errs = append(errs, d.Message)
				}
			}
			if len(errs) > 0 {
				errorsByFile[relPath] = errs
			}
		}
	}

	if crashFile != "" {
		t.Logf("Last processed file before any issues: %s", crashFile)
	}

	// Report summary
	if len(errorsByFile) == 0 {
		t.Logf("✓ All %d files loaded without semantic errors", len(files))
		return
	}

	t.Logf("Files with semantic errors: %d/%d", len(errorsByFile), len(files))
	
	// Count error types
	errorTypes := make(map[string]int)
	for _, errs := range errorsByFile {
		for _, msg := range errs {
			errorTypes[msg]++
		}
	}

	t.Log("\nError types by frequency:")
	for msg, count := range errorTypes {
		t.Logf("  %3d × %s", count, msg)
	}

	t.Log("\nFirst 10 files with errors:")
	count := 0
	for file, errs := range errorsByFile {
		if count >= 10 {
			break
		}
		t.Logf("\n%s:", file)
		for _, err := range errs {
			t.Logf("  - %s", err)
		}
		count++
	}
}
