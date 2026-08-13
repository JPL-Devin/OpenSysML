package grpc

import (
	"math"

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

// int32Clamp narrows a line or column number to the proto's int32, saturating
// rather than wrapping: a position past 2^31 would otherwise be reported as a
// negative one.
func int32Clamp(n int) int32 {
	if n > math.MaxInt32 {
		return math.MaxInt32
	}
	if n < math.MinInt32 {
		return math.MinInt32
	}
	return int32(n)
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
			StartLine: int32Clamp(start.Line),
			StartCol:  int32Clamp(start.Col),
			EndLine:   int32Clamp(end.Line),
			EndCol:    int32Clamp(end.Col),
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
			StartLine: int32Clamp(start.Line),
			StartCol:  int32Clamp(start.Col),
			EndLine:   int32Clamp(end.Line),
			EndCol:    int32Clamp(end.Col),
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
		StartLine: int32Clamp(start.Line),
		StartCol:  int32Clamp(start.Col),
		EndLine:   int32Clamp(end.Line),
		EndCol:    int32Clamp(end.Col),
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
	case runtime.ValQuantity:
		// The wire Value has no magnitude-and-unit form, and sending the bare
		// magnitude would drop the unit, so the value is reported unsupported.
		return &pb.Value{Kind: &pb.Value_Null{Null: "unsupported: quantity value"}}
	default:
		return &pb.Value{Kind: &pb.Value_Null{Null: "unsupported"}}
	}
}

// ProtoToValue converts a protobuf Value to a runtime.Value. It is the inverse
// of ValueToProto and is used to bind gRPC-supplied inputs into the runtime.
func ProtoToValue(pv *pb.Value) runtime.Value {
	if pv == nil {
		return runtime.Value{Kind: runtime.ValNull}
	}
	switch k := pv.GetKind().(type) {
	case *pb.Value_IntValue:
		return runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: k.IntValue}}
	case *pb.Value_RealValue:
		return runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: k.RealValue}}
	case *pb.Value_BoolValue:
		return runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValBool, Bool: k.BoolValue}}
	case *pb.Value_StringValue:
		return runtime.Value{Kind: runtime.ValString, Str: k.StringValue}
	case *pb.Value_InstanceId:
		return runtime.Value{Kind: runtime.ValInstance, Instance: k.InstanceId}
	case *pb.Value_Sequence:
		seq := runtime.NewSequence()
		if k.Sequence != nil {
			for _, elem := range k.Sequence.Elements {
				seq.Append(ProtoToValue(elem))
			}
		}
		return runtime.Value{Kind: runtime.ValSequence, Sequence: seq}
	case *pb.Value_Null:
		return runtime.Value{Kind: runtime.ValNull}
	default:
		return runtime.Value{Kind: runtime.ValNull}
	}
}

const (
	// maxGraphDepth bounds how deep Instantiate expands nested objects.
	maxGraphDepth = 8
	// maxGraphInstances caps how many instances one response serializes.
	maxGraphInstances = 1000
)

// InstanceGraphToProto converts inst and every instance reachable from it. The
// root is returned first; runtime instances live only for the duration of a
// request, so the whole reachable graph is serialized while the context is alive.
//
// Expansion stops at a child whose type is already on the path, at maxGraphDepth
// and at maxGraphInstances: reading a composite slot materializes the object it
// holds, so a self-referential part would otherwise instantiate forever. An
// unexpanded child stays a bare instance id.
func InstanceGraphToProto(rt *runtime.Context, inst *runtime.Instance, idx *symbols.Index) (*pb.Instance, []*pb.Instance) {
	var all []*pb.Instance
	seen := make(map[int64]bool)
	onPath := make(map[*symbols.Symbol]bool)

	var walk func(*runtime.Instance, int) *pb.Instance
	walk = func(cur *runtime.Instance, depth int) *pb.Instance {
		if seen[cur.ID] || len(all) >= maxGraphInstances {
			return nil
		}
		seen[cur.ID] = true
		onPath[cur.Type] = true
		defer delete(onPath, cur.Type)

		pbInst := InstanceToProto(rt, cur, idx)
		all = append(all, pbInst)

		if depth >= maxGraphDepth {
			return pbInst
		}

		for _, slot := range pbInst.Slots {
			for _, id := range instanceRefs(slot) {
				child, ok := rt.Instance(id)
				if !ok || onPath[child.Type] {
					continue
				}
				walk(child, depth+1)
			}
		}
		return pbInst
	}

	root := walk(inst, 0)
	return root, all
}

// instanceRefs collects the instance IDs a slot value references, scalar or not.
func instanceRefs(slot *pb.SlotValue) []int64 {
	var ids []int64
	var collect func(*pb.Value)
	collect = func(v *pb.Value) {
		switch k := v.GetKind().(type) {
		case *pb.Value_InstanceId:
			ids = append(ids, k.InstanceId)
		case *pb.Value_Sequence:
			for _, elem := range k.Sequence.GetElements() {
				collect(elem)
			}
		}
	}
	if slot.Value != nil {
		collect(slot.Value)
	}
	for _, v := range slot.Values {
		collect(v)
	}
	return ids
}

// InstanceToProto converts runtime.Instance to protobuf Instance. Slots are read
// through Instance.GetSlot, so a derived default is evaluated against the
// instance rather than reported as an unmaterialized slot.
func InstanceToProto(rt *runtime.Context, inst *runtime.Instance, idx *symbols.Index) *pb.Instance {
	pbSlots := make(map[string]*pb.SlotValue)

	for name := range inst.Slots {
		slot, err := inst.GetSlot(rt, name)
		if err != nil {
			pbSlots[name] = &pb.SlotValue{
				FeatureName: name,
				Error:       err.Error(),
			}
			continue
		}

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
		Id:           inst.ID,
		TypeSymbolId: idx.GetFQN(inst.Type),
		Slots:        pbSlots,
	}
}
