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
	mass, err := obj.GetSlot(ctx, "mass")
	if err != nil {
		t.Fatalf("GetSlot(mass): %v", err)
	}
	if got := mass.Value; got.Kind != ValConst || !strings.Contains(fmt.Sprint(got.Const), "1500") {
		t.Errorf("mass = %v, want the value its declaration states", got)
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

const adoptCalcSrc = `package Demo {
	calc def double { in x; return : ScalarValues::Real = x * 2.0; }
	part def Gauge { attribute reading = double(3.0); }
}`

// A slot holding what a value expression states is derived again in the context
// it is carried into, so it reads the declarations that expression names as they
// are now rather than keeping what they said when it was materialized.
func TestAdoptDerivesAValueAgainstTheNewDeclarations(t *testing.T) {
	prev := contextOver(t, adoptCalcSrc)
	obj, err := prev.Instantiate(lookupOne(t, prev.resolver.Index(), "Demo::Gauge"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	if slot, err := obj.GetSlot(prev, "reading"); err != nil {
		t.Fatalf("GetSlot(reading): %v", err)
	} else if got := fmt.Sprint(slot.Value.Const); !strings.Contains(got, "6") {
		t.Fatalf("reading = %s, want 6", got)
	}
	shapes := prev.ShapesOf(obj)

	ctx := contextOver(t, strings.Replace(adoptCalcSrc, "x * 2.0", "x * 3.0", 1))
	if err := ctx.Adopt(prev, shapes, obj); err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	slot, err := obj.GetSlot(ctx, "reading")
	if err != nil {
		t.Fatalf("GetSlot(reading): %v", err)
	}
	if got := fmt.Sprint(slot.Value.Const); !strings.Contains(got, "9") {
		t.Errorf("reading = %s, want 9 from the calc as it is declared now", got)
	}
}

const adoptConnectSrc = `package Demo {
	port def P;
	part def A { port p : P; }
	part def B { port q : P; }
	part def Sys { part a : A; part b : B; connect a.p to b.q; }
}`

// A connector its owner declares no name for is a member no name can be looked
// up again, so it is left to the new context to materialize rather than costing
// the object that owns it.
func TestAdoptCarriesAnObjectOwningAnAnonymousConnector(t *testing.T) {
	prev := contextOver(t, adoptConnectSrc)
	obj, err := prev.Instantiate(lookupOne(t, prev.resolver.Index(), "Demo::Sys"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	if conns, err := obj.OwnedConnectors(prev); err != nil {
		t.Fatalf("OwnedConnectors: %v", err)
	} else if len(conns) != 1 {
		t.Fatalf("the object owns %d anonymous connectors, want 1", len(conns))
	}
	before := obj.anonymous[0]
	shapes := prev.ShapesOf(obj)

	ctx := contextOver(t, adoptConnectSrc+"\npart def Widget;")
	if err := ctx.Adopt(prev, shapes, obj); err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	conns, err := obj.OwnedConnectors(ctx)
	if err != nil {
		t.Fatalf("OwnedConnectors after the carry-over: %v", err)
	}
	if len(conns) != 1 {
		t.Fatalf("the carried object owns %d anonymous connectors, want 1", len(conns))
	}
	if _, found := ctx.Instance(conns[0].ID); !found {
		t.Errorf("the connector object %d is not held by the context that materialized it", conns[0].ID)
	}
	// It is the same connector of the same object, so it is named the same.
	if conns[0].ID != before {
		t.Errorf("the connector object is %d after the carry-over, want the identity %d it had", conns[0].ID, before)
	}
	port := slotInstance(t, ctx, obj, "a", "p")
	if end := conns[0].Ends[0].Value; !holdsObject(end, port.ID) {
		t.Errorf("the connector end holds %v, want the port object %d of the carried object", end, port.ID)
	}
}

// holdsObject reports whether the value is the object of the given identity.
func holdsObject(val Value, id int64) bool {
	got, ok := val.Object()
	return ok && got == id
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
// before the re-analysis still materializes through, so every context that holds
// them hands out identities from one sequence — including the first one, several
// re-analyses later.
func TestAdoptSharesTheIdentitySequence(t *testing.T) {
	first := contextOver(t, adoptSrc)
	obj := vehicleIn(t, first)
	shapes := first.ShapesOf(obj)

	ctx := first
	for _, extra := range []string{"\npart def Widget;", "\npart def Gadget;"} {
		next := contextOver(t, adoptSrc+extra)
		if err := next.Adopt(ctx, shapes, obj); err != nil {
			t.Fatalf("Adopt: %v", err)
		}
		ctx = next
	}

	handed := map[int64]bool{}
	for _, id := range []int64{ctx.allocateID(), first.allocateID(), ctx.allocateID()} {
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
