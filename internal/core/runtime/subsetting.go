package runtime

import (
	"fmt"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
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
// the two names read one slot rather than two — `part subsystems :>> Subsystems`
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
			if ctx.isFeatureOf(owner, resolved) {
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
// declaration or through what it inherits: a declaration restating a feature
// (`attribute :>> own`) masks the feature it redefines under that name, which is
// still a feature of the owner.
func (ctx *Context) isFeatureOf(owner, feature *symbols.Symbol) bool {
	if member, ok := ctx.model.LookupMember(owner, feature.Name); ok && member == feature {
		return true
	}
	member, ok := ctx.model.LookupContributedMember(owner, feature.Name)
	return ok && member == feature
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

// aliasRedefinedSlots makes every feature that redefines another feature of this
// instance under a different name share that feature's slot: a redefinition
// declares the same feature again, so both names name one set of values.
func (ctx *Context) aliasRedefinedSlots(inst *Instance) {
	for _, feat := range ctx.FeaturesOf(inst.Type) {
		slot, ok := inst.Slots[feat.Name]
		if !ok || feat.Symbol == nil {
			continue
		}
		for _, redefined := range ctx.redefinedNames(feat.Symbol, inst.Type) {
			if redefined == feat.Name {
				continue
			}
			if _, ok := inst.Slots[redefined]; ok {
				inst.Slots[redefined] = slot
			}
		}
	}
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
		for _, redefined := range ctx.relatedFeatures(cur, owner, ast.RelRedefines) {
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

// subsettingContributions returns the values the features subsetting the named
// feature contribute to it, in declaration order. A redefinition shares one slot
// under two names, so membership is decided by the slot a subsetted name reads
// rather than by the name this collection was read under. Reading a subsetting
// feature materializes it, so a cycle between subsetting features is reported as
// ErrCyclicSlot rather than recursing until the step budget runs out.
func (ctx *Context) subsettingContributions(inst *Instance, name string) ([]Value, error) {
	target := inst.Slots[name]
	key := slotRef{instance: inst.ID, feature: name}
	if ctx.collectingSubsets[key] {
		return nil, fmt.Errorf("%w: %s.%s subsets itself", ErrCyclicSlot, inst.Type.Name, name)
	}
	ctx.collectingSubsets[key] = true
	defer delete(ctx.collectingSubsets, key)

	var values []Value
	for _, feat := range ctx.FeaturesOf(inst.Type) {
		if feat.Name == name || feat.Symbol == nil {
			continue
		}
		subsets := false
		for _, subsetted := range ctx.relatedFeatureNames(feat.Symbol, inst.Type, ast.RelSubsets) {
			if subsetted == name || (target != nil && inst.Slots[subsetted] == target) {
				subsets = true
				break
			}
		}
		if !subsets {
			continue
		}
		slot, ok := inst.Slots[feat.Name]
		if !ok || slot == inst.Slots[name] {
			continue
		}
		sub, err := inst.GetSlot(ctx, feat.Name)
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
