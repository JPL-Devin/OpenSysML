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
	w := &w8cWalker{seen: map[*symbols.Symbol]bool{}}
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
		scope := sym.OwnerScope
		if !a.Prefix && sym.Scope != nil {
			scope = sym.Scope
		}
		c.check(sym, scope, a.Node)
	}
}

func (c *metadataAnnotationChecker) check(sym *symbols.Symbol, scope *symbols.Scope, prefix *ast.PrefixMetadata) {
	if scope == nil {
		return
	}
	if metaclass, bad := c.model.AnnotatedElementViolation(sym, scope, prefix); bad {
		c.diags = append(c.diags, Diagnostic{
			Severity: SeverityError,
			Span:     prefix.Span(),
			Message:  msgCannotAnnotate + metaclass,
			Code:     "metadata-annotated-element",
			Source:   "constraint",
		})
	}
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
			Span:     c.metadataValueSpan(prefix, value),
			Message:  msgFilterNotEvaluable,
			Code:     "metadata-value-not-evaluable",
			Source:   "constraint",
		})
	}
}

func (c *metadataAnnotationChecker) metadataValueSpan(prefix *ast.PrefixMetadata, value ast.Node) source.Span {
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
	walk(prefix.Body)
	if found.Len > 0 {
		return found
	}
	return value.Span()
}
