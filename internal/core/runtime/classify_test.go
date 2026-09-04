package runtime

import (
	"errors"
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

// The values of a feature are instances of its type (KerML 1.0 §7.3.4.1): an
// untyped member listed as a value of `edges : Segment` is the same object,
// now a Segment carrying its features, whichever of the two is read first; and
// holding it again adds nothing.
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

// A value none of whose types is comparable with the feature's type is refused:
// a number or an object of an unrelated definition cannot become a Segment.
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
		private import ScalarValues::*;
		item def Segment { item ends [2]; }
		item def Rack {
			item slot : Segment [0..1];
			item raw [1];
			item spare [1];
		}
		item rack : Rack;
		attribute def Load { item seg : Segment; attribute n : Integer; }
	}
`

// isUnclassified reports whether an object still is what it was declared alone.
func isUnclassified(inst *Instance) bool {
	_, ends := inst.FeatureValues["ends"]
	return len(inst.classifiers) == 0 && !ends
}

// Classifying an object is part of the change that holds it: a probe undoes it
// with the write, and a value refused, or a message refused for another of its
// arguments, classifies nothing.
func TestClassificationIsKeptOrUndoneWithTheChange(t *testing.T) {
	ctx, idx := libraryShapeContext(t, heldObjectModel)
	rack := instantiateQualified(t, ctx, idx, "test::rack")
	raw, spare := readInstance(t, ctx, rack, "raw"), readInstance(t, ctx, rack, "spare")
	rawVal := Value{Kind: ValInstance, Instance: raw.ID}
	spareVal := Value{Kind: ValInstance, Instance: spare.ID}

	end := ctx.beginProbe()
	if err := rack.SetFeatureValue(ctx, "slot", rawVal); err != nil {
		t.Fatalf("SetFeatureValue(slot) under a probe: %v", err)
	}
	if isUnclassified(raw) {
		t.Fatal("raw is not a Segment while it is held by slot")
	}
	end()
	if !isUnclassified(raw) {
		t.Fatalf("after the probe, raw is classified by %v with %v", raw.classifiers, sortedNames(raw.FeatureValues))
	}
	if rack.FeatureValues["slot"].Materialized {
		t.Fatal("after the probe, slot is still written")
	}

	two := sequenceOf([]Value{rawVal, spareVal})
	if err := rack.SetFeatureValue(ctx, "slot", two); !errors.Is(err, ErrMultiplicityViolation) {
		t.Fatalf("SetFeatureValue(slot, two objects) = %v, want ErrMultiplicityViolation", err)
	}
	if !isUnclassified(raw) || !isUnclassified(spare) {
		t.Fatal("a refused write classified its objects")
	}

	load := idx.LookupQualified("test::Load")[0]
	args := map[string]Value{"seg": rawVal, "n": NewStringValue("many")}
	if _, err := ctx.SignalMessage(load, args, rack); !errors.Is(err, ErrSignalArgument) {
		t.Fatalf("SignalMessage(Load) = %v, want ErrSignalArgument", err)
	}
	if !isUnclassified(raw) {
		t.Fatal("a refused message classified the object of an argument it admitted")
	}
	args["n"] = integerValue(2)
	if _, err := ctx.SignalMessage(load, args, rack); err != nil {
		t.Fatalf("SignalMessage(Load): %v", err)
	}
	if isUnclassified(raw) {
		t.Fatal("raw is not a Segment once carried by Load.seg")
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

// A qualified name of a feature of an enclosing type, written in a nested
// usage, reads the enclosing object's value (KerML 1.0 §7.4.4: a feature is
// evaluated on the object it is a feature of), while a qualified name of a
// declaration outside the object reads that declaration.
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

// A feature valued by a chain over a collection holds the chain's values from
// every member, in member order, and the members' objects are made only when
// the chain is read.
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

// A collection short of its lower bound is made up through its optional
// subsetting features first, each within its upper bound, and through anonymous
// members only past them; a required subsetter is never made twice.
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
