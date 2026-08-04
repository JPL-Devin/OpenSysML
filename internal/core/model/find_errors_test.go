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
		t.Errorf("Found %d 'done' errors and %d 'start' errors", len(doneErrors), len(startErrors))
	}
}
