package resolve

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// importAllowsPrivate reports whether an import re-exports private members.
// Only `import all` widens visibility to include private members.
func importAllowsPrivate(imp *ast.Import) bool {
	return imp.IsAll
}

// visibleThroughImport reports whether sym may be surfaced by imp when
// enumerating a namespace's members. Private members are hidden unless the
// import is `import all`.
func visibleThroughImport(imp *ast.Import, sym *symbols.Symbol) bool {
	if sym.Visibility == ast.VisibilityPrivate {
		return importAllowsPrivate(imp)
	}
	return true
}
