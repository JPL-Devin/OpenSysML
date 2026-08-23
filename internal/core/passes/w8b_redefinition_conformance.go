package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Reference messages of the feature-conformance rules (KerML 8.3.3.3).
const (
	msgDirectionConformance  = "Redefining feature must have a compatible direction"
	msgUniquenessConformance = "Subsetting/redefining feature cannot be nonunique if subsetted/redefined feature is unique"
	msgConstancyConformance  = "Subsetting/redefining feature must be constant if subsetted/redefined feature is constant"
	msgOwningTypeFeature     = "Must redefine an owning-type feature"
)

// RedefinitionConformancePass reports the feature-conformance rules a subsetting
// or redefinition breaks: direction, uniqueness and constancy (KerML 8.3.3.3).
// The predicates live in semantics; this pass only locates and words them.
type RedefinitionConformancePass struct{}

func (RedefinitionConformancePass) Level() PassLevel { return LevelConstraint }

func (RedefinitionConformancePass) Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	rc := &redefinitionConformanceChecker{
		model: ctx.Model(),
		seen:  make(map[*symbols.Symbol]bool),
	}
	rc.walk(rootScope)
	return rc.diags
}

// RedefinitionDirectionPass reports direction conformance as a typing property,
// before constraint passes may be skipped by unrelated type errors in the same
// document.
type RedefinitionDirectionPass struct{}

func (RedefinitionDirectionPass) Level() PassLevel { return LevelType }

func (RedefinitionDirectionPass) Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	rc := &redefinitionConformanceChecker{
		model:         ctx.Model(),
		seen:          make(map[*symbols.Symbol]bool),
		directionOnly: true,
	}
	rc.walk(rootScope)
	return rc.diags
}

type redefinitionConformanceChecker struct {
	model         *semantics.Model
	seen          map[*symbols.Symbol]bool
	diags         []Diagnostic
	directionOnly bool
}

// walk visits every symbol of the subtree once, including anonymous members.
func (rc *redefinitionConformanceChecker) walk(scope *symbols.Scope) {
	if scope == nil {
		return
	}
	for _, sym := range scope.AllMembers() {
		if sym == nil || rc.seen[sym] {
			continue
		}
		rc.seen[sym] = true
		rc.check(sym)
		rc.walk(sym.Scope)
	}
}

// checkMetadataBodies reports the declarations in the metadata annotations of
// sym's declaration that restate no feature of the annotated type. An annotation
// is either a prefix of the declaration or a member of its body, and its names
// resolve in the scope holding it.
func (rc *redefinitionConformanceChecker) checkMetadataBodies(sym *symbols.Symbol) {
	for _, prefix := range declPrefixes(sym.Decl) {
		rc.reportMetadataBody(sym.OwnerScope, prefix)
	}
	for _, node := range declBodyMembers(sym.Decl) {
		if mem, ok := node.(*ast.Membership); ok {
			node = mem.Member
		}
		if prefix, ok := node.(*ast.PrefixMetadata); ok {
			rc.reportMetadataBody(sym.Scope, prefix)
		}
	}
}

func (rc *redefinitionConformanceChecker) reportMetadataBody(scope *symbols.Scope, prefix *ast.PrefixMetadata) {
	for _, node := range rc.model.MetadataBodyViolations(scope, prefix) {
		rc.diags = append(rc.diags, Diagnostic{
			Severity: SeverityError,
			Span:     node.Span(),
			Message:  msgOwningTypeFeature,
			Code:     "metadata-owning-type-feature",
			Source:   "constraint",
		})
	}
}

// declPrefixes returns the metadata annotations written ahead of a declaration.
func declPrefixes(decl ast.Node) []*ast.PrefixMetadata {
	switch d := decl.(type) {
	case *ast.Definition:
		return d.Prefixes
	case *ast.Usage:
		return d.Prefixes
	case *ast.Package:
		return d.Prefixes
	case *ast.Namespace:
		return d.Prefixes
	}
	return nil
}

// declBodyMembers returns the member nodes of a declaration's body.
func declBodyMembers(decl ast.Node) []ast.Node {
	switch d := decl.(type) {
	case *ast.Definition:
		return d.Members
	case *ast.Usage:
		return d.Members
	case *ast.Package:
		return d.Members
	case *ast.Namespace:
		return d.Members
	}
	return nil
}

func (rc *redefinitionConformanceChecker) check(sym *symbols.Symbol) {
	if !rc.directionOnly {
		rc.checkMetadataBodies(sym)
	}
	violations := rc.model.ConformanceViolations(sym)
	for _, v := range violations {
		if !rc.ownsViolation(v.Kind) {
			continue
		}
		msg, code := conformanceMessage(v.Kind)
		if msg == "" || v.Ref == nil {
			continue
		}
		rc.diags = append(rc.diags, Diagnostic{
			Severity: SeverityError,
			Span:     v.Ref.Span(),
			Message:  msg,
			Code:     code,
			Source:   "constraint",
		})
	}
}

func (rc *redefinitionConformanceChecker) ownsViolation(kind semantics.ConformanceViolationKind) bool {
	if rc.directionOnly {
		return kind == semantics.ViolationDirection
	}
	return kind != semantics.ViolationDirection
}

func conformanceMessage(kind semantics.ConformanceViolationKind) (msg, code string) {
	switch kind {
	case semantics.ViolationDirection:
		return msgDirectionConformance, "redefinition-direction-conformance"
	case semantics.ViolationUniqueness:
		return msgUniquenessConformance, "subsetting-uniqueness-conformance"
	case semantics.ViolationConstancy:
		return msgConstancyConformance, "subsetting-constancy-conformance"
	}
	return "", ""
}
