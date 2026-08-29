package main

import (
	"context"
	"errors"
	"fmt"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/client/opensysml"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// pkgClient runs scenarios through the public Go API, client/opensysml, so the
// suite holds the API to the wire contract rather than holding raw stubs to
// it. The "pkg" protocol answers in process; "pkg-connect" dials the started
// service through opensysml.Dial.
type pkgClient struct {
	name   string
	api    opensysml.Client
	models map[string]*opensysml.Model // hash → the handle later calls take
}

func newPkgClient(name string, api opensysml.Client) *pkgClient {
	return &pkgClient{name: name, api: api, models: map[string]*opensysml.Model{}}
}

// uncoveredError marks a call the public API's v1 surface cannot express,
// which the runner reports as a skip rather than hiding.
type uncoveredError struct{ reason string }

func (e *uncoveredError) Error() string { return e.reason }

func (c *pkgClient) protocol() string { return c.name }

func (c *pkgClient) close() { _ = c.api.Close() }

func (c *pkgClient) call(ctx context.Context, method string, request protoreflect.Message) (protoreflect.Message, error) {
	response, err := c.dispatch(ctx, method, request)
	if err != nil {
		return nil, err
	}
	return response.ProtoReflect(), nil
}

// dispatch makes the call through the public API and rebuilds the wire
// response from what the API returned, so the comparison sees exactly what a
// Go caller was given.
func (c *pkgClient) dispatch(ctx context.Context, method string, request protoreflect.Message) (proto.Message, error) {
	switch method {
	case "GetServerInfo":
		return c.serverInfo(ctx)
	case "ParseFile":
		return c.parseFile(ctx, request)
	case "GetSymbol":
		return c.getSymbol(ctx, request)
	case "GetDiagnostics":
		return c.getDiagnostics(ctx, request)
	case "Evaluate":
		return c.evaluate(ctx, request)
	case "Instantiate":
		return c.instantiate(ctx, request)
	default:
		return nil, &uncoveredError{reason: fmt.Sprintf("the public Go API's v1 surface does not cover %s", method)}
	}
}

func (c *pkgClient) serverInfo(ctx context.Context) (proto.Message, error) {
	info, err := c.api.ServerInfo(ctx)
	if err != nil {
		return nil, apiError(err)
	}
	return &pb.ServerInfoResponse{Version: info.Version, Capabilities: info.Capabilities}, nil
}

func (c *pkgClient) parseFile(ctx context.Context, request protoreflect.Message) (proto.Message, error) {
	req := &pb.ParseFileRequest{}
	if err := retype(request, req); err != nil {
		return nil, err
	}
	var opts []opensysml.ParseOption
	if req.Language != "" {
		opts = append(opts, opensysml.WithLanguage(opensysml.Language(req.Language)))
	}
	if req.StrictConformance {
		opts = append(opts, opensysml.WithStrictConformance())
	}
	var model *opensysml.Model
	var err error
	switch source := req.Source.(type) {
	case *pb.ParseFileRequest_FilePath:
		model, err = c.api.ParseFile(ctx, source.FilePath, opts...)
	case *pb.ParseFileRequest_Content:
		model, err = c.api.ParseSource(ctx, source.Content, opts...)
	default:
		return nil, &uncoveredError{reason: "the public Go API cannot send a parse request naming no source"}
	}
	var failure *opensysml.FailureError
	if errors.As(err, &failure) {
		return &pb.ParseFileResponse{Error: failure.Message, Diagnostics: diagnosticsToProto(failure.Diagnostics)}, nil
	}
	if err != nil {
		return nil, apiError(err)
	}
	c.models[model.Hash] = model
	return &pb.ParseFileResponse{
		ModelHash:   model.Hash,
		Root:        symbolToProto(model.Root),
		Diagnostics: diagnosticsToProto(model.Diagnostics),
	}, nil
}

func (c *pkgClient) getSymbol(ctx context.Context, request protoreflect.Message) (proto.Message, error) {
	req := &pb.GetSymbolRequest{}
	if err := retype(request, req); err != nil {
		return nil, err
	}
	symbol, err := c.api.LookupSymbol(ctx, c.model(req.ModelHash), req.SymbolId)
	var failure *opensysml.FailureError
	if errors.As(err, &failure) {
		return &pb.SymbolResponse{Error: failure.Message}, nil
	}
	if err != nil {
		return nil, apiError(err)
	}
	return &pb.SymbolResponse{Symbol: symbolToProto(symbol)}, nil
}

func (c *pkgClient) getDiagnostics(ctx context.Context, request protoreflect.Message) (proto.Message, error) {
	req := &pb.DiagnosticsRequest{}
	if err := retype(request, req); err != nil {
		return nil, err
	}
	diagnostics, err := c.api.Diagnostics(ctx, c.model(req.ModelHash))
	var failure *opensysml.FailureError
	if errors.As(err, &failure) {
		return &pb.DiagnosticsResponse{Error: failure.Message}, nil
	}
	if err != nil {
		return nil, apiError(err)
	}
	return &pb.DiagnosticsResponse{Diagnostics: diagnosticsToProto(diagnostics)}, nil
}

func (c *pkgClient) evaluate(ctx context.Context, request protoreflect.Message) (proto.Message, error) {
	req := &pb.EvaluateRequest{}
	if err := retype(request, req); err != nil {
		return nil, err
	}
	var opts []opensysml.EvaluateOption
	if req.ContextSymbolId != "" {
		opts = append(opts, opensysml.WithContextSymbol(req.ContextSymbolId))
	}
	if req.SubjectSymbolId != "" {
		opts = append(opts, opensysml.WithSubject(req.SubjectSymbolId))
	}
	value, err := c.api.Evaluate(ctx, c.model(req.ModelHash), req.Expression, opts...)
	var failure *opensysml.FailureError
	if errors.As(err, &failure) {
		return &pb.EvaluateResponse{Error: failure.Message, Diagnostics: diagnosticsToProto(failure.Diagnostics)}, nil
	}
	if err != nil {
		return nil, apiError(err)
	}
	return &pb.EvaluateResponse{Result: valueToProto(value)}, nil
}

func (c *pkgClient) instantiate(ctx context.Context, request protoreflect.Message) (proto.Message, error) {
	req := &pb.InstantiateRequest{}
	if err := retype(request, req); err != nil {
		return nil, err
	}
	instantiation, err := c.api.Instantiate(ctx, c.model(req.ModelHash), req.SymbolId)
	var failure *opensysml.FailureError
	if errors.As(err, &failure) {
		return &pb.InstantiateResponse{Error: failure.Message, Diagnostics: diagnosticsToProto(failure.Diagnostics)}, nil
	}
	if err != nil {
		return nil, apiError(err)
	}
	response := &pb.InstantiateResponse{
		Instance:    instanceToProto(instantiation.Root),
		Diagnostics: diagnosticsToProto(instantiation.Diagnostics),
	}
	for _, instance := range instantiation.Instances {
		response.Instances = append(response.Instances, instanceToProto(instance))
	}
	return response, nil
}

// model answers the handle a hash names: the parse this client made, or a bare
// handle so an unknown hash reaches the service and is refused there.
func (c *pkgClient) model(hash string) *opensysml.Model {
	if model, ok := c.models[hash]; ok {
		return model
	}
	return &opensysml.Model{Hash: hash}
}

// retype copies a dynamic request into its generated type.
func retype(request protoreflect.Message, into proto.Message) error {
	if request == nil {
		return nil
	}
	data, err := proto.Marshal(request.Interface())
	if err != nil {
		return err
	}
	return proto.Unmarshal(data, into)
}

// apiError maps the public API's StatusError back to the gRPC status the
// runner compares, so the documented mapping is itself under test.
func apiError(err error) error {
	var statusErr *opensysml.StatusError
	if errors.As(err, &statusErr) {
		return status.Error(codes.Code(statusErr.Code), statusErr.Message)
	}
	return err
}

// The rebuilds from public types to wire types. Their fidelity is part of what
// the suite checks: a public type that dropped a field would fail comparison.

func symbolToProto(symbol *opensysml.Symbol) *pb.SymbolInfo {
	if symbol == nil {
		return nil
	}
	out := &pb.SymbolInfo{
		Id:                        symbol.ID,
		Name:                      symbol.Name,
		Kind:                      symbol.Kind,
		Metadata:                  symbol.Metadata,
		ChildIds:                  symbol.ChildIDs,
		WithheldLibraryAttributes: int32(symbol.WithheldLibraryAttributes), // #nosec G115 -- attribute counts are small
	}
	for _, attribute := range symbol.Attributes {
		out.Attributes = append(out.Attributes, &pb.AttributeInfo{
			Name:  attribute.Name,
			Type:  attribute.Type,
			Value: valueToProto(attribute.Value),
			Unit:  attribute.Unit,
		})
	}
	if symbol.Type != nil {
		out.TypeInfo = &pb.TypeInfo{
			Declared:        symbol.Type.Declared,
			ResolvedId:      symbol.Type.ResolvedID,
			ResolvedKind:    symbol.Type.ResolvedKind,
			Primitive:       symbol.Type.Primitive,
			PrimitiveSource: symbol.Type.PrimitiveSource,
			Quantity:        symbol.Type.Quantity,
			Unit:            symbol.Type.Unit,
		}
	}
	if symbol.Multiplicity != nil {
		out.Multiplicity = &pb.MultiplicityInfo{Lower: symbol.Multiplicity.Lower, Upper: symbol.Multiplicity.Upper}
	}
	for _, specialization := range symbol.Specializations {
		out.Specializations = append(out.Specializations, &pb.Specialization{
			Kind:       specialization.Kind,
			Declared:   specialization.Declared,
			TargetId:   specialization.TargetID,
			TargetKind: specialization.TargetKind,
		})
	}
	return out
}

func diagnosticsToProto(diagnostics []opensysml.Diagnostic) []*pb.Diagnostic {
	var out []*pb.Diagnostic
	for _, diagnostic := range diagnostics {
		converted := &pb.Diagnostic{Severity: diagnostic.Severity, Message: diagnostic.Message}
		if diagnostic.Span != nil {
			// #nosec G115 -- line and column numbers fit in int32.
			converted.Span = &pb.Span{
				File:      diagnostic.Span.File,
				StartLine: int32(diagnostic.Span.StartLine),
				StartCol:  int32(diagnostic.Span.StartCol),
				EndLine:   int32(diagnostic.Span.EndLine),
				EndCol:    int32(diagnostic.Span.EndCol),
			}
		}
		out = append(out, converted)
	}
	return out
}

func valueToProto(value opensysml.Value) *pb.Value {
	switch v := value.(type) {
	case nil:
		return nil
	case opensysml.Int:
		return &pb.Value{Kind: &pb.Value_IntValue{IntValue: int64(v)}}
	case opensysml.Real:
		return &pb.Value{Kind: &pb.Value_RealValue{RealValue: float64(v)}}
	case opensysml.Bool:
		return &pb.Value{Kind: &pb.Value_BoolValue{BoolValue: bool(v)}}
	case opensysml.String:
		return &pb.Value{Kind: &pb.Value_StringValue{StringValue: string(v)}}
	case opensysml.InstanceID:
		return &pb.Value{Kind: &pb.Value_InstanceId{InstanceId: int64(v)}}
	case opensysml.Sequence:
		sequence := &pb.ValueSequence{}
		for _, element := range v {
			sequence.Elements = append(sequence.Elements, valueToProto(element))
		}
		return &pb.Value{Kind: &pb.Value_Sequence{Sequence: sequence}}
	case opensysml.Null:
		return &pb.Value{Kind: &pb.Value_Null{Null: string(v)}}
	case opensysml.Unset:
		return &pb.Value{Kind: &pb.Value_Unset{Unset: true}}
	case opensysml.Quantity:
		return &pb.Value{Kind: &pb.Value_Quantity{Quantity: quantityToProto(v)}}
	case opensysml.EnumLiteral:
		return &pb.Value{Kind: &pb.Value_EnumLiteral{EnumLiteral: &pb.EnumLiteral{
			LiteralId:     v.LiteralID,
			EnumerationId: v.EnumerationID,
			Name:          v.Name,
		}}}
	default:
		return nil
	}
}

func quantityToProto(quantity opensysml.Quantity) *pb.Quantity {
	out := &pb.Quantity{Unit: quantity.Unit}
	switch magnitude := quantity.Magnitude.(type) {
	case opensysml.Int:
		out.Magnitude = &pb.Quantity_IntMagnitude{IntMagnitude: int64(magnitude)}
	case opensysml.Real:
		out.Magnitude = &pb.Quantity_RealMagnitude{RealMagnitude: float64(magnitude)}
	}
	if quantity.Term != nil {
		term := &pb.UnitTerm{ScaleNum: quantity.Term.ScaleNum, ScaleDen: quantity.Term.ScaleDen}
		for _, factor := range quantity.Term.Factors {
			term.Factors = append(term.Factors, &pb.UnitFactor{UnitId: factor.UnitID, Exponent: factor.Exponent})
		}
		out.UnitTerm = term
	}
	return out
}

func instanceToProto(instance *opensysml.Instance) *pb.Instance {
	if instance == nil {
		return nil
	}
	out := &pb.Instance{Id: instance.ID, TypeSymbolId: instance.TypeSymbolID}
	if len(instance.FeatureValues) > 0 {
		out.FeatureValues = make(map[string]*pb.FeatureValue, len(instance.FeatureValues))
		for name, featureValue := range instance.FeatureValues {
			converted := &pb.FeatureValue{
				FeatureName:  featureValue.FeatureName,
				Value:        valueToProto(featureValue.Value),
				Materialized: featureValue.Materialized,
				Error:        featureValue.Error,
			}
			for _, element := range featureValue.Values {
				converted.Values = append(converted.Values, valueToProto(element))
			}
			out.FeatureValues[name] = converted
		}
	}
	return out
}
