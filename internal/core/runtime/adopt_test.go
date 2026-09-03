package runtime

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
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
	if _, err := obj.GetFeatureValue(ctx, "engine"); err != nil {
		t.Fatalf("GetFeatureValue(engine): %v", err)
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
	nested, ok := obj.FeatureValues["engine"].Value.Object()
	if !ok {
		t.Fatalf("engine feature value holds %v, want an object", obj.FeatureValues["engine"].Value)
	}

	ctx := contextOver(t, adoptSrc+"\npart def Widget;")
	if _, err := ctx.Adopt(prev, shapes, obj); err != nil {
		t.Fatalf("Adopt: %v", err)
	}

	if got, found := ctx.Instance(obj.ID); !found || got != obj {
		t.Fatalf("Instance(%d) = %v, %v; want the carried object", obj.ID, got, found)
	}
	if _, found := ctx.Instance(nested); !found {
		t.Errorf("the engine object %d it holds was not carried with it", nested)
	}
	if obj.Type != lookupOne(t, ctx.resolver.Index(), "Demo::Vehicle") {
		t.Error("the object is still of the declaration it was built against")
	}
	feat := obj.FeatureValues["mass"].Feature
	if features := ctx.FeaturesOf(obj.Type); feat != &features[indexOfFeature(t, features, "mass")] {
		t.Error("a feature value still fills a feature of the analysis the object was built against")
	}
	mass, err := obj.GetFeatureValue(ctx, "mass")
	if err != nil {
		t.Fatalf("GetFeatureValue(mass): %v", err)
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

// A feature value that holds what a value expression states is derived again in the context
// it is carried into, so it reads the declarations that expression names as they
// are now rather than keeping what they said when it was materialized.
func TestAdoptDerivesAValueAgainstTheNewDeclarations(t *testing.T) {
	prev := contextOver(t, adoptCalcSrc)
	obj, err := prev.Instantiate(lookupOne(t, prev.resolver.Index(), "Demo::Gauge"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	if fv, err := obj.GetFeatureValue(prev, "reading"); err != nil {
		t.Fatalf("GetFeatureValue(reading): %v", err)
	} else if got := fmt.Sprint(fv.Value.Const); !strings.Contains(got, "6") {
		t.Fatalf("reading = %s, want 6", got)
	}
	shapes := prev.ShapesOf(obj)

	ctx := contextOver(t, strings.Replace(adoptCalcSrc, "x * 2.0", "x * 3.0", 1))
	if _, err := ctx.Adopt(prev, shapes, obj); err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	fv, err := obj.GetFeatureValue(ctx, "reading")
	if err != nil {
		t.Fatalf("GetFeatureValue(reading): %v", err)
	}
	if got := fmt.Sprint(fv.Value.Const); !strings.Contains(got, "9") {
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
	if _, err := ctx.Adopt(prev, shapes, obj); err != nil {
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
	port := fvInstance(t, ctx, obj, "a", "p")
	if end := conns[0].Ends[0].Value; !holdsObject(end, port.ID) {
		t.Errorf("the connector end holds %v, want the port object %d of the carried object", end, port.ID)
	}
}

// The identities of the connectors a carry-over set aside are known without
// materializing them, and asking for one materializes it again under its identity.
func TestAdoptKeepsTheIdentitiesOfConnectorsSetAside(t *testing.T) {
	src := strings.Replace(adoptConnectSrc, "connect a.p to b.q;", "connect a.p to b.q; connection c connect a.p to b.q;", 1)
	prev := contextOver(t, src)
	obj, err := prev.Instantiate(lookupOne(t, prev.resolver.Index(), "Demo::Sys"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	if got := obj.KeptConnectorIDs(); len(got) != 0 {
		t.Fatalf("KeptConnectorIDs() = %v before any carry-over, want none", got)
	}
	conns, err := obj.OwnedConnectors(prev)
	if err != nil {
		t.Fatalf("OwnedConnectors: %v", err)
	}
	named := fvInstance(t, prev, obj, "c")
	anon := conns[0].ID
	shapes := prev.ShapesOf(obj)

	ctx := contextOver(t, src+"\npart def Widget;")
	if _, err := ctx.Adopt(prev, shapes, obj); err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	want := []int64{min(anon, named.ID), max(anon, named.ID)}
	if got := obj.KeptConnectorIDs(); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("KeptConnectorIDs() = %v after the carry-over, want %v", got, want)
	}
	if got := obj.MaterializedConnectors(ctx); len(got) != 0 {
		t.Errorf("MaterializedConnectors() = %v, want none until they are asked for", got)
	}
	held := len(ctx.InstanceIDs())
	if got, err := obj.RestoreConnector(ctx, 99); got != nil || err != nil {
		t.Errorf("RestoreConnector(99) = %v, %v; want nothing for an identity never kept", got, err)
	}
	if len(ctx.InstanceIDs()) != held {
		t.Fatalf("asking after the identities materialized objects: %v", ctx.InstanceIDs())
	}

	for _, id := range []int64{anon, named.ID} {
		conn, err := obj.RestoreConnector(ctx, id)
		if err != nil {
			t.Fatalf("RestoreConnector(%d): %v", id, err)
		}
		if conn == nil || conn.ID != id {
			t.Fatalf("RestoreConnector(%d) = %v, want the connector under that identity", id, conn)
		}
		if got, found := ctx.Instance(id); !found || got != conn {
			t.Errorf("Instance(%d) = %v, %v; want the connector materialized again", id, got, found)
		}
		port := fvInstance(t, ctx, obj, "a", "p")
		if end := conn.Ends[0].Value; !holdsObject(end, port.ID) {
			t.Errorf("connector %d's end holds %v, want the port object %d", id, end, port.ID)
		}
	}
	if got := obj.KeptConnectorIDs(); len(got) != 0 {
		t.Errorf("KeptConnectorIDs() = %v once both are materialized again, want none", got)
	}
	if got := obj.MaterializedConnectors(ctx); len(got) != 1 || got[0].ID != anon {
		t.Errorf("MaterializedConnectors() = %v, want the anonymous connector %d", got, anon)
	}
	if len(ctx.InstanceIDs()) != held+2 {
		t.Errorf("materializing the two connectors again left %v, want the two objects more", ctx.InstanceIDs())
	}
}

// carriedSysWithThreeConnectors materializes a Sys owning three anonymous
// connectors, carries it into a re-analysis, and returns the carried object with
// the identities its connectors had, in declaration order.
func carriedSysWithThreeConnectors(t *testing.T) (*Context, *Instance, []int64) {
	t.Helper()
	src := strings.Replace(adoptConnectSrc, "connect a.p to b.q;", "connect a.p to b.q; connect a.p to b.q; connect a.p to b.q;", 1)
	prev := contextOver(t, src)
	obj, err := prev.Instantiate(lookupOne(t, prev.resolver.Index(), "Demo::Sys"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	conns, err := obj.OwnedConnectors(prev)
	if err != nil {
		t.Fatalf("OwnedConnectors: %v", err)
	}
	if len(conns) != 3 {
		t.Fatalf("the object owns %d anonymous connectors, want 3", len(conns))
	}
	ids := make([]int64, 0, 3)
	for _, conn := range conns {
		ids = append(ids, conn.ID)
	}
	shapes := prev.ShapesOf(obj)
	ctx := contextOver(t, src+"\npart def Widget;")
	if _, err := ctx.Adopt(prev, shapes, obj); err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	return ctx, obj, ids
}

// restoreCost measures the steps restoring one of those connectors alone spends.
func restoreCost(t *testing.T) int64 {
	t.Helper()
	ctx, obj, ids := carriedSysWithThreeConnectors(t)
	spent := ctx.steps
	if _, err := obj.RestoreConnector(ctx, ids[2]); err != nil {
		t.Fatalf("RestoreConnector(%d): %v", ids[2], err)
	}
	return ctx.steps - spent
}

// Asking for one connector a carry-over set aside materializes that connector
// alone: its siblings stay set aside, under their identities, until each is asked
// for — so a budget that admits the one asked for does not have to admit them all.
func TestRestoreConnectorMaterializesTheOneAskedForAlone(t *testing.T) {
	ctx, obj, ids := carriedSysWithThreeConnectors(t)
	held := len(ctx.InstanceIDs())
	last, err := obj.RestoreConnector(ctx, ids[2])
	if err != nil {
		t.Fatalf("RestoreConnector(%d): %v", ids[2], err)
	}
	if last == nil || last.ID != ids[2] {
		t.Fatalf("RestoreConnector(%d) = %v, want the connector under that identity", ids[2], last)
	}
	// The ends read objects carried with the owner, so the connector is all that is new.
	if got := len(ctx.InstanceIDs()); got != held+1 {
		t.Errorf("restoring one connector left %d objects, want %d: it alone", got, held+1)
	}
	for _, id := range ids[:2] {
		if _, found := ctx.Instance(id); found {
			t.Errorf("restoring %d materialized its sibling %d", ids[2], id)
		}
	}
	if got := obj.KeptConnectorIDs(); fmt.Sprint(got) != fmt.Sprint(ids[:2]) {
		t.Errorf("KeptConnectorIDs() = %v after restoring %d, want its siblings %v still set aside", got, ids[2], ids[:2])
	}
	if got := obj.MaterializedConnectors(ctx); len(got) != 1 || got[0].ID != ids[2] {
		t.Errorf("MaterializedConnectors() = %v, want the one restored, %d", got, ids[2])
	}

	// A budget admitting exactly the one asked for admits it, whatever its siblings would cost.
	cost := restoreCost(t)
	ctx, obj, ids = carriedSysWithThreeConnectors(t)
	ctx.maxSteps = ctx.steps + cost
	if conn, err := obj.RestoreConnector(ctx, ids[2]); err != nil || conn == nil || conn.ID != ids[2] {
		t.Fatalf("RestoreConnector(%d) under a budget admitting it alone = %v, %v; want the connector", ids[2], conn, err)
	}
	if got := obj.KeptConnectorIDs(); fmt.Sprint(got) != fmt.Sprint(ids[:2]) {
		t.Errorf("KeptConnectorIDs() = %v, want the siblings %v still set aside", got, ids[:2])
	}
	// The siblings take their identities back when asked for, in either order.
	ctx.maxSteps = ctx.steps + 2*cost
	for _, id := range []int64{ids[0], ids[1]} {
		conn, err := obj.RestoreConnector(ctx, id)
		if err != nil || conn == nil || conn.ID != id {
			t.Fatalf("RestoreConnector(%d) = %v, %v; want the connector under that identity", id, conn, err)
		}
	}
	if got := obj.KeptConnectorIDs(); len(got) != 0 {
		t.Errorf("KeptConnectorIDs() = %v once all are materialized again, want none", got)
	}
	conns, err := obj.OwnedConnectors(ctx)
	if err != nil {
		t.Fatalf("OwnedConnectors: %v", err)
	}
	var got []int64
	for _, conn := range conns {
		got = append(got, conn.ID)
	}
	if fmt.Sprint(got) != fmt.Sprint(ids) {
		t.Errorf("OwnedConnectors() = %v, want the connectors under their identities %v in declaration order", got, ids)
	}
}

// A probe that restores one set-aside connector discards it with the probe and
// leaves the object as it found it: every identity kept, the one restored
// included, and what was materialized before still so.
func TestProbedRestorationLeavesTheIdentitiesKept(t *testing.T) {
	ctx, obj, ids := carriedSysWithThreeConnectors(t)
	if first, err := obj.RestoreConnector(ctx, ids[0]); err != nil || first == nil || first.ID != ids[0] {
		t.Fatalf("RestoreConnector(%d) = %v, %v; want the connector", ids[0], first, err)
	}
	held := len(ctx.InstanceIDs())

	end := ctx.beginProbe()
	mark := len(ctx.created)
	if last, err := obj.RestoreConnector(ctx, ids[2]); err != nil || last == nil || last.ID != ids[2] {
		t.Fatalf("RestoreConnector(%d) under a probe = %v, %v; want the connector under that identity", ids[2], last, err)
	}
	ctx.abandonInstancesSince(mark)
	end()

	if got := obj.KeptConnectorIDs(); fmt.Sprint(got) != fmt.Sprint(ids[1:]) {
		t.Errorf("KeptConnectorIDs() after the probe = %v, want %v set aside as before it", got, ids[1:])
	}
	if got := obj.MaterializedConnectors(ctx); len(got) != 1 || got[0].ID != ids[0] {
		t.Errorf("MaterializedConnectors() after the probe = %v, want the one restored before it, %d", got, ids[0])
	}
	if got := len(ctx.InstanceIDs()); got != held {
		t.Errorf("the probe left %d objects, want %d as before it", got, held)
	}
	if last, err := obj.RestoreConnector(ctx, ids[2]); err != nil || last == nil || last.ID != ids[2] {
		t.Errorf("RestoreConnector(%d) after the probe = %v, %v; want the connector under that identity", ids[2], last, err)
	}
	if got := obj.KeptConnectorIDs(); fmt.Sprint(got) != fmt.Sprint(ids[1:2]) {
		t.Errorf("KeptConnectorIDs() = %v, want the sibling %v alone still set aside", got, ids[1:2])
	}
}

// A restoration that fails leaves every identity set aside, the one asked for
// included, and no object behind, so asking again under a budget that admits it
// materializes the connector under its identity.
func TestRestoreConnectorThatFailsKeepsEveryIdentity(t *testing.T) {
	cost := restoreCost(t)
	ctx, obj, ids := carriedSysWithThreeConnectors(t)
	held := ctx.InstanceIDs()
	ctx.maxSteps = ctx.steps + cost - 1
	conn, err := obj.RestoreConnector(ctx, ids[1])
	if !errors.Is(err, ErrStepLimitExceeded) {
		t.Fatalf("RestoreConnector(%d) one step short = %v, %v; want the budget spent", ids[1], conn, err)
	}
	if got := obj.KeptConnectorIDs(); fmt.Sprint(got) != fmt.Sprint(ids) {
		t.Errorf("KeptConnectorIDs() = %v after the failure, want all of %v still set aside", got, ids)
	}
	if got := obj.MaterializedConnectors(ctx); len(got) != 0 {
		t.Errorf("MaterializedConnectors() = %v after the failure, want none", got)
	}
	for _, id := range ids {
		if _, found := ctx.Instance(id); found {
			t.Errorf("the failed restoration left connector %d behind", id)
		}
	}

	ctx.maxSteps = ctx.Budgets().MaxSteps + 1000
	conn, err = obj.RestoreConnector(ctx, ids[1])
	if err != nil || conn == nil || conn.ID != ids[1] {
		t.Fatalf("RestoreConnector(%d) asked again = %v, %v; want the connector under that identity", ids[1], conn, err)
	}
	port := fvInstance(t, ctx, obj, "a", "p")
	if end := conn.Ends[0].Value; !holdsObject(end, port.ID) {
		t.Errorf("the connector's end holds %v, want the port object %d", end, port.ID)
	}
	want := []int64{ids[0], ids[2]}
	if got := obj.KeptConnectorIDs(); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("KeptConnectorIDs() = %v, want the siblings %v still set aside", got, want)
	}
	if got := ctx.InstanceIDs(); len(got) != len(held)+1 {
		t.Errorf("the objects held are %v, want those before, %v, and the connector alone", got, held)
	}
}

const adoptTwoConnectSrc = `package Demo {
	port def P;
	part def A { port p : P; port r : P; }
	part def B { port q : P; port s : P; }
	part def Sys { part a : A; part b : B; connect a.p to b.q; connect a.r to b.s; }
}`

// endsOf names the ports the connector's ends hold, `a.p-b.q`, on the object owning them.
func endsOf(t *testing.T, ctx *Context, owner, conn *Instance) string {
	t.Helper()
	var names []string
	for _, end := range conn.Ends {
		id, held := end.Value.Object()
		if !held {
			t.Fatalf("connector %d's end holds %v, want a port object", conn.ID, end.Value)
		}
		for _, path := range [][2]string{{"a", "p"}, {"a", "r"}, {"b", "q"}, {"b", "s"}} {
			if fvInstance(t, ctx, owner, path[0], path[1]).ID == id {
				names = append(names, path[0]+"."+path[1])
			}
		}
	}
	return strings.Join(names, "-")
}

// A kept identity follows its declaration wherever it now stands among its siblings;
// the identity of a declaration edited away or removed is dropped, not handed on.
func TestKeptConnectorIdentitiesFollowTheirDeclarations(t *testing.T) {
	const (
		first  = "connect a.p to b.q;"
		second = "connect a.r to b.s;"
	)
	cases := []struct {
		name  string
		decls string
		// want maps the ends of each connector materialized before to those the
		// connector under its identity has after, "" for an identity dropped.
		want map[string]string
		// fresh is the connectors the new declarations add, by ends.
		fresh []string
	}{
		{"inserted before", "connect a.p to b.s; " + first + " " + second, map[string]string{"a.p-b.q": "a.p-b.q", "a.r-b.s": "a.r-b.s"}, []string{"a.p-b.s"}},
		{"reordered", second + " " + first, map[string]string{"a.p-b.q": "a.p-b.q", "a.r-b.s": "a.r-b.s"}, nil},
		{"edited", first + " connect a.r to b.q;", map[string]string{"a.p-b.q": "a.p-b.q", "a.r-b.s": ""}, []string{"a.r-b.q"}},
		{"removed", first, map[string]string{"a.p-b.q": "a.p-b.q", "a.r-b.s": ""}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prev := contextOver(t, adoptTwoConnectSrc)
			owner, err := prev.Instantiate(lookupOne(t, prev.resolver.Index(), "Demo::Sys"))
			if err != nil {
				t.Fatalf("Instantiate: %v", err)
			}
			conns, err := owner.OwnedConnectors(prev)
			if err != nil {
				t.Fatalf("OwnedConnectors: %v", err)
			}
			before := make(map[string]int64, len(conns))
			for _, conn := range conns {
				before[endsOf(t, prev, owner, conn)] = conn.ID
			}
			if len(before) != 2 {
				t.Fatalf("the connectors before are %v, want two with distinct ends", before)
			}
			shapes := prev.ShapesOf(owner)

			ctx := contextOver(t, strings.Replace(adoptTwoConnectSrc, first+" "+second, tc.decls, 1))
			if _, err := ctx.Adopt(prev, shapes, owner); err != nil {
				t.Fatalf("Adopt: %v", err)
			}
			var kept []int64
			for ends, id := range before {
				if tc.want[ends] != "" {
					kept = append(kept, id)
				}
			}
			slices.Sort(kept)
			if got := owner.KeptConnectorIDs(); fmt.Sprint(got) != fmt.Sprint(kept) {
				t.Errorf("KeptConnectorIDs() = %v, want %v: the identities of the declarations still there", got, kept)
			}
			for ends, id := range before {
				conn, err := owner.RestoreConnector(ctx, id)
				if err != nil {
					t.Fatalf("RestoreConnector(%d): %v", id, err)
				}
				if tc.want[ends] == "" {
					if conn != nil {
						t.Errorf("RestoreConnector(%d) of the %s connector = the %s connector, want nothing: its declaration is gone", id, ends, endsOf(t, ctx, owner, conn))
					}
					continue
				}
				if conn == nil || conn.ID != id {
					t.Fatalf("RestoreConnector(%d) = %v, want the connector under that identity", id, conn)
				}
				if got := endsOf(t, ctx, owner, conn); got != tc.want[ends] {
					t.Errorf("connector %d connects %s after the carry-over, want %s as before", id, got, tc.want[ends])
				}
			}
			after, err := owner.OwnedConnectors(ctx)
			if err != nil {
				t.Fatalf("OwnedConnectors after the carry-over: %v", err)
			}
			var fresh []string
			for _, conn := range after {
				if slices.Contains(slices.Collect(maps.Values(before)), conn.ID) {
					continue
				}
				fresh = append(fresh, endsOf(t, ctx, owner, conn))
			}
			slices.Sort(fresh)
			if fmt.Sprint(fresh) != fmt.Sprint(tc.fresh) {
				t.Errorf("the connectors under new identities connect %v, want %v", fresh, tc.fresh)
			}
			if got := owner.KeptConnectorIDs(); len(got) != 0 {
				t.Errorf("KeptConnectorIDs() = %v once all are materialized again, want none", got)
			}
		})
	}
}

// holdsObject reports whether the value is the object of the given identity.
func holdsObject(val Value, id int64) bool {
	got, ok := val.Object()
	return ok && got == id
}

// A declaration that no longer resolves to the shape an object was materialized
// against cannot hold that object, so it is refused rather than rebound onto
// feature values that no longer mean the same thing.
func TestAdoptRefusesAChangedShape(t *testing.T) {
	prev := contextOver(t, adoptSrc)
	obj := vehicleIn(t, prev)
	shapes := prev.ShapesOf(obj)

	ctx := contextOver(t, strings.Replace(adoptSrc, "mass = 1500.0", "mass = 900.0", 1))
	_, err := ctx.Adopt(prev, shapes, obj)
	if err == nil {
		t.Fatal("Adopt accepted an object of a declaration that changed")
	}
	if _, found := ctx.Instance(obj.ID); found {
		t.Error("the refused object was left in the context")
	}
}

// A change to a declaration an object only depends on invalidates it too: its
// feature values hold what that declaration says.
func TestAdoptRefusesAChangedDependency(t *testing.T) {
	prev := contextOver(t, adoptSrc)
	obj := vehicleIn(t, prev)
	shapes := prev.ShapesOf(obj)

	ctx := contextOver(t, strings.Replace(adoptSrc, "power = 300.0", "power = 100.0", 1))
	if _, err := ctx.Adopt(prev, shapes, obj); err == nil {
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
		if _, err := next.Adopt(ctx, shapes, obj); err != nil {
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

const adoptBehaviorSrc = `package Demo {
	part def Monitor {
		attribute count = 0;
		exhibit state modes {
			entry; then idle;
			state idle { entry action bump { assign count := count + 1; } }
		}
	}
}`

// A carried object's behavior is started again in the context carrying it: the
// object keeps its identity, its execution is a new one bound to the new
// analysis, and what the discarded run wrote is not read by the new one.
func TestAdoptRestartsACarriedObjectsBehavior(t *testing.T) {
	prev := contextOver(t, adoptBehaviorSrc)
	obj, err := prev.Instantiate(lookupOne(t, prev.resolver.Index(), "Demo::Monitor"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	before, ok := obj.ExhibitedState()
	if !ok {
		t.Fatal("the object exhibits no machine")
	}
	id, shapes := obj.ID, prev.ShapesOf(obj)

	ctx := contextOver(t, adoptBehaviorSrc+"\npart def Widget;")
	restarted, err := ctx.Adopt(prev, shapes, obj)
	if err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	if len(restarted) != 1 || !strings.Contains(restarted[0], "modes") {
		t.Errorf("Adopt reported %v restarted, want the machine modes", restarted)
	}
	if obj.ID != id {
		t.Errorf("the carried object has identity %d, want %d", obj.ID, id)
	}
	after, ok := obj.ExhibitedState()
	if !ok {
		t.Fatal("the carried object runs no machine after the carry-over")
	}
	if after.State == before.State {
		t.Error("the carried object still runs the execution of the previous analysis")
	}
	// The entry action ran once in the new execution, over the declared initial
	// value rather than over the 1 the discarded run left.
	count, err := obj.GetFeatureValue(ctx, "count")
	if err != nil {
		t.Fatalf("GetFeatureValue(count): %v", err)
	}
	if got := count.Value; got.Kind != ValConst || got.Const.Int != 1 {
		t.Errorf("count = %v, want 1: the restarted machine read the declared initial value", got)
	}
}

// A behavior that cannot be started again in the new context refuses the
// carry-over rather than leaving an object running nothing, and the object is not
// left registered in a context that cannot offer it.
func TestAdoptRefusesAnObjectWhoseBehaviorCannotRestart(t *testing.T) {
	prev := contextOver(t, adoptBehaviorSrc)
	obj, err := prev.Instantiate(lookupOne(t, prev.resolver.Index(), "Demo::Monitor"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	shapes := prev.ShapesOf(obj)

	ctx := contextOver(t, adoptBehaviorSrc+"\npart def Widget;")
	tight := DefaultBudgets()
	tight.MaxSteps = 1
	if err := ctx.SetBudgets(tight); err != nil {
		t.Fatalf("SetBudgets: %v", err)
	}
	if _, err := ctx.Adopt(prev, shapes, obj); err == nil {
		t.Fatal("Adopt accepted an object whose behavior could not be started")
	}
	if _, found := ctx.Instance(obj.ID); found {
		t.Error("the refused object was left in the context")
	}
	if len(obj.Behaviors()) != 0 {
		t.Errorf("the refused object still holds %d behaviors", len(obj.Behaviors()))
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

// An edit confined to a body governing over an inherited value changes what
// instantiating produces, so the shape follows it and the object is not carried
// with the value that body replaced.
func TestShapeFollowsAGoverningValueBody(t *testing.T) {
	const src = `package Demo {
	attribute def Cost { attribute v = 1.0; }
	part def Ring { attribute template : Cost { attribute :>> v = 9.0; } attribute cost : Cost = template; }
	part def Band :> Ring { attribute :>> cost { attribute :>> v = 11.0; } }
}`
	prev := contextOver(t, src)
	sym := lookupOne(t, prev.resolver.Index(), "Demo::Band")
	before := prev.ShapeDigest(sym)

	ctx := contextOver(t, strings.Replace(src, "v = 11.0", "v = 12.0", 1))
	if after := ctx.ShapeDigest(lookupOne(t, ctx.resolver.Index(), "Demo::Band")); after == before {
		t.Errorf("shape unchanged by an edit to the governing body: %s", after)
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

const crateSrc = `package Demo {
	private import Shapes::*;
	part def Crate { part box : Box; }
}`

// crateContextOver indexes the crate model over a Shapes library of its own,
// Domain content whose text the index vouches for unless vouch is false.
func crateContextOver(t *testing.T, lib string, vouch bool) *Context {
	t.Helper()
	idx := symbols.NewIndex()
	idx.AddDocument("Shapes.sysml", parser.New(source.New("Shapes.sysml", []byte(lib))).ParseFile())
	doc := symbols.LibraryDocument{Tier: symbols.TierDomain}
	if vouch {
		doc.Digest = symbols.TextDigest([]byte(lib))
	}
	idx.MarkLibraryDocument("Shapes.sysml", doc)
	idx.AddDocument("<test>", parser.New(source.New("<test>", []byte(crateSrc))).ParseFile())
	idx.ExpandWildcardImports()
	resolver := resolve.New(idx)
	ctx := NewContext(semantics.NewModel(resolver), resolver, 10000)
	ctx.RegisterSource(source.New("<test>", []byte(crateSrc)))
	ctx.RegisterSource(source.New("Shapes.sysml", []byte(lib)))
	return ctx
}

// crateIn materializes a crate and the box part inside it.
func crateIn(t *testing.T, ctx *Context) *Instance {
	t.Helper()
	obj, err := ctx.Instantiate(lookupOne(t, ctx.resolver.Index(), "Demo::Crate"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	if _, err := obj.GetFeatureValue(ctx, "box"); err != nil {
		t.Fatalf("GetFeatureValue(box): %v", err)
	}
	return obj
}

// A shape digest names a library type instead of expanding it, so it must say
// which library: two indexes loaded on their own may declare a type of the same
// name with different features, and an object of one is refused by the other.
func TestAdoptRefusesASameNamedLibraryTypeOfAnotherLibrary(t *testing.T) {
	const lib = `package Shapes { part def Box { attribute n = 1; } }`
	prev := crateContextOver(t, lib, true)
	obj := crateIn(t, prev)
	shapes := prev.ShapesOf(obj)

	same := crateContextOver(t, lib, true)
	if _, err := same.Adopt(prev, shapes, obj); err != nil {
		t.Fatalf("Adopt over the same library: %v", err)
	}

	other := crateContextOver(t, strings.Replace(lib, "attribute n = 1;", "attribute n = 1; attribute m = 2;", 1), true)
	if _, err := other.Adopt(prev, shapes, obj); err == nil {
		t.Fatal("Adopt accepted an object whose box type gained a feature in the other library")
	}
	if _, found := other.Instance(obj.ID); found {
		t.Error("the refused object was left in the context")
	}
}

// A library whose text the index does not vouch for is expanded like the model,
// so the digest still follows what its types declare.
func TestShapeDigestExpandsALibraryOfUnknownText(t *testing.T) {
	const lib = `package Shapes { part def Box { attribute n = 1; } }`
	ctx := crateContextOver(t, lib, false)
	if _, known := ctx.resolver.Index().LibraryIdentity(); known {
		t.Fatal("LibraryIdentity is known for a library document of no digest")
	}
	digest := ctx.ShapeDigest(lookupOne(t, ctx.resolver.Index(), "Demo::Crate"))
	if !strings.Contains(digest, "Shapes::Box/partDef{n:1..1=1@") {
		t.Errorf("digest names the library type without expanding it: %s", digest)
	}
	other := crateContextOver(t, strings.Replace(lib, "n = 1", "n = 2", 1), false)
	if other.ShapeDigest(lookupOne(t, other.resolver.Index(), "Demo::Crate")) == digest {
		t.Error("digest unchanged by an edit to the library type it expands")
	}
}
