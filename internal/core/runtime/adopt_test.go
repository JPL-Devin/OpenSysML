package runtime

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/source"
)

const adoptSrc = `package Demo {
	part def Engine { attribute power = 300.0; }
	part def Vehicle { attribute mass = 1500.0; part engine : Engine; }
}`

// contextOver indexes src and gives the context its text, which is what lets a
// shape be compared by what a declaration says rather than by where it sits.
func contextOver(t *testing.T, src string) *Context {
	t.Helper()
	ctx, _ := contextForSource(t, src)
	ctx.RegisterSource(source.New("<test>", []byte(src)))
	return ctx
}

// vehicleIn materializes a vehicle and the engine part inside it.
func vehicleIn(t *testing.T, ctx *Context) *Instance {
	t.Helper()
	idx := ctx.resolver.Index()
	obj, err := ctx.Instantiate(lookupOne(t, idx, "Demo::Vehicle"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	if _, err := obj.GetSlot(ctx, "engine"); err != nil {
		t.Fatalf("GetSlot(engine): %v", err)
	}
	return obj
}

// An object is carried into a re-analysis of the document it was materialized
// from: it keeps its identity and its values, and everything it points at is the
// declaration the new analysis produced rather than the one it was built against.
func TestAdoptCarriesAnObjectIntoAReanalysis(t *testing.T) {
	prev := contextOver(t, adoptSrc)
	obj := vehicleIn(t, prev)
	shapes := prev.ShapesOf(obj)
	nested, ok := obj.Slots["engine"].Value.Object()
	if !ok {
		t.Fatalf("engine slot holds %v, want an object", obj.Slots["engine"].Value)
	}

	ctx := contextOver(t, adoptSrc+"\npart def Widget;")
	if err := ctx.Adopt(prev, shapes, obj); err != nil {
		t.Fatalf("Adopt: %v", err)
	}

	if got, found := ctx.Instance(obj.ID); !found || got != obj {
		t.Fatalf("Instance(%d) = %v, %v; want the carried object", obj.ID, got, found)
	}
	if _, found := ctx.Instance(nested); !found {
		t.Errorf("the engine object %d it holds was not carried with it", nested)
	}
	if want := lookupOne(t, ctx.resolver.Index(), "Demo::Vehicle"); obj.Type != want {
		t.Error("the object is still of the declaration it was built against")
	}
	feat := obj.Slots["mass"].Feature
	if features := ctx.FeaturesOf(obj.Type); feat != &features[indexOfFeature(t, features, "mass")] {
		t.Error("a slot still fills a feature of the analysis the object was built against")
	}
	if got := obj.Slots["mass"].Value; got.Kind != ValConst || !strings.Contains(fmt.Sprint(got.Const), "1500") {
		t.Errorf("mass = %v, want the value it was materialized with", got)
	}
	// The identities carried over are taken, so the next object gets a new one.
	next, err := ctx.Instantiate(lookupOne(t, ctx.resolver.Index(), "Demo::Engine"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	if next.ID == obj.ID || next.ID == nested {
		t.Errorf("new object took the identity %d of one carried over", next.ID)
	}
}

// A declaration that no longer resolves to the shape an object was materialized
// against cannot hold that object, so it is refused rather than rebound onto
// slots that no longer mean the same thing.
func TestAdoptRefusesAChangedShape(t *testing.T) {
	prev := contextOver(t, adoptSrc)
	obj := vehicleIn(t, prev)
	shapes := prev.ShapesOf(obj)

	ctx := contextOver(t, strings.Replace(adoptSrc, "mass = 1500.0", "mass = 900.0", 1))
	err := ctx.Adopt(prev, shapes, obj)
	if err == nil {
		t.Fatal("Adopt accepted an object of a declaration that changed")
	}
	if _, found := ctx.Instance(obj.ID); found {
		t.Error("the refused object was left in the context")
	}
}

// A change to a declaration an object only depends on invalidates it too: its
// slots hold what that declaration says.
func TestAdoptRefusesAChangedDependency(t *testing.T) {
	prev := contextOver(t, adoptSrc)
	obj := vehicleIn(t, prev)
	shapes := prev.ShapesOf(obj)

	ctx := contextOver(t, strings.Replace(adoptSrc, "power = 300.0", "power = 100.0", 1))
	if err := ctx.Adopt(prev, shapes, obj); err == nil {
		t.Fatal("Adopt accepted an object whose engine declaration changed")
	}
}

// The objects are shared with the context they came from, which a run started
// before the re-analysis still materializes through, so both contexts hand out
// identities from one sequence rather than each from where it stood.
func TestAdoptSharesTheIdentitySequence(t *testing.T) {
	prev := contextOver(t, adoptSrc)
	obj := vehicleIn(t, prev)
	shapes := prev.ShapesOf(obj)

	ctx := contextOver(t, adoptSrc+"\npart def Widget;")
	if err := ctx.Adopt(prev, shapes, obj); err != nil {
		t.Fatalf("Adopt: %v", err)
	}

	handed := map[int64]bool{}
	for _, id := range []int64{ctx.allocateID(), prev.allocateID(), ctx.allocateID()} {
		if _, found := ctx.Instance(id); found {
			t.Errorf("identity %d is handed out again while the object holding it is live", id)
		}
		if handed[id] {
			t.Errorf("identity %d was handed out twice", id)
		}
		handed[id] = true
	}
}

// Changed names the declaration a context resolves differently, which is what a
// caller reports as having invalidated the state it holds.
func TestChangedNamesTheDeclarationThatMoved(t *testing.T) {
	prev := contextOver(t, adoptSrc)
	shapes := prev.ShapesOf(vehicleIn(t, prev))

	if fqn, ok := contextOver(t, adoptSrc+"\npart def Widget;").Changed(shapes); ok {
		t.Errorf("Changed reported %q over an unrelated declaration", fqn)
	}
	// A change to the engine is a change to what a vehicle is, but what moved is
	// the engine, so that is what is named rather than the type holding it.
	ctx := contextOver(t, strings.Replace(adoptSrc, "power = 300.0", "power = 100.0", 1))
	fqn, ok := ctx.Changed(shapes)
	if !ok || fqn != "Demo::Engine" {
		t.Errorf("Changed() = %q, %v; want Demo::Engine", fqn, ok)
	}
}

// indexOfFeature is the position of the named effective feature.
func indexOfFeature(t *testing.T, features []EffectiveFeature, name string) int {
	t.Helper()
	for i := range features {
		if features[i].Name == name {
			return i
		}
	}
	t.Fatalf("no feature %q among %d", name, len(features))
	return 0
}
