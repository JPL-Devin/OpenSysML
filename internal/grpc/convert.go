package grpc

import (
	pb "github.com/Open-MBEE/Systemica/api/proto"
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/passes"
	"github.com/Open-MBEE/Systemica/internal/core/runtime"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// SymbolToProto converts a Symbol to protobuf SymbolInfo.
// This is the public API for gRPC service use.
func SymbolToProto(sym *symbols.Symbol, idx *symbols.Index) *pb.SymbolInfo {
	info := &pb.SymbolInfo{
		Id:       idx.GetFQN(sym), // Fully qualified name
		Name:     sym.Name,
		Kind:     sym.Kind.String(),
		Metadata: make(map[string]string),
	}

	// Extract metadata from AST node
	extractMetadata(sym, info.Metadata)

	// Add visibility to metadata
	info.Metadata["visibility"] = visibilityToString(sym.Visibility)

	// Collect child IDs
	if sym.Scope != nil {
		var childIDs []string
		for _, childSym := range sym.Scope.AllMembers() {
			childIDs = append(childIDs, idx.GetFQN(childSym))
		}
		info.ChildIds = childIDs
	}

	// Attributes populated later when semantic layer ready
	info.Attributes = []*pb.AttributeInfo{}

	return info
}

// DiagnosticToProto converts a passes.Diagnostic to protobuf.
func DiagnosticToProto(diag passes.Diagnostic, sf *source.SourceFile) *pb.Diagnostic {
	li := sf.Lines()
	start := li.PosAt(diag.Span.Offset)
	end := li.PosAt(diag.Span.End())

	return &pb.Diagnostic{
		Severity: diag.Severity.String(),
		Message:  diag.Message,
		Span: &pb.Span{
			File:      sf.Name(),
			StartLine: int32(start.Line),
			StartCol:  int32(start.Col),
			EndLine:   int32(end.Line),
			EndCol:    int32(end.Col),
		},
	}
}

// ParserDiagnosticToProto converts a parser.Diagnostic to protobuf.
func ParserDiagnosticToProto(diag parser.Diagnostic, sf *source.SourceFile) *pb.Diagnostic {
	li := sf.Lines()
	start := li.PosAt(diag.Span.Offset)
	end := li.PosAt(diag.Span.End())

	return &pb.Diagnostic{
		Severity: "error", // Parser diagnostics are always errors
		Message:  diag.Message,
		Span: &pb.Span{
			File:      sf.Name(),
			StartLine: int32(start.Line),
			StartCol:  int32(start.Col),
			EndLine:   int32(end.Line),
			EndCol:    int32(end.Col),
		},
	}
}

// convertSpan converts source.Span → proto.Span.
// Requires the SourceFile and LineIndex to map byte offsets to line:col.
func convertSpan(sp source.Span, sf *source.SourceFile, li *source.LineIndex) *pb.Span {
	start := li.PosAt(sp.Offset)
	end := li.PosAt(sp.End())
	return &pb.Span{
		File:      sf.Name(),
		StartLine: int32(start.Line),
		StartCol:  int32(start.Col),
		EndLine:   int32(end.Line),
		EndCol:    int32(end.Col),
	}
}

// extractMetadata populates the metadata map from the symbol's AST node.
// Extracts: multiplicity, type, direction, abstract.
func extractMetadata(sym *symbols.Symbol, meta map[string]string) {
	if sym.Decl == nil {
		return
	}

	switch decl := sym.Decl.(type) {
	case *ast.Usage:
		// Multiplicity
		if decl.Multiplicity != nil {
			meta["multiplicity"] = formatMultiplicity(decl.Multiplicity)
		}
		// Type (first typing relationship)
		for _, rel := range decl.Relationships {
			if rel.Kind == ast.RelTyping {
				if qn, ok := rel.Target.(*ast.QualifiedName); ok {
					meta["type"] = formatQualifiedName(qn)
					break
				}
			}
		}
		// Direction
		if decl.Direction != ast.DirNone {
			meta["direction"] = decl.Direction.String()
		}
		// Abstract
		if decl.IsAbstract {
			meta["abstract"] = "true"
		}

	case *ast.Definition:
		// Abstract
		if decl.IsAbstract {
			meta["abstract"] = "true"
		}
		// Type (first specializes relationship for definitions)
		for _, rel := range decl.Relationships {
			if rel.Kind == ast.RelSpecializes {
				if qn, ok := rel.Target.(*ast.QualifiedName); ok {
					meta["specializes"] = formatQualifiedName(qn)
					break
				}
			}
		}
	}
}

// formatMultiplicity renders Multiplicity as "lower..upper" or "value".
func formatMultiplicity(m *ast.Multiplicity) string {
	if !m.IsRange {
		return formatMultiplicityBound(m.Lower)
	}
	lower := formatMultiplicityBound(m.Lower)
	upper := formatMultiplicityBound(m.Upper)
	return lower + ".." + upper
}

// formatMultiplicityBound renders a multiplicity bound node as a string.
func formatMultiplicityBound(n ast.Node) string {
	if n == nil {
		return ""
	}
	switch v := n.(type) {
	case *ast.LiteralInteger:
		return v.Value
	case *ast.LiteralInfinity:
		return "*"
	default:
		return "?"
	}
}

// formatQualifiedName renders QualifiedName as "A::B::C".
func formatQualifiedName(qn *ast.QualifiedName) string {
	if qn == nil {
		return ""
	}
	var parts []string
	for _, seg := range qn.Parts {
		parts = append(parts, seg.Text)
	}
	return joinParts(parts, "::")
}

// joinParts joins parts with separator.
func joinParts(parts []string, sep string) string {
	result := ""
	for i, part := range parts {
		if i > 0 {
			result += sep
		}
		result += part
	}
	return result
}

// visibilityToString converts ast.Visibility to string.
func visibilityToString(v ast.Visibility) string {
	switch v {
	case ast.VisibilityPublic:
		return "public"
	case ast.VisibilityPrivate:
		return "private"
	case ast.VisibilityProtected:
		return "protected"
	case ast.VisibilityDefault:
		return "default"
	default:
		return "default"
	}
}

// ValueToProto converts runtime.Value to protobuf Value.
func ValueToProto(val runtime.Value) *pb.Value {
	switch val.Kind {
	case runtime.ValConst:
		// Map semantics.Value to protobuf based on type
		switch val.Const.Kind {
		case semantics.ValInt:
			return &pb.Value{Kind: &pb.Value_IntValue{IntValue: val.Const.Int}}
		case semantics.ValReal:
			return &pb.Value{Kind: &pb.Value_RealValue{RealValue: val.Const.Real}}
		case semantics.ValBool:
			return &pb.Value{Kind: &pb.Value_BoolValue{BoolValue: val.Const.Bool}}
		case semantics.ValInfinity:
			return &pb.Value{Kind: &pb.Value_StringValue{StringValue: "*"}}
		default:
			return &pb.Value{Kind: &pb.Value_Null{Null: "unsupported const kind"}}
		}
	case runtime.ValString:
		return &pb.Value{Kind: &pb.Value_StringValue{StringValue: val.Str}}
	case runtime.ValNull:
		return &pb.Value{Kind: &pb.Value_Null{Null: ""}}
	case runtime.ValInstance:
		return &pb.Value{Kind: &pb.Value_InstanceId{InstanceId: val.Instance}}
	case runtime.ValSequence:
		// Recursively convert sequence elements
		var pbElements []*pb.Value
		if val.Sequence != nil {
			for _, elem := range val.Sequence.Elements() {
				pbElements = append(pbElements, ValueToProto(elem))
			}
		}
		return &pb.Value{Kind: &pb.Value_Sequence{Sequence: &pb.ValueSequence{Elements: pbElements}}}
	default:
		return &pb.Value{Kind: &pb.Value_Null{Null: "unsupported"}}
	}
}

// InstanceToProto converts runtime.Instance to protobuf Instance.
func InstanceToProto(inst *runtime.Instance, idx *symbols.Index) *pb.Instance {
	pbSlots := make(map[string]*pb.SlotValue)
	
	for name, slot := range inst.Slots {
		pbSlot := &pb.SlotValue{
			FeatureName:  name,
			Materialized: slot.Materialized,
		}
		
		// Check multiplicity to determine scalar vs collection
		mult := slot.Feature.Multiplicity
		if !mult.Upper.Infinite && mult.Upper.Value <= 1 {
			// Scalar slot
			pbSlot.Value = ValueToProto(slot.Value)
		} else {
			// Collection slot
			if slot.Values.Kind == runtime.ValSequence && slot.Values.Sequence != nil {
				for _, elem := range slot.Values.Sequence.Elements() {
					pbSlot.Values = append(pbSlot.Values, ValueToProto(elem))
				}
			}
		}
		
		pbSlots[name] = pbSlot
	}
	
	return &pb.Instance{
		Id:            inst.ID,
		TypeSymbolId:  idx.GetFQN(inst.Type),
		Slots:         pbSlots,
	}
}
