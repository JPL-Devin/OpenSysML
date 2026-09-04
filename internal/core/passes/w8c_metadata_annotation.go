package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// MetadataAnnotationPass checks a metadata annotation: what it may annotate, and
// that its body restates features of its type and binds model-level evaluable
// values (KerML 1.0 §7.4.9, §8.3.4.9).
type MetadataAnnotationPass struct{}

const (
	// msgCannotAnnotate is completed with the metaclass of the element the
	// annotation may not be applied to.
	msgCannotAnnotate = "Cannot annotate "
	// msgOwningTypeFeature is reported on a body feature restating nothing.
	msgOwningTypeFeature = "Must redefine an owning-type feature"
)

func (MetadataAnnotationPass) Level() PassLevel { return LevelType }

func (MetadataAnnotationPass) Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	c := &metadataAnnotationChecker{model: ctx.Model()}
	if c.model == nil {
		return nil
	}
	w := &w8cWalker{ctx: ctx}
	w.walk(rootScope, c.checkSymbol)
	return c.diags
}

type metadataAnnotationChecker struct {
	model *semantics.Model
	diags []Diagnostic
}

// checkSymbol checks each annotation of sym's declaration. A prefix annotation
// names its type in the owning namespace; one written as a member names it in
// the annotated element's own body scope.
func (c *metadataAnnotationChecker) checkSymbol(sym *symbols.Symbol) {
	for _, a := range semantics.MetadataAnnotationsOf(sym.Decl) {
		scope := c.annotationScope(sym, a)
		if scope == nil {
			continue
		}
		if metaclass, bad := c.model.AnnotatedElementViolation(sym, scope, a.Node.Type); bad {
			c.reportCannotAnnotate(a.Node.Span(), metaclass)
		}
		c.checkBody(scope, a.Node)
	}
	for _, a := range semantics.MetadataAnnotationsAboutOthers(sym.Decl) {
		scope := c.annotationScope(sym, a)
		if scope == nil {
			continue
		}
		for _, metaclass := range c.model.AboutAnnotatedElementViolations(scope, a.Node.Type, a.Node.About) {
			c.reportCannotAnnotate(a.Node.Span(), metaclass)
		}
		c.checkBody(scope, a.Node)
	}
	if u, ok := sym.Decl.(*ast.Usage); ok && u.Kind == ast.UsageMetadata {
		c.checkMetadataUsage(sym, u)
	}
}

// annotationScope is where an annotation names its type: a prefix in the owning
// namespace, a member in the annotated element's own body scope.
func (c *metadataAnnotationChecker) annotationScope(sym *symbols.Symbol, a semantics.MetadataAnnotation) *symbols.Scope {
	if !a.Prefix && sym.Scope != nil {
		return sym.Scope
	}
	return sym.OwnerScope
}

// checkMetadataUsage checks what `metadata m : M about x;` may annotate, or, with
// no `about`, whether M may annotate the element owning the usage.
func (c *metadataAnnotationChecker) checkMetadataUsage(sym *symbols.Symbol, u *ast.Usage) {
	if sym.OwnerScope == nil {
		return
	}
	var typeRef *ast.QualifiedName
	var about []*ast.QualifiedName
	for _, rel := range u.Relationships {
		if rel == nil {
			continue
		}
		qn, ok := rel.Target.(*ast.QualifiedName)
		if !ok {
			continue
		}
		switch rel.Kind {
		case ast.RelTyping:
			if typeRef == nil {
				typeRef = qn
			}
		case ast.RelAnnotates:
			about = append(about, qn)
		}
	}
	if typeRef == nil {
		return
	}
	if len(about) > 0 {
		for _, metaclass := range c.model.AboutAnnotatedElementViolations(sym.OwnerScope, typeRef, about) {
			c.reportCannotAnnotate(u.Span(), metaclass)
		}
		return
	}
	if metaclass, bad := c.model.AnnotatedElementViolation(sym.OwnerScope.Owner(), sym.OwnerScope, typeRef); bad {
		c.reportCannotAnnotate(u.Span(), metaclass)
	}
}

func (c *metadataAnnotationChecker) reportCannotAnnotate(span source.Span, metaclass string) {
	c.diags = append(c.diags, Diagnostic{
		Severity: SeverityError,
		Span:     span,
		Message:  msgCannotAnnotate + metaclass,
		Code:     "metadata-annotated-element",
		Source:   "constraint",
	})
}

// checkBody checks that the annotation's body restates features of its type and
// binds model-level evaluable values.
func (c *metadataAnnotationChecker) checkBody(scope *symbols.Scope, prefix *ast.PrefixMetadata) {
	for _, node := range c.model.MetadataBodyViolations(scope, prefix) {
		c.diags = append(c.diags, Diagnostic{
			Severity: SeverityError,
			Span:     node.Span(),
			Message:  msgOwningTypeFeature,
			Code:     "metadata-owning-type-feature",
			Source:   "constraint",
		})
	}
	for _, value := range c.model.MetadataBodyInevaluableValues(scope, prefix) {
		c.diags = append(c.diags, Diagnostic{
			Severity: SeverityError,
			Span:     metadataValueSpan(prefix.Body, value),
			Message:  msgFilterNotEvaluable,
			Code:     "metadata-value-not-evaluable",
			Source:   "constraint",
		})
	}
}

// metadataValueSpan is the span of the binding `= value` in a metadata body, or of the value alone.
func metadataValueSpan(body []ast.Node, value ast.Node) source.Span {
	var found source.Span
	var walk func([]ast.Node)
	walk = func(body []ast.Node) {
		for _, node := range body {
			if mem, ok := node.(*ast.Membership); ok {
				node = mem.Member
			}
			usage, ok := node.(*ast.Usage)
			if !ok {
				continue
			}
			if usage.Value == value && usage.ValueOperatorSpan.Len > 0 {
				end := value.Span().End()
				found = source.Span{
					Offset: usage.ValueOperatorSpan.Offset,
					Len:    end - usage.ValueOperatorSpan.Offset,
				}
				return
			}
			walk(usage.Members)
			if found.Len > 0 {
				return
			}
		}
	}
	walk(body)
	if found.Len > 0 {
		return found
	}
	return value.Span()
}
