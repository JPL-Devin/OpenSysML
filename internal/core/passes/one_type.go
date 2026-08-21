package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

const msgEnumerationAttributeTypes = "An enumeration attribute cannot have more than one type."

// Pilot SysMLValidator (2026-05) checkOneType: these usage kinds are typed by
// exactly one definition, so a second declared type is an error. Messages are
// the reference's, per kind.
var oneTypeUsageMessages = map[ast.UsageKind]string{
	ast.UsageCalc:             "A calculation must be typed by one calculation definition.",
	ast.UsageConstraint:       "A constraint must be typed by one constraint definition.",
	ast.UsageRequirement:      "A requirement must be typed by one requirement definition.",
	ast.UsageCase:             "A case must be typed by one case definition.",
	ast.UsageAnalysisCase:     "An analysis case must be typed by one analysis case definition.",
	ast.UsageVerificationCase: "A verification case must be typed by one verification case definition.",
	ast.UsageUseCase:          "A use case must be typed by one use case definition.",
	ast.UsageEnumeration:      "An enumeration must be typed by one enumeration definition.",
	ast.UsageRendering:        "A rendering must be typed by one rendering definition.",
	ast.UsageViewpoint:        "A viewpoint must be typed by one viewpoint definition.",
	ast.UsageView:             "A view must be typed by one view definition.",
	ast.UsageMetadata:         "A metadata usage must be typed by one metadata definition.",
}

// checkOneType reports a usage that declares more than one type where the
// reference admits one. A usage declaring none is silent: the reference counts
// the type its library base supplies, so there is exactly one.
func (tc *typeChecker) checkOneType(scope *symbols.Scope, u *ast.Usage) {
	typings := 0
	for _, rel := range u.Relationships {
		if rel != nil && rel.Kind == ast.RelTyping {
			typings++
		}
	}
	if typings <= 1 {
		return
	}
	msg, ok := oneTypeUsageMessages[u.Kind]
	// An attribute typed by an enumeration definition takes no other type
	// (SysMLValidator checkAttributeUsage), even though attributes otherwise may.
	if !ok && u.Kind == ast.UsageAttribute && tc.typesAnEnumeration(scope, u) {
		msg, ok = msgEnumerationAttributeTypes, true
	}
	if !ok {
		return
	}
	tc.diags = append(tc.diags, Diagnostic{
		Severity: SeverityError,
		Span:     u.Span(),
		Message:  msg,
		Code:     "one-type",
		Source:   "type",
	})
}

func (tc *typeChecker) typesAnEnumeration(scope *symbols.Scope, u *ast.Usage) bool {
	for _, rel := range u.Relationships {
		if rel == nil || rel.Kind != ast.RelTyping || rel.Target == nil {
			continue
		}
		target := rel.Target
		if fr, ok := target.(*ast.FeatureReference); ok {
			target = fr.Name
		}
		qn, ok := target.(*ast.QualifiedName)
		if !ok {
			continue
		}
		sym, ok := tc.resolver.ResolveQualified(scope, qn)
		if !ok || sym == nil {
			continue
		}
		if sym.Kind == symbols.SymbolAlias {
			if resolved, ok := tc.resolver.ResolveAliasTarget(sym); ok && resolved != nil {
				sym = resolved
			}
		}
		if sym.Kind == symbols.SymbolEnumerationDef {
			return true
		}
	}
	return false
}
