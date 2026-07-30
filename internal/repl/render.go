package repl

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/passes"
)

// Result is the outcome of one Submit: the top-level members parsed from the
// accumulated buffer (for the success summary), the names this submission
// declared, and any analysis diagnostics over the whole <repl> document.
type Result struct {
	Members     []ast.Node          // top-level members of the <repl> AST (Task 5 renders these)
	Declared    []string            // names introduced by THIS submission
	Diagnostics []passes.Diagnostic // eager analysis over the whole buffer
	Source      string              // the full joined <repl> content (Task 6 caret rendering)
}
