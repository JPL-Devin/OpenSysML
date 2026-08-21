package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Pilot KerMLValidator (2026-05) checkMetadataFeature / checkMetadataBodyFeature,
// constraints INVALID_METADATA_FEATURE_METACLASS_NOT_ABSTRACT and
// INVALID_METADATA_FEATURE_BODY.
const (
	msgMetadataConcreteType = "Must have a concrete type"
	msgMetadataBodyFeature  = "Must redefine an owning-type feature"
)

// W8DMetadataUsagePass checks an annotation against the metadata definition it
// names: the definition must be concrete, and every feature its body writes must
// be one of the definition's own (KerML §7.5 metadata features).
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
	var prefixes []*ast.PrefixMetadata
	var members []ast.Node
	switch d := sym.Decl.(type) {
	case *ast.Definition:
		prefixes, members = d.Prefixes, d.Members
	case *ast.Usage:
		prefixes, members = d.Prefixes, d.Members
		if d.Kind == ast.UsageMetadata {
			mc.checkMetadataUsage(sym, d)
		}
	case *ast.Package:
		prefixes, members = d.Prefixes, d.Members
	case *ast.Namespace:
		prefixes, members = d.Prefixes, d.Members
	default:
		return
	}
	for _, p := range prefixes {
		mc.checkAnnotation(sym.OwnerScope, p.Type, p.Body, p.Span())
	}
	for _, member := range members {
		if mem, ok := member.(*ast.Membership); ok {
			member = mem.Member
		}
		// Prefix metadata written as a member annotates the element owning the body.
		if p, ok := member.(*ast.PrefixMetadata); ok {
			mc.checkAnnotation(w8dScopeOf(sym), p.Type, p.Body, p.Span())
		}
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
	// A library symbol carries no declaration, so an abstract library metaclass
	// is not judged here (known limitation: libs.symRecord records no
	// abstractness).
	if def, isDef := typ.Decl.(*ast.Definition); isDef && def.IsAbstract {
		mc.diags = append(mc.diags, Diagnostic{
			Severity: SeverityError,
			Span:     span,
			Message:  msgMetadataConcreteType,
			Code:     "metadata-abstract-type",
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
