package grpc

import (
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

func (s *Service) symbolToProto(sym *symbols.Symbol, sc *SymbolContext) *pb.SymbolInfo {
	info := SymbolToProtoIn(sym, sc)
	if !s.capabilities.has(CapabilityTypeFacts) {
		info.TypeInfo = nil
		info.Multiplicity = nil
		info.Specializations = nil
	}
	if !s.capabilities.has(CapabilitySymbolAttributes) {
		info.Attributes = nil
		info.WithheldLibraryAttributes = 0
		return info
	}
	for _, attribute := range info.Attributes {
		s.filterValueCapabilities(attribute.Value)
	}
	return info
}

func (s *Service) valueToProto(rt *runtime.Context, value runtime.Value, idx *symbols.Index) *pb.Value {
	out := ValueToProtoIn(rt, value, idx)
	s.filterValueCapabilities(out)
	return out
}

func (s *Service) instanceGraphToProto(rt *runtime.Context, inst *runtime.Instance, idx *symbols.Index) (*pb.Instance, []*pb.Instance) {
	root, all := InstanceGraphToProto(rt, inst, idx)
	for _, instance := range all {
		s.filterInstanceCapabilities(instance)
	}
	return root, all
}

func (s *Service) filterInstanceCapabilities(instance *pb.Instance) {
	if instance == nil {
		return
	}
	if !s.capabilities.has(CapabilityFeatureValues) {
		instance.FeatureValues = nil
		return
	}
	for _, feature := range instance.FeatureValues {
		s.filterValueCapabilities(feature.Value)
		for _, value := range feature.Values {
			s.filterValueCapabilities(value)
		}
	}
}

func (s *Service) filterValueCapabilities(value *pb.Value) {
	if value == nil {
		return
	}
	switch kind := value.GetKind().(type) {
	case *pb.Value_Sequence:
		for _, element := range kind.Sequence.GetElements() {
			s.filterValueCapabilities(element)
		}
	case *pb.Value_EnumLiteral:
		if !s.capabilities.has(CapabilityEnumValues) {
			value.Kind = &pb.Value_Null{Null: "unsupported: enumeration literal"}
		}
	case *pb.Value_Unset:
		if !s.capabilities.has(CapabilityUnsetValue) {
			value.Kind = &pb.Value_Null{Null: "unsupported: unset value"}
		}
	case *pb.Value_Complex:
		if !s.capabilities.has(CapabilityComplexValues) {
			value.Kind = &pb.Value_Null{Null: "unsupported: complex number " + runtime.FormatComplex(ProtoToComplex(kind.Complex))}
		}
	}
}
