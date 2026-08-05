package grpc

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"

	pb "github.com/Open-MBEE/Systemica/api/proto"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/resolve"
	"github.com/Open-MBEE/Systemica/internal/core/runtime"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Service implements SysMLServiceServer
type Service struct {
	pb.UnimplementedSysMLServiceServer
	cache *Cache
}

// NewService creates a gRPC service with specified cache size
func NewService(cacheSize int) *Service {
	return &Service{
		cache: NewCache(cacheSize),
	}
}

// ParseFile parses a SysML file and caches the result
func (s *Service) ParseFile(ctx context.Context, req *pb.ParseFileRequest) (*pb.ParseFileResponse, error) {
	// Extract source content and file path
	var content string
	var filePath string

	switch src := req.Source.(type) {
	case *pb.ParseFileRequest_Content:
		content = src.Content
		filePath = "<content>"
	case *pb.ParseFileRequest_FilePath:
		filePath = src.FilePath
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, status.Errorf(codes.NotFound, "file not found: %v", err)
		}
		content = string(data)
	default:
		return nil, status.Error(codes.InvalidArgument, "source must be file_path or content")
	}

	// Check cache using content hash
	if req.ContentHash != "" {
		if cached, ok := s.cache.Get(req.ContentHash); ok {
			// Cache hit - return cached model
			return s.buildParseResponse(req.ContentHash, cached), nil
		}
	}

	// Parse the file
	srcFile := source.New(filePath, []byte(content))
	p := parser.New(srcFile)
	root := p.ParseFile()

	// Get parser diagnostics
	diags := p.Diagnostics

	// Build symbol index for lookups
	idx := symbols.NewIndex()
	idx.AddDocument(filePath, root)

	// Create cached model
	model := &CachedModel{
		Root:   root,
		Index:  idx,
		Source: srcFile,
		Diags:  diags,
	}

	// Compute model hash
	modelHash := req.ContentHash
	if modelHash == "" {
		modelHash = computeHash(content)
	}

	// Cache the model
	s.cache.Put(modelHash, model)

	return s.buildParseResponse(modelHash, model), nil
}

// GetSymbol retrieves symbol information by FQN
func (s *Service) GetSymbol(ctx context.Context, req *pb.GetSymbolRequest) (*pb.SymbolResponse, error) {
	// Lookup cached model
	cached, ok := s.cache.Get(req.ModelHash)
	if !ok {
		return nil, status.Errorf(codes.NotFound, "model not found: %s", req.ModelHash)
	}

	// Lookup symbol by FQN
	syms := cached.Index.LookupQualified(req.SymbolId)
	if len(syms) == 0 {
		return &pb.SymbolResponse{
			Error: fmt.Sprintf("symbol not found: %s", req.SymbolId),
		}, nil
	}

	// Convert first match to proto
	return &pb.SymbolResponse{
		Symbol: SymbolToProto(syms[0], cached.Index),
	}, nil
}

// GetDiagnostics retrieves all diagnostics for a parsed model
func (s *Service) GetDiagnostics(ctx context.Context, req *pb.DiagnosticsRequest) (*pb.DiagnosticsResponse, error) {
	// Lookup cached model
	cached, ok := s.cache.Get(req.ModelHash)
	if !ok {
		return nil, status.Errorf(codes.NotFound, "model not found: %s", req.ModelHash)
	}

	// Convert diagnostics to proto
	var pbDiags []*pb.Diagnostic
	for _, diag := range cached.Diags {
		pbDiags = append(pbDiags, ParserDiagnosticToProto(diag, cached.Source))
	}

	return &pb.DiagnosticsResponse{
		Diagnostics: pbDiags,
	}, nil
}

// Evaluate evaluates a SysML expression in the context of a parsed model
func (s *Service) Evaluate(ctx context.Context, req *pb.EvaluateRequest) (*pb.EvaluateResponse, error) {
	// Lookup cached model
	cached, ok := s.cache.Get(req.ModelHash)
	if !ok {
		return nil, status.Errorf(codes.NotFound, "model not found: %s", req.ModelHash)
	}

	// Parse expression
	exprSource := source.New("<expression>", []byte(req.Expression))
	p := parser.New(exprSource)
	exprNode := p.ParseExpression()

	// Check for parse errors
	if len(p.Diagnostics) > 0 {
		var pbDiags []*pb.Diagnostic
		for _, diag := range p.Diagnostics {
			pbDiags = append(pbDiags, ParserDiagnosticToProto(diag, exprSource))
		}
		return &pb.EvaluateResponse{
			Diagnostics: pbDiags,
			Error:       "expression parse failed",
		}, nil
	}

	// Determine scope
	var scope *symbols.Scope
	if req.ContextSymbolId != "" {
		// Lookup context symbol
		syms := cached.Index.LookupQualified(req.ContextSymbolId)
		if len(syms) > 0 && syms[0].Scope != nil {
			scope = syms[0].Scope
		}
	}
	if scope == nil {
		// Use document root as default scope
		scope = cached.Index.DocumentRoot(cached.Source.Name())
	}

	// Create runtime context
	resolver := resolve.New(cached.Index)
	semModel := semantics.NewModel(resolver)
	runtimeCtx := runtime.NewContext(semModel, resolver, 100000)

	// Create eval context and evaluate
	evalCtx := runtime.NewEvalContext(runtimeCtx, scope)
	result, err := evalCtx.Eval(exprNode)
	if err != nil {
		return &pb.EvaluateResponse{
			Error: fmt.Sprintf("evaluation failed: %v", err),
		}, nil
	}

	return &pb.EvaluateResponse{
		Result: ValueToProto(result),
	}, nil
}

// Instantiate creates a runtime instance of a part/usage
func (s *Service) Instantiate(ctx context.Context, req *pb.InstantiateRequest) (*pb.InstantiateResponse, error) {
	// Lookup cached model
	cached, ok := s.cache.Get(req.ModelHash)
	if !ok {
		return nil, status.Errorf(codes.NotFound, "model not found: %s", req.ModelHash)
	}

	// Lookup symbol
	syms := cached.Index.LookupQualified(req.SymbolId)
	if len(syms) == 0 {
		return &pb.InstantiateResponse{
			Error: fmt.Sprintf("symbol not found: %s", req.SymbolId),
		}, nil
	}
	sym := syms[0]

	// Create runtime context
	resolver := resolve.New(cached.Index)
	semModel := semantics.NewModel(resolver)
	runtimeCtx := runtime.NewContext(semModel, resolver, 100000)

	// Instantiate
	inst, err := runtimeCtx.Instantiate(sym)
	if err != nil {
		return &pb.InstantiateResponse{
			Error: fmt.Sprintf("instantiation failed: %v", err),
		}, nil
	}

	return &pb.InstantiateResponse{
		Instance: InstanceToProto(inst, cached.Index),
	}, nil
}

// ExecuteAction executes an action definition
func (s *Service) ExecuteAction(ctx context.Context, req *pb.ExecuteActionRequest) (*pb.ExecuteActionResponse, error) {
	// Lookup cached model
	cached, ok := s.cache.Get(req.ModelHash)
	if !ok {
		return nil, status.Errorf(codes.NotFound, "model not found: %s", req.ModelHash)
	}

	// Lookup action symbol
	syms := cached.Index.LookupQualified(req.ActionSymbolId)
	if len(syms) == 0 {
		return &pb.ExecuteActionResponse{
			Error: fmt.Sprintf("action not found: %s", req.ActionSymbolId),
		}, nil
	}
	action := syms[0]

	// Create runtime context
	resolver := resolve.New(cached.Index)
	semModel := semantics.NewModel(resolver)
	runtimeCtx := runtime.NewContext(semModel, resolver, 100000)

	// Convert inputs from req.Inputs into runtime values for parameter binding.
	var inputs map[string]runtime.Value
	if len(req.Inputs) > 0 {
		inputs = make(map[string]runtime.Value, len(req.Inputs))
		for name, pv := range req.Inputs {
			inputs[name] = ProtoToValue(pv)
		}
	}

	// Execute action with the supplied inputs
	outputs, err := runtimeCtx.ExecuteActionWithInputs(action, inputs)
	if err != nil {
		return &pb.ExecuteActionResponse{
			Error: fmt.Sprintf("action execution failed: %v", err),
		}, nil
	}

	// Convert outputs to protobuf
	pbOutputs := make(map[string]*pb.Value)
	for name, val := range outputs {
		pbOutputs[name] = ValueToProto(val)
	}

	return &pb.ExecuteActionResponse{
		Outputs: pbOutputs,
	}, nil
}

// ExecuteState executes a state machine
func (s *Service) ExecuteState(ctx context.Context, req *pb.ExecuteStateRequest) (*pb.ExecuteStateResponse, error) {
	// Lookup cached model
	cached, ok := s.cache.Get(req.ModelHash)
	if !ok {
		return nil, status.Errorf(codes.NotFound, "model not found: %s", req.ModelHash)
	}

	// Lookup state machine symbol
	syms := cached.Index.LookupQualified(req.StateMachineSymbolId)
	if len(syms) == 0 {
		return &pb.ExecuteStateResponse{
			Error: fmt.Sprintf("state machine not found: %s", req.StateMachineSymbolId),
		}, nil
	}
	stateMachine := syms[0]

	// Create runtime context
	resolver := resolve.New(cached.Index)
	semModel := semantics.NewModel(resolver)
	runtimeCtx := runtime.NewContext(semModel, resolver, 100000)

	// Execute state machine, injecting the requested events and capturing the
	// real ordered state-visit trace.
	finalContext, statesVisited, err := runtimeCtx.ExecuteStateWithEvents(stateMachine, req.Events)
	if err != nil {
		return &pb.ExecuteStateResponse{
			Error: fmt.Sprintf("state machine execution failed: %v", err),
		}, nil
	}

	// Convert final context to protobuf
	pbContext := make(map[string]*pb.Value)
	for name, val := range finalContext {
		pbContext[name] = ValueToProto(val)
	}

	return &pb.ExecuteStateResponse{
		StatesVisited: statesVisited,
		FinalContext:  pbContext,
	}, nil
}

// buildParseResponse constructs ParseFileResponse from cached model
func (s *Service) buildParseResponse(modelHash string, model *CachedModel) *pb.ParseFileResponse {
	// Convert diagnostics
	var pbDiags []*pb.Diagnostic
	for _, diag := range model.Diags {
		pbDiags = append(pbDiags, ParserDiagnosticToProto(diag, model.Source))
	}

	// Convert root namespace to SymbolInfo
	var rootSymbol *pb.SymbolInfo
	if model.Root != nil {
		// Find root symbol in index
		rootSyms := model.Index.LookupQualified("") // Root has empty name
		if len(rootSyms) == 0 {
			// Fallback: use DocumentRoot
			rootScope := model.Index.DocumentRoot(model.Source.Name())
			if rootScope != nil {
				rootSymbol = &pb.SymbolInfo{
					Id:       "",
					Name:     "",
					Kind:     "RootNamespace",
					Metadata: make(map[string]string),
					ChildIds: collectChildIDs(rootScope, model.Index),
				}
			}
		} else {
			rootSymbol = SymbolToProto(rootSyms[0], model.Index)
		}
	}

	return &pb.ParseFileResponse{
		ModelHash:   modelHash,
		Root:        rootSymbol,
		Diagnostics: pbDiags,
	}
}

// collectChildIDs extracts child symbol FQNs from a scope
func collectChildIDs(scope *symbols.Scope, idx *symbols.Index) []string {
	var ids []string
	for _, sym := range scope.AllMembers() {
		ids = append(ids, idx.GetFQN(sym))
	}
	return ids
}

// computeHash generates SHA-256 hash of content
func computeHash(content string) string {
	hash := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", hash)
}
