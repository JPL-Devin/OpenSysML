package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Pilot KerMLValidator (2026-05) checkMetadataBodyFeature, constraint
// INVALID_METADATA_FEATURE_BODY.
const msgMetadataBodyFeature = "Must redefine an owning-type feature"

// W8DMetadataUsagePass checks a `metadata m : A { … }` usage against the
// metadata definition it is typed by: every feature its body writes must be one
// of the definition's own (KerML §7.5), and the definition must be concrete.
// Prefix annotations are RedefinitionConformancePass's (body features) and
// MetadataTypePass's (concrete type).
type W8DMetadataUsagePass struct{}

func (W8DMetadataUsagePass) Level() PassLevel { return LevelConstraint }

func (W8DMetadataUsagePass) Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	mc := &w8dMetadataChecker{resolver: ctx.Resolver(), model: ctx.Model()}
	if mc.resolver == nil || mc.model == nil {
		return nil
	}
	w8dWalkSymbols(rootScope, mc.check)
	return mc.diags
}

type w8dMetadataChecker struct {
	resolver *resolve.Resolver
	model    *semantics.Model
	diags    []Diagnostic
}

func (mc *w8dMetadataChecker) check(sym *symbols.Symbol) {
	if u, ok := sym.Decl.(*ast.Usage); ok && u.Kind == ast.UsageMetadata {
		mc.checkMetadataUsage(sym, u)
	}
}

// checkMetadataUsage checks `metadata m : A { … }`, whose annotated metadata
// definition is what the usage is typed by.
func (mc *w8dMetadataChecker) checkMetadataUsage(sym *symbols.Symbol, u *ast.Usage) {
	for _, rel := range u.Relationships {
		if rel == nil || rel.Kind != ast.RelTyping {
			continue
		}
		qn, ok := rel.Target.(*ast.QualifiedName)
		if !ok {
			continue
		}
		mc.checkAnnotation(sym.OwnerScope, qn, u.Members, u.Span())
		return
	}
}

func (mc *w8dMetadataChecker) checkAnnotation(scope *symbols.Scope, typeRef *ast.QualifiedName, body []ast.Node, span source.Span) {
	if scope == nil || typeRef == nil {
		return
	}
	typ, ok := mc.resolver.ResolveQualified(scope, typeRef)
	if !ok || typ == nil {
		return
	}
	if resolved, aliasOK := mc.resolver.ResolveAliasTarget(typ); aliasOK {
		typ = resolved
	} else {
		return
	}
	if symbols.IsAbstract(typ) {
		mc.diags = append(mc.diags, Diagnostic{
			Severity: SeverityError,
			Span:     span,
			Message:  msgMetadataConcreteType,
			Code:     "metadata-concrete-type",
			Source:   "constraint",
		})
		return
	}
	mc.checkBody(typ, body)
}

// checkBody reports every feature of an annotation body that names no feature of
// the annotated type, and recurses into the body of the ones that do.
func (mc *w8dMetadataChecker) checkBody(typ *symbols.Symbol, body []ast.Node) {
	for _, member := range body {
		if mem, ok := member.(*ast.Membership); ok {
			member = mem.Member
		}
		u, ok := member.(*ast.Usage)
		if !ok {
			continue
		}
		name, _ := ast.EffectiveName(u)
		if name == "" {
			continue
		}
		feature, found := mc.model.LookupMember(typ, name)
		if !found || feature == nil {
			mc.diags = append(mc.diags, Diagnostic{
				Severity: SeverityError,
				Span:     u.Span(),
				Message:  msgMetadataBodyFeature,
				Code:     "metadata-body-feature",
				Source:   "constraint",
			})
			continue
		}
		mc.checkBody(feature, u.Members)
	}
}
