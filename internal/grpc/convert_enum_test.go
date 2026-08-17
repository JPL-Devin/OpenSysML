package grpc

import (
	"strings"
	"testing"

	pb "github.com/Open-MBEE/Systemica/api/proto"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/runtime"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

const enumWireModel = `package D {
  enum def Color { red; green; blue; }
  enum def Level { low; high; }
}`

// enumWireIndex indexes enumWireModel and returns the index with the literal named.
func enumWireIndex(t *testing.T, fqn string) (*symbols.Index, *symbols.Symbol) {
	t.Helper()
	root := parser.New(source.New("enum.sysml", []byte(enumWireModel))).ParseFile()
	idx := symbols.NewIndex()
	idx.AddDocument("enum.sysml", root)
	syms := idx.LookupQualified(fqn)
	if len(syms) == 0 {
		t.Fatalf("%s not found in index", fqn)
	}
	return idx, syms[0]
}

// TestEnumLiteralToProto verifies a literal travels as the declaration it is,
// not as an unsupported null.
func TestEnumLiteralToProto(t *testing.T) {
	idx, red := enumWireIndex(t, "D::Color::red")

	pv := ValueToProto(runtime.NewEnumLiteral(red), idx)
	lit := pv.GetEnumLiteral()
	if lit == nil {
		t.Fatalf("got kind %T, want enum_literal", pv.GetKind())
	}
	if lit.LiteralId != "D::Color::red" {
		t.Errorf("LiteralId: got %q, want %q", lit.LiteralId, "D::Color::red")
	}
	if lit.EnumerationId != "D::Color" {
		t.Errorf("EnumerationId: got %q, want %q", lit.EnumerationId, "D::Color")
	}
	if lit.Name != "Color::red" {
		t.Errorf("Name: got %q, want %q", lit.Name, "Color::red")
	}
}

// TestEnumLiteralRoundTrip verifies the wire form resolves back to the same
// literal, which is what makes two round-tripped values equal.
func TestEnumLiteralRoundTrip(t *testing.T) {
	idx, red := enumWireIndex(t, "D::Color::red")
	original := runtime.NewEnumLiteral(red)

	back, err := ProtoToValueIn(ValueToProto(original, idx), idx, nil)
	if err != nil {
		t.Fatalf("ProtoToValueIn: %v", err)
	}
	if back.Kind != runtime.ValEnumLiteral {
		t.Fatalf("Kind: got %v, want ValEnumLiteral", back.Kind)
	}
	// Identity, not text: the round-tripped value is the same declaration.
	if back.Literal != original.Literal {
		t.Errorf("Literal: got %v, want the D::Color::red declaration", back.Literal)
	}
}

// TestEnumLiteralRoundTripInSequence verifies a literal inside a sequence keeps
// its identity, and that distinct literals stay distinct.
func TestEnumLiteralRoundTripInSequence(t *testing.T) {
	idx, red := enumWireIndex(t, "D::Color::red")
	green := idx.LookupQualified("D::Color::green")
	if len(green) == 0 {
		t.Fatal("D::Color::green not found")
	}

	seq := runtime.NewSequence()
	seq.Append(runtime.NewEnumLiteral(red))
	seq.Append(runtime.NewEnumLiteral(green[0]))

	back, err := ProtoToValueIn(ValueToProto(runtime.Value{Kind: runtime.ValSequence, Sequence: seq}, idx), idx, nil)
	if err != nil {
		t.Fatalf("ProtoToValueIn: %v", err)
	}
	elems := back.Sequence.Elements()
	if len(elems) != 2 {
		t.Fatalf("got %d elements, want 2", len(elems))
	}
	if elems[0].Literal != red || elems[1].Literal != green[0] {
		t.Errorf("sequence elements lost their literal identity: %v", elems)
	}
}

// TestEnumLiteralUnresolvedIsAnError verifies a literal the model does not
// declare is reported rather than becoming a null or a made-up identity.
func TestEnumLiteralUnresolvedIsAnError(t *testing.T) {
	idx, _ := enumWireIndex(t, "D::Color::red")

	cases := map[string]*pb.EnumLiteral{
		"unknown literal": {LiteralId: "D::Color::purple", EnumerationId: "D::Color", Name: "Color::purple"},
		"not a literal":   {LiteralId: "D::Color", EnumerationId: "D::Color", Name: "Color"},
		"no literal_id":   {EnumerationId: "D::Color", Name: "Color::red"},
		"no enum literal": nil,
	}
	for name, lit := range cases {
		pv := &pb.Value{Kind: &pb.Value_EnumLiteral{EnumLiteral: lit}}
		if _, err := ProtoToValueIn(pv, idx, nil); err == nil {
			t.Errorf("%s: got no error, want one", name)
		}
	}

	// Without a model there is nothing to resolve against, which is an error too.
	pv := &pb.Value{Kind: &pb.Value_EnumLiteral{EnumLiteral: &pb.EnumLiteral{LiteralId: "D::Color::red"}}}
	if _, err := ProtoToValueIn(pv, nil, nil); err == nil || !strings.Contains(err.Error(), "no model") {
		t.Errorf("no index: got %v, want a no-model error", err)
	}
}

// TestInstantiate_EnumTypedSlotCarriesLiteral verifies an enum-typed default
// reaches a client as the literal it is rather than as an unsupported null.
func TestInstantiate_EnumTypedSlotCarriesLiteral(t *testing.T) {
	content := `
package D {
  enum def Color { red; green; blue; }
  part def Car {
    attribute c : Color = Color::red;
  }
}
`
	resp := instantiate(t, content, "enum-slot", "D::Car")
	slot := resp.Instance.FeatureValues["c"]
	if slot == nil {
		t.Fatalf("slot c not present in %v", resp.Instance.FeatureValues)
	}
	if slot.Error != "" {
		t.Fatalf("slot c: %s", slot.Error)
	}
	lit := slot.Value.GetEnumLiteral()
	if lit == nil {
		t.Fatalf("slot c: got kind %T, want enum_literal", slot.Value.GetKind())
	}
	if lit.LiteralId != "D::Color::red" || lit.Name != "Color::red" {
		t.Errorf("slot c: got %+v, want D::Color::red / Color::red", lit)
	}
}

// TestEnumValuesCapabilityReported verifies the enum-literal wire form is
// advertised, so a client can require it.
func TestEnumValuesCapabilityReported(t *testing.T) {
	for _, c := range Capabilities() {
		if c == CapabilityEnumValues {
			return
		}
	}
	t.Errorf("capabilities %v do not include %q", Capabilities(), CapabilityEnumValues)
}

// TestEnumLiteralWithoutDeclarationIsUnsupported verifies a value whose literal
// is missing is reported unsupported rather than sent as an empty identity.
func TestEnumLiteralWithoutDeclarationIsUnsupported(t *testing.T) {
	idx, _ := enumWireIndex(t, "D::Color::red")

	pv := ValueToProto(runtime.Value{Kind: runtime.ValEnumLiteral}, idx)
	if got := pv.GetNull(); !strings.Contains(got, "unresolved enumeration literal") {
		t.Errorf("got %q, want an unresolved-literal null", got)
	}
}
