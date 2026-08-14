package grpc

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"

	pb "github.com/Open-MBEE/Systemica/api/proto"
	"github.com/Open-MBEE/Systemica/internal/core/libs"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/passes"
	"github.com/Open-MBEE/Systemica/internal/core/resolve"
	"github.com/Open-MBEE/Systemica/internal/core/runtime"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// CapabilityTypeFacts names the capability of populating SymbolInfo.type_info,
// .multiplicity and .specializations, which typed code generation requires.
const CapabilityTypeFacts = "type_facts"

// CapabilityConvert names the capability of the Convert RPC, which writes a
// model back out as SysML notation or RDF Turtle.
const CapabilityConvert = "convert"

// capabilities is what this build supports, in report order. A capability is
// only ever added: renaming or dropping one breaks clients that require it.
var capabilities = []string{CapabilityTypeFacts, CapabilityConvert}

// Capabilities returns the capability names this build of the service reports.
func Capabilities() []string {
	return append([]string(nil), capabilities...)
}

// Service implements SysMLServiceServer
type Service struct {
	pb.UnimplementedSysMLServiceServer
	cache *Cache
	// budgets bounds every runtime context the service creates, read once from
	// the environment at construction.
	budgets runtime.Budgets
	// version is the build version GetServerInfo reports, informational only.
	version string
}

// NewService creates a gRPC service with specified cache size, reporting
// version as its build version. It returns an error if cacheSize is not
// positive, or if a budget variable holds anything but a positive integer.
func NewService(cacheSize int, version string) (*Service, error) {
	cache, err := NewCache(cacheSize)
	if err != nil {
		return nil, err
	}
	budgets, err := runtime.BudgetsFromEnv()
	if err != nil {
		return nil, err
	}
	return &Service{cache: cache, budgets: budgets, version: version}, nil
}

// GetServerInfo reports the service's build version and capabilities. A service
// too old to have this RPC fails the call with UNIMPLEMENTED, which is itself
// the answer that it predates every capability.
func (s *Service) GetServerInfo(ctx context.Context, req *pb.ServerInfoRequest) (*pb.ServerInfoResponse, error) {
	return &pb.ServerInfoResponse{
		Version:      s.version,
		Capabilities: Capabilities(),
	}, nil
}

// newRuntime returns a runtime context under the service's budgets.
func (s *Service) newRuntime(semModel *semantics.Model, resolver *resolve.Resolver) *runtime.Context {
	ctx := runtime.NewContext(semModel, resolver, s.budgets.MaxSteps)
	if err := ctx.SetBudgets(s.budgets); err != nil {
		// Unreachable: NewService validated these budgets.
		panic(fmt.Sprintf("grpc: invalid service budgets: %v", err))
	}
	return ctx
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
		// #nosec G304 -- the client names the model file it wants parsed; reading
		// arbitrary paths is the service's purpose, and it runs with the caller's
		// own privileges.
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, status.Errorf(codes.NotFound, "file not found: %v", err)
		}
		content = string(data)
	default:
		return nil, status.Error(codes.InvalidArgument, "source must be file_path or content")
	}

	// Keyed by what was read, not by the hash the request carried: a hash
	// disagreeing with its content would serve another model. The file name is
	// part of the key, since a record's diagnostics name the file it came from.
	modelHash := computeHash(filePath + "\x00" + content)
	if cached, ok := s.cache.Get(modelHash); ok {
		return s.buildParseResponse(modelHash, cached), nil
	}

	// Parse the file
	srcFile := source.New(filePath, []byte(content))
	p := parser.New(srcFile)
	root := p.ParseFile()

	// Get parser diagnostics
	parseDiags := p.Diagnostics

	// Build symbol index for lookups
	idx := symbols.NewIndex()

	// Load stdlib into index (required for type resolution)
	stdlibSrc := libs.DefaultSource()
	cache, _ := libs.NewCache() // Ignore cache errors, continue without
	loader := libs.NewLoader(stdlibSrc, cache)
	loaded := true
	for _, name := range stdlibSrc.List() {
		if err := loader.Load(name, idx); err != nil {
			loaded = false // Ignore load errors, continue
		}
	}

	// Expand wildcard imports (facade packages like ISQ re-exporting ISQMechanics)
	idx.ExpandWildcardImports()

	// Cache what was parsed, so the next request restores it instead. An
	// incomplete library is not cached: a record is keyed by content alone, so
	// it would be reused without the supertypes the missing file declared.
	if loaded {
		loader.Persist(idx)
	}

	// Add user document
	idx.AddDocument(filePath, root)

	// Run semantic passes (name-resolution, type, constraint)
	// Only run if no parse errors (tier gating per AGENTS.md §4)
	var passesDiags []passes.Diagnostic
	if len(parseDiags) == 0 {
		// passes.Analyze expects parser diagnostics converted to passes.Diagnostic
		parseDiagsConverted := make([]passes.Diagnostic, 0) // No parse errors to convert
		passesDiags = passes.Analyze(filePath, root, parseDiagsConverted, idx)
	}

	// Create cached model
	model := &CachedModel{
		Root:        root,
		Index:       idx,
		Source:      srcFile,
		ParseDiags:  parseDiags,
		PassesDiags: passesDiags,
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
		Symbol: SymbolToProtoIn(syms[0], cached.SymbolContext()),
	}, nil
}

// GetDiagnostics retrieves all diagnostics for a parsed model (parser + semantic passes)
func (s *Service) GetDiagnostics(ctx context.Context, req *pb.DiagnosticsRequest) (*pb.DiagnosticsResponse, error) {
	// Lookup cached model
	cached, ok := s.cache.Get(req.ModelHash)
	if !ok {
		return nil, status.Errorf(codes.NotFound, "model not found: %s", req.ModelHash)
	}

	// Convert parser diagnostics to proto
	var pbDiags []*pb.Diagnostic
	for _, diag := range cached.ParseDiags {
		pbDiags = append(pbDiags, ParserDiagnosticToProto(diag, cached.Source))
	}

	// Convert semantic pass diagnostics to proto
	for _, diag := range cached.PassesDiags {
		pbDiags = append(pbDiags, DiagnosticToProto(diag, cached.Source))
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
	runtimeCtx := s.newRuntime(semModel, resolver)

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
	runtimeCtx := s.newRuntime(semModel, resolver)

	// Instantiate
	inst, err := runtimeCtx.Instantiate(sym)
	if err != nil {
		return &pb.InstantiateResponse{
			Error: fmt.Sprintf("instantiation failed: %v", err),
		}, nil
	}

	root, all := InstanceGraphToProto(runtimeCtx, inst, cached.Index)
	return &pb.InstantiateResponse{
		Instance:  root,
		Instances: all,
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
	runtimeCtx := s.newRuntime(semModel, resolver)

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
	runtimeCtx := s.newRuntime(semModel, resolver)

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
	// Convert parser + semantic diagnostics
	var pbDiags []*pb.Diagnostic
	for _, diag := range model.ParseDiags {
		pbDiags = append(pbDiags, ParserDiagnosticToProto(diag, model.Source))
	}
	for _, diag := range model.PassesDiags {
		pbDiags = append(pbDiags, DiagnosticToProto(diag, model.Source))
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
			rootSymbol = SymbolToProtoIn(rootSyms[0], model.SymbolContext())
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
