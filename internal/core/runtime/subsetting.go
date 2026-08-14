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

// relatedFeatureNames returns the names of the features of owner that sym's
// relationships of the given kind name. A relationship written inside a usage
// names a feature of the object being materialized, which the scope the usage
// was written in need not see — `part a : Sub :> subsystem` subsets a feature
// inherited by a's owner — so the name is looked up among owner's members when
// the scope it was written in resolves nothing.
func (ctx *Context) relatedFeatureNames(sym, owner *symbols.Symbol, kind ast.RelationshipKind) []string {
	var names []string
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
			names = append(names, resolved.Name)
			continue
		}
		last := qn.Parts[len(qn.Parts)-1].Text
		if member, found := ctx.model.LookupMember(owner, last); found && member != nil && member != sym {
			names = append(names, member.Name)
		}
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
		for _, redefined := range ctx.relatedFeatureNames(feat.Symbol, inst.Type, ast.RelRedefines) {
			if redefined == feat.Name {
				continue
			}
			if _, ok := inst.Slots[redefined]; ok {
				inst.Slots[redefined] = slot
			}
		}
	}
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
