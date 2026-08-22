package passes

import "github.com/Open-MBEE/OpenSysML/internal/core/ast"

// SyntaxPass surfaces the parser-produced diagnostics carried on the Context.
// The passes package does not import parser; the caller adapts parser
// diagnostics into passes.Diagnostic and stores them on Context.ParseDiagnostics.
type SyntaxPass struct{}

// Level reports that syntax is the lowest dependency level.
func (SyntaxPass) Level() PassLevel { return LevelSyntax }

// Run returns the parse diagnostics carried on the context.
func (SyntaxPass) Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic {
	if ctx == nil || len(ctx.ParseDiagnostics) == 0 {
		return nil
	}
	out := make([]Diagnostic, len(ctx.ParseDiagnostics))
	copy(out, ctx.ParseDiagnostics)
	return out
}
