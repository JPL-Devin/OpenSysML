package passes

import (
	"fmt"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/resolve"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// TypeCheckPass validates that each def/usage relationship target has a symbol
// kind compatible with the source node and relationship kind (spec §6.3).
// It runs at LevelType, after name resolution; unresolved targets are skipped.
type TypeCheckPass struct{}

func (TypeCheckPass) Level() PassLevel { return LevelType }

func (TypeCheckPass) Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	tc := &typeChecker{resolver: ctx.Resolver()}
	tc.walk(rootScope, root.Members)
	return tc.diags
}

type typeChecker struct {
	resolver *resolve.Resolver
	diags    []Diagnostic
}

func (tc *typeChecker) walk(scope *symbols.Scope, members []ast.Node) {
	for _, m := range members {
		switch d := unwrapType(m).(type) {
		case *ast.Definition:
			tc.checkRelationships(scope, d.Relationships, true, d.Kind, 0)
			if child := childScopeOf(scope, d); child != nil {
				tc.walk(child, d.Members)
			}
		case *ast.Usage:
			tc.checkRelationships(scope, d.Relationships, false, 0, d.Kind)
			if child := childScopeOf(scope, d); child != nil {
				tc.walk(child, d.Members)
			}
		case *ast.Package:
			if child := childScopeOf(scope, d); child != nil {
				tc.walk(child, d.Members)
			}
		case *ast.Namespace:
			if child := childScopeOf(scope, d); child != nil {
				tc.walk(child, d.Members)
			}
		}
	}
}

func (tc *typeChecker) checkRelationships(scope *symbols.Scope, rels []*ast.Relationship, isDef bool, defKind ast.DefinitionKind, useKind ast.UsageKind) {
	for _, rel := range rels {
		if rel == nil || rel.Target == nil {
			continue
		}
		sym, ok := tc.resolver.ResolveQualified(scope, rel.Target)
		if !ok || sym == nil {
			continue // unresolved: name-resolution tier owns this
		}
		if msg := compatMessage(isDef, defKind, useKind, rel.Kind, sym.Kind); msg != "" {
			tc.diags = append(tc.diags, Diagnostic{
				Severity: SeverityError,
				Span:     rel.Target.Span(),
				Message:  msg,
				Code:     "type",
				Source:   "type",
			})
		}
	}
}

func compatMessage(isDef bool, defKind ast.DefinitionKind, useKind ast.UsageKind, rel ast.RelationshipKind, target symbols.SymbolKind) string {
	switch rel {
	case ast.RelSpecializes:
		want := defSymbolKind(defKind)
		if !isDef {
			return "only a definition may specialize; found a usage"
		}
		if !isDefKind(target) {
			return fmt.Sprintf("%s cannot specialize %s (target is not a definition)", defKind, target)
		}
		if target != want {
			return fmt.Sprintf("%s cannot specialize %s (kind mismatch)", defKind, target)
		}
	case ast.RelSubsets, ast.RelRedefines:
		if isDef {
			return fmt.Sprintf("a definition may not %s a feature", rel)
		}
		if !isUsageKind(target) {
			return fmt.Sprintf("%s target must be a usage, found %s", rel, target)
		}
	case ast.RelTyping:
		if isDef {
			return "" // typing on a definition is not produced by the parser; ignore
		}
		if !isDefKind(target) {
			return fmt.Sprintf("type must be a definition, found %s", target)
		}
		if target != usageWantsDefKind(useKind) {
			return fmt.Sprintf("%s cannot be typed by %s (kind mismatch)", useKind, target)
		}
	case ast.RelReferences, ast.RelCrosses:
		if !isDef && !isUsageKind(target) {
			return fmt.Sprintf("%s target must be a usage, found %s", rel, target)
		}
	}
	return ""
}

func defSymbolKind(k ast.DefinitionKind) symbols.SymbolKind {
	switch k {
	case ast.DefPart:
		return symbols.SymbolPartDef
	case ast.DefAttribute:
		return symbols.SymbolAttributeDef
	}
	return symbols.SymbolUnknown
}

func usageWantsDefKind(k ast.UsageKind) symbols.SymbolKind {
	switch k {
	case ast.UsagePart:
		return symbols.SymbolPartDef
	case ast.UsageAttribute:
		return symbols.SymbolAttributeDef
	}
	return symbols.SymbolUnknown
}

func isDefKind(k symbols.SymbolKind) bool {
	return k == symbols.SymbolPartDef || k == symbols.SymbolAttributeDef
}

func isUsageKind(k symbols.SymbolKind) bool {
	return k == symbols.SymbolPartUsage || k == symbols.SymbolAttributeUsage
}

func unwrapType(n ast.Node) ast.Node {
	if m, ok := n.(*ast.Membership); ok {
		return m.Member
	}
	return n
}

func childScopeOf(scope *symbols.Scope, decl ast.Node) *symbols.Scope {
	for _, c := range scope.Children() {
		if c.Node() == decl {
			return c
		}
	}
	return nil
}
