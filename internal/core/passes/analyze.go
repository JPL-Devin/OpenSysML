package passes

import (
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// DefaultRegistry returns the default pass registry: syntax, name resolution,
// state transitions, type checking, and semantic constraints.
func DefaultRegistry() *Registry {
	reg := NewRegistry()
	reg.Register(SyntaxPass{})
	reg.Register(ImportVisibilityPass{})
	reg.Register(NonstandardNotationPass{})
	reg.Register(NameResolutionPass{})
	reg.Register(StateTransitionPass{})
	reg.Register(TypeCheckPass{})
	reg.Register(ElementFilterPass{})
	reg.Register(ConstraintPass{})
	reg.Register(TypeRelationshipsPass{})
	reg.Register(MultiplicityBoundsPass{})
	reg.Register(ReferenceSubsettingPass{})
	reg.Register(TopLevelImportPass{})
	reg.Register(AssociationEndTypesPass{})
	reg.Register(VariableFeaturePass{})
	reg.Register(ResultExpressionPass{})
	reg.Register(FeatureReferencePass{})
	reg.Register(MetadataTypePass{})
	reg.Register(RedefinitionConformancePass{})
	return reg
}

// Analyze runs the default validation passes over a single parsed document and
// returns diagnostics sorted by span offset, then source, then message.
// parseDiags are the parser's diagnostics, already adapted to passes.Diagnostic
// by the caller.
func Analyze(name string, root *ast.RootNamespace, parseDiags []Diagnostic, idx *symbols.Index) []Diagnostic {
	return AnalyzeWithKind(name, source.KindOf(name), root, parseDiags, idx)
}

// AnalyzeWithKind validates a document whose language is not encoded in name.
func AnalyzeWithKind(name string, kind source.Kind, root *ast.RootNamespace,
	parseDiags []Diagnostic, idx *symbols.Index) []Diagnostic {
	ctx := NewContextWithKind(name, kind, idx, parseDiags)
	diags := DefaultRegistry().Run(ctx, name, root)
	sort.SliceStable(diags, func(i, j int) bool {
		a, b := diags[i], diags[j]
		if a.Span.Offset != b.Span.Offset {
			return a.Span.Offset < b.Span.Offset
		}
		if a.Source != b.Source {
			return a.Source < b.Source
		}
		return a.Message < b.Message
	})
	return diags
}
