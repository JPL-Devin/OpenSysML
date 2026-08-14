package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/lower"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// instantiatePart instantiates the named part def, for a case about the
// connectors an object of it owns.
func instantiatePart(t *testing.T, name, src string) (*Instance, *Context) {
	t.Helper()
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), name, ast.DefPart)
	if sym == nil {
		t.Fatalf("part def %s not found", name)
	}
	inst, err := ctx.Instantiate(sym)
	if err != nil {
		t.Fatalf("Instantiate %s: %v", name, err)
	}
	return inst, ctx
}

// slotInstance reads a slot expected to hold one object and returns it.
func slotInstance(t *testing.T, ctx *Context, inst *Instance, path ...string) *Instance {
	t.Helper()
	cur := inst
	for _, name := range path {
		slot, err := cur.GetSlot(ctx, name)
		if err != nil {
			t.Fatalf("GetSlot %s: %v", name, err)
		}
		if slot.Value.Kind != ValInstance && slot.Value.Kind != ValVariant {
			t.Fatalf("slot %s holds %s, want an object", name, slot.Value.Kind)
		}
		next, ok := ctx.Instance(slot.Value.Instance)
		if !ok {
			t.Fatalf("slot %s names object %d, which the context does not hold", name, slot.Value.Instance)
		}
		cur = next
	}
	return cur
}

const twoPortSystem = `
	package test {
		private import ScalarValues::Real;
		port def P { attribute rate : Real = 3.0; }
		part def A { port p : P; }
		part def B { port q : P; }
		connection def Link {
			end source : P;
			end target : P;
		}
		part def Sys {
			part a : A;
			part b : B;
			connection link : Link connect a.p to b.q;
			interface iface connect a.p to b.q;
			connect a.p to b.q;
		}
	}
`

// A connector end is the feature it attaches to, not a copy of it: the object
// at `link.source` is the very object `a.p` holds (KerML 1.0 §7.4.6).
func TestConnectorEndsAreTheConnectedFeatures(t *testing.T) {
	inst, ctx := instantiatePart(t, "Sys", twoPortSystem)
	port := slotInstance(t, ctx, inst, "a", "p")
	peer := slotInstance(t, ctx, inst, "b", "q")

	link := slotInstance(t, ctx, inst, "link")
	if got := slotInstance(t, ctx, link, "source"); got.ID != port.ID {
		t.Errorf("link.source is object %d, want a.p (%d)", got.ID, port.ID)
	}
	if got := slotInstance(t, ctx, link, "target"); got.ID != peer.ID {
		t.Errorf("link.target is object %d, want b.q (%d)", got.ID, peer.ID)
	}
}

// Writing through a connected port is read through the end attached to it:
// sharing identity is what makes the connection observable.
func TestWritingAConnectedPortIsReadThroughTheEnd(t *testing.T) {
	inst, ctx := instantiatePart(t, "Sys", twoPortSystem)
	port := slotInstance(t, ctx, inst, "a", "p")
	slot, err := port.GetSlot(ctx, "rate")
	if err != nil {
		t.Fatalf("GetSlot rate: %v", err)
	}
	slot.Value = Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: 9.5}}

	end := slotInstance(t, ctx, slotInstance(t, ctx, inst, "link"), "source")
	read, err := end.GetSlot(ctx, "rate")
	if err != nil {
		t.Fatalf("GetSlot rate through the end: %v", err)
	}
	if read.Value.Const.Real != 9.5 {
		t.Errorf("link.source.rate = %v, want the 9.5 written on a.p", read.Value)
	}
}

// An untyped connector usage names no definition, so its type is implicit
// (SysML v2 §8.3.13): it materializes all the same, with the connected features
// at its ends, rather than reading as an unknown value.
func TestUntypedConnectorUsageMaterializes(t *testing.T) {
	inst, ctx := instantiatePart(t, "Sys", twoPortSystem)
	port := slotInstance(t, ctx, inst, "a", "p")
	peer := slotInstance(t, ctx, inst, "b", "q")

	iface := slotInstance(t, ctx, inst, "iface")
	if len(iface.Ends) != 2 {
		t.Fatalf("iface has %d ends, want 2", len(iface.Ends))
	}
	if got := slotInstance(t, ctx, iface, "source"); got.ID != port.ID {
		t.Errorf("iface.source is object %d, want a.p (%d)", got.ID, port.ID)
	}
	if got := slotInstance(t, ctx, iface, "target"); got.ID != peer.ID {
		t.Errorf("iface.target is object %d, want b.q (%d)", got.ID, peer.ID)
	}
}

// An anonymous `connect a.p to b.q` is a member of the object no slot names, and
// joins its ends exactly as a named connector does.
func TestAnonymousConnectorJoinsItsEnds(t *testing.T) {
	inst, ctx := instantiatePart(t, "Sys", twoPortSystem)
	port := slotInstance(t, ctx, inst, "a", "p")
	peer := slotInstance(t, ctx, inst, "b", "q")

	conns, err := inst.OwnedConnectors(ctx)
	if err != nil {
		t.Fatalf("OwnedConnectors: %v", err)
	}
	if len(conns) != 1 {
		t.Fatalf("object owns %d anonymous connectors, want 1", len(conns))
	}
	ends := conns[0].Ends
	if len(ends) != 2 {
		t.Fatalf("anonymous connector has %d ends, want 2", len(ends))
	}
	if ends[0].Value.Instance != port.ID || ends[1].Value.Instance != peer.ID {
		t.Errorf("anonymous connector joins %d and %d, want a.p (%d) and b.q (%d)",
			ends[0].Value.Instance, ends[1].Value.Instance, port.ID, peer.ID)
	}
}

// Reading the same anonymous connector twice reads the same object: it is one
// member of the object, materialized once.
func TestAnonymousConnectorIsMaterializedOnce(t *testing.T) {
	inst, ctx := instantiatePart(t, "Sys", twoPortSystem)
	first, err := inst.OwnedConnectors(ctx)
	if err != nil {
		t.Fatalf("OwnedConnectors: %v", err)
	}
	second, err := inst.OwnedConnectors(ctx)
	if err != nil {
		t.Fatalf("OwnedConnectors again: %v", err)
	}
	if len(first) != 1 || len(second) != 1 || first[0].ID != second[0].ID {
		t.Errorf("anonymous connector read as %v then %v, want one object both times", first, second)
	}
}

const nestedSystem = `
	package test {
		private import ScalarValues::Real;
		port def P { attribute rate : Real = 3.0; }
		part def Inner { port p : P; }
		part def A { part inner : Inner; }
		part def B { port q : P; }
		connection def Link { end source : P; end target : P; }
		connection def Link2 :> Link { end s2 : P; end t2 : P; }
		part def Sys {
			part a : A;
			part b : B;
			connection nested : Link connect a.inner.p to b.q;
			connection parts connect a to b;
			connection tri connect (a, b, a.inner);
			connection sub : Link2 connect a.inner.p to b.q;
		}
	}
`

// An end may name a feature reached through a chain: `a.inner.p` attaches the
// port of the nested part, not a port of `a`.
func TestConnectorEndFollowsAFeatureChain(t *testing.T) {
	inst, ctx := instantiatePart(t, "Sys", nestedSystem)
	port := slotInstance(t, ctx, inst, "a", "inner", "p")
	nested := slotInstance(t, ctx, inst, "nested")
	if got := slotInstance(t, ctx, nested, "source"); got.ID != port.ID {
		t.Errorf("nested.source is object %d, want a.inner.p (%d)", got.ID, port.ID)
	}
}

// An end may attach to a part rather than a port: what a connector relates is
// features, of whatever kind.
func TestConnectorEndAttachesToAPart(t *testing.T) {
	inst, ctx := instantiatePart(t, "Sys", nestedSystem)
	a := slotInstance(t, ctx, inst, "a")
	b := slotInstance(t, ctx, inst, "b")
	parts := slotInstance(t, ctx, inst, "parts")
	if got := slotInstance(t, ctx, parts, "source"); got.ID != a.ID {
		t.Errorf("parts.source is object %d, want a (%d)", got.ID, a.ID)
	}
	if got := slotInstance(t, ctx, parts, "target"); got.ID != b.ID {
		t.Errorf("parts.target is object %d, want b (%d)", got.ID, b.ID)
	}
}

// A connector with more than two ends keeps every one of them, in declaration
// order, in the participant feature a link holds its ends in.
func TestNaryConnectorKeepsEveryEnd(t *testing.T) {
	inst, ctx := instantiatePart(t, "Sys", nestedSystem)
	want := []int64{
		slotInstance(t, ctx, inst, "a").ID,
		slotInstance(t, ctx, inst, "b").ID,
		slotInstance(t, ctx, inst, "a", "inner").ID,
	}
	tri := slotInstance(t, ctx, inst, "tri")
	slot, err := tri.GetSlot(ctx, participantEndName)
	if err != nil {
		t.Fatalf("GetSlot participant: %v", err)
	}
	if slot.Values.Kind != ValSequence {
		t.Fatalf("participant holds %s, want a sequence of the ends", slot.Values.Kind)
	}
	got := slot.Values.Sequence.Elements()
	if len(got) != len(want) {
		t.Fatalf("participant holds %d ends, want %d", len(got), len(want))
	}
	for i, el := range got {
		if el.Instance != want[i] {
			t.Errorf("participant#%d is object %d, want %d", i+1, el.Instance, want[i])
		}
	}
}

// A connector declared in a specialization redefines the inherited ends by
// position (SysML v2 §8.3.13), so both names read the one end.
func TestRedefinedEndSharesTheInheritedSlot(t *testing.T) {
	inst, ctx := instantiatePart(t, "Sys", nestedSystem)
	port := slotInstance(t, ctx, inst, "a", "inner", "p")
	peer := slotInstance(t, ctx, inst, "b", "q")
	sub := slotInstance(t, ctx, inst, "sub")
	for _, name := range []string{"s2", "source"} {
		if got := slotInstance(t, ctx, sub, name); got.ID != port.ID {
			t.Errorf("sub.%s is object %d, want a.inner.p (%d)", name, got.ID, port.ID)
		}
	}
	for _, name := range []string{"t2", "target"} {
		if got := slotInstance(t, ctx, sub, name); got.ID != peer.ID {
			t.Errorf("sub.%s is object %d, want b.q (%d)", name, got.ID, peer.ID)
		}
	}
}

const variationSystem = `
	package test {
		port def P;
		part def Part { port p1 : P; port p2 : P; port p3 : P; }
		part def Sys {
			part x : Part;
			variation interface link {
				variant interface direct connect x.p1 to x.p2;
				variant interface indirect connect x.p1 to x.p3;
			}
		}
		part chosenDirect : Sys { ref :>> link = link::direct; }
		part chosenIndirect : Sys { ref :>> link = link::indirect; }
	}
`

// The connection a selected `variant interface` declares is realized: the
// variation's slot holds that variant's connector, with the features it connects
// at its ends. Selecting the other variant realizes the other connection.
func TestSelectedVariantInterfaceIsRealized(t *testing.T) {
	for _, tt := range []struct {
		usage  string
		target string // the port the selected variant connects x.p1 to
	}{
		{"chosenDirect", "p2"},
		{"chosenIndirect", "p3"},
	} {
		idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, variationSystem))
		sym := findSymbolByName(idx.DocumentRoot("<test>"), tt.usage, ast.DefPart)
		if sym == nil {
			t.Fatalf("part %s not found", tt.usage)
		}
		inst, err := ctx.Instantiate(sym)
		if err != nil {
			t.Fatalf("Instantiate %s: %v", tt.usage, err)
		}
		source := slotInstance(t, ctx, inst, "x", "p1")
		want := slotInstance(t, ctx, inst, "x", tt.target)
		other := "p3"
		if tt.target == "p3" {
			other = "p2"
		}
		unselected := slotInstance(t, ctx, inst, "x", other)

		link := slotInstance(t, ctx, inst, "link")
		if len(link.Ends) != 2 {
			t.Fatalf("%s: the realized connection has %d ends, want 2", tt.usage, len(link.Ends))
		}
		if got := slotInstance(t, ctx, link, "source"); got.ID != source.ID {
			t.Errorf("%s: link.source is object %d, want x.p1 (%d)", tt.usage, got.ID, source.ID)
		}
		got := slotInstance(t, ctx, link, "target")
		if got.ID != want.ID {
			t.Errorf("%s: link.target is object %d, want x.%s (%d)", tt.usage, got.ID, tt.target, want.ID)
		}
		if got.ID == unselected.ID {
			t.Errorf("%s: link.target is x.%s, which the unselected variant connects", tt.usage, other)
		}
	}
}

// testUnattachableConnectorEnd: an end naming a feature no object reaches is
// reported, with where the end was written — a connector that cannot be attached
// is no connector, and must not read as an unknown value or fabricate an object
// of the end's declared type.
func testUnattachableConnectorEnd(t *testing.T) {
	inst, ctx := instantiatePart(t, "Sys", `
		package test {
			port def P;
			part def A { port p : P; }
			part def Sys {
				part a : A;
				connection link connect a.p to a.missing;
			}
		}
	`)
	_, err := inst.GetSlot(ctx, "link")
	if !errors.Is(err, ErrConnectorEnd) {
		t.Fatalf("expected ErrConnectorEnd, got: %v", err)
	}
	var endErr *ConnectorEndError
	if !errors.As(err, &endErr) {
		t.Fatalf("expected a *ConnectorEndError, got %T", err)
	}
	if endErr.End != "a.missing" {
		t.Errorf("error names end %q, want a.missing", endErr.End)
	}
	if !strings.Contains(endErr.Location, "<test>") {
		t.Errorf("error is located at %q, want the file the end was written in", endErr.Location)
	}
}

// testMultiplicityOnAConnector: a connector usage holding more than one
// connector is not one connection, so there is no set of ends to attach — that
// is reported rather than filled with objects of the connector's type.
func testMultiplicityOnAConnector(t *testing.T) {
	inst, ctx := instantiatePart(t, "Sys", `
		package test {
			port def P;
			part def A { port p : P; }
			part def B { port q : P; }
			part def Sys {
				part a : A;
				part b : B;
				connection links[2] connect a.p to b.q;
			}
		}
	`)
	_, err := inst.GetSlot(ctx, "links")
	if !errors.Is(err, ErrConnectorEnd) {
		t.Fatalf("expected ErrConnectorEnd, got: %v", err)
	}
	var endErr *ConnectorEndError
	if !errors.As(err, &endErr) {
		t.Fatalf("expected a *ConnectorEndError, got %T", err)
	}
	if !strings.Contains(endErr.Location, "<test>") {
		t.Errorf("error is located at %q, want the file the connector was written in", endErr.Location)
	}
}

// testConnectorAttachedToItself: an end naming the connector it belongs to would
// attach ends forever, so the cycle is reported rather than recursed into.
func testConnectorAttachedToItself(t *testing.T) {
	inst, ctx := instantiatePart(t, "Sys", `
		package test {
			port def P;
			part def A { port p : P; }
			part def Sys {
				part a : A;
				connection link connect link to a.p;
			}
		}
	`)
	if _, err := inst.GetSlot(ctx, "link"); !errors.Is(err, ErrCyclicSlot) {
		t.Fatalf("expected ErrCyclicSlot, got: %v", err)
	}
}

// testMutuallyAttachedConnectors: two connectors each naming the other as an end
// are a cycle across slots, reported the same way as a self-attached one.
func testMutuallyAttachedConnectors(t *testing.T) {
	inst, ctx := instantiatePart(t, "Sys", `
		package test {
			port def P;
			part def A { port p : P; }
			part def Sys {
				part a : A;
				connection here connect there to a.p;
				connection there connect here to a.p;
			}
		}
	`)
	if _, err := inst.GetSlot(ctx, "here"); !errors.Is(err, ErrCyclicSlot) {
		t.Fatalf("expected ErrCyclicSlot, got: %v", err)
	}
}

// An anonymous succession or transition carries ends too, but relates them in
// time rather than joining them, so it is no connector to materialize and must
// not be reported as one when the connectors of an object are read.
func TestAnonymousSuccessionIsNoConnector(t *testing.T) {
	inst, ctx := instantiatePart(t, "Sys", `
		package test {
			port def P;
			part def A { port p : P; }
			part def B { port q : P; }
			action def Step;
			part def Sys {
				part a : A;
				part b : B;
				action one : Step;
				action two : Step;
				succession one then two;
				connect a.p to b.q;
			}
		}
	`)
	conns, err := inst.OwnedConnectors(ctx)
	if err != nil {
		t.Fatalf("OwnedConnectors: %v", err)
	}
	if len(conns) != 1 {
		t.Fatalf("the object owns %d anonymous connectors, want only the `connect`", len(conns))
	}
	if len(conns[0].Ends) != 2 {
		t.Errorf("the connector has %d ends, want 2", len(conns[0].Ends))
	}
}

// A variation point belongs to the object that selected it: two objects of one
// type each selecting a variant must record their own selection, so neither
// overwrites the other's.
func TestVariantSelectionIsPerOwner(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `
	package test {
		port def P;
		part def Engine { port p : P; }
		abstract part family {
			variation part engine : Engine {
				variant part electric : Engine;
				variant part petrol : Engine;
			}
		}
		part sedan :> family { part :>> engine = engine::electric; }
		part coupe :> family { part :>> engine = engine::petrol; }
	}`))
	owners := map[string]*Instance{}
	for usage, want := range map[string]string{"test::sedan": "electric", "test::coupe": "petrol"} {
		inst, err := ctx.Instantiate(oneSymbol(t, idx, usage))
		if err != nil {
			t.Fatalf("%s: %v", usage, err)
		}
		if _, err := inst.GetSlot(ctx, "engine"); err != nil {
			t.Fatalf("%s.engine: %v", usage, err)
		}
		owners[want] = inst
		if got := ctx.selectedVariants[variantSelection{owner: inst.ID, variation: "engine"}]; got != want {
			t.Errorf("%s selected %q, want %q", usage, got, want)
		}
	}
	// A connection of an object is governed by that object's own selection, so
	// each object resolves the variation to the variant it selected.
	conn := lower.Connection{Variation: "engine", Owner: lower.OwnerObject}
	for want, inst := range owners {
		if got := ctx.selectedVariant(conn, inst); got != want {
			t.Errorf("routing resolved engine to %q for the object selecting %q", got, want)
		}
	}
}

// A connector usage is not a part: instantiating the object it denotes must not
// go through ordinary composite materialization, which would build an object of
// each end's declared type instead of attaching the connected features.
func TestConnectorUsageIsRecognized(t *testing.T) {
	idx, model, _ := buildRuntime(t, "<test>", parseAndBuild(t, twoPortSystem))
	scope := idx.DocumentRoot("<test>")
	sys := findSymbolByName(scope, "Sys", ast.DefPart)
	if sys == nil || sys.Scope == nil {
		t.Fatal("Sys part def not found")
	}
	for name, want := range map[string]bool{"link": true, "iface": true, "a": false, "b": false} {
		var sym *symbols.Symbol
		if found, ok := sys.Scope.LookupLocal(name); ok {
			sym = found
		}
		if sym == nil {
			t.Fatalf("member %s not found", name)
		}
		if got := model.IsConnectorUsage(sym); got != want {
			t.Errorf("IsConnectorUsage(%s) = %v, want %v", name, got, want)
		}
	}
}

// Every connector-like usage that states ends with a connect clause attaches the
// same way, whatever keyword declares it: an allocation states its ends with
// `allocate` (SysML v2 §8.3.19) and a KerML `connector` with `connect`. Both
// take their implicit type from the library when they name no definition.
func TestEveryConnectorKindAttachesItsEnds(t *testing.T) {
	inst, ctx := instantiatePart(t, "Sys", `
		package test {
			port def P;
			part def A { port p : P; }
			part def B { port q : P; }
			part def Sys {
				part a : A;
				part b : B;
				allocation alloc allocate a.p to b.q;
				connector wire connect a.p to b.q;
			}
		}
	`)
	port := slotInstance(t, ctx, inst, "a", "p")
	peer := slotInstance(t, ctx, inst, "b", "q")
	for _, name := range []string{"alloc", "wire"} {
		conn := slotInstance(t, ctx, inst, name)
		if got := slotInstance(t, ctx, conn, "source"); got.ID != port.ID {
			t.Errorf("%s.source is object %d, want a.p (%d)", name, got.ID, port.ID)
		}
		if got := slotInstance(t, ctx, conn, "target"); got.ID != peer.ID {
			t.Errorf("%s.target is object %d, want b.q (%d)", name, got.ID, peer.ID)
		}
	}
}
