package model

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindDoneStartErrors(t *testing.T) {
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
		t.Fatal(err)
	}
	
	t.Logf("Checking %d files", len(files))
	
	doneErrors := []string{}
	startErrors := []string{}
	
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		
		relPath, _ := filepath.Rel(trainingDir, file)
		ws.Open(relPath, data, 1)
		diags := ws.Diagnostics(relPath)
		
		for _, d := range diags {
			if d.Message == "unresolved reference: done" {
				doneErrors = append(doneErrors, relPath)
				t.Logf("DONE error in: %s", relPath)
			}
			if d.Message == "unresolved reference: start" {
				startErrors = append(startErrors, relPath)
				t.Logf("START error in: %s", relPath)
			}
		}
	}
	
	t.Logf("\nFiles with 'done' errors: %d", len(doneErrors))
	for _, f := range doneErrors {
		t.Logf("  %s", f)
	}
	
	t.Logf("\nFiles with 'start' errors: %d", len(startErrors))
	for _, f := range startErrors {
		t.Logf("  %s", f)
	}
	
	if len(doneErrors) > 0 || len(startErrors) > 0 {
		// Known OMG training bugs (documented in docs/TRAINING_EXAMPLES.md)
		knownBugs := map[string]bool{
			"27. Occurrences/Time Slice and Snapshot Example.sysml": true,
			"28. Individuals/Individuals and Time Slices.sysml":      true,
		}
		
		for _, f := range append(doneErrors, startErrors...) {
			if !knownBugs[f] {
				t.Errorf("Unexpected error in: %s", f)
			}
		}
		
		// Expect exactly 2 files with these errors (both have done + start)
		if len(doneErrors) != 2 || len(startErrors) != 2 {
			t.Errorf("Expected 2 'done' and 2 'start' errors (known OMG bugs), got %d and %d", len(doneErrors), len(startErrors))
		}
	}
}
