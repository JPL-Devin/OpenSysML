package grpc

import (
	"errors"
	"fmt"
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
// This is the public API for gRPC service use. It builds a conversion context
// of its own; a caller converting several symbols of one model should build one
// with NewSymbolContext and call SymbolToProtoIn, so that name resolution is
// memoized across the symbols.
func SymbolToProto(sym *symbols.Symbol, idx *symbols.Index) *pb.SymbolInfo {
	return SymbolToProtoIn(sym, NewSymbolContext(idx))
}

// SymbolToProtoIn converts a Symbol to protobuf SymbolInfo in an existing
// conversion context.
func SymbolToProtoIn(sym *symbols.Symbol, sc *SymbolContext) *pb.SymbolInfo {
	defer sc.Lock()()

	idx := sc.Index
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

	// Static type facts: the resolved type, the declared multiplicity and every
	// generalization edge. These are what a client needs to reconstruct the
	// element's type without re-deriving it from the metadata strings.
	info.TypeInfo = sc.typeInfoOf(sym)
	info.Multiplicity = sc.multiplicityOf(sym)
	info.Specializations = sc.specializationsOf(sym)

	// The attributes the element has, own and inherited, with their resolved
	// types and constant default values.
	info.Attributes = sc.attributesOf(sym)

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

// ValueToProto converts runtime.Value to protobuf Value. An enumeration literal
// is named by its declaration, which idx supplies the qualified name of.
func ValueToProto(val runtime.Value, idx *symbols.Index) *pb.Value {
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
				pbElements = append(pbElements, ValueToProto(elem, idx))
			}
		}
		return &pb.Value{Kind: &pb.Value_Sequence{Sequence: &pb.ValueSequence{Elements: pbElements}}}
	case runtime.ValVariant:
		// The wire Value has no variant form: the object a selected variant
		// materialized is reported by identity, a valueless selection as unsupported.
		if id, ok := val.Object(); ok {
			return &pb.Value{Kind: &pb.Value_InstanceId{InstanceId: id}}
		}
		return &pb.Value{Kind: &pb.Value_Null{Null: "unsupported: variant selection"}}
	case runtime.ValQuantity:
		pq := QuantityToProto(val.Quantity)
		if pq == nil {
			return &pb.Value{Kind: &pb.Value_Null{Null: "unsupported: quantity with a non-numeric magnitude"}}
		}
		return &pb.Value{Kind: &pb.Value_Quantity{Quantity: pq}}
	case runtime.ValEnumLiteral:
		lit := enumLiteralToProto(val, idx)
		if lit == nil {
			return &pb.Value{Kind: &pb.Value_Null{Null: "unsupported: unresolved enumeration literal"}}
		}
		return &pb.Value{Kind: &pb.Value_EnumLiteral{EnumLiteral: lit}}
	default:
		return &pb.Value{Kind: &pb.Value_Null{Null: "unsupported"}}
	}
}

// enumLiteralToProto names a literal by the declaration it is, which is its
// identity, and by the enumeration declaring it. Nil for an unresolved literal.
func enumLiteralToProto(val runtime.Value, idx *symbols.Index) *pb.EnumLiteral {
	if val.Literal == nil {
		return nil
	}
	lit := &pb.EnumLiteral{
		LiteralId: idx.GetFQN(val.Literal),
		Name:      val.LiteralText(),
	}
	if enum := semantics.EnumerationOwning(val.Literal); enum != nil {
		lit.EnumerationId = idx.GetFQN(enum)
	}
	return lit
}

// QuantityToProto marshals a quantity: the magnitude in the unit written, plus
// what that unit reduces to. Reports nil for a magnitude that is not a number.
func QuantityToProto(q *runtime.Quantity) *pb.Quantity {
	if q == nil {
		return nil
	}
	pq := &pb.Quantity{Unit: q.Unit.Text, UnitTerm: unitTermToProto(q.Unit.Term)}
	switch q.Num.Kind {
	case semantics.ValInt:
		pq.Magnitude = &pb.Quantity_IntMagnitude{IntMagnitude: q.Num.Int}
	case semantics.ValReal:
		pq.Magnitude = &pb.Quantity_RealMagnitude{RealMagnitude: q.Num.Real}
	default:
		return nil
	}
	return pq
}

// unitTermToProto marshals a unit's reduction to base units, naming each base
// unit by qualified name: a symbol pointer means nothing to another process.
func unitTermToProto(term semantics.UnitTerm) *pb.UnitTerm {
	pt := &pb.UnitTerm{ScaleNum: term.Scale.Num, ScaleDen: term.Scale.Den}
	for _, f := range term.Factors {
		pt.Factors = append(pt.Factors, &pb.UnitFactor{
			UnitId:   symbols.FQNOf(f.Unit),
			Exponent: f.Exponent,
		})
	}
	return pt
}

var (
	// ErrQuantityNeedsIndex reports a quantity converted from the wire without
	// the model's symbols, which its base units can only be resolved against.
	ErrQuantityNeedsIndex = errors.New("a quantity needs the model's symbols to be converted from the wire")

	// ErrUnknownBaseUnit reports a base unit the model does not declare, so no
	// unit can be rebuilt from the reduction naming it.
	ErrUnknownBaseUnit = errors.New("unknown base unit")

	// ErrNotAMeasurementUnit reports a reduction naming a symbol that is not a
	// measurement unit, which nothing is measured in.
	ErrNotAMeasurementUnit = errors.New("not a measurement unit")

	// ErrUnitScaleUnusable reports a unit reduction whose scale is zero or
	// undefined, which no magnitude can be converted through.
	ErrUnitScaleUnusable = errors.New("unit scale is not a usable ratio")

	// ErrUnitNotReduced reports a named unit sent without its reduction, over
	// which alone commensurability is decided.
	ErrUnitNotReduced = errors.New("unit carries no reduction to base units")
)

// ProtoToValueIn converts a protobuf Value to a runtime.Value in the model idx
// and sem describe, resolving a quantity's base units against them. Inverse of
// ValueToProto.
func ProtoToValueIn(pv *pb.Value, idx *symbols.Index, sem *semantics.Model) (runtime.Value, error) {
	if pv == nil {
		return runtime.Value{Kind: runtime.ValNull}, nil
	}
	switch k := pv.GetKind().(type) {
	case *pb.Value_Quantity:
		return ProtoToQuantity(k.Quantity, idx, sem)
	case *pb.Value_EnumLiteral:
		return enumLiteralFromProto(k.EnumLiteral, idx)
	case *pb.Value_Sequence:
		seq := runtime.NewSequence()
		if k.Sequence != nil {
			for _, elem := range k.Sequence.Elements {
				val, err := ProtoToValueIn(elem, idx, sem)
				if err != nil {
					return runtime.Value{}, err
				}
				seq.Append(val)
			}
		}
		return runtime.Value{Kind: runtime.ValSequence, Sequence: seq}, nil
	default:
		return protoToScalar(pv), nil
	}
}

// ProtoToQuantity rebuilds a quantity from the wire: the magnitude as sent, in
// the unit as written, over the base units idx resolves its reduction to.
func ProtoToQuantity(pq *pb.Quantity, idx *symbols.Index, sem *semantics.Model) (runtime.Value, error) {
	if pq == nil {
		return runtime.Value{Kind: runtime.ValNull}, nil
	}
	if (idx == nil || sem == nil) && len(pq.GetUnitTerm().GetFactors()) > 0 {
		return runtime.Value{}, fmt.Errorf("%w: %s", ErrQuantityNeedsIndex, pq.GetUnit())
	}
	if pq.GetUnitTerm() == nil && pq.GetUnit() != "" {
		return runtime.Value{}, fmt.Errorf("%w: %s", ErrUnitNotReduced, pq.GetUnit())
	}
	term, err := protoToUnitTerm(pq.GetUnitTerm(), idx, sem)
	if err != nil {
		return runtime.Value{}, err
	}

	var num semantics.Value
	switch m := pq.GetMagnitude().(type) {
	case *pb.Quantity_IntMagnitude:
		num = semantics.Value{Kind: semantics.ValInt, Int: m.IntMagnitude}
	case *pb.Quantity_RealMagnitude:
		num = semantics.Value{Kind: semantics.ValReal, Real: m.RealMagnitude}
	default:
		return runtime.Value{}, fmt.Errorf("quantity in %q carries no magnitude", pq.GetUnit())
	}

	quantity := &runtime.Quantity{Num: num, Unit: runtime.Unit{Text: pq.GetUnit(), Term: term}}
	return runtime.Value{Kind: runtime.ValQuantity, Quantity: quantity}, nil
}

// protoToUnitTerm rebuilds a unit's reduction, normalized so a term sent in any
// factor order is commensurable with the same unit derived in the model. A name
// the model does not declare uniquely as a measurement unit is an error, not a
// factor over whatever symbol it happened to resolve to.
func protoToUnitTerm(pt *pb.UnitTerm, idx *symbols.Index, sem *semantics.Model) (semantics.UnitTerm, error) {
	if pt == nil {
		// A magnitude sent under no unit at all: dimension one.
		return semantics.UnitTerm{Scale: semantics.UnitScale(1)}, nil
	}
	scale := semantics.Scale{Num: pt.GetScaleNum(), Den: pt.GetScaleDen()}
	if scale.IsZero() {
		return semantics.UnitTerm{}, fmt.Errorf("%w: %g/%g", ErrUnitScaleUnusable, scale.Num, scale.Den)
	}
	term := semantics.UnitTerm{Scale: scale}
	for _, f := range pt.GetFactors() {
		// An empty name is a lookup of the document root, so it is rejected here
		// rather than resolved to a symbol that measures nothing.
		if f.GetUnitId() == "" {
			return semantics.UnitTerm{}, fmt.Errorf("%w: unit factor names no unit", ErrUnknownBaseUnit)
		}
		matches := idx.LookupQualified(f.GetUnitId())
		if len(matches) != 1 {
			return semantics.UnitTerm{}, fmt.Errorf("%w: %s", ErrUnknownBaseUnit, f.GetUnitId())
		}
		if !sem.IsMeasurementUnit(matches[0]) {
			return semantics.UnitTerm{}, fmt.Errorf("%w: %s", ErrNotAMeasurementUnit, f.GetUnitId())
		}
		term.Factors = append(term.Factors, semantics.UnitFactor{
			Unit:     matches[0],
			Exponent: f.GetExponent(),
		})
	}
	return term.Normalized(), nil
}

// enumLiteralFromProto resolves a literal against the model, since a literal is
// the declaration it names: one the model does not declare has no identity here.
func enumLiteralFromProto(lit *pb.EnumLiteral, idx *symbols.Index) (runtime.Value, error) {
	if lit == nil || lit.GetLiteralId() == "" {
		return runtime.Value{}, fmt.Errorf("enumeration literal: literal_id names no declaration")
	}
	if idx == nil {
		return runtime.Value{}, fmt.Errorf("enumeration literal %s: no model to resolve it against", lit.GetLiteralId())
	}
	for _, sym := range idx.LookupQualified(lit.GetLiteralId()) {
		if semantics.EnumerationOwning(sym) != nil {
			return runtime.NewEnumLiteral(sym), nil
		}
	}
	return runtime.Value{}, fmt.Errorf("%s is not an enumeration literal of this model", lit.GetLiteralId())
}

// protoToScalar converts the arms of Value that name no symbol and hold no
// nested value.
func protoToScalar(pv *pb.Value) runtime.Value {
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

		// InstanceToProto reads every slot through GetSlot, which is what
		// lazily materializes the children the ids below resolve to.
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

// collectionElements returns what a collection slot holds; a multi-valued
// feature's contents can be either a sequence or a set.
func collectionElements(val runtime.Value) []runtime.Value {
	switch val.Kind {
	case runtime.ValSequence:
		if val.Sequence != nil {
			return val.Sequence.Elements()
		}
	case runtime.ValSet:
		if val.Set != nil {
			return val.Set.Elements()
		}
	}
	return nil
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
			// Scalar slot. An unmaterialized one holds no value; marshalling it
			// anyway would report the empty value as an unsupported null.
			if slot.Materialized {
				pbSlot.Value = ValueToProto(slot.Value, idx)
			}
		} else {
			for _, elem := range collectionElements(slot.Values) {
				pbSlot.Values = append(pbSlot.Values, ValueToProto(elem, idx))
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
