package runtime

import (
	"errors"
	"maps"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// instantiateQualified materializes the object the qualified name declares.
func instantiateQualified(t *testing.T, ctx *Context, idx *symbols.Index, name string) *Instance {
	t.Helper()
	matches := idx.LookupQualified(name)
	if len(matches) != 1 {
		t.Fatalf("%s: %d matching symbols, want 1", name, len(matches))
	}
	inst, err := ctx.Instantiate(matches[0])
	if err != nil {
		t.Fatalf("Instantiate(%s): %v", name, err)
	}
	return inst
}

// readInstance reads a feature holding one object and returns that object.
func readInstance(t *testing.T, ctx *Context, inst *Instance, name string) *Instance {
	t.Helper()
	fv, err := inst.GetFeatureValue(ctx, name)
	if err != nil {
		t.Fatalf("GetFeatureValue(%s): %v", name, err)
	}
	held := fv.HeldValue()
	if held.Kind != ValInstance {
		t.Fatalf("%s = %s, want one object", name, FormatValue(held))
	}
	obj, ok := ctx.getInstance(held.Instance)
	if !ok {
		t.Fatalf("%s: object %d not registered", name, held.Instance)
	}
	return obj
}

func readInt(t *testing.T, ctx *Context, inst *Instance, name string) int64 {
	t.Helper()
	fv, err := inst.GetFeatureValue(ctx, name)
	if err != nil {
		t.Fatalf("GetFeatureValue(%s): %v", name, err)
	}
	return intValue(t, map[string]Value{name: fv.HeldValue()}, name)
}

const classifiedValueModel = `
	package test {
		private import ScalarValues::*;
		item def Segment { item ends [2]; }
		item def Loop { item edges : Segment [*]; }
		item def Square :> Loop {
			item :>> edges [4] = (e1, e2, e3, e4);
			item e1 [1];
			item e2 [1];
			item e3 [1];
			item e4 [1];
		}
		item sq : Square;
	}
`

// A feature's values are instances of its type (KerML 1.0 §7.3.4.1): an untyped member listed
// in `edges : Segment` is the same object, now a Segment, whichever is read first, and only once.
func TestListedValueIsClassifiedByTheFeatureType(t *testing.T) {
	for _, first := range []string{"e1", "edges"} {
		t.Run(first+"_first", func(t *testing.T) {
			ctx, idx := libraryShapeContext(t, classifiedValueModel)
			sq := instantiateQualified(t, ctx, idx, "test::sq")
			segment := idx.LookupQualified("test::Segment")[0]
			if _, err := sq.GetFeatureValue(ctx, first); err != nil {
				t.Fatalf("GetFeatureValue(%s): %v", first, err)
			}

			e1 := readInstance(t, ctx, sq, "e1")
			edges, err := sq.GetFeatureValue(ctx, "edges")
			if err != nil {
				t.Fatalf("GetFeatureValue(edges): %v", err)
			}
			els := elementsOf(edges.HeldValue())
			if len(els) != 4 || els[0].Kind != ValInstance || els[0].Instance != e1.ID {
				t.Fatalf("edges = %s, want e1 (%d) first of four", FormatValue(edges.HeldValue()), e1.ID)
			}
			if e1.Type.Name != "e1" || !ctx.instanceConforms(e1, segment) {
				t.Fatalf("e1 is declared by %s and is a Segment: %v", e1.Type.Name, ctx.instanceConforms(e1, segment))
			}
			ends, err := e1.GetFeatureValue(ctx, "ends")
			if err != nil {
				t.Fatalf("GetFeatureValue(e1.ends): %v", err)
			}
			if got := len(elementsOf(ends.HeldValue())); got != 2 {
				t.Fatalf("e1.ends holds %d objects, want 2", got)
			}

			before := len(e1.classifiers)
			if err := ctx.classify(e1, segment); err != nil {
				t.Fatalf("classify again: %v", err)
			}
			if len(e1.classifiers) != before {
				t.Fatalf("classifying an object twice grew its classifiers from %d to %d", before, len(e1.classifiers))
			}
		})
	}
}

// A holding computed rather than listed — the object chosen by a condition, an index, an
// invocation or a body — classifies its object before it is first read all the same: the
// classifier's features and behavior are there whichever feature is read first.
func TestComputedHoldingIsClassifiedWhicheverIsReadFirst(t *testing.T) {
	holdings := map[string]string{
		"conditional": "if pickLead ? lead else trail",
		"indexed":     "(trail, lead)#(2)",
		"invocation":  "SequenceFunctions::head((lead, trail))",
		"body":        "(lead, trail)->select { in x; x == lead }",
	}
	for name, holding := range holdings {
		t.Run(name, func(t *testing.T) {
			ctx, idx := libraryShapeContext(t, `package test {
				private import ScalarValues::*;
				private import ControlFunctions::*;
				state def Glowing {
					attribute cycles : Integer = 0;
					entry; then lit;
					state lit { entry action count { assign cycles := cycles + 1; } }
				}
				item def Tallied { attribute tally : Integer = 7; exhibit state glow : Glowing; }
				item def Rack {
					attribute pickLead : Boolean = true;
					item lead [1];
					item trail [1];
					item tallied : Tallied [1] = `+holding+`;
				}
				item rack : Rack;
			}`)
			rack := instantiateQualified(t, ctx, idx, "test::rack")
			tallied := idx.LookupQualified("test::Tallied")[0]

			lead := readInstance(t, ctx, rack, "lead")
			if !ctx.instanceConforms(lead, tallied) {
				t.Fatalf("lead, read first, is classified by %v, want Tallied", lead.classifiers)
			}
			if got := readInt(t, ctx, lead, "tally"); got != 7 {
				t.Fatalf("lead.tally = %d, want 7", got)
			}
			glow, ok := lead.Behavior("glow")
			if !ok || glow.State == nil || glow.State.stateData["cycles"].Const.Int != 1 {
				t.Fatal("lead, read first, runs no glow state machine")
			}
			if held := readInstance(t, ctx, rack, "tallied"); held != lead {
				t.Fatalf("tallied holds object %d, want lead (%d)", held.ID, lead.ID)
			}
			if trail := readInstance(t, ctx, rack, "trail"); ctx.instanceConforms(trail, tallied) {
				t.Fatal("trail, which tallied does not hold, is a Tallied")
			}
		})
	}
}

// An argument a calc's returns never pass on is not held by the feature the call values:
// reading it neither computes that feature — which may fail — nor classifies the object
// the call does return, so no behavior starts on it.
func TestArgumentNotReturnedIsNotHeldByTheCall(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package test {
		private import ScalarValues::*;
		state def Glowing {
			attribute cycles : Integer = 0;
			entry; then lit;
			state lit { entry action count { assign cycles := cycles + 1; } }
		}
		item def Tallied { attribute tally : Integer = 7; exhibit state glow : Glowing; }
		calc def pickChosen { in chosen; in other; return : Anything = chosen; }
		item def Rack {
			item lead [1];
			item trail [1];
			item tallied : Tallied [1] = pickChosen(lead, trail);
			item failing : Tallied [1] = pickChosen(3, trail);
		}
		item rack : Rack;
	}`)
	rack := instantiateQualified(t, ctx, idx, "test::rack")
	tallied := idx.LookupQualified("test::Tallied")[0]

	trail := readInstance(t, ctx, rack, "trail")
	if ctx.instanceConforms(trail, tallied) {
		t.Fatal("trail, which no call returns, is a Tallied")
	}
	if rack.FeatureValues["tallied"].Materialized || rack.FeatureValues["lead"].Materialized {
		t.Fatal("reading trail computed tallied, which does not hold it")
	}
	lead := readInstance(t, ctx, rack, "lead")
	if !ctx.instanceConforms(lead, tallied) {
		t.Fatalf("lead, read first, is classified by %v, want Tallied", lead.classifiers)
	}
	if glow, ok := lead.Behavior("glow"); !ok || glow.State == nil || glow.State.stateData["cycles"].Const.Int != 1 {
		t.Fatal("lead, read first, runs no glow state machine")
	}
	if _, err := rack.GetFeatureValue(ctx, "failing"); err == nil {
		t.Fatal("failing holds 3 as a Tallied without error")
	}
}

// Only a feature that may answer another's objects as its own holds them: an attribute
// computing from a feature, a condition tested, a chain through a feature, and an argument
// a calc's returns do not pass on hold nothing, so reading those features forces no other.
// A function whose body the model does not state may return any argument.
func TestOnlyFeaturesPassingObjectsOnHoldThem(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package test {
		private import ScalarValues::*;
		private import SequenceFunctions::*;
		item def Gauge { item cell [1]; }
		item def Tallied { attribute tally : Integer = 7; }
		calc def pickFirst { in chosen : Gauge; in other : Gauge; return : Gauge = chosen; }
		calc def pickAt { in gauges : Gauge [*]; in n : Integer; gauges#(n) }
		calc def pickWhen {
			in gauges : Gauge [*]; in fallback : Gauge; in wanted : Boolean;
			if wanted { for g in gauges { return : Gauge = g; } }
			return : Gauge = fallback;
		}
		item def Rack {
			attribute pickLead : Boolean = true;
			attribute count : Integer = 2;
			attribute doubled : Integer = count * 2;
			item lead : Gauge [1];
			item trail : Gauge [1];
			item spare : Gauge [1];
			item tallied : Tallied [1] = if pickLead ? lead else trail;
			item cells [2] = (lead, trail).cell;
			item picked = (lead, trail)#(count - 1);
			item led : Tallied [1] = pickFirst(lead, trail);
			item trailed : Tallied [1] = pickFirst(chosen = trail, other = lead);
			item indexed : Tallied [1] = pickAt((lead, trail), count);
			item looped : Tallied [1] = pickWhen((lead, trail), spare, pickLead);
			item headed : Tallied [1] = head((lead, count));
		}
		item rack : Rack;
	}`)
	rack := idx.LookupQualified("test::Rack")[0]

	want := map[string][]string{
		"lead":     {"tallied", "picked", "led", "indexed", "looped", "headed"},
		"trail":    {"tallied", "picked", "trailed", "indexed", "looped"},
		"spare":    {"looped"},
		"pickLead": nil,
		"count":    nil,
		"doubled":  nil,
	}
	for name, holders := range want {
		if got := ctx.holdingFeatures(rack, name); !slices.Equal(got, holders) {
			t.Errorf("holdingFeatures(Rack, %s) = %v, want %v", name, got, holders)
		}
	}
}

// A collect body's parameter stands for the operand's elements: a body answering it, directly,
// through a feature it declares, or in a nested body, passes the operand's objects on, so the
// collecting feature holds them whichever is read first; a body answering another feature
// passes that one on instead, and one chaining through the parameter passes nothing on.
func TestCollectedObjectsAreHeldByTheCollectingFeature(t *testing.T) {
	src := `package test {
		private import ScalarValues::*;
		item def Tallied { attribute tally : Integer = 7; }
		item def Rack {
			item lead [1] { item cell [1]; }
			item trail [1] { item cell [1]; }
			item spare [1];
			item same : Tallied [2] = (lead, trail).{in x; x};
			item local : Tallied [2] = (lead, trail).{in x; private item held = x; held};
			item nested : Tallied [*] = (lead, trail).{in x; (x, spare).{in y; y}};
			item other : Tallied [2] = (lead, trail).{in x; spare};
			item cells [2] = (lead, trail).{in x; x.cell};
		}
		item rack : Rack;
	}`
	ctx, idx := libraryShapeContext(t, src)
	rack := idx.LookupQualified("test::Rack")[0]
	want := map[string][]string{
		"lead":  {"same", "local", "nested"},
		"trail": {"same", "local", "nested"},
		"spare": {"nested", "other"},
	}
	for name, holders := range want {
		if got := ctx.holdingFeatures(rack, name); !slices.Equal(got, holders) {
			t.Errorf("holdingFeatures(Rack, %s) = %v, want %v", name, got, holders)
		}
	}

	for _, order := range [][]string{{"lead", "same"}, {"same", "lead"}} {
		ctx, idx := libraryShapeContext(t, src)
		rack := instantiateQualified(t, ctx, idx, "test::rack")
		tallied := idx.LookupQualified("test::Tallied")[0]
		for _, name := range order {
			if _, err := rack.GetFeatureValue(ctx, name); err != nil {
				t.Fatalf("reading %v: %s: %v", order, name, err)
			}
			if lead := readInstance(t, ctx, rack, "lead"); !ctx.instanceConforms(lead, tallied) {
				t.Errorf("reading %v, after %s lead is classified by %v, want Tallied",
					order, name, lead.classifiers)
			}
		}
	}
}

// Calcs returning through each other are analysed as one: a parameter one passes on only
// through the other's returns is passed on, whichever calc is met first and wherever the
// direct return stands beside the call.
func TestMutuallyRecursiveCalcsPassArgumentsThroughEachOther(t *testing.T) {
	for name, holdings := range map[string]string{
		"through_first_met": "item viaA : Tallied [1] = pickA(lead, trail, true); item viaB : Tallied [1] = pickB(lead, trail, true);",
		"through_last_met":  "item viaB : Tallied [1] = pickB(lead, trail, true); item viaA : Tallied [1] = pickA(lead, trail, true);",
	} {
		t.Run(name, func(t *testing.T) {
			ctx, idx := libraryShapeContext(t, `package test {
				private import ScalarValues::*;
				item def Gauge;
				item def Tallied { attribute tally : Integer = 7; }
				calc def pickA {
					in a : Gauge; in b : Gauge; in again : Boolean;
					return : Gauge = if again ? pickB(a, b, false) else a;
				}
				calc def pickB {
					in c : Gauge; in d : Gauge; in again : Boolean;
					return : Gauge = if again ? pickA(d, c, false) else d;
				}
				item def Rack {
					item lead : Gauge [1];
					item trail : Gauge [1];
					`+holdings+`
				}
			}`)
			rack := idx.LookupQualified("test::Rack")[0]
			for _, held := range []string{"lead", "trail"} {
				got := ctx.holdingFeatures(rack, held)
				sort.Strings(got)
				if want := []string{"viaA", "viaB"}; !slices.Equal(got, want) {
					t.Errorf("holdingFeatures(Rack, %s) = %v, want %v", held, got, want)
				}
			}
		})
	}
}

// The object a selected variant materialized is held like any other: a typed feature
// holding a variation's value classifies that object, which then carries the feature's
// own features and runs its behaviors, and stays the variant's object.
func TestSelectedVariantObjectIsClassifiedByTheFeatureHoldingIt(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package test {
		private import ScalarValues::*;
		state def Glowing {
			attribute cycles : Integer = 0;
			entry; then lit;
			state lit { entry action count { assign cycles := cycles + 1; } }
		}
		item def Engine { attribute cylinders : Integer = 4; }
		item def Tallied :> Engine { attribute tally : Integer = 7; exhibit state glow : Glowing; }
		item def Car {
			variation item engine : Engine [1] {
				variant item small : Engine;
				variant item big : Engine { :>> cylinders = 8; }
			}
		}
		item def Garage {
			item car : Car { :>> engine = engine::big; }
			item tallied : Tallied [1] = car.engine { attribute label : String = "kept"; }
		}
		item garage : Garage;
	}`)
	garage := instantiateQualified(t, ctx, idx, "test::garage")
	tallied := idx.LookupQualified("test::Tallied")[0]

	fv, err := garage.GetFeatureValue(ctx, "tallied")
	if err != nil {
		t.Fatalf("garage.tallied: %v", err)
	}
	if fv.Value.Kind != ValVariant || fv.Value.Variant().Name != "big" || fv.Value.Instance == 0 {
		t.Fatalf("garage.tallied = %+v, want the materialized big variant", fv.Value)
	}
	car := readInstance(t, ctx, garage, "car")
	engine, err := car.GetFeatureValue(ctx, "engine")
	if err != nil || engine.Value.Instance != fv.Value.Instance {
		t.Fatalf("car.engine = %+v, %v; want the same object tallied holds (%d)", engine.Value, err, fv.Value.Instance)
	}
	big := ctx.instances[fv.Value.Instance]
	if !ctx.instanceConforms(big, tallied) {
		t.Fatalf("the big variant's object is classified by %v, want Tallied", big.classifiers)
	}
	if got := readInt(t, ctx, big, "cylinders"); got != 8 {
		t.Fatalf("big.cylinders = %d, want the variant's 8", got)
	}
	if got := readInt(t, ctx, big, "tally"); got != 7 {
		t.Fatalf("big.tally = %d, want 7", got)
	}
	if label, err := big.GetFeatureValue(ctx, "label"); err != nil || label.Value.Str() != "kept" {
		t.Fatalf("big.label = %+v, %v; want the holding feature's \"kept\"", label, err)
	}
	glow, ok := big.Behavior("glow")
	if !ok || glow.State == nil || glow.State.stateData["cycles"].Const.Int != 1 {
		t.Fatal("the big variant's object runs no glow state machine")
	}
}

// A value none of whose types is comparable with the feature's type is refused: a number,
// an object of an unrelated definition, or one already classified by one, cannot become a Segment.
func TestIncomparableValueIsRefusedByTheFeatureType(t *testing.T) {
	cases := map[string]struct {
		decl string
		want string
	}{
		"integer": {
			decl: `item :>> edges [1] = (count); attribute count : Integer = 3;`,
			want: "cannot write 3 (an Integer) to a feature typed by Segment",
		},
		"unrelated_object": {
			decl: `item :>> edges [1] = (bolt); item bolt : Bolt;`,
			want: "to a feature typed by Segment",
		},
		"object_classified_by_an_unrelated_definition": {
			decl: `item :>> edges [1] = (raw); item raw [1]; item fastener : Bolt [1] = raw;`,
			want: "to a feature typed by Segment",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ctx, idx := libraryShapeContext(t, `package test {
				private import ScalarValues::*;
				item def Segment;
				item def Bolt;
				item def Loop { item edges : Segment [*]; }
				item def Broken :> Loop { `+tc.decl+` }
				item broken : Broken;
			}`)
			broken := instantiateQualified(t, ctx, idx, "test::broken")
			_, err := broken.GetFeatureValue(ctx, "edges")
			if !errors.Is(err, ErrTypeMismatch) {
				t.Fatalf("edges: %v, want ErrTypeMismatch", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("edges: %v, want %q in the message", err, tc.want)
			}
		})
	}
}

const heldObjectModel = `
	package test {
		item def Segment { item ends [2]; }
		item def Bolt;
		item def Rack {
			item slot : Segment [0..1] = raw;
			item raw [1];
			item loose [1];
		}
		item rack : Rack;
		item def Bin {
			item pair : Segment [2] = (spare, bolt);
			item spare [1];
			item bolt : Bolt;
		}
		item bin : Bin;
		attribute def Load { item seg : Segment; }
	}
`

// isUnclassified reports whether an object still is what it was declared alone.
func isUnclassified(inst *Instance) bool {
	_, ends := inst.FeatureValues["ends"]
	return len(inst.classifiers) == 0 && !ends
}

// A declared holding classifies its object and a probe undoes both; a refused value
// classifies nothing, and a write or message admits an object by what it already is.
func TestClassificationIsKeptOrUndoneWithTheChange(t *testing.T) {
	ctx, idx := libraryShapeContext(t, heldObjectModel)
	rack := instantiateQualified(t, ctx, idx, "test::rack")

	// Reading raw reads slot, which holds it, first.
	end := ctx.beginProbe()
	raw := readInstance(t, ctx, rack, "raw")
	if isUnclassified(raw) {
		t.Fatal("raw is not a Segment while it is held by slot")
	}
	end()
	if !isUnclassified(raw) {
		t.Fatalf("after the probe, raw is classified by %v with %v", raw.classifiers, sortedNames(raw.FeatureValues))
	}
	if rack.FeatureValues["slot"].Materialized {
		t.Fatal("after the probe, slot still holds its value")
	}

	bin := instantiateQualified(t, ctx, idx, "test::bin")
	if _, err := bin.GetFeatureValue(ctx, "pair"); !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("pair = %v, want ErrTypeMismatch: bolt is a Bolt", err)
	}
	for _, inst := range ctx.instances {
		if !isUnclassified(inst) {
			t.Fatalf("a refused value classified object %d, a %s, by %v", inst.ID, symbolText(inst.Type), inst.classifiers)
		}
	}

	loose := readInstance(t, ctx, rack, "loose")
	looseVal := Value{Kind: ValInstance, Instance: loose.ID}
	if err := rack.SetFeatureValue(ctx, "slot", looseVal); !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("SetFeatureValue(slot, loose) = %v, want ErrTypeMismatch: loose is no Segment", err)
	}
	load := idx.LookupQualified("test::Load")[0]
	if _, err := ctx.SignalMessage(load, map[string]Value{"seg": looseVal}, rack); !errors.Is(err, ErrSignalArgument) {
		t.Fatalf("SignalMessage(Load, seg = loose) = %v, want ErrSignalArgument", err)
	}
	if !isUnclassified(loose) {
		t.Fatal("a refused write or message classified its object")
	}

	raw = readInstance(t, ctx, rack, "raw")
	rawVal := Value{Kind: ValInstance, Instance: raw.ID}
	if isUnclassified(raw) {
		t.Fatal("raw is not a Segment once slot holds it")
	}
	if err := rack.SetFeatureValue(ctx, "slot", rawVal); err != nil {
		t.Fatalf("SetFeatureValue(slot, raw), a Segment now: %v", err)
	}
	if _, err := ctx.SignalMessage(load, map[string]Value{"seg": rawVal}, rack); err != nil {
		t.Fatalf("SignalMessage(Load, seg = raw), a Segment now: %v", err)
	}
}

// sortedNames is the feature names an object carries, sorted.
func sortedNames(values map[string]*FeatureValue) []string {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// A qualified name of an enclosing type's feature, written in a nested usage, reads the enclosing
// object's value (KerML 1.0 §7.4.4); one naming a declaration outside the object reads that.
func TestQualifiedOuterFeatureReadsTheEnclosingObject(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package test {
		private import ScalarValues::*;
		package Consts { attribute unit : Real = 7.0; }
		item def Frame {
			attribute span : Real;
			attribute gap : Real;
			attribute scale : Real = Consts::unit;
			item bar { attribute len : Real = Frame::span; }
			item strut { attribute len : Real = bar.len + Frame::gap; }
		}
		item frame : Frame { :>> span = 4.0; :>> gap = 1.0; }
		item other : Frame { :>> span = 10.0; :>> gap = 1.0; }
	}`)
	for _, tc := range []struct{ object, feature, want string }{
		{"test::frame", "bar.len", "4.0"},
		{"test::frame", "strut.len", "5.0"},
		{"test::frame", "scale", "7.0"},
		{"test::other", "bar.len", "10.0"},
		{"test::other", "strut.len", "11.0"},
	} {
		inst := instantiateQualified(t, ctx, idx, tc.object)
		path := strings.Split(tc.feature, ".")
		for _, step := range path[:len(path)-1] {
			inst = readInstance(t, ctx, inst, step)
		}
		fv, err := inst.GetFeatureValue(ctx, path[len(path)-1])
		if err != nil {
			t.Fatalf("%s.%s: %v", tc.object, tc.feature, err)
		}
		if got := FormatTraceValue(fv.HeldValue()); got != tc.want {
			t.Fatalf("%s.%s = %s, want %s", tc.object, tc.feature, got, tc.want)
		}
	}
}

// A feature valued by a chain over a collection holds the chain's values from every member,
// in member order, and the members' objects are made only when the chain is read.
func TestChainValueCollectsAcrossTheCollection(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package test {
		item def Corner;
		item def Side { item corners : Corner [2]; }
		item def Panel {
			item sides : Side [3];
			item corners : Corner [*] = sides.corners;
		}
		item panel : Panel;
	}`)
	panel := instantiateQualified(t, ctx, idx, "test::panel")
	if panel.FeatureValues["sides"].Materialized || panel.FeatureValues["corners"].Materialized {
		t.Fatal("sides or corners materialized on instantiation")
	}
	made := len(ctx.instances)

	corners, err := panel.GetFeatureValue(ctx, "corners")
	if err != nil {
		t.Fatalf("GetFeatureValue(corners): %v", err)
	}
	got := elementsOf(corners.HeldValue())
	if len(got) != 6 {
		t.Fatalf("corners holds %d objects, want 6", len(got))
	}
	if len(ctx.instances)-made != 9 {
		t.Fatalf("reading corners made %d objects, want 3 sides and 6 corners", len(ctx.instances)-made)
	}
	sides, err := panel.GetFeatureValue(ctx, "sides")
	if err != nil {
		t.Fatalf("GetFeatureValue(sides): %v", err)
	}
	for i, side := range elementsOf(sides.HeldValue()) {
		obj, _ := ctx.getInstance(side.Instance)
		fv, err := obj.GetFeatureValue(ctx, "corners")
		if err != nil {
			t.Fatalf("sides.%d.corners: %v", i+1, err)
		}
		for j, c := range elementsOf(fv.HeldValue()) {
			if want := got[2*i+j]; want.Instance != c.Instance {
				t.Fatalf("corners.%d = %s, want sides.%d.corners.%d = %s", 2*i+j+1, FormatValue(want), i+1, j+1, FormatValue(c))
			}
		}
	}
}

// A collection short of its lower bound is made up through its optional subsetters first, within
// their upper bounds, and through anonymous members only past them; a required one is never made twice.
func TestOptionalSubsetterFillsTheCollection(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package test {
		item def Side;
		item def Panel {
			item sides : Side [4];
			item left : Side [1] :> sides;
			item top : Side [0..1] :> sides;
			item spare : Side [0..2] :> sides;
		}
		item panel : Panel;
	}`)
	panel := instantiateQualified(t, ctx, idx, "test::panel")
	sides, err := panel.GetFeatureValue(ctx, "sides")
	if err != nil {
		t.Fatalf("GetFeatureValue(sides): %v", err)
	}
	if got := len(elementsOf(sides.HeldValue())); got != 4 {
		t.Fatalf("sides holds %d objects, want 4", got)
	}
	top := readInstance(t, ctx, panel, "top")
	spare, err := panel.GetFeatureValue(ctx, "spare")
	if err != nil {
		t.Fatalf("GetFeatureValue(spare): %v", err)
	}
	if got := len(elementsOf(spare.HeldValue())); got != 2 {
		t.Fatalf("spare holds %d objects, want 2", got)
	}
	els := elementsOf(sides.HeldValue())
	if left := readInstance(t, ctx, panel, "left"); els[0].Instance != left.ID || els[1].Instance != top.ID {
		t.Fatalf("sides = %s, want left (%d) then top (%d) first", FormatValue(sides.HeldValue()), left.ID, top.ID)
	}
}

// A probe filling an optional subsetter that already holds an object is undone whole: the
// objects made up go, and what the subsetter held before, written as it was, stays.
func TestProbeFillingASubsetterKeepsWhatItHeld(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package test {
		item def Side;
		item def Panel {
			item sides : Side [4];
			item left : Side [1] :> sides;
			item spare : Side [0..3] :> sides;
		}
		item panel : Panel;
		item extra : Side;
	}`)
	panel := instantiateQualified(t, ctx, idx, "test::panel")
	extra := instantiateQualified(t, ctx, idx, "test::extra")
	held := sequenceOf([]Value{{Kind: ValInstance, Instance: extra.ID}})
	if err := panel.SetFeatureValue(ctx, "spare", held); err != nil {
		t.Fatalf("SetFeatureValue(spare): %v", err)
	}
	before := *panel.FeatureValues["spare"]
	objects := len(ctx.instances)

	end := ctx.beginProbe()
	sides, err := panel.GetFeatureValue(ctx, "sides")
	if err != nil {
		t.Fatalf("GetFeatureValue(sides): %v", err)
	}
	if got := len(elementsOf(sides.HeldValue())); got != 4 {
		t.Fatalf("sides holds %d objects in the probe, want 4", got)
	}
	if got := len(elementsOf(panel.FeatureValues["spare"].HeldValue())); got != 3 {
		t.Fatalf("spare holds %d objects in the probe, want extra and two made up", got)
	}
	end()

	if got := len(ctx.instances); got != objects {
		t.Fatalf("the probe left %d objects behind", got-objects)
	}
	if after := *panel.FeatureValues["spare"]; !after.Materialized || !after.Written ||
		FormatValue(after.HeldValue()) != FormatValue(before.HeldValue()) {
		t.Fatalf("the probe changed spare from %s (written) to %s (materialized %t, written %t)",
			FormatValue(before.HeldValue()), FormatValue(after.HeldValue()), after.Materialized, after.Written)
	}
	if sides := panel.FeatureValues["sides"]; sides.Materialized {
		t.Fatalf("the probe left sides materialized as %s", FormatValue(sides.HeldValue()))
	}
}

// A collection read is one change: charged whole before any object is made, and a read
// the budget refuses leaves no object, filled subsetter or charged element behind.
func TestCollectionReadRefusedByTheBudgetFillsNothing(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package test {
		item def Side;
		item def Panel {
			item sides : Side [4];
			item left : Side [1] :> sides;
			item top : Side [0..1] :> sides;
		}
		item panel : Panel;
	}`)
	panel := instantiateQualified(t, ctx, idx, "test::panel")
	left := readInstance(t, ctx, panel, "left")
	objects := len(ctx.instances)

	end := ctx.beginRun()
	defer end()
	ctx.maxElements = 3
	_, err := panel.GetFeatureValue(ctx, "sides")
	if !errors.Is(err, ErrElementLimitExceeded) {
		t.Fatalf("sides under a budget of 3: %v, want ErrElementLimitExceeded", err)
	}
	if ctx.elements != 0 {
		t.Fatalf("a refused read left %d elements charged", ctx.elements)
	}
	if got := len(ctx.instances); got != objects {
		t.Fatalf("a refused read left %d objects behind", got-objects)
	}
	if top := panel.FeatureValues["top"]; len(elementsOf(top.HeldValue())) != 0 {
		t.Fatalf("a refused read filled top with %s", FormatValue(top.HeldValue()))
	}

	// The budget runs out at each point of the read in turn; each refusal leaves nothing behind.
	ctx.maxElements = 4
	maxSteps, refused := ctx.maxSteps, 0
	var sides *FeatureValue
	for extra := int64(0); sides == nil; extra++ {
		ctx.maxSteps = ctx.steps + extra
		fv, err := panel.GetFeatureValue(ctx, "sides")
		ctx.maxSteps = maxSteps
		if err == nil {
			sides = fv
			break
		}
		if !errors.Is(err, ErrStepLimitExceeded) {
			t.Fatalf("sides with %d steps: %v, want ErrStepLimitExceeded", extra, err)
		}
		refused++
		if got := len(ctx.instances); got != objects {
			t.Fatalf("a read refused after %d steps left %d objects behind", extra, got-objects)
		}
		if top := panel.FeatureValues["top"]; len(elementsOf(top.HeldValue())) != 0 {
			t.Fatalf("a read refused after %d steps filled top with %s", extra, FormatValue(top.HeldValue()))
		}
		if ctx.elements != 0 {
			t.Fatalf("a read refused after %d steps left %d elements charged", extra, ctx.elements)
		}
	}
	if refused < 3 {
		t.Fatalf("only %d step budgets refused the read; the objects it makes each take one", refused)
	}
	els := elementsOf(sides.HeldValue())
	if top := readInstance(t, ctx, panel, "top"); len(els) != 4 || els[0].Instance != left.ID || els[1].Instance != top.ID {
		t.Fatalf("sides = %s, want left (%d) then top (%d) first of four", FormatValue(sides.HeldValue()), left.ID, top.ID)
	}
}

// Types conform one way or the other (KerML 1.0 §8.4.4.4, binding connector type conformance):
// an object held by a Segment feature and a Line :> Segment feature is both, in either order;
// held by two siblings under one general type, it is refused, as the pilot warns for such a binding.
func TestClassifiersMustBeComparable(t *testing.T) {
	model := `package test {
		item def Common;
		item def Segment :> Common;
		item def Line :> Segment;
		item def Bolt :> Common;
		item def Rack { item raw [1]; }
		item rack : Rack;
	}`
	for _, order := range [][2]string{{"Segment", "Line"}, {"Line", "Segment"}} {
		t.Run(order[0]+"_then_"+order[1], func(t *testing.T) {
			ctx, idx := libraryShapeContext(t, model)
			raw := readInstance(t, ctx, instantiateQualified(t, ctx, idx, "test::rack"), "raw")
			for _, name := range order {
				typ := idx.LookupQualified("test::" + name)[0]
				if err := ctx.classify(raw, typ); err != nil {
					t.Fatalf("classify(raw, %s): %v", name, err)
				}
			}
			for _, name := range []string{"Common", "Segment", "Line"} {
				if typ := idx.LookupQualified("test::" + name)[0]; !ctx.instanceConforms(raw, typ) {
					t.Fatalf("raw, a Segment and a Line, is no %s", name)
				}
			}
		})
	}
	t.Run("siblings", func(t *testing.T) {
		ctx, idx := libraryShapeContext(t, model)
		raw := readInstance(t, ctx, instantiateQualified(t, ctx, idx, "test::rack"), "raw")
		bolt, segment := idx.LookupQualified("test::Bolt")[0], idx.LookupQualified("test::Segment")[0]
		if err := ctx.classify(raw, bolt); err != nil {
			t.Fatalf("classify(raw, Bolt): %v", err)
		}
		if ctx.canClassify(raw, segment) {
			t.Fatal("a Bolt can become a Segment, though neither conforms to the other")
		}
		if !ctx.canClassify(raw, idx.LookupQualified("test::Common")[0]) {
			t.Fatal("a Bolt cannot be held as the Common it already is")
		}
	})
}

// A classification the classifier's body makes impossible (two names of one redefined
// feature valued) is refused whole outside any probe: the object keeps what it had.
func TestRefusedClassificationLeavesTheObjectAsItWas(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package test {
		private import ScalarValues::*;
		item def Ring { attribute ringCost : Real; }
		item def Band :> Ring { attribute bandCost :>> ringCost = 400.0; attribute :>> ringCost = 500.0; }
		item def Rack { item raw [1]; }
		item rack : Rack;
	}`)
	rack := instantiateQualified(t, ctx, idx, "test::rack")
	raw := readInstance(t, ctx, rack, "raw")
	before := sortedNames(raw.FeatureValues)

	band := idx.LookupQualified("test::Band")[0]
	if err := ctx.classify(raw, band); !errors.Is(err, ErrConflictingRedefinition) {
		t.Fatalf("classify(raw, Band) = %v, want ErrConflictingRedefinition", err)
	}
	if len(raw.classifiers) != 0 {
		t.Fatalf("a refused classification left raw classified by %v", raw.classifiers)
	}
	if after := sortedNames(raw.FeatureValues); strings.Join(after, ",") != strings.Join(before, ",") {
		t.Fatalf("a refused classification changed raw's features from %v to %v", before, after)
	}
}

// An object classified by a type exhibiting a behavior runs it (KerML 1.0 §7.4.9), once,
// however many features hold it; a probe undoes the run with the holding.
func TestClassificationStartsTheClassifierBehaviors(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package test {
		private import ScalarValues::*;
		state def Glowing {
			attribute cycles : Integer = 0;
			entry; then lit;
			state lit { entry action count { assign cycles := cycles + 1; } }
		}
		part def Lamp { exhibit state glow : Glowing; }
		part def Room {
			part lamp : Lamp [1] = bulb;
			part spare : Lamp [1] = bulb;
			part bulb [1];
		}
		part room : Room;
	}`)
	room := instantiateQualified(t, ctx, idx, "test::room")

	end := ctx.beginProbe()
	bulb := readInstance(t, ctx, room, "lamp")
	if _, ok := bulb.Behavior("glow"); !ok {
		t.Fatal("bulb, a Lamp while lamp holds it, runs no glow behavior")
	}
	end()
	if len(bulb.behaviors) != 0 || len(ctx.objectBehaviors) != 0 {
		t.Fatalf("after the probe, bulb runs %d behaviors and the context %d", len(bulb.behaviors), len(ctx.objectBehaviors))
	}

	if bulb = readInstance(t, ctx, room, "lamp"); readInstance(t, ctx, room, "spare") != bulb {
		t.Fatal("lamp and spare hold different objects")
	}
	if len(bulb.behaviors) != 1 {
		t.Fatalf("bulb, held by two Lamp features, runs %d behaviors, want 1", len(bulb.behaviors))
	}
	behavior, ok := bulb.Behavior("glow")
	if !ok || behavior.State == nil {
		t.Fatal("bulb runs no glow state machine")
	}
	if got := behavior.State.stateData["cycles"].Const.Int; got != 1 {
		t.Fatalf("glow ran to cycles = %d, want 1", got)
	}
}

// A classification whose behaviors fail to start is undone whole: what a behavior wrote to
// a feature the object already had goes with the features and behaviors it added.
func TestRefusedClassificationUndoesWhatItsBehaviorsDid(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package test {
		private import ScalarValues::*;
		item def Counter { attribute hits : Integer = 0; }
		item def Tallied :> Counter {
			exhibit state tally {
				entry; then on;
				state on { entry action bump { assign hits := hits + 1; } }
			}
			exhibit state break {
				entry; then on;
				state on { entry action fail { assign hits := 1 / 0; } }
			}
		}
		item def Rack { item raw : Counter [1]; }
		item rack : Rack;
	}`)
	rack := instantiateQualified(t, ctx, idx, "test::rack")
	raw := readInstance(t, ctx, rack, "raw")
	hits, err := raw.GetFeatureValue(ctx, "hits")
	if err != nil {
		t.Fatalf("hits: %v", err)
	}
	before := FormatValue(hits.HeldValue())

	if err := ctx.classify(raw, idx.LookupQualified("test::Tallied")[0]); !errors.Is(err, ErrDivisionByZero) {
		t.Fatalf("classify(raw, Tallied) = %v, want ErrDivisionByZero", err)
	}
	if len(raw.classifiers) != 0 || len(raw.behaviors) != 0 || len(ctx.objectBehaviors) != 0 {
		t.Fatalf("a refused classification left raw classified by %v running %d behaviors (context: %d)",
			raw.classifiers, len(raw.behaviors), len(ctx.objectBehaviors))
	}
	if _, ok := raw.FeatureValues["tally"]; ok {
		t.Fatal("a refused classification left raw with the tally feature")
	}
	if after := FormatValue(hits.HeldValue()); raw.FeatureValues["hits"] != hits || after != before {
		t.Fatalf("a refused classification changed hits from %s to %s", before, after)
	}
}

// A variant a refused classification's behavior selected goes with it: a value variant
// stands for no object to abandon, so the selection itself is undone.
func TestRefusedClassificationUndoesTheVariantsItSelected(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package test {
		private import ScalarValues::*;
		item def Counter { attribute hits : Integer = 0; }
		item def Car {
			variation attribute power : Real {
				variant attribute strong = 150.0;
				variant attribute weak = 120.0;
			}
		}
		item def Tallied :> Counter {
			item car : Car [1] { attribute :>> power = power::strong; }
			exhibit state tally {
				entry; then on;
				state on { entry action bump { assign hits := car.power / hits; } }
			}
		}
		item def Rack { item raw : Counter [1]; item own : Car [1] { attribute :>> power = power::weak; } }
		item rack : Rack;
	}`)
	rack := instantiateQualified(t, ctx, idx, "test::rack")
	raw := readInstance(t, ctx, rack, "raw")
	own := readInstance(t, ctx, rack, "own")
	if _, err := own.GetFeatureValue(ctx, "power"); err != nil {
		t.Fatalf("own.power: %v", err)
	}
	before := maps.Clone(ctx.selectedVariants)
	if got := before[variantSelection{owner: own.ID, variation: "power"}]; got != "weak" {
		t.Fatalf("own selected %q before classifying, want weak", got)
	}

	if err := ctx.classify(raw, idx.LookupQualified("test::Tallied")[0]); !errors.Is(err, ErrDivisionByZero) {
		t.Fatalf("classify(raw, Tallied) = %v, want ErrDivisionByZero", err)
	}
	if !maps.Equal(ctx.selectedVariants, before) {
		t.Fatalf("a refused classification changed the variants selected from %v to %v", before, ctx.selectedVariants)
	}
}

// A collection is classified whole: when a later object's classification fails, the objects
// classified before it are left as they were, behaviors, writes and all.
func TestRefusedCollectionClassificationUndoesTheEarlierObjects(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package test {
		private import ScalarValues::*;
		item def Counter { attribute hits : Integer; }
		item def Tallied :> Counter {
			exhibit state tally {
				entry; then on;
				state on { entry action bump { assign hits := 10 / hits; } }
			}
		}
		item def Rack {
			item lead : Counter [1] { :>> hits = 1; }
			item trail : Counter [1] { :>> hits = 0; }
			item tallied : Tallied [2] = (lead, trail);
		}
		item rack : Rack;
	}`)
	rack := instantiateQualified(t, ctx, idx, "test::rack")

	// lead is classified before trail's behavior fails; the read materializes it all the same.
	if _, err := rack.GetFeatureValue(ctx, "tallied"); !errors.Is(err, ErrDivisionByZero) {
		t.Fatalf("rack.tallied = %v, want ErrDivisionByZero", err)
	}
	held := rack.FeatureValues["lead"].HeldValue()
	lead, ok := ctx.getInstance(held.Instance)
	if held.Kind != ValInstance || !ok {
		t.Fatalf("lead = %s, want the object the failed read materialized", FormatValue(held))
	}
	if len(lead.classifiers) != 0 || len(lead.behaviors) != 0 || len(ctx.objectBehaviors) != 0 {
		t.Fatalf("a refused collection left lead classified by %v running %d behaviors (context: %d)",
			lead.classifiers, len(lead.behaviors), len(ctx.objectBehaviors))
	}
	if _, ok := lead.FeatureValues["tally"]; ok {
		t.Fatal("a refused collection left lead with the tally feature")
	}
	if hits := lead.FeatureValues["hits"]; hits.Written {
		t.Fatalf("a refused collection left lead.hits written to %s", FormatValue(hits.HeldValue()))
	}
}

// The objects a classification's behaviors make go with it when it is undone: a refused
// collection leaves the context holding what it held, however often the read is retried.
func TestRefusedCollectionClassificationAbandonsWhatItsBehaviorsMade(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package test {
		private import ScalarValues::*;
		item def Counter { attribute hits : Integer; }
		item def Gauge { attribute reading : Integer = 1; }
		item def Tallied :> Counter {
			item gauge : Gauge [1];
			exhibit state tally {
				entry; then on;
				state on { entry action bump { assign hits := gauge.reading / hits; } }
			}
		}
		item def Rack {
			item lead : Counter [1] { :>> hits = 1; }
			item trail : Counter [1] { :>> hits = 0; }
			item tallied : Tallied [2] = (lead, trail);
		}
		item rack : Rack;
	}`)
	rack := instantiateQualified(t, ctx, idx, "test::rack")

	// The read materializes rack's lead and trail, which stay; lead's gauge, made by the
	// tally lead ran as a Tallied, does not.
	var held []int64
	var occurrences map[*symbols.Symbol]int64
	for attempt := 1; attempt <= 3; attempt++ {
		if _, err := rack.GetFeatureValue(ctx, "tallied"); !errors.Is(err, ErrDivisionByZero) {
			t.Fatalf("attempt %d: rack.tallied = %v, want ErrDivisionByZero", attempt, err)
		}
		if attempt == 1 {
			held, occurrences = ctx.InstanceIDs(), maps.Clone(ctx.occurrences)
			if len(held) != 3 {
				t.Fatalf("a refused collection left the objects %v, want rack, lead and trail only", held)
			}
		}
		if after := ctx.InstanceIDs(); !slices.Equal(after, held) {
			t.Fatalf("attempt %d: a refused collection changed the objects held from %v to %v", attempt, held, after)
		}
		if !maps.Equal(ctx.occurrences, occurrences) {
			t.Fatalf("attempt %d: a refused collection changed the occurrences held to %v", attempt, ctx.occurrences)
		}
		if len(ctx.created) != len(held) || len(ctx.objectBehaviors) != 0 {
			t.Fatalf("attempt %d: a refused collection left %d creations and %d behaviors",
				attempt, len(ctx.created), len(ctx.objectBehaviors))
		}
		for _, name := range []string{"lead", "trail"} {
			inst, ok := ctx.getInstance(rack.FeatureValues[name].HeldValue().Instance)
			if !ok {
				t.Fatalf("attempt %d: %s holds no object", attempt, name)
			}
			if _, ok := inst.FeatureValues["gauge"]; ok || len(inst.classifiers) != 0 || len(inst.behaviors) != 0 {
				t.Fatalf("attempt %d: a refused collection left %s classified by %v running %d behaviors",
					attempt, name, inst.classifiers, len(inst.behaviors))
			}
		}
	}
}

// A classifier redefining a feature the object carries refines it (KerML 1.0 §7.3.4.5): the
// object reads the redefinition's default, type and multiplicity; what it held is kept where
// the redefinition admits it, and refused whole where it does not.
func TestNarrowerClassifierRefinesTheCarriedFeatures(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package test {
		private import ScalarValues::*;
		item def Engine;
		item def V8 :> Engine;
		item def Car {
			attribute doors : Integer default = 4;
			attribute tags : String [0..2] default = ("a", "b");
			item engine : Engine [1];
		}
		item def Coupe :> Car { attribute :>> doors default = 2; item :>> engine : V8; }
		item def Tagged :> Car { attribute :>> tags : String [1]; }
		item def Rack {
			item raw : Car [1];
			item sporty : Car [1] { attribute :>> doors default = 3; }
			item coupe : Coupe [1] = raw;
			item coupe2 : Coupe [1] = sporty;
		}
		item rack : Rack;
		item car : Car;
	}`)
	rack := instantiateQualified(t, ctx, idx, "test::rack")
	raw := readInstance(t, ctx, rack, "raw")
	coupe := idx.LookupQualified("test::Coupe")[0]
	if !ctx.instanceConforms(raw, coupe) {
		t.Fatalf("raw held by coupe is classified by %v, want Coupe", raw.classifiers)
	}
	if doors := readInt(t, ctx, raw, "doors"); doors != 2 {
		t.Errorf("raw.doors = %d as a Coupe, want Coupe's default 2", doors)
	}
	if feat := raw.FeatureValues["doors"].Feature; feat.OwnerType != coupe {
		t.Errorf("raw.doors reads %s's declaration, want Coupe's", feat.OwnerType.Name)
	}
	if feat := raw.FeatureValues["tags"].Feature; feat.OwnerType == coupe {
		t.Error("raw.tags, which Coupe does not redefine, reads a declaration of Coupe")
	}
	engine := readInstance(t, ctx, raw, "engine")
	if v8 := idx.LookupQualified("test::V8")[0]; !ctx.instanceConforms(engine, v8) {
		t.Errorf("raw.engine held by a Coupe's engine : V8 is classified by %v, want V8", engine.classifiers)
	}

	// A usage's own redefinition is not refined by a classifier redefining what it redefines.
	sporty := readInstance(t, ctx, rack, "sporty")
	if doors := readInt(t, ctx, sporty, "doors"); doors != 3 {
		t.Errorf("sporty.doors = %d as a Coupe, want the usage's own default 3", doors)
	}

	// A written value stays through the refinement, read by the refining declaration.
	car := instantiateQualified(t, ctx, idx, "test::car")
	if err := car.SetFeatureValue(ctx, "doors", integerValue(5)); err != nil {
		t.Fatalf("write car.doors: %v", err)
	}
	if err := ctx.classify(car, coupe); err != nil {
		t.Fatalf("classify(car, Coupe): %v", err)
	}
	if doors := car.FeatureValues["doors"]; !doors.Written || doors.HeldValue().Const.Int != 5 || doors.Feature.OwnerType != coupe {
		t.Errorf("car.doors = %s (written %t, declared by %s) after classifying, want the written 5 read by Coupe's declaration",
			FormatValue(doors.HeldValue()), doors.Written, doors.Feature.OwnerType.Name)
	}

	// A held value the refined declaration does not admit refuses the classification whole.
	if err := car.SetFeatureValue(ctx, "tags", sequenceOf([]Value{NewStringValue("x"), NewStringValue("y")})); err != nil {
		t.Fatalf("write car.tags: %v", err)
	}
	if err := ctx.classify(car, idx.LookupQualified("test::Tagged")[0]); !errors.Is(err, ErrMultiplicityViolation) {
		t.Fatalf("classify(car, Tagged) = %v, want ErrMultiplicityViolation for two values in tags [1]", err)
	}
	if len(car.classifiers) != 1 || car.FeatureValues["tags"].Feature.OwnerType == idx.LookupQualified("test::Tagged")[0] {
		t.Errorf("a refused classification left car classified by %v reading tags from %s",
			car.classifiers, car.FeatureValues["tags"].Feature.OwnerType.Name)
	}
}

// An object's relationships are those of every type classifying it — declared type first,
// then each classifier — and one a classifier inherits from a type already counted is not listed twice.
func TestRelationshipsComeFromEveryTypeOfTheObject(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package test {
		private import ScalarValues::*;
		item def Base {
			attribute a : Integer;
			attribute b : Integer = 1;
			bind a = b;
			attribute xs : Integer [*];
			attribute x1 : Integer [*] :> xs = (1);
			item p; item q;
			connect p to q;
		}
		item def Wide :> Base {
			attribute c : Integer = 2;
			bind a = c;
			attribute x2 : Integer [*] :> xs = (2);
			item r;
			connect q to r;
		}
		item def Rack { item raw : Base [1]; }
		item rack : Rack;
	}`)
	raw := readInstance(t, ctx, instantiateQualified(t, ctx, idx, "test::rack"), "raw")
	if err := ctx.classify(raw, idx.LookupQualified("test::Wide")[0]); err != nil {
		t.Fatalf("classify(raw, Wide): %v", err)
	}
	bindings := ctx.bindingsOf(raw, "a")
	if len(bindings) != 2 || bindings[0].Ends[1].Path != "b" || bindings[1].Ends[1].Path != "c" {
		t.Fatalf("bindings of a = %+v, want Base's a = b then Wide's a = c", bindings)
	}
	var subsetters []string
	for _, feat := range ctx.subsettingFeaturesOf(raw, "xs") {
		subsetters = append(subsetters, feat.Name)
	}
	if strings.Join(subsetters, ",") != "x1,x2" {
		t.Fatalf("subsetters of xs = %v, want x1 then x2", subsetters)
	}
	var conns []string
	for _, conn := range ctx.connectionsOf(raw) {
		if ends := strings.Join(conn.Ends, "-"); ends == "p-q" || ends == "q-r" {
			conns = append(conns, ends)
		}
	}
	if strings.Join(conns, ",") != "p-q,q-r" {
		t.Fatalf("connections = %v, want Base's p-q then Wide's q-r, each once", conns)
	}
	if anon := ctx.anonymousConnectorsOf(raw.types()); len(anon) != 2 {
		t.Fatalf("anonymous connectors = %d, want Base's and Wide's", len(anon))
	}
}
