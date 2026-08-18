package model

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
)

// exprTypeDiagnostics returns the expression type-checker findings for one
// document, ignoring the relationship-kind tier that shares the "type" source.
func exprTypeDiagnostics(ws *Workspace, name string) []string {
	var out []string
	for _, d := range ws.Diagnostics(name) {
		if d.Code == "type.expr" {
			out = append(out, name+": "+d.Message)
		}
	}
	return out
}

// TestExprTypeCheckNoStdlibFalsePositives guards the expression type checker
// against over-reporting: the shipped standard library is by definition
// well-typed, so any finding in it is a checker bug.
func TestExprTypeCheckNoStdlibFalsePositives(t *testing.T) {
	src := libs.DefaultSource()
	ws := NewWorkspace()
	var found []string
	for _, name := range src.List() {
		data, err := src.Read(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		ws.Open(name, data, 1)
		found = append(found, exprTypeDiagnostics(ws, name)...)
		ws.Close(name)
	}
	if len(found) != 0 {
		t.Fatalf("expression type checker reported %d finding(s) in the standard library:\n%s",
			len(found), strings.Join(found, "\n"))
	}
}

// TestExprTypeCheckNoExampleFalsePositives runs the same guard over the
// well-formed models the repository ships as examples and runtime fixtures.
func TestExprTypeCheckNoExampleFalsePositives(t *testing.T) {
	roots := []string{
		filepath.Join("..", "..", "..", "examples"),
		filepath.Join("..", "runtime", "testdata"),
	}
	ws := NewWorkspace()
	var found []string
	for _, root := range roots {
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return err
			}
			if !strings.HasSuffix(path, ".sysml") && !strings.HasSuffix(path, ".kerml") {
				return nil
			}
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			ws.Open(path, data, 1)
			found = append(found, exprTypeDiagnostics(ws, path)...)
			ws.Close(path)
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
	if len(found) != 0 {
		t.Fatalf("expression type checker reported %d finding(s) in example models:\n%s",
			len(found), strings.Join(found, "\n"))
	}
}
