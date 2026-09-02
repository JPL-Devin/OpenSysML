package grpc

import (
	"context"
	"errors"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// quantityModel exercises every shape of quantity a feature value can hold: one written
// with a simple unit, one computed into a compound unit, one written in a scaled
// compound unit, and one inside a nested part.
const quantityModel = `
package P {
	private import ScalarValues::*;

	part def Engine {
		attribute power : ISQ::PowerValue = 300.0 [SI::W];
	}

	part def Car {
		attribute m : ISQ::MassValue = 5.0 [SI::kg];
		attribute n : ScalarValues::Real = 2.0;
		attribute derivedSpeed = 10.0 [SI::m] / 2.0 [SI::s];
		attribute writtenSpeed = 5.4 [SI::km/SI::h];
		attribute count = 3 [SI::m];
		part engine : Engine;
	}
}
`

// mustQuantityModel parses quantityModel and returns the service, the model hash,
// the index the model's names resolve against and the semantics over it.
func mustQuantityModel(t *testing.T) (*Service, string, *symbols.Index, *semantics.Model) {
	t.Helper()

	srv := mustNewService(t, 4)
	resp, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source:      &pb.ParseFileRequest_Content{Content: quantityModel},
		ContentHash: "quantity-model",
	})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	for _, diag := range resp.Diagnostics {
		if diag.Severity == "error" {
			t.Fatalf("model has a diagnostic error: %s", diag.Message)
		}
	}
	cached, ok := srv.cache.Get(resp.ModelHash)
	if !ok {
		t.Fatal("parsed model is not cached")
	}
	return srv, resp.ModelHash, cached.Index, NewSymbolContext(cached.Index).Semantics
}

// mustEvaluateQuantity evaluates expr and returns the quantity it produced.
func mustEvaluateQuantity(t *testing.T, srv *Service, modelHash, expr string) *pb.Quantity {
	t.Helper()

	resp, err := srv.Evaluate(context.Background(), &pb.EvaluateRequest{
		ModelHash:  modelHash,
		Expression: expr,
	})
	if err != nil {
		t.Fatalf("Evaluate(%s): %v", expr, err)
	}
	if resp.Error != "" {
		t.Fatalf("Evaluate(%s): %s", expr, resp.Error)
	}
	quantity := resp.Result.GetQuantity()
	if quantity == nil {
		t.Fatalf("Evaluate(%s) = %v, want a quantity", expr, resp.Result)
	}
	return quantity
}

// TestQuantityCrossesTheWire pins what a quantity carries: the magnitude in the
// unit written, never reduced, plus the reduction that makes it comparable.
func TestQuantityCrossesTheWire(t *testing.T) {
	srv, modelHash, _, _ := mustQuantityModel(t)

	tests := []struct {
		expr       string
		unit       string
		real       float64
		intVal     int64
		isInt      bool
		reduction  string
		wantScaled bool
	}{
		// A prefixed unit reduces to its base unit and a scale: kg is 1000 grams.
		{expr: "5.0 [SI::kg]", unit: "SI::kg", real: 5.0, reduction: "1000/1·SI::gram", wantScaled: true},
		{expr: "3 [SI::m]", unit: "SI::m", intVal: 3, isInt: true, reduction: "SI::metre"},
		{expr: "10.0 [SI::m] / 2.0 [SI::s]", unit: "SI::m/SI::s", real: 5.0, reduction: "SI::metre·SI::second^-1"},
		{expr: "5.4 [SI::km/SI::h]", unit: "SI::km/SI::h", real: 5.4, reduction: "5/18·SI::metre·SI::second^-1", wantScaled: true},
	}

	for _, tc := range tests {
		t.Run(tc.expr, func(t *testing.T) {
			got := mustEvaluateQuantity(t, srv, modelHash, tc.expr)
			if got.GetUnit() != tc.unit {
				t.Errorf("unit = %q, want %q", got.GetUnit(), tc.unit)
			}
			if tc.isInt {
				if got.GetIntMagnitude() != tc.intVal {
					t.Errorf("int_magnitude = %d, want %d", got.GetIntMagnitude(), tc.intVal)
				}
			} else if got.GetRealMagnitude() != tc.real {
				t.Errorf("real_magnitude = %v, want %v", got.GetRealMagnitude(), tc.real)
			}
			if reduction := describeUnitTerm(got.GetUnitTerm()); reduction != tc.reduction {
				t.Errorf("reduction = %q, want %q", reduction, tc.reduction)
			}
			scaled := got.GetUnitTerm().GetScaleNum() != got.GetUnitTerm().GetScaleDen()
			if scaled != tc.wantScaled {
				t.Errorf("scale = %v/%v, want scaled = %v",
					got.GetUnitTerm().GetScaleNum(), got.GetUnitTerm().GetScaleDen(), tc.wantScaled)
			}
		})
	}
}

// TestQuantityRoundTrip is the fidelity requirement: a quantity that goes out
// and comes back is the same quantity, unit included — same magnitude, same unit
// as written, and a reduction over the very base-unit symbols it left with.
func TestQuantityRoundTrip(t *testing.T) {
	srv, modelHash, idx, sem := mustQuantityModel(t)

	for _, expr := range []string{
		"5.0 [SI::kg]",
		"3 [SI::m]",
		"10.0 [SI::m] / 2.0 [SI::s]",
		"5.4 [SI::km/SI::h]",
		"(2.0 [SI::m])**2",
	} {
		t.Run(expr, func(t *testing.T) {
			sent := mustEvaluateQuantity(t, srv, modelHash, expr)

			val, err := ProtoToValueIn(&pb.Value{Kind: &pb.Value_Quantity{Quantity: sent}}, idx, sem)
			if err != nil {
				t.Fatalf("ProtoToValueIn: %v", err)
			}
			if val.Kind != runtime.ValQuantity || val.Quantity() == nil {
				t.Fatalf("kind = %v, want a quantity", val.Kind)
			}

			back := QuantityToProto(val.Quantity())
			if back.GetUnit() != sent.GetUnit() {
				t.Errorf("unit = %q, want %q", back.GetUnit(), sent.GetUnit())
			}
			if back.GetIntMagnitude() != sent.GetIntMagnitude() || back.GetRealMagnitude() != sent.GetRealMagnitude() {
				t.Errorf("magnitude = %v, want %v", back.GetMagnitude(), sent.GetMagnitude())
			}
			if describeUnitTerm(back.GetUnitTerm()) != describeUnitTerm(sent.GetUnitTerm()) {
				t.Errorf("reduction = %q, want %q",
					describeUnitTerm(back.GetUnitTerm()), describeUnitTerm(sent.GetUnitTerm()))
			}
			if !val.Quantity().Unit.Term.Commensurable(mustUnitTerm(t, sent, idx, sem)) {
				t.Error("round-tripped quantity is not commensurable with the one sent")
			}
		})
	}
}

// mustUnitTerm rebuilds the unit term of sent, for comparing a round-trip
// against a second, independent reconstruction.
func mustUnitTerm(t *testing.T, sent *pb.Quantity, idx *symbols.Index, sem *semantics.Model) semantics.UnitTerm {
	t.Helper()

	val, err := ProtoToQuantity(sent, idx, sem)
	if err != nil {
		t.Fatalf("ProtoToQuantity: %v", err)
	}
	return val.Quantity().Unit.Term
}

// TestQuantityFromWireIsNormalized pins that a hand-built reduction — factors in
// any order, a base unit repeated, an exponent that cancels — is commensurable
// with the same unit the model derives, which compares factors element-wise.
func TestQuantityFromWireIsNormalized(t *testing.T) {
	srv, modelHash, idx, sem := mustQuantityModel(t)
	derived := mustEvaluateQuantity(t, srv, modelHash, "10.0 [SI::m] / 2.0 [SI::s]")

	byHand := &pb.Quantity{
		Magnitude: &pb.Quantity_RealMagnitude{RealMagnitude: 5},
		Unit:      "SI::m/SI::s",
		UnitTerm: &pb.UnitTerm{ScaleNum: 1, ScaleDen: 1, Factors: []*pb.UnitFactor{
			{UnitId: "SI::second", Exponent: -1},
			{UnitId: "SI::gram", Exponent: 0},
			{UnitId: "SI::metre", Exponent: 2},
			{UnitId: "SI::metre", Exponent: -1},
		}},
	}

	val, err := ProtoToQuantity(byHand, idx, sem)
	if err != nil {
		t.Fatalf("ProtoToQuantity: %v", err)
	}
	if !val.Quantity().Unit.Term.Commensurable(mustUnitTerm(t, derived, idx, sem)) {
		t.Errorf("reduction = %s, want it commensurable with %s",
			val.Quantity().Unit.Term, describeUnitTerm(derived.GetUnitTerm()))
	}
}

// TestQuantityFromWireNeedsTheModel pins the two ways a quantity cannot be read
// back: without the model's symbols, and over a base unit it does not declare.
func TestQuantityFromWireNeedsTheModel(t *testing.T) {
	srv, modelHash, idx, sem := mustQuantityModel(t)
	sent := mustEvaluateQuantity(t, srv, modelHash, "5.0 [SI::kg]")

	if _, err := ProtoToQuantity(sent, nil, nil); !errors.Is(err, ErrQuantityNeedsIndex) {
		t.Errorf("without an index: err = %v, want ErrQuantityNeedsIndex", err)
	}

	unknown := &pb.Quantity{
		Magnitude: &pb.Quantity_RealMagnitude{RealMagnitude: 1},
		Unit:      "Made::up",
		UnitTerm:  &pb.UnitTerm{ScaleNum: 1, ScaleDen: 1, Factors: []*pb.UnitFactor{{UnitId: "Made::up", Exponent: 1}}},
	}
	if _, err := ProtoToQuantity(unknown, idx, sem); !errors.Is(err, ErrUnknownBaseUnit) {
		t.Errorf("over an undeclared base unit: err = %v, want ErrUnknownBaseUnit", err)
	}

	for _, scale := range []*pb.UnitTerm{{ScaleNum: 1, ScaleDen: 0}, {ScaleNum: 0, ScaleDen: 1}} {
		unusable := &pb.Quantity{
			Magnitude: &pb.Quantity_RealMagnitude{RealMagnitude: 1},
			Unit:      "SI::m",
			UnitTerm:  scale,
		}
		if _, err := ProtoToQuantity(unusable, idx, sem); !errors.Is(err, ErrUnitScaleUnusable) {
			t.Errorf("over scale %g/%g: err = %v, want ErrUnitScaleUnusable",
				scale.ScaleNum, scale.ScaleDen, err)
		}
	}

	noMagnitude := &pb.Quantity{Unit: "SI::kg"}
	if _, err := ProtoToQuantity(noMagnitude, idx, sem); err == nil {
		t.Error("a quantity with no magnitude must be reported, not read as zero")
	}
}

// TestQuantityOverSomethingThatIsNotAUnit pins that a reduction is only accepted
// over measurement units: a name resolving to a part, or to nothing at all, is
// rejected rather than measured in.
func TestQuantityOverSomethingThatIsNotAUnit(t *testing.T) {
	_, _, idx, sem := mustQuantityModel(t)

	overAPart := &pb.Quantity{
		Magnitude: &pb.Quantity_RealMagnitude{RealMagnitude: 1},
		Unit:      "P::Car",
		UnitTerm:  &pb.UnitTerm{ScaleNum: 1, ScaleDen: 1, Factors: []*pb.UnitFactor{{UnitId: "P::Car", Exponent: 1}}},
	}
	if _, err := ProtoToQuantity(overAPart, idx, sem); !errors.Is(err, ErrNotAMeasurementUnit) {
		t.Errorf("over a part: err = %v, want ErrNotAMeasurementUnit", err)
	}

	// An empty name is a lookup of the document root, which would otherwise
	// resolve to exactly one symbol and pass as a base unit.
	unnamed := &pb.Quantity{
		Magnitude: &pb.Quantity_RealMagnitude{RealMagnitude: 1},
		Unit:      "made up",
		UnitTerm:  &pb.UnitTerm{ScaleNum: 1, ScaleDen: 1, Factors: []*pb.UnitFactor{{Exponent: 1}}},
	}
	if _, err := ProtoToQuantity(unnamed, idx, sem); !errors.Is(err, ErrUnknownBaseUnit) {
		t.Errorf("over an unnamed factor: err = %v, want ErrUnknownBaseUnit", err)
	}
}

// TestQuantityFeatureValuesAndNestedQuantities drives Instantiate: every quantity feature value
// of a part, and the quantity inside the part it holds, cross as quantities.
func TestQuantityFeatureValuesAndNestedQuantities(t *testing.T) {
	srv, modelHash, _, _ := mustQuantityModel(t)

	resp, err := srv.Instantiate(context.Background(), &pb.InstantiateRequest{
		ModelHash: modelHash,
		SymbolId:  "P::Car",
	})
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("Instantiate: %s", resp.Error)
	}

	for name, want := range map[string]string{
		"m":            "5 [SI::kg] = 1000/1·SI::gram",
		"derivedSpeed": "5 [SI::m/SI::s] = SI::metre·SI::second^-1",
		"writtenSpeed": "5.4 [SI::km/SI::h] = 5/18·SI::metre·SI::second^-1",
		"count":        "3 [SI::m] = SI::metre",
	} {
		fv, ok := resp.Instance.FeatureValues[name]
		if !ok {
			t.Errorf("missing feature value %q", name)
			continue
		}
		if fv.Error != "" {
			t.Errorf("feature value %q: %s", name, fv.Error)
			continue
		}
		if got := describeQuantity(fv.Value.GetQuantity()); got != want {
			t.Errorf("feature value %q = %q, want %q", name, got, want)
		}
	}

	// The ordinary real feature value is untouched by the quantity arm.
	if got := resp.Instance.FeatureValues["n"].GetValue().GetRealValue(); got != 2.0 {
		t.Errorf("feature value n = %v, want 2", got)
	}

	engineID := resp.Instance.FeatureValues["engine"].GetValue().GetInstanceId()
	if engineID == 0 {
		t.Fatal("feature value engine holds no instance")
	}
	var engine *pb.Instance
	for _, inst := range resp.Instances {
		if inst.Id == engineID {
			engine = inst
		}
	}
	if engine == nil {
		t.Fatalf("instance %d is not in the response graph", engineID)
	}
	wantPower := "300 [SI::W] = 1000/1·SI::gram·SI::metre^2·SI::second^-3"
	if got := describeQuantity(engine.FeatureValues["power"].GetValue().GetQuantity()); got != wantPower {
		t.Errorf("nested feature value power = %q, want %q", got, wantPower)
	}
}

// TestQuantityWithoutItsReduction pins that a named unit sent with no reduction
// is rejected: dimension one would make it commensurable with a bare number.
func TestQuantityWithoutItsReduction(t *testing.T) {
	_, _, idx, sem := mustQuantityModel(t)

	unreduced := &pb.Quantity{
		Magnitude: &pb.Quantity_RealMagnitude{RealMagnitude: 5},
		Unit:      "Furlongs::furlong",
	}
	if _, err := ProtoToQuantity(unreduced, idx, sem); !errors.Is(err, ErrUnitNotReduced) {
		t.Errorf("error = %v, want %v", err, ErrUnitNotReduced)
	}

	// A magnitude under no unit at all is dimension one, which is what it says.
	dimensionless := &pb.Quantity{Magnitude: &pb.Quantity_RealMagnitude{RealMagnitude: 5}}
	val, err := ProtoToQuantity(dimensionless, idx, sem)
	if err != nil {
		t.Fatalf("ProtoToQuantity: %v", err)
	}
	if len(val.Quantity().Unit.Term.Factors) != 0 {
		t.Errorf("factors = %v, want none", val.Quantity().Unit.Term.Factors)
	}
}

// TestQuantityFromWireComposesAsWritten pins that a quantity read back from the
// wire keeps the named units its unit text composes, so an operation on it in a
// calc cancels and merges them exactly as it does for a quantity written in the model.
func TestQuantityFromWireComposesAsWritten(t *testing.T) {
	srv := mustNewService(t, 4)
	source := `package Q {
	private import ScalarValues::*;
	private import SI::*;
	calc def Dist { in v; in dt; v * dt }
	calc def Area { in a; in b; a * b }
	calc def Metre { 2.0 [m] }
}
`
	hash := mustVerifyModel(t, srv, source, "quantity-composes-as-written")
	evaluate := func(calc string, args ...*pb.Quantity) *pb.Quantity {
		t.Helper()
		req := &pb.EvaluateCalcRequest{ModelHash: hash, SymbolId: calc}
		for _, arg := range args {
			req.Arguments = append(req.Arguments, &pb.Value{Kind: &pb.Value_Quantity{Quantity: arg}})
		}
		resp, err := srv.EvaluateCalc(context.Background(), req)
		if err != nil {
			t.Fatalf("EvaluateCalc %s: %v", calc, err)
		}
		if resp.Error != "" {
			t.Fatalf("EvaluateCalc %s reported %q", calc, resp.Error)
		}
		if resp.Result.GetQuantity() == nil {
			t.Fatalf("EvaluateCalc %s = %v, want a quantity", calc, resp.Result)
		}
		return resp.Result.GetQuantity()
	}

	speed := mustEvaluateQuantity(t, srv, hash, "3.0 [SI::m] / 1.0 [SI::s]")
	if speed.GetUnit() != "SI::m/SI::s" {
		t.Fatalf("speed crosses the wire in %q, want SI::m/SI::s", speed.GetUnit())
	}
	dist := evaluate("Q::Dist", speed, mustEvaluateQuantity(t, srv, hash, "2.0 [SI::s]"))
	if got := describeQuantity(dist); got != "6 [SI::m] = SI::metre" {
		t.Errorf("m/s * s over the wire = %s, want 6 [SI::m] = SI::metre", got)
	}

	// A scaled named unit stays the unit it was written in, as it does locally.
	byHand := &pb.Quantity{
		Magnitude: &pb.Quantity_RealMagnitude{RealMagnitude: 2},
		Unit:      "SI::km",
		UnitTerm:  mustEvaluateQuantity(t, srv, hash, "1.0 [SI::km]").GetUnitTerm(),
	}
	area := evaluate("Q::Area", byHand, byHand)
	if got := describeQuantity(area); got != "4 [SI::km**2] = 1e+06/1·SI::metre^2" {
		t.Errorf("km * km over the wire = %s, want 4 [SI::km**2] = 1e+06/1·SI::metre^2", got)
	}

	// Unit text that is no unit expression is one opaque unit: still a quantity
	// over the reduction sent, and still what the sender wrote.
	opaque := &pb.Quantity{
		Magnitude: &pb.Quantity_RealMagnitude{RealMagnitude: 5},
		Unit:      "metres per second",
		UnitTerm:  speed.GetUnitTerm(),
	}
	got := describeQuantity(evaluate("Q::Dist", opaque, mustEvaluateQuantity(t, srv, hash, "1.0 [SI::s]")))
	if got != "5 [SI::s*metres per second] = SI::metre" {
		t.Errorf("opaque unit over the wire = %s, want 5 [SI::s*metres per second] = SI::metre", got)
	}

	// A unit written short, as an import let the sender write it, is read in the
	// namespace of the base units it reduces to: `m` is SI::m, and cancels or
	// merges with SI::m written in full.
	unqualified := &pb.Quantity{
		Magnitude: &pb.Quantity_RealMagnitude{RealMagnitude: 5},
		Unit:      "m/s",
		UnitTerm:  speed.GetUnitTerm(),
	}
	got = describeQuantity(evaluate("Q::Dist", unqualified, mustEvaluateQuantity(t, srv, hash, "1.0 [SI::s]")))
	if got != "5 [m] = SI::metre" {
		t.Errorf("unqualified unit over the wire = %s, want 5 [m] = SI::metre", got)
	}
	metre := evaluate("Q::Metre")
	if metre.GetUnit() != "m" {
		t.Fatalf("a quantity written under an import crosses the wire in %q, want m", metre.GetUnit())
	}
	got = describeQuantity(evaluate("Q::Area", metre, mustEvaluateQuantity(t, srv, hash, "3.0 [SI::m]")))
	if got != "6 [m**2] = SI::metre^2" {
		t.Errorf("m * SI::m over the wire = %s, want 6 [m**2] = SI::metre^2", got)
	}

	// A short name those namespaces do not declare, or declare as a unit the
	// reduction contradicts, is opaque: it was not certainly that unit.
	for _, tc := range []struct{ unit, want string }{
		{"ft", "6 [SI::m*ft] = SI::metre^2"},
		{"km", "6 [SI::m*km] = SI::metre^2"},
	} {
		short := &pb.Quantity{
			Magnitude: &pb.Quantity_RealMagnitude{RealMagnitude: 2},
			Unit:      tc.unit,
			UnitTerm:  metre.GetUnitTerm(),
		}
		got = describeQuantity(evaluate("Q::Area", short, mustEvaluateQuantity(t, srv, hash, "3.0 [SI::m]")))
		if got != tc.want {
			t.Errorf("%s over a metre reduction * SI::m = %s, want %s", tc.unit, got, tc.want)
		}
	}
}

// TestQuantityFromWireRejectsUnitTextItsReductionContradicts pins that a unit
// written as one thing but reduced to another is rejected rather than read as
// the text for display and the reduction for arithmetic.
func TestQuantityFromWireRejectsUnitTextItsReductionContradicts(t *testing.T) {
	srv, hash, idx, sem := mustQuantityModel(t)
	seconds := mustEvaluateQuantity(t, srv, hash, "1.0 [SI::s]").GetUnitTerm()
	kilograms := mustEvaluateQuantity(t, srv, hash, "1.0 [SI::kg]").GetUnitTerm()
	metres := mustEvaluateQuantity(t, srv, hash, "1.0 [SI::m]").GetUnitTerm()

	for _, tc := range []struct {
		name string
		unit string
		term *pb.UnitTerm
	}{
		{"another dimension", "SI::m", seconds},
		{"a composed unit over another dimension", "SI::m/SI::s", kilograms},
		{"another scale of the same dimension", "SI::km", metres},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pq := &pb.Quantity{
				Magnitude: &pb.Quantity_RealMagnitude{RealMagnitude: 1},
				Unit:      tc.unit,
				UnitTerm:  tc.term,
			}
			if _, err := ProtoToQuantity(pq, idx, sem); !errors.Is(err, ErrUnitTextMismatch) {
				t.Errorf("ProtoToQuantity(%s over %s) err = %v, want ErrUnitTextMismatch",
					tc.unit, describeUnitTerm(tc.term), err)
			}
		})
	}

	// The same text over the reduction it does have is read, in either factor order.
	agreeing := &pb.Quantity{
		Magnitude: &pb.Quantity_RealMagnitude{RealMagnitude: 1},
		Unit:      "SI::m/SI::s",
		UnitTerm: &pb.UnitTerm{ScaleNum: 1, ScaleDen: 1, Factors: []*pb.UnitFactor{
			{UnitId: "SI::second", Exponent: -1},
			{UnitId: "SI::metre", Exponent: 1},
		}},
	}
	if _, err := ProtoToQuantity(agreeing, idx, sem); err != nil {
		t.Errorf("ProtoToQuantity(SI::m/SI::s over metre·second^-1): %v", err)
	}
}

// TestQuantityAsAnActionInput drives ExecuteAction with a quantity input: it is
// decoded against the model, and one that cannot be read is reported by name.
func TestQuantityAsAnActionInput(t *testing.T) {
	srv := mustNewService(t, 4)
	content := `
package A {
	private import ScalarValues::*;

	action heavier {
		attribute mass : ISQ::MassValue = 1.0 [SI::kg];
		first start;
		action inner {
			assign mass := mass + 1.0 [SI::kg];
		}
		done;
		succession first start then inner;
		succession first inner then done;
	}
}
`
	parseResp, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source:      &pb.ParseFileRequest_Content{Content: content},
		ContentHash: "quantity-action-input",
	})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	for _, diag := range parseResp.Diagnostics {
		if diag.Severity == "error" {
			t.Fatalf("model has a diagnostic error: %s", diag.Message)
		}
	}

	sent := mustEvaluateQuantity(t, srv, parseResp.ModelHash, "5.0 [SI::kg]")
	resp, err := srv.ExecuteAction(context.Background(), &pb.ExecuteActionRequest{
		ModelHash:      parseResp.ModelHash,
		ActionSymbolId: "A::heavier",
		Inputs:         map[string]*pb.Value{"mass": {Kind: &pb.Value_Quantity{Quantity: sent}}},
	})
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("ExecuteAction: %s", resp.Error)
	}
	want := "6 [SI::kg] = 1000/1·SI::gram"
	if got := describeQuantity(resp.Outputs["mass"].GetQuantity()); got != want {
		t.Errorf("output mass = %q, want %q", got, want)
	}

	unreadable := &pb.Quantity{
		Magnitude: &pb.Quantity_RealMagnitude{RealMagnitude: 5},
		Unit:      "Made::up",
		UnitTerm:  &pb.UnitTerm{ScaleNum: 1, ScaleDen: 1, Factors: []*pb.UnitFactor{{UnitId: "Made::up", Exponent: 1}}},
	}
	bad, err := srv.ExecuteAction(context.Background(), &pb.ExecuteActionRequest{
		ModelHash:      parseResp.ModelHash,
		ActionSymbolId: "A::heavier",
		Inputs:         map[string]*pb.Value{"mass": {Kind: &pb.Value_Quantity{Quantity: unreadable}}},
	})
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	if !strings.Contains(bad.Error, "mass") || !strings.Contains(bad.Error, "unknown base unit") {
		t.Errorf("error = %q, want it to name the input and the unknown base unit", bad.Error)
	}
}

// TestQuantityInVerdict drives a constraint over quantities, whose verdict path
// reports the values it compared.
func TestQuantityInVerdict(t *testing.T) {
	srv := mustNewService(t, 4)
	content := `
package V {
	private import ScalarValues::*;

	part def Car {
		attribute mass : ISQ::MassValue = 2500.0 [SI::kg];

		constraint withinLimit {
			mass <= 3000.0 [SI::kg]
		}
	}
}
`
	parseResp, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source:      &pb.ParseFileRequest_Content{Content: content},
		ContentHash: "quantity-verdict",
	})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	for _, diag := range parseResp.Diagnostics {
		if diag.Severity == "error" {
			t.Fatalf("model has a diagnostic error: %s", diag.Message)
		}
	}

	resp, err := srv.VerifyConstraint(context.Background(), &pb.VerifyConstraintRequest{
		ModelHash:       parseResp.ModelHash,
		SymbolId:        "V::Car::withinLimit",
		SubjectSymbolId: "V::Car",
	})
	if err != nil {
		t.Fatalf("VerifyConstraint: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("VerifyConstraint: %s", resp.Error)
	}
	if resp.Verdict == nil || !resp.Verdict.Holds {
		t.Fatalf("verdict = %v, want one that holds", resp.Verdict)
	}

	// The subject's quantity feature value reads back from the verdict, which is what makes
	// a verdict over quantities diagnosable from a client.
	var subject *pb.Instance
	for _, inst := range resp.Instances {
		if inst.Id == resp.Verdict.InstanceId {
			subject = inst
		}
	}
	if subject == nil {
		t.Fatalf("verdict instance %d is not in the response", resp.Verdict.InstanceId)
	}
	wantMass := "2500 [SI::kg] = 1000/1·SI::gram"
	if got := describeQuantity(subject.FeatureValues["mass"].GetValue().GetQuantity()); got != wantMass {
		t.Errorf("subject mass = %q, want %q", got, wantMass)
	}
}
