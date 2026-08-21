package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

// KerMLValidator's validateImportTopLevelVisibility message.
const msgTopLevelImportPrivate = "Top level import must be private"

// TopLevelImportPass checks that an import owned by the root namespace is
// private (KerML 8.2.3.4.2, validateImportTopLevelVisibility): a root namespace
// has no owner, so a non-private import there exports into nothing. It runs
// above LevelSyntax so a recovered parse is not reported on twice.
type TopLevelImportPass struct{}

func (TopLevelImportPass) Level() PassLevel { return LevelNameResolution }

func (TopLevelImportPass) Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic {
	if root == nil {
		return nil
	}
	var diags []Diagnostic
	for _, m := range root.Members {
		imp, ok := unwrapType(m).(*ast.Import)
		if !ok || imp.IsExpose {
			continue
		}
		if imp.Visibility != ast.VisibilityPublic && imp.Visibility != ast.VisibilityProtected {
			continue
		}
		diags = append(diags, Diagnostic{
			Severity: SeverityError,
			Span:     imp.Span(),
			Message:  msgTopLevelImportPrivate,
			Code:     "import-top-level-visibility",
			Source:   "constraint",
		})
	}
	return diags
}
