package opensysml

import (
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// The conversions from the wire types to the public ones. Every conversion
// copies: nothing a Client returns aliases engine state or a protobuf message,
// so the caller owns what it gets, in process or not.

func symbolFromProto(sym *pb.SymbolInfo) *Symbol {
	if sym == nil {
		return nil
	}
	out := &Symbol{
		ID:                        sym.Id,
		Name:                      sym.Name,
		Kind:                      sym.Kind,
		WithheldLibraryAttributes: int(sym.WithheldLibraryAttributes),
	}
	if len(sym.Metadata) > 0 {
		out.Metadata = make(map[string]string, len(sym.Metadata))
		for key, value := range sym.Metadata {
			out.Metadata[key] = value
		}
	}
	if len(sym.ChildIds) > 0 {
		out.ChildIDs = append([]string(nil), sym.ChildIds...)
	}
	for _, attr := range sym.Attributes {
		out.Attributes = append(out.Attributes, Attribute{
			Name:  attr.Name,
			Type:  attr.Type,
			Value: valueFromProto(attr.Value),
			Unit:  attr.Unit,
		})
	}
	if sym.TypeInfo != nil {
		out.Type = &TypeInfo{
			Declared:        sym.TypeInfo.Declared,
			ResolvedID:      sym.TypeInfo.ResolvedId,
			ResolvedKind:    sym.TypeInfo.ResolvedKind,
			Primitive:       sym.TypeInfo.Primitive,
			PrimitiveSource: sym.TypeInfo.PrimitiveSource,
			Quantity:        sym.TypeInfo.Quantity,
			Unit:            sym.TypeInfo.Unit,
		}
	}
	if sym.Multiplicity != nil {
		out.Multiplicity = &Multiplicity{Lower: sym.Multiplicity.Lower, Upper: sym.Multiplicity.Upper}
	}
	for _, spec := range sym.Specializations {
		out.Specializations = append(out.Specializations, Specialization{
			Kind:       spec.Kind,
			Declared:   spec.Declared,
			TargetID:   spec.TargetId,
			TargetKind: spec.TargetKind,
		})
	}
	return out
}

func diagnosticsFromProto(diags []*pb.Diagnostic) []Diagnostic {
	if len(diags) == 0 {
		return nil
	}
	out := make([]Diagnostic, 0, len(diags))
	for _, diag := range diags {
		converted := Diagnostic{Severity: diag.Severity, Message: diag.Message}
		if diag.Span != nil {
			converted.Span = &Span{
				File:      diag.Span.File,
				StartLine: int(diag.Span.StartLine),
				StartCol:  int(diag.Span.StartCol),
				EndLine:   int(diag.Span.EndLine),
				EndCol:    int(diag.Span.EndCol),
			}
		}
		out = append(out, converted)
	}
	return out
}

func valueFromProto(value *pb.Value) Value {
	if value == nil {
		return nil
	}
	switch kind := value.Kind.(type) {
	case *pb.Value_IntValue:
		return Int(kind.IntValue)
	case *pb.Value_RealValue:
		return Real(kind.RealValue)
	case *pb.Value_BoolValue:
		return Bool(kind.BoolValue)
	case *pb.Value_StringValue:
		return String(kind.StringValue)
	case *pb.Value_InstanceId:
		return InstanceID(kind.InstanceId)
	case *pb.Value_Sequence:
		elements := make(Sequence, 0, len(kind.Sequence.GetElements()))
		for _, element := range kind.Sequence.GetElements() {
			elements = append(elements, valueFromProto(element))
		}
		return elements
	case *pb.Value_Null:
		return Null(kind.Null)
	case *pb.Value_Quantity:
		return quantityFromProto(kind.Quantity)
	case *pb.Value_EnumLiteral:
		return EnumLiteral{
			LiteralID:     kind.EnumLiteral.GetLiteralId(),
			EnumerationID: kind.EnumLiteral.GetEnumerationId(),
			Name:          kind.EnumLiteral.GetName(),
		}
	case *pb.Value_Unset:
		return Unset{}
	default:
		return nil
	}
}

func quantityFromProto(quantity *pb.Quantity) Quantity {
	out := Quantity{Unit: quantity.GetUnit()}
	switch magnitude := quantity.GetMagnitude().(type) {
	case *pb.Quantity_IntMagnitude:
		out.Magnitude = Int(magnitude.IntMagnitude)
	case *pb.Quantity_RealMagnitude:
		out.Magnitude = Real(magnitude.RealMagnitude)
	}
	if term := quantity.GetUnitTerm(); term != nil {
		converted := &UnitTerm{ScaleNum: term.ScaleNum, ScaleDen: term.ScaleDen}
		for _, factor := range term.Factors {
			converted.Factors = append(converted.Factors, UnitFactor{UnitID: factor.UnitId, Exponent: factor.Exponent})
		}
		out.Term = converted
	}
	return out
}

func instanceFromProto(inst *pb.Instance) *Instance {
	if inst == nil {
		return nil
	}
	out := &Instance{ID: inst.Id, TypeSymbolID: inst.TypeSymbolId}
	if len(inst.FeatureValues) > 0 {
		out.FeatureValues = make(map[string]FeatureValue, len(inst.FeatureValues))
		for name, fv := range inst.FeatureValues {
			converted := FeatureValue{
				FeatureName:  fv.FeatureName,
				Value:        valueFromProto(fv.Value),
				Materialized: fv.Materialized,
				Error:        fv.Error,
			}
			for _, value := range fv.Values {
				converted.Values = append(converted.Values, valueFromProto(value))
			}
			out.FeatureValues[name] = converted
		}
	}
	return out
}
