package solve

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// pinnedQuery translates one constraint of the panel fixture with the values the
// model declares for the part fixed.
func pinnedQuery(t *testing.T, element string) (*runtime.Context, *symbols.Index, *Query) {
	t.Helper()
	ctx, idx := fixtureFile(t, "panel_pins.sysml")
	panel := symbolNamed(t, idx, "test::Panel")
	pins, unfixed := Fixed(ctx, panel, nil)
	if len(unfixed) != 0 {
		t.Fatalf("declared values could not be read: %+v", unfixed)
	}
	sym := symbolNamed(t, idx, element)
	q, err := ConstraintWith(ctx, sym, sym.OwnerScope, pins)
	if err != nil {
		t.Fatalf("translate %s with fixed values: %v", element, err)
	}
	return ctx, idx, q
}

// TestGoldenWithFixedValues locks the script written for a query whose fixed
// values the model declares, translation and writer together.
func TestGoldenWithFixedValues(t *testing.T) {
	cases := map[string]string{
		"panel_fits_pinned.smt2":   "test::Panel::fits",
		"panel_speed_pinned.smt2":  "test::Panel::speedIsBounded",
		"panel_finish_pinned.smt2": "test::Panel::polishedIsWide",
	}
	for golden, element := range cases {
		t.Run(golden, func(t *testing.T) {
			_, _, q := pinnedQuery(t, element)
			compareGolden(t, golden, Script(q))
		})
	}
}

// TestFixedReadsTheValuesTheModelDeclares: every attribute stating a value is
// read, and one stating none stays free rather than being fixed to nothing.
func TestFixedReadsTheValuesTheModelDeclares(t *testing.T) {
	ctx, idx := fixtureFile(t, "panel_pins.sysml")
	pins, unfixed := Fixed(ctx, symbolNamed(t, idx, "test::Panel"), nil)
	if len(unfixed) != 0 {
		t.Fatalf("declared values could not be read: %+v", unfixed)
	}
	got := map[string]runtime.ValueKind{}
	for _, p := range pins {
		got[p.Name] = p.Value.Kind
		if p.Source != PinDeclared {
			t.Errorf("%s came from %s, want a declared value", p.Name, p.Source)
		}
	}
	want := map[string]runtime.ValueKind{
		"width":    runtime.ValConst,
		"finish":   runtime.ValEnumLiteral,
		"maxSpeed": runtime.ValQuantity,
		"label":    runtime.ValString,
	}
	for name, kind := range want {
		if got[name] != kind {
			t.Errorf("%s was read as %v, want %v", name, got[name], kind)
		}
	}
	if _, fixed := got["height"]; fixed {
		t.Error("height states no value, so nothing fixes it")
	}
}

// TestNoPinsTranslateAsBefore: a translation given no fixed values writes the
// very script the plain translation writes, which is what keeps every existing
// golden and verdict standing.
func TestNoPinsTranslateAsBefore(t *testing.T) {
	ctx, idx := fixtureFile(t, "panel_pins.sysml")
	for _, element := range []string{"test::Panel::fits", "test::Panel::speedIsBounded"} {
		sym := symbolNamed(t, idx, element)
		plain, err := Constraint(ctx, sym, sym.OwnerScope)
		if err != nil {
			t.Fatalf("translate %s: %v", element, err)
		}
		with, err := ConstraintWith(ctx, sym, sym.OwnerScope, nil)
		if err != nil {
			t.Fatalf("translate %s with no fixed values: %v", element, err)
		}
		if Script(plain) != Script(with) {
			t.Errorf("%s wrote a different script with no values fixed:\n%s\n%s", element, Script(plain), Script(with))
		}
		if with.Fixes() || len(with.Pinned) != 0 {
			t.Errorf("%s fixes %d values with none given", element, len(with.Pinned))
		}
	}
}

// TestPinnedValueIsAssertedAndReported: the value the model declares becomes an
// equality the query asserts, at the position the pin records, and the variable
// it fixes is no longer one the solver is free to choose.
func TestPinnedValueIsAssertedAndReported(t *testing.T) {
	_, _, q := pinnedQuery(t, "test::Panel::fits")
	if len(q.Pinned) != 1 || q.Pinned[0].Var.Name != "test::Panel::width" {
		t.Fatalf("fixed %+v, want the declared width alone:\n%s", q.Pinned, Script(q))
	}
	pinned := q.Pinned[0]
	if pinned.Value != "4" || pinned.Source != PinDeclared {
		t.Errorf("width was fixed to %q from %s, want 4 declared", pinned.Value, pinned.Source)
	}
	if at := q.Assertions[pinned.Index]; at.From.Role != RolePinned {
		t.Errorf("assertion %d is %s, want the fixed value's own", pinned.Index, at.From.Role)
	}
	for _, v := range q.Free() {
		if v.Name == "test::Panel::width" {
			t.Error("width is fixed, so it is not left for the solver to choose")
		}
	}
	if !strings.Contains(Script(q), "(assert (= |test::Panel::width| 4))") {
		t.Errorf("the script does not fix the width:\n%s", Script(q))
	}
}

// TestPinnedQuantityIsScaledExactly: a value declared in km/h fixes the very
// magnitude the same quantity written in the condition scales to — 5.4 [km/h] is
// exactly 1.5 [m/s], with no rounding of the binary float it was read as.
func TestPinnedQuantityIsScaledExactly(t *testing.T) {
	_, _, q := pinnedQuery(t, "test::Panel::speedIsBounded")
	script := Script(q)
	if !strings.Contains(script, "(assert (= |test::Panel::maxSpeed| 1.5))") {
		t.Errorf("the fixed speed is not exactly 1.5 in base units:\n%s", script)
	}
	if len(q.Pinned) != 1 || q.Pinned[0].Value != "5.4 [km/h]" {
		t.Errorf("fixed %+v, want the speed as it was declared", q.Pinned)
	}
}

// TestPinnedEnumerationNamesItsConstructor: an enumeration literal fixes the
// value of the datatype sort the writer declares for it.
func TestPinnedEnumerationNamesItsConstructor(t *testing.T) {
	_, _, q := pinnedQuery(t, "test::Panel::polishedIsWide")
	script := Script(q)
	if !strings.Contains(script, "(assert (= |test::Panel::finish| |test::Finish::polished|))") {
		t.Errorf("the fixed finish is not the datatype's own value:\n%s", script)
	}
}

// TestPinnedStringIsAsserted: a declared string fixes the string the writer
// quotes.
func TestPinnedStringIsAsserted(t *testing.T) {
	_, _, q := pinnedQuery(t, "test::Panel::labelled")
	if !strings.Contains(Script(q), `(assert (= |test::Panel::label| "left"))`) {
		t.Errorf("the fixed label is not asserted:\n%s", Script(q))
	}
}

// TestPinnedValueNoConditionReadsIsReported: a value fixed for a feature no
// condition of the element reads is reported as unread rather than dropped, and
// asserts nothing about conditions that say nothing about it.
func TestPinnedValueNoConditionReadsIsReported(t *testing.T) {
	_, _, q := pinnedQuery(t, "test::Panel::labelled")
	unread := map[string]bool{}
	for _, u := range q.Unread {
		unread[u.Pin.Name] = true
		if u.Reason == "" {
			t.Errorf("%s is unread for no stated reason", u.Pin.Name)
		}
	}
	for _, name := range []string{"width", "finish", "maxSpeed"} {
		if !unread[name] {
			t.Errorf("%s is not read by this constraint, so it should be reported unread: %+v", name, q.Unread)
		}
	}
}

// TestFixedValuesAreOrderedByTheirVariable: the fixed values a query reports are
// in the order of the assertions fixing them, so the position each records is the
// assertion an unsat core names.
func TestFixedValuesAreOrderedByTheirVariable(t *testing.T) {
	ctx, idx := fixtureFile(t, "panel_pins.sysml")
	panel := symbolNamed(t, idx, "test::Panel")
	pins, _ := Fixed(ctx, panel, nil)
	sym := symbolNamed(t, idx, "test::Panel::polishedIsWide")
	q, err := ConstraintWith(ctx, sym, sym.OwnerScope, pins)
	if err != nil {
		t.Fatalf("translate with fixed values: %v", err)
	}
	if len(q.Pinned) != 2 {
		t.Fatalf("fixed %+v, want the finish and the width", q.Pinned)
	}
	previous := ""
	for _, p := range q.Pinned {
		if p.Var.Name < previous {
			t.Errorf("fixed values are out of order at %s", p.Var.Name)
		}
		previous = p.Var.Name
		at := q.Assertions[p.Index]
		if at.From.Role != RolePinned || at.From.Element != p.Var.Name {
			t.Errorf("assertion %d is %s of %s, want the value fixed for %s",
				p.Index, at.From.Role, at.From.Element, p.Var.Name)
		}
	}
}

// TestPinRefusesAnIncommensurableQuantity: a value measured in another dimension
// fixes nothing, since no scaling reconciles it; the refusal is typed and names
// the feature.
func TestPinRefusesAnIncommensurableQuantity(t *testing.T) {
	ctx, idx := fixtureFile(t, "panel_pins.sysml")
	length := quantityValue(t, ctx, idx, "test::Panel::clearance")
	sym := symbolNamed(t, idx, "test::Panel::speedIsBounded")
	speed := symbolNamed(t, idx, "test::Panel::maxSpeed")
	_, err := ConstraintWith(ctx, sym, sym.OwnerScope, []Pin{{
		Feature: speed, Name: "maxSpeed", Value: length, Source: PinHeld, Object: 7,
	}})
	var refusal *PinError
	if !errors.As(err, &refusal) || !errors.Is(err, ErrNotPinnable) {
		t.Fatalf("error %v, want a typed refusal to fix the value", err)
	}
	if !strings.Contains(refusal.Feature, "maxSpeed") || refusal.Reason == "" {
		t.Errorf("refusal %+v does not name the feature and why", refusal)
	}
}

// TestPinRefusesAValueWithNoLiteral: a value the term language cannot write is a
// refusal, never a query quietly missing what it was asked to fix.
func TestPinRefusesAValueWithNoLiteral(t *testing.T) {
	ctx, idx := fixtureFile(t, "panel_pins.sysml")
	sym := symbolNamed(t, idx, "test::Panel::fits")
	width := symbolNamed(t, idx, "test::Panel::width")
	_, err := ConstraintWith(ctx, sym, sym.OwnerScope, []Pin{{
		Feature: width, Name: "width", Value: runtime.Value{Kind: runtime.ValSequence}, Source: PinChosen,
	}})
	if !errors.Is(err, ErrNotPinnable) {
		t.Fatalf("error %v, want a refusal to fix a value with no literal", err)
	}
}

// TestPinRefusesAValueOfTheWrongType: a string does not fix a number, whatever
// the model states.
func TestPinRefusesAValueOfTheWrongType(t *testing.T) {
	ctx, idx := fixtureFile(t, "panel_pins.sysml")
	sym := symbolNamed(t, idx, "test::Panel::fits")
	width := symbolNamed(t, idx, "test::Panel::width")
	_, err := ConstraintWith(ctx, sym, sym.OwnerScope, []Pin{{
		Feature: width, Name: "width", Value: runtime.Value{Kind: runtime.ValString, Str: "wide"}, Source: PinChosen,
	}})
	if !errors.Is(err, ErrNotPinnable) {
		t.Fatalf("error %v, want a refusal to fix a number to a string", err)
	}
}

// TestPinnedBareNumberRefusesAMeasuredFeature: a plain number states no unit, so
// it cannot fix a feature whose values are magnitudes of a dimension.
func TestPinnedBareNumberRefusesAMeasuredFeature(t *testing.T) {
	ctx, idx := fixtureFile(t, "panel_pins.sysml")
	sym := symbolNamed(t, idx, "test::Panel::speedIsBounded")
	speed := symbolNamed(t, idx, "test::Panel::maxSpeed")
	bare := runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: 1.5}}
	_, err := ConstraintWith(ctx, sym, sym.OwnerScope, []Pin{{
		Feature: speed, Name: "maxSpeed", Value: bare, Source: PinChosen,
	}})
	if !errors.Is(err, ErrNotPinnable) {
		t.Fatalf("error %v, want a refusal to fix a magnitude to a bare number", err)
	}
}

// quantityValue evaluates the declared value of a feature of the fixture, which
// is how a test gets a runtime quantity without building one by hand.
func quantityValue(t *testing.T, ctx *runtime.Context, idx *symbols.Index, fqn string) runtime.Value {
	t.Helper()
	pins, unfixed := Fixed(ctx, symbolNamed(t, idx, "test::Panel"), nil)
	if len(unfixed) != 0 {
		t.Fatalf("declared values could not be read: %+v", unfixed)
	}
	for _, p := range pins {
		if p.Feature != nil && strings.HasSuffix(fqn, "::"+p.Name) {
			return p.Value
		}
	}
	t.Fatalf("%s declares no value to read", fqn)
	return runtime.Value{}
}
