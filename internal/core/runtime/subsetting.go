package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// This file materializes the two ways a feature of an object takes its values
// from another feature of the same object.
//
// Subsetting: the values of a subsetting feature are always values of the
// feature it subsets (KerML 1.0 §7.3.4.4), so a nested part declared
// `part a : Sub :> subsystem` is one of the objects the `subsystem` collection
// holds. That is what a roll-up over an unbounded feature sums over.
//
// Redefinition: a redefining feature is the same feature as the one it
// redefines, seen through a new declaration (KerML 1.0 §7.3.4.5), so
// the two names read one feature value rather than two — `part subsystems :>> Subsystems`
// makes `Subsystems.mass` read the values held under `subsystems`.

// relatedFeatures returns the features of owner that sym's relationships of the
// given kind name. An unqualified name the declaring scope cannot see is looked
// up among owner's members (`part a : Sub :> subsystem`); a target resolving
// outside owner names no feature of it (`:> ISQ::mass`).
func (ctx *Context) relatedFeatures(sym, owner *symbols.Symbol, kind ast.RelationshipKind) []*symbols.Symbol {
	var features []*symbols.Symbol
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel == nil || rel.Kind != kind || rel.Target == nil {
			continue
		}
		target := rel.Target
		if fr, ok := target.(*ast.FeatureReference); ok {
			target = fr.Name
		}
		qn, ok := target.(*ast.QualifiedName)
		if !ok || len(qn.Parts) == 0 {
			continue
		}
		if resolved, ok := ctx.resolver.ResolveQualified(sym.OwnerScope, qn); ok && resolved != nil && resolved != sym {
			if ctx.isFeatureOf(owner, resolved, sym) {
				features = append(features, resolved)
			}
			continue
		}
		if len(qn.Parts) != 1 {
			continue
		}
		if member, found := ctx.model.LookupMember(owner, qn.Parts[0].Text); found && member != nil && member != sym {
			features = append(features, member)
		}
	}
	return features
}

// isFeatureOf reports whether owner carries feature under its name, as its own
// declaration or through what it inherits. A declaration restating a feature
// (`attribute :>> own`) masks the feature it restates under that name, so the
// masked feature is still a feature of the owner and masking names it.
func (ctx *Context) isFeatureOf(owner, feature, masking *symbols.Symbol) bool {
	carries := func(member *symbols.Symbol, ok bool) bool {
		return ok && (member == feature || member == masking)
	}
	if carries(ctx.model.LookupMember(owner, feature.Name)) {
		return true
	}
	return carries(ctx.model.LookupContributedMember(owner, feature.Name))
}

// relatedFeatureNames is relatedFeatures by name.
func (ctx *Context) relatedFeatureNames(sym, owner *symbols.Symbol, kind ast.RelationshipKind) []string {
	features := ctx.relatedFeatures(sym, owner, kind)
	names := make([]string, 0, len(features))
	for _, feat := range features {
		names = append(names, feat.Name)
	}
	return names
}

// aliasRedefinedFeatureValues makes every name of one feature of this instance read one
// feature value: a redefinition declares the redefined feature again under a new name, so
// however many names a chain of redefinitions gives it, they name one set of
// values. The feature value they share is the one the most specific declaration writing a
// value created, whichever of the names that declaration used; one declaration
// valuing two names of the feature is reported rather than silently picked from.
func (ctx *Context) aliasRedefinedFeatureValues(inst *Instance) error {
	features := ctx.FeaturesOf(inst.Type)
	byName := make(map[string]*EffectiveFeature, len(features))
	for i := range features {
		byName[features[i].Name] = &features[i]
	}

	for _, names := range ctx.redefinitionGroups(inst.Type, features) {
		chosen, err := ctx.sharedRedefinitionName(inst, byName, names)
		if err != nil {
			return err
		}
		fv := inst.FeatureValues[chosen]
		for _, name := range names {
			inst.FeatureValues[name] = fv
		}
	}
	return nil
}

// redefinitionGroups groups the names of typ's features by the feature they name,
// in feature order, keeping only names that share a feature with another. Each
// group's names are ordered from the most specific declaration to the least,
// which is the order the features are computed in.
func (ctx *Context) redefinitionGroups(typ *symbols.Symbol, features []EffectiveFeature) [][]string {
	declared := make(map[string]bool, len(features))
	for _, feat := range features {
		declared[feat.Name] = true
	}
	leader := map[string]string{}
	var find func(string) string
	find = func(name string) string {
		next, ok := leader[name]
		if !ok || next == name {
			leader[name] = name
			return name
		}
		root := find(next)
		leader[name] = root
		return root
	}
	for _, feat := range features {
		if feat.Symbol == nil {
			continue
		}
		for _, redefined := range ctx.redefinedNames(feat.Symbol, typ) {
			if redefined == feat.Name || !declared[redefined] {
				continue
			}
			if a, b := find(feat.Name), find(redefined); a != b {
				leader[b] = a
			}
		}
	}

	var groups [][]string
	index := map[string]int{}
	for _, feat := range features {
		if _, ok := leader[feat.Name]; !ok {
			continue
		}
		root := find(feat.Name)
		if at, ok := index[root]; ok {
			groups[at] = append(groups[at], feat.Name)
			continue
		}
		index[root] = len(groups)
		groups = append(groups, []string{feat.Name})
	}
	return groups
}

// redefinitionAliases returns the names under which typ's features read the
// feature called name: name itself and every name redefinition makes one with it.
func (ctx *Context) redefinitionAliases(typ *symbols.Symbol, name string) map[string]bool {
	aliases := map[string]bool{name: true}
	for _, names := range ctx.redefinitionGroups(typ, ctx.FeaturesOf(typ)) {
		for _, n := range names {
			if n != name {
				continue
			}
			for _, alias := range names {
				aliases[alias] = true
			}
			return aliases
		}
	}
	return aliases
}

// sharedRedefinitionName returns the name in a group of names of one feature
// whose feature value the group shares: the first name, in most-specific-first order,
// whose own declaration writes a value, and otherwise the most specific name.
// Two names valued by one declaration are ErrConflictingRedefinition.
func (ctx *Context) sharedRedefinitionName(inst *Instance, byName map[string]*EffectiveFeature, names []string) (string, error) {
	valued := ""
	var valuedBy *symbols.Scope
	for _, name := range names {
		feat, ok := byName[name]
		if !ok || feat.Symbol == nil || ctx.extractDefaultValue(feat.Symbol) == nil {
			continue
		}
		if valued == "" {
			valued, valuedBy = name, feat.Symbol.OwnerScope
			continue
		}
		if feat.Symbol.OwnerScope == valuedBy {
			return "", fmt.Errorf("%w: %s values %s and %s, which redefinition makes one feature",
				ErrConflictingRedefinition, inst.Type.Name, valued, name)
		}
	}
	if valued != "" {
		return valued, nil
	}
	return names[0], nil
}

// redefinedNames returns the names of every feature of owner sym redefines,
// directly or through a redefinition of a redefinition: a restated redefinition
// still declares the feature at the end of the chain (KerML 1.0 §7.3.4.5).
func (ctx *Context) redefinedNames(sym, owner *symbols.Symbol) []string {
	var names []string
	seen := map[*symbols.Symbol]bool{sym: true}
	for queue := []*symbols.Symbol{sym}; len(queue) > 0; {
		cur := queue[0]
		queue = queue[1:]
		// An end of a connector redefines the end at its position of each
		// connector its owner specializes without naming it, so the two names read
		// one feature value as an explicit redefinition's do.
		redefines := append(ctx.relatedFeatures(cur, owner, ast.RelRedefines),
			ctx.model.ImplicitEndRedefinitions(cur)...)
		for _, redefined := range redefines {
			if seen[redefined] {
				continue
			}
			seen[redefined] = true
			names = append(names, redefined.Name)
			queue = append(queue, redefined)
		}
	}
	return names
}

// SubsettingFeatures returns the features of typ subsetting the named feature under any
// of its redefinition names, in declaration order, reading nothing; inst is nil for a type alone.
func (ctx *Context) SubsettingFeatures(inst *Instance, typ *symbols.Symbol, name string) []EffectiveFeature {
	aliases := ctx.redefinitionAliases(typ, name)
	var subsetting []EffectiveFeature
	for _, feat := range ctx.FeaturesOf(typ) {
		if aliases[feat.Name] || feat.Symbol == nil {
			continue
		}
		if inst != nil {
			if _, ok := inst.FeatureValues[feat.Name]; !ok {
				continue
			}
		}
		for _, subsetted := range ctx.relatedFeatureNames(feat.Symbol, typ, ast.RelSubsets) {
			if aliases[subsetted] {
				subsetting = append(subsetting, feat)
				break
			}
		}
	}
	return subsetting
}

// subsettingContributions returns the values the features subsetting the named
// feature contribute to it, in declaration order. A redefinition shares one feature value
// under two names, so membership is decided by the feature value a subsetted name reads
// rather than by the name this collection was read under. Reading a subsetting
// feature materializes it, so a cycle between subsetting features is reported as
// ErrCyclicFeatureValue rather than recursing until the step budget runs out.
func (ctx *Context) subsettingContributions(inst *Instance, name string) ([]Value, error) {
	key := featureValueRef{instance: inst.ID, feature: name}
	if ctx.collectingSubsets[key] {
		return nil, fmt.Errorf("%w: %s.%s subsets itself", ErrCyclicFeatureValue, inst.Type.Name, name)
	}
	ctx.collectingSubsets[key] = true
	defer delete(ctx.collectingSubsets, key)

	var values []Value
	for _, feat := range ctx.SubsettingFeatures(inst, inst.Type, name) {
		sub, err := inst.GetFeatureValue(ctx, feat.Name)
		if err != nil {
			return nil, fmt.Errorf("subsetting feature %s of %s: %w", feat.Name, name, err)
		}
		values = append(values, elementsOf(sub.HeldValue())...)
	}
	if err := ctx.chargeElements(int64(len(values))); err != nil {
		return nil, err
	}
	return values, nil
}
