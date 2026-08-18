package grpc

import (
	"context"
	"errors"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/Systemica/api/proto"
	"github.com/Open-MBEE/Systemica/internal/core/runtime"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// quantityModel exercises every shape of quantity a feature value can hold: one written
// with a simple unit, one computed into a compound unit, one written in a scaled
// compound unit, and one inside a nested part.
const quantityModel = `
package P {
	import ScalarValues::*;

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
			if val.Kind != runtime.ValQuantity || val.Quantity == nil {
				t.Fatalf("kind = %v, want a quantity", val.Kind)
			}

			back := QuantityToProto(val.Quantity)
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
			if !val.Quantity.Unit.Term.Commensurable(mustUnitTerm(t, sent, idx, sem)) {
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
	return val.Quantity.Unit.Term
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
	if !val.Quantity.Unit.Term.Commensurable(mustUnitTerm(t, derived, idx, sem)) {
		t.Errorf("reduction = %s, want it commensurable with %s",
			val.Quantity.Unit.Term, describeUnitTerm(derived.GetUnitTerm()))
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
	if len(val.Quantity.Unit.Term.Factors) != 0 {
		t.Errorf("factors = %v, want none", val.Quantity.Unit.Term.Factors)
	}
}

// TestQuantityAsAnActionInput drives ExecuteAction with a quantity input: it is
// decoded against the model, and one that cannot be read is reported by name.
func TestQuantityAsAnActionInput(t *testing.T) {
	srv := mustNewService(t, 4)
	content := `
package A {
	import ScalarValues::*;

	action heavier {
		attribute mass : ISQ::MassValue = 1.0 [SI::kg];
		first start;
		action inner {
			assign mass := mass + 1.0 [SI::kg];
		}
		done end;
		then start inner;
		then inner end;
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
	import ScalarValues::*;

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
