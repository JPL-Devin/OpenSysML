package runtime

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// EffectiveFeature represents one feature value in a type's flattened schema:
// own + inherited − redefined/masked, carrying type + multiplicity + default.
type EffectiveFeature struct {
	Name         string
	Symbol       *symbols.Symbol // the declaring feature symbol
	OwnerType    *symbols.Symbol // type that declares this feature (may be supertype)
	Type         *symbols.Symbol // resolved type (nil if untyped)
	Multiplicity semantics.Range // declared or inherited (default 1..1)
	DefaultValue ast.Node        // value-binding expression (nil if none)
	DefaultDecl  *symbols.Symbol // feature the DefaultValue was written on (nil if none)
}

// DefaultScope returns the scope DefaultValue resolves its names in, which for
// an inherited default is where the redefined declaration wrote it.
func (f *EffectiveFeature) DefaultScope() *symbols.Scope {
	if f.DefaultDecl != nil {
		return f.DefaultDecl.OwnerScope
	}
	return f.DeclScope()
}

// DeclScope returns the scope the feature was declared in, which is the scope a
// default value written on it must be evaluated in: an inherited feature's
// default refers to names visible where the supertype was written, not where
// the instantiated type is.
func (f *EffectiveFeature) DeclScope() *symbols.Scope {
	if f.Symbol == nil {
		return nil
	}
	return f.Symbol.OwnerScope
}

// declScope returns the scope a declaration's body was written in: the scope the
// declaration owns, in which its own members are visible to each other, falling
// back to the scope it was declared in when it owns none. It is the scope an
// expression written among its members resolves its names against — an
// attribute default, a guard, an assignment in a nested action body.
func declScope(sym *symbols.Symbol) *symbols.Scope {
	if sym == nil {
		return nil
	}
	if sym.Scope != nil {
		return sym.Scope
	}
	return sym.OwnerScope
}

// FeaturesOf returns the ordered, deduplicated effective-feature list for the given type symbol.
// Result: own + inherited − redefined/masked, memoized per symbol.
func (ctx *Context) FeaturesOf(typeSym *symbols.Symbol) []EffectiveFeature {
	if typeSym == nil {
		return nil
	}

	// Memoization
	if cached, ok := ctx.features[typeSym]; ok {
		return cached
	}

	features := ctx.buildFeatures(typeSym)
	ctx.features[typeSym] = features
	return features
}

// buildFeatures constructs the effective-feature list by walking the type hierarchy.
func (ctx *Context) buildFeatures(typeSym *symbols.Symbol) []EffectiveFeature {
	// Collect all members (local + inherited) using semantics.MembersOf
	allMembers := ctx.model.MembersOf(typeSym)

	// Track which features to keep (deduplication by name: last declarator wins per masking/redefinition)
	featureMap := make(map[string]EffectiveFeature)
	seen := make(map[*symbols.Symbol]bool)

	// Process members in order (local first, then inherited)
	for _, memberSym := range allMembers {
		// Dedupe by pointer (short+primary names alias the same symbol)
		if seen[memberSym] {
			continue
		}
		seen[memberSym] = true

		// Only include features (attributes, parts, etc.)
		if !isFeature(memberSym) {
			continue
		}
		// A variant is a choice offered for its variation, not a feature of the
		// object declaring it: it materializes no feature value of its own. A `variant`
		// outside a variation offers no choice, so it stays an ordinary feature.
		if ctx.model.VariationPointOwning(memberSym) != nil {
			continue
		}

		name := memberSym.Name
		typ := ctx.extractType(memberSym)
		mult, multStated := ctx.extractMultiplicity(memberSym)
		if !multStated {
			if inherited, ok := ctx.redefinedMultiplicity(memberSym, typeSym); ok {
				mult = inherited
			}
		}
		defaultVal := ctx.extractDefaultValue(memberSym)
		defaultDecl := memberSym
		if defaultVal == nil {
			defaultVal, defaultDecl = ctx.redefinedDefault(memberSym, typeSym)
		}

		// Determine owner type (walk up to find the declaration scope's owner)
		ownerType := ctx.findOwnerType(memberSym)

		// Store feature; last one wins (redefinition/masking)
		featureMap[name] = EffectiveFeature{
			Name:         name,
			Symbol:       memberSym,
			OwnerType:    ownerType,
			Type:         typ,
			Multiplicity: mult,
			DefaultValue: defaultVal,
			DefaultDecl:  defaultDecl,
		}
	}

	// Convert map to ordered list (stable order: iterate over allMembers and pick from map)
	result := make([]EffectiveFeature, 0, len(featureMap))
	seenNames := make(map[string]bool)
	for _, memberSym := range allMembers {
		name := memberSym.Name
		if seenNames[name] {
			continue
		}
		if feat, ok := featureMap[name]; ok {
			result = append(result, feat)
			seenNames[name] = true
		}
	}

	return append(result, ctx.connectorEndFeatures(typeSym, seenNames)...)
}

// isFeature returns true if the symbol represents a structural feature (attribute, part, etc.).
func isFeature(sym *symbols.Symbol) bool {
	switch sym.Kind {
	case symbols.SymbolAttributeUsage, symbols.SymbolPartUsage, symbols.SymbolItemUsage,
		symbols.SymbolPortUsage, symbols.SymbolConnectionUsage, symbols.SymbolActionUsage,
		symbols.SymbolStateUsage, symbols.SymbolConstraintUsage, symbols.SymbolRequirementUsage,
		symbols.SymbolOccurrenceUsage, symbols.SymbolIndividualUsage,
		symbols.SymbolInterfaceUsage, symbols.SymbolFlowUsage,
		// An allocation usage is a connection usage of the allocation library
		// (SysML v2 §8.3.19), so an object carries it as a feature.
		symbols.SymbolAllocationUsage:
		return true
	default:
		return false
	}
}

// extractType resolves the type of a feature: the one it declares, or the one
// it inherits from what it redefines or subsets when it restates none
// (KerML 1.0 §7.4.7).
func (ctx *Context) extractType(featureSym *symbols.Symbol) *symbols.Symbol {
	if typ := ctx.declaredType(featureSym); typ != nil {
		return typ
	}
	for _, sup := range ctx.model.AllSupertypes(featureSym) {
		if typ := ctx.declaredType(sup); typ != nil {
			return typ
		}
	}
	return nil
}

// declaredType resolves the type a feature states itself, ignoring inheritance.
func (ctx *Context) declaredType(featureSym *symbols.Symbol) *symbols.Symbol {
	// Check usage relationships for typing
	rels := semantics.RelationshipsOf(featureSym)
	for _, rel := range rels {
		if rel.Kind == ast.RelTyping && rel.Target != nil {
			// Unwrap FeatureReference if needed
			target := rel.Target
			if fr, ok := target.(*ast.FeatureReference); ok {
				target = fr.Name
			}
			if qn, ok := target.(*ast.QualifiedName); ok {
				if resolved, ok := ctx.resolver.ResolveQualified(featureSym.OwnerScope, qn); ok {
					return resolved
				}
			}
		}
	}
	return nil
}

// extractMultiplicity returns the multiplicity governing a feature. stated is
// false when it declares none and the assumed 1..1 governs it instead.
func (ctx *Context) extractMultiplicity(featureSym *symbols.Symbol) (r semantics.Range, stated bool) {
	_, stated = ctx.model.MultiplicityOf(featureSym)
	return ctx.model.EffectiveMultiplicityOf(featureSym), stated
}

// extractDefaultValue returns the default-value expression for a feature (nil if none).
func (ctx *Context) extractDefaultValue(featureSym *symbols.Symbol) ast.Node {
	if usage, ok := featureSym.Decl.(*ast.Usage); ok {
		return usage.Value // nil if no default
	}
	return nil
}

// redefinedMultiplicity returns the multiplicity a feature takes from the
// feature it redefines when it declares none: a redefining feature is the
// redefined feature declared again (KerML 1.0 §7.3.4.5), so `:>> xs = (1, 2)`
// is bound by the `xs` it restates. Subsetting is not inherited this way — a
// subsetting feature is one of the subsetted feature's values, not the same
// feature.
func (ctx *Context) redefinedMultiplicity(sym, owner *symbols.Symbol) (semantics.Range, bool) {
	seen := map[*symbols.Symbol]bool{sym: true}
	for queue := []*symbols.Symbol{sym}; len(queue) > 0; {
		cur := queue[0]
		queue = queue[1:]
		for _, redefined := range ctx.relatedFeatures(cur, owner, ast.RelRedefines) {
			if seen[redefined] {
				continue
			}
			seen[redefined] = true
			if mult, ok := ctx.model.MultiplicityOf(redefined); ok {
				return mult, true
			}
			queue = append(queue, redefined)
		}
	}
	return semantics.Range{}, false
}

// redefinedDefault returns the value a feature takes from the feature it
// redefines, and the declaration that wrote it: a redefining feature is the
// redefined feature declared again (KerML 1.0 §7.3.4.5).
func (ctx *Context) redefinedDefault(sym, owner *symbols.Symbol) (ast.Node, *symbols.Symbol) {
	seen := map[*symbols.Symbol]bool{sym: true}
	for queue := []*symbols.Symbol{sym}; len(queue) > 0; {
		cur := queue[0]
		queue = queue[1:]
		for _, redefined := range ctx.relatedFeatures(cur, owner, ast.RelRedefines) {
			if seen[redefined] {
				continue
			}
			seen[redefined] = true
			if val := ctx.extractDefaultValue(redefined); val != nil {
				return val, redefined
			}
			queue = append(queue, redefined)
		}
	}
	return nil, nil
}

// findOwnerType walks up the scope chain to find the type symbol that owns the feature's declaration.
func (ctx *Context) findOwnerType(featureSym *symbols.Symbol) *symbols.Symbol {
	// Start from the feature's owner scope (the scope that contains the declaration)
	ownerScope := featureSym.OwnerScope
	if ownerScope == nil {
		return nil
	}

	// The owner scope's node is the definition/usage that contains the feature
	ownerNode := ownerScope.Node()
	if ownerNode == nil {
		return nil
	}

	// Look up the symbol for the owner node in the parent scope
	parentScope := ownerScope.Parent()
	if parentScope == nil {
		return nil
	}

	// Find the symbol in the parent scope that declares the owner node
	for _, name := range parentScope.MemberNames() {
		syms := parentScope.LookupLocalAll(name)
		for _, sym := range syms {
			if sym.Decl == ownerNode {
				return sym
			}
		}
	}

	return nil
}
