package passes

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

// NameResolutionPass resolves every reference in a document via the Plan-3
// resolver and reports unresolved / ambiguous references.
type NameResolutionPass struct{}

// Level reports the name-resolution dependency level.
func (NameResolutionPass) Level() PassLevel { return LevelNameResolution }

// Run resolves the document and adapts resolver diagnostics.
func (NameResolutionPass) Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	// Initialize model to enable inheritance-aware resolution
	_ = ctx.Model()
	r := ctx.Resolver()
	r.ResolveDocument(name, root)
	rd := r.Diagnostics
	if len(rd) == 0 {
		return nil
	}
	out := make([]Diagnostic, 0, len(rd))
	for _, d := range rd {
		code := d.Code
		if code == "" {
			code = "unresolved"
			if strings.HasPrefix(d.Message, "ambiguous") {
				code = "ambiguous"
			}
		}
		out = append(out, Diagnostic{
			Severity: SeverityError,
			Span:     d.Span,
			Message:  d.Message,
			Code:     code,
			Source:   "name-resolution",
			Fixes:    d.Fixes,
		})
	}
	return out
}
