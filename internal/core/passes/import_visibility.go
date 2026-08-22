package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// importKeyword is the keyword the diagnostic spans.
const importKeyword = "import"

// ImportVisibilityPass reports an `import` written without the visibility
// indicator ImportPrefix makes mandatory (KerML.xtext:169-172, SysML.xtext:241-244).
// The reference rejects the bare form, so it is an error in every mode (D2,
// docs/project/wave10-decisions.md); the form still parses, so only severity moves.
type ImportVisibilityPass struct{}

func (ImportVisibilityPass) Level() PassLevel { return LevelSyntax }

func (ImportVisibilityPass) Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic {
	if root == nil {
		return nil
	}
	return checkImportVisibility(root.Members)
}

// checkImportVisibility reports every bare import in the member subtree.
func checkImportVisibility(members []ast.Node) []Diagnostic {
	var diags []Diagnostic
	for _, m := range members {
		switch n := unwrapType(m).(type) {
		case *ast.Import:
			if d, ok := importVisibilityDiagnostic(n); ok {
				diags = append(diags, d)
			}
			diags = append(diags, checkImportVisibility(n.Body)...)
		case *ast.Package:
			diags = append(diags, checkImportVisibility(n.Members)...)
		case *ast.Namespace:
			diags = append(diags, checkImportVisibility(n.Members)...)
		case *ast.Definition:
			diags = append(diags, checkImportVisibility(n.Members)...)
		case *ast.Usage:
			diags = append(diags, checkImportVisibility(n.Members)...)
		}
	}
	return diags
}

// importVisibilityDiagnostic reports imp when it carries no visibility
// indicator. An `expose` is exempt: the pilot grammar gives it an implicit
// protected visibility (ExposeVisibilityKind, SysML.xtext:2366-2372).
func importVisibilityDiagnostic(imp *ast.Import) (Diagnostic, bool) {
	if imp.IsExpose || imp.Visibility != ast.VisibilityDefault {
		return Diagnostic{}, false
	}
	return Diagnostic{
		Severity: SeverityError,
		Span:     importKeywordSpan(imp),
		Message:  "import without a visibility indicator: SysML v2 requires public, private or protected before 'import'",
		Code:     "import-visibility",
		Source:   "syntax",
	}, true
}

// importKeywordSpan spans the `import` keyword, which with no visibility prefix
// opens the node.
func importKeywordSpan(imp *ast.Import) source.Span {
	sp := imp.Span()
	if sp.Len > len(importKeyword) {
		sp.Len = len(importKeyword)
	}
	return sp
}
