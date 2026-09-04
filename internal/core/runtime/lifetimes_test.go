package runtime

import (
	"errors"
	"strings"
	"testing"
)

// lifetimeFixture instantiates test::<name> of src and answers the object and a
// way to invoke test::<calc> over values, for the lifetime tests below.
func lifetimeFixture(t *testing.T, src string) (instantiate func(name string) *Instance, invoke func(calc string, args ...Value) (Value, error), ctx *Context) {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	instantiate = func(name string) *Instance {
		matches := idx.LookupQualified("test::" + name)
		if len(matches) != 1 {
			t.Fatalf("test::%s: %d matching symbols, want 1", name, len(matches))
		}
		inst, err := ctx.Instantiate(matches[0])
		if err != nil {
			t.Fatalf("Instantiate(%s): %v", name, err)
		}
		return inst
	}
	invoke = func(calc string, args ...Value) (Value, error) {
		sym, scope := calcByName(t, idx.DocumentRoot("<test>"), "test", calc)
		return ctx.InvokeCalc(sym, args, scope)
	}
	return instantiate, invoke, ctx
}

func objectValue(inst *Instance) Value { return Value{Kind: ValInstance, Instance: inst.ID} }

const lifetimeModel = `
	package test {
		private import ScalarValues::*;
		private import OccurrenceFunctions::*;
		part def Widget { attribute n : Integer = 1; }
		part def Bench { part w : Widget; part spare : Widget; }
		part def Rover {
			exhibit state modes { entry; then idle; state idle; }
		}
		calc def Destroy { in b : Bench[0..1]; return : Bench[0..1] = destroy(b); }
		calc def DestroyRover { in r : Rover; return : Rover = destroy(r); }
		calc def During { in o : Bench; return : Boolean = isDuring(o); }
		calc def WidgetDuring { in w : Widget; return : Boolean = isDuring(w); }
		calc def Create { in b : Bench; return : Bench = create(b); }
		calc def CreateSpare { in b : Bench; return : Widget = create(b.spare); }
		calc def AddAt { in g : Widget[0..*] ordered nonunique; in w : Widget; in i : Positive; return : Widget[0..*] = addNewAt(g, w, i); }
	}
`

// TestOccurrenceLifeBeginsWithWhole: an instantiated object is alive from its
// materialization, and a part of it began when its whole did.
func TestOccurrenceLifeBeginsWithWhole(t *testing.T) {
	instantiate, invoke, ctx := lifetimeFixture(t, lifetimeModel)
	bench := instantiate("Bench")
	whole, ok := ctx.OccurrenceLife(bench.ID)
	if !ok || !whole.Alive() {
		t.Fatalf("OccurrenceLife(bench) = %v, %v; want alive", whole, ok)
	}
	fv, err := bench.GetFeatureValue(ctx, "w")
	if err != nil {
		t.Fatal(err)
	}
	id, _ := fv.HeldValue().Object()
	part, ok := ctx.OccurrenceLife(id)
	if !ok || part.Began != whole.Began || part.Ended != 0 {
		t.Errorf("OccurrenceLife(w) = %v, %v; want began at %d, alive", part, ok, whole.Began)
	}
	if got, err := invoke("During", objectValue(bench)); err != nil || !got.Const.Bool {
		t.Errorf("isDuring(bench) = %s, %v; want true", FormatValue(got), err)
	}
	if _, ok := ctx.OccurrenceLife(bench.ID + 100); ok {
		t.Error("OccurrenceLife of an unregistered identity answered one")
	}
}

// TestDestroyEndsPortionsAndRefusesReads: `destroy` ends the object and the parts
// it holds, after which no feature of theirs is read or written, and none is
// destroyed again.
func TestDestroyEndsPortionsAndRefusesReads(t *testing.T) {
	instantiate, invoke, ctx := lifetimeFixture(t, lifetimeModel)
	bench := instantiate("Bench")
	fv, err := bench.GetFeatureValue(ctx, "w")
	if err != nil {
		t.Fatal(err)
	}
	partID, _ := fv.HeldValue().Object()
	part, _ := ctx.getInstance(partID)

	got, err := invoke("Destroy", objectValue(bench))
	if err != nil || !valueIdentical(got, objectValue(bench)) {
		t.Fatalf("destroy(bench) = %s, %v; want bench", FormatValue(got), err)
	}
	for _, inst := range []*Instance{bench, part} {
		l, ok := ctx.OccurrenceLife(inst.ID)
		if !ok || !l.Destroyed || l.Alive() {
			t.Errorf("OccurrenceLife(#%d) = %v; want destroyed", inst.ID, l)
		}
	}
	if _, err := bench.GetFeatureValue(ctx, "w"); !errors.Is(err, ErrOccurrenceDestroyed) {
		t.Errorf("read of a destroyed object: %v, want %v", err, ErrOccurrenceDestroyed)
	}
	if _, err := part.GetFeatureValue(ctx, "n"); !errors.Is(err, ErrOccurrenceDestroyed) {
		t.Errorf("read of a destroyed portion: %v, want %v", err, ErrOccurrenceDestroyed)
	}
	if err := part.SetFeatureValue(ctx, "n", constInt(2)); !errors.Is(err, ErrOccurrenceDestroyed) {
		t.Errorf("write to a destroyed portion: %v, want %v", err, ErrOccurrenceDestroyed)
	}
	if got, err := invoke("WidgetDuring", objectValue(part)); err != nil || got.Const.Bool {
		t.Errorf("isDuring(destroyed part) = %s, %v; want false", FormatValue(got), err)
	}
	if _, err := invoke("Destroy", objectValue(bench)); !errors.Is(err, ErrOccurrenceDestroyed) {
		t.Errorf("second destroy: %v, want %v", err, ErrOccurrenceDestroyed)
	}
	if got, err := invoke("Destroy", nullValue()); err != nil || !isEmptyValue(got) {
		t.Errorf("destroy() = %s, %v; want nothing", FormatValue(got), err)
	}
}

// TestDestroyRefusedWhilePerforming: an object whose exhibited state machine has
// not completed cannot end; the refusal names the behavior under way.
func TestDestroyRefusedWhilePerforming(t *testing.T) {
	instantiate, invoke, ctx := lifetimeFixture(t, lifetimeModel)
	rover := instantiate("Rover")
	_, err := invoke("DestroyRover", objectValue(rover))
	if !errors.Is(err, ErrOccurrenceLifetime) || !strings.Contains(err.Error(), "under way") {
		t.Fatalf("destroy(rover) = %v; want %v naming the behavior under way", err, ErrOccurrenceLifetime)
	}
	if l, _ := ctx.OccurrenceLife(rover.ID); !l.Alive() {
		t.Errorf("OccurrenceLife(rover) = %v after the refusal; want alive", l)
	}
}

// TestCreateOnlyWhatTheCallReaches: `create` begins an object the call first
// reaches and refuses one that began before it; `addNewAt` refuses an index past
// one beyond the group's end before it creates anything.
func TestCreateOnlyWhatTheCallReaches(t *testing.T) {
	instantiate, invoke, ctx := lifetimeFixture(t, lifetimeModel)
	bench := instantiate("Bench")
	if _, err := invoke("Create", objectValue(bench)); !errors.Is(err, ErrOccurrenceLifetime) {
		t.Errorf("create(bench) = %v; want %v", err, ErrOccurrenceLifetime)
	}
	whole, _ := ctx.OccurrenceLife(bench.ID)
	got, err := invoke("CreateSpare", objectValue(bench))
	if err != nil {
		t.Fatalf("create(bench.spare): %v", err)
	}
	spareID, _ := got.Object()
	spare, _ := ctx.OccurrenceLife(spareID)
	if !spare.Alive() || spare.Began <= whole.Began {
		t.Errorf("OccurrenceLife(spare) = %v; want begun after the bench at %d", spare, whole.Began)
	}
	if _, err := invoke("CreateSpare", objectValue(bench)); !errors.Is(err, ErrOccurrenceLifetime) {
		t.Errorf("second create(bench.spare) = %v; want %v", err, ErrOccurrenceLifetime)
	}

	widget := instantiate("Widget")
	before, _ := ctx.OccurrenceLife(widget.ID)
	_, err = invoke("AddAt", nullValue(), objectValue(widget), constInt(2))
	if !errors.Is(err, ErrIndexOutOfRange) {
		t.Errorf("addNewAt((), w, 2) = %v; want %v", err, ErrIndexOutOfRange)
	}
	if after, _ := ctx.OccurrenceLife(widget.ID); after != before {
		t.Errorf("OccurrenceLife(w) = %v after the refusal; want %v unchanged", after, before)
	}
}

// TestLifetimeChangesRollBackWithTheJournal: a destruction inside a journal that
// rolls back leaves the object alive, as the feature values it restores are.
func TestLifetimeChangesRollBackWithTheJournal(t *testing.T) {
	instantiate, invoke, ctx := lifetimeFixture(t, lifetimeModel)
	bench := instantiate("Bench")
	before, _ := ctx.OccurrenceLife(bench.ID)
	_, rollback := ctx.beginJournal()
	if _, err := invoke("Destroy", objectValue(bench)); err != nil {
		t.Fatal(err)
	}
	if l, _ := ctx.OccurrenceLife(bench.ID); !l.Destroyed {
		t.Fatalf("OccurrenceLife(bench) = %v inside the journal; want destroyed", l)
	}
	rollback()
	if after, _ := ctx.OccurrenceLife(bench.ID); after != before {
		t.Errorf("OccurrenceLife(bench) = %v after rollback; want %v", after, before)
	}
	if _, err := bench.GetFeatureValue(ctx, "w"); err != nil {
		t.Errorf("read after rollback: %v", err)
	}
}

// TestAbandonedObjectsLoseTheirLives: an object a failed creation abandons
// leaves no lifetime behind.
func TestAbandonedObjectsLoseTheirLives(t *testing.T) {
	instantiate, _, ctx := lifetimeFixture(t, lifetimeModel)
	mark := len(ctx.created)
	widget := instantiate("Widget")
	ctx.abandonInstancesSince(mark)
	if _, ok := ctx.OccurrenceLife(widget.ID); ok {
		t.Error("OccurrenceLife of an abandoned object answered one")
	}
}
