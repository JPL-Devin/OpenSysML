package runtime

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Shapes records the resolved shape of every type a set of objects was
// materialized against, taken while that resolution is still the current one. It
// is what a later context compares its own resolution against to decide whether
// those objects still mean the same thing.
type Shapes struct {
	digests map[string]string // type FQN → resolved shape
	types   []string          // the FQNs above, in the order they were reached
	unnamed bool              // an object of a type with no qualified name
}

// AdoptError says which object could not be carried over into a new context, and
// why, so the loss is reported rather than silently absorbed.
type AdoptError struct {
	Type   string // qualified name of the object's type, as far as it is known
	Reason string
}

func (e *AdoptError) Error() string {
	if e.Type == "" {
		return "cannot carry the object over: " + e.Reason
	}
	return fmt.Sprintf("cannot carry the object of %s over: %s", e.Type, e.Reason)
}

// ShapesOf records the shapes obj and everything it holds were materialized
// against: the objects reachable through its feature values and connector ends, plus the
// variants its values selected. A connector no name reaches is materialized
// again rather than carried, so it is no part of this.
func (ctx *Context) ShapesOf(obj *Instance) *Shapes {
	shapes := &Shapes{digests: make(map[string]string)}
	ctx.recordShapes(obj, shapes, make(map[int64]bool))
	return shapes
}

// ShapesOfType records the shape of one declaration as this context resolves it,
// which is what state held over a declaration rather than an object — a
// debugging session over an action — is invalidated by a change to.
func (ctx *Context) ShapesOfType(sym *symbols.Symbol) *Shapes {
	shapes := &Shapes{digests: make(map[string]string)}
	ctx.recordShape(sym, shapes)
	return shapes
}

// Changed returns the declaration this context no longer resolves the way shapes
// recorded it, so a caller can name what invalidated the state it took those
// shapes for. They were recorded outwards, so reading them back names the one
// that changed rather than one that only holds it.
func (ctx *Context) Changed(shapes *Shapes) (string, bool) {
	if shapes == nil || ctx.resolver == nil || ctx.resolver.Index() == nil {
		return "", false
	}
	for i := len(shapes.types) - 1; i >= 0; i-- {
		fqn := shapes.types[i]
		if !ctx.resolvesTo(fqn, shapes.digests[fqn]) {
			return fqn, true
		}
	}
	return "", false
}

// resolvesTo reports whether some declaration of the qualified name still has
// the recorded shape.
func (ctx *Context) resolvesTo(fqn, digest string) bool {
	for _, cand := range ctx.resolver.Index().LookupQualified(fqn) {
		if ctx.ShapeDigest(cand) == digest {
			return true
		}
	}
	return false
}

func (ctx *Context) recordShapes(obj *Instance, shapes *Shapes, seen map[int64]bool) {
	if obj == nil || seen[obj.ID] {
		return
	}
	seen[obj.ID] = true
	ctx.recordShape(obj.Type, shapes)
	for _, val := range obj.held() {
		ctx.walkValue(val, func(v Value) {
			if v.Kind == ValVariant {
				ctx.recordShape(v.Variant, shapes)
			}
			if id, ok := v.Object(); ok {
				if held, found := ctx.instances[id]; found {
					ctx.recordShapes(held, shapes, seen)
				}
			}
		})
	}
}

// recordShape records the shape of a declaration state was taken against, or
// notes that there is none to compare it by: a declaration of no name of its own
// is reached by no name in a later resolution, whatever scope it sits in.
func (ctx *Context) recordShape(sym *symbols.Symbol, shapes *Shapes) {
	if sym == nil || sym.Name == "" || ctx.fqnOf(sym) == "" {
		shapes.unnamed = true
		return
	}
	ctx.recordReached(sym, shapes)
}

// recordReached records sym's shape and the shapes of the types its features
// hold: a change to any of them is a change to what an object of sym holds.
func (ctx *Context) recordReached(sym *symbols.Symbol, shapes *Shapes) {
	fqn := ctx.fqnOf(sym)
	if fqn == "" {
		return
	}
	if _, done := shapes.digests[fqn]; done {
		return
	}
	shapes.digests[fqn] = ctx.ShapeDigest(sym)
	shapes.types = append(shapes.types, fqn)
	features := ctx.FeaturesOf(sym)
	for i := range features {
		ctx.recordReached(features[i].Type, shapes)
	}
}

// featureNames lists the object's feature names in a fixed order, so what is done
// over its feature values does not depend on map order.
func (obj *Instance) featureNames() []string {
	names := make([]string, 0, len(obj.FeatureValues))
	for name := range obj.FeatureValues {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// held returns the values the object carries: its feature values and its connector ends.
func (obj *Instance) held() []Value {
	names := obj.featureNames()
	out := make([]Value, 0, 2*len(names)+len(obj.Ends))
	for _, name := range names {
		fv := obj.FeatureValues[name]
		out = append(out, fv.Value, fv.Values)
	}
	for _, end := range obj.Ends {
		out = append(out, end.Value)
	}
	return out
}

// carried returns the values the object keeps across a carry-over: what a value
// expression states is left out, since it is derived again rather than kept.
func (obj *Instance) carried(ctx *Context) []Value {
	names := obj.featureNames()
	out := make([]Value, 0, 2*len(names)+len(obj.Ends))
	for _, name := range names {
		fv := obj.FeatureValues[name]
		if ctx.derivedFeatureValue(fv) || ctx.connectorFeatureValue(fv) {
			continue
		}
		out = append(out, fv.Value, fv.Values)
	}
	for _, end := range obj.Ends {
		out = append(out, end.Value)
	}
	return out
}

// derivedFeatureValue reports whether the feature value holds what a value expression states,
// which a new context computes again from the declarations that expression now
// reads. What a variation's default states is a variant rather than a value, so
// the object bound to it is carried instead of bound again.
func (ctx *Context) derivedFeatureValue(s *FeatureValue) bool {
	if s.Feature == nil || !ctx.valueBinds(s.Feature) {
		return false
	}
	return !ctx.model.IsVariationFeature(s.Feature.Symbol)
}

// collectedFeatureValue reports whether the feature value holds values copied out of the features
// subsetting it, which are read again here since one of them may be derived from
// a declaration that changed. A collection of objects is kept: they are carried.
func (ctx *Context) collectedFeatureValue(s *FeatureValue) bool {
	if !s.Materialized || (s.Values.Kind != ValSequence && s.Values.Kind != ValSet) {
		return false
	}
	object := false
	ctx.walkValue(s.Values, func(v Value) {
		if _, held := v.Object(); held {
			object = true
		}
	})
	return !object
}

// connectorFeatureValue reports whether the feature value holds the object of a connector, whose
// ends a new context attaches again rather than keeping what they read before.
func (ctx *Context) connectorFeatureValue(s *FeatureValue) bool {
	return s.Feature != nil && ctx.model.IsConnectorUsage(s.Feature.Symbol)
}

// walkValue visits a value and everything nested in it.
func (ctx *Context) walkValue(val Value, visit func(Value)) {
	visit(val)
	switch val.Kind {
	case ValSequence:
		if val.Sequence != nil {
			for _, elem := range val.Sequence.Elements() {
				ctx.walkValue(elem, visit)
			}
		}
	case ValSet:
		if val.Set != nil {
			for _, elem := range val.Set.Elements() {
				ctx.walkValue(elem, visit)
			}
		}
	}
}

// ShapeDigest renders what instantiating a type produces as this context
// resolves it now: the features an object of it gets, with the type,
// multiplicity and default of each, and the shapes of the types those features
// hold. Two contexts that agree on the digest agree on the object.
func (ctx *Context) ShapeDigest(sym *symbols.Symbol) string {
	var b strings.Builder
	ctx.writeShape(&b, sym, make(map[string]bool))
	return b.String()
}

func (ctx *Context) writeShape(b *strings.Builder, sym *symbols.Symbol, open map[string]bool) {
	if sym == nil {
		b.WriteString("<untyped>")
		return
	}
	fqn := ctx.fqnOf(sym)
	if fqn == "" {
		fqn = "<unnamed>"
	}
	fmt.Fprintf(b, "%s/%s", fqn, sym.Kind)
	// A type reached through its own features is named rather than expanded
	// again, so a recursive shape has a finite digest.
	if open[fqn] {
		b.WriteString("…")
		return
	}
	open[fqn] = true
	defer delete(open, fqn)
	b.WriteString("{")
	features := ctx.FeaturesOf(sym)
	for i := range features {
		feat := &features[i]
		fmt.Fprintf(b, "%s:%s..%s", feat.Name, bound(feat.Multiplicity.Lower), bound(feat.Multiplicity.Upper))
		if ctx.model.IsVariationFeature(feat.Symbol) {
			b.WriteString("|variation")
		}
		if feat.DefaultValue != nil {
			owner := feat.DefaultDecl
			if owner == nil {
				owner = feat.Symbol
			}
			fmt.Fprintf(b, "=%s", ctx.declText(owner, feat.DefaultValue.Span()))
		}
		// A body governing over an inherited value is what materializing reads,
		// so the shape follows edits confined to that body.
		if feat.DefaultValue != nil && ctx.bodyGovernsInheritedValue(feat) && feat.Symbol.Decl != nil {
			fmt.Fprintf(b, "|body:%s", ctx.declText(feat.Symbol, feat.Symbol.Decl.Span()))
		}
		b.WriteString("@")
		ctx.writeShape(b, feat.Type, open)
		b.WriteString(";")
	}
	b.WriteString("}")
}

func bound(b semantics.Bound) string {
	switch {
	case !b.Known:
		return "?"
	case b.Infinite:
		return "*"
	default:
		return fmt.Sprint(b.Value)
	}
}

// declText renders the text a declaration wrote at the given span, falling back
// to the span itself for a file whose text this context was not given — an
// unchanged library file, where the span is as stable as the text.
func (ctx *Context) declText(owner *symbols.Symbol, span source.Span) string {
	file := ""
	if owner != nil {
		file = owner.DocName
	}
	if sf, ok := ctx.sources[file]; ok && span.End() <= sf.Len() {
		return strings.Join(strings.Fields(sf.Text(span)), " ")
	}
	return fmt.Sprintf("%s#%d+%d", file, span.Offset, span.Len)
}

func (ctx *Context) fqnOf(sym *symbols.Symbol) string {
	if sym == nil || ctx.resolver == nil {
		return ""
	}
	idx := ctx.resolver.Index()
	if idx == nil {
		return ""
	}
	return idx.GetFQN(sym)
}

// Adopt takes obj — and every object it holds — over from prev into this
// context, keeping the identity and the values they carry across a re-analysis
// of the document they were materialized from. Each object is rebound to the
// declaration of the same qualified name here, which must resolve to the shape
// recorded in shapes; anything else is refused with the context left untouched.
// The objects are moved rather than copied, so prev holds them too afterwards
// and this context takes over its identity sequence; nothing new should be
// materialized through prev, which registers it there alone.
func (ctx *Context) Adopt(prev *Context, shapes *Shapes, obj *Instance) error {
	if prev == nil || shapes == nil || obj == nil {
		return &AdoptError{Reason: "there is nothing to carry over"}
	}
	if shapes.unnamed {
		return &AdoptError{Reason: "it is of a declaration with no qualified name"}
	}
	a := &adoption{ctx: ctx, prev: prev, shapes: shapes,
		plans:   make(map[int64]*adoptPlan),
		rebound: make(map[*symbols.Symbol]*symbols.Symbol),
	}
	if err := a.plan(obj); err != nil {
		return err
	}
	a.commit()
	return nil
}

// adoption is one carry-over: what it has planned, and the symbols it rebound
// while planning, applied only once the whole closure has been accepted.
type adoption struct {
	ctx     *Context
	prev    *Context
	shapes  *Shapes
	plans   map[int64]*adoptPlan
	rebound map[*symbols.Symbol]*symbols.Symbol
}

// adoptPlan is what one object becomes in the new context: the declaration it is
// of, and the feature each of its feature values fills.
type adoptPlan struct {
	obj      *Instance
	typeSym  *symbols.Symbol
	features map[string]*EffectiveFeature
}

// featureFor returns the feature a feature value reached under name fills: the one its
// own name gives, which for a feature value shared by several names of one feature is the
// name it was created under.
func (p *adoptPlan) featureFor(name string, fv *FeatureValue) *EffectiveFeature {
	if fv.Feature != nil {
		if feat, ok := p.features[fv.Feature.Name]; ok {
			return feat
		}
	}
	return p.features[name]
}

func (a *adoption) plan(obj *Instance) error {
	if obj == nil {
		return &AdoptError{Reason: "there is nothing to carry over"}
	}
	if _, planned := a.plans[obj.ID]; planned {
		return nil
	}
	// An object already in this context was carried over with another root; a
	// different object holding its ID is a collision that must not be papered
	// over by overwriting either of them.
	if existing, ok := a.ctx.instances[obj.ID]; ok {
		if existing == obj {
			return nil
		}
		return &AdoptError{Type: a.ctx.fqnOf(obj.Type), Reason: fmt.Sprintf("its identity %d is taken", obj.ID)}
	}
	typeSym, err := a.rebind(obj.Type, "its type")
	if err != nil {
		return err
	}
	fqn := a.ctx.fqnOf(typeSym)
	want, recorded := a.shapes.digests[fqn]
	if !recorded {
		return &AdoptError{Type: fqn, Reason: "the shape it was materialized against was not recorded"}
	}
	if got := a.ctx.ShapeDigest(typeSym); got != want {
		return &AdoptError{Type: fqn, Reason: "its declaration resolves to a different shape now"}
	}
	features := a.ctx.FeaturesOf(typeSym)
	byName := make(map[string]*EffectiveFeature, len(features))
	for i := range features {
		byName[features[i].Name] = &features[i]
	}
	plan := &adoptPlan{obj: obj, typeSym: typeSym, features: make(map[string]*EffectiveFeature, len(obj.FeatureValues))}
	for _, name := range obj.featureNames() {
		feat, err := a.planFeature(fqn, typeSym, name, obj.FeatureValues[name], byName)
		if err != nil {
			return err
		}
		plan.features[name] = feat
	}
	a.plans[obj.ID] = plan
	for _, val := range obj.carried(a.prev) {
		if err := a.planValue(fqn, val); err != nil {
			return err
		}
	}
	return nil
}

// planFeature is the feature a feature value fills in this context: the effective feature
// of the rebound declaration, or — for a feature value a connector added, which no
// declaration of the type carries — the recorded one with its symbols rebound.
func (a *adoption) planFeature(owner string, typeSym *symbols.Symbol, name string, fv *FeatureValue, byName map[string]*EffectiveFeature) (*EffectiveFeature, error) {
	if feat, ok := byName[name]; ok {
		return feat, nil
	}
	if fv.Feature == nil || fv.Feature.DefaultValue != nil {
		return nil, &AdoptError{Type: owner, Reason: fmt.Sprintf("it no longer has a feature %q", name)}
	}
	feat := *fv.Feature
	feat.OwnerType = typeSym
	for _, ref := range []**symbols.Symbol{&feat.Symbol, &feat.Type} {
		if *ref == nil {
			continue
		}
		found, err := a.rebind(*ref, fmt.Sprintf("the feature %q", name))
		if err != nil {
			return nil, err
		}
		*ref = found
	}
	return &feat, nil
}

func (a *adoption) planValue(owner string, val Value) error {
	var err error
	a.prev.walkValue(val, func(v Value) {
		if err != nil {
			return
		}
		// An unevaluated expression reads names in the document that just
		// changed, so it cannot be carried over as the value it stands for.
		if v.Kind == ValExpr {
			err = &AdoptError{Type: owner, Reason: "it holds an expression that was never evaluated"}
			return
		}
		if v.Kind == ValVariant {
			if _, rebindErr := a.rebind(v.Variant, "a variant it selected"); rebindErr != nil {
				err = rebindErr
				return
			}
		}
		if id, ok := v.Object(); ok {
			err = a.planHeld(owner, id)
		}
	})
	return err
}

func (a *adoption) planHeld(owner string, id int64) error {
	held, ok := a.prev.instances[id]
	if !ok {
		return &AdoptError{Type: owner, Reason: fmt.Sprintf("the object %d it holds is gone", id)}
	}
	return a.plan(held)
}

// rebind maps a symbol of the previous context to the one declaration of the
// same qualified name and kind here, and records it for the values that name it.
func (a *adoption) rebind(sym *symbols.Symbol, what string) (*symbols.Symbol, error) {
	if sym == nil {
		return nil, &AdoptError{Reason: what + " was never resolved"}
	}
	if found, ok := a.rebound[sym]; ok {
		return found, nil
	}
	fqn := a.prev.fqnOf(sym)
	if fqn == "" {
		fqn = a.ctx.fqnOf(sym)
	}
	if fqn == "" {
		return nil, &AdoptError{Reason: what + " has no qualified name"}
	}
	idx := a.ctx.resolver.Index()
	var found *symbols.Symbol
	for _, cand := range idx.LookupQualified(fqn) {
		if cand.Kind != sym.Kind {
			continue
		}
		if found != nil {
			return nil, &AdoptError{Type: fqn, Reason: what + " is now declared more than once"}
		}
		found = cand
	}
	if found == nil {
		return nil, &AdoptError{Type: fqn, Reason: what + " is no longer declared"}
	}
	a.rebound[sym] = found
	return found, nil
}

// AdoptIdentities takes over the identity sequence of a context this one
// replaces, without carrying any object over: objects a run started before still
// materializes through it, so neither context may hand out the other's.
func (ctx *Context) AdoptIdentities(prev *Context) {
	if prev == nil || prev == ctx || prev.ids == nil || prev.ids == ctx.ids {
		return
	}
	prev.ids.atLeast(ctx.ids.next)
	ctx.ids = prev.ids
}

// commit moves the planned objects into this context, rebinding what each of
// them points at and taking over the derived state that is about them.
func (a *adoption) commit() {
	a.ctx.AdoptIdentities(a.prev)
	adopted := make(map[int64]bool, len(a.plans))
	for id, plan := range a.plans {
		adopted[id] = true
		plan.obj.Type = plan.typeSym
		// Names of one redefined feature share a feature value, which is rebound once, to
		// the feature of the name the shared feature value was created under.
		done := make(map[*FeatureValue]bool, len(plan.obj.FeatureValues))
		for _, name := range plan.obj.featureNames() {
			fv := plan.obj.FeatureValues[name]
			if done[fv] {
				continue
			}
			done[fv] = true
			fv.Feature = plan.featureFor(name, fv)
			// A value an expression states is derived again here, so it cannot go
			// stale against what that expression now reads.
			if a.ctx.derivedFeatureValue(fv) {
				fv.Value, fv.Values, fv.Materialized = Value{}, Value{}, false
				continue
			}
			if a.ctx.collectedFeatureValue(fv) {
				fv.Value, fv.Values, fv.Materialized = Value{}, Value{}, false
				continue
			}
			// A connector reads the features the `connect` clause names, which are
			// read again here — under the identity its object had, which names the
			// same connector.
			if a.ctx.connectorFeatureValue(fv) {
				if id, held := fv.Value.Object(); held {
					plan.obj.keepConnector(fv, id)
				}
				fv.Value, fv.Values, fv.Materialized = Value{}, Value{}, false
				continue
			}
			fv.Value = a.rewrite(fv.Value)
			fv.Values = a.rewrite(fv.Values)
		}
		for i := range plan.obj.Ends {
			plan.obj.Ends[i].Value = a.rewrite(plan.obj.Ends[i].Value)
		}
		// The connectors the owner names no name are reached by no name here, so
		// they are materialized again against the declarations as they are now —
		// under the identities they had, which name the same connectors.
		if plan.obj.anonymous != nil {
			plan.obj.keptAnonymous, plan.obj.anonymous = plan.obj.anonymous, nil
		}
		a.ctx.registerInstance(plan.obj)
		a.ctx.ids.atLeast(id + 1)
	}
	a.carryDerived(adopted)
}

// carryDerived takes over the state the previous context derived about the
// objects carried over: which usage denotes which occurrence, and which variant
// each of them selected. What a rebound declaration no longer has is dropped, so
// it is derived again rather than kept wrong.
func (a *adoption) carryDerived(adopted map[int64]bool) {
	for sym, id := range a.prev.occurrences {
		if !adopted[id] {
			continue
		}
		if found, ok := a.rebound[sym]; ok {
			a.ctx.occurrences[found] = id
			continue
		}
		if found, err := a.rebind(sym, "a usage of it"); err == nil {
			a.ctx.occurrences[found] = id
		}
	}
	for key, id := range a.prev.variantObjects {
		if !adopted[key.owner] || !adopted[id] {
			continue
		}
		variation, err := a.rebind(key.variation, "a variation of it")
		if err != nil {
			continue
		}
		variant, err := a.rebind(key.variant, "a variant of it")
		if err != nil {
			continue
		}
		a.ctx.variantObjects[variantObject{owner: key.owner, variation: variation, variant: variant}] = id
	}
	for key, variant := range a.prev.selectedVariants {
		if adopted[key.owner] {
			a.ctx.selectedVariants[key] = variant
		}
	}
}

// rewrite returns the value as this context holds it: the same value with every
// symbol it names rebound. Collections are rebuilt rather than edited, since a
// set is keyed on the values it holds.
func (a *adoption) rewrite(val Value) Value {
	switch val.Kind {
	case ValVariant:
		if found, ok := a.rebound[val.Variant]; ok {
			val.Variant = found
		}
		return val
	case ValSequence:
		if val.Sequence == nil {
			return val
		}
		seq := NewSequence()
		for _, elem := range val.Sequence.Elements() {
			seq.Append(a.rewrite(elem))
		}
		val.Sequence = seq
		return val
	case ValSet:
		if val.Set == nil {
			return val
		}
		set := NewSet()
		for _, elem := range val.Set.Elements() {
			set.Add(a.rewrite(elem))
		}
		val.Set = set
		return val
	default:
		return val
	}
}
