package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// KerMLValidator's validateMetadataFeatureMetaclassNotAbstract message.
const msgMetadataConcreteType = "Must have a concrete type"

// MetadataTypePass reports an annotation whose metaclass is abstract
// (KerML 1.0 §8.3.4.9.2, validateMetadataFeatureMetaclassNotAbstract).
type MetadataTypePass struct{}

func (MetadataTypePass) Level() PassLevel { return LevelType }

// Its subject is one annotation, and the reference that subject rests on is the
// metaclass it names: a failure on another element does not hide it.
func (MetadataTypePass) ElementScoped() { /* marker: per-element gating */ }

func (MetadataTypePass) Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic {
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	c := &metadataTypeChecker{ctx: ctx, resolver: ctx.Resolver()}
	w := &w8cWalker{ctx: ctx}
	w.walk(rootScope, c.checkSymbol)
	return c.diags
}

type metadataTypeChecker struct {
	ctx      *Context
	resolver interface {
		ResolveQualified(*symbols.Scope, *ast.QualifiedName) (*symbols.Symbol, bool)
	}
	diags []Diagnostic
}

func (c *metadataTypeChecker) checkSymbol(sym *symbols.Symbol) {
	scope := sym.Scope
	if scope == nil {
		scope = sym.OwnerScope
	}
	if scope == nil {
		return
	}
	for _, pm := range w8cPrefixMetadata(sym.Decl) {
		c.check(scope, pm)
	}
}

func (c *metadataTypeChecker) check(scope *symbols.Scope, pm *ast.PrefixMetadata) {
	if pm.Type == nil || c.ctx.DownstreamOfFailure(pm.Type) {
		return
	}
	target, ok := c.resolver.ResolveQualified(scope, pm.Type)
	if !ok || target == nil {
		return
	}
	if !symbols.IsAbstract(target) {
		return
	}
	c.diags = append(c.diags, Diagnostic{
		Severity: SeverityError,
		Span:     pm.Span(),
		Message:  msgMetadataConcreteType,
		Code:     "metadata-concrete-type",
		Source:   "constraint",
	})
}

// w8cPrefixMetadata returns the annotations a declaration carries, both as
// prefixes and as members of its body.
func w8cPrefixMetadata(decl ast.Node) []*ast.PrefixMetadata {
	var prefixes []*ast.PrefixMetadata
	var members []ast.Node
	switch d := decl.(type) {
	case *ast.Definition:
		prefixes, members = d.Prefixes, d.Members
	case *ast.Usage:
		prefixes, members = d.Prefixes, d.Members
	case *ast.Package:
		prefixes, members = d.Prefixes, d.Members
	case *ast.Namespace:
		prefixes, members = d.Prefixes, d.Members
	default:
		return nil
	}
	out := append([]*ast.PrefixMetadata(nil), prefixes...)
	for _, m := range members {
		if mem, ok := m.(*ast.Membership); ok {
			if pm, ok := mem.Member.(*ast.PrefixMetadata); ok {
				out = append(out, pm)
			}
		}
	}
	return out
}
